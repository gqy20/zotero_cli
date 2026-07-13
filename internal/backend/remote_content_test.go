package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/domain"
)

func TestRemoteReader_FullTextPreview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items/ABC123/preview" {
			t.Fatalf("expected preview path, got %s", r.URL.Path)
		}
		writeOK(w, "preview text")
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	got, err := r.FullTextPreview(context.Background(), domain.Item{Key: "ABC123"})
	if err != nil {
		t.Fatalf("FullTextPreview: %v", err)
	}
	if got != "preview text" {
		t.Fatalf("unexpected preview: %q", got)
	}
}

func TestRemoteReader_FullTextSnippet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items/ABC123/snippet" {
			t.Fatalf("expected snippet path, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "speciation" {
			t.Fatalf("expected q=speciation, got %q", got)
		}
		writeOK(w, "snippet text")
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	got, err := r.FullTextSnippet(context.Background(), domain.Item{Key: "ABC123"}, "speciation")
	if err != nil {
		t.Fatalf("FullTextSnippet: %v", err)
	}
	if got != "snippet text" {
		t.Fatalf("unexpected snippet: %q", got)
	}
}

func TestRemoteReader_ExtractItemAttachmentTexts(t *testing.T) {
	expected := ItemFullTextResult{
		Text:                 "full text",
		PrimaryAttachmentKey: "ATT1",
		Attachments: []AttachmentFullText{{
			Attachment: domain.Attachment{Key: "ATT1", Title: "paper.pdf"},
			Text:       "full text",
			Source:     "pymupdf",
			CacheHit:   true,
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items/ABC123/text" {
			t.Fatalf("expected text path, got %s", r.URL.Path)
		}
		writeOK(w, expected)
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	got, err := r.ExtractItemAttachmentTexts(context.Background(), domain.Item{Key: "ABC123"})
	if err != nil {
		t.Fatalf("ExtractItemAttachmentTexts: %v", err)
	}
	if got.Text != expected.Text || got.PrimaryAttachmentKey != expected.PrimaryAttachmentKey || len(got.Attachments) != 1 {
		t.Fatalf("unexpected text result: %#v", got)
	}
}

func TestRemoteReader_ExtractItemAttachmentPageTexts(t *testing.T) {
	expected := ItemPageTextResult{
		Text:                 "page two text",
		PrimaryAttachmentKey: "ATT1",
		Attachments: []AttachmentPageText{{
			Attachment: domain.Attachment{Key: "ATT1", Title: "paper.pdf"},
			Pages:      []PageText{{Page: 2, Text: "page two text"}},
			Source:     "pymupdf",
			CacheHit:   true,
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items/ABC123/text" {
			t.Fatalf("expected text path, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("pages"); got != "true" {
			t.Fatalf("expected pages=true, got %q", got)
		}
		writeOK(w, expected)
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	got, err := r.ExtractItemAttachmentPageTexts(context.Background(), domain.Item{Key: "ABC123"})
	if err != nil {
		t.Fatalf("ExtractItemAttachmentPageTexts: %v", err)
	}
	if got.Text != expected.Text || got.PrimaryAttachmentKey != expected.PrimaryAttachmentKey || len(got.Attachments) != 1 {
		t.Fatalf("unexpected page text result: %#v", got)
	}
}

func TestRemoteReader_ReadItemAnnotations(t *testing.T) {
	expected := ItemAnnotationsResult{
		ItemKey:       "ABC123",
		AttachmentKey: "ATT1",
		PDFAnnotations: []PDFAnnotation{{
			Page: 1,
			Type: "highlight",
			Text: "hello",
		}},
		TotalPDF: 1,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items/ABC123/annotations" {
			t.Fatalf("expected annotations path, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("attachment") != "ATT1" {
			t.Fatalf("attachment query=%q", r.URL.Query().Get("attachment"))
		}
		writeOK(w, expected)
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	got, err := r.ReadItemAnnotations(context.Background(), domain.Item{Key: "ABC123"}, "ATT1")
	if err != nil {
		t.Fatalf("ReadItemAnnotations: %v", err)
	}
	if got.ItemKey != "ABC123" || got.AttachmentKey != "ATT1" || got.TotalPDF != 1 {
		t.Fatalf("unexpected annotations result: %#v", got)
	}
}
