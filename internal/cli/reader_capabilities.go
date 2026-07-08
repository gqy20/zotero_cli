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

type attachmentPageTextReader interface {
	ExtractItemAttachmentPageTexts(context.Context, domain.Item) (backend.ItemPageTextResult, error)
}

type itemAnnotationsReader interface {
	ReadItemAnnotations(context.Context, domain.Item) (backend.ItemAnnotationsResult, error)
}

type pdfAnnotationReader interface {
	ReadPDFAnnotations(context.Context, domain.Attachment) (backend.ReadAnnotationsResult, error)
}

type pdfAnnotationDeleter interface {
	DeletePDFAnnotations(context.Context, domain.Attachment, backend.DeleteAnnotationsRequest) (backend.DeleteAnnotationsResult, error)
}

type dbAnnotationDeleter interface {
	DeleteDBAnnotations(context.Context, string, backend.DeleteAnnotationsRequest) (backend.DeleteDBAnnotationsResult, error)
}

type pdfAnnotator interface {
	AnnotatePDF(context.Context, domain.Attachment, backend.AnnotateRequest) (backend.AnnotateResult, error)
}

type itemAnnotator interface {
	AnnotateItem(context.Context, domain.Item, backend.AnnotateRequest) (backend.AnnotateResult, error)
}

type itemAnnotationClearer interface {
	ClearItemAnnotations(context.Context, domain.Item, backend.DeleteAnnotationsRequest) (backend.ItemAnnotationClearResult, error)
}

type attachmentFullTextReader interface {
	ExtractAttachmentFullText(context.Context, domain.Item, domain.Attachment) (backend.FullTextDocument, bool, error)
}

type attachmentFullTextExtractor interface {
	ExtractAttachmentFullTextOnly(context.Context, domain.Item, domain.Attachment) (backend.FullTextDocument, bool, error)
}

type fullTextWriter interface {
	SaveFullText(backend.FullTextDocument) error
}

type fullTextBatchWriter interface {
	SaveFullTextBatch([]backend.FullTextDocument) error
}

type fullTextCacheChecker interface {
	IsFullTextCached(domain.Attachment) bool
	IsMarkedFailed(string) bool
}

type fullTextIndexChecker interface {
	IsFullTextIndexed(domain.Attachment) bool
}

type failedMarker interface {
	IsMarkedFailed(string) bool
	MarkExtractFailed(string) error
}

type collectionItemKeyReader interface {
	CollectionItemKeys(context.Context, string, int) ([]string, error)
}

type cslJSONExporter interface {
	ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error)
}
