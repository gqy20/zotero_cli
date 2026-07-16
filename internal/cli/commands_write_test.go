package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteCancellation(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_ALLOW_DELETE", "1")

	for _, args := range [][]string{{"item", "delete", "ITEM1", "--if-version", "1"}} {
		_, stderr := captureOutput(t)
		testCLI.stdin = strings.NewReader("n\n")
		if code := Run(args); code != 130 {
			t.Fatalf("Run(%v) code=%d stderr=%q", args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "operation cancelled") {
			t.Fatalf("Run(%v) stderr=%q", args, stderr.String())
		}
	}
}

func TestCanonicalWritesPreserveConflictErrors(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	server := newErrorAPI(t, http.StatusPreconditionFailed, "library version conflict")
	defer server.cleanup()
	t.Setenv("ZOT_BASE_URL", server.url)

	for _, args := range [][]string{
		{"item", "edit", "ITEM1", "--set", "title=New", "--if-version", "1", "--json"},
		{"item", "edit", "ITEM1", "--data", `{"title":"New"}`, "--if-version", "1", "--json"},
	} {
		stdout, stderr := captureOutput(t)
		if code := Run(args); code != ExitError {
			t.Fatalf("Run(%v) code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "precondition_failed") {
			t.Fatalf("Run(%v) stdout=%q", args, stdout.String())
		}
	}
}

func TestRunCreateItemJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "new", "--data", `{"itemType":"book","title":"My Book"}`, "--if-version", "41", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "item new" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}

func TestRunItemImportDryRunJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	pdfPath := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-test"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connector/ping" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("ZOT_CONNECTOR_URL", server.URL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "import", pdfPath, "--dry-run", "--json"})
	if exitCode != 0 {
		t.Fatalf("exit code=%d stderr=%q", exitCode, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if got["command"] != "item import" {
		t.Fatalf("command=%#v", got["command"])
	}
	meta := got["meta"].(map[string]any)
	if meta["dry_run"] != true {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestRunItemImportBatchPDFDryRunJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.pdf")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("%PDF-test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pingCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connector/ping" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		pingCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("ZOT_CONNECTOR_URL", server.URL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "import", first, second, "--dry-run", "--json"})
	if exitCode != 0 {
		t.Fatalf("exit code=%d stderr=%q", exitCode, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	data := got["data"].(map[string]any)
	if data["total"] != float64(2) || data["ready"] != float64(2) || len(data["items"].([]any)) != 2 {
		t.Fatalf("data=%#v", data)
	}
	if pingCount != 1 {
		t.Fatalf("pingCount=%d", pingCount)
	}
	if !strings.Contains(stderr.String(), "import [2/2]") || !strings.Contains(stderr.String(), "100%") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunUpdateItemText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "edit", "ABCD2345", "--data", `{"title":"Updated Title"}`, "--if-version", "7"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "updated item ABCD2345") {
		t.Fatalf("unexpected update output: %q", got)
	}
}

func TestRunDeleteItemText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "delete", "ABCD2345", "--if-version", "8", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted item ABCD2345") {
		t.Fatalf("unexpected delete output: %q", got)
	}
}

func TestRunAddTagJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "tag", "ITEMA001", "ITEMA002", "--tag", "paper", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if got["command"] != "item tag" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(53) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunRemoveTagText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "untag", "ITEMA001", "ITEMA002", "--tag", "ai"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, `removed tag "ai" on 2 item(s), 0 unchanged, at library version 53`) {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunTagReplacePreviewJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"tag", "replace", "--match", `^(ai|transformers)$`, "--replace", `topic/$1`, "--json"})
	if exitCode != ExitOK {
		t.Fatalf("code=%d stderr=%q", exitCode, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["command"] != "tag replace" {
		t.Fatalf("command = %#v", got["command"])
	}
	meta := got["meta"].(map[string]any)
	if meta["preview"] != true || meta["matched_tags"] != float64(2) {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestRunTagApplyDryRun(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	planPath := filepath.Join(t.TempDir(), "tags.json")
	if err := os.WriteFile(planPath, []byte(`[{"keys":["ITEMA001","ITEMA002"],"add":["进化","综述"]},{"keys":["ITEMA001"],"remove":["旧标签"]}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"tag", "apply", "--from", planPath, "--dry-run", "--json"})
	if exitCode != ExitOK {
		t.Fatalf("code=%d stderr=%q", exitCode, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["command"] != "tag apply" {
		t.Fatalf("command = %#v", got["command"])
	}
	data := got["data"].(map[string]any)
	if data["items"] != float64(2) || data["add_assignments"] != float64(4) || data["remove_assignments"] != float64(1) {
		t.Fatalf("data = %#v", data)
	}
}

func TestRunCreateItemFromFileText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(payloadPath, []byte(`{"itemType":"book","title":"From File"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "new", "--from", payloadPath, "--if-version", "41"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "created item NEWA1234") || !strings.Contains(got, "library version 42") {
		t.Fatalf("unexpected create output: %q", got)
	}
}

func TestRunUpdateItemFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "patch.json")
	if err := os.WriteFile(payloadPath, []byte(`{"title":"Updated From File"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"item", "edit", "ABCD2345", "--from", payloadPath, "--if-version", "7", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(8) {
		t.Fatalf("unexpected update payload: %#v", got["data"])
	}
}

func TestRunCreateCollectionText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"coll", "new", "--data", `{"name":"New Collection"}`, "--if-version", "10"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "created collection COLLNEW1") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunUpdateCollectionFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "collection.json")
	if err := os.WriteFile(payloadPath, []byte(`{"key":"COLL1234","version":11,"name":"Renamed Collection"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"coll", "edit", "COLL1234", "--from", payloadPath, "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(12) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunDeleteCollectionText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"coll", "delete", "COLL1234", "--if-version", "12", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted collection COLL1234") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunCreateSearchText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"search", "new", "--data", `{"name":"Unread PDFs","conditions":[{"condition":"itemType","operator":"is","value":"attachment"}]}`, "--if-version", "17"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "created search SCH67890") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunUpdateSearchFromFileJSON(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	payloadPath := filepath.Join(t.TempDir(), "search.json")
	if err := os.WriteFile(payloadPath, []byte(`{"key":"SCH12345","version":21,"name":"Important PDFs"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"search", "edit", "SCH12345", "--from", payloadPath, "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["last_modified_version"] != float64(49) {
		t.Fatalf("unexpected payload: %#v", got["data"])
	}
}

func TestRunDeleteSearchText(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"search", "delete", "SCH12345", "--if-version", "22", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "deleted search SCH12345") {
		t.Fatalf("unexpected output: %q", got)
	}
}
