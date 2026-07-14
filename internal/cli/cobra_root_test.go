package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"zotero_cli/internal/backend"
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

func TestCanonicalCommandsExist(t *testing.T) {
	root := testCLI.newRootCommand()
	paths := [][]string{
		{"lib", "show"}, {"lib", "stats"}, {"lib", "log"},
		{"item", "list"}, {"coll", "list"}, {"tag", "list"}, {"tag", "replace"},
		{"note", "list"}, {"search", "list"}, {"group", "list"},
		{"item", "find"}, {"item", "show"}, {"item", "new"}, {"item", "edit"}, {"item", "delete"}, {"item", "tag"}, {"item", "untag"}, {"item", "supp"}, {"item", "export"},
		{"coll", "show"}, {"coll", "new"}, {"coll", "edit"}, {"coll", "delete"}, {"coll", "add"}, {"coll", "remove"},
		{"note", "find"}, {"note", "show"}, {"note", "new"}, {"note", "edit"}, {"note", "delete"},
		{"search", "show"}, {"search", "new"}, {"search", "edit"}, {"search", "delete"},
		{"file", "show"}, {"file", "check"}, {"pdf", "text"}, {"pdf", "figs"}, {"pdf", "open"}, {"ann", "list"}, {"ann", "new"}, {"ann", "delete"},
		{"ref", "show"}, {"ref", "find"}, {"ref", "related"}, {"ref", "cited"}, {"ref", "ctx"}, {"ref", "links"}, {"ref", "entities"}, {"ref", "profile"}, {"ref", "build"}, {"ref", "resolve"}, {"ref", "status"},
		{"index", "build"}, {"index", "status"}, {"schema", "list"}, {"schema", "show"},
		{"config", "init"}, {"config", "show"}, {"config", "check"},
		{"serve"}, {"sync"}, {"completion"}, {"version"},
	}
	for _, path := range paths {
		cmd, remaining, err := root.Find(path)
		if err != nil || len(remaining) != 0 || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v = %q remaining=%v err=%v", path, cmd.CommandPath(), remaining, err)
		}
	}
}

func TestFormalShortcutsExpandToCanonicalArgs(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"find", "crispr", "--type", "article"}, []string{"item", "find", "crispr", "--type", "article"}},
		{[]string{"show", "ITEM1"}, []string{"item", "show", "ITEM1"}},
		{[]string{"export", "ITEM1", "--as", "ris"}, []string{"item", "export", "ITEM1", "--as", "ris"}},
	}
	for _, tt := range tests {
		got := expandShortcutArgs(tt.input)
		if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
			t.Fatalf("expand %v = %v; want %v", tt.input, got, tt.want)
		}
	}
}

func TestRetiredLegacyCommandsAreUnknown(t *testing.T) {
	for _, name := range []string{"overview", "setup", "create-item", "extract-text", "key-info", "server"} {
		stdout, stderr := captureOutput(t)
		if code := Run([]string{name}); code != ExitUsage {
			t.Fatalf("%s code=%d stdout=%q stderr=%q", name, code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "unknown command") {
			t.Fatalf("%s stderr=%q", name, stderr.String())
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

func TestRootUsesCobraHelp(t *testing.T) {
	stdout, stderr := captureOutput(t)
	if code := Run(nil); code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Usage:", "item", "ref", "completion", "Common shortcuts:", "zot find QUERY", "zot show KEY", "zot export [KEY...]"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("root help missing %q: %q", want, stdout.String())
		}
	}
}

func TestFileCheckOnlyExposesHealthRelevantFlags(t *testing.T) {
	root := testCLI.newRootCommand()
	check, _, err := root.Find([]string{"file", "check"})
	if err != nil {
		t.Fatal(err)
	}
	if check.Flags().Lookup("item") == nil {
		t.Fatal("file check missing --item")
	}
	for _, name := range []string{"sheet", "head", "max-sheets", "max-columns"} {
		if check.Flags().Lookup(name) != nil {
			t.Fatalf("file check unexpectedly exposes --%s", name)
		}
	}

	show, _, err := root.Find([]string{"file", "show"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"item", "sheet", "head", "max-sheets", "max-columns"} {
		if show.Flags().Lookup(name) == nil {
			t.Fatalf("file show missing --%s", name)
		}
	}
}

func TestAnnotationCreateValidationMatchesPhysicalPDFTypes(t *testing.T) {
	valid := []backend.AnnotateRequest{
		{Type: "highlight", Text: "target"},
		{Type: "underline", Page: 2, Rect: &[4]float64{1, 2, 3, 4}},
		{Type: "note", Page: 3, Point: &[2]float64{10, 20}},
	}
	for _, request := range valid {
		if err := validateAnnotationRequest(request); err != nil {
			t.Fatalf("valid request %#v rejected: %v", request, err)
		}
	}
	invalid := []backend.AnnotateRequest{
		{Type: "image", Text: "target"},
		{Type: "note", Text: "target"},
		{Type: "highlight", Page: 3, Point: &[2]float64{10, 20}},
		{Type: "highlight", Text: "target", Page: 2, Rect: &[4]float64{1, 2, 3, 4}},
	}
	for _, request := range invalid {
		if err := validateAnnotationRequest(request); err == nil {
			t.Fatalf("invalid request %#v was accepted", request)
		}
	}
}

func TestAnnotationBatchInfersNoteForPointTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "annotations.json")
	if err := os.WriteFile(path, []byte(`[{"attachment_key":"ATT2","page":2,"point":[10,20],"comment":"note"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	targets, err := annotationTargets("ITEM1", backend.AnnotateRequest{Type: "highlight", Color: "yellow"}, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Request.Type != "note" || targets[0].Request.AttachmentKey != "ATT2" {
		t.Fatalf("targets=%#v", targets)
	}
}

func TestAnnotationCommandsExposeAttachmentSelector(t *testing.T) {
	root := testCLI.newRootCommand()
	for _, path := range [][]string{{"ann", "list"}, {"ann", "new"}, {"ann", "delete"}} {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		if command.Flags().Lookup("attachment") == nil {
			t.Fatalf("%s is missing --attachment", strings.Join(path, " "))
		}
	}
}

func TestPDFTextExposesCollectionAndRegexGrep(t *testing.T) {
	root := testCLI.newRootCommand()
	command, _, err := root.Find([]string{"pdf", "text"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("collection") == nil {
		t.Fatal("pdf text is missing --collection")
	}
	grep := command.Flags().Lookup("grep")
	if grep == nil || !strings.Contains(grep.Usage, "regular expression") {
		t.Fatalf("pdf text --grep help does not describe regex semantics: %#v", grep)
	}
}

func TestQueryCommandsExposeOneScopeSelector(t *testing.T) {
	root := testCLI.newRootCommand()
	tests := []struct {
		path    []string
		present []string
		retired []string
	}{
		{[]string{"item", "find"}, []string{"in"}, []string{"fulltext", "fulltext-only", "metadata-only", "fulltext-any"}},
		{[]string{"ref", "find"}, []string{"in"}, []string{"contexts", "references", "metadata"}},
		{[]string{"note", "list"}, nil, []string{"query"}},
		{[]string{"item", "export"}, []string{"from"}, []string{"query", "collection", "all", "type", "tag", "attachment-name", "has-pdf"}},
	}
	for _, tt := range tests {
		command, _, err := root.Find(tt.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range tt.present {
			if command.Flags().Lookup(name) == nil {
				t.Fatalf("%s is missing --%s", strings.Join(tt.path, " "), name)
			}
		}
		for _, name := range tt.retired {
			if command.Flags().Lookup(name) != nil {
				t.Fatalf("%s unexpectedly exposes retired --%s", strings.Join(tt.path, " "), name)
			}
		}
	}
}

func TestBenchmarkManifestUsesCanonicalCommandPaths(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	content, err := os.ReadFile(filepath.Join(repositoryRoot, "benchmarks", "cli", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Commands []struct {
			Path string `json:"path"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	root := testCLI.newRootCommand()
	for _, entry := range manifest.Commands {
		path := strings.Fields(entry.Path)
		command, remaining, err := root.Find(path)
		if err != nil || command == nil || len(path) == 0 {
			t.Fatalf("benchmark command %q is unavailable: remaining=%v err=%v", entry.Path, remaining, err)
		}
		if len(remaining) != 0 || command.Name() != path[len(path)-1] {
			t.Fatalf("benchmark command %q is not canonical: command=%q remaining=%v", entry.Path, command.CommandPath(), remaining)
		}
	}
}

func TestStableDocumentationMatchesCurrentCommandSurface(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	skill, err := os.ReadFile(filepath.Join(root, ".codex", "skills", "zotero-cli", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(skill), "sync pull") {
		t.Fatal("zotero-cli skill documents retired sync pull command")
	}

	documentPaths := []string{
		"README.md",
		filepath.Join("docs", "reference", "performance-baseline.md"),
		filepath.Join("docs", "reference", "performance-benchmark.md"),
		filepath.Join("docs", "user", "commands.md"),
		filepath.Join("docs", "user", "quickstart.md"),
		filepath.Join("docs", "user", "examples", "annotations.md"),
		filepath.Join(".codex", "skills", "zotero-cli", "SKILL.md"),
		filepath.Join(".claude", "skills", "zotero-cli", "SKILL.md"),
		filepath.Join(".claude", "skills", "zotero-cli", "reference.md"),
	}
	for _, relativePath := range documentPaths {
		content, err := os.ReadFile(filepath.Join(root, relativePath))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if strings.Contains(text, "snippet 默认限制 50") || strings.Contains(text, "--snippet` 默认限 50") {
			t.Fatalf("%s documents retired snippet default of 50", relativePath)
		}
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			isCommand := strings.HasPrefix(trimmed, "zot ") || strings.HasPrefix(trimmed, `.\zot.exe `)
			if isCommand && strings.Contains(line, "pdf text") && strings.Contains(line, "--workers") {
				t.Fatalf("%s documents unsupported pdf text --workers flag: %q", relativePath, line)
			}
			if isCommand && strings.Contains(line, "ann delete") && !strings.Contains(line, "--source") {
				t.Fatalf("%s documents unsafe ann delete without --source: %q", relativePath, line)
			}
			if isCommand && strings.Contains(line, "find ") {
				for _, retired := range []string{"--fulltext", "--fulltext-only", "--metadata-only", "--fulltext-any"} {
					if strings.Contains(line, retired) {
						t.Fatalf("%s documents retired find flag %s: %q", relativePath, retired, line)
					}
				}
			}
			if isCommand && strings.Contains(line, "ref find") {
				for _, retired := range []string{"--contexts", "--references", "--metadata"} {
					if strings.Contains(line, retired) {
						t.Fatalf("%s documents retired ref find flag %s: %q", relativePath, retired, line)
					}
				}
			}
			if isCommand && strings.Contains(line, "note list") && strings.Contains(line, "--query") {
				t.Fatalf("%s documents retired note list --query: %q", relativePath, line)
			}
			if isCommand && strings.Contains(line, "export ") {
				exportInvocation := line[strings.Index(line, "export "):]
				for _, retired := range []string{"--query", "--collection", "--all", "--type", "--tag", "--attachment-name", "--has-pdf"} {
					if strings.Contains(exportInvocation, retired) {
						t.Fatalf("%s documents retired export selector %s: %q", relativePath, retired, line)
					}
				}
			}
		}
	}

	claudeSkill, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "zotero-cli", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"默认限制 100 条", "默认限制 20 条", "item import --collection", "zotero_desktop_connector_available", "content_path"} {
		if !strings.Contains(string(claudeSkill), required) {
			t.Fatalf("Claude zotero-cli skill is missing current behavior %q", required)
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
