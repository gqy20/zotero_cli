package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
)

func TestSyncPullUsesExplicitServerWithoutConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/manifest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"sqlite": []any{}, "storage": []any{}, "fulltext": []any{}}})
	}))
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "mirror")
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{}, "", config.ErrNotFound }
	result, err := service.Pull(context.Background(), SyncPullRequest{ServerAddr: server.URL, DataDir: dir, Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := result.Data.(SyncSummary)
	if !ok || summary.DataDir != dir || !summary.Storage {
		t.Fatalf("unexpected summary: %#v", result.Data)
	}
}

func TestSyncPullRejectsInvalidConcurrency(t *testing.T) {
	service := NewSyncService()
	if _, err := service.Pull(context.Background(), SyncPullRequest{ServerAddr: "http://example.invalid", Concurrency: -1}); err == nil {
		t.Fatal("expected concurrency error")
	}
}
