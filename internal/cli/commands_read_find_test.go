package cli

import (
	"context"
	"encoding/json"
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

func TestRunFindRejectsLocalModeWithoutDataDir(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "local mode requires data_dir") {
		t.Fatalf("expected local mode error, got %q", got)
	}
}

func TestRemoteReadCommandsRejectLocalMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "cite", args: []string{"cite", "X42A7DEE"}, want: "web API commands are not available in local mode; use web or hybrid mode"},
		{name: "export", args: []string{"export", "--item-key", "X42A7DEE"}, want: "web API commands are not available in local mode; use web or hybrid mode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr := captureOutput(t)
			exitCode := Run(tc.args)
			if exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
			}
			if got := stderr.String(); !strings.Contains(got, tc.want) {
				t.Fatalf("expected local-mode guard %q, got %q", tc.want, got)
			}
		})
	}
}

func TestRunFindHybridFallsBackToWebWithoutDataDir(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "hybrid")

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
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["read_source"] != "web" {
		t.Fatalf("unexpected meta payload: %#v", got["meta"])
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
}

func TestRunFindJSONReportsSnapshotReadSource(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() {
		testCLI.backendNewReader = previousNewReader
	})
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return stubMetadataReader{
			items: []domain.Item{{Key: "SNAP1", ItemType: "journalArticle", Title: "Snapshot Item"}},
			meta:  backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--json"})
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
	if meta["read_source"] != "snapshot" || meta["sqlite_fallback"] != true {
		t.Fatalf("unexpected snapshot meta: %#v", meta)
	}
}

func TestRunFindTextWarnsWhenUsingSnapshotFallback(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() {
		testCLI.backendNewReader = previousNewReader
	})
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return stubMetadataReader{
			items: []domain.Item{{Key: "SNAP1", ItemType: "journalArticle", Title: "Snapshot Item"}},
			meta:  backend.ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true},
		}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "using snapshot fallback") {
		t.Fatalf("expected snapshot warning in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "SNAP1") {
		t.Fatalf("expected find output to include item, got %q", stdout.String())
	}
}

func TestRunFindLocalJSONUsesSnapshotFallbackUnderRealLock(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")
	t.Setenv("ZOT_LOCAL_BUSY_TIMEOUT_MS", "25")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalFindFixture(t, dataDir, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stopHelper := startSQLiteLockHelper(t, sqlitePath)
	defer stopHelper()

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--json"})
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
	if meta["read_source"] != "snapshot" || meta["sqlite_fallback"] != true {
		t.Fatalf("unexpected snapshot meta: %#v", meta)
	}
}

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

func TestRunFindLocalJSONFiltersNonTopItemsByDefault(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed", "--json"})
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
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
	for _, raw := range data {
		item := raw.(map[string]any)
		itemType, _ := item["item_type"].(string)
		if itemType == "attachment" || itemType == "note" || itemType == "annotation" {
			t.Fatalf("expected primary items only, got %#v", item)
		}
	}
}

func TestRunFindLocalJSONSupportsItemTypeAndLimit(t *testing.T) {
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
	item := data[0].(map[string]any)
	if item["key"] != "ART67890" || item["item_type"] != "journalArticle" {
		t.Fatalf("unexpected item payload: %#v", item)
	}
}

func TestRunFindLocalJSONMatchesAttachmentFilename(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed.pdf", "--json"})
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
		if raw == "attachment_filename" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected attachment_filename in matched_on: %#v", matchedOn)
	}
}

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

func TestRunFindLocalJSONMatchesLinkedAttachmentPathFromPrefs(t *testing.T) {
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
	buildLocalLinkedAttachmentFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)

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
	exitCode := Run([]string{"find", "papers/linked.pdf", "--json"})
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
	if item["key"] != "ITEMLINK" {
		t.Fatalf("unexpected item payload: %#v", item)
	}
}

func TestRunFindLocalJSONSupportsHasPDFAndAttachmentTypeFilters(t *testing.T) {
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
	exitCode := Run([]string{"find", "--all", "--has-pdf", "--attachment-type", "application/pdf", "--json"})
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
}

func TestRunFindLocalJSONSupportsAttachmentPathFilterFromPrefs(t *testing.T) {
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
	buildLocalLinkedAttachmentFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)

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
	exitCode := Run([]string{"find", "--all", "--attachment-path", "papers/linked.pdf", "--attachment-name", "linked.pdf", "--json"})
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
	if item["key"] != "ITEMLINK" {
		t.Fatalf("unexpected item payload: %#v", item)
	}
}

func TestRunFindLocalAllJSON(t *testing.T) {
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
	exitCode := Run([]string{"find", "--all", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 4 {
		t.Fatalf("unexpected data payload: %#v", got["data"])
	}
}

func TestRunFindLocalJSONSupportsMultipleTagsWithAndSemantics(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed", "--tag", "ai", "--tag", "survey", "--json"})
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
}

func TestRunFindLocalJSONSupportsTagAnySemantics(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed", "--tag", "classic", "--tag", "survey", "--tag-any", "--json"})
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

func TestRunFindLocalJSONSupportsDateRangeFiltering(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed", "--date-after", "2024", "--json"})
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
}

func TestRunFindLocalJSONSupportsSortingAndPagination(t *testing.T) {
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
	exitCode := Run([]string{"find", "mixed", "--sort", "title", "--direction", "asc", "--start", "1", "--json"})
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
}

func TestRunFindLocalRejectsQMode(t *testing.T) {
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

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--qmode", "everything"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "local does not support find --qmode") {
		t.Fatalf("expected qmode local error, got %q", got)
	}
}

func TestRunFindLocalRejectsIncludeTrashed(t *testing.T) {
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

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "mixed", "--include-trashed"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "local does not support find --include-trashed") {
		t.Fatalf("expected include-trashed local error, got %q", got)
	}
}

func TestRunFindWebRejectsAttachmentFilters(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention", "--attachment-name", "attention.pdf"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "web does not support find attachment filters") {
		t.Fatalf("expected web attachment filter error, got %q", got)
	}
}

func TestRunFindWebRejectsFullText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "speciation", "--fulltext"})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "web does not support find --fulltext") {
		t.Fatalf("expected web fulltext error, got %q", got)
	}
}
