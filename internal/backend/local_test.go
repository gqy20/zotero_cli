package backend

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
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
