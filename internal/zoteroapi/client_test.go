package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		Query:     "attention",
		ItemType:  "journalArticle",
		Limit:     5,
		Tag:       "ml",
		Start:     10,
		Sort:      "title",
		Direction: "asc",
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
