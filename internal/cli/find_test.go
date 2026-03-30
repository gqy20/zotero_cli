package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setTestConfigDir(t *testing.T, root string) {
	t.Helper()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
}

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

func TestRunShowJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

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

	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) != 2 {
		t.Fatalf("unexpected attachments payload: %#v", data["attachments"])
	}
}

func TestRunShowTextOutputFormatsAttachmentsClearly(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

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
	exitCode := Run([]string{"show", "X42A7DEE"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Attachments: 2",
		"[pdf] attention-is-all-you-need.pdf",
		"[link] Notion",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}
