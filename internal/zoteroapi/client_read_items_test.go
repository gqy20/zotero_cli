package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
		if got := r.URL.Query().Get("itemType"); got != "journalArticle" {
			t.Fatalf("unexpected itemType: %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("unexpected limit: %q", got)
		}
		if got := r.URL.Query().Get("tag"); got != "ml" {
			t.Fatalf("unexpected tag: %q", got)
		}
		if got := r.URL.Query().Get("start"); got != "10" {
			t.Fatalf("unexpected start: %q", got)
		}
		if got := r.URL.Query().Get("sort"); got != "title" {
			t.Fatalf("unexpected sort: %q", got)
		}
		if got := r.URL.Query().Get("direction"); got != "asc" {
			t.Fatalf("unexpected direction: %q", got)
		}
		if got := r.URL.Query().Get("qmode"); got != "everything" {
			t.Fatalf("unexpected qmode: %q", got)
		}
		if got := r.URL.Query().Get("includeTrashed"); got != "1" {
			t.Fatalf("unexpected includeTrashed: %q", got)
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
		Query:          "attention",
		ItemType:       "journalArticle",
		Limit:          5,
		Tag:            "ml",
		Start:          10,
		Sort:           "title",
		Direction:      "asc",
		QMode:          "everything",
		IncludeTrashed: true,
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

func TestClientListTrashItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items/trash" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key": "TRASH123",
				"data": map[string]any{
					"itemType": "journalArticle",
					"title":    "Removed Paper",
					"date":     "2022",
				},
			},
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

	items, err := client.ListTrashItems(context.Background(), FindOptions{})
	if err != nil {
		t.Fatalf("ListTrashItems returned error: %v", err)
	}

	if len(items) != 1 || items[0].Key != "TRASH123" {
		t.Fatalf("unexpected trash items: %#v", items)
	}
}

func TestClientListTopCollections(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/collections/top" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key": "COLLTOP1",
				"data": map[string]any{
					"name":             "Top Folder",
					"parentCollection": false,
				},
				"meta": map[string]any{
					"numCollections": 1,
					"numItems":       8,
				},
			},
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

	collections, err := client.ListTopCollections(context.Background())
	if err != nil {
		t.Fatalf("ListTopCollections returned error: %v", err)
	}

	if len(collections) != 1 || collections[0].Key != "COLLTOP1" {
		t.Fatalf("unexpected collections: %#v", collections)
	}
}

func TestClientListPublicationsItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/publications/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key": "PUB12345",
				"data": map[string]any{
					"itemType": "journalArticle",
					"title":    "Published Article",
					"date":     "2020",
				},
			},
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

	items, err := client.ListPublicationsItems(context.Background(), FindOptions{})
	if err != nil {
		t.Fatalf("ListPublicationsItems returned error: %v", err)
	}

	if len(items) != 1 || items[0].Key != "PUB12345" {
		t.Fatalf("unexpected publications items: %#v", items)
	}
}

func TestClientExportItemsBibTeX(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemKey"); got != "X42A7DEE,ART12345" {
			t.Fatalf("unexpected itemKey: %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "bibtex" {
			t.Fatalf("unexpected format: %q", got)
		}

		_, _ = w.Write([]byte("@article{vaswani2017,\n  title = {Attention Is All You Need}\n}"))
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.ExportItems(context.Background(), []string{"X42A7DEE", "ART12345"}, ExportOptions{
		Format: "bibtex",
	})
	if err != nil {
		t.Fatalf("ExportItems returned error: %v", err)
	}

	if result.Format != "bibtex" || !strings.Contains(result.Text, "@article{vaswani2017") {
		t.Fatalf("unexpected export result: %#v", result)
	}
}

func TestClientExportItemsCSLJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("format"); got != "csljson" {
			t.Fatalf("unexpected format: %q", got)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{"id": "X42A7DEE", "title": "Attention Is All You Need"},
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

	result, err := client.ExportItems(context.Background(), []string{"X42A7DEE"}, ExportOptions{
		Format: "csljson",
	})
	if err != nil {
		t.Fatalf("ExportItems returned error: %v", err)
	}

	payload, ok := result.Data.([]any)
	if !ok || len(payload) != 1 {
		t.Fatalf("unexpected export payload: %#v", result)
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

func TestClientFindItemsFollowsPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		start, err := strconv.Atoi(defaultString(r.URL.Query().Get("start"), "0"))
		if err != nil {
			t.Fatalf("unexpected start value: %v", err)
		}

		total := 30
		w.Header().Set("Total-Results", strconv.Itoa(total))
		if start == 0 {
			w.Header().Set("Link", `</users/123/items?start=25>; rel="next"`)
		}

		items := make([]map[string]any, 0, min(25, total-start))
		end := min(start+25, total)
		for i := start; i < end; i++ {
			items = append(items, map[string]any{
				"key": "ITEM" + strconv.Itoa(i),
				"data": map[string]any{
					"itemType": "journalArticle",
					"title":    "Item " + strconv.Itoa(i),
				},
			})
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

	items, err := client.FindItems(context.Background(), FindOptions{})
	if err != nil {
		t.Fatalf("FindItems returned error: %v", err)
	}

	if len(items) != 30 {
		t.Fatalf("expected 30 items after pagination, got %d", len(items))
	}
	if items[29].Key != "ITEM29" {
		t.Fatalf("unexpected final item: %#v", items[29])
	}
}
