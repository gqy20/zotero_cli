package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"zotero_cli/internal/domain"
)

type PDFAnnotation struct {
	Page    int       `json:"page"`
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Comment string    `json:"comment,omitempty"`
	Color   string    `json:"color,omitempty"`
	Rect    [4]float64 `json:"rect,omitempty"`
	Author  string    `json:"author,omitempty"`
	Date    string    `json:"date,omitempty"`
}

type ReadAnnotationsResult struct {
	AttachmentKey string          `json:"attachment_key"`
	PDFPath       string          `json:"pdf_path"`
	Annotations   []PDFAnnotation `json:"annotations"`
	Total         int             `json:"total"`
}

func (r *LocalReader) ReadPDFAnnotations(ctx context.Context, attachment domain.Attachment) (ReadAnnotationsResult, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return ReadAnnotationsResult{}, fmt.Errorf("attachment %s has no resolved path", attachment.Key)
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return ReadAnnotationsResult{}, fmt.Errorf("python with pymupdf not found")
	}

	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
doc = fitz.open(pdf_path)
results = []
ANNO_TYPES = {0: "highlight", 1: "note", 2: "image", 3: "ink",
    4: "link", 5: "freetext", 6: "line", 7: "square",
    8: "circle", 9: "polygon", 10: "polyline", 11: "stamp",
    12: "caret", 13: "attachment", 14: "screen",
    15: "underline", 16: "strikeout", 17: "squiggly",
    18: "redact", 19: "trapezoid"}

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
        if atype in ("highlight", "underline"):
            text = annot.get_text("text").strip().replace("\n", " ")
            if text:
                entry["text"] = text[:500]
        elif atype in ("text", "freetext"):
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

type DeleteAnnotationsRequest struct {
	Page  int    `json:"page,omitempty"`
	Type  string `json:"type,omitempty"`
	Author string `json:"author,omitempty"`
}

type DeleteAnnotationsResult struct {
	AttachmentKey string `json:"attachment_key"`
	PDFPath       string `json:"pdf_path"`
	Deleted       int    `json:"deleted"`
}

func (r *LocalReader) DeletePDFAnnotations(ctx context.Context, attachment domain.Attachment, req DeleteAnnotationsRequest) (DeleteAnnotationsResult, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return DeleteAnnotationsResult{}, fmt.Errorf("attachment %s has no resolved path", attachment.Key)
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return DeleteAnnotationsResult{}, fmt.Errorf("python with pymupdf not found")
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

for pi in range(len(doc)):
    if req.get("page") and req["page"] != pi + 1:
        continue
    page = doc[pi]
    for annot in list(page.annots()):
        info = annot.info or {}
        # type filter
        if req.get("type"):
            t = annot.type
            if isinstance(t, (tuple, list)):
                t = t[0]
            ANNO_TYPES = {0: "highlight", 1: "note", 15: "underline", 16: "strikeout", 17: "squiggly"}
            atype = ANNO_TYPES.get(int(t), str(t)) if isinstance(t, int) else str(t)
            if atype.lower() != req["type"].lower():
                continue
        # author filter
        if req.get("author"):
            author = info.get("title", "")
            if author != req["author"]:
                continue
        page.delete_annot(annot)
        deleted += 1

doc.save(pdf_path, incremental=True, encryption=fitz.PDF_ENCRYPT_KEEP)

payload = json.dumps({"deleted": deleted})
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	cmd := exec.CommandContext(ctx, pythonCmd, "-", attachment.ResolvedPath)
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
	result.AttachmentKey = attachment.Key
	result.PDFPath = attachment.ResolvedPath
	return result, nil
}
