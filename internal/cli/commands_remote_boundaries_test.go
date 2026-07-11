package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func newRemoteWriteServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/items/ITEM123":
			writeTestJSON(w, map[string]any{
				"ok": true,
				"data": map[string]any{
					"key":       "ITEM123",
					"item_type": "journalArticle",
					"title":     "Remote Test Item",
					"attachments": []map[string]any{
						{
							"key":           "PDF123",
							"item_type":     "attachment",
							"content_type":  "application/pdf",
							"resolved_path": "",
							"zotero_path":   "",
							"resolved":      true,
						},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/items/ITEM123/annotate":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			writeTestJSON(w, map[string]any{
				"ok": true,
				"data": map[string]any{
					"attachment_key": "PDF123",
					"pdf_path":       "",
					"dry_run":        req["dry_run"],
					"matches": []map[string]any{
						{"page": 1, "text": "hello", "rect": []float64{1, 2, 3, 4}, "type": "highlight", "color": "yellow"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/items/ITEM123/annotations/clear":
			writeTestJSON(w, map[string]any{
				"ok": true,
				"data": map[string]any{
					"attachment_key": "PDF123",
					"pdf_path":       "",
					"pdf_deleted":    1,
					"db_deleted":     1,
					"deleted":        2,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestRunAnnotateRemoteUsesServer(t *testing.T) {
	root := t.TempDir()
	setTestConfigDir(t, root)
	writeTestConfig(t, root)

	srv := newRemoteWriteServer(t)
	defer srv.Close()

	t.Setenv("ZOT_MODE", "remote")
	t.Setenv("ZOT_SERVER_ADDR", srv.URL)

	previousLocalReader := testCLI.newLocalReader
	t.Cleanup(func() {
		testCLI.newLocalReader = previousLocalReader
	})
	testCLI.newLocalReader = func(config.Config) (backend.Reader, error) {
		t.Fatal("newLocalReader should not be called in remote annotate")
		return nil, nil
	}

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"ann", "new", "ITEM123", "--text", "hello", "--json"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%q", exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "error:") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunAnnotationsClearRemoteUsesServer(t *testing.T) {
	root := t.TempDir()
	setTestConfigDir(t, root)
	writeTestConfig(t, root)

	srv := newRemoteWriteServer(t)
	defer srv.Close()

	t.Setenv("ZOT_MODE", "remote")
	t.Setenv("ZOT_SERVER_ADDR", srv.URL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"annotations", "ITEM123", "--clear"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%q", exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "error:") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunAnnotateBatchRemoteDryRun(t *testing.T) {
	root := t.TempDir()
	setTestConfigDir(t, root)
	writeTestConfig(t, root)

	srv := newRemoteWriteServer(t)
	defer srv.Close()

	t.Setenv("ZOT_MODE", "remote")
	t.Setenv("ZOT_SERVER_ADDR", srv.URL)

	batchPath := filepath.Join(t.TempDir(), "annotations.json")
	if err := os.WriteFile(batchPath, []byte(`[{"page":1,"text":"hello"},{"item_key":"ITEM123","page":2,"rect":[1,2,3,4]}]`), 0o600); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"annotate", "ITEM123", "--from-file", batchPath, "--dry-run", "--json"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.String())
	}
	if got["ok"] != true {
		t.Fatalf("expected ok response, got %#v", got)
	}
	data := got["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 batch results, got %#v", got["data"])
	}
	for _, raw := range data {
		entry := raw.(map[string]any)
		if entry["dry_run"] != true {
			t.Fatalf("expected dry_run result, got %#v", entry)
		}
		if entry["item_key"] != "ITEM123" {
			t.Fatalf("expected inherited item key, got %#v", entry)
		}
	}
}
