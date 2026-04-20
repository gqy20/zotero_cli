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

type pyMuPDFResult struct {
	Text  string `json:"text"`
	Pages int    `json:"pages"`
	Chars int    `json:"chars"`
}

func (r *LocalReader) extractFullTextWithPyMuPDF(ctx context.Context, attachment domain.Attachment) (fullTextDocument, bool, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return fullTextDocument{}, false, nil
	}
	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return fullTextDocument{}, false, nil
	}
	script := `
import json, sys
import fitz

pdf_path = sys.argv[1]
doc = fitz.open(pdf_path)
parts = []
for page in doc:
    text = page.get_text("text", flags=fitz.TEXT_PRESERVE_WHITESPACE)
    if text.strip():
        parts.append(text)
payload = json.dumps({
  "text": "\n".join(parts),
  "pages": len(doc),
  "chars": sum(len(p) for p in parts)
}, ensure_ascii=False)
sys.stdout.buffer.write(payload.encode("utf-8"))
`
	cmd := exec.CommandContext(ctx, pythonCmd, "-", attachment.ResolvedPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fullTextDocument{}, false, fmt.Errorf("pymupdf extract failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result pyMuPDFResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return fullTextDocument{}, false, err
	}
	if strings.TrimSpace(result.Text) == "" {
		return fullTextDocument{}, false, nil
	}
	sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
	if !ok {
		return fullTextDocument{}, false, nil
	}
	return fullTextDocument{
		Text: normalizeFullTextText(result.Text),
		Meta: fullTextCacheMeta{
			AttachmentKey:   attachment.Key,
			ResolvedPath:    sourcePath,
			ContentType:     attachment.ContentType,
			Extractor:       "pymupdf",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
			Pages:           result.Pages,
			Chars:           result.Chars,
		},
	}, true, nil
}

