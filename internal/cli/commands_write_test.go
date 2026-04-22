package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCreateItemJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"create-item", "--data", `{"itemType":"book","title":"My Book"}`, "--if-unmodified-since-version", "41", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "create-item" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}

func TestRunUpdateItemText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"update-item", "ABCD2345", "--data", `{"title":"Updated Title"}`, "--if-unmodified-since-version", "7"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "updated item ABCD2345") {
		t.Fatalf("unexpected update output: %q", got)
	}
}

func TestRunDeleteItemText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"delete-item", "ABCD2345", "--if-unmodified-since-version", "8", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted item ABCD2345") {
		t.Fatalf("unexpected delete output: %q", got)
	}
}

func TestRunAddTagJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"add-tag", "--items", "ITEMA001,ITEMA002", "--tag", "paper", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "add-tag" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(53) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunRemoveTagText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"remove-tag", "--items", "ITEMA001,ITEMA002", "--tag", "ai"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, `removed tag "ai" on 2 items at library version 53`) {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunCreateItemFromFileText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(payloadPath, []byte(`{"itemType":"book","title":"From File"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"create-item", "--from-file", payloadPath, "--if-unmodified-since-version", "41"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "created item NEWA1234") || !strings.Contains(got, "library version 42") {
		t.Fatalf("unexpected create output: %q", got)
	}
}

func TestRunUpdateItemFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "patch.json")
	if err := os.WriteFile(payloadPath, []byte(`{"title":"Updated From File"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"update-item", "ABCD2345", "--from-file", payloadPath, "--if-unmodified-since-version", "7", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(8) {
		t.Fatalf("unexpected update payload: %#v", got["data"])
	}
}

func TestRunCreateCollectionText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"create-collection", "--data", `{"name":"New Collection"}`, "--if-unmodified-since-version", "10"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "created collection COLLNEW1") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunUpdateCollectionFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "collection.json")
	if err := os.WriteFile(payloadPath, []byte(`{"key":"COLL1234","version":11,"name":"Renamed Collection"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"update-collection", "COLL1234", "--from-file", payloadPath, "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(12) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunDeleteCollectionText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"delete-collection", "COLL1234", "--if-unmodified-since-version", "12", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted collection COLL1234") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunCreateSearchText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"create-search", "--data", `{"name":"Unread PDFs","conditions":[{"condition":"itemType","operator":"is","value":"attachment"}]}`, "--if-unmodified-since-version", "17"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "created search SCH67890") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunUpdateSearchFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "search.json")
	if err := os.WriteFile(payloadPath, []byte(`{"key":"SCH12345","version":21,"name":"Important PDFs"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"update-search", "SCH12345", "--from-file", payloadPath, "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(49) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunDeleteSearchText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"delete-search", "SCH12345", "--if-unmodified-since-version", "22", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted search SCH12345") {
		t.Fatalf("unexpected output: %q", got)
	}
}
