package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zotero_cli/internal/config"
)

func TestNewUsesConfiguredTimeoutForDefaultClient(t *testing.T) {
	t.Parallel()

	client := New(config.Config{
		LibraryType:    "user",
		LibraryID:      "123",
		APIKey:         "secret",
		TimeoutSeconds: 7,
	}, "", nil)

	if client.httpClient == nil {
		t.Fatal("expected http client to be initialized")
	}
	if got := client.httpClient.Timeout; got != 7*time.Second {
		t.Fatalf("expected timeout 7s, got %s", got)
	}
}

func TestClientRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()

	client := New(config.Config{
		Mode:        "local",
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, "https://example.com", &http.Client{})

	_, err := client.FindItems(context.Background(), FindOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
	if got := err.Error(); got != `unsupported mode "local"` {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestClientUsesGroupLibraryPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups/456/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "group",
		LibraryID:   "456",
		APIKey:      "secret",
	}, server.URL, server.Client())

	if _, err := client.FindItems(context.Background(), FindOptions{}); err != nil {
		t.Fatalf("FindItems returned error: %v", err)
	}
}

func TestClientGetKeyInfo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys/secret" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"userID": 123456,
			"access": map[string]any{
				"user": map[string]any{
					"library": true,
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	info, err := client.GetKeyInfo(context.Background(), "secret")
	if err != nil {
		t.Fatalf("GetKeyInfo returned error: %v", err)
	}

	if info.UserID != 123456 {
		t.Fatalf("unexpected key info: %#v", info)
	}
	if info.Access["user"] == nil {
		t.Fatalf("expected access payload: %#v", info)
	}
}

func TestClientListGroupsForUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123456/groups" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": 111,
				"data": map[string]any{
					"name": "Research Lab",
				},
			},
			{
				"id": 222,
				"data": map[string]any{
					"name": "Paper Club",
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{}, server.URL, server.Client())

	groups, err := client.ListGroupsForUser(context.Background(), "123456")
	if err != nil {
		t.Fatalf("ListGroupsForUser returned error: %v", err)
	}

	if len(groups) != 2 || groups[0].ID != 111 || groups[0].Name != "Research Lab" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
}

func TestClientValidateLibraryAccessForUser(t *testing.T) {
	t.Parallel()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123456",
		APIKey:      "secret",
	}, "", nil)
	client.baseURL = "http://example.test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys/secret" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"userID": 123456,
			"access": map[string]any{"user": map[string]any{"library": true}},
		})
	}))
	defer server.Close()
	client.baseURL = server.URL
	client.httpClient = server.Client()

	result, err := client.ValidateLibraryAccess(context.Background())
	if err != nil {
		t.Fatalf("ValidateLibraryAccess returned error: %v", err)
	}
	if result.KeyUserID != 123456 || result.LibraryID != "123456" {
		t.Fatalf("unexpected validation result: %#v", result)
	}
}

func TestClientValidateLibraryAccessForGroup(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/keys/secret":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"userID": 123456,
				"access": map[string]any{"user": map[string]any{"library": true}},
			})
		case "/users/123456/groups":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 999, "data": map[string]any{"name": "Other"}},
				{"id": 222, "data": map[string]any{"name": "Team"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "group",
		LibraryID:   "222",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.ValidateLibraryAccess(context.Background())
	if err != nil {
		t.Fatalf("ValidateLibraryAccess returned error: %v", err)
	}
	if !result.GroupFound || result.LibraryID != "222" {
		t.Fatalf("unexpected validation result: %#v", result)
	}
}

func TestClientGetLibraryStats(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/123/items":
			if r.URL.Query().Get("format") != "versions" || r.URL.Query().Get("since") != "0" {
				t.Fatalf("unexpected item stats query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Last-Modified-Version", "111")
			_ = json.NewEncoder(w).Encode(map[string]int{
				"ITEM1234": 90,
				"ITEM5678": 91,
			})
		case "/users/123/collections":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"key": "COLL1234", "data": map[string]any{"name": "Papers"}},
			})
		case "/users/123/searches":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"key": "SCH12345", "data": map[string]any{"name": "Unread PDFs"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	stats, err := client.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryStats returned error: %v", err)
	}
	if stats.TotalItems != 2 || stats.TotalCollections != 1 || stats.TotalSearches != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}
