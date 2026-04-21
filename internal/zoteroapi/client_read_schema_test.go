package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/config"
)

func TestClientGetDeleted(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/deleted" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"collections": []string{"COLL1234"},
			"searches":    []string{"SCH12345"},
			"items":       []string{"ITEM1234", "ITEM5678"},
			"tags":        []string{"obsolete"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	deleted, err := client.GetDeleted(context.Background())
	if err != nil {
		t.Fatalf("GetDeleted returned error: %v", err)
	}

	if len(deleted.Items) != 2 || deleted.Items[1] != "ITEM5678" {
		t.Fatalf("unexpected deleted items: %#v", deleted.Items)
	}
	if len(deleted.Tags) != 1 || deleted.Tags[0] != "obsolete" {
		t.Fatalf("unexpected deleted tags: %#v", deleted.Tags)
	}
}

func TestClientGetVersionsForItemsTopWithTrashed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items/top" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("since"); got != "42" {
			t.Fatalf("unexpected since: %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "versions" {
			t.Fatalf("unexpected format: %q", got)
		}
		if got := r.URL.Query().Get("includeTrashed"); got != "1" {
			t.Fatalf("unexpected includeTrashed: %q", got)
		}

		if err := json.NewEncoder(w).Encode(map[string]int{
			"ITEM1234": 100,
			"ITEM5678": 101,
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	versions, err := client.GetVersions(context.Background(), VersionsOptions{
		ObjectType:     "items-top",
		Since:          42,
		IncludeTrashed: true,
	})
	if err != nil {
		t.Fatalf("GetVersions returned error: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions["ITEM5678"] != 101 {
		t.Fatalf("unexpected versions map: %#v", versions)
	}
}

func TestClientGetVersionsForCollections(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/collections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("since"); got != "7" {
			t.Fatalf("unexpected since: %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "versions" {
			t.Fatalf("unexpected format: %q", got)
		}
		if got := r.URL.Query().Get("includeTrashed"); got != "" {
			t.Fatalf("unexpected includeTrashed: %q", got)
		}

		if err := json.NewEncoder(w).Encode(map[string]int{
			"COLL1234": 9,
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	versions, err := client.GetVersions(context.Background(), VersionsOptions{
		ObjectType: "collections",
		Since:      7,
	})
	if err != nil {
		t.Fatalf("GetVersions returned error: %v", err)
	}

	if versions["COLL1234"] != 9 {
		t.Fatalf("unexpected versions map: %#v", versions)
	}
}

func TestClientGetVersionsIncludesLastModifiedVersion(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified-Version", "99")
		if err := json.NewEncoder(w).Encode(map[string]int{
			"ITEM1234": 100,
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.GetVersionsResult(context.Background(), VersionsOptions{
		ObjectType: "items",
		Since:      0,
	})
	if err != nil {
		t.Fatalf("GetVersionsResult returned error: %v", err)
	}

	if result.LastModifiedVersion != 99 {
		t.Fatalf("unexpected last modified version: %#v", result)
	}
	if result.NotModified {
		t.Fatalf("expected not modified to be false: %#v", result)
	}
}

func TestClientGetVersionsReturnsNotModifiedOn304(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-Modified-Since-Version"); got != "88" {
			t.Fatalf("unexpected If-Modified-Since-Version: %q", got)
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.GetVersionsResult(context.Background(), VersionsOptions{
		ObjectType:             "items",
		Since:                  0,
		IfModifiedSinceVersion: 88,
	})
	if err != nil {
		t.Fatalf("GetVersionsResult returned error: %v", err)
	}

	if !result.NotModified {
		t.Fatalf("expected not modified result: %#v", result)
	}
	if len(result.Versions) != 0 {
		t.Fatalf("expected empty versions map: %#v", result)
	}
}

func TestClientListItemTypes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/itemTypes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("locale"); got != "en-US" {
			t.Fatalf("unexpected locale: %q", got)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"itemType": "book", "localized": "Book"},
			{"itemType": "note", "localized": "Note"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	types, err := client.ListItemTypes(context.Background(), "en-US")
	if err != nil {
		t.Fatalf("ListItemTypes returned error: %v", err)
	}

	if len(types) != 2 || types[0].ID != "book" || types[0].Localized != "Book" {
		t.Fatalf("unexpected item types: %#v", types)
	}
}

func TestClientListItemFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/itemFields" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"field": "title", "localized": "Title"},
			{"field": "url", "localized": "URL"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	fields, err := client.ListItemFields(context.Background(), "")
	if err != nil {
		t.Fatalf("ListItemFields returned error: %v", err)
	}

	if len(fields) != 2 || fields[1].ID != "url" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestClientListCreatorFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/creatorFields" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"field": "firstName", "localized": "First"},
			{"field": "lastName", "localized": "Last"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	fields, err := client.ListCreatorFields(context.Background(), "")
	if err != nil {
		t.Fatalf("ListCreatorFields returned error: %v", err)
	}

	if len(fields) != 2 || fields[0].Localized != "First" {
		t.Fatalf("unexpected creator fields: %#v", fields)
	}
}

func TestClientListItemTypeFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/itemTypeFields" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemType"); got != "book" {
			t.Fatalf("unexpected itemType: %q", got)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"field": "title", "localized": "Title"},
			{"field": "abstractNote", "localized": "Abstract"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	fields, err := client.ListItemTypeFields(context.Background(), "book", "")
	if err != nil {
		t.Fatalf("ListItemTypeFields returned error: %v", err)
	}

	if len(fields) != 2 || fields[1].ID != "abstractNote" {
		t.Fatalf("unexpected item type fields: %#v", fields)
	}
}

func TestClientListItemTypeCreatorTypes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/itemTypeCreatorTypes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemType"); got != "book" {
			t.Fatalf("unexpected itemType: %q", got)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"creatorType": "author", "localized": "Author"},
			{"creatorType": "editor", "localized": "Editor"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	types, err := client.ListItemTypeCreatorTypes(context.Background(), "book", "")
	if err != nil {
		t.Fatalf("ListItemTypeCreatorTypes returned error: %v", err)
	}

	if len(types) != 2 || types[0].ID != "author" {
		t.Fatalf("unexpected creator types: %#v", types)
	}
}

func TestClientGetItemTemplate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items/new" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemType"); got != "book" {
			t.Fatalf("unexpected itemType: %q", got)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"itemType": "book",
			"title":    "",
			"creators": []map[string]any{
				{"creatorType": "author", "firstName": "", "lastName": ""},
			},
			"tags":        []any{},
			"collections": []any{},
			"relations":   map[string]any{},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	template, err := client.GetItemTemplate(context.Background(), "book")
	if err != nil {
		t.Fatalf("GetItemTemplate returned error: %v", err)
	}

	if itemType, _ := template["itemType"].(string); itemType != "book" {
		t.Fatalf("unexpected template: %#v", template)
	}
}
