package backend

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/config"

	_ "modernc.org/sqlite"
)

func TestLocalSQLiteDSNUsesReadOnlyPragmas(t *testing.T) {
	dsn := localSQLiteDSN(`D:\Zotero\zotero.sqlite`)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if u.Scheme != "file" {
		t.Fatalf("unexpected scheme: %q", u.Scheme)
	}
	if got := u.Query().Get("mode"); got != "ro" {
		t.Fatalf("unexpected mode query param: %q", got)
	}
	pragmas := u.Query()["_pragma"]
	if len(pragmas) != 2 {
		t.Fatalf("unexpected pragmas: %#v", pragmas)
	}
	if pragmas[0] != "busy_timeout=5000" && pragmas[1] != "busy_timeout=5000" {
		t.Fatalf("expected busy_timeout pragma, got %#v", pragmas)
	}
	if pragmas[0] != "query_only=1" && pragmas[1] != "query_only=1" {
		t.Fatalf("expected query_only pragma, got %#v", pragmas)
	}
}

func TestLocalSQLiteDSNRespectsBusyTimeoutOverride(t *testing.T) {
	t.Setenv("ZOT_LOCAL_BUSY_TIMEOUT_MS", "25")

	dsn := localSQLiteDSN(`D:\Zotero\zotero.sqlite`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	pragmas := u.Query()["_pragma"]
	found := false
	for _, pragma := range pragmas {
		if pragma == "busy_timeout=25" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected busy_timeout override, got %#v", pragmas)
	}
}

func TestCreateSQLiteSnapshotCopiesDatabaseAndSidecars(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	journalPath := sqlitePath + "-journal"
	walPath := sqlitePath + "-wal"

	if err := os.WriteFile(sqlitePath, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte("journal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, []byte("wal"), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshotDir, snapshotPath, err := createSQLiteSnapshot(sqlitePath)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	defer os.RemoveAll(snapshotDir)

	for path, want := range map[string]string{
		snapshotPath:              "db",
		snapshotPath + "-journal": "journal",
		snapshotPath + "-wal":     "wal",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read snapshot file %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("unexpected snapshot contents for %s: %q", path, string(data))
		}
	}
}

func TestWithReadableDBFallsBackToSnapshotWhenQueryHitsBusy(t *testing.T) {
	liveDB, err := sql.Open("sqlite", "file:live-fallback?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer liveDB.Close()

	snapshotDB, err := sql.Open("sqlite", "file:snapshot-fallback?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotDB.Close()

	previousOpen := openSQLiteDBFunc
	previousSnapshot := createSQLiteSnapshotFunc
	t.Cleanup(func() {
		openSQLiteDBFunc = previousOpen
		createSQLiteSnapshotFunc = previousSnapshot
	})

	openSQLiteDBFunc = func(dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "snapshot.sqlite") {
			return snapshotDB, nil
		}
		return liveDB, nil
	}
	createSQLiteSnapshotFunc = func(string) (string, string, error) {
		snapshotDir := t.TempDir()
		return snapshotDir, filepath.Join(snapshotDir, "snapshot.sqlite"), nil
	}

	reader := &LocalReader{SQLitePath: filepath.Join(t.TempDir(), "zotero.sqlite")}
	attempts := 0
	err = reader.withReadableDB(context.Background(), func(db *sql.DB) error {
		attempts++
		if db == liveDB {
			return errors.New("SQLITE_BUSY: database is locked")
		}
		if db != snapshotDB {
			t.Fatalf("unexpected db pointer %p", db)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withReadableDB() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("withReadableDB() attempts = %d, want 2", attempts)
	}
	meta := reader.ConsumeReadMetadata()
	if meta.ReadSource != "snapshot" || !meta.SQLiteFallback {
		t.Fatalf("ConsumeReadMetadata() = %#v, want snapshot metadata", meta)
	}
}

func TestNewLocalReaderLoadsDataDirAndAttachmentBaseDirFromPrefs(t *testing.T) {
	appData := t.TempDir()
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseAttachmentDir := t.TempDir()
	prefsPath := filepath.Join(appData, "Zotero", "Zotero", "Profiles", "abcd.default", "prefs.js")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	prefsContent := strings.Join([]string{
		`user_pref("extensions.zotero.baseAttachmentPath", "` + strings.ReplaceAll(baseAttachmentDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.dataDir", "` + strings.ReplaceAll(dataDir, `\`, `\\`) + `");`,
		"",
	}, "\n")
	if err := os.WriteFile(prefsPath, []byte(prefsContent), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", t.TempDir())

	reader, err := NewLocalReader(config.Config{})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	if reader.DataDir != dataDir {
		t.Fatalf("reader.DataDir = %q, want %q", reader.DataDir, dataDir)
	}
	if reader.AttachmentBaseDir != baseAttachmentDir {
		t.Fatalf("reader.AttachmentBaseDir = %q, want %q", reader.AttachmentBaseDir, baseAttachmentDir)
	}
}

func TestResolveAttachmentPathSupportsAttachmentsRelativeBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	relativePath := filepath.Join("papers", "example.pdf")
	absolutePath := filepath.Join(baseDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolutePath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{AttachmentBaseDir: baseDir}
	got, ok := reader.resolveAttachmentPath("ATTACH1", "attachments:papers/example.pdf", "example.pdf")
	if !ok {
		t.Fatal("resolveAttachmentPath() did not resolve attachments: path")
	}
	if got != absolutePath {
		t.Fatalf("resolveAttachmentPath() = %q, want %q", got, absolutePath)
	}
}

func TestResolveAttachmentPathSupportsAbsolutePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absolute.pdf")
	if err := os.WriteFile(path, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{}
	got, ok := reader.resolveAttachmentPath("ATTACH1", path, "absolute.pdf")
	if !ok {
		t.Fatal("resolveAttachmentPath() did not resolve absolute path")
	}
	if got != path {
		t.Fatalf("resolveAttachmentPath() = %q, want %q", got, path)
	}
}
