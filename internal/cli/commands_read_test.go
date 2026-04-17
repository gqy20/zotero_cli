package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"

	_ "modernc.org/sqlite"
)

type stubMetadataReader struct {
	items     []domain.Item
	item      domain.Item
	relations []domain.Relation
	stats     backend.LibraryStats
	meta      backend.ReadMetadata
}

func (r stubMetadataReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	return append([]domain.Item(nil), r.items...), nil
}

func (r stubMetadataReader) GetItem(context.Context, string) (domain.Item, error) {
	return r.item, nil
}

func (r stubMetadataReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return append([]domain.Relation(nil), r.relations...), nil
}

func (r stubMetadataReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return r.stats, nil
}

func (r stubMetadataReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

type stubLocalExportReader struct {
	keys    []string
	payload []map[string]any
	meta    backend.ReadMetadata
}

type stubLocalTextReader struct {
	item        domain.Item
	text        string
	attachments []backend.AttachmentFullText
	meta        backend.ReadMetadata
}

func (r stubLocalExportReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	items := make([]domain.Item, 0, len(r.keys))
	for _, key := range r.keys {
		items = append(items, domain.Item{Key: key})
	}
	return items, nil
}

func (r stubLocalExportReader) GetItem(context.Context, string) (domain.Item, error) {
	return domain.Item{}, nil
}

func (r stubLocalExportReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return nil, nil
}

func (r stubLocalExportReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}

func (r stubLocalExportReader) CollectionItemKeys(context.Context, string, int) ([]string, error) {
	return append([]string(nil), r.keys...), nil
}

func (r stubLocalExportReader) ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error) {
	return append([]map[string]any(nil), r.payload...), nil
}

func (r stubLocalExportReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

func (r stubLocalTextReader) GetItem(context.Context, string) (domain.Item, error) {
	return r.item, nil
}

func (r stubLocalTextReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	return nil, nil
}

func (r stubLocalTextReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return nil, nil
}

func (r stubLocalTextReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}

func (r stubLocalTextReader) ExtractItemFullText(context.Context, domain.Item) (string, error) {
	return r.text, nil
}

func (r stubLocalTextReader) ExtractItemAttachmentTexts(context.Context, domain.Item) (backend.ItemFullTextResult, error) {
	return backend.ItemFullTextResult{
		Text:                 r.text,
		PrimaryAttachmentKey: r.meta.FullTextAttachmentKey,
		Attachments:          append([]backend.AttachmentFullText(nil), r.attachments...),
	}, nil
}

func (r stubLocalTextReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

func TestHoldSQLiteExclusiveLockHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SQLITE_LOCK_HELPER") != "1" {
		return
	}
	dbPath := os.Getenv("LOCK_DB_PATH")
	readyPath := os.Getenv("LOCK_READY_PATH")
	releasePath := os.Getenv("LOCK_RELEASE_PATH")
	if dbPath == "" || readyPath == "" || releasePath == "" {
		os.Exit(2)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA locking_mode=EXCLUSIVE;`); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`BEGIN EXCLUSIVE;`); err != nil {
		panic(err)
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		panic(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(releasePath); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := db.Exec(`ROLLBACK;`); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func startSQLiteLockHelper(t *testing.T, sqlitePath string) func() {
	t.Helper()

	readyPath := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	cmd := exec.Command(os.Args[0], "-test.run=TestHoldSQLiteExclusiveLockHelper")
	cmd.Env = append(os.Environ(),
		"GO_WANT_SQLITE_LOCK_HELPER=1",
		"LOCK_DB_PATH="+sqlitePath,
		"LOCK_READY_PATH="+readyPath,
		"LOCK_RELEASE_PATH="+releasePath,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start lock helper: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("lock helper did not become ready")
		}
		time.Sleep(50 * time.Millisecond)
	}

	return func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o600)
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("lock helper wait: %v", err)
			}
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("lock helper did not exit")
		}
	}
}

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
	buildLocalFindFixture(t, sqlitePath, storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

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

func TestRunFindLocalJSONSupportsExperimentalFullTextIndex(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")
	t.Setenv("ZOT_EXPERIMENTAL_FTS", "1")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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

func buildLocalShowFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ATTACHPDF', 1, 2), (3, 'ATTACHURL', 1, 2), (4, 'NOTE1234', 1, 2);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'publicationTitle'), (4, 'DOI'), (5, 'url'), (6, 'filename'), (7, 'note'), (8, 'volume'), (9, 'issue'), (10, 'pages');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Attention Is All You Need'), (2, '2024-01-08 2024-01-08 00:00:00'), (3, 'NeurIPS'), (4, '10.1/example'), (5, 'https://example.com/paper'), (6, 'attention.pdf'), (7, 'Web Snapshot'), (8, '<p>Local note summary</p>'), (9, '37'), (10, '11'), (11, '1234-1248');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (1, 3, 3), (1, 4, 4), (1, 5, 5), (1, 8, 9), (1, 9, 10), (1, 10, 11), (2, 1, 1), (2, 6, 6), (3, 1, 7), (4, 7, 8);`,
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Ashish', 'Vaswani', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0);`,
		`INSERT INTO tags(tagID, name) VALUES (1, 'transformers');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 0, 'storage:attention.pdf'), (3, 1, 'text/html', 3, 'attachments:snapshots/page.html');`,
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (4, 1, '<p>Local note summary</p>', 'Local note');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	attachmentDir := filepath.Join(storageDir, "ATTACHPDF")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "attention.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, ".zotero-ft-cache"), []byte("Attention is all you need full text preview from zotero cache."), 0o600); err != nil {
		t.Fatal(err)
	}
}

func buildLocalRelateFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE relationPredicates (predicateID INTEGER PRIMARY KEY, predicate TEXT);`,
		`CREATE TABLE itemRelations (itemID INTEGER, predicateID INTEGER, object TEXT);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'RELA1111', 1, 1), (3, 'RELB2222', 1, 1);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Primary Item'), (2, 'Related Incoming'), (3, 'Related Outgoing');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (2, 1, 2), (3, 1, 3);`,
		`INSERT INTO relationPredicates(predicateID, predicate) VALUES (1, 'dc:relation');`,
		`INSERT INTO itemRelations(itemID, predicateID, object) VALUES (1, 1, 'http://zotero.org/users/123456/items/RELB2222'), (2, 1, 'http://zotero.org/users/123456/items/ITEM1234');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func buildLocalLinkedAttachmentFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEMLINK', 3, 1), (2, 'ATTLINK', 1, 2);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'filename');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Linked PDF Item'), (2, '2024-06-01'), (3, 'linked.pdf');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (2, 1, 1), (2, 3, 3);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 2, 'attachments:papers/linked.pdf');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func writeZoteroPrefsFixture(t *testing.T, root string, dataDir string, baseAttachmentDir string) {
	t.Helper()

	prefsPath := filepath.Join(root, "Zotero", "Zotero", "Profiles", "abcd.default", "prefs.js")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`user_pref("extensions.zotero.baseAttachmentPath", "` + strings.ReplaceAll(baseAttachmentDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.dataDir", "` + strings.ReplaceAll(dataDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.saveRelativeAttachmentPath", true);`,
		"",
	}, "\n")
	if err := os.WriteFile(prefsPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func buildLocalStatsFixture(t *testing.T, sqlitePath string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE savedSearches (savedSearchID INTEGER PRIMARY KEY, savedSearchName TEXT);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'book');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ART67890', 3, 1), (3, 'BOOK1234', 2, 2);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning'), (2, 'COLL5678', 'Chestnut');`,
		`INSERT INTO savedSearches(savedSearchID, savedSearchName) VALUES (1, 'Unread PDFs');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func buildLocalFindFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
		`CREATE TABLE fulltextItems (itemID INTEGER PRIMARY KEY, indexedChars INTEGER, totalChars INTEGER, indexedPages INTEGER, totalPages INTEGER, version INTEGER, synced INTEGER);`,
		`CREATE TABLE fulltextWords (wordID INTEGER PRIMARY KEY, word TEXT);`,
		`CREATE TABLE fulltextItemWords (wordID INTEGER, itemID INTEGER);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'book'), (3, 'attachment'), (4, 'note'), (5, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ART67890', 3, 1), (3, 'BOOK1234', 2, 2), (4, 'ATTA1111', 1, 3), (5, 'NOTE1111', 1, 4), (6, 'ANNO1111', 1, 5), (7, 'ARTFULL2', 4, 1), (8, 'ATTB2222', 1, 3);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'publicationTitle'), (4, 'DOI'), (5, 'url'), (6, 'filename'), (7, 'note'), (8, 'volume'), (9, 'issue'), (10, 'pages');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Attention Is All You Need'), (2, '2024-01-08 2024-01-08 00:00:00'), (3, 'NeurIPS'), (4, '10.1/example'), (5, 'https://example.com/paper'), (6, 'Mixed Survey'), (7, '2024-05-03'), (8, 'Mixed Book'), (9, '2023'), (10, 'mixed.pdf'), (11, '<p>Mixed note</p>'), (12, 'Mixed Attachment'), (13, '37'), (14, '11'), (15, '1234-1248'), (16, '29'), (17, '20'), (18, 'R1094-R1103'), (19, 'Prefix Match Article'), (20, '2024-06-07'), (21, 'prefix.pdf'), (22, 'Prefix Attachment');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (1, 3, 3), (1, 4, 4), (1, 5, 5), (1, 8, 13), (1, 9, 14), (1, 10, 15), (2, 1, 6), (2, 2, 7), (2, 8, 16), (2, 9, 17), (2, 10, 18), (3, 1, 8), (3, 2, 9), (4, 1, 12), (4, 6, 10), (5, 7, 11), (7, 1, 19), (7, 2, 20), (8, 1, 22), (8, 6, 21);`,
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Ashish', 'Vaswani', 0), (2, 'Jane', 'Roe', 0), (3, 'John', 'Doe', 0), (4, 'Alex', 'Smith', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0), (2, 2, 1, 0), (3, 3, 1, 0), (7, 4, 1, 0);`,
		`INSERT INTO tags(tagID, name) VALUES (1, 'transformers'), (2, 'ai'), (3, 'survey'), (4, 'classic'), (5, 'genomics');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1), (2, 2), (2, 3), (3, 4), (7, 5);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning'), (2, 'COLL5678', 'Books');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1), (1, 2), (2, 3);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (4, 2, 'application/pdf', 0, 'storage:mixed.pdf'), (8, 7, 'text/plain', 0, 'storage:prefix.pdf');`,
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (5, 2, '<p>Mixed note</p>', 'Mixed note');`,
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (6, 4, 0, '', '', '', '#ffd400', '1', '00001|00000|00000', '{"pageIndex":0}', 0);`,
		`INSERT INTO fulltextItems(itemID, indexedPages, totalPages, version, synced) VALUES (4, 5, 5, 1, 1), (8, 4, 4, 1, 1);`,
		`INSERT INTO fulltextWords(wordID, word) VALUES (1, 'speciation'), (2, 'genome'), (3, 'genomic'), (4, 'species');`,
		`INSERT INTO fulltextItemWords(wordID, itemID) VALUES (1, 4), (2, 4), (3, 8), (4, 8);`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	attachmentDir := filepath.Join(storageDir, "ATTA1111")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "mixed.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, ".zotero-ft-cache"), []byte("Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."), 0o600); err != nil {
		t.Fatal(err)
	}
	attachmentDir = filepath.Join(storageDir, "ATTB2222")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "prefix.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
}

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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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
	buildLocalFindFixture(t, filepath.Join(dataDir, "zotero.sqlite"), storageDir)
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

// --- Annotation tests (TDD: write failing tests first) ---

func buildLocalAnnotationFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
			itemID INTEGER PRIMARY KEY,
			parentItemID INT NOT NULL,
			type INT NOT NULL,
			authorName TEXT,
			text TEXT,
			comment TEXT,
			color TEXT,
			pageLabel TEXT,
			sortIndex TEXT NOT NULL,
			position TEXT NOT NULL,
			isExternal INT NOT NULL
		);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		// itemTypes
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		// items: 1=article, 2=PDF attachment, 3=note
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ATTACHPDF', 1, 2), (3, 'NOTE1234', 1, 2), (10, 'ANNO1', 1, 3), (11, 'ANNO2', 1, 3), (12, 'ANNO3', 1, 3), (13, 'ANNO4', 1, 3);`,
		// fields + data for article title
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Annotated Paper');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1);`,
		// creator
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Jane', 'Doe', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0);`,
		// tag
		`INSERT INTO tags(tagID, name) VALUES (1, 'genomics');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1);`,
		// collection
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1', 'Research');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1);`,
		// PDF attachment (parentItemID=1 means child of article)
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 0, 'storage:paper.pdf');`,
		// note
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (3, 1, '<p>A regular note</p>', 'Note');`,
		// === Annotations on the PDF attachment (parentItemID = 2, the PDF's itemID) ===
		// Annotation 1: highlight with comment on page 2
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (10, 2, 1, '', 'Key finding about genome assembly', 'This is important for the discussion section', '#ffd400', '2', '00001|00000|00000', '{"pageIndex":1,"rects":[{"left":100,"top":200,"right":500,"bottom":250}]}', 0)`,
		// Annotation 2: note-only on page 3
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (11, 2, 1, '', 'Another highlighted passage', 'Need to verify this claim', '#ff6666', '3', '00002|00000|00000', '{"pageIndex":2,"rects":[{"left":50,"top":300,"right":450,"bottom":330}]}', 0)`,
		// Annotation 3: highlight without comment on page 5
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (12, 2, 0, '', 'A pure highlight text', '', '#5c9eff', '5', '00003|00000|00000', '{"pageIndex":4,"rects":[{"left":200,"top":100,"right":600,"bottom":140}]}', 0)`,
		// Annotation 4: ink/drawing on page 1
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (13, 2, 3, '', '', 'A hand-drawn sketch note', '#000000', '1', '00004|00000|00000', '{"pageIndex":0}', 0)`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	// Create storage dir with a dummy PDF so attachment resolves
	attachmentDir := filepath.Join(storageDir, "ATTACHPDF")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "paper.pdf"), []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAnnotationsReturnsAllTypes(t *testing.T) {
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
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	reader, err := backend.NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error: %v", err)
	}

	// Load the article item which has a PDF attachment with annotations
	item, err := reader.GetItem(context.Background(), "ITEM1234")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}

	// Verify annotations are loaded on the item
	if len(item.Annotations) == 0 {
		t.Fatalf("expected annotations on ITEM1234, got 0")
	}

	// Should have exactly 4 annotations
	if len(item.Annotations) != 4 {
		t.Fatalf("expected 4 annotations, got %d", len(item.Annotations))
	}

	// Annotation 1: note type with text+comment
	a0 := item.Annotations[0]
	if a0.Key != "10" { // itemIDs are auto-generated in this fixture
		// Key comes from items table, let's just check content
	}
	// Find by text to verify regardless of key ordering
	foundHighlightNote := false
	foundPureHighlight := false
	foundInk := false
	for _, a := range item.Annotations {
		if a.Text == "Key finding about genome assembly" {
			foundHighlightNote = true
			if a.Type != "note" {
				t.Errorf("expected type=note for highlight-note annotation, got %s", a.Type)
			}
			if a.Comment != "This is important for the discussion section" {
				t.Errorf("unexpected comment: %q", a.Comment)
			}
			if a.Color != "#ffd400" {
				t.Errorf("unexpected color: %q", a.Color)
			}
			if a.PageLabel != "2" {
				t.Errorf("unexpected pageLabel: %q", a.PageLabel)
			}
			if a.PageIndex != 1 {
				t.Errorf("expected pageIndex=1, got %d", a.PageIndex)
			}
			if a.IsExternal {
				t.Error("expected IsExternal=false")
			}
		}
		if a.Text == "A pure highlight text" {
			foundPureHighlight = true
			if a.Type != "highlight" {
				t.Errorf("expected type=highlight, got %s", a.Type)
			}
			if a.Comment != "" {
				t.Errorf("expected empty comment for pure highlight, got %q", a.Comment)
			}
			if a.Color != "#5c9eff" {
				t.Errorf("unexpected color: %q", a.Color)
			}
			if a.PageLabel != "5" {
				t.Errorf("unexpected pageLabel: %q", a.PageLabel)
			}
		}
		if a.Type == "ink" {
			foundInk = true
			if a.Comment != "A hand-drawn sketch note" {
				t.Errorf("unexpected ink comment: %q", a.Comment)
			}
		}
	}
	if !foundHighlightNote {
		t.Error("could not find the highlight-note annotation")
	}
	if !foundPureHighlight {
		t.Error("could not find the pure highlight annotation")
	}
	if !foundInk {
		t.Error("could not find the ink annotation")
	}
}

func TestLoadAnnotationsEmptyWhenNoneExist(t *testing.T) {
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
	buildLocalShowFixture(t, sqlitePath, storageDir) // standard fixture has no real annotations
	t.Setenv("ZOT_DATA_DIR", dataDir)

	reader, err := backend.NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error: %v", err)
	}

	item, err := reader.GetItem(context.Background(), "ITEM1234")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}

	if len(item.Annotations) != 0 {
		t.Fatalf("expected 0 annotations for item without annotations, got %d", len(item.Annotations))
	}
}

func TestShowLocalJSONIncludesAnnotations(t *testing.T) {
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
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
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

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be an object, got %#v", got["data"])
	}

	annos, ok := data["annotations"].([]any)
	if !ok {
		t.Fatalf("expected annotations array in show output, got %#v", data["annotations"])
	}
	if len(annos) != 4 {
		t.Fatalf("expected 4 annotations, got %d", len(annos))
	}

	// Verify first annotation structure
	first := annos[0].(map[string]any)
	requiredFields := []string{"key", "type", "text", "comment", "color", "page_label", "page_index", "position", "is_external"}
	existingKeys := make([]string, 0, len(first))
	for k := range first {
		existingKeys = append(existingKeys, k)
	}
	for _, f := range requiredFields {
		if _, exists := first[f]; !exists {
			t.Errorf("annotation missing required field %q; available keys: %v", f, existingKeys)
		}
	}
}

func TestShowLocalTextOutputIncludesAnnotations(t *testing.T) {
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
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Annotations: 4",
		"color=#ffd400",
		"page=2",
		"Key finding about genome assembly",
		"This is important for the discussion section",
		"color=#5c9eff",
		"A pure highlight text",
		"page=5",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in show text output, got:\n%s", want, got)
		}
	}
}
