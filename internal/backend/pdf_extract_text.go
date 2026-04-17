package backend

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/domain"
)

func (r *LocalReader) ExtractItemFullText(ctx context.Context, item domain.Item) (string, error) {
	cache := newFullTextCache(r.FullTextCacheDir)
	for _, attachment := range item.Attachments {
		if !strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			continue
		}
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return "", err
		}
		if ok && strings.TrimSpace(doc.Text) != "" {
			r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
				FullTextSource:        doc.Meta.Extractor,
				FullTextAttachmentKey: doc.Meta.AttachmentKey,
			})
			return doc.Text, nil
		}
	}
	return "", fmt.Errorf("no PDF attachment text available for item %s", item.Key)
}
