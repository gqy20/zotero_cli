package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"zotero_cli/internal/domain"
)

type pyMuPDFResult struct {
	Text  string `json:"text"`
	Pages int    `json:"pages"`
	Chars int    `json:"chars"`
}

var findPythonCommandFunc = findPythonCommand

func (r *LocalReader) extractFullTextWithPyMuPDF(ctx context.Context, attachment domain.Attachment) (fullTextDocument, bool, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return fullTextDocument{}, false, nil
	}
	pythonCmd, ok := findPythonCommandFunc()
	if !ok {
		return fullTextDocument{}, false, nil
	}
	script := `
import json
import fitz
import sys

pdf_path = sys.argv[1]
doc = fitz.open(pdf_path)
text = "\n".join(page.get_text() for page in doc)
print(json.dumps({
  "text": text,
  "pages": len(doc),
  "chars": len(text)
}, ensure_ascii=False))
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

func findPythonCommand() (string, bool) {
	candidates := []string{
		filepath.Join(`C:\Users\gqy17\miniforge3`, "python.exe"),
		"python",
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, `\`) {
			if _, err := exec.LookPath(candidate); err == nil {
				return candidate, true
			}
			continue
		}
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}
