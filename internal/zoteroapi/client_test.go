package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestClientCreateItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "41" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}

		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body[0]["itemType"] != "book" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "42")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"successful": map[string]any{
				"0": map[string]any{
					"key":     "NEWA1234",
					"version": 42,
				},
			},
			"unchanged": map[string]any{},
			"failed":    map[string]any{},
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

	result, err := client.CreateItem(context.Background(), map[string]any{
		"itemType": "book",
		"title":    "My Book",
	}, 41)
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	if result.Key != "NEWA1234" || result.LastModifiedVersion != 42 {
		t.Fatalf("unexpected create result: %#v", result)
	}
}

func TestClientUpdateItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/items/ABCD2345" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "7" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["title"] != "Updated Title" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "8")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.UpdateItem(context.Background(), "ABCD2345", map[string]any{
		"title": "Updated Title",
	}, 7)
	if err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}
	if result.LastModifiedVersion != 8 {
		t.Fatalf("unexpected update result: %#v", result)
	}
}

func TestClientDeleteItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/items/ABCD2345" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "8" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}
		w.Header().Set("Last-Modified-Version", "9")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.DeleteItem(context.Background(), "ABCD2345", 8)
	if err != nil {
		t.Fatalf("DeleteItem returned error: %v", err)
	}
	if result.LastModifiedVersion != 9 {
		t.Fatalf("unexpected delete result: %#v", result)
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

func TestClientListCollectionsFollowsPaginationAndMapsParentKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/collections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		start, err := strconv.Atoi(defaultString(r.URL.Query().Get("start"), "0"))
		if err != nil {
			t.Fatalf("unexpected start value: %v", err)
		}

		total := 26
		w.Header().Set("Total-Results", strconv.Itoa(total))
		if start == 0 {
			w.Header().Set("Link", `</users/123/collections?start=25>; rel="next"`)
		}

		payload := make([]map[string]any, 0, min(25, total-start))
		end := min(start+25, total)
		for i := start; i < end; i++ {
			entry := map[string]any{
				"key": "COLL" + strconv.Itoa(i),
				"data": map[string]any{
					"name":             "Collection " + strconv.Itoa(i),
					"parentCollection": false,
				},
				"meta": map[string]any{
					"numCollections": 0,
					"numItems":       i,
				},
			}
			if i == 25 {
				entry["data"] = map[string]any{
					"name":             "Collection 25",
					"parentCollection": "COLL1",
				}
			}
			payload = append(payload, entry)
		}

		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	collections, err := client.ListCollections(context.Background())
	if err != nil {
		t.Fatalf("ListCollections returned error: %v", err)
	}

	if len(collections) != 26 {
		t.Fatalf("expected 26 collections after pagination, got %d", len(collections))
	}
	if collections[25].ParentKey != "COLL1" {
		t.Fatalf("unexpected parent key: %#v", collections[25])
	}
}

func TestClientListNotesFollowsPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemType"); got != "note" {
			t.Fatalf("unexpected itemType: %q", got)
		}

		start, err := strconv.Atoi(defaultString(r.URL.Query().Get("start"), "0"))
		if err != nil {
			t.Fatalf("unexpected start value: %v", err)
		}

		total := 26
		w.Header().Set("Total-Results", strconv.Itoa(total))
		if start == 0 {
			w.Header().Set("Link", `</users/123/items?itemType=note&start=25>; rel="next"`)
		}

		payload := make([]map[string]any, 0, min(25, total-start))
		end := min(start+25, total)
		for i := start; i < end; i++ {
			payload = append(payload, map[string]any{
				"key": "NOTE" + strconv.Itoa(i),
				"data": map[string]any{
					"itemType": "note",
					"note":     "<p>Note " + strconv.Itoa(i) + "</p>",
				},
			})
		}

		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	notes, err := client.ListNotes(context.Background())
	if err != nil {
		t.Fatalf("ListNotes returned error: %v", err)
	}

	if len(notes) != 26 {
		t.Fatalf("expected 26 notes after pagination, got %d", len(notes))
	}
	if notes[25].Content != "Note 25" {
		t.Fatalf("unexpected final note: %#v", notes[25])
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

func TestClientListTags(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"tag": "transformers",
				"meta": map[string]any{
					"numItems": 4,
				},
			},
			{
				"tag": "ai",
				"meta": map[string]any{
					"numItems": 2,
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

	tags, err := client.ListTags(context.Background())
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "transformers" || tags[0].NumItems != 4 {
		t.Fatalf("unexpected tag: %#v", tags[0])
	}
}

func TestClientListSearches(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/searches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key": "SCH12345",
				"data": map[string]any{
					"name": "Unread PDFs",
					"conditions": []map[string]any{
						{"condition": "itemType", "operator": "is", "value": "attachment"},
						{"condition": "tag", "operator": "contains", "value": "pdf"},
					},
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

	searches, err := client.ListSearches(context.Background())
	if err != nil {
		t.Fatalf("ListSearches returned error: %v", err)
	}

	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Key != "SCH12345" || searches[0].Name != "Unread PDFs" || searches[0].NumConditions != 2 {
		t.Fatalf("unexpected search: %#v", searches[0])
	}
}

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

func TestClientFindItemsMapsUnauthorizedError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	_, err := client.FindItems(context.Background(), FindOptions{})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if got := err.Error(); got != "zotero api unauthorized (401): check library id and api key" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestClientGetItemMapsNotFoundError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	_, err := client.GetItem(context.Background(), "MISSING")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if got := err.Error(); got != "zotero api not found (404)" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestClientFindItemsIncludesRetryAfterForRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	_, err := client.FindItems(context.Background(), FindOptions{})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if got := err.Error(); got != "zotero api rate limited (429): retry after 7s" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
