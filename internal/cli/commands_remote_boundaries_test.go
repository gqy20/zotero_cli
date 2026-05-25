package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	exitCode := Run([]string{"annotate", "ITEM123", "--text", "hello", "--json"})
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
