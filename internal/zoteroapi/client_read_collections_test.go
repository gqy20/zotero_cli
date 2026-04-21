package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"zotero_cli/internal/config"
)

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
