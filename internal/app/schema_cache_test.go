package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func newSchemaCacheTestService(t *testing.T, handler http.Handler, now *time.Time) (SchemaService, *int) {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	root := t.TempDir()
	service := NewSchemaService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{Mode: "web", Locale: "en-US"}, filepath.Join(root, ".env"), nil
	}
	service.NewClient = func(cfg config.Config) (*zoteroapi.Client, error) {
		return zoteroapi.New(cfg, server.URL, server.Client()), nil
	}
	service.CacheDir = func(string) string { return filepath.Join(root, "schema") }
	service.Now = func() time.Time { return *now }
	return service, &requests
}

func TestSchemaListCachesAndRefreshes(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	service, requests := newSchemaCacheTestService(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"itemType":"book","localized":"Book"}]`))
	}), &now)

	first, err := service.List(context.Background(), "types", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Meta["read_source"] != "web" || *requests != 1 {
		t.Fatalf("first result meta=%#v requests=%d", first.Meta, *requests)
	}
	second, err := service.List(context.Background(), "types", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.Meta["read_source"] != "cache" || *requests != 1 {
		t.Fatalf("cached result meta=%#v requests=%d", second.Meta, *requests)
	}
	_, err = service.ListWithOptions(context.Background(), "types", "", SchemaOptions{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 2 {
		t.Fatalf("refresh requests=%d, want 2", *requests)
	}
}

func TestSchemaListFallsBackToStaleCache(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	fail := false
	service, _ := newSchemaCacheTestService(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"itemType":"book","localized":"Book"}]`))
	}), &now)
	if _, err := service.List(context.Background(), "types", ""); err != nil {
		t.Fatal(err)
	}
	now = now.Add(schemaCacheTTL + time.Hour)
	fail = true
	result, err := service.List(context.Background(), "types", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta["stale"] != true || len(result.Warnings) != 1 {
		t.Fatalf("expected stale warning, result=%#v", result)
	}
}

func TestSchemaShowUsesIndependentCacheKey(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	service, requests := newSchemaCacheTestService(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"itemType":"book","title":""}`))
	}), &now)
	if _, err := service.Show(context.Background(), "book"); err != nil {
		t.Fatal(err)
	}
	result, err := service.Show(context.Background(), "book")
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 1 || result.Meta["read_source"] != "cache" {
		t.Fatalf("requests=%d meta=%#v", *requests, result.Meta)
	}
}
