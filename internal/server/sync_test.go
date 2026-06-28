package server

import (
	"archive/tar"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newSyncMux builds a Handler rooted at a temp dataDir and returns its mux.
func newSyncMux(t *testing.T, dataDir string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	h := NewHandlerWithDir(nil, dataDir)
	h.RegisterRoutes(mux)
	return mux
}

func writeSyncFixture(t *testing.T, dataDir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dataDir, "zotero.sqlite"), "FAKE SQLITE")
	mustWrite(t, filepath.Join(dataDir, "storage", "KEY1", "paper.pdf"), "PDF BYTES")
	mustWrite(t, filepath.Join(dataDir, "storage", "KEY2", "notes.md"), "NOTES")
	mustWrite(t, filepath.Join(dataDir, ".zotero_cli", "fulltext", "index.sqlite"), "FTS INDEX")
	mustWrite(t, filepath.Join(dataDir, ".zotero_cli", "fulltext", "cache", "KEY1", "content.txt"), "EXTRACTED TEXT")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncManifest(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/manifest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Ok   bool         `json:"ok"`
		Data syncManifest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !resp.Ok {
		t.Fatal("expected ok=true")
	}
	if resp.Data.SQLite.Name != "zotero.sqlite" || resp.Data.SQLite.Size != int64(len("FAKE SQLITE")) {
		t.Fatalf("sqlite entry wrong: %+v", resp.Data.SQLite)
	}
	if resp.Data.SQLite.Mtime == 0 {
		t.Fatal("sqlite mtime should be set")
	}
	keys := map[string]bool{}
	for _, e := range resp.Data.Storage {
		keys[e.Key] = true
		for _, f := range e.Files {
			if f.Size == 0 || f.Mtime == 0 {
				t.Fatalf("file entry missing size/mtime: %+v", f)
			}
		}
	}
	if !keys["KEY1"] || !keys["KEY2"] {
		t.Fatalf("expected KEY1 and KEY2 in storage, got %v", keys)
	}

	// fulltext tree: index.sqlite + cache/KEY1/content.txt
	ftPaths := map[string]bool{}
	for _, f := range resp.Data.Fulltext {
		if f.Size == 0 || f.Mtime == 0 {
			t.Fatalf("fulltext entry missing size/mtime: %+v", f)
		}
		ftPaths[f.Path] = true
	}
	if !ftPaths["index.sqlite"] || !ftPaths["cache/KEY1/content.txt"] {
		t.Fatalf("expected fulltext index.sqlite + cache/KEY1/content.txt, got %v", ftPaths)
	}
}

func TestSyncFulltextFile(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/fulltext/cache/KEY1/content.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "EXTRACTED TEXT" {
		t.Fatalf("body wrong: %q", rec.Body.String())
	}
}

func TestSyncFulltextFileNotFound(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/fulltext/cache/KEY1/missing.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSyncSQLiteTar(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	// Add a -wal sidecar to confirm it's included.
	mustWrite(t, filepath.Join(dataDir, "zotero.sqlite-wal"), "WAL")
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/sqlite", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-tar" {
		t.Fatalf("expected tar content-type, got %s", ct)
	}

	tr := tar.NewReader(rec.Body)
	names := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		body, _ := io.ReadAll(tr)
		names[hdr.Name] = string(body)
	}
	if names["zotero.sqlite"] != "FAKE SQLITE" {
		t.Fatalf("sqlite content wrong: %q", names["zotero.sqlite"])
	}
	if names["zotero.sqlite-wal"] != "WAL" {
		t.Fatalf("wal not included: %+v", names)
	}
}

func TestSyncStorageFile(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/storage/KEY1/paper.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "PDF BYTES" {
		t.Fatalf("body wrong: %q", rec.Body.String())
	}
}

func TestSyncStorageFileNotFound(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/storage/KEY1/missing.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSyncNoDataDir(t *testing.T) {
	mux := newSyncMux(t, "") // dataDir empty

	req := httptest.NewRequest("GET", "/api/v1/sync/manifest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestPathIsWithin(t *testing.T) {
	dir := filepath.Join("tmp", "storage")
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(dir, "KEY", "a.pdf"), true},
		{dir, true}, // the dir itself is within
		{filepath.Join(dir, "..", "secret"), false},
		{filepath.Join(dir, "KEY", "..", "..", "etc"), false},
	}
	for _, c := range cases {
		if got := pathIsWithin(c.path, dir); got != c.want {
			t.Errorf("pathIsWithin(%q, %q) = %v, want %v", c.path, dir, got, c.want)
		}
	}
}
