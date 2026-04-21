package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunShowRejectsUnsupportedMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "bogus")

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "X42A7DEE"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "unsupported mode \"bogus\"") {
		t.Fatalf("expected unsupported mode error, got %q", got)
	}
}

func TestRunShowWebRejectsSnippet(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "X42A7DEE", "--snippet"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "show --snippet requires local or hybrid mode with local data") {
		t.Fatalf("expected show snippet error, got %q", got)
	}
}

func TestRunShowJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

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
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["read_source"] != "web" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
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

func TestRunShowLocalJSONIncludesBibliographicFields(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalShowFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234", "--json"})
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

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}

	for field, want := range map[string]any{
		"volume": "37",
		"issue":  "11",
		"pages":  "1234-1248",
	} {
		if data[field] != want {
			t.Fatalf("unexpected %s: %#v", field, data[field])
		}
	}
}

func TestRunShowLocalJSONIncludesFullTextPreviewWhenSnippetRequested(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalShowFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234", "--snippet", "--json"})
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
	if meta["read_source"] != "live" || meta["full_text_source"] != "zotero_ft_cache" || meta["full_text_attachment_key"] != "ATTACHPDF" {
		t.Fatalf("unexpected meta payload: %#v", meta)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	preview, ok := data["full_text_preview"].(string)
	if !ok || !strings.Contains(preview, "Attention is all you need") {
		t.Fatalf("unexpected full_text_preview: %#v", data["full_text_preview"])
	}
}

func TestRunShowTextOutputFormatsAttachmentsClearly(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

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
		"[pdf] attention-is-all-you-need.pdf (PDF12345)",
		"[link] Notion (URL12345)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunShowLocalTextOutputIncludesCollectionsAndResolvedPaths(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalShowFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: ITEM1234",
		"Date: 2024-01-08",
		"Volume: 37",
		"Issue: 11",
		"Pages: 1234-1248",
		"Collections: Machine Learning",
		"Attachments: 2",
		"[pdf] attention.pdf (ATTACHPDF)",
		"path: " + filepath.Join(storageDir, "ATTACHPDF", "attention.pdf"),
		"[link] Web Snapshot (ATTACHURL)",
		"path: unresolved (attachments:snapshots/page.html)",
		"Notes: 1",
		"NOTE1234: Local note summary",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunShowLocalTextOutputIncludesFullTextPreviewWhenSnippetRequested(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalShowFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234", "--snippet"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "Full Text Preview: Attention is all you need") {
		t.Fatalf("expected full text preview in output %q", got)
	}
	if !strings.Contains(got, "Full Text Source: zotero_ft_cache [ATTACHPDF]") {
		t.Fatalf("expected full text source in output %q", got)
	}
}

func TestRunShowLocalAutoDetectsPrefsAndResolvesLinkedAttachmentPaths(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")
	t.Setenv("ZOT_DATA_DIR", "")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalLinkedAttachmentFixture(t, sqlitePath, storageDir)

	baseAttachmentDir := t.TempDir()
	linkedPDF := filepath.Join(baseAttachmentDir, "papers", "linked.pdf")
	if err := os.MkdirAll(filepath.Dir(linkedPDF), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkedPDF, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeZoteroPrefsFixture(t, configRoot, dataDir, baseAttachmentDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEMLINK"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Key: ITEMLINK",
		"[pdf] linked.pdf (ATTLINK)",
		"path: " + linkedPDF,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}

func TestRunRelateLocalJSONShowsExplicitRelations(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalRelateFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"relate", "ITEM1234", "--json"})
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
	data, ok := got["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("unexpected relate payload: %#v", got["data"])
	}
	first := data[0].(map[string]any)
	if first["predicate"] != "dc:relation" {
		t.Fatalf("unexpected first relation: %#v", first)
	}
}

func TestRunRelateLocalTextOutputShowsDirectionAndTarget(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalRelateFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"relate", "ITEM1234"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Item: ITEM1234",
		"Explicit Relations: 2",
		"[dc:relation][incoming] RELA1111  journalArticle  Related Incoming",
		"[dc:relation][outgoing] RELB2222  journalArticle  Related Outgoing",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}
