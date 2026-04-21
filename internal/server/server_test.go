package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zotero_cli/internal/domain"
)

func TestServerHealthCheck(t *testing.T) {
	srv := NewMockServer()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", data["status"])
	}
}

func TestServerCORSHeaders(t *testing.T) {
	srv := NewMockServer()

	req := httptest.NewRequest("OPTIONS", "/api/v1/stats", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rec.Code)
	}

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Fatalf("expected CORS origin *, got %s", origin)
	}
}

func TestServerNotFound(t *testing.T) {
	srv := NewMockServer()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONResponse[LibraryStats]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true, got error: %s", resp.Error)
	}
	if resp.Data.TotalItems != 0 {
		t.Fatalf("expected 0 items in mock, got %d", resp.Data.TotalItems)
	}
}

func TestCollectionsEndpoint(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/collections", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONResponse[[]Collection]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

func TestTagsEndpoint(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/tags", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONResponse[[]Tag]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

func TestItemsEndpointBasic(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/items?limit=5&start=0", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONResponse[[]domain.Item]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true, error: %s", resp.Error)
	}
}

func TestItemDetailEndpoint(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/items/ABC123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Mock reader returns ErrItemNotFound for any key
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnifiedResponseFormat(t *testing.T) {
	srv := NewMockServerWithReader()

	endpoints := []string{
		"/api/v1/stats",
		"/api/v1/collections",
		"/api/v1/tags",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest("GET", ep, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Errorf("%s: failed to parse JSON: %v", ep, err)
			continue
		}
		if _, ok := raw["ok"]; !ok {
			t.Errorf("%s: missing 'ok' field", ep)
		}
		if _, ok := raw["data"]; !ok {
			t.Errorf("%s: missing 'data' field", ep)
		}
		if _, ok := raw["error"]; !ok {
			t.Errorf("%s: missing 'error' field", ep)
		}
		if _, ok := raw["meta"]; !ok {
			t.Errorf("%s: missing 'meta' field", ep)
		}
	}
}
