package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunStatsHybridModeUsesRemoteClient(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

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
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["read_source"] != "web" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	if got["command"] != "stats" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}

func TestRunStatsLocalModeUsesLocalLibrary(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalStatsFixture(t, filepath.Join(dataDir, "zotero.sqlite"))
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"stats", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["read_source"] != "live" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	if got["command"] != "stats" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	for field, want := range map[string]any{
		"library_type":      "user",
		"library_id":        "123456",
		"total_items":       float64(3),
		"total_collections": float64(2),
		"total_searches":    float64(1),
	} {
		if data[field] != want {
			t.Fatalf("unexpected %s: %#v", field, data[field])
		}
	}
}

func TestRunStatsHybridModePrefersLocalLibrary(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalStatsFixture(t, filepath.Join(dataDir, "zotero.sqlite"))
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"stats", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data := got["data"].(map[string]any)
	if data["total_items"] != float64(3) || data["total_collections"] != float64(2) || data["total_searches"] != float64(1) {
		t.Fatalf("unexpected stats payload: %#v", data)
	}
}
