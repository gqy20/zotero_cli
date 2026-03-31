package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setConfigEnv(t *testing.T, root string) {
	t.Helper()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
}

func TestLoadReturnsEnvConfigWhenFileMissing(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	envDir := filepath.Join(root, ".zot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(envDir, ".env")
	envBody := "ZOT_LIBRARY_TYPE=user\nZOT_LIBRARY_ID=123456\nZOT_API_KEY=secret\nZOT_TIMEOUT_SECONDS=9\nZOT_RETRY_MAX_ATTEMPTS=4\nZOT_RETRY_BASE_DELAY_MS=125\nZOT_ALLOW_WRITE=1\nZOT_ALLOW_DELETE=0\n"
	if err := os.WriteFile(envPath, []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}

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
	if cfg.RetryMaxAttempts != 4 {
		t.Fatalf("expected retry max attempts 4, got %d", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelayMilliseconds != 125 {
		t.Fatalf("expected retry base delay 125, got %d", cfg.RetryBaseDelayMilliseconds)
	}
	if !cfg.AllowWrite || cfg.AllowDelete {
		t.Fatalf("unexpected permissions: %+v", cfg)
	}
}

func TestLoadEnvOverridesEnvFile(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	configDir := filepath.Join(root, ".zot")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configBody := strings.Join([]string{
		"ZOT_MODE=web",
		"ZOT_LIBRARY_TYPE=group",
		"ZOT_LIBRARY_ID=file-id",
		"ZOT_API_KEY=file-key",
		"ZOT_STYLE=apa",
		"ZOT_LOCALE=en-US",
		"ZOT_TIMEOUT_SECONDS=20",
		"ZOT_RETRY_MAX_ATTEMPTS=3",
		"ZOT_RETRY_BASE_DELAY_MS=250",
		"ZOT_ALLOW_WRITE=0",
		"ZOT_ALLOW_DELETE=1",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZOT_LIBRARY_ID", "env-id")
	t.Setenv("ZOT_API_KEY", "env-key")
	t.Setenv("ZOT_TIMEOUT_SECONDS", "15")
	t.Setenv("ZOT_RETRY_MAX_ATTEMPTS", "5")
	t.Setenv("ZOT_ALLOW_WRITE", "1")

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
	if cfg.RetryMaxAttempts != 5 {
		t.Fatalf("expected env to override retry attempts, got %d", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelayMilliseconds != 250 {
		t.Fatalf("expected env-file retry delay to remain, got %d", cfg.RetryBaseDelayMilliseconds)
	}
	if !cfg.AllowWrite || !cfg.AllowDelete {
		t.Fatalf("unexpected permissions after override: %+v", cfg)
	}
}

func TestLoadReturnsNotFoundWithoutFileOrEnv(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)

	_, _, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
