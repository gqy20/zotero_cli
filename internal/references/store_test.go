package references

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreResultStatusFailureAndFreshness(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "ref", "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result := Result{ItemKey: "ITEM1", ItemTitle: "Source", Identifiers: Identifiers{DOI: "10.1/source", PMID: "1", PMCID: "PMC1"}, Strategy: string(SourcePMC), References: []Reference{{Index: 1, Title: "Cited", DOI: "10.1/cited", Authors: []Author{{Family: "Smith"}}, Source: SourcePMC}}}
	if err := store.SaveResult(ctx, result, "fp1"); err != nil {
		t.Fatal(err)
	}
	if fresh, err := store.IsFresh(ctx, "ITEM1", "fp1"); err != nil || !fresh {
		t.Fatalf("fresh=%v err=%v", fresh, err)
	}
	loaded, ok, err := store.LoadResult(ctx, "ITEM1")
	if err != nil || !ok || len(loaded.References) != 1 || loaded.References[0].Authors[0].Family != "Smith" {
		t.Fatalf("loaded=%#v ok=%v err=%v", loaded, ok, err)
	}
	if err := store.SaveFailure(ctx, "ITEM2", "Broken", "fp2", errors.New("network down")); err != nil {
		t.Fatal(err)
	}
	status, err := store.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.IndexedItems != 2 || status.SuccessfulItems != 1 || status.FailedItems != 1 || status.TotalReferences != 1 {
		t.Fatalf("status=%#v", status)
	}
	failed, err := store.Failed(ctx)
	if err != nil || len(failed) != 1 || failed[0].ItemKey != "ITEM2" {
		t.Fatalf("failed=%#v err=%v", failed, err)
	}
}
