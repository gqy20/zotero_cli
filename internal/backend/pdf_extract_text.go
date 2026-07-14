package backend

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"zotero_cli/internal/domain"
)

type AttachmentFullText struct {
	Attachment  domain.Attachment
	Text        string
	Source      string
	CacheHit    bool
	ContentPath string `json:"-"`
	ChunksPath  string `json:"-"`
}

type ItemFullTextResult struct {
	Text                 string
	PrimaryAttachmentKey string
	PrimaryContentPath   string `json:"-"`
	PrimaryChunksPath    string `json:"-"`
	Attachments          []AttachmentFullText
}

type PageText struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type AttachmentPageText struct {
	Attachment domain.Attachment `json:"attachment"`
	Pages      []PageText        `json:"pages"`
	Source     string            `json:"source,omitempty"`
	CacheHit   bool              `json:"cache_hit,omitempty"`
}

type ItemPageTextResult struct {
	Text                 string               `json:"text"`
	PrimaryAttachmentKey string               `json:"primary_attachment_key,omitempty"`
	Attachments          []AttachmentPageText `json:"attachments,omitempty"`
}

func (r *LocalReader) ExtractItemFullText(ctx context.Context, item domain.Item) (string, error) {
	result, err := r.ExtractItemAttachmentTexts(ctx, item)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (r *HybridReader) ExtractItemFullText(ctx context.Context, item domain.Item) (string, error) {
	textReader, ok := r.local.(interface {
		ExtractItemFullText(context.Context, domain.Item) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("extract-text requires local full-text extraction support")
	}
	text, err := textReader.ExtractItemFullText(ctx, item)
	if err != nil {
		return "", err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return text, nil
}

func (r *LocalReader) ExtractItemAttachmentTexts(ctx context.Context, item domain.Item) (ItemFullTextResult, error) {
	cache := r.fullTextCache()
	result := ItemFullTextResult{}
	bestScore := -1 << 30
	for _, attachment := range item.Attachments {
		if !strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			continue
		}
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return ItemFullTextResult{}, err
		}
		if ok && strings.TrimSpace(doc.Text) != "" {
			entry := AttachmentFullText{
				Attachment:  attachment,
				Text:        doc.Text,
				Source:      doc.Meta.Extractor,
				CacheHit:    doc.CacheHit,
				ContentPath: cache.contentPath(attachment.Key),
			}
			if _, err := os.Stat(cache.chunksPath(attachment.Key)); err == nil {
				entry.ChunksPath = cache.chunksPath(attachment.Key)
			}
			result.Attachments = append(result.Attachments, entry)
			score := primaryFullTextAttachmentScore(item, attachment, doc.Text)
			if score > bestScore {
				bestScore = score
				result.Text = entry.Text
				result.PrimaryAttachmentKey = entry.Attachment.Key
				result.PrimaryContentPath = entry.ContentPath
				result.PrimaryChunksPath = entry.ChunksPath
				r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
					FullTextSource:        entry.Source,
					FullTextAttachmentKey: entry.Attachment.Key,
					FullTextCacheHit:      entry.CacheHit,
				})
			}
		}
	}
	if result.Text == "" {
		return ItemFullTextResult{}, fmt.Errorf("no PDF attachment text available for item %s", item.Key)
	}
	return result, nil
}

func (r *HybridReader) ExtractItemAttachmentTexts(ctx context.Context, item domain.Item) (ItemFullTextResult, error) {
	textReader, ok := r.local.(interface {
		ExtractItemAttachmentTexts(context.Context, domain.Item) (ItemFullTextResult, error)
	})
	if !ok {
		return ItemFullTextResult{}, fmt.Errorf("extract-text requires local full-text extraction support")
	}
	result, err := textReader.ExtractItemAttachmentTexts(ctx, item)
	if err != nil {
		return ItemFullTextResult{}, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return result, nil
}

func (r *LocalReader) ExtractItemAttachmentPageTexts(ctx context.Context, item domain.Item) (ItemPageTextResult, error) {
	cache := r.fullTextCache()
	result := ItemPageTextResult{}
	bestScore := -1 << 30
	for _, attachment := range item.Attachments {
		if !strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			continue
		}
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return ItemPageTextResult{}, err
		}
		if !ok || strings.TrimSpace(doc.Text) == "" {
			continue
		}
		pages := chunksToPageTexts(doc.Chunks)
		if len(pages) == 0 {
			return ItemPageTextResult{}, fmt.Errorf("page filtering requires page-aware full-text cache for attachment %s; rebuild the full-text cache from the PDF", attachment.Key)
		}
		entry := AttachmentPageText{
			Attachment: attachment,
			Pages:      pages,
			Source:     doc.Meta.Extractor,
			CacheHit:   doc.CacheHit,
		}
		result.Attachments = append(result.Attachments, entry)
		score := primaryFullTextAttachmentScore(item, attachment, doc.Text)
		if score > bestScore {
			bestScore = score
			result.Text = joinPageTexts(pages)
			result.PrimaryAttachmentKey = attachment.Key
			r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
				FullTextSource:        entry.Source,
				FullTextAttachmentKey: entry.Attachment.Key,
				FullTextCacheHit:      entry.CacheHit,
			})
		}
	}
	if result.Text == "" {
		return ItemPageTextResult{}, fmt.Errorf("no page-aware PDF attachment text available for item %s", item.Key)
	}
	return result, nil
}

func (r *HybridReader) ExtractItemAttachmentPageTexts(ctx context.Context, item domain.Item) (ItemPageTextResult, error) {
	textReader, ok := r.local.(interface {
		ExtractItemAttachmentPageTexts(context.Context, domain.Item) (ItemPageTextResult, error)
	})
	if !ok {
		return ItemPageTextResult{}, fmt.Errorf("extract-text --pages requires local full-text extraction support")
	}
	result, err := textReader.ExtractItemAttachmentPageTexts(ctx, item)
	if err != nil {
		return ItemPageTextResult{}, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return result, nil
}

func chunksToPageTexts(chunks []chunk) []PageText {
	if len(chunks) == 0 {
		return nil
	}
	byPage := map[int][]string{}
	for _, ch := range chunks {
		if ch.Page <= 0 || strings.TrimSpace(ch.Text) == "" {
			continue
		}
		byPage[ch.Page] = append(byPage[ch.Page], ch.Text)
	}
	pages := make([]int, 0, len(byPage))
	for page := range byPage {
		pages = append(pages, page)
	}
	sort.Ints(pages)
	out := make([]PageText, 0, len(pages))
	for _, page := range pages {
		text := normalizeFullTextText(strings.Join(byPage[page], "\n"))
		if text == "" {
			continue
		}
		out = append(out, PageText{Page: page, Text: text})
	}
	return out
}

func joinPageTexts(pages []PageText) string {
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		if strings.TrimSpace(page.Text) != "" {
			parts = append(parts, page.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func primaryFullTextAttachmentScore(item domain.Item, attachment domain.Attachment, text string) int {
	score := 0
	combinedAttachment := strings.ToLower(strings.Join([]string{
		attachment.Title,
		attachment.Filename,
		attachment.ZoteroPath,
		attachment.ResolvedPath,
	}, " "))
	leadingText := strings.ToLower(text)
	if len(leadingText) > 1200 {
		leadingText = leadingText[:1200]
	}

	for _, token := range fullTextPrimaryTokens(item.Title) {
		if strings.Contains(combinedAttachment, token) {
			score += 4
		}
		if strings.Contains(leadingText, token) {
			score += 3
		}
	}

	switch {
	case strings.Contains(combinedAttachment, "supplementary"):
		score -= 50
	case strings.Contains(combinedAttachment, "supplemental"):
		score -= 50
	case strings.Contains(combinedAttachment, "reporting summary"):
		score -= 40
	case strings.Contains(combinedAttachment, "accepted article"):
		score -= 15
	}

	switch {
	case strings.Contains(leadingText, "supplementary information"):
		score -= 45
	case strings.Contains(leadingText, "supplemental information"):
		score -= 45
	case strings.Contains(leadingText, "reporting summary"):
		score -= 40
	case strings.Contains(leadingText, "accepted article"):
		score -= 15
	}

	if strings.Contains(leadingText, "abstract") {
		score += 8
	}
	if strings.Contains(leadingText, "introduction") {
		score += 5
	}
	return score
}

func fullTextPrimaryTokens(title string) []string {
	fields := strings.Fields(strings.ToLower(title))
	tokens := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		if len(token) < 4 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}
