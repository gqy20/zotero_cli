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
	dataDir := filepath.Join(root, "zotero")
	envBody := "ZOT_DATA_DIR=" + dataDir + "\nZOT_LIBRARY_TYPE=user\nZOT_LIBRARY_ID=123456\nZOT_API_KEY=secret\nZOT_TIMEOUT_SECONDS=9\nZOT_RETRY_MAX_ATTEMPTS=4\nZOT_RETRY_BASE_DELAY_MS=125\nZOT_ALLOW_WRITE=1\nZOT_ALLOW_DELETE=0\n"
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
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir to load, got %q", cfg.DataDir)
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
		"ZOT_DATA_DIR=" + filepath.Join(root, "file-data"),
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
	envDataDir := filepath.Join(root, "env-data")
	t.Setenv("ZOT_DATA_DIR", envDataDir)
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
	if cfg.DataDir != envDataDir {
		t.Fatalf("expected env to override data dir, got %q", cfg.DataDir)
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

func TestLoadRejectsRelativeDataDir(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".zot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".zot", ".env"), []byte("ZOT_DATA_DIR=relative/path\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("Load() error = %v; want absolute-path error", err)
	}
}

func TestSaveNormalizesRelativeDataDir(t *testing.T) {
	root := t.TempDir()
	setConfigEnv(t, root)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := Save(Config{DataDir: "library"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".zot", ".env"))
	if err != nil {
		t.Fatal(err)
	}
	want := "ZOT_DATA_DIR=" + filepath.Join(root, "library")
	if !strings.Contains(string(data), want) {
		t.Fatalf("saved config does not contain %q:\n%s", want, data)
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
