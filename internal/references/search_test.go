package references

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreSearchReferencesAndContexts(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result := Result{ItemKey: "SOURCE", ItemTitle: "Source paper", Strategy: string(SourcePMC), References: []Reference{{Index: 1, ID: "b1", Title: "Genome assembly with long reads", DOI: "10.1/a", Source: SourcePMC, TargetItemKey: "TARGET", MatchStatus: "resolved", MatchMethod: "doi", MatchScore: 1}}, Contexts: []Context{{ReferenceID: "b1", ReferenceIndex: 1, Marker: "[1]", Section: "Methods", Paragraph: "We used long-read sequencing for genome reconstruction.", Source: SourcePMC}}}
	if err := store.SaveResult(ctx, result, "fp"); err != nil {
		t.Fatal(err)
	}
	hits, err := store.Search(ctx, SearchOptions{Query: "genome assembly", In: "references", Limit: 10})
	if err != nil || len(hits) != 1 || hits[0].MatchedOn[0] != "reference" {
		t.Fatalf("reference hits=%+v err=%v", hits, err)
	}
	hits, err = store.Search(ctx, SearchOptions{Query: "long read sequencing", In: "contexts", Section: "Method", Limit: 10})
	if err != nil || len(hits) != 1 || len(hits[0].Contexts) != 1 || hits[0].MatchedOn[0] != "context" {
		t.Fatalf("context hits=%+v err=%v", hits, err)
	}
	hits, err = store.Search(ctx, SearchOptions{Query: "genome", Target: "TARGET", Source: string(SourcePMC), Limit: 10})
	if err != nil || len(hits) != 1 || len(hits[0].MatchedOn) != 2 {
		t.Fatalf("combined hits=%+v err=%v", hits, err)
	}
	if err := store.SaveResult(ctx, result, "fp2"); err != nil {
		t.Fatal(err)
	}
	hits, err = store.Search(ctx, SearchOptions{Query: "genome", Limit: 10})
	if err != nil || len(hits) != 1 {
		t.Fatalf("refresh duplicated hits=%+v err=%v", hits, err)
	}
}
