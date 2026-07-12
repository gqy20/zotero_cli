package app

import (
	"context"
	"database/sql"
	"os"
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
	if result.Meta["read_mode"] != "read_only" {
		t.Fatalf("meta = %#v", result.Meta)
	}
	second, err := service.Status(context.Background(), ReferenceStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Meta["status_cache_hit"] != true {
		t.Fatalf("second meta = %#v", second.Meta)
	}
}

func TestReferenceStatusDoesNotCreateMissingIndex(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir()}
	service := NewReferenceService()
	service.LoadConfig = func() (config.Config, string, error) { return cfg, "", nil }
	result, err := service.Status(context.Background(), ReferenceStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta["initialized"] != false || result.Meta["read_mode"] != "none" {
		t.Fatalf("meta = %#v", result.Meta)
	}
	if _, err := os.Stat(referenceStorePath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("status created index or returned unexpected stat error: %v", err)
	}
}

func TestReferenceStatusMigratesOutdatedIndexOnce(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir()}
	store, err := openReferenceStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", referenceStorePath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	service := NewReferenceService()
	service.LoadConfig = func() (config.Config, string, error) { return cfg, "", nil }
	result, err := service.Status(context.Background(), ReferenceStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta["read_mode"] != "migrated" {
		t.Fatalf("meta = %#v", result.Meta)
	}
	second, err := service.Status(context.Background(), ReferenceStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Meta["read_mode"] != "read_only" {
		t.Fatalf("second meta = %#v", second.Meta)
	}
}

func TestReferenceBuildRejectsAmbiguousScope(t *testing.T) {
	service := NewReferenceService()
	_, err := service.Build(context.Background(), ReferenceBuildRequest{Failed: true, Contexts: true})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v", err)
	}
}
