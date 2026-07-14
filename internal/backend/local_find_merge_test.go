package backend

import (
	"testing"

	"zotero_cli/internal/domain"
)

func TestMetadataFindScorePrioritizesExactAndTitleMatches(t *testing.T) {
	exact := metadataFindScore(domain.Item{Title: "Genome graph"}, "genome graph")
	prefix := metadataFindScore(domain.Item{Title: "Genome graph methods"}, "genome graph")
	metadata := metadataFindScore(domain.Item{Title: "Other", Tags: []string{"genome graph"}}, "genome graph")
	if !(exact > prefix && prefix > metadata) {
		t.Fatalf("scores exact=%d prefix=%d metadata=%d", exact, prefix, metadata)
	}
}

func TestFullTextCandidateLimitIncludesOffsetWhenNoPostFiltersExist(t *testing.T) {
	opts := FindOptions{In: "fulltext", Start: 20, Limit: 11}
	if got := fullTextCandidateLimit(opts); got != 31 {
		t.Fatalf("candidate limit = %d, want 31", got)
	}
}

func TestFullTextCandidateLimitScansAllBeforeFilteringOrCustomSorting(t *testing.T) {
	tests := []FindOptions{
		{In: "fulltext", Limit: 11, Tags: []string{"genomics"}},
		{In: "fulltext", Limit: 11, Collection: []string{"COLL1"}},
		{In: "fulltext", Limit: 11, Sort: "date"},
		{In: "fulltext", Limit: 11, Sort: "relevance", Direction: "asc"},
	}
	for _, opts := range tests {
		if got := fullTextCandidateLimit(opts); got != 0 {
			t.Fatalf("candidate limit for %#v = %d, want complete scan", opts, got)
		}
	}
}
