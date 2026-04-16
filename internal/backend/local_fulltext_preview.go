package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"zotero_cli/internal/domain"
)

const fullTextPreviewLimit = 280

func (r *LocalReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	doc, ok, err := r.loadFullTextDocument(ctx, item, "")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return normalizeFullTextPreview(doc.Text), nil
}

func (r *LocalReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	doc, ok, err := r.loadFullTextDocument(ctx, item, query)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return buildFullTextSnippet(doc.Text, query), nil
}

func (r *LocalReader) loadFullTextDocument(_ context.Context, item domain.Item, query string) (fullTextDocument, bool, error) {
	cache := newFullTextCache(r.FullTextCacheDir)
	bestDoc := fullTextDocument{}
	bestScore := -1
	for _, attachment := range item.Attachments {
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return fullTextDocument{}, false, err
		}
		if !ok || doc.Text == "" {
			continue
		}
		score := fullTextAttachmentMatchScore(attachment, doc.Text, query)
		if item.SnippetAttachmentKey != "" && item.SnippetAttachmentKey == attachment.Key {
			score += 100
		}
		if query == "" {
			score = 0
		}
		if bestScore < score || bestScore < 0 {
			bestDoc = doc
			bestScore = score
			if query == "" {
				break
			}
		}
	}
	if bestScore < 0 {
		return fullTextDocument{}, false, nil
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
		FullTextSource:        bestDoc.Meta.Extractor,
		FullTextAttachmentKey: bestDoc.Meta.AttachmentKey,
		FullTextCacheHit:      bestDoc.CacheHit,
	})
	return bestDoc, true, nil
}

func (r *HybridReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	previewer, ok := r.local.(interface {
		FullTextPreview(context.Context, domain.Item) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("show --snippet requires local or hybrid mode with local data")
	}
	preview, err := previewer.FullTextPreview(ctx, item)
	if err != nil {
		return "", err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return preview, nil
}

func (r *HybridReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	snippeter, ok := r.local.(interface {
		FullTextSnippet(context.Context, domain.Item, string) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("find --snippet requires local or hybrid mode with local data")
	}
	snippet, err := snippeter.FullTextSnippet(ctx, item, query)
	if err != nil {
		return "", err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return snippet, nil
}

func (r *LocalReader) buildFullTextDocument(item domain.Item, attachment domain.Attachment) (fullTextDocument, bool, error) {
	text, err := r.readAttachmentFullTextText(attachment.Key)
	if err != nil {
		return fullTextDocument{}, false, err
	}
	if text != "" {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return fullTextDocument{}, false, nil
		}
		return fullTextDocument{
			Text: normalizeFullTextText(text),
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ParentItemKey:   item.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Title:           item.Title,
				Creators:        joinFullTextCreators(item.Creators),
				Tags:            strings.Join(item.Tags, " "),
				AttachmentTitle: attachment.Title,
				AttachmentName:  firstNonEmptyString(attachment.Filename, attachment.Title),
				AttachmentPath:  firstNonEmptyString(attachment.ResolvedPath, attachment.ZoteroPath),
				Extractor:       "zotero_ft_cache",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
				ExtractedAt:     time.Now().UTC().Format(time.RFC3339),
			},
		}, true, nil
	}
	doc, ok, err := r.extractFullTextWithPyMuPDF(context.Background(), attachment)
	if err != nil {
		return fullTextDocument{}, false, err
	}
	if !ok {
		return fullTextDocument{}, false, nil
	}
	doc.Meta.ParentItemKey = item.Key
	doc.Meta.Title = item.Title
	doc.Meta.Creators = joinFullTextCreators(item.Creators)
	doc.Meta.Tags = strings.Join(item.Tags, " ")
	doc.Meta.AttachmentTitle = attachment.Title
	doc.Meta.AttachmentName = firstNonEmptyString(attachment.Filename, attachment.Title)
	doc.Meta.AttachmentPath = firstNonEmptyString(attachment.ResolvedPath, attachment.ZoteroPath)
	doc.Meta.ExtractedAt = time.Now().UTC().Format(time.RFC3339)
	return doc, true, nil
}

func (r *LocalReader) loadFullTextDocumentForAttachment(item domain.Item, attachment domain.Attachment, cache fullTextCache) (fullTextDocument, bool, error) {
	doc, ok, err := cache.Load(attachment)
	if err != nil {
		return fullTextDocument{}, false, err
	}
	if ok && doc.Text != "" {
		if err := cache.syncIndex(doc); err != nil {
			return fullTextDocument{}, false, err
		}
		doc.Meta.ParentItemKey = firstNonEmptyString(doc.Meta.ParentItemKey, item.Key)
		doc.CacheHit = true
		return doc, true, nil
	}
	doc, ok, err = r.buildFullTextDocument(item, attachment)
	if err != nil {
		return fullTextDocument{}, false, err
	}
	if !ok || doc.Text == "" {
		return fullTextDocument{}, false, nil
	}
	if err := cache.Save(doc); err != nil {
		return fullTextDocument{}, false, err
	}
	return doc, true, nil
}

func (r *LocalReader) readAttachmentFullTextText(attachmentKey string) (string, error) {
	if strings.TrimSpace(attachmentKey) == "" {
		return "", nil
	}
	cachePath := filepath.Join(r.StorageDir, attachmentKey, ".zotero-ft-cache")
	content, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}

func normalizeFullTextText(value string) string {
	replacer := strings.NewReplacer(
		"\u00ad", "",
		"\u00a0", " ",
		"\u202f", " ",
		"\r\n", "\n",
		"\r", "\n",
	)
	cleaned := replacer.Replace(value)
	return strings.TrimSpace(cleaned)
}

func normalizeFullTextPreview(value string) string {
	normalized := strings.Join(strings.Fields(value), " ")
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) <= fullTextPreviewLimit {
		return normalized
	}
	return string(runes[:fullTextPreviewLimit]) + "..."
}

func buildFullTextSnippet(text string, query string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		return ""
	}
	tokens := fullTextQueryTokens(query)
	if len(tokens) == 0 {
		return normalizeFullTextPreview(normalized)
	}

	lowerRunes := []rune(strings.ToLower(normalized))
	textRunes := []rune(normalized)
	bestIdx := -1
	bestLen := 0
	for _, token := range tokens {
		tokenRunes := []rune(strings.ToLower(token))
		if len(tokenRunes) == 0 {
			continue
		}
		idx := indexRuneSlice(lowerRunes, tokenRunes)
		if idx < 0 {
			continue
		}
		if bestIdx == -1 || idx < bestIdx || (idx == bestIdx && len(tokenRunes) > bestLen) {
			bestIdx = idx
			bestLen = len(tokenRunes)
		}
	}
	if bestIdx < 0 {
		return normalizeFullTextPreview(normalized)
	}
	const contextRadius = 60
	start := bestIdx - contextRadius
	if start < 0 {
		start = 0
	}
	end := bestIdx + bestLen + contextRadius
	if end > len(textRunes) {
		end = len(textRunes)
	}
	snippet := string(textRunes[start:end])
	snippet = strings.TrimSpace(snippet)
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(textRunes) {
		snippet += "..."
	}
	return snippet
}

func fullTextAttachmentMatchScore(attachment domain.Attachment, text string, query string) int {
	if strings.TrimSpace(query) == "" {
		return 0
	}
	score := 0
	for _, token := range fullTextQueryTokens(query) {
		lowerToken := strings.ToLower(token)
		if strings.Contains(strings.ToLower(text), lowerToken) {
			score += 10
		}
		if strings.Contains(strings.ToLower(attachment.Title), lowerToken) {
			score += 3
		}
		if strings.Contains(strings.ToLower(attachment.Filename), lowerToken) {
			score += 3
		}
		if strings.Contains(strings.ToLower(attachment.ZoteroPath), lowerToken) || strings.Contains(strings.ToLower(attachment.ResolvedPath), lowerToken) {
			score += 2
		}
		if strings.Contains(strings.ToLower(attachment.ContentType), lowerToken) {
			score++
		}
	}
	return score
}

func joinFullTextCreators(creators []domain.Creator) string {
	names := make([]string, 0, len(creators))
	for _, creator := range creators {
		if strings.TrimSpace(creator.Name) != "" {
			names = append(names, creator.Name)
		}
	}
	return strings.Join(names, " ")
}

func fullTextQueryTokens(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	tokens := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		if token == "" {
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

func indexRuneSlice(text []rune, token []rune) int {
	if len(token) == 0 || len(text) < len(token) {
		return -1
	}
outer:
	for i := 0; i <= len(text)-len(token); i++ {
		for j := range token {
			if text[i+j] != token[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
