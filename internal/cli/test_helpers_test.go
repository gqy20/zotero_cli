package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func setTestConfigDir(t *testing.T, root string) {
	t.Helper()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
}

func writeTestConfig(t *testing.T, root string) {
	t.Helper()

	configDir := filepath.Join(root, "zotcli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configJSON := `{
  "mode": "web",
  "library_type": "user",
  "library_id": "123456",
  "api_key": "secret",
  "style": "apa",
  "locale": "en-US",
  "timeout_seconds": 20
}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newTestAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/123456/items":
			query := r.URL.Query().Get("q")
			itemType := r.URL.Query().Get("itemType")
			limit := r.URL.Query().Get("limit")
			tag := r.URL.Query().Get("tag")
			start := r.URL.Query().Get("start")
			sort := r.URL.Query().Get("sort")
			direction := r.URL.Query().Get("direction")

			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "111")
				_ = json.NewEncoder(w).Encode(map[string]int{
					"ITEM1234": 90,
					"ITEM5678": 91,
				})
				return
			}

			if itemType == "note" {
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{
						"key": "NOTE1111",
						"data": map[string]any{
							"itemType": "note",
							"note":     "<p>Key finding about transformers</p>",
						},
					},
					{
						"key": "NOTE2222",
						"data": map[string]any{
							"itemType": "note",
							"note":     "<p>Follow-up reading list</p>",
						},
					},
					{
						"key": "NOTE3333",
						"data": map[string]any{
							"itemType": "note",
							"note":     "<p>X42A7DEE {\"readingTime\":1234,\"progress\":0.7}</p>",
						},
					},
				})
				return
			}

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

			if tag == "ai" {
				filtered := make([]map[string]any, 0, len(items))
				for _, item := range items {
					if item["key"] == "ART67890" {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}

			if start == "1" && len(items) > 1 {
				items = items[1:]
			}

			if sort == "title" && direction == "asc" && len(items) > 1 {
				items[0], items[1] = items[1], items[0]
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
		case "/users/123456/collections":
			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "333")
				_ = json.NewEncoder(w).Encode(map[string]int{
					"COLL1234": 9,
				})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "COLL1234",
					"data": map[string]any{
						"name":             "Projects",
						"parentCollection": false,
					},
					"meta": map[string]any{
						"numCollections": 2,
						"numItems":       5,
					},
				},
				{
					"key": "COLL5678",
					"data": map[string]any{
						"name":             "Reading",
						"parentCollection": "COLL1234",
					},
					"meta": map[string]any{
						"numCollections": 0,
						"numItems":       12,
					},
				},
			})
		case "/users/123456/tags":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"tag": "transformers",
					"meta": map[string]any{
						"numItems": 4,
					},
				},
				{
					"tag": "ai",
					"meta": map[string]any{
						"numItems": 2,
					},
				},
			})
		case "/users/123456/items/top":
			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "222")
				_ = json.NewEncoder(w).Encode(map[string]int{
					"ITEM1234": 100,
					"ITEM5678": 101,
				})
				return
			}
			http.NotFound(w, r)
		case "/users/123456/searches":
			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "444")
				_ = json.NewEncoder(w).Encode(map[string]int{
					"SCH12345": 12,
				})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "SCH12345",
					"data": map[string]any{
						"name": "Unread PDFs",
						"conditions": []map[string]any{
							{"condition": "itemType", "operator": "is", "value": "attachment"},
							{"condition": "tag", "operator": "contains", "value": "pdf"},
						},
					},
				},
			})
		case "/users/123456/deleted":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"collections": []string{"COLL1234"},
				"searches":    []string{"SCH12345"},
				"items":       []string{"ITEM1234", "ITEM5678"},
				"tags":        []string{"obsolete"},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}

func newMachineOnlyNotesAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/123456/items":
			if r.URL.Query().Get("itemType") != "note" {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"key": "NOTE9000",
					"data": map[string]any{
						"itemType": "note",
						"note":     "<p>ITEM1234 {\"readingTime\":88}</p>",
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

type errorAPIServer struct {
	url     string
	cleanup func()
}

func newErrorAPI(t *testing.T, status int, retryAfter string) errorAPIServer {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		http.Error(w, http.StatusText(status), status)
	}))

	return errorAPIServer{
		url:     server.URL,
		cleanup: server.Close,
	}
}

func newConditionalVersionsAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123456/items" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("format") != "versions" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("If-Modified-Since-Version"); got != "88" {
			t.Fatalf("unexpected If-Modified-Since-Version: %q", got)
		}

		w.WriteHeader(http.StatusNotModified)
	}))

	return server.URL, server.Close
}
