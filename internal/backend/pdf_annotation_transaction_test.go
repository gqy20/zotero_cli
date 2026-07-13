package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/domain"
)

func TestPreparePDFAnnotationMutationCommitsReplacement(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	workPath, commit, cleanup, err := preparePDFAnnotationMutation(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if filepath.Dir(workPath) != filepath.Dir(source) {
		t.Fatalf("transaction file must share source directory: %q", workPath)
	}
	if err := os.WriteFile(workPath, []byte("updated"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := commit(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "updated" {
		t.Fatalf("source content = %q", got)
	}
	entries, err := os.ReadDir(filepath.Dir(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".zot-ann-") {
			t.Fatalf("transaction artifact remained: %s", entry.Name())
		}
	}
}

func TestPDFAnnotationTypesMatchPyMuPDFConstants(t *testing.T) {
	want := map[int]string{0: "note", 8: "highlight", 9: "underline", 11: "strikeout", 15: "ink", 17: "attachment"}
	for code, name := range want {
		if got := pdfAnnotationTypeNames[code]; got != name {
			t.Fatalf("annotation type %d = %q, want %q", code, got, name)
		}
	}
}

func TestDeletePDFAnnotationsWithoutExactXRefsDoesNothing(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("not touched"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{}
	result, err := reader.DeletePDFAnnotations(context.Background(), domainAttachment(source), DeleteAnnotationsRequest{Type: "highlight"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 0 {
		t.Fatalf("deleted = %d", result.Deleted)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "not touched" {
		t.Fatalf("source was changed: %q", got)
	}
}

func TestPreparePDFAnnotationMutationRefusesConcurrentSourceChange(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	workPath, commit, cleanup, err := preparePDFAnnotationMutation(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(workPath, []byte("annotation edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("concurrent edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := commit(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("commit error = %v", err)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "concurrent edit" {
		t.Fatalf("concurrent source was overwritten: %q", got)
	}
}

func domainAttachment(path string) domain.Attachment {
	return domain.Attachment{Key: "ATT1", Resolved: true, ResolvedPath: path}
}
