package backend

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"zotero_cli/internal/domain"
)

type AttachmentFullText struct {
	Attachment domain.Attachment
	Text       string
	Source     string
	CacheHit   bool
}

type ItemFullTextResult struct {
	Text                 string
	PrimaryAttachmentKey string
	Attachments          []AttachmentFullText
}

func (r *LocalReader) ExtractItemFullText(ctx context.Context, item domain.Item) (string, error) {
	result, err := r.ExtractItemAttachmentTexts(ctx, item)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (r *LocalReader) ExtractItemAttachmentTexts(ctx context.Context, item domain.Item) (ItemFullTextResult, error) {
	cache := newFullTextCache(r.FullTextCacheDir)
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
				Attachment: attachment,
				Text:       doc.Text,
				Source:     doc.Meta.Extractor,
				CacheHit:   doc.CacheHit,
			}
			result.Attachments = append(result.Attachments, entry)
			score := primaryFullTextAttachmentScore(item, attachment, doc.Text)
			if score > bestScore {
				bestScore = score
				result.Text = entry.Text
				result.PrimaryAttachmentKey = entry.Attachment.Key
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
