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

func TestRunCiteJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"cite", "X42A7DEE", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["command"] != "cite" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	if data["text"] != "(Vaswani, 2017)" {
		t.Fatalf("unexpected citation text: %#v", data["text"])
	}
	if data["format"] != "citation" {
		t.Fatalf("unexpected format: %#v", data["format"])
	}
}

func TestRunCiteBibText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"cite", "X42A7DEE", "--format", "bib"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "Vaswani, A. (2017). Attention Is All You Need.") {
		t.Fatalf("unexpected bib text: %q", got)
	}
}

func TestRunExportByItemKeyJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--item-key", "X42A7DEE", "--json"})
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
	if got["command"] != "export" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected export payload: %#v", got["data"])
	}
	if data["format"] != "bib" {
		t.Fatalf("unexpected export format: %#v", data)
	}
}

func TestRunExportByQueryText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "mixed", "--limit", "1"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "Lovelace, A. (2024). Primary Article.") {
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
	exitCode := Run([]string{"export", "--item-key", "X42A7DEE", "--format", "bibtex"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	if got := stdout.String(); !strings.Contains(got, "@article{vaswani2017") {
		t.Fatalf("unexpected bibtex output: %q", got)
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
	exitCode := Run([]string{"export", "--item-key", "X42A7DEE", "--format", "csljson", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["command"] != "export" {
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
	exitCode := Run([]string{"export", "--item-key", "ITEM1234", "--format", "csljson", "--json"})
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
	if got["command"] != "export" {
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
	exitCode := Run([]string{"export", "--item-key", "ITEM1234", "--format", "csljson", "--json"})
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
	exitCode := Run([]string{"export", "--item-key", "X42A7DEE", "--format", "csljson", "--json"})
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

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--item-key", "ITEM1234", "--format", "csljson", "--json"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "local csljson cache corrupted") {
		t.Fatalf("expected unexpected local export error, got %q", stderr.String())
	}
}

func TestRunExportCSLJSONLocalByQuery(t *testing.T) {
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
	exitCode := Run([]string{"export", "mixed", "--limit", "1", "--format", "csljson", "--json"})
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

func TestRunExportCSLJSONLocalByCollection(t *testing.T) {
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
	exitCode := Run([]string{"export", "--collection", "COLL1234", "--format", "csljson", "--json"})
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
	exitCode := Run([]string{"export", "--item-key", "SNAP1", "--format", "csljson"})
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
					Attachment: domain.Attachment{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: "D:/paper.pdf", Resolved: true},
					Text:       "full extracted text",
					Source:     "pdfium",
				},
				{
					Attachment: domain.Attachment{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: "D:/supplement.pdf", Resolved: true},
					Text:       "supplement extracted text",
					Source:     "zotero_ft_cache",
					CacheHit:   true,
				},
			},
			meta: backend.ReadMetadata{ReadSource: "live", FullTextSource: "pdfium", FullTextAttachmentKey: "ATT123"},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"extract-text", "ITEM123", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "extract-text" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_source"] != "pdfium" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["text"] != "full extracted text" {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	if data["primary_attachment_key"] != "ATT123" {
		t.Fatalf("unexpected primary_attachment_key: %#v", data["primary_attachment_key"])
	}
	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) != 2 {
		t.Fatalf("unexpected attachments payload: %#v", data["attachments"])
	}
	first, ok := attachments[0].(map[string]any)
	if !ok || first["attachment_key"] != "ATT123" || first["text"] != "full extracted text" {
		t.Fatalf("unexpected first attachment payload: %#v", attachments[0])
	}
	second, ok := attachments[1].(map[string]any)
	if !ok || second["attachment_key"] != "ATT456" || second["text"] != "supplement extracted text" || second["full_text_cache_hit"] != true {
		t.Fatalf("unexpected second attachment payload: %#v", attachments[1])
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
	exitCode := Run([]string{"stats"})
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

func TestRunRelateTextWarnsWhenUsingSnapshotFallback(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() {
		testCLI.backendNewReader = previousNewReader
	})
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return stubMetadataReader{
			relations: []domain.Relation{{Predicate: "dc:relation", Direction: "outgoing", Target: domain.ItemRef{Key: "SNAP2"}}},
			meta:      backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"relate", "SNAP1"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "using snapshot fallback") {
		t.Fatalf("expected snapshot warning in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Explicit Relations: 1") {
		t.Fatalf("expected relate output to include relation count, got %q", stdout.String())
	}
}

func TestRunExportByCollectionText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"export", "--collection", "COLL1234"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Lovelace, A. (2024). Primary Article.",
		"Hopper, G. (2023). Secondary Article.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output %q", want, got)
		}
	}
}
