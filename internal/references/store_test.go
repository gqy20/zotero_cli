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
	if err := store.SaveUnsupported(ctx, "ITEM3", "Outside NCBI", "fp3", &UnsupportedError{ItemKey: "ITEM3", Reason: "not indexed"}); err != nil {
		t.Fatal(err)
	}
	if fresh, err := store.IsFresh(ctx, "ITEM3", "fp3"); err != nil || !fresh {
		t.Fatalf("unsupported fresh=%v err=%v", fresh, err)
	}
	status, err := store.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.IndexedItems != 3 || status.SuccessfulItems != 1 || status.FailedItems != 1 || status.UnsupportedItems != 1 || status.TotalReferences != 1 {
		t.Fatalf("status=%#v", status)
	}
	failed, err := store.Failed(ctx)
	if err != nil || len(failed) != 1 || failed[0].ItemKey != "ITEM2" {
		t.Fatalf("failed=%#v err=%v", failed, err)
	}
	unsupported, err := store.Unsupported(ctx)
	if err != nil || len(unsupported) != 1 || unsupported[0].ItemKey != "ITEM3" {
		t.Fatalf("unsupported=%#v err=%v", unsupported, err)
	}
}

func TestOpenStoreReadOnlyReadsWithoutAllowingWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "index.sqlite")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUnsupported(ctx, "ITEM1", "Paper", "fp", &UnsupportedError{ItemKey: "ITEM1", Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStoreReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	status, err := store.Status(ctx)
	if err != nil || status.UnsupportedItems != 1 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if _, err := store.db.Exec(`DELETE FROM ref_items`); err == nil {
		t.Fatal("read-only store accepted a write")
	}
}

func TestCachedStatusInvalidatesWhenStoreChanges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "index.sqlite")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUnsupported(ctx, "ITEM1", "Paper", "fp", &UnsupportedError{ItemKey: "ITEM1", Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	readOnly, err := OpenStoreReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	status, hit, err := readOnly.CachedStatus(ctx)
	if err != nil || hit || status.UnsupportedItems != 1 {
		t.Fatalf("first status=%#v hit=%v err=%v", status, hit, err)
	}
	_, hit, err = readOnly.CachedStatus(ctx)
	if err != nil || !hit {
		t.Fatalf("second hit=%v err=%v", hit, err)
	}
	_ = readOnly.Close()

	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUnsupported(ctx, "ITEM2", "Paper 2", "fp2", &UnsupportedError{ItemKey: "ITEM2", Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	readOnly, err = OpenStoreReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	status, hit, err = readOnly.CachedStatus(ctx)
	if err != nil || hit || status.UnsupportedItems != 2 {
		t.Fatalf("invalidated status=%#v hit=%v err=%v", status, hit, err)
	}
}

func TestStoreMigratesLegacyUnsupportedFailures(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "index.sqlite")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := errors.New("item LEGACY has no DOI, PMID, or PMCID usable by NCBI")
	if err := store.SaveFailure(ctx, "LEGACY", "Legacy", "fp", legacy); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	status, err := store.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.FailedItems != 0 || status.UnsupportedItems != 1 {
		t.Fatalf("status=%+v", status)
	}
	if fresh, err := store.IsFresh(ctx, "LEGACY", "fp"); err != nil || !fresh {
		t.Fatalf("fresh=%v err=%v", fresh, err)
	}
}
