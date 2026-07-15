package app

import (
	"context"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type annotationTestReader struct {
	backend.Reader
	item      domain.Item
	result    backend.ItemAnnotationsResult
	clearReq  backend.DeleteAnnotationsRequest
	clearCall bool
	readKey   string
}

func (r *annotationTestReader) GetItem(context.Context, string) (domain.Item, error) {
	return r.item, nil
}

func (r *annotationTestReader) ReadItemAnnotations(_ context.Context, _ domain.Item, attachmentKey string) (backend.ItemAnnotationsResult, error) {
	r.readKey = attachmentKey
	return r.result, nil
}

func (r *annotationTestReader) ClearItemAnnotations(_ context.Context, _ domain.Item, req backend.DeleteAnnotationsRequest) (backend.ItemAnnotationClearResult, error) {
	r.clearReq = req
	r.clearCall = true
	return backend.ItemAnnotationClearResult{AttachmentKey: "ATT1", PDFDeleted: len(req.PDFXRefs), Deleted: len(req.PDFXRefs)}, nil
}

type annotationTestDeleteClient struct {
	keys    []string
	version int
}

func TestFilePathRejectsRemoteModeBeforeDownloading(t *testing.T) {
	created := false
	service := ReadService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "remote"}, "", nil
		},
		NewReader: func(config.Config) (backend.Reader, error) {
			created = true
			return nil, nil
		},
	}
	_, err := service.Files(context.Background(), FileRequest{AttachmentKey: "ATT1", PathOnly: true})
	if err == nil || !strings.Contains(err.Error(), "run `zot sync`") {
		t.Fatalf("error = %v", err)
	}
	if created {
		t.Fatal("remote reader was created before file path mode gate")
	}
}

func (c *annotationTestDeleteClient) GetLibraryVersion(context.Context) (int, error) {
	return 17, nil
}

func (c *annotationTestDeleteClient) DeleteItems(_ context.Context, keys []string, version int) (zoteroapi.BatchWriteResult, error) {
	c.keys = append([]string(nil), keys...)
	c.version = version
	return zoteroapi.BatchWriteResult{LastModifiedVersion: version + 1}, nil
}

func TestAnnotationDeleteGateRunsBeforeReaderCreation(t *testing.T) {
	created := false
	service := AnnotationService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "local", AllowDelete: false}, "", nil
		},
		NewReader: func(config.Config) (backend.Reader, error) { created = true; return nil, nil },
	}
	_, err := service.Delete(context.Background(), "ITEM1", AnnotationFilter{}, "zotero", SafetyOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "delete operations are disabled") {
		t.Fatalf("error = %v", err)
	}
	if created {
		t.Fatal("reader was created before delete gate")
	}
}

func TestAnnotationFilters(t *testing.T) {
	pdf := filterPDFAnnotations([]backend.PDFAnnotation{{Page: 1, Type: "highlight", Author: "Alice"}, {Page: 2, Type: "note", Author: "Bob"}}, AnnotationFilter{Page: 2, Author: "bob"})
	if len(pdf) != 1 || pdf[0].Page != 2 {
		t.Fatalf("pdf filter = %#v", pdf)
	}
	db := filterDBAnnotations([]domain.Annotation{{PageIndex: 0, Type: "highlight", Author: "Alice"}, {PageIndex: 1, Type: "note", Author: "Bob"}}, AnnotationFilter{Type: "note", Author: "BOB"})
	if len(db) != 1 || db[0].PageIndex != 1 {
		t.Fatalf("db filter = %#v", db)
	}
}

func TestAnnotationDeleteSourceAndTypeValidation(t *testing.T) {
	if _, err := normalizeAnnotationDeleteSource(""); err == nil {
		t.Fatal("missing source was accepted")
	}
	if got, err := normalizeAnnotationDeleteSource(" PDF "); err != nil || got != "pdf" {
		t.Fatalf("source = %q, error = %v", got, err)
	}
	if err := validateAnnotationFilter(AnnotationFilter{Type: "unknown"}); err == nil {
		t.Fatal("unknown annotation type was accepted")
	}
	if err := validateAnnotationFilter(AnnotationFilter{Type: "underline"}); err != nil {
		t.Fatalf("underline rejected: %v", err)
	}
}

func TestAnnotationDeleteZoteroUsesExactItemKeysAndVersion(t *testing.T) {
	reader := &annotationTestReader{
		item: domain.Item{Key: "ITEM1"},
		result: backend.ItemAnnotationsResult{DBAnnotations: []domain.Annotation{
			{Key: "ANN1", Type: "highlight", PageIndex: 0},
			{Key: "ANN2", Type: "underline", PageIndex: 1},
		}},
	}
	client := &annotationTestDeleteClient{}
	service := AnnotationService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "hybrid", AllowDelete: true}, "", nil
		},
		NewReader:       func(config.Config) (backend.Reader, error) { return reader, nil },
		NewDeleteClient: func(config.Config) (annotationDeleteClient, error) { return client, nil },
	}
	result, err := service.Delete(context.Background(), "ITEM1", AnnotationFilter{Page: 2}, "zotero", SafetyOptions{Yes: true, IfVersion: 23})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.keys) != 1 || client.keys[0] != "ANN2" || client.version != 23 {
		t.Fatalf("keys=%#v version=%d", client.keys, client.version)
	}
	if reader.clearCall {
		t.Fatal("Zotero deletion unexpectedly modified the PDF")
	}
	if result.Meta["delete_source"] != "zotero_web_api" {
		t.Fatalf("meta=%#v", result.Meta)
	}
}

func TestAnnotationDeletePDFUsesExactXRefs(t *testing.T) {
	reader := &annotationTestReader{
		item: domain.Item{Key: "ITEM1"},
		result: backend.ItemAnnotationsResult{PDFAnnotations: []backend.PDFAnnotation{
			{XRef: 11, Page: 1, Type: "highlight"},
			{XRef: 22, Page: 2, Type: "underline"},
		}},
	}
	service := AnnotationService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "local", AllowDelete: true}, "", nil
		},
		NewReader: func(config.Config) (backend.Reader, error) { return reader, nil },
	}
	_, err := service.Delete(context.Background(), "ITEM1", AnnotationFilter{AttachmentKey: "ATT2", Type: "underline"}, "pdf", SafetyOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if reader.readKey != "ATT2" || !reader.clearCall || reader.clearReq.AttachmentKey != "ATT2" || len(reader.clearReq.PDFXRefs) != 1 || reader.clearReq.PDFXRefs[0] != 22 {
		t.Fatalf("clear request=%#v", reader.clearReq)
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
