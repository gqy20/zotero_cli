package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/config"
)

func TestResolveLibraryTastePathPrefersDataDir(t *testing.T) {
	dataDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), ".zot", ".env")
	want := filepath.Join(dataDir, ".zotero_cli", "taste.md")
	if got := ResolveLibraryTastePath(config.Config{DataDir: dataDir}, configPath); got != want {
		t.Fatalf("path = %q; want %q", got, want)
	}
}

func TestResolveLibraryTastePathFallsBackToConfigDir(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".zot", ".env")
	want := filepath.Join(filepath.Dir(configPath), ".zotero_cli", "taste.md")
	if got := ResolveLibraryTastePath(config.Config{}, configPath); got != want {
		t.Fatalf("path = %q; want %q", got, want)
	}
}

func TestInitAndReadLibraryTaste(t *testing.T) {
	dataDir := t.TempDir()
	service := ReadService{LoadConfig: func() (config.Config, string, error) {
		return config.Config{DataDir: dataDir}, filepath.Join(t.TempDir(), ".zot", ".env"), nil
	}}

	missing, err := service.Taste(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Warnings) != 1 || missing.Warnings[0].Code != "taste_missing" {
		t.Fatalf("missing warnings = %#v", missing.Warnings)
	}

	created, err := service.InitTaste(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	taste, ok := created.Data.(LibraryTaste)
	if !ok || !taste.Exists || !strings.Contains(taste.Content, "# Zotero Library Taste") {
		t.Fatalf("created taste = %#v", created.Data)
	}
	if _, err := os.Stat(taste.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitTaste(context.Background(), false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second init error = %v", err)
	}

	if err := os.WriteFile(taste.Path, []byte("custom"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitTaste(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	read, err := service.Taste(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	loaded := read.Data.(LibraryTaste)
	if loaded.Content == "custom" || !strings.Contains(loaded.Content, "# Zotero Library Taste") {
		t.Fatalf("force init content = %q", loaded.Content)
	}
}
