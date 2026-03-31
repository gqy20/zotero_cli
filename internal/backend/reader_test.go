package backend

import (
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
)

func TestNewReaderDefaultsToWebMode(t *testing.T) {
	reader, err := NewReader(config.Config{}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*WebReader); !ok {
		t.Fatalf("NewReader() returned %T, want *WebReader", reader)
	}
}

func TestNewReaderWebMode(t *testing.T) {
	reader, err := NewReader(config.Config{Mode: "web"}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*WebReader); !ok {
		t.Fatalf("NewReader() returned %T, want *WebReader", reader)
	}
}

func TestNewReaderLocalModeBuildsLocalReader(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "zotero.sqlite"), []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}

	reader, err := NewReader(config.Config{Mode: "local", DataDir: root}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*LocalReader); !ok {
		t.Fatalf("NewReader() returned %T, want *LocalReader", reader)
	}
}

func TestNewReaderRejectsUnimplementedHybridMode(t *testing.T) {
	_, err := NewReader(config.Config{Mode: "hybrid"}, nil)
	if err == nil {
		t.Fatalf("NewReader() error = nil, want error")
	}
	if err.Error() != "hybrid mode is not implemented yet" {
		t.Fatalf("NewReader() error = %q, want hybrid error", err.Error())
	}
}

func TestNewReaderRejectsUnsupportedMode(t *testing.T) {
	_, err := NewReader(config.Config{Mode: "bogus"}, nil)
	if err == nil {
		t.Fatalf("NewReader() error = nil, want error")
	}
	if err.Error() != "unsupported mode \"bogus\"" {
		t.Fatalf("NewReader() error = %q, want unsupported mode error", err.Error())
	}
}
