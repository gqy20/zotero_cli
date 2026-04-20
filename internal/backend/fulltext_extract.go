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

type extractBlock struct {
	Page int        `json:"page"`
	BBox [4]float64 `json:"bbox"`
	Text string     `json:"text"`
	Size float64    `json:"size"`
	Bold bool       `json:"bold"`
}

type pyMuPDFDictResult struct {
	Blocks     []extractBlock `json:"blocks"`
	Pages      int            `json:"pages"`
	TotalChars int            `json:"total_chars"`
}

func (r *LocalReader) extractFullTextWithPyMuPDF(ctx context.Context, attachment domain.Attachment) (FullTextDocument, bool, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return FullTextDocument{}, false, nil
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return FullTextDocument{}, false, nil
	}
	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
doc = fitz.open(pdf_path)
blocks = []
for pi, page in enumerate(doc):
    try:
        d = page.get_text("dict")
    except Exception:
        continue
    for block in d.get("blocks", []):
        if block.get("type") != 0:
            continue
        bbox = block.get("bbox", [0, 0, 0, 0])
        lines_text = []
        max_size = 0
        has_bold = False
        for line in block.get("lines", []):
            for span in line.get("spans", []):
                size = span.get("size", 0)
                flags = span.get("flags", 0)
                if size > max_size:
                    max_size = size
                if flags & (2**4):
                    has_bold = True
                st = span.get("text", "")
                if st:
                    lines_text.append(st)
        text = " ".join(lines_text).strip()
        if text:
            blocks.append({
                "page": pi + 1,
                "bbox": [round(bbox[0], 1), round(bbox[1], 1), round(bbox[2], 1), round(bbox[3], 1)],
                "text": text,
                "size": round(max_size, 1),
                "bold": has_bold
            })
payload = json.dumps({
  "blocks": blocks,
  "pages": len(doc),
  "total_chars": sum(len(b["text"]) for b in blocks)
}, ensure_ascii=False)
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	cmd := exec.CommandContext(ctx, pythonCmd, "-", attachment.ResolvedPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return FullTextDocument{}, false, fmt.Errorf("pymupdf extract failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var dictResult pyMuPDFDictResult
	if err := json.Unmarshal(stdout.Bytes(), &dictResult); err != nil {
		return FullTextDocument{}, false, err
	}
	if len(dictResult.Blocks) == 0 {
		return FullTextDocument{}, false, nil
	}
	sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
	if !ok {
		return FullTextDocument{}, false, nil
	}
	chunks := blocksToChunks(dictResult.Blocks)
	fullText := chunksToPlainText(chunks)
	if fullText == "" {
		return FullTextDocument{}, false, nil
	}
	return FullTextDocument{
		Text:   normalizeFullTextText(fullText),
		Chunks: chunks,
		Meta: fullTextCacheMeta{
			AttachmentKey:   attachment.Key,
			ResolvedPath:    sourcePath,
			ContentType:     attachment.ContentType,
			Extractor:       "pymupdf",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
			Pages:           dictResult.Pages,
			Chars:           dictResult.TotalChars,
		},
	}, true, nil
}
