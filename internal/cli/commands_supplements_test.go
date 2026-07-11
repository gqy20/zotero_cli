package cli

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	_ "modernc.org/sqlite"
)

func TestRunSupplementsLocalJSON(t *testing.T) {
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
	addSupplementFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"supplements", "ITEM1234", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "item supp" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["total"] != float64(1) || meta["scanned_items"] != float64(1) {
		t.Fatalf("unexpected meta: %#v", got["meta"])
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data: %#v", got["data"])
	}
	supplement, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected supplement: %#v", data[0])
	}
	if supplement["kind"] != "supplementary_dataset" {
		t.Fatalf("unexpected kind: %#v", supplement["kind"])
	}
	if supplement["resolution_status"] != "stored_file_found" {
		t.Fatalf("unexpected resolution_status: %#v", supplement["resolution_status"])
	}
	if supplement["local_path"] == "" {
		t.Fatalf("expected local_path in supplement: %#v", supplement)
	}
}

func TestRunSupplementsAllHonorsLimit(t *testing.T) {
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
	addSupplementFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"supplements", "--all", "--json", "--limit", "1"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	meta := got["meta"].(map[string]any)
	if meta["total"] != float64(1) || meta["total_before_limit"] != float64(1) {
		t.Fatalf("unexpected meta: %#v", got["meta"])
	}
}

func TestRunInspectAttachmentLocalJSON(t *testing.T) {
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
	addSupplementFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"inspect-attachment", "SUPPXLSX", "--sheet", "Table S1", "--head", "3", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "inspect-attachment" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data: %#v", got["data"])
	}
	result := data[0].(map[string]any)
	if result["attachment_key"] != "SUPPXLSX" {
		t.Fatalf("unexpected attachment_key: %#v", result["attachment_key"])
	}
	workbook := result["workbook"].(map[string]any)
	sheets := workbook["sheets"].([]any)
	sheet := sheets[0].(map[string]any)
	if sheet["name"] != "Table S1" || sheet["header_row"] != float64(2) {
		t.Fatalf("unexpected sheet: %#v", sheet)
	}
}

func TestRunInspectAttachmentItemJSON(t *testing.T) {
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
	addSupplementFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"inspect-attachment", "--item", "ITEM1234", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data := got["data"].([]any)
	result := data[0].(map[string]any)
	if result["item_key"] != "ITEM1234" || result["label"] != "41588_2024_1715_MOESM4_ESM" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunInspectAttachmentItemHealthJSON(t *testing.T) {
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
	addMissingAttachmentFixture(t, sqlitePath)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"inspect-attachment", "--item", "ITEM1234", "--health", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data := got["data"].([]any)
	if len(data) == 0 {
		t.Fatalf("expected health results, got %#v", got["data"])
	}
	var missing map[string]any
	for _, raw := range data {
		result := raw.(map[string]any)
		if result["attachment_key"] == "MISSPDF1" {
			missing = result
			break
		}
	}
	if missing == nil {
		t.Fatalf("missing health result not found: %#v", data)
	}
	health := missing["health"].(map[string]any)
	if health["ok"] != false || health["status"] != "error" {
		t.Fatalf("unexpected health payload: %#v", health)
	}
	issues := health["issues"].([]any)
	issue := issues[0].(map[string]any)
	if issue["code"] != "unresolved_path" {
		t.Fatalf("unexpected issue payload: %#v", issue)
	}
}

func addSupplementFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	inserts := []string{
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (5, 'SUPPXLSX', 1, 2);`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (12, '41588_2024_1715_MOESM4_ESM'), (13, '41588_2024_1715_MOESM4_ESM.xlsx');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (5, 1, 12), (5, 6, 13);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (5, 1, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 0, 'storage:41588_2024_1715_MOESM4_ESM.xlsx');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	attachmentDir := filepath.Join(storageDir, "SUPPXLSX")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeXLSXFixture(filepath.Join(attachmentDir, "41588_2024_1715_MOESM4_ESM.xlsx")); err != nil {
		t.Fatal(err)
	}
}

func addMissingAttachmentFixture(t *testing.T, sqlitePath string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (20, 'MISSPDF1', 1, 3);`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (30, 'download.pdf'), (31, 'Missing PDF');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (20, 1, 31), (20, 6, 30);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (20, 1, 'application/pdf', 0, 'storage:download.pdf');`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func writeXLSXFixture(path string) error {
	file := excelize.NewFile()
	index, err := file.NewSheet("Table S1")
	if err != nil {
		return err
	}
	file.SetActiveSheet(index)
	if err := file.SetSheetRow("Table S1", "A1", &[]any{"Supplementary Table 1"}); err != nil {
		return err
	}
	if err := file.SetSheetRow("Table S1", "A2", &[]any{"Gene", "Position", "P value"}); err != nil {
		return err
	}
	if err := file.SetSheetRow("Table S1", "A3", &[]any{"ABC1", "chr1:10", "0.01"}); err != nil {
		return err
	}
	if err := file.SetSheetName("Sheet1", "Legend"); err != nil {
		return err
	}
	if err := file.SetSheetRow("Legend", "A1", &[]any{"Table contents:"}); err != nil {
		return err
	}
	return file.SaveAs(path)
}
