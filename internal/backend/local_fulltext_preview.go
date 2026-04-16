package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zotero_cli/internal/domain"
)

const fullTextPreviewLimit = 280

func (r *LocalReader) FullTextPreview(_ context.Context, item domain.Item) (string, error) {
	cache := newFullTextCache(r.FullTextCacheDir)
	for _, attachment := range item.Attachments {
		doc, ok, err := cache.Load(attachment)
		if err != nil {
			return "", err
		}
		if ok && doc.Text != "" {
			r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
				FullTextSource:        doc.Meta.Extractor,
				FullTextAttachmentKey: doc.Meta.AttachmentKey,
				FullTextCacheHit:      true,
			})
			return normalizeFullTextPreview(doc.Text), nil
		}
		doc, ok, err = r.buildFullTextDocument(item, attachment)
		if err != nil {
			return "", err
		}
		if !ok || doc.Text == "" {
			continue
		}
		if err := cache.Save(doc); err != nil {
			return "", err
		}
		r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
			FullTextSource:        doc.Meta.Extractor,
			FullTextAttachmentKey: doc.Meta.AttachmentKey,
		})
		return normalizeFullTextPreview(doc.Text), nil
	}
	return "", nil
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
