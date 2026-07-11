package references

import (
	"testing"
	"zotero_cli/internal/domain"
)

func TestResolverIdentifierAndTitleRoutes(t *testing.T) {
	r := NewResolver([]domain.Item{{Key: "DOI", Title: "DOI paper", DOI: "10.1000/ABC"}, {Key: "PMID", Title: "PubMed paper", URL: "https://pubmed.ncbi.nlm.nih.gov/12345/"}, {Key: "TITLE", Title: "Single-cell analysis of immune responses"}})
	tests := []struct {
		ref         Reference
		key, method string
	}{{Reference{DOI: "10.1000/abc"}, "DOI", "doi"}, {Reference{PMID: "12345"}, "PMID", "pmid"}, {Reference{Title: "Single-cell analysis of immune responses"}, "TITLE", "title_exact"}, {Reference{Title: "Single-cell analysis of immune responses today"}, "TITLE", "title_fuzzy"}}
	for _, test := range tests {
		got := r.Resolve(test.ref, "SOURCE")
		if got.TargetItemKey != test.key || got.MatchMethod != test.method {
			t.Errorf("Resolve(%+v) = %+v", test.ref, got)
		}
	}
}

func TestResolverRejectsWeakAndSelfMatches(t *testing.T) {
	r := NewResolver([]domain.Item{{Key: "A", Title: "A detailed study of cancer cell signaling", DOI: "10.1/a"}})
	if got := r.Resolve(Reference{DOI: "10.1/a"}, "A"); got.MatchStatus != "unresolved" {
		t.Fatalf("self match = %+v", got)
	}
	if got := r.Resolve(Reference{Title: "Cancer study"}, "B"); got.MatchStatus != "unresolved" {
		t.Fatalf("weak match = %+v", got)
	}
}
