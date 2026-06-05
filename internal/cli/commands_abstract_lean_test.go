package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

// abstractStubReader returns items from a lookup map by key.
type abstractStubReader struct {
	itemMap map[string]domain.Item
}

func (r abstractStubReader) FindItems(_ context.Context, opts backend.FindOptions) ([]domain.Item, error) {
	return nil, backend.ErrUnsupportedFeature
}
func (r abstractStubReader) GetItem(_ context.Context, key string) (domain.Item, error) {
	if item, ok := r.itemMap[key]; ok {
		return item, nil
	}
	return domain.Item{}, backend.ErrItemNotFound
}
func (r abstractStubReader) GetRelated(_ context.Context, _ string) ([]domain.Relation, error) {
	return nil, nil
}
func (r abstractStubReader) GetLibraryStats(_ context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}
func (r abstractStubReader) ListNotes(_ context.Context) ([]domain.Note, error) { return nil, nil }
func (r abstractStubReader) ListTags(_ context.Context) ([]backend.Tag, error)  { return nil, nil }
func (r abstractStubReader) ListCollections(_ context.Context) ([]backend.Collection, error) {
	return nil, nil
}
func (r abstractStubReader) GetAttachmentFile(_ context.Context, _ string) (string, string, error) {
	return "", "", backend.ErrItemNotFound
}

func TestAbstractJSONDefaultIsLeanWithAbstract(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item := domain.Item{
		Key:       "ABS1",
		ItemType:  "journalArticle",
		Title:     "Hybrid Speciation in Plants",
		Date:      "2007",
		Creators:  []domain.Creator{{Name: "Baack, EJ"}, {Name: "Rieseberg, LH"}},
		Container: "Current Opinion in Genetics & Development",
		Volume:    "17",
		Issue:     "6",
		Pages:     "513-518",
		DOI:       "10.1016/j.gde.2007.09.001",
		Tags:      []string{"speciation", "hybridization"},
		Abstract:  "Hybridization has long been recognized as an important process in plant evolution. We review the genomic consequences of hybridization and introgression, focusing on homoploid hybrid speciation as a mechanism for rapid ecological adaptation and reproductive isolation.",
		Attachments: []domain.Attachment{
			{Key: "ATT_PDF", ItemType: "attachment", Title: "paper.pdf", ContentType: "application/pdf"},
		},
		Notes: []domain.Note{
			{Key: "NOTE1", Content: "Good review of HHS theory"},
		},
		Annotations: []domain.Annotation{
			{Key: "ANN1", Type: "highlight", Text: "homoploid hybrid speciation", Comment: "key concept"},
			{Key: "ANN2", Type: "note", Comment: "See also figure 2"},
		},
		JournalRank: &domain.JournalRank{
			Ranks: map[string]string{"sciif": "4.2", "sci": "Q2"},
		},
	}

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: item}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"abstract", "ABS1", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	dataArr, ok := got["data"].([]any)
	if !ok || len(dataArr) != 1 {
		t.Fatalf("expected data array with 1 item, got: %#v", got["data"])
	}
	data := dataArr[0].(map[string]any)

	// Core fields present
	if data["key"] != "ABS1" {
		t.Errorf("key = %q, want ABS1", data["key"])
	}
	if data["title"] != "Hybrid Speciation in Plants" {
		t.Errorf("title = %q", data["title"])
	}
	if data["container"] != "Current Opinion in Genetics & Development" {
		t.Errorf("container = %q", data["container"])
	}

	// Creators folded
	creators, ok := data["creators"].(string)
	if !ok || creators != "Baack, EJ et al." {
		t.Errorf("creators = %q (%T), want 'Baack, EJ et al.'", data["creators"], data["creators"])
	}

	// Abstract MUST be present (abstract command exception)
	if data["abstract"] == nil || data["abstract"] == "" {
		t.Error("abstract --json must include abstract field even in lean mode")
	}
	absStr := data["abstract"].(string)
	if len(absStr) < 20 {
		t.Errorf("abstract too short: %q", absStr)
	}

	// But other verbose fields must NOT exist
	for _, forbidden := range []string{"attachments", "notes", "annotations", "journal_rank"} {
		if _, exists := data[forbidden]; exists {
			t.Errorf("abstract --json lean must not contain %q", forbidden)
		}
	}
}

func TestAbstractJSONMultipleItemsAllLean(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item1 := domain.Item{
		Key: "ABS_A", Title: "Paper A", ItemType: "journalArticle",
		Creators:    []domain.Creator{{Name: "Author A"}},
		Abstract:    "Abstract A content here.",
		Attachments: []domain.Attachment{{Key: "ATT1"}},
		Annotations: []domain.Annotation{{Key: "ANN1"}},
	}
	item2 := domain.Item{
		Key: "ABS_B", Title: "Paper B", ItemType: "book",
		Creators: []domain.Creator{{Name: "Author B"}},
		// No abstract
		Attachments: []domain.Attachment{{Key: "ATT2"}},
		Notes:       []domain.Note{{Key: "N1"}},
	}
	item3 := domain.Item{
		Key: "ABS_C", Title: "Paper C", ItemType: "preprint",
		Creators:    []domain.Creator{{Name: "Author C1"}, {Name: "Author C2"}},
		Abstract:    "Abstract C content here.",
		JournalRank: &domain.JournalRank{Ranks: map[string]string{"sciif": "10.0"}},
	}

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })

	// Build a lookup map so GetItem returns the correct item per key
	itemMap := map[string]domain.Item{
		"ABS_A": item1,
		"ABS_B": item2,
		"ABS_C": item3,
	}
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return abstractStubReader{itemMap: itemMap}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"abstract", "ABS_A", "ABS_B", "ABS_C", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var got map[string]any
	json.Unmarshal(stdout.Bytes(), &got)
	dataArr := got["data"].([]any)

	if len(dataArr) != 3 {
		t.Fatalf("expected 3 items, got %d", len(dataArr))
	}

	for i, raw := range dataArr {
		item := raw.(map[string]any)

		// All items must be lean (no attachments/notes/annotations/journal_rank)
		for _, forbidden := range []string{"attachments", "notes", "annotations", "journal_rank"} {
			if _, exists := item[forbidden]; exists {
				t.Errorf("item[%d][%q] should not exist in lean output", i, forbidden)
			}
		}

		// Items with abstract should have it
		if i == 0 && (item["abstract"] == nil || item["abstract"] == "") {
			t.Error("item[0] should have abstract")
		}
		if i == 2 && (item["abstract"] == nil || item["abstract"] == "") {
			t.Error("item[2] should have abstract")
		}
		// Item without abstract should not have it or it's empty
		if i == 1 {
			if abs, ok := item["abstract"]; ok && abs != "" && abs != nil {
				t.Error("item[1] has no abstract but field is non-empty")
			}
		}
	}
}

func TestAbstractJSONPayloadSize(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item := verboseTestItem()

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: item}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"abstract", "VERBOSE1", "--json"})
	if exitCode != 0 {
		t.Fatalf("exit code %d", exitCode)
	}

	payloadSize := stdout.Len()
	// Lean single-item target: includes abstract text (~500 chars) + other fields
	// Full would be > 25 KB due to annotations/journal_rank/attachments
	if payloadSize > 3000 {
		t.Errorf("lean payload too large: %d bytes (expecting < 3 KB for single item, full would be >25 KB)", payloadSize)
	}
}
