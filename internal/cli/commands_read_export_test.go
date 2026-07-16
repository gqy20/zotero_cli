package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"

	_ "modernc.org/sqlite"
)

func TestRunExportByItemKeyJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "X42A7DEE", "--json"})
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
	if got["command"] != "item export" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected export payload: %#v", got["data"])
	}
	if data["format"] != "bibtex" {
		t.Fatalf("unexpected export format: %#v", data)
	}
}

func TestRunExportFromFindJSONText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)
	input := filepath.Join(t.TempDir(), "find.json")
	if err := os.WriteFile(input, []byte(`{"ok":true,"data":[{"key":"X42A7DEE"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--from", input})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "@article{vaswani2017") {
		t.Fatalf("unexpected export output: %q", got)
	}
}

func TestRunExportBibTeXText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "X42A7DEE", "--as", "bibtex"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	if got := stdout.String(); !strings.Contains(got, "@article{vaswani2017") {
		t.Fatalf("unexpected bibtex output: %q", got)
	}
}

func TestRunExportBibliographyNatureText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ART12345", "ART67890", "--as", "bibliography", "--style", "nature"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "1. Lovelace, A. Primary Article. Nature (2024).") ||
		!strings.Contains(got, "2. Hopper, G. Secondary Article. Nature (2023).") ||
		strings.Contains(got, "<div") {
		t.Fatalf("unexpected bibliography text: %q", got)
	}
}

func TestRunExportBibliographyStreamWritesHTML(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ART12345", "--as", "bibliography", "--style", "nature", "--stream"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, `<div class="csl-bib-body">`) || !strings.Contains(got, "<i>Primary Article</i>") {
		t.Fatalf("unexpected bibliography HTML: %q", got)
	}
}

func TestRunExportStyleRequiresBibliography(t *testing.T) {
	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ART12345", "--as", "bibtex", "--style", "nature"})
	if exitCode != ExitUsage {
		t.Fatalf("expected usage exit code, got %d; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--style requires --as bibliography") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}

func TestRunExportCSLJSONJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "X42A7DEE", "--as", "csljson", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["command"] != "item export" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["format"] != "csljson" {
		t.Fatalf("unexpected export payload: %#v", got["data"])
	}
}

func TestRunExportCSLJSONLocalByItemKey(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, dataDir, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ITEM1234", "--as", "csljson", "--json"})
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
	if got["command"] != "item export" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["format"] != "csljson" {
		t.Fatalf("unexpected export payload: %#v", got["data"])
	}
	payload, ok := data["data"].([]any)
	if !ok || len(payload) != 1 {
		t.Fatalf("unexpected csljson payload: %#v", data["data"])
	}
	item := payload[0].(map[string]any)
	for field, want := range map[string]any{
		"id":              "ITEM1234",
		"title":           "Attention Is All You Need",
		"container-title": "NeurIPS",
		"volume":          "37",
		"issue":           "11",
		"page":            "1234-1248",
		"DOI":             "10.1/example",
		"URL":             "https://example.com/paper",
	} {
		if item[field] != want {
			t.Fatalf("unexpected %s: %#v", field, item[field])
		}
	}
}

func TestRunExportCSLJSONLocalFromFindJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, dataDir, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)
	input := filepath.Join(t.TempDir(), "find.json")
	if err := os.WriteFile(input, []byte(`{"data":[{"key":"ART67890"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--from", input, "--as", "csljson", "--json"})
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
	data := got["data"].(map[string]any)
	payload := data["data"].([]any)
	if len(payload) != 1 {
		t.Fatalf("expected one exported item, got %#v", payload)
	}
	item := payload[0].(map[string]any)
	if item["id"] != "ART67890" {
		t.Fatalf("unexpected from-find csljson payload: %#v", item)
	}
}

func TestRunExportCSLJSONHybridPrefersLocalByItemKey(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, dataDir, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)
	t.Setenv("ZOT_BASE_URL", "http://127.0.0.1:1")

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ITEM1234", "--as", "csljson", "--json"})
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
	data := got["data"].(map[string]any)
	payload := data["data"].([]any)
	item := payload[0].(map[string]any)
	if item["id"] != "ITEM1234" {
		t.Fatalf("unexpected hybrid csljson payload: %#v", item)
	}
}

func TestRunExportCSLJSONHybridFallsBackWhenLocalExportIsTemporarilyUnavailable(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalExportReader{
			exportErr: backend.ErrLocalTemporarilyUnavailable,
		}, nil
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "X42A7DEE", "--as", "csljson", "--json"})
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
}

func TestRunExportCSLJSONHybridPreservesUnexpectedLocalExportError(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalExportReader{
			exportErr: errors.New("local csljson cache corrupted"),
		}, nil
	}

	t.Setenv("ZOT_BASE_URL", "http://127.0.0.1:1")

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "ITEM1234", "--as", "csljson", "--json"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "local csljson cache corrupted") {
		t.Fatalf("expected unexpected local export error, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunExportCSLJSONLocalFromItemArray(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, dataDir, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)
	input := filepath.Join(t.TempDir(), "items.json")
	if err := os.WriteFile(input, []byte(`[{"key":"ART67890"}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--from", input, "--as", "csljson", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data := got["data"].(map[string]any)
	payload := data["data"].([]any)
	if len(payload) != 1 {
		t.Fatalf("unexpected csljson payload: %#v", payload)
	}
	item := payload[0].(map[string]any)
	if item["id"] != "ART67890" {
		t.Fatalf("unexpected query csljson payload: %#v", item)
	}
}

func TestRunExportCSLJSONLocalFromKeyArray(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, dataDir, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)
	input := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(input, []byte(`["ITEM1234","ART67890"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--from", input, "--as", "csljson", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data := got["data"].(map[string]any)
	payload := data["data"].([]any)
	if len(payload) != 2 {
		t.Fatalf("unexpected collection csljson payload: %#v", payload)
	}
	ids := []string{payload[0].(map[string]any)["id"].(string), payload[1].(map[string]any)["id"].(string)}
	if !(ids[0] == "ITEM1234" && ids[1] == "ART67890") {
		t.Fatalf("unexpected collection ids: %#v", ids)
	}
}

func TestRunExportCSLJSONTextWarnsWhenUsingSnapshotFallback(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalExportReader{
			keys: []string{"SNAP1"},
			payload: []map[string]any{
				{"id": "SNAP1", "title": "Snapshot Export"},
			},
			meta: backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "SNAP1", "--as", "csljson"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "using snapshot fallback") {
		t.Fatalf("expected snapshot warning in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"id\": \"SNAP1\"") {
		t.Fatalf("expected export output to include item id, got %q", stdout.String())
	}
}

func TestRunExtractTextLocalJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalTextReader{
			item: domain.Item{
				Key: "ITEM123",
				Attachments: []domain.Attachment{
					{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: "D:/supplement.pdf", Resolved: true},
				},
			},
			text: "full extracted text",
			attachments: []backend.AttachmentFullText{
				{
					Attachment:  domain.Attachment{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					Text:        "full extracted text",
					Source:      "pdfium",
					ContentPath: "D:/cache/ATT123/content.txt",
					ChunksPath:  "D:/cache/ATT123/chunks.json",
				},
				{
					Attachment:  domain.Attachment{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: "D:/supplement.pdf", Resolved: true},
					Text:        "supplement extracted text",
					Source:      "zotero_ft_cache",
					CacheHit:    true,
					ContentPath: "D:/cache/ATT456/content.txt",
				},
			},
			meta: backend.ReadMetadata{ReadSource: "live", FullTextSource: "pdfium", FullTextAttachmentKey: "ATT123"},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"pdf", "text", "ITEM123", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "pdf text" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_source"] != "pdfium" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["content_path"] != "D:/cache/ATT123/content.txt" || data["chunks_path"] != "D:/cache/ATT123/chunks.json" {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	if _, exists := data["text"]; exists {
		t.Fatalf("default local response must not inline full text: %#v", data)
	}
	if data["primary_attachment_key"] != "ATT123" {
		t.Fatalf("unexpected primary_attachment_key: %#v", data["primary_attachment_key"])
	}
	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) != 2 {
		t.Fatalf("unexpected attachments payload: %#v", data["attachments"])
	}
	first, ok := attachments[0].(map[string]any)
	if !ok || first["attachment_key"] != "ATT123" || first["content_path"] != "D:/cache/ATT123/content.txt" || first["total_chars"] != float64(19) {
		t.Fatalf("unexpected first attachment payload: %#v", attachments[0])
	}
	second, ok := attachments[1].(map[string]any)
	if !ok || second["attachment_key"] != "ATT456" || second["content_path"] != "D:/cache/ATT456/content.txt" || second["full_text_cache_hit"] != true {
		t.Fatalf("unexpected second attachment payload: %#v", attachments[1])
	}
}

func TestRunExtractTextLocalJSONOutputControls(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalTextReader{
			item: domain.Item{
				Key: "ITEM123",
				Attachments: []domain.Attachment{
					{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: "D:/supplement.pdf", Resolved: true},
				},
			},
			text: "abstract\nmethods: short primary text\nresults",
			attachments: []backend.AttachmentFullText{
				{
					Attachment: domain.Attachment{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					Text:       "abstract\nmethods: short primary text\nresults",
				},
				{
					Attachment: domain.Attachment{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: "D:/supplement.pdf", Resolved: true},
					Text:       "intro\nMethods: supplementary dataset and processing details\nresults",
				},
			},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"pdf", "text", "ITEM123", "--json", "--attachment", "ATT456", "--grep", "methods", "--max-chars", "20"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta := got["meta"].(map[string]any)
	if meta["truncated"] != true || meta["returned_chars"] != float64(20) {
		t.Fatalf("unexpected meta: %#v", meta)
	}
	filters := meta["filters"].(map[string]any)
	if filters["attachment_key"] != "ATT456" || filters["grep"] != "methods" || filters["max_chars"] != float64(20) {
		t.Fatalf("unexpected filters: %#v", filters)
	}
	data := got["data"].(map[string]any)
	if _, exists := data["text"]; exists {
		t.Fatalf("top-level text must not duplicate attachment text: %#v", data)
	}
	attachments := data["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("expected one filtered attachment, got %#v", attachments)
	}
	attachment := attachments[0].(map[string]any)
	if attachment["attachment_key"] != "ATT456" || attachment["text"] != "intro\nMethods: suppl" || attachment["truncated"] != true {
		t.Fatalf("unexpected attachment payload: %#v", attachment)
	}
}

func TestRunExtractTextLocalJSONPagesOutputControls(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		return stubLocalTextReader{
			item: domain.Item{
				Key: "ITEM123",
				Attachments: []domain.Attachment{
					{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
				},
			},
			pageAttachments: []backend.AttachmentPageText{
				{
					Attachment: domain.Attachment{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					Pages: []backend.PageText{
						{Page: 1, Text: "abstract text"},
						{Page: 2, Text: "methods page two contains sampling and analysis"},
						{Page: 3, Text: "results text"},
					},
					Source: "pymupdf",
				},
			},
			meta: backend.ReadMetadata{ReadSource: "live", FullTextSource: "pymupdf", FullTextAttachmentKey: "ATT123"},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"pdf", "text", "ITEM123", "--json", "--pages", "2", "--grep", "methods", "--max-chars", "14"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta := got["meta"].(map[string]any)
	filters := meta["filters"].(map[string]any)
	if filters["pages"] != "2" || filters["grep"] != "methods" || filters["max_chars"] != float64(14) {
		t.Fatalf("unexpected filters: %#v", filters)
	}
	returnedPages := meta["returned_pages"].([]any)
	if len(returnedPages) != 1 || returnedPages[0] != float64(2) {
		t.Fatalf("unexpected returned_pages: %#v", meta["returned_pages"])
	}
	data := got["data"].(map[string]any)
	if _, exists := data["text"]; exists {
		t.Fatalf("top-level text must not duplicate page text: %#v", data)
	}
	attachments := data["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	pages := attachment["pages"].([]any)
	if len(pages) != 1 || pages[0].(map[string]any)["page"] != float64(2) || pages[0].(map[string]any)["text"] != "methods page t" {
		t.Fatalf("unexpected pages payload: %#v", pages)
	}
}

func TestRunShowTextWarnsWhenUsingSnapshotFallback(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() {
		testCLI.backendNewReader = previousNewReader
	})
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return stubMetadataReader{
			item: domain.Item{Key: "SNAP1", ItemType: "journalArticle", Title: "Snapshot Item"},
			meta: backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "SNAP1"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "using snapshot fallback") {
		t.Fatalf("expected snapshot warning in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Key: SNAP1") {
		t.Fatalf("expected show output to include item key, got %q", stdout.String())
	}
}

func TestRunStatsTextWarnsWhenUsingSnapshotFallback(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() {
		testCLI.backendNewReader = previousNewReader
	})
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return stubMetadataReader{
			stats: backend.LibraryStats{LibraryType: "user", LibraryID: "123456", TotalItems: 2},
			meta:  backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"lib", "stats"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "using snapshot fallback") {
		t.Fatalf("expected snapshot warning in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "items=2") {
		t.Fatalf("expected stats output to include items count, got %q", stdout.String())
	}
}

func TestRunExportFromKeyArrayText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)
	input := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(input, []byte(`["X42A7DEE"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--from", input})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "@article{vaswani2017") {
		t.Fatalf("unexpected export output: %q", got)
	}
}
