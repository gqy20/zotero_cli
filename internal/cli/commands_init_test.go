package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/config"
)

func TestRunInitInteractiveWebMode(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("web\nuser\n123456\nsecret\n")
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

func TestRunInitPdfWithExistingConfigDoesNotFailAlreadyExists(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"init", "--pdf"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "config already exists") {
		t.Fatalf("expected --pdf to use existing config, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "has no effect in web mode") {
		t.Fatalf("expected web-mode warning, got %q", stderr.String())
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
		"zot config init",
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

func TestRunInitRemoteModeNonInteractive(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)

	exitCode := Run([]string{
		"init",
		"--mode", "remote",
		"--server-addr", "http://192.168.1.100:8021",
	})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	cfgStr := string(content)
	if !strings.Contains(cfgStr, "ZOT_MODE=remote") {
		t.Fatalf("expected ZOT_MODE=remote, got:\n%s", cfgStr)
	}
	if !strings.Contains(cfgStr, "ZOT_SERVER_ADDR=http://192.168.1.100:8021") {
		t.Fatalf("expected ZOT_SERVER_ADDR, got:\n%s", cfgStr)
	}
}

func TestRunInitRemoteWithWebAPINonInteractive(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)

	exitCode := Run([]string{
		"init",
		"--mode", "remote",
		"--server-addr", "http://192.168.1.100:8021",
		"--library-id", "12345",
		"--api-key", "mykey",
	})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	cfgStr := string(content)
	for _, expected := range []string{
		"ZOT_MODE=remote",
		"ZOT_SERVER_ADDR=http://192.168.1.100:8021",
		"ZOT_LIBRARY_ID=12345",
		"ZOT_API_KEY=mykey",
	} {
		if !strings.Contains(cfgStr, expected) {
			t.Fatalf("expected %q in config, got:\n%s", expected, cfgStr)
		}
	}
}

func TestRunInitRemoteWithWebAPIInteractive(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("remote\nhttp://localhost:8021\ny\nuser\n999\napikey123\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init", "--no-pdf"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Also configure Zotero Web API") {
		t.Fatalf("expected web API prompt, got %q", out)
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	cfgStr := string(content)
	for _, expected := range []string{
		"ZOT_MODE=remote",
		"ZOT_SERVER_ADDR=http://localhost:8021",
		"ZOT_LIBRARY_TYPE=user",
		"ZOT_LIBRARY_ID=999",
		"ZOT_API_KEY=apikey123",
	} {
		if !strings.Contains(cfgStr, expected) {
			t.Fatalf("expected %q in config, got:\n%s", expected, cfgStr)
		}
	}
}

func TestRunInitRemoteWithoutWebAPIInteractive(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)

	stdout, stderr := captureOutput(t)
	oldStdin := testCLI.stdin
	testCLI.stdin = strings.NewReader("remote\nhttp://localhost:8021\nn\n")
	t.Cleanup(func() { testCLI.stdin = oldStdin })

	exitCode := Run([]string{"init", "--no-pdf"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	configPath := filepath.Join(configRoot, ".zot", ".env")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	cfgStr := string(content)
	if !strings.Contains(cfgStr, "ZOT_MODE=remote") {
		t.Fatalf("expected ZOT_MODE=remote, got:\n%s", cfgStr)
	}
	for _, line := range strings.Split(cfgStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ZOT_API_KEY=") {
			val := strings.TrimPrefix(line, "ZOT_API_KEY=")
			if val != "" {
				t.Fatalf("did not expect non-empty ZOT_API_KEY in pure remote config, got %q", val)
			}
		}
	}
}

func TestRemoteClientConfig_RemoteWithAPIKey(t *testing.T) {
	cfg := config.Config{
		Mode:       "remote",
		ServerAddr: "http://localhost:8021",
		APIKey:     "mykey",
		LibraryID:  "12345",
	}
	result, err := testCLI.remoteClientConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Mode != "web" {
		t.Fatalf("expected mode to be normalized to web, got %q", result.Mode)
	}
	if result.APIKey != "mykey" {
		t.Fatalf("expected API key preserved, got %q", result.APIKey)
	}
}

func TestRemoteClientConfig_RemoteWithoutAPIKey(t *testing.T) {
	cfg := config.Config{
		Mode:       "remote",
		ServerAddr: "http://localhost:8021",
	}
	_, err := testCLI.remoteClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for remote mode without API key")
	}
	if !strings.Contains(err.Error(), "without API key") {
		t.Fatalf("expected API key hint in error, got %q", err.Error())
	}
}

func TestRemoteClientConfig_RemoteWithOnlyAPIKey(t *testing.T) {
	cfg := config.Config{
		Mode:       "remote",
		ServerAddr: "http://localhost:8021",
		APIKey:     "mykey",
	}
	_, err := testCLI.remoteClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error when library_id is missing")
	}
}
