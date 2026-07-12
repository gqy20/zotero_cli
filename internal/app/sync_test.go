package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
)

func TestSyncUsesConfiguredServerAndDefaultMirror(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/manifest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"sqlite": []any{}, "storage": []any{}, "fulltext": []any{}}})
	}))
	defer server.Close()

	configDir := t.TempDir()
	dir := filepath.Join(configDir, "sync")
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{ServerAddr: server.URL}, "", nil
	}
	service.DefaultPath = func() (string, error) { return filepath.Join(configDir, ".env"), nil }
	result, err := service.Sync(context.Background(), SyncRequest{})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := result.Data.(SyncSummary)
	if !ok || summary.DataDir != dir || !summary.Storage {
		t.Fatalf("unexpected summary: %#v", result.Data)
	}
}

func TestDownloadOneResumesMatchingPartialFile(t *testing.T) {
	target := t.TempDir()
	content := []byte("0123456789")
	file := fileDownload{relPath: "KEY/file.pdf", size: int64(len(content)), mtime: 123}
	dest := filepath.Join(target, "KEY", "file.pdf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	part := dest + ".part-10-123"
	if err := os.WriteFile(part, content[:4], 0o644); err != nil {
		t.Fatal(err)
	}
	fetch := func(_ context.Context, _ string, offset int64) (io.ReadCloser, bool, error) {
		if offset != 4 {
			t.Fatalf("offset=%d, want 4", offset)
		}
		return io.NopCloser(bytes.NewReader(content[offset:])), true, nil
	}
	if err := downloadOne(context.Background(), target, file, fetch); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content=%q, want %q", got, content)
	}
}

func TestSyncRequiresConfiguredServer(t *testing.T) {
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{}, "", nil }
	if _, err := service.Sync(context.Background(), SyncRequest{}); err == nil {
		t.Fatal("expected missing server error")
	}
}
