package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	oldVersion := version
	oldCommit := commit
	oldBuildDate := buildDate

	version = "v1.2.3"
	commit = "abc1234"
	buildDate = "2026-03-30T22:30:00Z"
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		buildDate = oldBuildDate
	})

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"version"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	got := stdout.String()
	for _, want := range []string{
		"zot v1.2.3",
		"commit: abc1234",
		"built: 2026-03-30T22:30:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunFindJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	t.Setenv("ZOT_BASE_URL", "http://127.0.0.1:1")
	writeTestConfig(t, configRoot)

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

	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}

	if item["item_type"] != "conferencePaper" {
		t.Fatalf("unexpected item type: %#v", item["item_type"])
	}
}

func TestRunFindJSONFiltersNonTopItemsByDefault(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--json"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].([]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	if len(data) != 2 {
		t.Fatalf("expected only top-level items, got %#v", got["data"])
	}
	for _, raw := range data {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("unexpected item payload: %#v", raw)
		}
		itemType, _ := item["item_type"].(string)
		if itemType == "attachment" || itemType == "note" {
			t.Fatalf("expected attachment/note to be filtered out, got %#v", item)
		}
	}
}

func TestRunFindJSONSupportsItemTypeAndLimit(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--item-type", "journalArticle", "--limit", "1", "--json"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}

	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
	if item["item_type"] != "journalArticle" {
		t.Fatalf("unexpected item type: %#v", item["item_type"])
	}
}

func TestRunFindTextOutputShowsOnlyTopItems(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 visible lines, got %d: %q", len(lines), stdout.String())
	}
	if strings.Contains(stdout.String(), "Attachment PDF") || strings.Contains(stdout.String(), "My note") {
		t.Fatalf("unexpected non-top-level items in output: %q", stdout.String())
	}
}
