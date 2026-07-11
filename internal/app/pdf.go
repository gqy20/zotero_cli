package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type PDFTextRequest struct {
	Keys          []string
	All           bool
	OutputDir     string
	Pages         string
	MaxChars      int
	Grep          string
	AttachmentKey string
}

type PDFFiguresRequest struct {
	Keys       []string
	All        bool
	OutputDir  string
	Workers    int
	MaxPerPage int
}

type PDFOpenRequest struct {
	Key  string
	Page int
}

type PDFService struct {
	LoadConfig func() (config.Config, string, error)
	NewReader  func(config.Config) (backend.Reader, error)
	OpenFile   func(string) error
}

func NewPDFService() PDFService {
	read := NewReadService()
	return PDFService{LoadConfig: config.Load, NewReader: read.NewReader, OpenFile: openSystemFile}
}

type fullTextExtractor interface {
	ExtractItemFullText(context.Context, domain.Item) (string, error)
}

type attachmentTextExtractor interface {
	ExtractItemAttachmentTexts(context.Context, domain.Item) (backend.ItemFullTextResult, error)
}

type pageTextExtractor interface {
	ExtractItemAttachmentPageTexts(context.Context, domain.Item) (backend.ItemPageTextResult, error)
}

type localFigureExtractor interface {
	ExtractFigures(context.Context, domain.Item, string) (backend.ExtractFiguresResult, error)
}

type remoteFigureExtractor interface {
	ExtractFigures(context.Context, string, string) (backend.ExtractFiguresResult, error)
}

func (s PDFService) openReader() (config.Config, backend.Reader, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	reader, err := s.NewReader(cfg)
	return cfg, reader, err
}

func (s PDFService) Text(ctx context.Context, req PDFTextRequest) (Result, error) {
	cfg, reader, err := s.openReader()
	if err != nil {
		return Result{}, err
	}
	if req.All && cfg.Mode == "remote" {
		return Result{}, fmt.Errorf("pdf text --all is supported in local or hybrid mode")
	}
	items, err := pdfItems(ctx, reader, req.Keys, req.All)
	if err != nil {
		return Result{}, err
	}
	ranges, err := parsePageRanges(req.Pages)
	if err != nil {
		return Result{}, err
	}
	fileOutput := req.OutputDir != "" || req.All || len(items) > 1
	outputDir := req.OutputDir
	if fileOutput && outputDir == "" {
		outputDir = filepath.Join(cfg.DataDir, ".zotero_cli", "markdown")
	}
	if fileOutput {
		outputDir, err = filepath.Abs(outputDir)
		if err != nil {
			return Result{}, err
		}
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return Result{}, err
		}
	}
	results := make([]map[string]any, 0, len(items))
	var singleText string
	for _, item := range items {
		entry, text, err := extractPDFText(ctx, reader, item, req, ranges)
		if err != nil {
			if len(items) == 1 {
				return Result{}, err
			}
			entry = map[string]any{"item_key": item.Key, "title": item.Title, "error": err.Error()}
		}
		if fileOutput && err == nil {
			name := sanitizePDFName(firstNonEmpty(item.Title, item.Key)) + "-" + item.Key + ".md"
			path := filepath.Join(outputDir, name)
			content := fmt.Sprintf("# %s\n\n- Zotero key: `%s`\n\n%s\n", item.Title, item.Key, text)
			if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
				entry["error"] = writeErr.Error()
			} else {
				entry["file"] = path
			}
		}
		results = append(results, entry)
		singleText = text
	}
	meta := readMeta(reader)
	meta["total"] = len(results)
	filters := map[string]any{}
	if req.Pages != "" {
		filters["pages"] = req.Pages
	}
	if req.Grep != "" {
		filters["grep"] = req.Grep
	}
	if req.MaxChars > 0 {
		filters["max_chars"] = req.MaxChars
	}
	if req.AttachmentKey != "" {
		filters["attachment_key"] = req.AttachmentKey
	}
	if len(filters) > 0 {
		meta["filters"] = filters
	}
	if fileOutput {
		meta["output_dir"] = outputDir
		return Result{Data: results, Meta: meta, Text: fmt.Sprintf("wrote %d Markdown full-text file(s) to %s", len(results), outputDir), Warnings: readWarnings(meta)}, nil
	}
	if total, ok := results[0]["total"]; ok {
		meta["total"] = total
	}
	if returned, ok := results[0]["returned_chars"]; ok {
		meta["returned_chars"] = returned
	}
	if truncated, _ := results[0]["truncated"].(bool); truncated {
		meta["truncated"] = true
	}
	if pages, ok := results[0]["returned_pages"]; ok {
		meta["returned_pages"] = pages
	}
	return Result{Data: results[0], Meta: meta, Text: singleText, Warnings: readWarnings(meta)}, nil
}

func pdfItems(ctx context.Context, reader backend.Reader, keys []string, all bool) ([]domain.Item, error) {
	if all {
		return reader.FindItems(ctx, backend.FindOptions{All: true, Full: true, HasPDF: true})
	}
	items := make([]domain.Item, 0, len(keys))
	for _, key := range keys {
		item, err := reader.GetItem(ctx, key)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func extractPDFText(ctx context.Context, reader backend.Reader, item domain.Item, req PDFTextRequest, ranges []pdfPageRange) (map[string]any, string, error) {
	if req.Pages != "" {
		extractor, ok := reader.(pageTextExtractor)
		if !ok {
			return nil, "", fmt.Errorf("pdf text --pages requires page-aware extraction support")
		}
		result, err := extractor.ExtractItemAttachmentPageTexts(ctx, item)
		if err != nil {
			return nil, "", err
		}
		attachments := make([]map[string]any, 0)
		var combined []string
		returnedPages := make([]int, 0)
		totalChars := 0
		truncatedAny := false
		for _, attachment := range result.Attachments {
			if req.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, req.AttachmentKey) {
				continue
			}
			pages := make([]map[string]any, 0)
			for _, page := range attachment.Pages {
				if !pageInPDFRanges(page.Page, ranges) {
					continue
				}
				text, total, truncated := filterPDFText(page.Text, req.Grep, req.MaxChars)
				totalChars += total
				truncatedAny = truncatedAny || truncated
				pages = append(pages, map[string]any{"page": page.Page, "text": text, "total": total, "returned_chars": len(text), "truncated": truncated})
				combined = append(combined, text)
				returnedPages = append(returnedPages, page.Page)
			}
			attachments = append(attachments, map[string]any{"attachment_key": attachment.Attachment.Key, "pages": pages})
		}
		if req.AttachmentKey != "" && len(attachments) == 0 {
			return nil, "", fmt.Errorf("attachment %s not found on item %s", req.AttachmentKey, item.Key)
		}
		text := strings.Join(combined, "\n")
		return map[string]any{"item_key": item.Key, "text": text, "attachments": attachments, "total": totalChars, "returned_chars": len(text), "truncated": truncatedAny, "returned_pages": returnedPages}, text, nil
	}
	var result backend.ItemFullTextResult
	if extractor, ok := reader.(attachmentTextExtractor); ok {
		var err error
		result, err = extractor.ExtractItemAttachmentTexts(ctx, item)
		if err != nil {
			return nil, "", err
		}
	} else if extractor, ok := reader.(fullTextExtractor); ok {
		text, err := extractor.ExtractItemFullText(ctx, item)
		if err != nil {
			return nil, "", err
		}
		result.Text = text
	} else {
		return nil, "", fmt.Errorf("pdf text requires full-text extraction support")
	}
	text := result.Text
	if req.AttachmentKey != "" {
		text = ""
		for _, attachment := range result.Attachments {
			if strings.EqualFold(attachment.Attachment.Key, req.AttachmentKey) {
				text = attachment.Text
				break
			}
		}
		if text == "" {
			return nil, "", fmt.Errorf("attachment %s not found on item %s", req.AttachmentKey, item.Key)
		}
	}
	filtered, total, truncated := filterPDFText(text, req.Grep, req.MaxChars)
	entry := map[string]any{"item_key": item.Key, "text": filtered, "total": total, "returned_chars": len(filtered), "truncated": truncated}
	if result.PrimaryAttachmentKey != "" {
		entry["primary_attachment_key"] = result.PrimaryAttachmentKey
	}
	attachments := make([]map[string]any, 0)
	for _, attachment := range result.Attachments {
		if req.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, req.AttachmentKey) {
			continue
		}
		value, attachmentTotal, attachmentTruncated := filterPDFText(attachment.Text, req.Grep, req.MaxChars)
		attachments = append(attachments, map[string]any{"attachment_key": attachment.Attachment.Key, "text": value, "total": attachmentTotal, "returned_chars": len(value), "truncated": attachmentTruncated, "full_text_source": attachment.Source, "full_text_cache_hit": attachment.CacheHit})
	}
	if len(attachments) > 0 {
		entry["attachments"] = attachments
	}
	return entry, filtered, nil
}

func filterPDFText(text, grep string, maxChars int) (string, int, bool) {
	total := len(text)
	if grep != "" {
		lines := strings.Split(text, "\n")
		matched := lines[:0]
		needle := strings.ToLower(grep)
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), needle) {
				matched = append(matched, line)
			}
		}
		text = strings.Join(matched, "\n")
	}
	truncated := false
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}
	return text, total, truncated
}

type pdfPageRange struct{ Start, End int }

func parsePageRanges(value string) ([]pdfPageRange, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var result []pdfPageRange
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		startRaw, endRaw, hasRange := strings.Cut(part, "-")
		start, err := strconv.Atoi(strings.TrimSpace(startRaw))
		if err != nil || start < 1 {
			return nil, fmt.Errorf("invalid --pages value %q", value)
		}
		end := start
		if hasRange {
			end, err = strconv.Atoi(strings.TrimSpace(endRaw))
			if err != nil || end < start {
				return nil, fmt.Errorf("invalid --pages value %q", value)
			}
		}
		result = append(result, pdfPageRange{start, end})
	}
	return result, nil
}

func pageInPDFRanges(page int, ranges []pdfPageRange) bool {
	if len(ranges) == 0 {
		return true
	}
	for _, value := range ranges {
		if page >= value.Start && page <= value.End {
			return true
		}
	}
	return false
}

func sanitizePDFName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, value)
	if len(value) > 100 {
		value = value[:100]
	}
	if value == "" {
		return "item"
	}
	return value
}

func (s PDFService) Figures(ctx context.Context, req PDFFiguresRequest) (Result, error) {
	cfg, reader, err := s.openReader()
	if err != nil {
		return Result{}, err
	}
	if req.All && cfg.Mode == "remote" {
		return Result{}, fmt.Errorf("pdf figs --all is supported in local or hybrid mode")
	}
	items, err := pdfItems(ctx, reader, req.Keys, req.All)
	if err != nil {
		return Result{}, err
	}
	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(cfg.DataDir, ".zotero_cli", "figures")
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return Result{}, err
	}
	workers := req.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers < 2 {
			workers = 2
		}
		if workers > 8 {
			workers = 8
		}
	}
	type figureResult struct {
		key   string
		value backend.ExtractFiguresResult
		err   error
	}
	results := make([]figureResult, 0, len(items))
	var mu sync.Mutex
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, item := range items {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var value backend.ExtractFiguresResult
			var extractErr error
			if remote, ok := reader.(remoteFigureExtractor); ok {
				value, extractErr = remote.ExtractFigures(ctx, item.Key, outputDir)
			} else if local, ok := reader.(localFigureExtractor); ok {
				value, extractErr = local.ExtractFigures(ctx, item, outputDir)
			} else {
				extractErr = fmt.Errorf("pdf figs requires figure extraction support")
			}
			if extractErr != nil {
				value.Error = extractErr.Error()
			}
			capPDFPageFigures(&value, outputDir, req.MaxPerPage)
			mu.Lock()
			results = append(results, figureResult{item.Key, value, extractErr})
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].key < results[j].key })
	data := make([]map[string]any, 0, len(results))
	totalFigures := 0
	failed := 0
	for _, result := range results {
		if result.err != nil {
			failed++
		}
		totalFigures += len(result.value.Figures)
		data = append(data, map[string]any{"item_key": result.key, "pdf": filepath.Base(result.value.PDFPath), "total_pages": result.value.TotalPages, "figures": result.value.Figures, "elapsed_sec": result.value.ElapsedSec, "error": result.value.Error})
	}
	return Result{Data: data, Meta: map[string]any{"total": len(data), "figures": totalFigures, "failed": failed, "output_dir": outputDir, "workers": workers}, Text: fmt.Sprintf("%d item(s), %d figure(s), %d failed", len(data), totalFigures, failed)}, nil
}

func capPDFPageFigures(result *backend.ExtractFiguresResult, outputDir string, max int) {
	if max <= 0 {
		return
	}
	byPage := map[int][]backend.FigureInfo{}
	for _, figure := range result.Figures {
		byPage[figure.Page] = append(byPage[figure.Page], figure)
	}
	kept := make([]backend.FigureInfo, 0, len(result.Figures))
	for _, figures := range byPage {
		sort.SliceStable(figures, func(i, j int) bool { return pdfFigureArea(figures[i].SizePx) > pdfFigureArea(figures[j].SizePx) })
		for i, figure := range figures {
			if i < max {
				kept = append(kept, figure)
			} else {
				_ = os.Remove(filepath.Join(outputDir, figure.AttachmentKey, figure.File))
			}
		}
	}
	result.Figures = kept
}

func pdfFigureArea(value string) int64 {
	width, height, _ := strings.Cut(value, "x")
	w, _ := strconv.ParseInt(strings.TrimSpace(width), 10, 64)
	h, _ := strconv.ParseInt(strings.TrimSpace(height), 10, 64)
	return w * h
}

func (s PDFService) Open(ctx context.Context, req PDFOpenRequest) (Result, error) {
	_, reader, err := s.openReader()
	if err != nil {
		return Result{}, err
	}
	item, err := reader.GetItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	var attachment domain.Attachment
	for _, candidate := range item.Attachments {
		if strings.EqualFold(candidate.ContentType, "application/pdf") {
			attachment = candidate
			break
		}
	}
	if attachment.Key == "" {
		return Result{}, fmt.Errorf("item %s has no PDF attachment", req.Key)
	}
	path := attachment.ResolvedPath
	if path == "" {
		path, _, err = reader.GetAttachmentFile(ctx, attachment.Key)
		if err != nil {
			return Result{}, err
		}
	}
	if err := s.OpenFile(path); err != nil {
		return Result{}, fmt.Errorf("open PDF: %w", err)
	}
	data := map[string]any{"item_key": item.Key, "attachment_key": attachment.Key, "path": path, "page": req.Page}
	text := fmt.Sprintf("Opened: %s\nItem: %s (%s)\nAttachment: %s", path, item.Key, item.Title, attachment.Key)
	if req.Page > 0 {
		text += fmt.Sprintf("\nPage hint: %d", req.Page)
	}
	return Result{Data: data, Meta: readMeta(reader), Text: text}, nil
}

func openSystemFile(path string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		command = exec.Command("open", path)
	default:
		command = exec.Command("xdg-open", path)
	}
	return command.Start()
}
