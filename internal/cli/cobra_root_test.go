package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestStageFourCanonicalCommandsExist(t *testing.T) {
	root := testCLI.newRootCommand()
	for _, path := range [][]string{{"file", "show"}, {"file", "check"}, {"pdf", "text"}, {"pdf", "figs"}, {"pdf", "open"}, {"ann", "list"}, {"ann", "new"}, {"ann", "delete"}} {
		cmd, remaining, err := root.Find(path)
		if err != nil || len(remaining) != 0 || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v = %q remaining=%v err=%v", path, cmd.CommandPath(), remaining, err)
		}
	}
}

func TestStageFiveCanonicalCommandsExist(t *testing.T) {
	root := testCLI.newRootCommand()
	for _, path := range [][]string{{"ref", "show"}, {"ref", "find"}, {"ref", "related"}, {"ref", "cited"}, {"ref", "ctx"}, {"ref", "links"}, {"ref", "entities"}, {"ref", "profile"}, {"ref", "build"}, {"ref", "resolve"}, {"ref", "status"}, {"index", "build"}, {"index", "status"}} {
		cmd, remaining, err := root.Find(path)
		if err != nil || len(remaining) != 0 || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v = %q remaining=%v err=%v", path, cmd.CommandPath(), remaining, err)
		}
	}
}

func TestStageFiveLegacyTranslation(t *testing.T) {
	tests := []struct{ legacy, canonical []string }{
		{[]string{"ref", "ITEM1", "--json"}, []string{"ref", "show", "ITEM1", "--json"}},
		{[]string{"ref", "search", "mesh", "--field", "mesh"}, []string{"ref", "find", "mesh", "--field", "mesh"}},
		{[]string{"ref", "cited-by", "ITEM1"}, []string{"ref", "cited", "ITEM1"}},
		{[]string{"ref", "contexts", "ITEM1"}, []string{"ref", "ctx", "ITEM1"}},
		{[]string{"ref", "contexts", "build", "--workers", "2"}, []string{"ref", "build", "--contexts", "--workers", "2"}},
		{[]string{"ref", "annotations", "ITEM1"}, []string{"ref", "entities", "ITEM1"}},
		{[]string{"ref", "retry", "--workers", "2"}, []string{"ref", "build", "--failed", "--workers", "2"}},
		{[]string{"ref", "failed"}, []string{"ref", "status", "--failed"}},
		{[]string{"ref", "unsupported"}, []string{"ref", "status", "--unsupported"}},
		{[]string{"ref", "grobid", "status"}, []string{"ref", "status", "--grobid"}},
		{[]string{"ref", "grobid", "build", "--limit", "2"}, []string{"ref", "build", "--grobid", "--limit", "2"}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.legacy)
		if !ok || !reflect.DeepEqual(got, tt.canonical) {
			t.Fatalf("translate %v = %v, %t; want %v", tt.legacy, got, ok, tt.canonical)
		}
	}
}

func TestStageFourLegacyTranslation(t *testing.T) {
	tests := []struct{ legacy, canonical []string }{
		{[]string{"inspect-attachment", "ATT1"}, []string{"file", "show", "ATT1"}},
		{[]string{"inspect-attachment", "--item", "I1", "--health"}, []string{"file", "check", "--item", "I1"}},
		{[]string{"extract-text", "I1", "--pages", "2"}, []string{"pdf", "text", "I1", "--pages", "2"}},
		{[]string{"extract-figures", "I1", "--workers", "2"}, []string{"pdf", "figs", "I1", "--workers", "2"}},
		{[]string{"open", "I1", "--page", "4"}, []string{"pdf", "open", "I1", "--page", "4"}},
		{[]string{"annotations", "I1", "--type", "note"}, []string{"ann", "list", "I1", "--type", "note"}},
		{[]string{"annotations", "I1", "--clear", "--page", "2"}, []string{"ann", "delete", "I1", "--page", "2", "--yes"}},
		{[]string{"annotate", "I1", "--from-file", "ann.json"}, []string{"ann", "new", "I1", "--from", "ann.json"}},
		{[]string{"annotate", "I1", "--clear", "--type", "highlight"}, []string{"ann", "delete", "I1", "--type", "highlight", "--yes"}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.legacy)
		if !ok || !reflect.DeepEqual(got, tt.canonical) {
			t.Fatalf("translate %v = %v, %t; want %v", tt.legacy, got, ok, tt.canonical)
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

func TestStageSixLegacyTranslation(t *testing.T) {
	tests := []struct {
		legacy, canonical []string
	}{
		{[]string{"schema", "types"}, []string{"schema", "list", "types"}},
		{[]string{"schema", "fields-for", "article"}, []string{"schema", "list", "fields", "article"}},
		{[]string{"schema", "creator-types-for", "book"}, []string{"schema", "list", "roles", "book"}},
		{[]string{"schema", "template", "book"}, []string{"schema", "show", "book"}},
		{[]string{"server", "--port", "9000"}, []string{"server", "start", "--port", "9000"}},
		{[]string{"sync", "--force"}, []string{"sync", "pull", "--force"}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.legacy)
		if !ok || !reflect.DeepEqual(got, tt.canonical) {
			t.Fatalf("translate %v = %v, %t; want %v", tt.legacy, got, ok, tt.canonical)
		}
	}
}

func TestStageSixCanonicalTree(t *testing.T) {
	root := testCLI.newRootCommand()
	for _, path := range [][]string{{"schema", "list"}, {"schema", "show"}, {"server", "start"}, {"sync", "pull"}, {"completion"}, {"version"}} {
		cmd := root
		for _, name := range path {
			found := false
			for _, child := range cmd.Commands() {
				if child.Name() == name {
					cmd, found = child, true
					break
				}
			}
			if !found {
				t.Fatalf("missing canonical command path %v", path)
			}
		}
	}
}

func TestCompletionGeneratesAllSupportedShellsWithoutConfig(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		stdout, stderr := captureOutput(t)
		if code := Run([]string{"completion", shell}); code != ExitOK {
			t.Fatalf("completion %s: code=%d stderr=%q", shell, code, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("completion %s produced no output", shell)
		}
	}
}

func TestStageSevenRootUsesOnlyCobraHelp(t *testing.T) {
	stdout, stderr := captureOutput(t)
	if code := Run(nil); code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Usage:", "item", "ref", "completion"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("root help missing %q: %q", want, stdout.String())
		}
	}
	for name := range legacyOnlyCommands {
		if strings.Contains(stdout.String(), "  "+name+" ") {
			t.Fatalf("legacy-only command %q leaked into root help", name)
		}
	}
}

func TestStageSevenRedirectOnlyCommandsDoNotExecuteLegacyBusiness(t *testing.T) {
	for name, replacement := range legacyOnlyCommands {
		stdout, stderr := captureOutput(t)
		if code := Run([]string{name, "ignored", "--json"}); code != ExitUsage {
			t.Fatalf("%s code=%d stderr=%q", name, code, stderr.String())
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), replacement) {
			t.Fatalf("%s stdout=%q stderr=%q", name, stdout.String(), stderr.String())
		}
	}
}

func TestStageSevenLegacyWarningPathDoesNotEchoArguments(t *testing.T) {
	translated, ok := translateStageOneArgs([]string{"init", "--api-key", "TOP-SECRET", "--mode", "web"})
	if !ok {
		t.Fatal("expected legacy translation")
	}
	if got := canonicalPath(translated); got != "config init" || strings.Contains(got, "TOP-SECRET") {
		t.Fatalf("unsafe warning path %q", got)
	}
}

func TestCanonicalCommandTreeHasAtMostResourceAndActionDepth(t *testing.T) {
	root := testCLI.newRootCommand()
	var walk func(*cobra.Command, int)
	walk = func(cmd *cobra.Command, depth int) {
		if !cmd.Hidden && depth > 2 {
			t.Fatalf("canonical command %q exceeds resource/action depth", cmd.CommandPath())
		}
		for _, child := range cmd.Commands() {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
}
