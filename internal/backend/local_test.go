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

func TestNormalizeLocalDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "year only", input: "2017", want: "2017"},
		{name: "date only", input: "2019-03-29", want: "2019-03-29"},
		{name: "duplicate date", input: "2019-03-29 2019-03-29", want: "2019-03-29"},
		{name: "duplicate date with time", input: "2024-01-08 2024-01-08 00:00:00", want: "2024-01-08"},
		{name: "whitespace cleanup", input: " 2024-01-08   2024-01-08 ", want: "2024-01-08"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLocalDate(tt.input); got != tt.want {
				t.Fatalf("normalizeLocalDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
