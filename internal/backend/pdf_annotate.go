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

type AnnotateRequest struct {
	Text    string      `json:"text,omitempty"`
	Color   string      `json:"color"`
	Comment string      `json:"comment,omitempty"`
	Type    string      `json:"type"` // "highlight" | "underline" | "note"
	Page    int         `json:"page,omitempty"`
	Rect    *[4]float64 `json:"rect,omitempty"`
	Point   *[2]float64 `json:"point,omitempty"`
	DryRun  bool        `json:"dry_run,omitempty"`
}

type AnnotateMatch struct {
	Page  int        `json:"page"`
	Text  string     `json:"text,omitempty"`
	Rect  [4]float64 `json:"rect"`
	Type  string     `json:"type"`
	Color string     `json:"color"`
}

type AnnotateResult struct {
	AttachmentKey string          `json:"attachment_key"`
	PDFPath       string          `json:"pdf_path"`
	Matches       []AnnotateMatch `json:"matches"`
	DryRun        bool            `json:"dry_run,omitempty"`
}

type annotateMatchRaw struct {
	Page int       `json:"page"`
	Rect []float64 `json:"rect"`
	Text string    `json:"text"`
}

func (r *LocalReader) AnnotatePDF(ctx context.Context, attachment domain.Attachment, req AnnotateRequest) (AnnotateResult, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return AnnotateResult{}, fmt.Errorf("attachment %s has no resolved path", attachment.Key)
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return AnnotateResult{}, fmt.Errorf("python with pymupdf not found")
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return AnnotateResult{}, err
	}

	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
req = ` + string(reqJSON) + `

doc = fitz.open(pdf_path)
results = []
dry_run = bool(req.get("dry_run", False))
MATCH_TEXT_LIMIT = 500

COLOR_MAP = {
    "yellow": (1.0, 0.83, 0.0),
    "red": (1.0, 0.4, 0.4),
    "green": (0.37, 0.7, 0.12),
    "blue": (0.18, 0.66, 0.9),
}

def parse_color(c):
    c = c.strip()
    if c in COLOR_MAP:
        return COLOR_MAP[c]
    if c.startswith("#") and len(c) == 7:
        r = int(c[1:3], 16) / 255.0
        g = int(c[3:5], 16) / 255.0
        b = int(c[5:7], 16) / 255.0
        return (r, g, b)
    return COLOR_MAP["yellow"]

atype = req.get("type", "highlight")
color = parse_color(req.get("color", "yellow"))
comment = req.get("comment", "")
info = {"title": "zotero_cli"}
if comment:
    info["content"] = comment

def preview_text(value):
    value = value.strip().replace("\n", " ")
    if len(value) > MATCH_TEXT_LIMIT:
        return value[:MATCH_TEXT_LIMIT]
    return value

def result_rect(rect):
    return [round(rect.x0, 1), round(rect.y0, 1),
            round(rect.x1, 1), round(rect.y1, 1)]

def add_result(page_number, rect, text):
    results.append({
        "page": page_number,
        "rect": result_rect(rect),
        "text": preview_text(text),
    })

def add_text_annotation(page, q):
    if dry_run:
        return
    if atype == "highlight":
        annot = page.add_highlight_annot(q)
    elif atype == "underline":
        annot = page.add_underline_annot(q)
    else:
        annot = page.add_highlight_annot(q)
    annot.set_colors(stroke=color)
    annot.set_info(**info)
    annot.update()

def add_rect_annotation(page, rect):
    if dry_run:
        return
    if atype == "highlight":
        annot = page.add_highlight_annot(rect)
    elif atype == "underline":
        annot = page.add_underline_annot(rect)
    else:
        annot = page.add_highlight_annot(rect)
    annot.set_colors(stroke=color)
    annot.set_info(**info)
    annot.update()

# Mode 1: text search across all pages
search_text = req.get("text", "")
if search_text and not req.get("page"):
    for pi in range(len(doc)):
        page = doc[pi]
        quads = page.search_for(search_text, quads=True)
        for q in quads:
            add_text_annotation(page, q)
            text = page.get_text("text", clip=q.rect).strip().replace("\n", " ")
            add_result(pi + 1, q.rect, text)

# Mode 1.5: text search on a specific page only
elif search_text and req.get("page"):
    pi = req["page"] - 1
    if 0 <= pi < len(doc):
        page = doc[pi]
        quads = page.search_for(search_text, quads=True)
        for q in quads:
            add_text_annotation(page, q)
            text = page.get_text("text", clip=q.rect).strip().replace("\n", " ")
            add_result(pi + 1, q.rect, text)

# Mode 2: specific page + rect (direct position annotation)
elif req.get("page") and req.get("rect") is not None:
    pi = req["page"] - 1
    if 0 <= pi < len(doc):
        page = doc[pi]
        rc = req["rect"]
        rect = fitz.Rect(rc[0], rc[1], rc[2], rc[3])
        add_rect_annotation(page, rect)
        text = page.get_text("text", clip=rect).strip().replace("\n", " ")
        add_result(req["page"], rect, text)
    # Also search text on this page if provided
    if search_text:
        page = doc[pi]
        quads = page.search_for(search_text, quads=True)
        for q in quads:
            add_text_annotation(page, q)
            t = page.get_text("text", clip=q.rect).strip().replace("\n", " ")
            add_result(req["page"], q.rect, t)

# Mode 3: specific page + point (note/sticky note)
elif req.get("page") and req.get("point") is not None:
    pi = req["page"] - 1
    if 0 <= pi < len(doc):
        page = doc[pi]
        pt = req["point"]
        point = fitz.Point(pt[0], pt[1])
        note_text = comment or "Note"
        if not dry_run:
            annot = page.add_text_annot(point, note_text, icon="Note")
            annot.set_info(**info)
            annot.update()
        rect = fitz.Rect(pt[0], pt[1], pt[0], pt[1])
        add_result(req["page"], rect, note_text)

if not dry_run:
    doc.save(pdf_path, incremental=True, encryption=fitz.PDF_ENCRYPT_KEEP)

payload = json.dumps({"matches": results, "dry_run": dry_run}, ensure_ascii=False)
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	cmd := exec.CommandContext(ctx, pythonCmd, "-", attachment.ResolvedPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return AnnotateResult{}, fmt.Errorf("annotate failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var rawResult struct {
		Matches []annotateMatchRaw `json:"matches"`
		DryRun  bool               `json:"dry_run"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rawResult); err != nil {
		return AnnotateResult{}, err
	}
	matches := make([]AnnotateMatch, len(rawResult.Matches))
	for i, m := range rawResult.Matches {
		var rect [4]float64
		if len(m.Rect) >= 4 {
			copy(rect[:], m.Rect[:4])
		}
		matches[i] = AnnotateMatch{
			Page:  m.Page,
			Text:  m.Text,
			Rect:  rect,
			Type:  req.Type,
			Color: req.Color,
		}
	}
	return AnnotateResult{
		AttachmentKey: attachment.Key,
		PDFPath:       attachment.ResolvedPath,
		Matches:       matches,
		DryRun:        rawResult.DryRun,
	}, nil
}

func (r *LocalReader) AnnotateItem(ctx context.Context, item domain.Item, req AnnotateRequest) (AnnotateResult, error) {
	att, found := firstPDFAttachment(item.Attachments)
	if !found {
		return AnnotateResult{}, fmt.Errorf("item %s has no PDF attachment", item.Key)
	}
	result, err := r.AnnotatePDF(ctx, att, req)
	if err != nil {
		return AnnotateResult{}, err
	}
	result.AttachmentKey = att.Key
	return result, nil
}
