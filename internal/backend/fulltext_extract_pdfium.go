package backend

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"zotero_cli/internal/domain"
)

var (
	extractFullTextWithPDFiumFunc = func(ctx context.Context, reader *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		return reader.extractFullTextWithPDFium(ctx, attachment)
	}

	pdfiumPoolOnce sync.Once
	pdfiumPoolRef  pdfium.Pool
	pdfiumPoolErr  error
)

func getPDFiumPool() (pdfium.Pool, error) {
	pdfiumPoolOnce.Do(func() {
		pdfiumPoolRef, pdfiumPoolErr = webassembly.Init(webassembly.Config{
			MinIdle:      1,
			MaxIdle:      1,
			MaxTotal:     1,
			ReuseWorkers: true,
		})
	})
	if pdfiumPoolErr != nil {
		return nil, pdfiumPoolErr
	}
	return pdfiumPoolRef, nil
}

func (r *LocalReader) extractFullTextWithPDFium(ctx context.Context, attachment domain.Attachment) (FullTextDocument, bool, error) {
	if !attachment.Resolved || strings.TrimSpace(attachment.ResolvedPath) == "" {
		return FullTextDocument{}, false, nil
	}

	pdfBytes, err := os.ReadFile(attachment.ResolvedPath)
	if err != nil {
		return FullTextDocument{}, false, err
	}

	pool, err := getPDFiumPool()
	if err != nil {
		return FullTextDocument{}, false, fmt.Errorf("init pdfium: %w", err)
	}

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		return FullTextDocument{}, false, fmt.Errorf("get pdfium instance: %w", err)
	}
	defer instance.Close()

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
	if err != nil {
		return FullTextDocument{}, false, fmt.Errorf("open pdf with pdfium: %w", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return FullTextDocument{}, false, fmt.Errorf("get pdf page count: %w", err)
	}

	pageTexts := make([]string, 0, pageCount.PageCount)
	totalChars := 0
	for pageIndex := 0; pageIndex < pageCount.PageCount; pageIndex++ {
		pageText, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    pageIndex,
				},
			},
		})
		if err != nil {
			return FullTextDocument{}, false, fmt.Errorf("extract pdf text for page %d: %w", pageIndex+1, err)
		}
		totalChars += len([]rune(pageText.Text))
		if strings.TrimSpace(pageText.Text) != "" {
			pageTexts = append(pageTexts, pageText.Text)
		}
	}

	text := strings.Join(pageTexts, "\n")
	if strings.TrimSpace(text) == "" {
		return FullTextDocument{}, false, nil
	}

	sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
	if !ok {
		return FullTextDocument{}, false, nil
	}

	return FullTextDocument{
		Text: normalizeFullTextText(text),
		Meta: fullTextCacheMeta{
			AttachmentKey:   attachment.Key,
			ResolvedPath:    sourcePath,
			ContentType:     attachment.ContentType,
			Extractor:       "pdfium",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
			Pages:           pageCount.PageCount,
			Chars:           totalChars,
		},
	}, true, nil
}
