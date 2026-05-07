package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

	tmpDir := os.TempDir()
	pattern := filepath.Join(tmpDir, "zot-remote-ATT1-*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		t.Errorf("temp file not cleaned up: %v", matches)
	}
}

// --- FindOptions round-trip tests ---

func TestFindOptionsRoundTrip_AllFields(t *testing.T) {
	original := FindOptions{
		Query:             "CRISPR",
		FullText:          true,
		FullTextAny:       true,
		All:               true,
		Full:              true,
		ItemType:          "journalArticle",
		Limit:             50,
		Start:             10,
		Tag:               "important",
		Tags:              []string{"tag1", "tag2"},
		TagAny:            true,
		IncludeFields:     []string{"title", "date"},
		Sort:              "date",
		Direction:         "desc",
		QMode:             "everything",
		IncludeTrashed:    true,
		DateAfter:         "2024-01-01",
		DateBefore:        "2024-12-31",
		HasPDF:            true,
		AttachmentName:    "supplement",
		AttachmentPath:    "/papers",
		AttachmentType:    "pdf",
		Collection:        []string{"COL1", "COL2"},
		NoCollection:      []string{"COL3"},
		TagContains:       []string{"gen"},
		ExcludeTags:       []string{"draft"},
		ExcludeItemType:   "note",
		DateModifiedAfter: "2024-06-01",
		DateAddedAfter:    "2024-03-01",
	}

	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		writeOK(w, []domain.Item{})
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, err := r.FindItems(context.Background(), original)
	if err != nil {
		t.Fatalf("FindItems: %v", err)
	}

	assertQueryEqual(t, capturedQuery, "q", "CRISPR")
	assertQueryEqual(t, capturedQuery, "full_text", "true")
	assertQueryEqual(t, capturedQuery, "full_text_any", "true")
	assertQueryEqual(t, capturedQuery, "all", "true")
	assertQueryEqual(t, capturedQuery, "full", "true")
	assertQueryEqual(t, capturedQuery, "item_type", "journalArticle")
	assertQueryEqual(t, capturedQuery, "limit", "50")
	assertQueryEqual(t, capturedQuery, "start", "10")
	assertQueryEqual(t, capturedQuery, "tag", "important")
	assertQueryEqual(t, capturedQuery, "tags", "tag1,tag2")
	assertQueryEqual(t, capturedQuery, "tag_any", "true")
	assertQueryEqual(t, capturedQuery, "include_fields", "title,date")
	assertQueryEqual(t, capturedQuery, "sort", "date")
	assertQueryEqual(t, capturedQuery, "direction", "desc")
	assertQueryEqual(t, capturedQuery, "qmode", "everything")
	assertQueryEqual(t, capturedQuery, "include_trashed", "true")
	assertQueryEqual(t, capturedQuery, "date_after", "2024-01-01")
	assertQueryEqual(t, capturedQuery, "date_before", "2024-12-31")
	assertQueryEqual(t, capturedQuery, "has_pdf", "true")
	assertQueryEqual(t, capturedQuery, "attachment_name", "supplement")
	assertQueryEqual(t, capturedQuery, "attachment_path", "/papers")
	assertQueryEqual(t, capturedQuery, "attachment_type", "pdf")
	assertQueryEqual(t, capturedQuery, "collection", "COL1,COL2")
	assertQueryEqual(t, capturedQuery, "no_collection", "COL3")
	assertQueryEqual(t, capturedQuery, "tag_contains", "gen")
	assertQueryEqual(t, capturedQuery, "exclude_tags", "draft")
	assertQueryEqual(t, capturedQuery, "exclude_item_type", "note")
	assertQueryEqual(t, capturedQuery, "date_modified_after", "2024-06-01")
	assertQueryEqual(t, capturedQuery, "date_added_after", "2024-03-01")
}

func TestFindOptionsRoundTrip_EmptyFields(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		writeOK(w, []domain.Item{})
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, err := r.FindItems(context.Background(), FindOptions{})
	if err != nil {
		t.Fatalf("FindItems: %v", err)
	}

	// empty options should not send any of the optional params
	optionalParams := []string{
		"q", "full_text", "full_text_any", "all", "item_type",
		"tag", "tags", "tag_any", "include_fields", "sort", "direction",
		"qmode", "include_trashed", "date_after", "date_before",
		"has_pdf", "attachment_name", "attachment_path", "attachment_type",
		"collection", "no_collection", "tag_contains", "exclude_tags",
		"exclude_item_type", "date_modified_after", "date_added_after",
	}
	for _, p := range optionalParams {
		if capturedQuery.Get(p) != "" {
			t.Errorf("empty opts should not set %s, got %q", p, capturedQuery.Get(p))
		}
	}
}

func TestParseFindOptions_AllFields(t *testing.T) {
	// Build the query string that RemoteReader would produce for a full FindOptions
	q := url.Values{}
	q.Set("q", "CRISPR")
	q.Set("full_text", "true")
	q.Set("full_text_any", "true")
	q.Set("all", "true")
	q.Set("full", "true")
	q.Set("item_type", "journalArticle")
	q.Set("limit", "50")
	q.Set("start", "10")
	q.Set("tag", "important")
	q.Set("tags", "tag1,tag2")
	q.Set("tag_any", "true")
	q.Set("include_fields", "title,date")
	q.Set("sort", "date")
	q.Set("direction", "desc")
	q.Set("qmode", "everything")
	q.Set("include_trashed", "true")
	q.Set("date_after", "2024-01-01")
	q.Set("date_before", "2024-12-31")
	q.Set("has_pdf", "true")
	q.Set("attachment_name", "supplement")
	q.Set("attachment_path", "/papers")
	q.Set("attachment_type", "pdf")
	q.Set("collection", "COL1,COL2")
	q.Set("no_collection", "COL3")
	q.Set("tag_contains", "gen")
	q.Set("exclude_tags", "draft")
	q.Set("exclude_item_type", "note")
	q.Set("date_modified_after", "2024-06-01")
	q.Set("date_added_after", "2024-03-01")

	opts := parseFindOptionsForTest(t, q)

	if opts.Query != "CRISPR" {
		t.Errorf("Query: got %q", opts.Query)
	}
	if !opts.FullText {
		t.Error("FullText: expected true")
	}
	if !opts.FullTextAny {
		t.Error("FullTextAny: expected true")
	}
	if !opts.All {
		t.Error("All: expected true")
	}
	if !opts.Full {
		t.Error("Full: expected true")
	}
	if opts.ItemType != "journalArticle" {
		t.Errorf("ItemType: got %q", opts.ItemType)
	}
	if opts.Limit != 50 {
		t.Errorf("Limit: got %d", opts.Limit)
	}
	if opts.Start != 10 {
		t.Errorf("Start: got %d", opts.Start)
	}
	if opts.Tag != "important" {
		t.Errorf("Tag: got %q", opts.Tag)
	}
	if len(opts.Tags) != 2 || opts.Tags[0] != "tag1" || opts.Tags[1] != "tag2" {
		t.Errorf("Tags: got %v", opts.Tags)
	}
	if !opts.TagAny {
		t.Error("TagAny: expected true")
	}
	if len(opts.IncludeFields) != 2 || opts.IncludeFields[0] != "title" || opts.IncludeFields[1] != "date" {
		t.Errorf("IncludeFields: got %v", opts.IncludeFields)
	}
	if opts.Sort != "date" {
		t.Errorf("Sort: got %q", opts.Sort)
	}
	if opts.Direction != "desc" {
		t.Errorf("Direction: got %q", opts.Direction)
	}
	if opts.QMode != "everything" {
		t.Errorf("QMode: got %q", opts.QMode)
	}
	if !opts.IncludeTrashed {
		t.Error("IncludeTrashed: expected true")
	}
	if opts.DateAfter != "2024-01-01" {
		t.Errorf("DateAfter: got %q", opts.DateAfter)
	}
	if opts.DateBefore != "2024-12-31" {
		t.Errorf("DateBefore: got %q", opts.DateBefore)
	}
	if !opts.HasPDF {
		t.Error("HasPDF: expected true")
	}
	if opts.AttachmentName != "supplement" {
		t.Errorf("AttachmentName: got %q", opts.AttachmentName)
	}
	if opts.AttachmentPath != "/papers" {
		t.Errorf("AttachmentPath: got %q", opts.AttachmentPath)
	}
	if opts.AttachmentType != "pdf" {
		t.Errorf("AttachmentType: got %q", opts.AttachmentType)
	}
	if len(opts.Collection) != 2 || opts.Collection[0] != "COL1" || opts.Collection[1] != "COL2" {
		t.Errorf("Collection: got %v", opts.Collection)
	}
	if len(opts.NoCollection) != 1 || opts.NoCollection[0] != "COL3" {
		t.Errorf("NoCollection: got %v", opts.NoCollection)
	}
	if len(opts.TagContains) != 1 || opts.TagContains[0] != "gen" {
		t.Errorf("TagContains: got %v", opts.TagContains)
	}
	if len(opts.ExcludeTags) != 1 || opts.ExcludeTags[0] != "draft" {
		t.Errorf("ExcludeTags: got %v", opts.ExcludeTags)
	}
	if opts.ExcludeItemType != "note" {
		t.Errorf("ExcludeItemType: got %q", opts.ExcludeItemType)
	}
	if opts.DateModifiedAfter != "2024-06-01" {
		t.Errorf("DateModifiedAfter: got %q", opts.DateModifiedAfter)
	}
	if opts.DateAddedAfter != "2024-03-01" {
		t.Errorf("DateAddedAfter: got %q", opts.DateAddedAfter)
	}
}

// --- baseURL trailing slash tests ---

func TestRemoteReader_TrailingSlashStripped(t *testing.T) {
	srv := httptest.NewServer(newRemoteTestHandler(t, "getStats", LibraryStats{}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL+"/", srv.Client())
	_, err := r.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("trailing slash should be handled: %v", err)
	}
}

func TestRemoteReader_TrailingSlash_GetItem(t *testing.T) {
	srv := httptest.NewServer(newRemoteTestHandler(t, "getItem", domain.Item{Key: "ABC123"}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL+"/", srv.Client())
	item, err := r.GetItem(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("GetItem with trailing slash: %v", err)
	}
	if item.Key != "ABC123" {
		t.Errorf("expected ABC123, got %s", item.Key)
	}
}

func TestRemoteReader_TrailingSlash_FindItemsPath(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Path
		writeOK(w, []domain.Item{})
	}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL+"/", srv.Client())
	r.FindItems(context.Background(), FindOptions{Query: "test"})

	if captured != "/api/v1/items" {
		t.Errorf("expected /api/v1/items, got %s", captured)
	}
	if strings.Contains(captured, "//") {
		t.Errorf("path contains double slash: %s", captured)
	}
}

// --- NewReader integration tests ---

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

func TestNewReader_RemoteMode_TrailingSlashStripped(t *testing.T) {
	cfg := testRemoteConfig()
	cfg.Mode = "remote"
	cfg.ServerAddr = "http://192.168.1.100:8021/"

	reader, err := NewReader(cfg, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	rr := reader.(*RemoteReader)
	if strings.HasSuffix(rr.baseURL, "/") {
		t.Errorf("trailing slash not stripped: %s", rr.baseURL)
	}
}

// --- helpers ---

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
			if r.URL.Path != "/api/v1/items/ABC123" {
				t.Errorf("expected /api/v1/items/ABC123, got %s", r.URL.Path)
			}
		case "getRelated":
			if r.URL.Path != "/api/v1/items/ABC123/related" {
				t.Errorf("expected /api/v1/items/ABC123/related, got %s", r.URL.Path)
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
		writeOK(w, data)
	}
}

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": data,
		"meta": map[string]any{},
	})
}

func assertQueryEqual(t *testing.T, q url.Values, key, expected string) {
	t.Helper()
	got := q.Get(key)
	if got != expected {
		t.Errorf("query param %q: expected %q, got %q", key, expected, got)
	}
}

func TestRemoteReader_ConsumeReadMetadata(t *testing.T) {
	srv := httptest.NewServer(newRemoteTestHandler(t, "getStats", LibraryStats{}))
	defer srv.Close()

	r := NewRemoteReader(srv.URL, srv.Client())
	_, err := r.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryStats: %v", err)
	}

	meta := r.ConsumeReadMetadata()
	if meta.ReadSource != "remote" {
		t.Errorf("expected read_source 'remote', got %q", meta.ReadSource)
	}

	// second call should return empty (consumed)
	meta2 := r.ConsumeReadMetadata()
	if meta2.ReadSource != "" {
		t.Errorf("expected empty after consume, got %q", meta2.ReadSource)
	}
}

func TestRemoteReader_ConsumeReadMetadata_BeforeAnyCall(t *testing.T) {
	r := NewRemoteReader("http://localhost:1", nil)
	meta := r.ConsumeReadMetadata()
	if meta.ReadSource != "" {
		t.Errorf("expected empty before any call, got %q", meta.ReadSource)
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

// parseFindOptionsForTest wraps server.parseFindOptions for cross-package testing.
func parseFindOptionsForTest(t *testing.T, q url.Values) FindOptions {
	t.Helper()
	return ParseFindOptionsFromQuery(q)
}
