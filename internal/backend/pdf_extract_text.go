package backend

import (
	"context"
	"fmt"
	"strings"

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
	for _, attachment := range item.Attachments {
		if !strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			continue
		}
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return ItemFullTextResult{}, err
		}
		if ok && strings.TrimSpace(doc.Text) != "" {
			result.Attachments = append(result.Attachments, AttachmentFullText{
				Attachment: attachment,
				Text:       doc.Text,
				Source:     doc.Meta.Extractor,
				CacheHit:   doc.CacheHit,
			})
			if result.Text == "" {
				result.Text = doc.Text
				result.PrimaryAttachmentKey = doc.Meta.AttachmentKey
				r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
					FullTextSource:        doc.Meta.Extractor,
					FullTextAttachmentKey: doc.Meta.AttachmentKey,
					FullTextCacheHit:      doc.CacheHit,
				})
			}
		}
	}
	if result.Text == "" {
		return ItemFullTextResult{}, fmt.Errorf("no PDF attachment text available for item %s", item.Key)
	}
	return result, nil
}
