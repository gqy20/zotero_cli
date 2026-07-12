package cli

import (
	"encoding/json"
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

func TestFormalShortcutsTranslateWithoutLegacyFlags(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"find", "crispr", "--type", "article"}, []string{"item", "find", "crispr", "--type", "article"}},
		{[]string{"show", "ITEM1"}, []string{"item", "show", "ITEM1"}},
		{[]string{"export", "ITEM1", "--as", "ris"}, []string{"item", "export", "ITEM1", "--as", "ris"}},
	}
	for _, tt := range tests {
		got, ok := translateStageOneArgs(tt.input)
		if !ok || strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
			t.Fatalf("translate %v = %v, %t; want %v", tt.input, got, ok, tt.want)
		}
	}
}

func TestRetiredLegacyCommandsAreUnknown(t *testing.T) {
	for _, name := range []string{"overview", "setup", "create-item", "extract-text", "key-info"} {
		stdout, stderr := captureOutput(t)
		if code := Run([]string{name}); code != ExitUsage {
			t.Fatalf("%s code=%d stdout=%q stderr=%q", name, code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "unknown command") {
			t.Fatalf("%s stderr=%q", name, stderr.String())
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
