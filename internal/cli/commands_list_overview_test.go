package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunOverviewJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"overview", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["command"] != "overview" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	if got["ok"] != true {
		t.Fatalf("expected ok=true, got: %#v", got["ok"])
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got: %#v", got["data"])
	}

	// Verify data sections exist
	for _, key := range []string{"stats", "collections", "tags", "recent_items"} {
		if data[key] == nil {
			t.Fatalf("missing data section: %s", key)
		}
	}

	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta to be a map")
	}

	// Verify meta fields
	if meta["total_items"] == nil {
		t.Fatal("meta missing total_items")
	}
	if meta["index_status"] == nil {
		t.Fatal("meta missing index_status")
	}
}

func TestRunOverviewText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"overview"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Library:",
		"Items:",
		"Collections:",
		"Searches:",
		"Top Collections",
		"Projects",
		"Top Tags",
		"transformers",
		"Recent Items",
		"X42A7DEE",
		"Index:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output; got:\n%s", want, got)
		}
	}
}

func TestRunOverviewHelpShowsUsage(t *testing.T) {
	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"overview", "--help"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	got := stdout.String()
	if !strings.Contains(got, "zot overview") {
		t.Fatalf("expected usage in help output, got: %q", got)
	}
}
