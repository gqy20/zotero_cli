package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/config"
)

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
		if r.Method != http.MethodPut {
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

func TestClientCreateCollection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/collections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "10" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}

		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body[0]["name"] != "New Collection" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "11")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"successful": map[string]any{
				"0": map[string]any{
					"key":     "COLLNEW1",
					"version": 11,
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

	result, err := client.CreateCollection(context.Background(), map[string]any{
		"name": "New Collection",
	}, 10)
	if err != nil {
		t.Fatalf("CreateCollection returned error: %v", err)
	}
	if result.Key != "COLLNEW1" || result.LastModifiedVersion != 11 {
		t.Fatalf("unexpected create result: %#v", result)
	}
}

func TestClientUpdateCollection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/collections/COLL1234" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["name"] != "Renamed Collection" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "12")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key":     "COLL1234",
			"version": 12,
			"name":    "Renamed Collection",
		})
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.UpdateCollection(context.Background(), "COLL1234", map[string]any{
		"key":     "COLL1234",
		"version": 11,
		"name":    "Renamed Collection",
	}, 0)
	if err != nil {
		t.Fatalf("UpdateCollection returned error: %v", err)
	}
	if result.Key != "COLL1234" || result.LastModifiedVersion != 12 {
		t.Fatalf("unexpected update result: %#v", result)
	}
}

func TestClientDeleteCollection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/collections/COLL1234" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "12" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}
		w.Header().Set("Last-Modified-Version", "13")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.DeleteCollection(context.Background(), "COLL1234", 12)
	if err != nil {
		t.Fatalf("DeleteCollection returned error: %v", err)
	}
	if result.Key != "COLL1234" || result.LastModifiedVersion != 13 {
		t.Fatalf("unexpected delete result: %#v", result)
	}
}

func TestClientCreateSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/searches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "17" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}

		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body[0]["name"] != "Unread PDFs" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "48")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"successful": map[string]any{
				"0": map[string]any{
					"key":     "SCH67890",
					"version": 48,
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

	result, err := client.CreateSearch(context.Background(), map[string]any{
		"name": "Unread PDFs",
		"conditions": []map[string]any{
			{"condition": "itemType", "operator": "is", "value": "attachment"},
		},
	}, 17)
	if err != nil {
		t.Fatalf("CreateSearch returned error: %v", err)
	}
	if result.Key != "SCH67890" || result.LastModifiedVersion != 48 {
		t.Fatalf("unexpected create result: %#v", result)
	}
}

func TestClientUpdateSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/searches/SCH12345" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["name"] != "Important PDFs" {
			t.Fatalf("unexpected request body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "49")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key":     "SCH12345",
			"version": 49,
			"name":    "Important PDFs",
		})
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.UpdateSearch(context.Background(), "SCH12345", map[string]any{
		"key":     "SCH12345",
		"version": 21,
		"name":    "Important PDFs",
	}, 0)
	if err != nil {
		t.Fatalf("UpdateSearch returned error: %v", err)
	}
	if result.Key != "SCH12345" || result.LastModifiedVersion != 49 {
		t.Fatalf("unexpected update result: %#v", result)
	}
}

func TestClientDeleteSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/searches/SCH12345" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "22" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}
		w.Header().Set("Last-Modified-Version", "50")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.DeleteSearch(context.Background(), "SCH12345", 22)
	if err != nil {
		t.Fatalf("DeleteSearch returned error: %v", err)
	}
	if result.Key != "SCH12345" || result.LastModifiedVersion != 50 {
		t.Fatalf("unexpected delete result: %#v", result)
	}
}

func TestClientCreateItems(t *testing.T) {
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
		if len(body) != 2 {
			t.Fatalf("expected 2 items, got %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "52")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"successful": map[string]any{
				"0": map[string]any{"key": "ITEMA001", "version": 51},
				"1": map[string]any{"key": "ITEMA002", "version": 52},
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

	result, err := client.CreateItems(context.Background(), []map[string]any{
		{"itemType": "book", "title": "Book One"},
		{"itemType": "book", "title": "Book Two"},
	}, 41)
	if err != nil {
		t.Fatalf("CreateItems returned error: %v", err)
	}
	if result.LastModifiedVersion != 52 || len(result.Successful) != 2 || result.Successful[0].Key != "ITEMA001" || result.Successful[1].Key != "ITEMA002" {
		t.Fatalf("unexpected batch result: %#v", result)
	}
}

func TestClientUpdateItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 2 || body[0]["key"] != "ITEMA001" {
			t.Fatalf("unexpected body: %#v", body)
		}

		w.Header().Set("Last-Modified-Version", "53")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"successful": map[string]any{
				"0": map[string]any{"key": "ITEMA001", "version": 53},
			},
			"unchanged": map[string]any{
				"1": 52,
			},
			"failed": map[string]any{},
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

	result, err := client.UpdateItems(context.Background(), []map[string]any{
		{"key": "ITEMA001", "version": 52, "title": "Updated One"},
		{"key": "ITEMA002", "version": 52, "title": "Updated Two"},
	}, 0)
	if err != nil {
		t.Fatalf("UpdateItems returned error: %v", err)
	}
	if result.LastModifiedVersion != 53 || len(result.Successful) != 1 || result.Successful[0].Key != "ITEMA001" || len(result.Unchanged) != 1 || result.Unchanged[0] != "1" {
		t.Fatalf("unexpected batch result: %#v", result)
	}
}

func TestClientDeleteItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemKey"); got != "ITEMA001,ITEMA002" {
			t.Fatalf("unexpected itemKey: %q", got)
		}
		if got := r.Header.Get("If-Unmodified-Since-Version"); got != "53" {
			t.Fatalf("unexpected If-Unmodified-Since-Version: %q", got)
		}
		w.Header().Set("Last-Modified-Version", "54")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	result, err := client.DeleteItems(context.Background(), []string{"ITEMA001", "ITEMA002"}, 53)
	if err != nil {
		t.Fatalf("DeleteItems returned error: %v", err)
	}
	if result.LastModifiedVersion != 54 || len(result.Successful) != 2 || result.Successful[1].Key != "ITEMA002" {
		t.Fatalf("unexpected delete result: %#v", result)
	}
}

func TestClientGetItemsByKeys(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("itemKey"); got != "ITEMA001,ITEMA002" {
			t.Fatalf("unexpected itemKey: %q", got)
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key":     "ITEMA001",
				"version": 52,
				"data": map[string]any{
					"itemType": "book",
					"title":    "Book One",
					"tags": []map[string]any{
						{"tag": "ai"},
					},
				},
			},
			{
				"key":     "ITEMA002",
				"version": 53,
				"data": map[string]any{
					"itemType": "book",
					"title":    "Book Two",
					"tags": []map[string]any{
						{"tag": "ml"},
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

	items, err := client.GetItemsByKeys(context.Background(), []string{"ITEMA001", "ITEMA002"})
	if err != nil {
		t.Fatalf("GetItemsByKeys returned error: %v", err)
	}
	if len(items) != 2 || items[0].Version != 52 || len(items[1].Tags) != 1 || items[1].Tags[0] != "ml" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
