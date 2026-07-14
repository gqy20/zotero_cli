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
	exitCode := Run([]string{"find", "speciation genome", "--in", "fulltext", "--json"})
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

func TestRunFindLocalJSONReturnsIndexedMatchedChunkWhenSnippetRequested(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir, []ftsCacheRow{
		{"ATTA1111", "ART67890", "Mixed Survey", "mixed.pdf",
			"Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."},
	})

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", `"Mixed survey"`, "--in", "fulltext", "--snippet", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_engine"] != "index_sqlite" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	item := data[0].(map[string]any)
	if _, exists := item["full_text_preview"]; exists {
		t.Fatalf("matched context must not be duplicated as full_text_preview: %#v", item)
	}
	matched, ok := item["matched_chunk"].(map[string]any)
	if !ok || !strings.Contains(matched["context"].(string), "Mixed survey full text preview") {
		t.Fatalf("unexpected matched evidence: %#v", item["matched_chunk"])
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
	exitCode := Run([]string{"find", "speciation genome", "--in", "fulltext", "--snippet", "--json"})
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
	if _, exists := item["full_text_preview"]; exists {
		t.Fatalf("matched context must not be duplicated as full_text_preview: %#v", item)
	}
	matched, ok := item["matched_chunk"].(map[string]any)
	if !ok || matched["page"] != float64(1) || !strings.Contains(matched["context"].(string), "speciation genome") {
		t.Fatalf("unexpected matched evidence: %#v", item["matched_chunk"])
	}
	if _, exists := matched["text"]; exists {
		t.Fatalf("raw matched chunk must not be serialized with its context: %#v", matched)
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
	exitCode := Run([]string{"find", "speciation genome", "--in", "fulltext", "--snippet", "--json"})
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
	if _, exists := itemData["full_text_preview"]; exists {
		t.Fatalf("matched context must not be duplicated as full_text_preview: %#v", itemData)
	}
	matched, ok := itemData["matched_chunk"].(map[string]any)
	if !ok || !strings.Contains(matched["context"].(string), "speciation genome patterns in plants") {
		t.Fatalf("unexpected matched evidence: %#v", itemData["matched_chunk"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["full_text_source"] != "zotero_ft_cache" || meta["full_text_engine"] != "index_sqlite" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
}

func TestRunFindLocalJSONSupportsNativeFTSOrAndPrefixMatching(t *testing.T) {
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
	exitCode := Run([]string{"find", "specia* OR genom*", "--in", "fulltext", "--json"})
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
	keys := map[any]bool{first["key"]: true, second["key"]: true}
	if !keys["ART67890"] || !keys["ARTFULL2"] {
		t.Fatalf("unexpected keys: %#v", got["data"])
	}
}

func TestRunFindFullTextAppliesOffsetAfterNativeRanking(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir, []ftsCacheRow{
		{"ATTA1111", "ART67890", "Mixed Survey", "", "pagination target target target"},
		{"ATTB2222", "ARTFULL2", "Prefix Match Article", "", "pagination target"},
	})

	stdout, stderr := captureOutput(t)
	if code := Run([]string{"find", "pagination", "--in", "fulltext", "--limit", "10", "--json"}); code != 0 {
		t.Fatalf("all results code=%d stderr=%q", code, stderr.String())
	}
	var all struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &all); err != nil || len(all.Data) != 2 {
		t.Fatalf("all results=%#v err=%v", all, err)
	}

	stdout, stderr = captureOutput(t)
	if code := Run([]string{"find", "pagination", "--in", "fulltext", "--offset", "1", "--limit", "1", "--json"}); code != 0 {
		t.Fatalf("paged results code=%d stderr=%q", code, stderr.String())
	}
	var page struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &page); err != nil || len(page.Data) != 1 || page.Data[0].Key != all.Data[1].Key {
		t.Fatalf("paged results=%#v want second=%#v err=%v", page, all.Data[1], err)
	}
}

func TestRunFindFullTextFiltersBeforeApplyingLimit(t *testing.T) {
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
	buildGlobalFTSCacheForTest(t, dataDir, []ftsCacheRow{
		{"ATTA1111", "ART67890", "Mixed Survey", "", "filtertarget filtertarget filtertarget"},
		{"ATTB2222", "ARTFULL2", "Prefix Match Article", "", "filtertarget"},
	})

	stdout, stderr := captureOutput(t)
	if code := Run([]string{"find", "filtertarget", "--in", "fulltext", "--tag", "genomics", "--limit", "1", "--json"}); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var got struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || len(got.Data) != 1 || got.Data[0].Key != "ARTFULL2" {
		t.Fatalf("filtered results=%#v err=%v", got, err)
	}
}

func TestRunFindRejectsInvalidSearchScope(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "genome", "--in", "all"})
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "--in must be metadata or fulltext") {
		t.Fatalf("expected search scope usage error, got %q", got)
	}
}
