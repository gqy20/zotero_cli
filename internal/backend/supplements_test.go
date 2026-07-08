package backend

import (
	"testing"

	"zotero_cli/internal/domain"
)

func TestClassifyLocalSupplementDetectsPublisherDataFile(t *testing.T) {
	item := domain.Item{Key: "ITEM1234", Title: "A paper"}
	attachment := domain.Attachment{
		Key:          "ATT12345",
		Title:        "41588_2024_1715_MOESM4_ESM",
		Filename:     "41588_2024_1715_MOESM4_ESM.xlsx",
		ContentType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		ZoteroPath:   "storage:41588_2024_1715_MOESM4_ESM.xlsx",
		ResolvedPath: "C:/tmp/41588_2024_1715_MOESM4_ESM.xlsx",
		Resolved:     true,
	}

	got, ok := ClassifyLocalSupplement(item, attachment)
	if !ok {
		t.Fatalf("expected supplement candidate")
	}
	if got.Kind != "supplementary_dataset" {
		t.Fatalf("Kind = %q, want supplementary_dataset", got.Kind)
	}
	if got.ResolutionStatus != "stored_file_found" {
		t.Fatalf("ResolutionStatus = %q, want stored_file_found", got.ResolutionStatus)
	}
	if got.Confidence < 0.9 {
		t.Fatalf("Confidence = %v, want >= 0.9", got.Confidence)
	}
}

func TestClassifyLocalSupplementIgnoresParentTitleForOrdinaryPDF(t *testing.T) {
	item := domain.Item{Key: "ITEM1234", Title: "The database and its supplement"}
	attachment := domain.Attachment{
		Key:         "PDF12345",
		Title:       "The database and its supplement",
		Filename:    "paper.pdf",
		ContentType: "application/pdf",
		ZoteroPath:  "storage:paper.pdf",
	}

	if got, ok := ClassifyLocalSupplement(item, attachment); ok {
		t.Fatalf("unexpected supplement candidate: %#v", got)
	}
}

func TestClassifyLocalSupplementReportsLinkedUnresolved(t *testing.T) {
	item := domain.Item{Key: "ITEM1234", Title: "A paper"}
	attachment := domain.Attachment{
		Key:         "ATT12345",
		Title:       "Supplementary Tables",
		Filename:    "Supplementary Tables.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		ZoteroPath:  "attachments:supplement/Supplementary Tables.xlsx",
	}

	got, ok := ClassifyLocalSupplement(item, attachment)
	if !ok {
		t.Fatalf("expected supplement candidate")
	}
	if got.ResolutionStatus != "linked_file_unresolved" {
		t.Fatalf("ResolutionStatus = %q, want linked_file_unresolved", got.ResolutionStatus)
	}
}

func TestClassifyLocalSupplementHandlesUnderscoreSupplementaryTables(t *testing.T) {
	item := domain.Item{Key: "ITEM1234", Title: "A paper"}
	attachment := domain.Attachment{
		Key:         "ATT12345",
		Title:       "Supplementary_tables_v9",
		Filename:    "Supplementary_tables_v9.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		ZoteroPath:  "storage:Supplementary_tables_v9.xlsx",
	}

	got, ok := ClassifyLocalSupplement(item, attachment)
	if !ok {
		t.Fatalf("expected supplement candidate")
	}
	if got.Kind != "supplementary_dataset" {
		t.Fatalf("Kind = %q, want supplementary_dataset", got.Kind)
	}
}
