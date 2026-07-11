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
