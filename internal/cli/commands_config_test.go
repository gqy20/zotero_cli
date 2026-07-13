package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	exitCode := Run([]string{"item", "delete", "ABCD2345", "--if-version", "8"})
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
	exitCode := Run([]string{"item", "new", "--data", `{"itemType":"book","title":"My Book"}`, "--if-version", "41"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "writes are disabled") {
		t.Fatalf("expected write disabled message, got %q", stderr.String())
	}
}

func TestRunConfigCheckJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)
	connector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connector/ping" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer connector.Close()
	t.Setenv("ZOT_CONNECTOR_URL", connector.URL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"config", "check", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "config check" {
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
	if meta["zotero_desktop_connector_available"] != true {
		t.Fatalf("expected available desktop connector, got %#v", meta)
	}
	if meta["zotero_desktop_connector_url"] != connector.URL {
		t.Fatalf("unexpected desktop connector URL: %#v", meta["zotero_desktop_connector_url"])
	}
}

func TestRunConfigCheckKeepsSuccessWhenDesktopConnectorUnavailable(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)
	connector := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	connectorURL := connector.URL
	connector.Close()
	t.Setenv("ZOT_CONNECTOR_URL", connectorURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"config", "check", "--json"})
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
	if meta["zotero_desktop_connector_available"] != false {
		t.Fatalf("expected unavailable desktop connector, got %#v", meta)
	}
	errMessage, _ := meta["zotero_desktop_connector_error"].(string)
	if !strings.Contains(errMessage, "start Zotero and try again") || !strings.Contains(errMessage, connectorURL) {
		t.Fatalf("unexpected connector error: %q", errMessage)
	}
}

func TestRunConfigCheckJSONReportsUnavailableLocalReader(t *testing.T) {
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
	exitCode := Run([]string{"config", "check", "--json"})
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
