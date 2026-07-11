package backend

import (
	"testing"

	"zotero_cli/internal/domain"
)

func TestMergeFindItemsDeduplicatesAndPreservesFullTextMatch(t *testing.T) {
	metadata := []domain.Item{{Key: "A", Title: "Exact title", MatchedOn: []string{"metadata"}, SearchScore: 1_000_003}}
	fulltext := []domain.Item{
		{Key: "A", MatchedOn: []string{"fulltext_attachment"}, SnippetAttachmentKey: "ATT1", MatchedChunk: &domain.MatchedChunkInfo{Text: "hit"}},
		{Key: "B", Title: "Body only", MatchedOn: []string{"fulltext_attachment"}, SearchScore: 2},
	}

	got := mergeFindItems(metadata, fulltext)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].SnippetAttachmentKey != "ATT1" || got[0].MatchedChunk == nil {
		t.Fatalf("full-text context was not merged: %#v", got[0])
	}
	if len(got[0].MatchedOn) != 2 || got[0].MatchedOn[1] != "fulltext_attachment" {
		t.Fatalf("matched_on = %v", got[0].MatchedOn)
	}
}

func TestMetadataFindScorePrioritizesExactAndTitleMatches(t *testing.T) {
	exact := metadataFindScore(domain.Item{Title: "Genome graph"}, "genome graph")
	prefix := metadataFindScore(domain.Item{Title: "Genome graph methods"}, "genome graph")
	metadata := metadataFindScore(domain.Item{Title: "Other", Tags: []string{"genome graph"}}, "genome graph")
	if !(exact > prefix && prefix > metadata) {
		t.Fatalf("scores exact=%d prefix=%d metadata=%d", exact, prefix, metadata)
	}
}
