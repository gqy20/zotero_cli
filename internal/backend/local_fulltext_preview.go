package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/domain"
)

const fullTextPreviewLimit = 280

func (r *LocalReader) FullTextPreview(_ context.Context, item domain.Item) (string, error) {
	for _, attachment := range item.Attachments {
		preview, err := r.readAttachmentFullTextPreview(attachment.Key)
		if err != nil {
			return "", err
		}
		if preview != "" {
			return preview, nil
		}
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
	return previewer.FullTextPreview(ctx, item)
}

func (r *LocalReader) readAttachmentFullTextPreview(attachmentKey string) (string, error) {
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
	return normalizeFullTextPreview(string(content)), nil
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
