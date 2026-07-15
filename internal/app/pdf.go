package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

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
	Collection    string
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

func (s PDFService) openReader() (config.Config, string, backend.Reader, error) {
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, "", nil, err
	}
	reader, err := s.NewReader(cfg)
	return cfg, configPath, reader, err
}

func (s PDFService) Text(ctx context.Context, req PDFTextRequest) (Result, error) {
	pattern, err := compilePDFTextPattern(req.Grep)
	if err != nil {
		return Result{}, err
	}
	cfg, _, reader, err := s.openReader()
	if err != nil {
		return Result{}, err
	}
	if req.All && cfg.Mode == "remote" {
		return Result{}, fmt.Errorf("pdf text --all is supported in local or hybrid mode")
	}
	items, collection, err := pdfTextItems(ctx, reader, req.Keys, req.All, req.Collection)
	if err != nil {
		return Result{}, err
	}
	ranges, err := parsePageRanges(req.Pages)
	if err != nil {
		return Result{}, err
	}
	fileOutput := req.OutputDir != ""
	pathOnly := shouldReturnPDFCachePaths(cfg.Mode, req)
	outputDir := req.OutputDir
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
		entry, text, err := extractPDFText(ctx, reader, item, req, ranges, pathOnly, pattern)
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
	if collection != nil {
		filters["collection"] = map[string]any{"key": collection.Key, "name": collection.Name, "path": collection.Path}
	}
	if len(filters) > 0 {
		meta["filters"] = filters
	}
	if fileOutput {
		meta["output_dir"] = outputDir
		return Result{Data: results, Meta: meta, Text: fmt.Sprintf("wrote %d Markdown full-text file(s) to %s", len(results), outputDir), Warnings: readWarnings(meta)}, nil
	}
	if len(results) == 0 {
		return Result{Data: results, Meta: meta, Text: "no PDF items matched the selected scope", Warnings: readWarnings(meta)}, nil
	}
	if len(results) > 1 || req.All {
		return Result{Data: results, Meta: meta, Text: fmt.Sprintf("prepared full-text cache paths for %d item(s)", len(results)), Warnings: readWarnings(meta)}, nil
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

func shouldReturnPDFCachePaths(mode string, req PDFTextRequest) bool {
	return mode != "remote" && req.OutputDir == "" && req.Pages == "" && req.Grep == "" && req.MaxChars == 0
}

type pdfCollectionResolver interface {
	CollectionTarget(context.Context, string) (backend.CollectionTarget, error)
}

func pdfTextItems(ctx context.Context, reader backend.Reader, keys []string, all bool, collection string) ([]domain.Item, *backend.CollectionTarget, error) {
	if all {
		items, err := reader.FindItems(ctx, backend.FindOptions{All: true, Full: true, HasPDF: true})
		return items, nil, err
	}
	if strings.TrimSpace(collection) != "" {
		target, err := resolvePDFCollection(ctx, reader, collection)
		if err != nil {
			return nil, nil, err
		}
		items, err := reader.FindItems(ctx, backend.FindOptions{All: true, Full: true, HasPDF: true, Collection: []string{target.Key}})
		return items, &target, err
	}
	items := make([]domain.Item, 0, len(keys))
	for _, key := range keys {
		item, err := reader.GetItem(ctx, key)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, nil, nil
}

func pdfItems(ctx context.Context, reader backend.Reader, keys []string, all bool) ([]domain.Item, error) {
	items, _, err := pdfTextItems(ctx, reader, keys, all, "")
	return items, err
}

func resolvePDFCollection(ctx context.Context, reader backend.Reader, selector string) (backend.CollectionTarget, error) {
	if resolver, ok := reader.(pdfCollectionResolver); ok {
		return resolver.CollectionTarget(ctx, selector)
	}
	collections, err := reader.ListCollections(ctx)
	if err != nil {
		return backend.CollectionTarget{}, err
	}
	value := strings.TrimSpace(selector)
	keyMatches := make([]backend.Collection, 0, 1)
	for _, collection := range collections {
		if strings.EqualFold(collection.Key, value) {
			keyMatches = append(keyMatches, collection)
		}
	}
	if len(keyMatches) == 1 {
		return backend.CollectionTarget{Key: keyMatches[0].Key, Name: keyMatches[0].Name, Path: keyMatches[0].Name}, nil
	}
	matches := make([]backend.Collection, 0, 1)
	for _, collection := range collections {
		if strings.EqualFold(collection.Name, value) {
			matches = append(matches, collection)
		}
	}
	if len(matches) == 0 {
		return backend.CollectionTarget{}, fmt.Errorf("collection %q not found; use `zot coll list` to inspect collection keys and names", selector)
	}
	if len(matches) > 1 {
		return backend.CollectionTarget{}, fmt.Errorf("collection %q is ambiguous; use a collection key", selector)
	}
	return backend.CollectionTarget{Key: matches[0].Key, Name: matches[0].Name, Path: matches[0].Name}, nil
}

func extractPDFText(ctx context.Context, reader backend.Reader, item domain.Item, req PDFTextRequest, ranges []pdfPageRange, pathOnly bool, pattern *regexp.Regexp) (map[string]any, string, error) {
	if req.Pages != "" || pattern != nil {
		extractor, ok := reader.(pageTextExtractor)
		if !ok {
			if req.Pages != "" {
				return nil, "", fmt.Errorf("pdf text --pages requires page-aware extraction support")
			}
		} else {
			entry, text, available, err := extractPDFTextByPages(ctx, extractor, item, req, ranges, pattern)
			if err != nil && req.Pages != "" {
				return nil, "", err
			}
			if err == nil && available {
				return entry, text, nil
			}
		}
	}
	return extractPDFTextWithoutPages(ctx, reader, item, req, pathOnly, pattern)
}

func extractPDFTextByPages(ctx context.Context, extractor pageTextExtractor, item domain.Item, req PDFTextRequest, ranges []pdfPageRange, pattern *regexp.Regexp) (map[string]any, string, bool, error) {
	result, err := extractor.ExtractItemAttachmentPageTexts(ctx, item)
	if err != nil {
		return nil, "", false, err
	}
	if len(result.Attachments) == 0 {
		return nil, "", false, nil
	}
	attachments := make([]map[string]any, 0)
	var combined []string
	returnedPages := make([]int, 0)
	totalChars := 0
	matchCount := 0
	truncatedAny := false
	for _, attachment := range result.Attachments {
		if req.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, req.AttachmentKey) {
			continue
		}
		pages := make([]map[string]any, 0)
		attachmentMatches := 0
		for _, page := range attachment.Pages {
			if !pageInPDFRanges(page.Page, ranges) {
				continue
			}
			text, total, pageMatches, truncated := filterPDFText(page.Text, pattern, req.MaxChars)
			if pattern != nil && pageMatches == 0 {
				continue
			}
			totalChars += total
			matchCount += pageMatches
			attachmentMatches += pageMatches
			truncatedAny = truncatedAny || truncated
			pages = append(pages, map[string]any{"page": page.Page, "text": text, "match_count": pageMatches, "total": total, "returned_chars": utf8.RuneCountInString(text), "truncated": truncated})
			combined = append(combined, text)
			returnedPages = append(returnedPages, page.Page)
		}
		attachments = append(attachments, map[string]any{"attachment_key": attachment.Attachment.Key, "match_count": attachmentMatches, "pages": pages, "full_text_source": attachment.Source, "full_text_cache_hit": attachment.CacheHit})
	}
	if req.AttachmentKey != "" && len(attachments) == 0 {
		return nil, "", false, fmt.Errorf("attachment %s not found on item %s", req.AttachmentKey, item.Key)
	}
	text := strings.Join(combined, "\n")
	return map[string]any{"item_key": item.Key, "title": item.Title, "attachments": attachments, "match_count": matchCount, "total": totalChars, "returned_chars": utf8.RuneCountInString(text), "truncated": truncatedAny, "returned_pages": returnedPages}, text, true, nil
}

func extractPDFTextWithoutPages(ctx context.Context, reader backend.Reader, item domain.Item, req PDFTextRequest, pathOnly bool, pattern *regexp.Regexp) (map[string]any, string, error) {
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
	if pathOnly {
		return pdfTextCachePathEntry(item, result, req.AttachmentKey)
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
	filtered, total, matches, truncated := filterPDFText(text, pattern, req.MaxChars)
	entry := map[string]any{"item_key": item.Key, "title": item.Title, "match_count": matches, "total": total, "returned_chars": utf8.RuneCountInString(filtered), "truncated": truncated}
	if result.PrimaryAttachmentKey != "" {
		entry["primary_attachment_key"] = result.PrimaryAttachmentKey
	}
	attachments := make([]map[string]any, 0)
	for _, attachment := range result.Attachments {
		if req.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, req.AttachmentKey) {
			continue
		}
		value, attachmentTotal, attachmentMatches, attachmentTruncated := filterPDFText(attachment.Text, pattern, req.MaxChars)
		attachments = append(attachments, map[string]any{"attachment_key": attachment.Attachment.Key, "text": value, "match_count": attachmentMatches, "total": attachmentTotal, "returned_chars": utf8.RuneCountInString(value), "truncated": attachmentTruncated, "full_text_source": attachment.Source, "full_text_cache_hit": attachment.CacheHit})
	}
	if len(attachments) > 0 {
		entry["attachments"] = attachments
	} else {
		entry["text"] = filtered
	}
	return entry, filtered, nil
}

func pdfTextCachePathEntry(item domain.Item, result backend.ItemFullTextResult, attachmentKey string) (map[string]any, string, error) {
	selected := make([]backend.AttachmentFullText, 0, len(result.Attachments))
	for _, attachment := range result.Attachments {
		if attachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, attachmentKey) {
			continue
		}
		selected = append(selected, attachment)
	}
	if attachmentKey != "" && len(selected) == 0 {
		return nil, "", fmt.Errorf("attachment %s not found on item %s", attachmentKey, item.Key)
	}
	if len(selected) == 0 {
		return nil, "", fmt.Errorf("full-text cache path is unavailable for item %s", item.Key)
	}
	attachments := make([]map[string]any, 0, len(selected))
	primaryPath := ""
	primaryChunksPath := ""
	primaryKey := result.PrimaryAttachmentKey
	if attachmentKey != "" {
		primaryKey = selected[0].Attachment.Key
	}
	for _, attachment := range selected {
		if strings.TrimSpace(attachment.ContentPath) == "" {
			return nil, "", fmt.Errorf("full-text cache path is unavailable for attachment %s", attachment.Attachment.Key)
		}
		entry := map[string]any{
			"attachment_key":      attachment.Attachment.Key,
			"content_path":        attachment.ContentPath,
			"total_chars":         utf8.RuneCountInString(attachment.Text),
			"full_text_source":    attachment.Source,
			"full_text_cache_hit": attachment.CacheHit,
		}
		if attachment.ChunksPath != "" {
			entry["chunks_path"] = attachment.ChunksPath
		}
		attachments = append(attachments, entry)
		if attachment.Attachment.Key == primaryKey {
			primaryPath = attachment.ContentPath
			primaryChunksPath = attachment.ChunksPath
		}
	}
	if primaryPath == "" {
		primaryKey = selected[0].Attachment.Key
		primaryPath = selected[0].ContentPath
		primaryChunksPath = selected[0].ChunksPath
	}
	entry := map[string]any{
		"item_key":               item.Key,
		"primary_attachment_key": primaryKey,
		"content_path":           primaryPath,
		"attachments":            attachments,
	}
	if primaryChunksPath != "" {
		entry["chunks_path"] = primaryChunksPath
	}
	return entry, primaryPath, nil
}

func compilePDFTextPattern(grep string) (*regexp.Regexp, error) {
	if strings.TrimSpace(grep) == "" {
		return nil, nil
	}
	pattern, err := regexp.Compile("(?i:" + grep + ")")
	if err != nil {
		return nil, fmt.Errorf("invalid --grep regular expression: %w", err)
	}
	return pattern, nil
}

func filterPDFText(text string, pattern *regexp.Regexp, maxChars int) (string, int, int, bool) {
	total := utf8.RuneCountInString(text)
	matches := 0
	if pattern != nil {
		indexes := pattern.FindAllStringIndex(text, -1)
		matches = len(indexes)
		text = pdfTextEvidenceWindows(text, indexes)
	}
	truncated := false
	runes := []rune(text)
	if maxChars > 0 && len(runes) > maxChars {
		text = string(runes[:maxChars])
		truncated = true
	}
	return text, total, matches, truncated
}

func pdfTextEvidenceWindows(text string, indexes [][]int) string {
	lines := strings.Split(text, "\n")
	if len(indexes) == 0 {
		return ""
	}
	lineOffsets := make([]int, len(lines)+1)
	for index, line := range lines {
		lineOffsets[index+1] = lineOffsets[index] + len(line) + 1
	}
	selected := make(map[int]struct{})
	for _, match := range indexes {
		startLine := sort.Search(len(lines), func(i int) bool { return lineOffsets[i+1] > match[0] })
		endOffset := match[1] - 1
		if endOffset < match[0] {
			endOffset = match[0]
		}
		endLine := sort.Search(len(lines), func(i int) bool { return lineOffsets[i+1] > endOffset })
		for neighbor := startLine - 1; neighbor <= endLine+1; neighbor++ {
			if neighbor >= 0 && neighbor < len(lines) {
				selected[neighbor] = struct{}{}
			}
		}
	}
	if len(selected) == 0 {
		return ""
	}
	selectedIndexes := make([]int, 0, len(selected))
	for index := range selected {
		selectedIndexes = append(selectedIndexes, index)
	}
	sort.Ints(selectedIndexes)
	parts := make([]string, 0, len(selectedIndexes))
	previous := -2
	for _, index := range selectedIndexes {
		if previous >= 0 && index > previous+1 {
			parts = append(parts, "…")
		}
		parts = append(parts, lines[index])
		previous = index
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
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
	cfg, configPath, reader, err := s.openReader()
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
		resolvedCfg := cfg
		if resolvedCfg.DataDir == "" {
			if local := localReader(reader); local != nil {
				resolvedCfg.DataDir = local.DataDir
			}
		}
		outputDir, err = defaultFigureOutputDir(resolvedCfg, configPath)
		if err != nil {
			return Result{}, err
		}
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
	return Result{Data: data, Meta: map[string]any{"total": len(data), "figures": totalFigures, "failed": failed, "output_dir": outputDir, "workers": workers}, Text: fmt.Sprintf("%d item(s), %d figure(s), %d failed\nOutput: %s", len(data), totalFigures, failed, outputDir)}, nil
}

func defaultFigureOutputDir(cfg config.Config, configPath string) (string, error) {
	if cfg.DataDir != "" {
		return filepath.Join(cfg.DataDir, ".zotero_cli", "figures"), nil
	}
	if strings.TrimSpace(configPath) == "" {
		var err error
		configPath, err = config.DefaultPath()
		if err != nil {
			return "", fmt.Errorf("resolve default figure output: %w", err)
		}
	}
	return filepath.Join(filepath.Dir(configPath), "figures"), nil
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
	_, _, reader, err := s.openReader()
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
