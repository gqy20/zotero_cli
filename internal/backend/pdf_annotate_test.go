package backend

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/domain"
)

func TestAnnotatePDFRequestExecutesInPython(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		python, err = exec.LookPath("python3")
	}
	if err != nil {
		t.Skip("Python is required to validate generated Python requests")
	}
	for _, dryRun := range []bool{true, false} {
		t.Run(map[bool]string{true: "dry-run", false: "write"}[dryRun], func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "paper.pdf")
			if err := os.WriteFile(source, []byte("original PDF"), 0o600); err != nil {
				t.Fatal(err)
			}
			req := AnnotateRequest{Type: "highlight", Color: "yellow", Text: "true false null", Comment: "引号\" ' \\路径\n换行\t😀\x00", DryRun: dryRun}
			withFakePDFAnnotator(t, nil)
			runPDFAnnotationScriptFunc = func(ctx context.Context, _, script, _ string) ([]byte, error) {
				// Execute the generated request with real Python, without requiring PyMuPDF.
				start := strings.Index(script, "req = ")
				end := strings.Index(script[start:], "\n") + start
				cmd := exec.CommandContext(ctx, python, "-c", "import json\n"+script[start:end]+"\nprint(json.dumps(req))")
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("generated request failed: %v: %s", err, output)
				}
				var got AnnotateRequest
				if err := json.Unmarshal(output, &got); err != nil {
					t.Fatal(err)
				}
				if got != req {
					t.Fatalf("request round trip: got %#v, want %#v", got, req)
				}
				return []byte(`{"matches":[{"page":1,"rect":[1,2,3,4]}],"dry_run":` + map[bool]string{true: "true", false: "false"}[dryRun] + `}`), nil
			}
			if _, err := (&LocalReader{}).AnnotatePDF(context.Background(), domainAttachment(source), req); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnnotatePDFHandlesStdoutWarnings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		output    string
		wantError bool
	}{
		{"warning-before", "DeprecationWarning: deprecated API\n" + `{"matches":[{"page":1,"rect":[1,2,3,4]}],"dry_run":false}`, false},
		{"warning-after", `{"matches":[{"page":1,"rect":[1,2,3,4]}],"dry_run":false}` + "\nDeprecationWarning: deprecated API\n", false},
		{"warning-only", "DeprecationWarning: deprecated API\n", true},
		{"malformed-json", "DeprecationWarning: deprecated API\n{broken}", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "paper.pdf")
			if err := os.WriteFile(source, []byte("original PDF"), 0o600); err != nil {
				t.Fatal(err)
			}
			withFakePDFAnnotator(t, func(workPath string) ([]byte, error) {
				if err := os.WriteFile(workPath, []byte("annotated PDF"), 0o600); err != nil {
					return nil, err
				}
				return []byte(tc.output), nil
			})
			_, err := (&LocalReader{}).AnnotatePDF(context.Background(), domainAttachment(source), AnnotateRequest{Type: "highlight", Color: "yellow", Text: "target"})
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, wantError=%v", err, tc.wantError)
			}
			got, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			want := "annotated PDF"
			if tc.wantError {
				want = "original PDF"
			}
			if string(got) != want {
				t.Fatalf("PDF content=%q, want %q", got, want)
			}
		})
	}
}

func withFakePDFAnnotator(t *testing.T, run func(string) ([]byte, error)) {
	t.Helper()
	previousFind := findPythonCommandFunc
	previousRun := runPDFAnnotationScriptFunc
	findPythonCommandFunc = func(string) (string, bool) { return "fake-python", true }
	runPDFAnnotationScriptFunc = func(_ context.Context, _, _, pdfPath string) ([]byte, error) {
		return run(pdfPath)
	}
	t.Cleanup(func() {
		findPythonCommandFunc = previousFind
		runPDFAnnotationScriptFunc = previousRun
	})
}

func TestAnnotatePDFCommitsValidatedTransaction(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("original PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFakePDFAnnotator(t, func(workPath string) ([]byte, error) {
		if workPath == source {
			t.Fatal("non-dry annotation ran against the original PDF")
		}
		if err := os.WriteFile(workPath, []byte("annotated PDF"), 0o600); err != nil {
			return nil, err
		}
		return []byte(`{"matches":[{"page":1,"rect":[1,2,3,4],"text":"target"}],"dry_run":false}`), nil
	})

	result, err := (&LocalReader{}).AnnotatePDF(context.Background(), domainAttachment(source), AnnotateRequest{Type: "highlight", Color: "yellow", Text: "target"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches=%#v", result.Matches)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "annotated PDF" {
		t.Fatalf("source content=%q", got)
	}
}

func TestAnnotatePDFZeroMatchesKeepsOriginal(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("original PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFakePDFAnnotator(t, func(workPath string) ([]byte, error) {
		if err := os.WriteFile(workPath, []byte("unexpected mutation"), 0o600); err != nil {
			return nil, err
		}
		return []byte(`{"matches":[],"dry_run":false}`), nil
	})

	_, err := (&LocalReader{}).AnnotatePDF(context.Background(), domainAttachment(source), AnnotateRequest{Type: "highlight", Color: "yellow", Text: "missing"})
	if err == nil || !strings.Contains(err.Error(), "did not match") {
		t.Fatalf("error=%v", err)
	}
	got, readErr := os.ReadFile(source)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "original PDF" {
		t.Fatalf("zero-match request changed source: %q", got)
	}
}

func TestAnnotatePDFDryRunAllowsZeroMatchesWithoutCopy(t *testing.T) {
	source := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(source, []byte("original PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFakePDFAnnotator(t, func(workPath string) ([]byte, error) {
		if workPath != source {
			t.Fatalf("dry-run path=%q, want original %q", workPath, source)
		}
		return []byte(`{"matches":[],"dry_run":true}`), nil
	})

	result, err := (&LocalReader{}).AnnotatePDF(context.Background(), domainAttachment(source), AnnotateRequest{Type: "highlight", Color: "yellow", Text: "missing", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || len(result.Matches) != 0 {
		t.Fatalf("result=%#v", result)
	}
}

func TestSelectPDFAttachmentUsesExplicitKey(t *testing.T) {
	attachments := []domain.Attachment{
		{Key: "PDF1", ContentType: "application/pdf"},
		{Key: "PDF2", ContentType: "application/pdf"},
		{Key: "DATA", ContentType: "text/csv"},
	}
	got, err := selectPDFAttachment(attachments, "pdf2")
	if err != nil || got.Key != "PDF2" {
		t.Fatalf("attachment=%#v error=%v", got, err)
	}
	if _, err := selectPDFAttachment(attachments, "DATA"); err == nil || !strings.Contains(err.Error(), "not a PDF") {
		t.Fatalf("non-PDF error=%v", err)
	}
	if _, err := selectPDFAttachment(attachments, "MISSING"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing error=%v", err)
	}
}
