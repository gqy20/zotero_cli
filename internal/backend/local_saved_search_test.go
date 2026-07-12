package backend

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestLocalReaderListSavedSearches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zotero.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE savedSearches (savedSearchID INTEGER PRIMARY KEY, savedSearchName TEXT NOT NULL, libraryID INT NOT NULL, key TEXT NOT NULL, version INT, synced INT);`,
		`CREATE TABLE savedSearchConditions (savedSearchID INT NOT NULL, searchConditionID INT NOT NULL, condition TEXT NOT NULL, operator TEXT, value TEXT, required);`,
		`INSERT INTO savedSearches VALUES (1, 'Unread PDFs', 1, 'SEARCH01', 1, 1);`,
		`INSERT INTO savedSearchConditions VALUES (1, 1, 'tag', 'contains', 'unread', 1);`,
		`INSERT INTO savedSearchConditions VALUES (1, 2, 'itemType', 'is', 'attachment', 1);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{SQLitePath: path, SnapshotCacheDir: filepath.Join(t.TempDir(), "snapshot"), openSQLiteDB: openSQLiteDB}
	rows, err := reader.ListSavedSearches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != "SEARCH01" || rows[0].NumConditions != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].Conditions[1].Value != "attachment" {
		t.Fatalf("conditions = %#v", rows[0].Conditions)
	}
}

func TestLocalReaderSavedSearchSchemaMismatchIsUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zotero.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE savedSearches (savedSearchID INTEGER PRIMARY KEY, savedSearchName TEXT);`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	reader := &LocalReader{SQLitePath: path, SnapshotCacheDir: filepath.Join(t.TempDir(), "snapshot"), openSQLiteDB: openSQLiteDB}
	_, err = reader.ListSavedSearches(context.Background())
	if err == nil || !errors.Is(err, ErrUnsupportedFeature) {
		t.Fatalf("expected unsupported feature, got %v", err)
	}
}
