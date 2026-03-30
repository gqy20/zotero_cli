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

func TestClientGetItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items/X42A7DEE" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Zotero-API-Key"); got != "secret" {
			t.Fatalf("unexpected api key: %q", got)
		}

		item := map[string]any{
			"key": "X42A7DEE",
			"data": map[string]any{
				"itemType":         "conferencePaper",
				"title":            "Attention Is All You Need",
				"date":             "2017",
				"url":              "https://arxiv.org/abs/1706.03762",
				"DOI":              "10.48550/arXiv.1706.03762",
				"proceedingsTitle": "NeurIPS 2017",
				"creators": []map[string]any{
					{
						"creatorType": "author",
						"firstName":   "Ashish",
						"lastName":    "Vaswani",
					},
				},
				"tags": []map[string]any{
					{"tag": "transformers"},
				},
			},
		}

		if err := json.NewEncoder(w).Encode(item); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	item, err := client.GetItem(context.Background(), "X42A7DEE")
	if err != nil {
		t.Fatalf("GetItem returned error: %v", err)
	}

	if item.Key != "X42A7DEE" {
		t.Fatalf("unexpected key: %q", item.Key)
	}
	if item.DOI != "10.48550/arXiv.1706.03762" {
		t.Fatalf("unexpected doi: %q", item.DOI)
	}
	if item.URL != "https://arxiv.org/abs/1706.03762" {
		t.Fatalf("unexpected url: %q", item.URL)
	}
	if len(item.Tags) != 1 || item.Tags[0] != "transformers" {
		t.Fatalf("unexpected tags: %#v", item.Tags)
	}
}
