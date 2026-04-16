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
	doc, ok, err := r.loadFullTextDocument(ctx, item)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return normalizeFullTextPreview(doc.Text), nil
}

func (r *LocalReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	doc, ok, err := r.loadFullTextDocument(ctx, item)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return buildFullTextSnippet(doc.Text, query), nil
}

func (r *LocalReader) loadFullTextDocument(_ context.Context, item domain.Item) (fullTextDocument, bool, error) {
	cache := newFullTextCache(r.FullTextCacheDir)
	for _, attachment := range item.Attachments {
		doc, ok, err := cache.Load(attachment)
		if err != nil {
			return fullTextDocument{}, false, err
		}
		if ok && doc.Text != "" {
			if err := cache.syncIndex(doc); err != nil {
				return fullTextDocument{}, false, err
			}
			r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
				FullTextSource:        doc.Meta.Extractor,
				FullTextAttachmentKey: doc.Meta.AttachmentKey,
				FullTextCacheHit:      true,
			})
			return doc, true, nil
		}
		doc, ok, err = r.buildFullTextDocument(item, attachment)
		if err != nil {
			return fullTextDocument{}, false, err
		}
		if !ok || doc.Text == "" {
			continue
		}
		if err := cache.Save(doc); err != nil {
			return fullTextDocument{}, false, err
		}
		r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
			FullTextSource:        doc.Meta.Extractor,
			FullTextAttachmentKey: doc.Meta.AttachmentKey,
		})
		return doc, true, nil
	}
	return fullTextDocument{}, false, nil
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
	doc.Meta.ExtractedAt = time.Now().UTC().Format(time.RFC3339)
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
