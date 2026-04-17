package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigInitCreatesFileWhenOnlyEnvConfigExists(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := defaultCLI.stdin
	defaultCLI.stdin = strings.NewReader("user\n123456\nsecret\n\n\ny\nn\n")
	t.Cleanup(func() {
		defaultCLI.stdin = oldStdin
	})
	exitCode := Run([]string{"config", "init"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to be created, stat err=%v", err)
	}
	if !strings.Contains(stdout.String(), "created config at") {
		t.Fatalf("expected success message, got %q", stdout.String())
	}
	for _, want := range []string{
		"https://www.zotero.org/settings/keys",
		"https://www.zotero.org/groups",
		"https://www.zotero.org/support/dev/web_api/v3/basics",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in init output, got %q", want, stdout.String())
		}
	}
}

func TestRunDeleteItemBlockedWhenDeleteDisabled(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	envPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.ReplaceAll(string(content), "ZOT_ALLOW_DELETE=1", "ZOT_ALLOW_DELETE=0")
	if err := os.WriteFile(envPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"delete-item", "ABCD2345", "--if-unmodified-since-version", "8"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "delete operations are disabled") {
		t.Fatalf("expected delete disabled message, got %q", stderr.String())
	}
}

func TestRunCreateItemBlockedWhenWriteDisabled(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	envPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.ReplaceAll(string(content), "ZOT_ALLOW_WRITE=1", "ZOT_ALLOW_WRITE=0")
	if err := os.WriteFile(envPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"create-item", "--data", `{"itemType":"book","title":"My Book"}`, "--if-unmodified-since-version", "41"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "writes are disabled") {
		t.Fatalf("expected write disabled message, got %q", stderr.String())
	}
}

func TestRunConfigValidateJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"config", "validate"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "config-validate" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	if data["library_type"] != "user" {
		t.Fatalf("unexpected library_type: %#v", data["library_type"])
	}

	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	if meta["mode"] != "web" {
		t.Fatalf("unexpected mode meta: %#v", meta["mode"])
	}
	if meta["data_dir_configured"] != false {
		t.Fatalf("unexpected data_dir_configured meta: %#v", meta["data_dir_configured"])
	}
	if meta["config_path"] == "" {
		t.Fatalf("expected config_path meta, got %#v", meta)
	}
}

func TestRunConfigValidateJSONReportsUnavailableLocalReader(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	dataDir := t.TempDir()
	envPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.ReplaceAll(string(content), "ZOT_TIMEOUT_SECONDS=20", "ZOT_TIMEOUT_SECONDS=20\nZOT_DATA_DIR="+dataDir)
	if err := os.WriteFile(envPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"config", "validate"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	if meta["data_dir_configured"] != true {
		t.Fatalf("expected data_dir_configured=true, got %#v", meta["data_dir_configured"])
	}
	if meta["local_reader_available"] != false {
		t.Fatalf("expected local_reader_available=false, got %#v", meta["local_reader_available"])
	}
	if errMsg, _ := meta["local_reader_error"].(string); !strings.Contains(errMsg, "zotero.sqlite") {
		t.Fatalf("expected local_reader_error to mention zotero.sqlite, got %#v", meta["local_reader_error"])
	}
}

func TestRunStatsJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"stats", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "stats" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}
