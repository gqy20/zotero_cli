package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["read_source"] != "web" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
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

func TestRunFindJSONSupportsPaginationAndSortingFlags(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "mixed",
		"--tag", "ai",
		"--start", "1",
		"--sort", "title",
		"--direction", "asc",
		"--json",
	})

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
	if item["key"] != "ART67890" {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestRunFindJSONSupportsQModeAndIncludeTrashed(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "full text",
		"--qmode", "everything",
		"--include-trashed",
		"--json",
	})

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
	if !ok || item["key"] != "TRASH9000" {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
}

func TestRunFindJSONSupportsDateRangeFiltering(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "mixed",
		"--date-after", "2024",
		"--json",
	})

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
	if !ok || item["key"] != "ART12345" {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
}

func TestRunFindJSONSupportsFullDateFiltering(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "mixed",
		"--date-after", "2024-05-01",
		"--date-before", "2024-12-31",
		"--json",
	})
	restoreOutput()

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
	if !ok || item["key"] != "ART12345" {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
}

func TestRunFindJSONSupportsMultipleTagsWithAndSemantics(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "mixed",
		"--tag", "ai",
		"--tag", "survey",
		"--json",
	})

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
	if !ok || item["key"] != "ART67890" {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
}

func TestRunFindJSONSupportsTagAnySemantics(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{
		"find", "mixed",
		"--tag", "classic",
		"--tag", "survey",
		"--tag-any",
		"--json",
	})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
}

func TestRunFindJSONIncludesStructuredFieldsForAgents(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
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

	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}

	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
	if item["version"] != float64(42) {
		t.Fatalf("unexpected version: %#v", item["version"])
	}
	if item["url"] != "https://example.org/attention" {
		t.Fatalf("unexpected url: %#v", item["url"])
	}
	if item["doi"] != "10.5555/attention" {
		t.Fatalf("unexpected doi: %#v", item["doi"])
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

func TestRunFindTextOutputSupportsIncludeFields(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention", "--include-fields", "url,version"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: X42A7DEE",
		"Version: 42",
		"URL: https://example.org/attention",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunFindLocalTextOutputSupportsBibliographicIncludeFields(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention", "--include-fields", "volume,issue,pages"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: ITEM1234",
		"Volume: 37",
		"Issue: 11",
		"Pages: 1234-1248",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunFindLocalTextOutputShowsMatchedOnInFullMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed.pdf", "--full"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: ART67890",
		"Matched On: attachment_filename",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunFindTextOutputSupportsFullMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention", "--full"})
	restoreOutput()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: X42A7DEE",
		"DOI: 10.5555/attention",
		"URL: https://example.org/attention",
		"Tags: transformers, classic",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunFindAllJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "--all", "--json"})

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
	if !ok || item["key"] != "X42A7DEE" {
		t.Fatalf("unexpected item payload: %#v", data[0])
	}
}

func TestRunFindAllowsExplicitEmptyQuery(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "", "--json"})
	restoreOutput()

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
}
