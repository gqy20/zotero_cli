package backend

import (
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
)

func TestNewLocalReaderRequiresDataDir(t *testing.T) {
	_, err := NewLocalReader(config.Config{})
	if err == nil {
		t.Fatalf("NewLocalReader() error = nil, want error")
	}
	if err.Error() != "local mode requires data_dir" {
		t.Fatalf("NewLocalReader() error = %q, want data_dir error", err.Error())
	}
}

func TestNewLocalReaderBuildsDerivedPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "zotero.sqlite"), []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}

	reader, err := NewLocalReader(config.Config{DataDir: root})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	if reader.DataDir != root {
		t.Fatalf("DataDir = %q, want %q", reader.DataDir, root)
	}
	if reader.SQLitePath != filepath.Join(root, "zotero.sqlite") {
		t.Fatalf("SQLitePath = %q", reader.SQLitePath)
	}
	if reader.StorageDir != filepath.Join(root, "storage") {
		t.Fatalf("StorageDir = %q", reader.StorageDir)
	}
}
