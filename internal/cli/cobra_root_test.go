package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCobraHelpDoesNotLoadConfig(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	stdout, stderr := captureOutput(t)
	root := testCLI.newRootCommand()
	root.SetArgs([]string{"config", "check", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Validate configuration") {
		t.Fatalf("help = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestStageTwoCanonicalCommandsExist(t *testing.T) {
	root := testCLI.newRootCommand()
	for _, path := range [][]string{
		{"lib", "show"}, {"lib", "stats"}, {"lib", "log"},
		{"item", "list"}, {"coll", "list"}, {"tag", "list"},
		{"note", "list"}, {"search", "list"}, {"group", "list"},
	} {
		cmd, remaining, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if len(remaining) != 0 || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v = command %q, remaining %v", path, cmd.CommandPath(), remaining)
		}
	}
}

func TestStageThreeCanonicalCommandsExist(t *testing.T) {
	root := testCLI.newRootCommand()
	paths := [][]string{
		{"item", "find"}, {"item", "show"}, {"item", "new"}, {"item", "edit"}, {"item", "delete"}, {"item", "tag"}, {"item", "untag"}, {"item", "supp"}, {"item", "export"},
		{"coll", "show"}, {"coll", "new"}, {"coll", "edit"}, {"coll", "delete"}, {"coll", "add"}, {"coll", "remove"},
		{"note", "find"}, {"note", "show"}, {"note", "new"}, {"note", "edit"}, {"note", "delete"},
		{"search", "show"}, {"search", "new"}, {"search", "edit"}, {"search", "delete"},
	}
	for _, path := range paths {
		cmd, remaining, err := root.Find(path)
		if err != nil || len(remaining) != 0 || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v = %q remaining=%v err=%v", path, cmd.CommandPath(), remaining, err)
		}
	}
}

func TestStageThreeLegacyTranslation(t *testing.T) {
	tests := []struct {
		legacy, canonical []string
	}{
		{[]string{"find", "crispr", "--item-type", "article", "--start", "2", "--direction", "desc"}, []string{"item", "find", "crispr", "--type", "article", "--offset", "2", "--order", "desc"}},
		{[]string{"show", "ITEM1"}, []string{"item", "show", "ITEM1"}},
		{[]string{"supplements", "ITEM1"}, []string{"item", "supp", "ITEM1"}},
		{[]string{"export", "--item-key", "ITEM1", "--format", "ris"}, []string{"item", "export", "ITEM1", "--as", "ris"}},
		{[]string{"create-item", "--from-file", "item.json", "--if-unmodified-since-version", "7"}, []string{"item", "new", "--from", "item.json", "--if-version", "7"}},
		{[]string{"update-item", "ITEM1", "--data", `{}`, "--if-unmodified-since-version", "7"}, []string{"item", "edit", "ITEM1", "--data", `{}`, "--if-version", "7"}},
		{[]string{"delete-item", "ITEM1", "--if-unmodified-since-version", "7", "--yes"}, []string{"item", "delete", "ITEM1", "--if-version", "7", "--yes"}},
		{[]string{"add-tag", "--items", "A,B", "--tag", "review"}, []string{"item", "tag", "A", "B", "--tag", "review"}},
		{[]string{"remove-tag", "--items=A,B", "--tag", "review"}, []string{"item", "untag", "A", "B", "--tag", "review"}},
		{[]string{"create-collection", "--data", `{}`, "--if-unmodified-since-version", "2"}, []string{"coll", "new", "--data", `{}`, "--if-version", "2"}},
		{[]string{"update-search", "S1", "--data", `{}`}, []string{"search", "edit", "S1", "--data", `{}`}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.legacy)
		if !ok || !reflect.DeepEqual(got, tt.canonical) {
			t.Fatalf("translate %v = %v, %t; want %v", tt.legacy, got, ok, tt.canonical)
		}
	}
}

func TestStageThreeCanonicalAndLegacyDryRunsAreEquivalent(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_ALLOW_DELETE", "1")
	tests := []struct {
		canonical, legacy []string
	}{
		{[]string{"item", "new", "--set", "itemType=book", "--dry-run", "--json"}, []string{"create-item", "--data", `{"itemType":"book"}`, "--dry-run", "--json"}},
		{[]string{"item", "edit", "I1", "--set", "title=New", "--dry-run", "--json"}, []string{"update-item", "I1", "--data", `{"title":"New"}`, "--dry-run", "--json"}},
		{[]string{"item", "delete", "I1", "--dry-run", "--json"}, []string{"delete-item", "I1", "--dry-run", "--json"}},
		{[]string{"coll", "new", "--name", "Inbox", "--dry-run", "--json"}, []string{"create-collection", "--data", `{"name":"Inbox"}`, "--dry-run", "--json"}},
		{[]string{"search", "edit", "S1", "--set", "name=Recent", "--dry-run", "--json"}, []string{"update-search", "S1", "--data", `{"name":"Recent"}`, "--dry-run", "--json"}},
	}
	run := func(args []string) map[string]any {
		stdout, stderr := captureOutput(t)
		if code := Run(args); code != ExitOK {
			t.Fatalf("Run(%v) code=%d stderr=%q", args, code, stderr.String())
		}
		var got map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, tt := range tests {
		canonical, legacy := run(tt.canonical), run(tt.legacy)
		if canonical["command"] != legacy["command"] || !reflect.DeepEqual(canonical["data"], legacy["data"]) || !reflect.DeepEqual(canonical["meta"], legacy["meta"]) {
			t.Fatalf("dry-run mismatch for %v and %v:\n%#v\n%#v", tt.canonical, tt.legacy, canonical, legacy)
		}
	}
}

func TestStageTwoLegacyTranslation(t *testing.T) {
	tests := []struct {
		legacy    []string
		canonical []string
	}{
		{[]string{"overview", "--json"}, []string{"lib", "show", "--json"}},
		{[]string{"stats"}, []string{"lib", "stats"}},
		{[]string{"deleted"}, []string{"lib", "log", "--deleted"}},
		{[]string{"changes", "items", "--since", "4", "--if-modified-since-version", "3"}, []string{"lib", "log", "--kind", "items", "--since", "4", "--if-modified-version", "3"}},
		{[]string{"collections-top"}, []string{"coll", "list", "--top"}},
		{[]string{"collections"}, []string{"coll", "list"}},
		{[]string{"tags"}, []string{"tag", "list"}},
		{[]string{"notes"}, []string{"note", "list"}},
		{[]string{"searches"}, []string{"search", "list"}},
		{[]string{"groups"}, []string{"group", "list"}},
		{[]string{"trash"}, []string{"item", "list", "--scope", "trash"}},
		{[]string{"publications"}, []string{"item", "list", "--scope", "pubs"}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.legacy)
		if !ok || !reflect.DeepEqual(got, tt.canonical) {
			t.Fatalf("translate %v = %v, %t; want %v", tt.legacy, got, ok, tt.canonical)
		}
	}
}

func TestStageTwoCanonicalAndLegacyResultsAreEquivalent(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	tests := []struct {
		canonical []string
		legacy    []string
	}{
		{[]string{"lib", "show", "--json"}, []string{"overview", "--json"}},
		{[]string{"lib", "stats", "--json"}, []string{"stats", "--json"}},
		{[]string{"lib", "log", "--deleted", "--json"}, []string{"deleted", "--json"}},
		{[]string{"lib", "log", "--kind", "items", "--since", "1", "--json"}, []string{"changes", "items", "--since", "1", "--json"}},
		{[]string{"item", "list", "--scope", "trash", "--json"}, []string{"trash", "--json"}},
		{[]string{"item", "list", "--scope", "pubs", "--json"}, []string{"publications", "--json"}},
		{[]string{"coll", "list", "--json"}, []string{"collections", "--json"}},
		{[]string{"coll", "list", "--top", "--json"}, []string{"collections-top", "--json"}},
		{[]string{"tag", "list", "--json"}, []string{"tags", "--json"}},
		{[]string{"note", "list", "--json"}, []string{"notes", "--json"}},
		{[]string{"search", "list", "--json"}, []string{"searches", "--json"}},
		{[]string{"group", "list", "--json"}, []string{"groups", "--json"}},
	}

	runJSON := func(args []string) map[string]any {
		stdout, stderr := captureOutput(t)
		if code := Run(args); code != ExitOK {
			t.Fatalf("Run(%v) code=%d stderr=%q", args, code, stderr.String())
		}
		var envelope map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("Run(%v) returned invalid JSON: %v", args, err)
		}
		return envelope
	}

	for _, tt := range tests {
		canonical := runJSON(tt.canonical)
		legacy := runJSON(tt.legacy)
		if !reflect.DeepEqual(canonical["data"], legacy["data"]) || !reflect.DeepEqual(canonical["meta"], legacy["meta"]) {
			t.Fatalf("canonical %v and legacy %v differ:\ncanonical=%#v\nlegacy=%#v", tt.canonical, tt.legacy, canonical, legacy)
		}
		if canonical["command"] != legacy["command"] {
			t.Fatalf("canonical command differs for %v and %v: %#v vs %#v", tt.canonical, tt.legacy, canonical["command"], legacy["command"])
		}
	}
}

func TestCobraTreeCanBeBuiltRepeatedly(t *testing.T) {
	stdout, _ := captureOutput(t)
	for i := 0; i < 3; i++ {
		stdout.Reset()
		root := testCLI.newRootCommand()
		root.SetArgs([]string{"version", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["command"] != "version" {
			t.Fatalf("command = %#v", got["command"])
		}
	}
}

func TestOutputEnvironmentProvidesGlobalDefault(t *testing.T) {
	t.Setenv("ZOT_OUTPUT", "json")
	stdout, stderr := captureOutput(t)
	if code := Run([]string{"version"}); code != ExitOK {
		t.Fatalf("code = %d stderr=%q", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["command"] != "version" {
		t.Fatalf("command = %#v", got["command"])
	}
}
