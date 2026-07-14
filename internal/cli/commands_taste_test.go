package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/app"
)

func TestRunLibraryTasteLifecycle(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	dataDir := t.TempDir()
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	if code := Run([]string{"lib", "taste", "--json"}); code != ExitOK {
		t.Fatalf("missing taste code=%d stderr=%q", code, stderr.String())
	}
	var missing map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &missing); err != nil {
		t.Fatal(err)
	}
	data := missing["data"].(map[string]any)
	if data["exists"] != false || !strings.HasSuffix(data["path"].(string), filepath.Join(".zotero_cli", "taste.md")) {
		t.Fatalf("missing taste data = %#v", data)
	}
	if len(missing["warnings"].([]any)) != 1 {
		t.Fatalf("missing warnings = %#v", missing["warnings"])
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"lib", "taste", "init", "--json"}); code != ExitOK {
		t.Fatalf("init taste code=%d stderr=%q", code, stderr.String())
	}
	tastePath := filepath.Join(dataDir, ".zotero_cli", "taste.md")
	if _, err := os.Stat(tastePath); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"lib", "taste", "--json"}); code != ExitOK {
		t.Fatalf("read taste code=%d stderr=%q", code, stderr.String())
	}
	var loaded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	loadedData := loaded["data"].(map[string]any)
	if loadedData["exists"] != true || !strings.Contains(loadedData["content"].(string), "# Zotero Library Taste") {
		t.Fatalf("loaded taste data = %#v", loadedData)
	}

	stdout.Reset()
	if code := Run([]string{"lib", "taste", "path", "--json"}); code != ExitOK {
		t.Fatalf("taste path code=%d stderr=%q", code, stderr.String())
	}
	var pathResult map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &pathResult); err != nil {
		t.Fatal(err)
	}
	if pathResult["command"] != "lib taste" || pathResult["data"].(map[string]any)["path"] != tastePath {
		t.Fatalf("path result = %#v", pathResult)
	}
}

func TestClassificationWritesWarnWhenTasteIsMissing(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	dataDir := t.TempDir()
	t.Setenv("ZOT_DATA_DIR", dataDir)

	warning := libraryTasteWriteWarning(app.CommandPath{Resource: "tag", Action: "apply"})
	if warning == nil || warning.Code != "taste_missing" {
		t.Fatalf("warning = %#v", warning)
	}
	if warning := libraryTasteWriteWarning(app.CommandPath{Resource: "item", Action: "edit"}); warning != nil {
		t.Fatalf("unrelated write warning = %#v", warning)
	}

	path := filepath.Join(dataDir, ".zotero_cli", "taste.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("preferences"), 0o600); err != nil {
		t.Fatal(err)
	}
	if warning := libraryTasteWriteWarning(app.CommandPath{Resource: "coll", Action: "edit"}); warning != nil {
		t.Fatalf("configured taste warning = %#v", warning)
	}
}
