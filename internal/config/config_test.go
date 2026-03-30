package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setConfigEnv(t *testing.T, root string) {
	t.Helper()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
}

func TestLoadReturnsEnvConfigWhenFileMissing(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	envPath := filepath.Join(root, ".env")
	envBody := "ZOT_LIBRARY_TYPE=user\nZOT_LIBRARY_ID=123456\nZOT_API_KEY=secret\nZOT_TIMEOUT_SECONDS=9\n"
	if err := os.WriteFile(envPath, []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LibraryType != "user" || cfg.LibraryID != "123456" || cfg.APIKey != "secret" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	if cfg.TimeoutSeconds != 9 {
		t.Fatalf("expected timeout 9, got %d", cfg.TimeoutSeconds)
	}
}

func TestLoadEnvOverridesConfigFile(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	configDir := filepath.Join(root, "zotcli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configBody := `{
  "mode": "web",
  "library_type": "group",
  "library_id": "file-id",
  "api_key": "file-key",
  "style": "apa",
  "locale": "en-US",
  "timeout_seconds": 20
}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZOT_LIBRARY_ID", "env-id")
	t.Setenv("ZOT_API_KEY", "env-key")
	t.Setenv("ZOT_TIMEOUT_SECONDS", "15")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LibraryType != "group" {
		t.Fatalf("expected file-backed library type to remain, got %q", cfg.LibraryType)
	}
	if cfg.LibraryID != "env-id" {
		t.Fatalf("expected env to override library id, got %q", cfg.LibraryID)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("expected env to override api key, got %q", cfg.APIKey)
	}
	if cfg.TimeoutSeconds != 15 {
		t.Fatalf("expected env to override timeout, got %d", cfg.TimeoutSeconds)
	}
}

func TestLoadReturnsNotFoundWithoutFileOrEnv(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	_, _, err = Load()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
