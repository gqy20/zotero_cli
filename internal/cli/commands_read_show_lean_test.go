package cli

import (
	"encoding/json"
	"net/http"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

func TestShowJSONDefaultIsLean(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item := verboseTestItem()

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: item}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"show", "VERBOSE1", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got %T", got["data"])
	}

	// Lean fields present
	if data["key"] != "VERBOSE1" {
		t.Errorf("key = %q, want VERBOSE1", data["key"])
	}
	if data["title"] != "CRISPR-Cas9 Gene Editing System" {
		t.Errorf("title = %q, want CRISPR-Cas9 Gene Editing System", data["title"])
	}
	if data["container"] != "Nature Reviews Genetics" {
		t.Errorf("container = %q, want Nature Reviews Genetics", data["container"])
	}
	if data["doi"] != "10.1038/nrg.2024.001" {
		t.Errorf("doi = %q, want 10.1038/nrg.2024.001", data["doi"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta to be a map, got %T", got["meta"])
	}
	if meta["lean"] != true || meta["full_hint"] == "" {
		t.Fatalf("expected lean meta hint, got %#v", meta)
	}
	omitted, ok := meta["omitted_fields"].([]any)
	if !ok || len(omitted) == 0 {
		t.Fatalf("expected omitted_fields in meta, got %#v", meta["omitted_fields"])
	}

	// Creators must be folded string
	creators, ok := data["creator_summary"].(string)
	if !ok {
		t.Fatalf("creator_summary should be string in lean mode, got %T: %#v", data["creator_summary"], data["creator_summary"])
	}
	if creators != "Zhang, Feng et al." {
		t.Errorf("creators = %q, want 'Zhang, Feng et al.'", creators)
	}

	// Verbose fields must NOT exist
	for _, forbidden := range []string{"abstract", "attachments", "notes", "annotations", "journal_rank"} {
		if _, exists := data[forbidden]; exists {
			t.Errorf("show --json lean must not contain %q", forbidden)
		}
	}
}

func TestShowJSONWithFullFlagOutputsCompleteItem(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item := verboseTestItem()

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: item}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"show", "VERBOSE1", "--json", "--full"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr may have info", exitCode)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data := got["data"].(map[string]any)

	// --full must include verbose fields
	if data["abstract"] == nil || data["abstract"] == "" {
		t.Errorf("--full must include abstract")
	}
	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) < 2 {
		t.Errorf("--full must include attachments with >=2 items, got: %#v", data["attachments"])
	}
	// Creators should be array (original format)
	creatorsArr, ok := data["creators"].([]any)
	if !ok || len(creatorsArr) != 5 {
		t.Errorf("--full creators should be array of 5, got: %#v (%T)", data["creators"], data["creators"])
	}
}

func TestShowFullFlagAcceptedNoError(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: domain.Item{Key: "S1", Title: "Test"}}, nil
	}

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "S1", "--json", "--full"})
	// --full should not cause usage error or parse error
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 with --full flag, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v", err)
	}
	if got["ok"] != true {
		t.Errorf("ok should be true, got %#v", got["ok"])
	}
}

func TestShowJSONLeanCollectionsNamesOnly(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	item := domain.Item{
		Key:      "SHOWCOLL",
		Title:    "Collection Test",
		ItemType: "journalArticle",
		Creators: []domain.Creator{{Name: "Author"}},
		Collections: []domain.Collection{
			{Key: "C111", Name: "Bioinformatics"},
			{Key: "C222", Name: "Systems Biology"},
			{Key: "C333", Name: "Genomics"},
		},
	}

	previousNewReader := testCLI.backendNewReader
	t.Cleanup(func() { testCLI.backendNewReader = previousNewReader })
	testCLI.backendNewReader = func(config.Config, *http.Client) (backend.Reader, error) {
		return leanStubReader{item: item}, nil
	}

	stdout, _ := captureOutput(t)
	exitCode := Run([]string{"show", "SHOWCOLL", "--json"})
	if exitCode != 0 {
		t.Fatalf("exit code %d", exitCode)
	}

	var got map[string]any
	json.Unmarshal(stdout.Bytes(), &got)
	data := got["data"].(map[string]any)

	colls, ok := data["collection_names"].([]any)
	if !ok || len(colls) != 3 {
		t.Fatalf("collection_names should be 3 strings, got: %#v", data["collection_names"])
	}
	wantNames := []string{"Bioinformatics", "Systems Biology", "Genomics"}
	for i, name := range wantNames {
		if colls[i] != name {
			t.Errorf("collections[%d] = %q, want %q", i, colls[i], name)
		}
	}
}
