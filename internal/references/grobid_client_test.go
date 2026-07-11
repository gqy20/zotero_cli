package references

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestGrobidClientProcessRetriesAndCaches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/isalive" {
			io.WriteString(w, "true")
			return
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.FormFile("input"); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<TEI/>")
	}))
	defer server.Close()
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewGrobidClient(server.URL, filepath.Join(dir, "cache"), "", 5*time.Second)
	data, hit, err := client.Process(context.Background(), pdf, "ITEM", false)
	if err != nil || hit || string(data) != "<TEI/>" {
		t.Fatalf("first data=%s hit=%v err=%v", data, hit, err)
	}
	_, hit, err = client.Process(context.Background(), pdf, "ITEM", false)
	if err != nil || !hit || calls.Load() != 2 {
		t.Fatalf("cache hit=%v calls=%d err=%v", hit, calls.Load(), err)
	}
}
