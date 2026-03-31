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

func TestClientFindItemsRetriesOnRateLimit(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"key": "ITEM1234",
				"data": map[string]any{
					"itemType": "book",
					"title":    "Retried",
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType:                "user",
		LibraryID:                  "123",
		APIKey:                     "secret",
		RetryMaxAttempts:           2,
		RetryBaseDelayMilliseconds: 1,
	}, server.URL, server.Client())
	client.sleep = func(time.Duration) {}

	items, err := client.FindItems(context.Background(), FindOptions{})
	if err != nil {
		t.Fatalf("FindItems returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if len(items) != 1 || items[0].Key != "ITEM1234" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestClientCreateItemDoesNotRetryWrites(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType:                "user",
		LibraryID:                  "123",
		APIKey:                     "secret",
		RetryMaxAttempts:           3,
		RetryBaseDelayMilliseconds: 1,
	}, server.URL, server.Client())
	client.sleep = func(time.Duration) {}

	_, err := client.CreateItem(context.Background(), map[string]any{"itemType": "book"}, 41)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for write request, got %d", attempts)
	}
}

func TestClientUpdateItemReturnsReadablePreconditionError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "library version advanced to 88", http.StatusPreconditionFailed)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	_, err := client.UpdateItem(context.Background(), "ABCD2345", map[string]any{"title": "Updated"}, 7)
	if err == nil {
		t.Fatal("expected precondition error")
	}
	if got := err.Error(); got != "zotero api precondition failed (412): library version changed; refresh and retry: library version advanced to 88" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestClientCreateItemReturnsReadableConflictError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "collection key already exists", http.StatusConflict)
	}))
	defer server.Close()

	client := New(config.Config{
		LibraryType: "user",
		LibraryID:   "123",
		APIKey:      "secret",
	}, server.URL, server.Client())

	_, err := client.CreateItem(context.Background(), map[string]any{"itemType": "book"}, 41)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if got := err.Error(); got != "zotero api conflict (409): request conflicts with existing data: collection key already exists" {
		t.Fatalf("unexpected error: %q", got)
	}
}
