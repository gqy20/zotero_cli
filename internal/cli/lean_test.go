package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"zotero_cli/internal/domain"
)

func TestToLeanItemBasicFields(t *testing.T) {
	item := domain.Item{
		Key:       "ABC123",
		ItemType:  "journalArticle",
		Title:     "Attention Is All You Need",
		Date:      "2024-01-08",
		Container: "NeurIPS",
		Volume:    "37",
		Issue:     "1",
		Pages:     "1234-1248",
		DOI:       "10.48550/arXiv.1706.03762",
		URL:       "https://arxiv.org/abs/1706.03762",
		Tags:      []string{"transformer", "nlp"},
		DateAdded: "2024-06-01 00:00:00",
		MatchedOn: []string{"title"},
	}

	lean := toLeanItem(item, false)

	if lean.Key != "ABC123" {
		t.Errorf("Key = %q, want %q", lean.Key, "ABC123")
	}
	if lean.ItemType != "journalArticle" {
		t.Errorf("ItemType = %q, want %q", lean.ItemType, "journalArticle")
	}
	if lean.Title != "Attention Is All You Need" {
		t.Errorf("Title = %q, want %q", lean.Title, "Attention Is All You Need")
	}
	if lean.Date != "2024-01-08" {
		t.Errorf("Date = %q, want %q", lean.Date, "2024-01-08")
	}
	if lean.Container != "NeurIPS" {
		t.Errorf("Container = %q, want %q", lean.Container, "NeurIPS")
	}
	if lean.Volume != "37" {
		t.Errorf("Volume = %q, want %q", lean.Volume, "37")
	}
	if lean.Issue != "1" {
		t.Errorf("Issue = %q, want %q", lean.Issue, "1")
	}
	if lean.Pages != "1234-1248" {
		t.Errorf("Pages = %q, want %q", lean.Pages, "1234-1248")
	}
	if lean.DOI != "10.48550/arXiv.1706.03762" {
		t.Errorf("DOI = %q, want %q", lean.DOI, "10.48550/arXiv.1706.03762")
	}
	if lean.URL != "https://arxiv.org/abs/1706.03762" {
		t.Errorf("URL = %q, want %q", lean.URL, "https://arxiv.org/abs/1706.03762")
	}
	if len(lean.Tags) != 2 || lean.Tags[0] != "transformer" || lean.Tags[1] != "nlp" {
		t.Errorf("Tags = %v, want [transformer nlp]", lean.Tags)
	}
	if lean.DateAdded != "2024-06-01 00:00:00" {
		t.Errorf("DateAdded = %q, want %q", lean.DateAdded, "2024-06-01 00:00:00")
	}
	if len(lean.MatchedOn) != 1 || lean.MatchedOn[0] != "title" {
		t.Errorf("MatchedOn = %v, want [title]", lean.MatchedOn)
	}
}

func TestToLeanItemCreatorsSummary(t *testing.T) {
	tests := []struct {
		name     string
		creators []domain.Creator
		want     string
	}{
		{
			name:     "single creator",
			creators: []domain.Creator{{Name: "Alice Smith", CreatorType: "author"}},
			want:     "Alice Smith",
		},
		{
			name: "two creators",
			creators: []domain.Creator{
				{Name: "Alice Smith", CreatorType: "author"},
				{Name: "Bob Jones", CreatorType: "author"},
			},
			want: "Alice Smith et al.",
		},
		{
			name: "many creators",
			creators: []domain.Creator{
				{Name: "A", CreatorType: "author"},
				{Name: "B", CreatorType: "author"},
				{Name: "C", CreatorType: "editor"},
				{Name: "D", CreatorType: "author"},
			},
			want: "A et al.",
		},
		{
			name:     "no creators",
			creators: nil,
			want:     "",
		},
		{
			name:     "empty creator list",
			creators: []domain.Creator{},
			want:     "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := domain.Item{Creators: tc.creators}
			lean := toLeanItem(item, false)
			if lean.CreatorsSummary != tc.want {
				t.Errorf("CreatorsSummary = %q, want %q", lean.CreatorsSummary, tc.want)
			}
		})
	}
}

func TestToLeanItemDropsVerboseFields(t *testing.T) {
	item := domain.Item{
		Key:             "K1",
		Title:           "Verbose Item",
		ItemType:        "journalArticle",
		Abstract:        "This is a very long abstract text that exceeds normal lengths and should be removed in lean mode...",
		Attachments:     []domain.Attachment{{Key: "ATT1", Title: "paper.pdf", Filename: "paper.pdf"}},
		Notes:           []domain.Note{{Key: "N1", Content: "note content here"}},
		Annotations:     []domain.Annotation{{Key: "A1", Type: "highlight", Text: "important passage"}},
		JournalRank:     &domain.JournalRank{Ranks: map[string]string{"sciif": "15.0", "jci": "2.5"}},
		MatchedChunk:    &domain.MatchedChunkInfo{Text: "chunk text", Page: 5},
		FullTextPreview: "full text preview content...",
		Version:         99,
		Creators:        []domain.Creator{{Name: "Author One", CreatorType: "author"}},
	}

	lean := toLeanItem(item, false)

	data, err := json.Marshal(lean)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)

	for _, forbidden := range []string{
		`"abstract"`, `"attachments"`, `"notes"`,
		`"annotations"`, `"journal_rank"`, `"matched_chunk"`,
		`"full_text_preview"`, `"version"`,
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("lean JSON must not contain %q, got:\n%s", forbidden, s)
		}
	}
}

func TestToLeanItemIncludesAbstractWhenRequested(t *testing.T) {
	item := domain.Item{
		Key:      "K1",
		Title:    "Paper with abstract",
		ItemType: "journalArticle",
		Abstract: "The actual abstract content that matters.",
		Creators: []domain.Creator{{Name: "Author"}},
	}

	// Without abstract
	leanNoAbs := toLeanItem(item, false)
	dataNoAbs, _ := json.Marshal(leanNoAbs)
	if strings.Contains(string(dataNoAbs), `"abstract"`) {
		t.Error("lean without includeAbstract must not contain abstract field")
	}

	// With abstract
	leanWithAbs := toLeanItem(item, true)
	dataWithAbs, _ := json.Marshal(leanWithAbs)
	if !strings.Contains(string(dataWithAbs), `"abstract"`) {
		t.Error("lean with includeAbstract=true must contain abstract field")
	}
	if leanWithAbs.Abstract != "The actual abstract content that matters." {
		t.Errorf("Abstract = %q, want full text", leanWithAbs.Abstract)
	}
}

func TestToLeanItemCollectionsNamesOnly(t *testing.T) {
	item := domain.Item{
		Key:      "K1",
		Title:    "Collected Paper",
		ItemType: "journalArticle",
		Collections: []domain.Collection{
			{Key: "COLL1", Name: "AI Papers"},
			{Key: "COLL2", Name: "ML Papers"},
		},
		Creators: []domain.Creator{{Name: "Author"}},
	}

	lean := toLeanItem(item, false)

	if len(lean.Collections) != 2 {
		t.Fatalf("Collections length = %d, want 2", len(lean.Collections))
	}
	if lean.Collections[0] != "AI Papers" {
		t.Errorf("Collections[0] = %q, want %q", lean.Collections[0], "AI Papers")
	}
	if lean.Collections[1] != "ML Papers" {
		t.Errorf("Collections[1] = %q, want %q", lean.Collections[1], "ML Papers")
	}

	// Verify no collection keys in JSON output
	data, _ := json.Marshal(lean)
	s := string(data)
	// The word "key" should not appear near collection names (no struct keys in output)
	if strings.Contains(s, `"COLL1"`) || strings.Contains(s, `"COLL2"`) {
		t.Errorf("collection keys should not appear in lean JSON, got:\n%s", s)
	}
}

func TestToLeanItemEmptyFieldsOmitted(t *testing.T) {
	item := domain.Item{
		Key:      "K1",
		ItemType: "book",
		Title:    "Minimal Work",
		Creators: []domain.Creator{{Name: "Solo Author"}},
	}

	lean := toLeanItem(item, false)
	data, err := json.Marshal(lean)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)

	// These omitempty fields should NOT appear as null in JSON
	for _, field := range []string{"date", "container", "volume", "issue", "pages", "doi", "url", "date_added"} {
		pattern := `"` + field + `":null`
		if strings.Contains(s, pattern) {
			t.Errorf("field %q should be omitted when empty, not null; got:\n%s", field, s)
		}
	}
}

func TestToLeanItemsBatch(t *testing.T) {
	items := []domain.Item{
		{Key: "A", ItemType: "journalArticle", Title: "First", Creators: []domain.Creator{{Name: "A1"}}},
		{Key: "B", ItemType: "book", Title: "Second", Creators: []domain.Creator{{Name: "B1"}}},
		{Key: "C", ItemType: "preprint", Title: "Third"},
	}

	leans := toLeanItems(items, false)

	if len(leans) != 3 {
		t.Fatalf("len(leans) = %d, want 3", len(leans))
	}
	if leans[0].Key != "A" || leans[1].Key != "B" || leans[2].Key != "C" {
		t.Errorf("keys mismatch: %v", []string{leans[0].Key, leans[1].Key, leans[2].Key})
	}
	if leans[0].Title != "First" || leans[1].Title != "Second" || leans[2].Title != "Third" {
		t.Errorf("titles mismatch")
	}
}
