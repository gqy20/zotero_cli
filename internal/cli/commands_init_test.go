package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitInteractiveWebMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("\nuser\n123456\nsecret\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file created, stat err=%v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "created config at") {
		t.Fatalf("expected success message, got %q", out)
	}
	for _, want := range []string{
		"Initialize ~/.zot/.env",
		"https://www.zotero.org/settings/keys",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
	content, _ := os.ReadFile(configPath)
	cfgStr := string(content)
	if !strings.Contains(cfgStr, "ZOT_MODE=web") {
		t.Fatalf("expected ZOT_MODE=web in config, got:\n%s", cfgStr)
	}
	if strings.Contains(cfgStr, "PyMuPDF") {
		t.Fatal("web mode should not mention PyMuPDF")
	}
}

func TestRunInitInteractiveHybridMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, _ := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("hybrid\nuser\n123456\nsecret\n/tmp/zotero\nn")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	out := stdout.String()
	if !strings.Contains(out, "Zotero data directory") {
		t.Fatalf("expected data dir prompt in hybrid mode, got %q", out)
	}
	if !strings.Contains(out, "Set up PyMuPDF") {
		t.Fatalf("expected PyMuPDF prompt in hybrid mode, got %q", out)
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, _ := os.ReadFile(configPath)
	cfgStr := string(content)
	if !strings.Contains(cfgStr, "ZOT_MODE=hybrid") {
		t.Fatalf("expected ZOT_MODE=hybrid in config, got:\n%s", cfgStr)
	}
	if !strings.Contains(cfgStr, "ZOT_DATA_DIR=/tmp/zotero") {
		t.Fatalf("expected DATA_DIR in config, got:\n%s", cfgStr)
	}
}

func TestRunInitNonInteractive(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"init",
		"--mode", "web",
		"--library-type", "group",
		"--library-id", "789",
		"--api-key", "mykey",
	})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file, err=%v", err)
	}
	cfgStr := string(content)
	for _, expected := range []string{
		"ZOT_MODE=web",
		"ZOT_LIBRARY_TYPE=group",
		"ZOT_LIBRARY_ID=789",
		"ZOT_API_KEY=mykey",
	} {
		if !strings.Contains(cfgStr, expected) {
			t.Fatalf("expected %q in config, got:\n%s", expected, cfgStr)
		}
	}
	if !strings.Contains(stdout.String(), "created config at") {
		t.Fatalf("expected success message, got %q", stdout.String())
	}
}

func TestRunInitPartialFlagsPromptsRest(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("user\n456\nabc\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init", "--mode", "local", "--api-key", "secret", "--no-pdf"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Library type") {
		t.Fatalf("should prompt for missing library_type, got %q", out)
	}
	if !strings.Contains(out, "Library ID") {
		t.Fatalf("should prompt for missing library_id, got %q", out)
	}
	if !strings.Contains(out, "data directory") {
		t.Fatalf("should prompt for data_dir in local mode, got %q", out)
	}
}

func TestRunInitConfigAlreadyExists(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"init"})
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "config already exists") {
		t.Fatalf("expected already-exists error, got %q", stderr.String())
	}
}

func TestRunInitHelp(t *testing.T) {
	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"init", "--help"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	out := stdout.String()
	for _, expected := range []string{
		"zot init",
		"--mode",
		"--library-id",
		"--api-key",
		"--no-pdf",
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in help text, got %q", expected, out)
		}
	}
	if stderr.Len() > 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}

func TestRunInitNoPdfFlagSkipsPdfSetup(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("local\nuser\n123\nkey\n/data\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init", "--no-pdf"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "PyMuPDF setup complete") {
		t.Fatal("--no-pdf should skip PyMuPDF setup")
	}
	if strings.Contains(out, "Set up PyMuPDF") {
		t.Fatal("--no-pdf should skip PyMuPDF prompt")
	}
}

func TestRunInitBackwardCompatConfigInitStillWorks(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("user\n123456\nsecret\n\n\ny\nn\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"config", "init"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tip: use `zot init`") {
		t.Fatalf("expected tip about zot init, got %q", stdout.String())
	}
	configPath := filepath.Join(configRoot, ".zot", ".env")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file should still be created by config init, err=%v", err)
	}
}
