package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/backend"
)

type linkedSyncReader struct {
	mockReader
	path string
}

func (r *linkedSyncReader) ListSyncLinkedAttachments(context.Context) ([]backend.SyncLinkedAttachment, error) {
	info, err := os.Stat(r.path)
	if err != nil {
		return nil, err
	}
	return []backend.SyncLinkedAttachment{
		{Key: "LINK1", Name: filepath.Base(r.path), Size: info.Size(), Mtime: info.ModTime().Unix(), Available: true},
		{Key: "MISSING", Name: "missing.pdf", Error: "source file is unavailable"},
	}, nil
}

func (r *linkedSyncReader) GetAttachmentFile(context.Context, string) (string, string, error) {
	return r.path, "application/pdf", nil
}

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
	// SQLite is a per-file list (main db + sidecars); main db must be present.
	sqlitePaths := map[string]int64{}
	for _, e := range resp.Data.SQLite {
		if e.Mtime == 0 {
			t.Fatalf("sqlite entry missing mtime: %+v", e)
		}
		sqlitePaths[e.Path] = e.Size
	}
	if sqlitePaths["zotero.sqlite"] != int64(len("FAKE SQLITE")) {
		t.Fatalf("sqlite main entry wrong: %+v", sqlitePaths)
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

func TestSyncLinkedAttachmentManifestAndDownload(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	linkedPath := filepath.Join(t.TempDir(), "linked.pdf")
	mustWrite(t, linkedPath, "LINKED PDF")
	mux := http.NewServeMux()
	NewHandlerWithDir(&linkedSyncReader{path: linkedPath}, dataDir).RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/sync/manifest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("manifest: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data syncManifest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Linked) != 2 || !response.Data.Linked[0].Available || response.Data.Linked[1].Available {
		t.Fatalf("unexpected linked manifest: %#v", response.Data.Linked)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/sync/linked/LINK1/linked.pdf", nil)
	req.Header.Set("Range", "bytes=7-")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "PDF" {
		t.Fatalf("linked range response: code=%d body=%q", rec.Code, rec.Body.String())
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

func TestSyncSqliteFile(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mustWrite(t, filepath.Join(dataDir, "zotero.sqlite-wal"), "WAL")
	mux := newSyncMux(t, dataDir)

	// Main db and sidecar are each fetchable independently (per-file sync).
	for _, c := range []struct {
		path string
		want string
	}{
		{"/api/v1/sync/sqlite-file/zotero.sqlite", "FAKE SQLITE"},
		{"/api/v1/sync/sqlite-file/zotero.sqlite-wal", "WAL"},
	} {
		req := httptest.NewRequest("GET", c.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", c.path, rec.Code)
		}
		if rec.Body.String() != c.want {
			t.Fatalf("%s: body %q, want %q", c.path, rec.Body.String(), c.want)
		}
	}

	// Non-zotero.sqlite files are rejected.
	req := httptest.NewRequest("GET", "/api/v1/sync/sqlite-file/secret.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// writeSyncFixture writes storage/KEY1/paper.pdf etc., not secret.txt at root
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-sqlite name, got %d", rec.Code)
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

func TestSyncStorageFileSupportsRange(t *testing.T) {
	dataDir := t.TempDir()
	writeSyncFixture(t, dataDir)
	mux := newSyncMux(t, dataDir)

	req := httptest.NewRequest("GET", "/api/v1/sync/storage/KEY1/paper.pdf", nil)
	req.Header.Set("Range", "bytes=4-")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", rec.Code)
	}
	if rec.Body.String() != "BYTES" {
		t.Fatalf("unexpected range body %q", rec.Body.String())
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

func TestSyncStorageFileRejectsEscapingSymlink(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "storage", "KEY"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "zotero.sqlite"), []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dataDir, "storage", "KEY", "link.pdf")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	mux := newSyncMux(t, dataDir)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/sync/storage/KEY/link.pdf", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected escaping symlink to be rejected, got %d", rec.Code)
	}
}
