package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOverviewEndpoint(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/overview", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	dataRaw, ok := resp["data"]
	if !ok {
		t.Fatal("missing 'data' field")
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}
	if _, ok := data["stats"]; !ok {
		t.Fatal("missing 'stats' field in overview data")
	}
	if _, ok := data["recent_items"]; !ok {
		t.Fatal("missing 'recent_items' field in overview data")
	}
}

func TestItemsWithQueryParams(t *testing.T) {
	srv := NewMockServerWithReader()

	tests := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{"with type filter", "/api/v1/items?item_type=journalArticle", http.StatusOK},
		{"with tag filter", "/api/v1/items?tag=important", http.StatusOK},
		{"with collection", "/api/v1/items?collection=ABC123", http.StatusOK},
		{"with date range", "/api/v1/items?date_after=2024-01-01&date_before=2024-12-31", http.StatusOK},
		{"with pagination", "/api/v1/items?limit=10&start=5", http.StatusOK},
		{"with sort", "/api/v1/items?sort=title&direction=asc", http.StatusOK},
		{"with has_pdf", "/api/v1/items?has_pdf=true", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
