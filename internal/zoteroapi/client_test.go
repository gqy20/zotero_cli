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
		if got := r.URL.Query().Get("itemType"); got != "journalArticle" {
			t.Fatalf("unexpected itemType: %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("unexpected limit: %q", got)
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

	items, err := client.FindItems(context.Background(), FindOptions{
		Query:    "attention",
		ItemType: "journalArticle",
		Limit:    5,
	})
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
		if got := r.Header.Get("Zotero-API-Key"); got != "secret" {
			t.Fatalf("unexpected api key: %q", got)
		}

		switch r.URL.Path {
		case "/users/123/items/X42A7DEE":
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
		case "/users/123/items/X42A7DEE/children":
			children := []map[string]any{
				{
					"key": "PDF12345",
					"data": map[string]any{
						"itemType":    "attachment",
						"title":       "Attention Is All You Need PDF",
						"contentType": "application/pdf",
						"linkMode":    "imported_file",
						"filename":    "attention-is-all-you-need.pdf",
					},
				},
			}
			if err := json.NewEncoder(w).Encode(children); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
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
	if len(item.Attachments) != 1 {
		t.Fatalf("unexpected attachments: %#v", item.Attachments)
	}
	if item.Attachments[0].Key != "PDF12345" {
		t.Fatalf("unexpected attachment key: %q", item.Attachments[0].Key)
	}
	if item.Attachments[0].ContentType != "application/pdf" {
		t.Fatalf("unexpected content type: %q", item.Attachments[0].ContentType)
	}
}

func TestClientGetCitation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Zotero-API-Key"); got != "secret" {
			t.Fatalf("unexpected api key: %q", got)
		}
		if r.URL.Path != "/users/123/items/X42A7DEE" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != "citation" {
			t.Fatalf("unexpected include: %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "json" {
			t.Fatalf("unexpected format: %q", got)
		}
		if got := r.URL.Query().Get("style"); got != "apa" {
			t.Fatalf("unexpected style: %q", got)
		}
		if got := r.URL.Query().Get("locale"); got != "en-US" {
			t.Fatalf("unexpected locale: %q", got)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"key":      "X42A7DEE",
			"citation": "<span>(Vaswani, 2017)</span>",
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

	got, err := client.GetCitation(context.Background(), "X42A7DEE", CitationOptions{
		Format: "citation",
		Style:  "apa",
		Locale: "en-US",
	})
	if err != nil {
		t.Fatalf("GetCitation returned error: %v", err)
	}

	if got.Key != "X42A7DEE" {
		t.Fatalf("unexpected key: %q", got.Key)
	}
	if got.Text != "(Vaswani, 2017)" {
		t.Fatalf("unexpected text: %q", got.Text)
	}
	if got.HTML != "<span>(Vaswani, 2017)</span>" {
		t.Fatalf("unexpected html: %q", got.HTML)
	}
}

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
