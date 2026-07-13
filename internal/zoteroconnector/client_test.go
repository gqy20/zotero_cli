package zoteroconnector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientImportsPDF(t *testing.T) {
	var gotBody, gotMetadata string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/connector/ping":
			w.WriteHeader(http.StatusOK)
		case "/connector/saveStandaloneAttachment":
			if r.Method != http.MethodPost {
				t.Fatalf("method=%s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/pdf" {
				t.Fatalf("content type=%q", r.Header.Get("Content-Type"))
			}
			gotMetadata = r.Header.Get("X-Metadata")
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"canRecognize":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, server.Client())
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error=%v", err)
	}
	result, err := client.ImportPDF(context.Background(), ImportPDFRequest{
		SessionID: "session-1", Title: "paper.pdf", SourceURL: "file:///paper.pdf",
		Content: strings.NewReader("%PDF-test"), ContentLength: int64(len("%PDF-test")),
	})
	if err != nil {
		t.Fatalf("ImportPDF() error=%v", err)
	}
	if !result.CanRecognize || gotBody != "%PDF-test" {
		t.Fatalf("result=%+v body=%q", result, gotBody)
	}
	for _, fragment := range []string{`"sessionID":"session-1"`, `"title":"paper.pdf"`, `"url":"file:///paper.pdf"`} {
		if !strings.Contains(gotMetadata, fragment) {
			t.Fatalf("metadata=%q missing %q", gotMetadata, fragment)
		}
	}
}

func TestClientReportsConnectorError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "disabled", http.StatusForbidden)
	}))
	defer server.Close()
	err := New(server.URL, server.Client()).Ping(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("Ping() error=%v", err)
	}
}

func TestClientUpdatesSessionTarget(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	err := New(server.URL, server.Client()).UpdateSession(context.Background(), UpdateSessionRequest{SessionID: "session-1", Target: "C23"})
	if err != nil {
		t.Fatalf("UpdateSession() error=%v", err)
	}
	if !strings.Contains(got, `"sessionID":"session-1"`) || !strings.Contains(got, `"target":"C23"`) {
		t.Fatalf("body=%q", got)
	}
}

func TestClientWaitsForRecognizedItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connector/getRecognizedItem" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"itemType":"journalArticle","title":"Recognized paper"}`))
	}))
	defer server.Close()

	item, recognized, err := New(server.URL, server.Client()).WaitForRecognizedItem(context.Background(), "session-1")
	if err != nil || !recognized || item.Title != "Recognized paper" || item.ItemType != "journalArticle" {
		t.Fatalf("item=%+v recognized=%v error=%v", item, recognized, err)
	}
}

func TestClientRecognizedItemNotReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	_, recognized, err := New(server.URL, server.Client()).WaitForRecognizedItem(context.Background(), "session-1")
	if err != nil || recognized {
		t.Fatalf("recognized=%v error=%v", recognized, err)
	}
}
