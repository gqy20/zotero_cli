package references

import (
	"context"
	"path/filepath"
	"testing"

	"zotero_cli/internal/domain"
)

func TestStoreResolveCitedByAndContexts(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "ref.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result := Result{ItemKey: "SOURCE", ItemTitle: "Source", Strategy: "pmc_jats", References: []Reference{{Index: 1, ID: "B1", Title: "Target article", DOI: "10.1000/target", Source: SourcePMC}}, Contexts: []Context{{ReferenceID: "B1", ReferenceIndex: 1, Marker: "[1]", Section: "Results", Paragraph: "Evidence was reported [1].", Source: SourcePMC}}}
	if err := store.SaveResult(ctx, result, "fp"); err != nil {
		t.Fatal(err)
	}
	report, err := store.Resolve(ctx, NewResolver([]domain.Item{{Key: "TARGET", Title: "Target article", DOI: "10.1000/target"}}))
	if err != nil {
		t.Fatal(err)
	}
	if report.Resolved != 1 || report.DOI != 1 {
		t.Fatalf("report = %+v", report)
	}
	cited, err := store.CitedBy(ctx, "TARGET")
	if err != nil {
		t.Fatal(err)
	}
	if len(cited) != 1 || len(cited[0].Contexts) != 1 || cited[0].Contexts[0].TargetItemKey != "TARGET" {
		t.Fatalf("cited = %+v", cited)
	}
	loaded, ok, err := store.LoadResult(ctx, "SOURCE")
	if err != nil || !ok {
		t.Fatalf("load ok=%v err=%v", ok, err)
	}
	if loaded.References[0].TargetItemKey != "TARGET" || len(loaded.Contexts) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
}
