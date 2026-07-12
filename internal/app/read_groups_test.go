package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func TestGroupsUsesConfiguredUserLibraryID(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/users/123/groups" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	service := NewReadService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{Mode: "web", LibraryType: "user", LibraryID: "123"}, "", nil
	}
	service.NewClient = func(cfg config.Config) (*zoteroapi.Client, error) {
		return zoteroapi.New(cfg, server.URL, server.Client()), nil
	}
	result, err := service.Groups(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if result.Meta["read_source"] != "web" {
		t.Fatalf("meta = %#v", result.Meta)
	}
}
