package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunFindJSON(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	t.Setenv("ZOT_BASE_URL", "http://127.0.0.1:1")

	configDir := filepath.Join(configRoot, "zotcli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configJSON := `{
  "mode": "web",
  "library_type": "user",
  "library_id": "123456",
  "api_key": "secret",
  "style": "apa",
  "locale": "en-US",
  "timeout_seconds": 20
}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention", "--json"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", got["ok"])
	}

	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
}

func TestRunShowJSON(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)

	configDir := filepath.Join(configRoot, "zotcli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configJSON := `{
  "mode": "web",
  "library_type": "user",
  "library_id": "123456",
  "api_key": "secret",
  "style": "apa",
  "locale": "en-US",
  "timeout_seconds": 20
}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "X42A7DEE", "--json"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}

	if data["key"] != "X42A7DEE" {
		t.Fatalf("unexpected key: %#v", data["key"])
	}

	if data["doi"] != "10.48550/arXiv.1706.03762" {
		t.Fatalf("unexpected doi: %#v", data["doi"])
	}
}
