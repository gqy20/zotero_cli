package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"

	_ "modernc.org/sqlite"
)

func TestRunFindLocalJSONMatchesFullTextAttachmentTerms(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir,
		[]ftsCacheRow{
			{"ATTA1111", "ART67890", "Mixed Survey", "",
				"Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."},
		})

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "speciation genome", "--fulltext", "--json"})
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
	item := data[0].(map[string]any)
	if item["key"] != "ART67890" {
		t.Fatalf("unexpected item payload: %#v", item)
	}
	matchedOn, ok := item["matched_on"].([]any)
	if !ok || len(matchedOn) == 0 {
		t.Fatalf("expected matched_on in item payload: %#v", item)
	}
	found := false
	for _, raw := range matchedOn {
		if raw == "fulltext_attachment" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fulltext_attachment in matched_on: %#v", matchedOn)
	}
}

func TestRunFindLocalJSONIncludesFullTextPreviewWhenSnippetRequested(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed.pdf", "--snippet", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_source"] != "zotero_ft_cache" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	item := data[0].(map[string]any)
	preview, ok := item["full_text_preview"].(string)
	if !ok || !strings.Contains(preview, "Mixed survey full text preview") {
		t.Fatalf("unexpected full_text_preview: %#v", item["full_text_preview"])
	}
	if _, exists := item["attachments"]; exists {
		t.Fatalf("did not expect attachments to be exposed by snippet-only output: %#v", item["attachments"])
	}
}

func TestRunFindLocalJSONUsesMatchedSnippetForFullTextQuery(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir,
		[]ftsCacheRow{
			{"ATTA1111", "ART67890", "Mixed Survey", "",
				"Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."},
		})

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "speciation genome", "--fulltext", "--snippet", "--json"})
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
	item := data[0].(map[string]any)
	preview, ok := item["full_text_preview"].(string)
	if !ok || !strings.Contains(preview, "speciation genome patterns in plants") {
		t.Fatalf("unexpected full_text_preview: %#v", item["full_text_preview"])
	}
	if strings.Contains(preview, "Mixed survey full text preview from zotero cache.") {
		t.Fatalf("expected matched snippet instead of leading preview: %q", preview)
	}
}

func TestRunFindLocalJSONSupportsFullTextIndex(t *testing.T) {
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

	reader, err := backend.NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	item, err := reader.GetItem(context.Background(), "ART67890")
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}
	if _, err := reader.FullTextPreview(context.Background(), item); err != nil {
		t.Fatalf("FullTextPreview() error = %v", err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "speciation genome", "--fulltext", "--snippet", "--json"})
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
	itemData := data[0].(map[string]any)
	if itemData["key"] != "ART67890" {
		t.Fatalf("unexpected item payload: %#v", itemData)
	}
	preview, ok := itemData["full_text_preview"].(string)
	if !ok || !strings.Contains(preview, "speciation genome patterns in plants") {
		t.Fatalf("unexpected full_text_preview: %#v", itemData["full_text_preview"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_source"] != "zotero_ft_cache" || meta["full_text_engine"] != "index_sqlite" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
}

func TestRunFindLocalJSONSupportsFullTextAnyAndPrefixMatching(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir,
		[]ftsCacheRow{
			{"ATTA1111", "ART67890", "Mixed Survey", "",
				"Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."},
			{"ATTB2222", "ARTFULL2", "Prefix Match Article", "",
				"Prefix Match Article discusses genomic species diversity and prefix-based search patterns."},
		})

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "specia genom", "--fulltext", "--fulltext-any", "--json"})
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
	first := data[0].(map[string]any)
	second := data[1].(map[string]any)
	if first["key"] != "ART67890" || second["key"] != "ARTFULL2" {
		t.Fatalf("unexpected ordering or keys: %#v", got["data"])
	}
}

func TestRunFindRejectsFullTextAnyWithoutFullText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "genome", "--fulltext-any"})
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "--fulltext-any requires --fulltext") {
		t.Fatalf("expected fulltext-any usage error, got %q", got)
	}
}
