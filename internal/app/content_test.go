package app

import (
	"context"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

func TestAnnotationDeleteGateRunsBeforeReaderCreation(t *testing.T) {
	created := false
	service := AnnotationService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "local", AllowDelete: false}, "", nil
		},
		NewReader: func(config.Config) (backend.Reader, error) { created = true; return nil, nil },
	}
	_, err := service.Delete(context.Background(), "ITEM1", AnnotationFilter{}, SafetyOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "delete operations are disabled") {
		t.Fatalf("error = %v", err)
	}
	if created {
		t.Fatal("reader was created before delete gate")
	}
}

func TestAnnotationFilters(t *testing.T) {
	pdf := filterPDFAnnotations([]backend.PDFAnnotation{{Page: 1, Type: "highlight"}, {Page: 2, Type: "note"}}, AnnotationFilter{Page: 2})
	if len(pdf) != 1 || pdf[0].Page != 2 {
		t.Fatalf("pdf filter = %#v", pdf)
	}
	db := filterDBAnnotations([]domain.Annotation{{PageIndex: 0, Type: "highlight"}, {PageIndex: 1, Type: "note"}}, AnnotationFilter{Type: "note"})
	if len(db) != 1 || db[0].PageIndex != 1 {
		t.Fatalf("db filter = %#v", db)
	}
}

func TestWorkbookPathRecognition(t *testing.T) {
	for _, value := range []string{"table.xlsx", "TABLE.XLSM", "template.xltx"} {
		if !isWorkbookPath(value) {
			t.Fatalf("expected workbook path: %s", value)
		}
	}
	if isWorkbookPath("paper.pdf") {
		t.Fatal("PDF classified as workbook")
	}
}
