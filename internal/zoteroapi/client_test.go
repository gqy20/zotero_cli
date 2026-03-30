package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/config"
)

func TestClientFindItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Zotero-API-Key"); got != "secret" {
			t.Fatalf("unexpected api key: %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "attention" {
			t.Fatalf("unexpected query: %q", got)
		}

		items := []map[string]any{
			{
				"key": "X42A7DEE",
				"data": map[string]any{
					"itemType": "conferencePaper",
					"title":    "Attention Is All You Need",
					"date":     "2017",
					"creators": []map[string]any{
						{
							"creatorType": "author",
							"firstName":   "Ashish",
							"lastName":    "Vaswani",
						},
					},
				},
			},
		}

		if err := json.NewEncoder(w).Encode(items); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	items, err := client.FindItems(context.Background(), "attention")
	if err != nil {
		t.Fatalf("FindItems returned error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].Key != "X42A7DEE" {
		t.Fatalf("unexpected key: %q", items[0].Key)
	}

	if items[0].Title != "Attention Is All You Need" {
		t.Fatalf("unexpected title: %q", items[0].Title)
	}

	if len(items[0].Creators) != 1 || items[0].Creators[0].Name != "Ashish Vaswani" {
		t.Fatalf("unexpected creators: %+v", items[0].Creators)
	}
}
