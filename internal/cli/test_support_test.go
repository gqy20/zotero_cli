package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureOutput(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	oldStdout := defaultCLI.stdout
	oldStderr := defaultCLI.stderr
	oldStdin := defaultCLI.stdin

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	defaultCLI.stdout = out
	defaultCLI.stderr = errOut
	defaultCLI.stdin = strings.NewReader("")

	t.Cleanup(func() {
		defaultCLI.stdout = oldStdout
		defaultCLI.stderr = oldStderr
		defaultCLI.stdin = oldStdin
	})

	return out, errOut
}

func restoreOutput() {}

func setTestConfigDir(t *testing.T, root string) {
	t.Helper()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
}

func writeTestConfig(t *testing.T, root string) {
	t.Helper()

	configDir := filepath.Join(root, ".zot")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configEnv := strings.Join([]string{
		"ZOT_MODE=web",
		"ZOT_LIBRARY_TYPE=user",
		"ZOT_LIBRARY_ID=123456",
		"ZOT_API_KEY=secret",
		"ZOT_STYLE=apa",
		"ZOT_LOCALE=en-US",
		"ZOT_TIMEOUT_SECONDS=20",
		"ZOT_RETRY_MAX_ATTEMPTS=3",
		"ZOT_RETRY_BASE_DELAY_MS=1",
		"ZOT_ALLOW_WRITE=1",
		"ZOT_ALLOW_DELETE=1",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(configEnv), 0o600); err != nil {
		t.Fatal(err)
	}
}
