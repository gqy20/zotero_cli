package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func captureOutput(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	oldStdout := stdout
	oldStderr := stderr

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	stdout = out
	stderr = errOut

	t.Cleanup(func() {
		stdout = oldStdout
		stderr = oldStderr
	})

	return out, errOut
}

func restoreOutput() {}

func newTestAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/123456/items":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "X42A7DEE",
					"data": map[string]any{
						"itemType": "conferencePaper",
						"title":    "Attention Is All You Need",
						"date":     "2017",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Ashish",
								"lastName":    "Vaswani",
							},
						},
					},
				},
			})
		case "/users/123456/items/X42A7DEE":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"key": "X42A7DEE",
				"data": map[string]any{
					"itemType":         "conferencePaper",
					"title":            "Attention Is All You Need",
					"date":             "2017",
					"url":              "https://arxiv.org/abs/1706.03762",
					"DOI":              "10.48550/arXiv.1706.03762",
					"proceedingsTitle": "NeurIPS 2017",
					"creators": []map[string]any{
						{
							"creatorType": "author",
							"firstName":   "Ashish",
							"lastName":    "Vaswani",
						},
					},
					"tags": []map[string]any{
						{"tag": "transformers"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}

var _ = os.ErrNotExist
