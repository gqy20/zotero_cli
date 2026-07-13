package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"zotero_cli/internal/domain"
)

type PDFAnnotation struct {
	XRef    int        `json:"xref"`
	Page    int        `json:"page"`
	Type    string     `json:"type"`
	Text    string     `json:"text,omitempty"`
	Comment string     `json:"comment,omitempty"`
	Color   string     `json:"color,omitempty"`
	Rect    [4]float64 `json:"rect,omitempty"`
	Author  string     `json:"author,omitempty"`
	Date    string     `json:"date,omitempty"`
}

var pdfAnnotationTypeNames = map[int]string{
	0: "note", 1: "link", 2: "freetext", 3: "line", 4: "square", 5: "circle",
	6: "polygon", 7: "polyline", 8: "highlight", 9: "underline", 10: "squiggly",
	11: "strikeout", 12: "redact", 13: "stamp", 14: "caret", 15: "ink", 16: "popup",
	17: "attachment", 18: "sound", 19: "movie", 20: "richmedia", 21: "widget",
	22: "screen", 23: "printermark", 24: "trapnet", 25: "watermark", 26: "3d", 27: "projection",
}

func pdfAnnotationTypesJSON() string {
	encoded, _ := json.Marshal(pdfAnnotationTypeNames)
	return string(encoded)
}

type ReadAnnotationsResult struct {
	AttachmentKey string          `json:"attachment_key"`
	PDFPath       string          `json:"pdf_path"`
	Annotations   []PDFAnnotation `json:"annotations"`
	Total         int             `json:"total"`
}

type ItemAnnotationsResult struct {
	ItemKey        string              `json:"item_key"`
	AttachmentKey  string              `json:"attachment_key"`
	PDFPath        string              `json:"pdf_path,omitempty"`
	PDFAnnotations []PDFAnnotation     `json:"pdf_annotations"`
	DBAnnotations  []domain.Annotation `json:"db_annotations"`
	TotalPDF       int                 `json:"total_pdf"`
	TotalDB        int                 `json:"total_db"`
	PDFError       string              `json:"pdf_error,omitempty"`
}

type ItemAnnotationClearResult struct {
	ItemKey       string `json:"item_key"`
	AttachmentKey string `json:"attachment_key"`
	PDFPath       string `json:"pdf_path,omitempty"`
	PDFDeleted    int    `json:"pdf_deleted"`
	DBDeleted     int    `json:"db_deleted"`
	Deleted       int    `json:"deleted"`
	DBError       string `json:"db_error,omitempty"`
}

func (r *LocalReader) ReadPDFAnnotations(ctx context.Context, attachment domain.Attachment) (ReadAnnotationsResult, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return ReadAnnotationsResult{}, fmt.Errorf("attachment %s has no resolved path", attachment.Key)
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return ReadAnnotationsResult{}, pyMuPDFUnavailableError()
	}

	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
doc = fitz.open(pdf_path)
results = []
ANNO_TYPES = {int(k): v for k, v in ` + pdfAnnotationTypesJSON() + `.items()}

def anno_type_name(annot):
    t = annot.type
    if isinstance(t, (tuple, list)) and len(t) > 0:
        return ANNO_TYPES.get(t[0], f"unknown({t[0]})")
    try:
        return ANNO_TYPES.get(int(t), str(t))
    except (ValueError, TypeError):
        return str(t)

for pi in range(len(doc)):
    page = doc[pi]
    for annot in page.annots():
        info = annot.info
        atype = anno_type_name(annot)
        entry = {
            "xref": annot.xref,
            "page": pi + 1,
            "type": atype,
            "rect": [round(annot.rect.x0, 1), round(annot.rect.y0, 1),
                    round(annot.rect.x1, 1), round(annot.rect.y1, 1)],
        }
        colors = annot.colors or {}
        stroke = colors.get("stroke")
        if stroke and len(stroke) >= 3:
            r, g, b = stroke[0], stroke[1], stroke[2]
            entry["color"] = "#%02x%02x%02x" % (int(r*255), int(g*255), int(b*255))
        if atype in ("highlight", "underline", "squiggly", "strikeout"):
            text = annot.get_text("text").strip().replace("\n", " ")
            if text:
                entry["text"] = text[:500]
        elif atype in ("note", "freetext"):
            content = info.get("content", "")
            if content:
                entry["comment"] = content[:500]
        author = info.get("title", "")
        if author:
            entry["author"] = author[:200]
        mod_date = info.get("modDate", "")
        if mod_date:
            entry["date"] = mod_date
        results.append(entry)

payload = json.dumps({"annotations": results}, ensure_ascii=False)
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	cmd := exec.CommandContext(ctx, pythonCmd, "-", attachment.ResolvedPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ReadAnnotationsResult{}, fmt.Errorf("read annotations failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var rawResult struct {
		Annotations []PDFAnnotation `json:"annotations"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rawResult); err != nil {
		return ReadAnnotationsResult{}, err
	}
	return ReadAnnotationsResult{
		AttachmentKey: attachment.Key,
		PDFPath:       attachment.ResolvedPath,
		Annotations:   rawResult.Annotations,
		Total:         len(rawResult.Annotations),
	}, nil
}

func (r *LocalReader) ReadItemAnnotations(ctx context.Context, item domain.Item, attachmentKey string) (ItemAnnotationsResult, error) {
	att, err := selectPDFAttachment(item.Attachments, attachmentKey)
	if err != nil {
		return ItemAnnotationsResult{}, fmt.Errorf("item %s: %w", item.Key, err)
	}

	var pdfAnns []PDFAnnotation
	pdfResult, err := r.ReadPDFAnnotations(ctx, att)
	pdfError := ""
	if err == nil {
		pdfAnns = pdfResult.Annotations
	} else {
		pdfError = err.Error()
	}

	dbAnns := make([]domain.Annotation, 0, len(item.Annotations))
	for _, annotation := range item.Annotations {
		if annotation.AttachmentKey == "" || strings.EqualFold(annotation.AttachmentKey, att.Key) {
			dbAnns = append(dbAnns, annotation)
		}
	}
	return ItemAnnotationsResult{
		ItemKey:        item.Key,
		AttachmentKey:  att.Key,
		PDFPath:        att.ResolvedPath,
		PDFAnnotations: pdfAnns,
		DBAnnotations:  dbAnns,
		TotalPDF:       len(pdfAnns),
		TotalDB:        len(dbAnns),
		PDFError:       pdfError,
	}, nil
}

func (r *LocalReader) ClearItemAnnotations(ctx context.Context, item domain.Item, req DeleteAnnotationsRequest) (ItemAnnotationClearResult, error) {
	att, err := selectPDFAttachment(item.Attachments, req.AttachmentKey)
	if err != nil {
		return ItemAnnotationClearResult{}, fmt.Errorf("item %s: %w", item.Key, err)
	}

	pdfResult, err := r.DeletePDFAnnotations(ctx, att, req)
	if err != nil {
		return ItemAnnotationClearResult{}, err
	}

	return ItemAnnotationClearResult{
		ItemKey:       item.Key,
		AttachmentKey: att.Key,
		PDFPath:       att.ResolvedPath,
		PDFDeleted:    pdfResult.Deleted,
		Deleted:       pdfResult.Deleted,
	}, nil
}

func (r *HybridReader) ReadItemAnnotations(ctx context.Context, item domain.Item, attachmentKey string) (ItemAnnotationsResult, error) {
	reader, ok := r.local.(interface {
		ReadItemAnnotations(context.Context, domain.Item, string) (ItemAnnotationsResult, error)
	})
	if !ok {
		return ItemAnnotationsResult{}, fmt.Errorf("annotations require local or hybrid mode with local data")
	}
	result, err := reader.ReadItemAnnotations(ctx, item, attachmentKey)
	if err != nil {
		return ItemAnnotationsResult{}, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return result, nil
}

func (r *HybridReader) AnnotateItem(ctx context.Context, item domain.Item, req AnnotateRequest) (AnnotateResult, error) {
	reader, ok := r.local.(interface {
		AnnotateItem(context.Context, domain.Item, AnnotateRequest) (AnnotateResult, error)
	})
	if !ok {
		return AnnotateResult{}, fmt.Errorf("annotation writing requires local or hybrid mode with local data")
	}
	result, err := reader.AnnotateItem(ctx, item, req)
	if err != nil {
		return AnnotateResult{}, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return result, nil
}

func (r *HybridReader) ClearItemAnnotations(ctx context.Context, item domain.Item, req DeleteAnnotationsRequest) (ItemAnnotationClearResult, error) {
	reader, ok := r.local.(interface {
		ClearItemAnnotations(context.Context, domain.Item, DeleteAnnotationsRequest) (ItemAnnotationClearResult, error)
	})
	if !ok {
		return ItemAnnotationClearResult{}, fmt.Errorf("annotation deletion requires local or hybrid mode with local data")
	}
	result, err := reader.ClearItemAnnotations(ctx, item, req)
	if err != nil {
		return ItemAnnotationClearResult{}, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return result, nil
}

type DeleteAnnotationsRequest struct {
	AttachmentKey string `json:"attachment_key,omitempty"`
	Page          int    `json:"page,omitempty"`
	Type          string `json:"type,omitempty"`
	Author        string `json:"author,omitempty"`
	PDFXRefs      []int  `json:"pdf_xrefs,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
}

type DeleteAnnotationsResult struct {
	AttachmentKey string `json:"attachment_key"`
	PDFPath       string `json:"pdf_path"`
	Deleted       int    `json:"deleted"`
}

func selectPDFAttachment(attachments []domain.Attachment, attachmentKey string) (domain.Attachment, error) {
	for _, attachment := range attachments {
		if attachmentKey != "" && !strings.EqualFold(attachment.Key, attachmentKey) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			if attachmentKey != "" {
				return domain.Attachment{}, fmt.Errorf("attachment %s is not a PDF", attachmentKey)
			}
			continue
		}
		return attachment, nil
	}
	if attachmentKey != "" {
		return domain.Attachment{}, fmt.Errorf("PDF attachment %s not found", attachmentKey)
	}
	return domain.Attachment{}, fmt.Errorf("has no PDF attachment")
}

func (r *LocalReader) DeletePDFAnnotations(ctx context.Context, attachment domain.Attachment, req DeleteAnnotationsRequest) (DeleteAnnotationsResult, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return DeleteAnnotationsResult{}, fmt.Errorf("attachment %s has no resolved path", attachment.Key)
	}
	if len(req.PDFXRefs) == 0 {
		return DeleteAnnotationsResult{AttachmentKey: attachment.Key, PDFPath: attachment.ResolvedPath}, nil
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return DeleteAnnotationsResult{}, pyMuPDFUnavailableError()
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return DeleteAnnotationsResult{}, err
	}

	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
req = ` + string(reqJSON) + `

doc = fitz.open(pdf_path)
deleted = 0
target_xrefs = set(req.get("pdf_xrefs") or [])

for pi in range(len(doc)):
    page = doc[pi]
    for annot in list(page.annots()):
        if annot.xref not in target_xrefs:
            continue
        page.delete_annot(annot)
        deleted += 1

doc.save(pdf_path, incremental=True, encryption=fitz.PDF_ENCRYPT_KEEP)
doc.close()
check = fitz.open(pdf_path)
len(check)
check.close()

payload = json.dumps({"deleted": deleted})
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	workPath, commit, cleanup, err := preparePDFAnnotationMutation(attachment.ResolvedPath)
	if err != nil {
		return DeleteAnnotationsResult{}, err
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, pythonCmd, "-", workPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return DeleteAnnotationsResult{}, fmt.Errorf("delete annotations failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result DeleteAnnotationsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return DeleteAnnotationsResult{}, err
	}
	if result.Deleted != len(req.PDFXRefs) {
		return DeleteAnnotationsResult{}, fmt.Errorf("delete annotations matched %d of %d selected PDF annotations", result.Deleted, len(req.PDFXRefs))
	}
	if err := commit(); err != nil {
		return DeleteAnnotationsResult{}, err
	}
	result.AttachmentKey = attachment.Key
	result.PDFPath = attachment.ResolvedPath
	return result, nil
}

func preparePDFAnnotationMutation(source string) (string, func() error, func(), error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", nil, nil, fmt.Errorf("stat PDF %q: %w", source, err)
	}
	dir := filepath.Dir(source)
	temp, err := os.CreateTemp(dir, ".zot-ann-*.pdf")
	if err != nil {
		return "", nil, nil, fmt.Errorf("create PDF transaction file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	input, err := os.Open(source)
	if err != nil {
		_ = temp.Close()
		cleanup()
		return "", nil, nil, fmt.Errorf("open PDF %q: %w", source, err)
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(temp, hasher), input)
	originalHash := hasher.Sum(nil)
	closeInputErr := input.Close()
	chmodErr := temp.Chmod(info.Mode())
	syncErr := temp.Sync()
	closeTempErr := temp.Close()
	for _, candidate := range []error{copyErr, closeInputErr, chmodErr, syncErr, closeTempErr} {
		if candidate != nil {
			cleanup()
			return "", nil, nil, fmt.Errorf("prepare PDF transaction: %w", candidate)
		}
	}
	committed := false
	commit := func() error {
		current, err := os.Open(source)
		if err != nil {
			return fmt.Errorf("reopen original PDF before replacement: %w", err)
		}
		currentHasher := sha256.New()
		_, hashErr := io.Copy(currentHasher, current)
		closeErr := current.Close()
		if hashErr != nil {
			return fmt.Errorf("verify original PDF before replacement: %w", hashErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close original PDF after verification: %w", closeErr)
		}
		if !bytes.Equal(originalHash, currentHasher.Sum(nil)) {
			return fmt.Errorf("original PDF changed while annotations were being prepared; refusing replacement")
		}
		backupFile, err := os.CreateTemp(dir, ".zot-ann-backup-*.pdf")
		if err != nil {
			return fmt.Errorf("create PDF rollback path: %w", err)
		}
		backupPath := backupFile.Name()
		if err := backupFile.Close(); err != nil {
			_ = os.Remove(backupPath)
			return err
		}
		if err := os.Remove(backupPath); err != nil {
			return err
		}
		if err := os.Rename(source, backupPath); err != nil {
			return fmt.Errorf("prepare original PDF replacement: %w", err)
		}
		if err := os.Rename(tempPath, source); err != nil {
			rollbackErr := os.Rename(backupPath, source)
			if rollbackErr != nil {
				return fmt.Errorf("replace PDF: %v; rollback also failed: %w", err, rollbackErr)
			}
			return fmt.Errorf("replace PDF: %w", err)
		}
		committed = true
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("remove PDF transaction backup: %w", err)
		}
		return nil
	}
	return tempPath, commit, func() {
		if !committed {
			cleanup()
		}
	}, nil
}
