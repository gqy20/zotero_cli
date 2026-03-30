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
			query := r.URL.Query().Get("q")
			itemType := r.URL.Query().Get("itemType")
			limit := r.URL.Query().Get("limit")

			items := []map[string]any{
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
			}

			if query == "mixed" {
				items = []map[string]any{
					{
						"key": "ART12345",
						"data": map[string]any{
							"itemType": "journalArticle",
							"title":    "Primary Article",
							"date":     "2024",
							"creators": []map[string]any{
								{
									"creatorType": "author",
									"firstName":   "Ada",
									"lastName":    "Lovelace",
								},
							},
						},
					},
					{
						"key": "ATT12345",
						"data": map[string]any{
							"itemType": "attachment",
							"title":    "Attachment PDF",
							"date":     "",
						},
					},
					{
						"key": "NOTE1234",
						"data": map[string]any{
							"itemType": "note",
							"title":    "My note",
							"date":     "",
						},
					},
					{
						"key": "ART67890",
						"data": map[string]any{
							"itemType": "journalArticle",
							"title":    "Secondary Article",
							"date":     "2023",
							"creators": []map[string]any{
								{
									"creatorType": "author",
									"firstName":   "Grace",
									"lastName":    "Hopper",
								},
							},
						},
					},
				}
			}

			if itemType != "" {
				filtered := make([]map[string]any, 0, len(items))
				for _, item := range items {
					data, _ := item["data"].(map[string]any)
					if data["itemType"] == itemType {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}

			if limit == "1" && len(items) > 1 {
				items = items[:1]
			}

			_ = json.NewEncoder(w).Encode(items)
		case "/users/123456/items/X42A7DEE":
			include := r.URL.Query().Get("include")
			switch include {
			case "citation":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"key":      "X42A7DEE",
					"citation": "<span>(Vaswani, 2017)</span>",
				})
			case "bib":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"key": "X42A7DEE",
					"bib": "<div class=\"csl-bib-body\"><div class=\"csl-entry\">Vaswani, A. (2017). <i>Attention Is All You Need</i>.</div></div>",
				})
			default:
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
			}
		case "/users/123456/items/ART12345":
			include := r.URL.Query().Get("include")
			switch include {
			case "bib":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"key": "ART12345",
					"bib": "<div class=\"csl-bib-body\"><div class=\"csl-entry\">Lovelace, A. (2024). <i>Primary Article</i>.</div></div>",
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"key": "ART12345",
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Primary Article",
						"date":     "2024",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Ada",
								"lastName":    "Lovelace",
							},
						},
					},
				})
			}
		case "/users/123456/items/X42A7DEE/children":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "PDF12345",
					"data": map[string]any{
						"itemType":    "attachment",
						"title":       "Attention Is All You Need PDF",
						"contentType": "application/pdf",
						"linkMode":    "imported_file",
						"filename":    "attention-is-all-you-need.pdf",
					},
				},
				{
					"key": "URL12345",
					"data": map[string]any{
						"itemType": "attachment",
						"title":    "Notion",
						"linkMode": "linked_url",
					},
				},
			})
		case "/users/123456/items/ART12345/children":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}

var _ = os.ErrNotExist
