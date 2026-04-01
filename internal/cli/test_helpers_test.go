package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeTestJSON(w http.ResponseWriter, payload any) {
	_ = json.NewEncoder(w).Encode(payload)
}

func writeVersionedBatchResult(w http.ResponseWriter, version string, successful map[string]any, unchanged map[string]any) {
	w.Header().Set("Last-Modified-Version", version)
	if unchanged == nil {
		unchanged = map[string]any{}
	}
	writeTestJSON(w, map[string]any{
		"successful": successful,
		"unchanged":  unchanged,
		"failed":     map[string]any{},
	})
}

func writeVersionedNoContent(w http.ResponseWriter, version string) {
	w.Header().Set("Last-Modified-Version", version)
	w.WriteHeader(http.StatusNoContent)
}

func newTestAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/users/123456/items" {
			var body any
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if items, ok := body.([]any); ok && len(items) > 1 {
					writeVersionedBatchResult(w, "52", map[string]any{
						"0": map[string]any{
							"key":     "ITEMA001",
							"version": 51,
						},
						"1": map[string]any{
							"key":     "ITEMA002",
							"version": 52,
						},
					}, nil)
					return
				}
			}
			writeVersionedBatchResult(w, "42", map[string]any{
				"0": map[string]any{
					"key":     "NEWA1234",
					"version": 42,
				},
			}, nil)
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/users/123456/items" {
			var body []map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) > 0 {
				if tags, ok := body[0]["tags"]; ok && tags != nil {
					writeVersionedBatchResult(w, "53", map[string]any{
						"0": map[string]any{
							"key":     "ITEMA001",
							"version": 53,
						},
						"1": map[string]any{
							"key":     "ITEMA002",
							"version": 53,
						},
					}, nil)
					return
				}
			}
			writeVersionedBatchResult(w, "53", map[string]any{
				"0": map[string]any{
					"key":     "ITEMA001",
					"version": 53,
				},
			}, map[string]any{
				"1": 52,
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/users/123456/items" {
			writeVersionedNoContent(w, "54")
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/users/123456/items/ABCD2345" {
			writeVersionedNoContent(w, "8")
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/users/123456/items/ABCD2345" {
			writeVersionedNoContent(w, "9")
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/users/123456/collections" {
			writeVersionedBatchResult(w, "11", map[string]any{
				"0": map[string]any{
					"key":     "COLLNEW1",
					"version": 11,
				},
			}, nil)
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/users/123456/collections/COLL1234" {
			w.Header().Set("Last-Modified-Version", "12")
			writeTestJSON(w, map[string]any{
				"key":     "COLL1234",
				"version": 12,
				"name":    "Renamed Collection",
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/users/123456/collections/COLL1234" {
			writeVersionedNoContent(w, "13")
			return
		}

		switch r.URL.Path {
		case "/users/123456/items":
			query := r.URL.Query().Get("q")
			itemType := r.URL.Query().Get("itemType")
			limit := r.URL.Query().Get("limit")
			tag := r.URL.Query().Get("tag")
			start := r.URL.Query().Get("start")
			sort := r.URL.Query().Get("sort")
			direction := r.URL.Query().Get("direction")
			qmode := r.URL.Query().Get("qmode")
			includeTrashed := r.URL.Query().Get("includeTrashed")
			format := r.URL.Query().Get("format")
			itemKey := r.URL.Query().Get("itemKey")

			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "111")
				writeTestJSON(w, map[string]int{
					"ITEM1234": 90,
					"ITEM5678": 91,
				})
				return
			}

			if itemKey != "" && format == "bibtex" {
				_, _ = w.Write([]byte("@article{vaswani2017,\n  title = {Attention Is All You Need}\n}\n"))
				return
			}
			if itemKey != "" && format == "csljson" {
				writeTestJSON(w, []map[string]any{
					{"id": "X42A7DEE", "title": "Attention Is All You Need"},
				})
				return
			}
			if itemKey == "ITEMA001,ITEMA002" {
				writeTestJSON(w, []map[string]any{
					{
						"key":     "ITEMA001",
						"version": 52,
						"data": map[string]any{
							"itemType": "book",
							"title":    "Book One",
							"tags": []map[string]any{
								{"tag": "ai"},
							},
						},
					},
					{
						"key":     "ITEMA002",
						"version": 52,
						"data": map[string]any{
							"itemType": "book",
							"title":    "Book Two",
							"tags": []map[string]any{
								{"tag": "ml"},
							},
						},
					},
				})
				return
			}

			if itemType == "note" {
				writeTestJSON(w, []map[string]any{
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
					"key":     "X42A7DEE",
					"version": 42,
					"data": map[string]any{
						"itemType": "conferencePaper",
						"title":    "Attention Is All You Need",
						"date":     "2017",
						"DOI":      "10.5555/attention",
						"url":      "https://example.org/attention",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Ashish",
								"lastName":    "Vaswani",
							},
						},
						"tags": []map[string]any{
							{"tag": "transformers"},
							{"tag": "classic"},
						},
					},
				},
			}

			if query == "mixed" {
				items = []map[string]any{
					{
						"key":     "ART12345",
						"version": 18,
						"data": map[string]any{
							"itemType": "journalArticle",
							"title":    "Primary Article",
							"date":     "2024-06-15",
							"url":      "https://example.org/primary",
							"creators": []map[string]any{
								{
									"creatorType": "author",
									"firstName":   "Ada",
									"lastName":    "Lovelace",
								},
							},
							"tags": []map[string]any{
								{"tag": "ai"},
								{"tag": "classic"},
							},
						},
					},
					{
						"key":     "ATT12345",
						"version": 18,
						"data": map[string]any{
							"itemType": "attachment",
							"title":    "Attachment PDF",
							"date":     "",
						},
					},
					{
						"key":     "NOTE1234",
						"version": 18,
						"data": map[string]any{
							"itemType": "note",
							"title":    "My note",
							"date":     "",
						},
					},
					{
						"key":     "ART67890",
						"version": 21,
						"data": map[string]any{
							"itemType": "journalArticle",
							"title":    "Secondary Article",
							"date":     "2023-02-01",
							"DOI":      "10.5555/secondary",
							"url":      "https://example.org/secondary",
							"creators": []map[string]any{
								{
									"creatorType": "author",
									"firstName":   "Grace",
									"lastName":    "Hopper",
								},
							},
							"tags": []map[string]any{
								{"tag": "ai"},
								{"tag": "survey"},
							},
						},
					},
				}
			}

			if query == "full text" && qmode == "everything" && includeTrashed == "1" {
				items = []map[string]any{
					{
						"key": "TRASH9000",
						"data": map[string]any{
							"itemType": "journalArticle",
							"title":    "Recovered From Trash Search",
							"date":     "2021",
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

			writeTestJSON(w, items)
		case "/users/123456/items/X42A7DEE":
			include := r.URL.Query().Get("include")
			switch include {
			case "citation":
				writeTestJSON(w, map[string]any{
					"key":      "X42A7DEE",
					"citation": "<span>(Vaswani, 2017)</span>",
				})
			case "bib":
				writeTestJSON(w, map[string]any{
					"key": "X42A7DEE",
					"bib": "<div class=\"csl-bib-body\"><div class=\"csl-entry\">Vaswani, A. (2017). <i>Attention Is All You Need</i>.</div></div>",
				})
			default:
				writeTestJSON(w, map[string]any{
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
				writeTestJSON(w, map[string]any{
					"key": "ART12345",
					"bib": "<div class=\"csl-bib-body\"><div class=\"csl-entry\">Lovelace, A. (2024). <i>Primary Article</i>.</div></div>",
				})
			default:
				writeTestJSON(w, map[string]any{
					"key": "ART12345",
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Primary Article",
						"date":     "2024-06-15",
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
		case "/users/123456/items/ART67890":
			include := r.URL.Query().Get("include")
			switch include {
			case "bib":
				writeTestJSON(w, map[string]any{
					"key": "ART67890",
					"bib": "<div class=\"csl-bib-body\"><div class=\"csl-entry\">Hopper, G. (2023). <i>Secondary Article</i>.</div></div>",
				})
			default:
				writeTestJSON(w, map[string]any{
					"key": "ART67890",
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Secondary Article",
						"date":     "2023-02-01",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Grace",
								"lastName":    "Hopper",
							},
						},
					},
				})
			}
		case "/users/123456/items/X42A7DEE/children":
			writeTestJSON(w, []map[string]any{
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
			writeTestJSON(w, []map[string]any{})
		case "/users/123456/collections":
			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "333")
				writeTestJSON(w, map[string]int{
					"COLL1234": 9,
				})
				return
			}
			writeTestJSON(w, []map[string]any{
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
		case "/users/123456/collections/top":
			writeTestJSON(w, []map[string]any{
				{
					"key": "COLLTOP1",
					"data": map[string]any{
						"name":             "Top Folder",
						"parentCollection": false,
					},
					"meta": map[string]any{
						"numCollections": 1,
						"numItems":       8,
					},
				},
			})
		case "/users/123456/tags":
			writeTestJSON(w, []map[string]any{
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
				writeTestJSON(w, map[string]int{
					"ITEM1234": 100,
					"ITEM5678": 101,
				})
				return
			}
			http.NotFound(w, r)
		case "/users/123456/collections/COLL1234/items":
			writeTestJSON(w, []map[string]any{
				{
					"key":     "ART12345",
					"version": 18,
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Primary Article",
						"date":     "2024-06-15",
						"url":      "https://example.org/primary",
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
					"key":     "ART67890",
					"version": 21,
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Secondary Article",
						"date":     "2023-02-01",
						"DOI":      "10.5555/secondary",
						"url":      "https://example.org/secondary",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Grace",
								"lastName":    "Hopper",
							},
						},
					},
				},
			})
		case "/users/123456/items/trash":
			writeTestJSON(w, []map[string]any{
				{
					"key": "TRASH123",
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Removed Paper",
						"date":     "2022",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Dana",
								"lastName":    "Scott",
							},
						},
					},
				},
			})
		case "/users/123456/publications/items":
			writeTestJSON(w, []map[string]any{
				{
					"key": "PUB12345",
					"data": map[string]any{
						"itemType": "journalArticle",
						"title":    "Published Article",
						"date":     "2020",
						"creators": []map[string]any{
							{
								"creatorType": "author",
								"firstName":   "Claude",
								"lastName":    "Shannon",
							},
						},
					},
				},
			})
		case "/users/123456/searches":
			if r.URL.Query().Get("format") == "versions" {
				w.Header().Set("Last-Modified-Version", "444")
				writeTestJSON(w, map[string]int{
					"SCH12345": 12,
				})
				return
			}
			if r.Method == http.MethodPost {
				writeVersionedBatchResult(w, "48", map[string]any{
					"0": map[string]any{
						"key":     "SCH67890",
						"version": 48,
					},
				}, nil)
				return
			}
			writeTestJSON(w, []map[string]any{
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
		case "/users/123456/searches/SCH12345":
			switch r.Method {
			case http.MethodPut:
				w.Header().Set("Last-Modified-Version", "49")
				w.WriteHeader(http.StatusOK)
			case http.MethodDelete:
				writeVersionedNoContent(w, "50")
			default:
				http.NotFound(w, r)
			}
		case "/users/123456/deleted":
			writeTestJSON(w, map[string]any{
				"collections": []string{"COLL1234"},
				"searches":    []string{"SCH12345"},
				"items":       []string{"ITEM1234", "ITEM5678"},
				"tags":        []string{"obsolete"},
			})
		case "/itemTypes":
			writeTestJSON(w, []map[string]any{
				{"itemType": "book", "localized": "Book"},
				{"itemType": "note", "localized": "Note"},
			})
		case "/itemFields":
			writeTestJSON(w, []map[string]any{
				{"field": "title", "localized": "Title"},
				{"field": "url", "localized": "URL"},
			})
		case "/creatorFields":
			writeTestJSON(w, []map[string]any{
				{"field": "firstName", "localized": "First"},
				{"field": "lastName", "localized": "Last"},
			})
		case "/itemTypeFields":
			writeTestJSON(w, []map[string]any{
				{"field": "title", "localized": "Title"},
				{"field": "abstractNote", "localized": "Abstract"},
			})
		case "/itemTypeCreatorTypes":
			writeTestJSON(w, []map[string]any{
				{"creatorType": "author", "localized": "Author"},
				{"creatorType": "editor", "localized": "Editor"},
			})
		case "/items/new":
			writeTestJSON(w, map[string]any{
				"itemType": "book",
				"title":    "",
				"creators": []map[string]any{
					{"creatorType": "author", "firstName": "", "lastName": ""},
				},
				"tags":        []any{},
				"collections": []any{},
				"relations":   map[string]any{},
			})
		case "/keys/current", "/keys/secret":
			writeTestJSON(w, map[string]any{
				"userID": 123456,
				"access": map[string]any{
					"user": map[string]any{
						"library": true,
					},
				},
			})
		case "/users/123456/groups":
			writeTestJSON(w, []map[string]any{
				{
					"id": 111,
					"data": map[string]any{
						"name": "Research Lab",
					},
				},
				{
					"id": 222,
					"data": map[string]any{
						"name": "Paper Club",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}

func newEmptyListAPI(t *testing.T) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/keys/current":
			writeTestJSON(w, map[string]any{
				"userID": 123456,
				"access": map[string]any{
					"user": map[string]any{
						"library": true,
					},
				},
			})
		case "/users/123456/collections", "/users/123456/tags", "/users/123456/searches", "/users/123456/groups", "/users/123456/collections/top", "/users/123456/publications/items":
			writeTestJSON(w, []map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))

	return server.URL, server.Close
}
