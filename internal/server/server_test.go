package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
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

func TestServeFromConfig_RejectsRemoteMode(t *testing.T) {
	cfg := config.Config{
		Mode:       "remote",
		ServerAddr: "http://localhost:8021",
	}
	_, err := ServeFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error when serving with remote mode")
	}
}

func TestServeFile_ContentDispositionEscapesQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	specialName := "test paper (2024).pdf"
	targetPath := filepath.Join(tmpDir, specialName)
	if err := os.WriteFile(targetPath, []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &fileMockReader{filePath: targetPath, contentType: "application/pdf"}
	logger := NewLogger(io.Discard, "info")
	mux := http.NewServeMux()
	h := NewHandler(mock)
	h.RegisterRoutes(mux)
	handler := corsMiddleware(requestIDMiddleware(logger)(recoverMiddleware(logger)(mux)))

	req := httptest.NewRequest("GET", "/api/v1/files/TEST1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	disp := rec.Header().Get("Content-Disposition")
	if disp == "" {
		t.Fatal("missing Content-Disposition header")
	}
	if !strings.Contains(disp, specialName) {
		t.Errorf("Content-Disposition should contain filename %q, got: %s", specialName, disp)
	}
}

func TestFormatContentDisposition_EscapesDoubleQuotes(t *testing.T) {
	// Simulate a filename that contains a double quote (e.g. from non-Windows systems)
	disp := formatContentDisposition(`file"name.pdf`)
	if strings.Contains(disp, `file"name`) {
		t.Errorf("unescaped double quote in Content-Disposition: %s", disp)
	}
	if !strings.Contains(disp, `file\"name.pdf`) && !strings.Contains(disp, `file%22name.pdf`) {
		t.Errorf("expected escaped quote, got: %s", disp)
	}
}

func TestFormatContentDisposition_AsciiSafe(t *testing.T) {
	disp := formatContentDisposition("simple.pdf")
	if !strings.Contains(disp, `simple.pdf`) {
		t.Errorf("expected simple filename, got: %s", disp)
	}
}

type fileMockReader struct {
	filePath    string
	contentType string
}

func (m *fileMockReader) FindItems(ctx context.Context, opts backend.FindOptions) ([]domain.Item, error) {
	return nil, nil
}
func (m *fileMockReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return domain.Item{}, backend.ErrItemNotFound
}
func (m *fileMockReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return nil, nil
}
func (m *fileMockReader) GetLibraryStats(ctx context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}
func (m *fileMockReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	return nil, nil
}
func (m *fileMockReader) ListTags(ctx context.Context) ([]backend.Tag, error) {
	return nil, nil
}
func (m *fileMockReader) ListCollections(ctx context.Context) ([]backend.Collection, error) {
	return nil, nil
}
func (m *fileMockReader) GetAttachmentFile(ctx context.Context, key string) (string, string, error) {
	return m.filePath, m.contentType, nil
}
