package backend

import (
	"reflect"
	"strings"
	"testing"
)

func TestLocalFindQueryPushesDownSimplePagination(t *testing.T) {
	query, args := localFindQuery(FindOptions{All: true, Limit: 5, Start: 10})
	if !strings.Contains(query, "LIMIT ? OFFSET ?") {
		t.Fatalf("expected SQL pagination, query:\n%s", query)
	}
	if got := args[len(args)-2:]; !reflect.DeepEqual(got, []any{5, 10}) {
		t.Fatalf("pagination args = %#v", got)
	}
}

func TestLocalFindQueryKeepsPostFilteredPaginationInGo(t *testing.T) {
	for _, opts := range []FindOptions{
		{Limit: 5, Query: "biology"},
		{Limit: 5, Sort: "title"},
		{Limit: 5, DateAfter: "2020"},
		{Limit: 5, HasPDF: true},
	} {
		query, _ := localFindQuery(opts)
		if strings.Contains(query, "LIMIT ? OFFSET ?") {
			t.Fatalf("unsafe SQL pagination for %#v", opts)
		}
	}
}

func TestLocalFindQueryPushesDownDescendingKeyOrder(t *testing.T) {
	query, _ := localFindQuery(FindOptions{Limit: 5, Direction: "desc"})
	if !strings.Contains(query, "ORDER BY i.key DESC") {
		t.Fatalf("expected descending key order, query:\n%s", query)
	}
}

func TestLocalExactKeyFindQueryUsesIndexedKeyPredicate(t *testing.T) {
	query, args := localExactKeyFindQuery(FindOptions{Query: "22RXXF7B", Limit: 5})
	if !strings.Contains(query, "i.key = ?") || strings.Contains(query, "LOWER(i.key)") {
		t.Fatalf("expected exact key predicate, query:\n%s", query)
	}
	if len(args) == 0 || args[0] != "22RXXF7B" {
		t.Fatalf("args = %#v", args)
	}
}

func TestLocalExactKeyFastPathHasNarrowSemanticBoundary(t *testing.T) {
	if !localCanUseExactKeyFastPath(FindOptions{Query: "22RXXF7B", Limit: 5}) {
		t.Fatal("expected simple key query to use fast path")
	}
	for _, opts := range []FindOptions{
		{Query: "22rxxf7b", Limit: 5},
		{Query: "22RXXF7B", Limit: 0},
		{Query: "22RXXF7B", Limit: 5, Start: 1},
		{Query: "22RXXF7B", Limit: 5, Tags: []string{"reviewed"}},
		{Query: "22RXXF7B", Limit: 5, HasPDF: true},
	} {
		if localCanUseExactKeyFastPath(opts) {
			t.Fatalf("unexpected exact key fast path for %#v", opts)
		}
	}
}

func TestLocalPaginationBeforeHydrationSafety(t *testing.T) {
	if !localCanPaginateBeforeHydration(FindOptions{Query: "biology", Limit: 5, Sort: "title", DateAfter: "2020"}) {
		t.Fatal("metadata-only post filters should paginate before hydration")
	}
	if localCanPaginateBeforeHydration(FindOptions{Query: "biology", Limit: 5, AttachmentName: "paper.pdf"}) {
		t.Fatal("attachment filters require hydration before pagination")
	}
}

func TestLocalCandidateExactKeyUsesKeyIndexShape(t *testing.T) {
	query, args := localExactKeyCandidateQuery(FindOptions{Query: "22RXXF7B", Limit: 5})
	if !strings.Contains(query, "i.key = ?") || strings.Contains(query, "LOWER(i.key)") {
		t.Fatalf("exact candidate query does not preserve indexable key predicate:\n%s", query)
	}
	if len(args) == 0 || args[0] != "22RXXF7B" {
		t.Fatalf("args = %#v", args)
	}
}
