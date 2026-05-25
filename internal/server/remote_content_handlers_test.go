package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type contentReader struct{}

func (contentReader) FindItems(ctx context.Context, opts backend.FindOptions) ([]domain.Item, error) {
	return []domain.Item{contentReaderItem()}, nil
}
func (contentReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return contentReaderItem(), nil
}
func (contentReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return nil, nil
}
func (contentReader) GetLibraryStats(ctx context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}
func (contentReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	return nil, nil
}
func (contentReader) ListTags(ctx context.Context) ([]backend.Tag, error) {
	return nil, nil
}
func (contentReader) ListCollections(ctx context.Context) ([]backend.Collection, error) {
	return nil, nil
}
func (contentReader) GetAttachmentFile(ctx context.Context, key string) (string, string, error) {
	return "", "", nil
}
func (contentReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	return "preview text", nil
}
func (contentReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	return "snippet for " + query, nil
}
func (contentReader) ExtractItemAttachmentTexts(ctx context.Context, item domain.Item) (backend.ItemFullTextResult, error) {
	return backend.ItemFullTextResult{
		Text:                 "full text",
		PrimaryAttachmentKey: "ATT1",
		Attachments: []backend.AttachmentFullText{{
			Attachment: contentReaderItem().Attachments[0],
			Text:       "full text",
			Source:     "pymupdf",
		}},
	}, nil
}
func (contentReader) ReadItemAnnotations(ctx context.Context, item domain.Item) (backend.ItemAnnotationsResult, error) {
	return backend.ItemAnnotationsResult{
		ItemKey:       item.Key,
		AttachmentKey: "ATT1",
		PDFPath:       "D:/secret.pdf",
		PDFAnnotations: []backend.PDFAnnotation{{
			Page: 1,
			Type: "highlight",
			Text: "hello",
		}},
		TotalPDF: 1,
	}, nil
}

func contentReaderItem() domain.Item {
	return domain.Item{
		Key:   "ABC123",
		Title: "Paper",
		Attachments: []domain.Attachment{{
			Key:          "ATT1",
			Title:        "paper.pdf",
			ContentType:  "application/pdf",
			ResolvedPath: "D:/secret.pdf",
			ZoteroPath:   "attachments:secret.pdf",
			Resolved:     true,
		}},
	}
}

func TestItemEndpointSanitizesAttachmentPaths(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(contentReader{}).RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/v1/items/ABC123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data domain.Item `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(resp.Data.Attachments))
	}
	if resp.Data.Attachments[0].ResolvedPath != "" || resp.Data.Attachments[0].ZoteroPath != "" {
		t.Fatalf("expected sanitized attachment paths, got %#v", resp.Data.Attachments[0])
	}
}

func TestPreviewSnippetTextAndAnnotationsEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(contentReader{}).RegisterRoutes(mux)

	tests := []string{
		"/api/v1/items/ABC123/preview",
		"/api/v1/items/ABC123/snippet?q=speciation",
		"/api/v1/items/ABC123/text",
		"/api/v1/items/ABC123/annotations",
	}
	for _, path := range tests {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	var textResp struct {
		Data backend.ItemFullTextResult `json:"data"`
	}
	textReq := httptest.NewRequest("GET", "/api/v1/items/ABC123/text", nil)
	textRec := httptest.NewRecorder()
	mux.ServeHTTP(textRec, textReq)
	if err := json.Unmarshal(textRec.Body.Bytes(), &textResp); err != nil {
		t.Fatalf("unmarshal text response: %v", err)
	}
	if len(textResp.Data.Attachments) != 1 || textResp.Data.Attachments[0].Attachment.ResolvedPath != "" {
		t.Fatalf("expected sanitized text attachment payload, got %#v", textResp.Data.Attachments)
	}

	var annResp struct {
		Data backend.ItemAnnotationsResult `json:"data"`
	}
	annReq := httptest.NewRequest("GET", "/api/v1/items/ABC123/annotations", nil)
	annRec := httptest.NewRecorder()
	mux.ServeHTTP(annRec, annReq)
	if err := json.Unmarshal(annRec.Body.Bytes(), &annResp); err != nil {
		t.Fatalf("unmarshal annotations response: %v", err)
	}
	if annResp.Data.PDFPath != "" {
		t.Fatalf("expected sanitized pdf_path, got %q", annResp.Data.PDFPath)
	}
}
