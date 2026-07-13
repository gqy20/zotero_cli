package backend

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectionTargetResolvesKeyNameAndFullPath(t *testing.T) {
	reader := collectionTargetTestReader(t)

	tests := []struct {
		selector string
		key      string
		path     string
	}{
		{selector: "shared1", key: "SHARED1", path: "Research/Plants/Shared"},
		{selector: "Plants", key: "PLANTS01", path: "Research/Plants"},
		{selector: " Research / Plants / Shared ", key: "SHARED1", path: "Research/Plants/Shared"},
		{selector: `Research\Plants\Shared`, key: "SHARED1", path: "Research/Plants/Shared"},
		{selector: "Archive/Shared", key: "SHARED2", path: "Archive/Shared"},
	}
	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			got, err := reader.CollectionTarget(context.Background(), tt.selector)
			if err != nil {
				t.Fatal(err)
			}
			if got.Key != tt.key || got.Path != tt.path {
				t.Fatalf("CollectionTarget() = %#v, want key=%s path=%s", got, tt.key, tt.path)
			}
		})
	}
}

func TestCollectionTargetRejectsAmbiguousNameWithCandidates(t *testing.T) {
	reader := collectionTargetTestReader(t)
	_, err := reader.CollectionTarget(context.Background(), "Shared")
	if err == nil {
		t.Fatal("expected ambiguous collection error")
	}
	for _, want := range []string{"ambiguous", "Archive/Shared (SHARED2)", "Research/Plants/Shared (SHARED1)"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestCollectionTargetNotFoundSuggestsCollectionList(t *testing.T) {
	reader := collectionTargetTestReader(t)
	_, err := reader.CollectionTarget(context.Background(), "Missing")
	if err == nil || !strings.Contains(err.Error(), "zot coll list") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func collectionTargetTestReader(t *testing.T) *LocalReader {
	t.Helper()
	path := filepath.Join(t.TempDir(), "zotero.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT NOT NULL, collectionName TEXT NOT NULL, parentCollectionID INTEGER);`,
		`INSERT INTO collections VALUES (1, 'ROOT0001', 'Research', NULL);`,
		`INSERT INTO collections VALUES (2, 'ROOT0002', 'Archive', NULL);`,
		`INSERT INTO collections VALUES (3, 'PLANTS01', 'Plants', 1);`,
		`INSERT INTO collections VALUES (4, 'SHARED1', 'Shared', 3);`,
		`INSERT INTO collections VALUES (5, 'SHARED2', 'Shared', 2);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return &LocalReader{SQLitePath: path, SnapshotCacheDir: filepath.Join(t.TempDir(), "snapshot"), openSQLiteDB: openSQLiteDB}
}
