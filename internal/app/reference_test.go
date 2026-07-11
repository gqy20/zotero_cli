package app

import (
	"context"
	"strings"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/references"
)

func TestReferenceStatusUsesConfiguredStoreLifecycle(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir()}
	store, err := openReferenceStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResult(context.Background(), references.Result{ItemKey: "ITEM1", ItemTitle: "Paper", Strategy: "pmc", References: []references.Reference{{Index: 1, Title: "Source"}}}, "fingerprint"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	service := NewReferenceService()
	service.LoadConfig = func() (config.Config, string, error) { return cfg, "", nil }
	result, err := service.Status(context.Background(), ReferenceStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "Reference index: 1 items") {
		t.Fatalf("text = %q", result.Text)
	}
}

func TestReferenceBuildRejectsAmbiguousScope(t *testing.T) {
	service := NewReferenceService()
	_, err := service.Build(context.Background(), ReferenceBuildRequest{Failed: true, Contexts: true})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v", err)
	}
}
