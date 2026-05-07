package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

func TestRemoteReader_FindItems(t *testing.T) {
	expected := []domain.Item{
		{Key: "ABC123", ItemType: "journalArticle", Title: "CRISPR Study"},
		{Key: "DEF456", ItemType: "journalArticle", Title: "Another Paper"},
	}
	srv := httptest.NewServer(newRemoteTestHandler(t, "findItems", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	items, err := r.FindItems(context.Background(), FindOptions{Query: "CRISPR"})
	if err != nil {
		t.Fatalf("FindItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Key != "ABC123" {
		t.Errorf("expected key ABC123, got %s", items[0].Key)
	}
}

func TestRemoteReader_GetItem(t *testing.T) {
	expected := domain.Item{Key: "ABC123", ItemType: "journalArticle", Title: "Test Paper"}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getItem", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	item, err := r.GetItem(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if item.Key != "ABC123" {
		t.Errorf("expected key ABC123, got %s", item.Key)
	}
}

func TestRemoteReader_GetRelated(t *testing.T) {
	expected := []domain.Relation{
		{Predicate: "dc:relation", Direction: "outgoing", Target: domain.ItemRef{Key: "XYZ789"}},
	}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getRelated", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	relations, err := r.GetRelated(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("GetRelated: %v", err)
	}
	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}
	if relations[0].Target.Key != "XYZ789" {
		t.Errorf("expected target key XYZ789, got %s", relations[0].Target.Key)
	}
}

func TestRemoteReader_GetLibraryStats(t *testing.T) {
	expected := LibraryStats{
		LibraryType:      "user",
		LibraryID:        "12345",
		TotalItems:       100,
		TotalCollections: 10,
		TotalSearches:    5,
	}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getStats", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	stats, err := r.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryStats: %v", err)
	}
	if stats.TotalItems != 100 {
		t.Errorf("expected 100 items, got %d", stats.TotalItems)
	}
}

func TestRemoteReader_ListTags(t *testing.T) {
	expected := []Tag{{Name: "important", NumItems: 5}, {Name: "review", NumItems: 3}}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getTags", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	tags, err := r.ListTags(context.Background())
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "important" {
		t.Errorf("expected tag 'important', got %s", tags[0].Name)
	}
}

func TestRemoteReader_ListCollections(t *testing.T) {
	expected := []Collection{{Key: "COL1", Name: "Papers"}, {Key: "COL2", Name: "Books"}}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getCollections", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	collections, err := r.ListCollections(context.Background())
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(collections))
	}
	if collections[0].Name != "Papers" {
		t.Errorf("expected collection 'Papers', got %s", collections[0].Name)
	}
}

func TestRemoteReader_ListNotes(t *testing.T) {
	expected := []domain.Note{
		{Key: "NOTE1", ParentItemKey: "ABC123", Content: "<p>Note text</p>"},
	}
	srv := httptest.NewServer(newRemoteTestHandler(t, "getNotes", expected))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	notes, err := r.ListNotes(context.Background())
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Key != "NOTE1" {
		t.Errorf("expected note key NOTE1, got %s", notes[0].Key)
	}
}

func TestRemoteReader_GetAttachmentFile(t *testing.T) {
	content := []byte("%PDF-1.4 test content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/files/ATT1" {
			t.Errorf("expected path /api/v1/files/ATT1, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="test.pdf"`)
		w.Write(content)
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	filePath, contentType, err := r.GetAttachmentFile(context.Background(), "ATT1")
	if err != nil {
		t.Fatalf("GetAttachmentFile: %v", err)
	}
	defer os.Remove(filePath)

	if contentType != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", contentType)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != string(content) {
		t.Errorf("file content mismatch")
	}
}

func TestRemoteReader_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "internal server error",
		})
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, err := r.GetLibraryStats(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// remoteTestHandler routes requests to the appropriate endpoint
// and returns the provided data as a standard JSON response.
func newRemoteTestHandler(t *testing.T, endpoint string, data any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "findItems":
			if r.URL.Path != "/api/v1/items" {
				t.Errorf("expected /api/v1/items, got %s", r.URL.Path)
			}
		case "getItem":
			expected := "/api/v1/items/ABC123"
			if r.URL.Path != expected {
				t.Errorf("expected %s, got %s", expected, r.URL.Path)
			}
		case "getRelated":
			expected := "/api/v1/items/ABC123/related"
			if r.URL.Path != expected {
				t.Errorf("expected %s, got %s", expected, r.URL.Path)
			}
		case "getStats":
			if r.URL.Path != "/api/v1/stats" {
				t.Errorf("expected /api/v1/stats, got %s", r.URL.Path)
			}
		case "getTags":
			if r.URL.Path != "/api/v1/tags" {
				t.Errorf("expected /api/v1/tags, got %s", r.URL.Path)
			}
		case "getCollections":
			if r.URL.Path != "/api/v1/collections" {
				t.Errorf("expected /api/v1/collections, got %s", r.URL.Path)
			}
		case "getNotes":
			if r.URL.Path != "/api/v1/notes" {
				t.Errorf("expected /api/v1/notes, got %s", r.URL.Path)
			}
		default:
			t.Errorf("unhandled endpoint: %s", endpoint)
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"ok":   true,
			"data": data,
			"meta": map[string]any{},
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func TestNewReader_RemoteMode(t *testing.T) {
	cfg := testRemoteConfig()
	cfg.Mode = "remote"
	cfg.ServerAddr = "http://localhost:8021"

	reader, err := NewReader(cfg, nil)
	if err != nil {
		t.Fatalf("NewReader remote: %v", err)
	}
	rr, ok := reader.(*RemoteReader)
	if !ok {
		t.Fatal("expected *RemoteReader")
	}
	if rr.baseURL != "http://localhost:8021" {
		t.Errorf("expected baseURL http://localhost:8021, got %s", rr.baseURL)
	}
}

func TestNewReader_RemoteMode_MissingServerAddr(t *testing.T) {
	cfg := testRemoteConfig()
	cfg.Mode = "remote"
	cfg.ServerAddr = ""

	_, err := NewReader(cfg, nil)
	if err == nil {
		t.Fatal("expected error when server_addr is empty")
	}
}

func TestRemoteReader_GetAttachmentFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "not found",
		})
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, _, err := r.GetAttachmentFile(context.Background(), "MISSING")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestRemoteReader_GetAttachmentFile_CleanupOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusGatewayTimeout)
		w.Write([]byte("timeout"))
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, _, err := r.GetAttachmentFile(context.Background(), "ATT1")
	if err == nil {
		t.Fatal("expected error for 504")
	}

	// temp file should be cleaned up
	tmpDir := os.TempDir()
	pattern := filepath.Join(tmpDir, "zot-remote-ATT1-*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		t.Errorf("temp file not cleaned up: %v", matches)
	}
}

func testRemoteConfig() config.Config {
	return config.Config{
		Mode:        "web",
		LibraryType: "user",
		LibraryID:   "12345",
		APIKey:      "testkey",
	}
}
