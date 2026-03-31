package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMachineOnlyNotesAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/123456/items":
			if r.URL.Query().Get("itemType") != "note" {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "NOTE9000",
					"data": map[string]any{
						"itemType": "note",
						"note":     "<p>ITEM1234 {\"readingTime\":88}</p>",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}

func newConditionalVersionsAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123456/items" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("format") != "versions" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("If-Modified-Since-Version"); got != "88" {
			t.Fatalf("unexpected If-Modified-Since-Version: %q", got)
		}

		w.WriteHeader(http.StatusNotModified)
	}))

	return server.URL, server.Close
}
