package cli

import (
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func TestRunAnnotateRemoteReturnsExplicitError(t *testing.T) {
	root := t.TempDir()
	setTestConfigDir(t, root)
	writeTestConfig(t, root)
	t.Setenv("ZOT_MODE", "remote")
	t.Setenv("ZOT_SERVER_ADDR", "http://127.0.0.1:8021")

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		t.Fatal("newLocalReader should not be called in remote annotate")
		return nil, nil
	}

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"annotate", "ITEM123", "--text", "hello"})
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "annotate is not available in remote mode") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunAnnotationsClearRemoteReturnsExplicitError(t *testing.T) {
	root := t.TempDir()
	setTestConfigDir(t, root)
	writeTestConfig(t, root)
	t.Setenv("ZOT_MODE", "remote")
	t.Setenv("ZOT_SERVER_ADDR", "http://127.0.0.1:8021")

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"annotations", "ITEM123", "--clear"})
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "annotation deletion is not available in remote mode") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
