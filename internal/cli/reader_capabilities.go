package cli

import (
	"context"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type snippetReader interface {
	FullTextSnippet(context.Context, domain.Item, string) (string, error)
}

type previewReader interface {
	FullTextPreview(context.Context, domain.Item) (string, error)
}

type fullTextReader interface {
	ExtractItemFullText(context.Context, domain.Item) (string, error)
}

type attachmentTextReader interface {
	ExtractItemAttachmentTexts(context.Context, domain.Item) (backend.ItemFullTextResult, error)
}

type fullTextCacheChecker interface {
	IsFullTextCached(domain.Attachment) bool
}

type collectionItemKeyReader interface {
	CollectionItemKeys(context.Context, string, int) ([]string, error)
}

type cslJSONExporter interface {
	ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error)
}
