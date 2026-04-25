package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

func (c *CLI) runExtractText(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageExtractText)
	}

	itemKey, jsonOutput, ok := c.parseExtractTextArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		return c.printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		var (
			result backend.ItemFullTextResult
			err    error
		)
		if attachmentReader, ok := localReader.(attachmentTextReader); ok {
			result, err = attachmentReader.ExtractItemAttachmentTexts(context.Background(), item)
		} else {
			textReader, ok := localReader.(fullTextReader)
			if !ok {
				return c.printErr(fmt.Errorf("extract-text requires local full-text extraction support"))
			}
			var text string
			text, err = textReader.ExtractItemFullText(context.Background(), item)
			result = backend.ItemFullTextResult{Text: text}
		}
		if err != nil {
			return c.printErr(err)
		}

		readMeta := c.consumeReaderReadMetadata(localReader)
		meta := map[string]any{
			"total": len([]rune(result.Text)),
		}
		c.appendExplicitReadMetadata(meta, readMeta)
		attachments := make([]map[string]any, 0, len(result.Attachments))
		for _, attachment := range result.Attachments {
			entry := map[string]any{
				"attachment_key": attachment.Attachment.Key,
				"text":           attachment.Text,
				"total":          len([]rune(attachment.Text)),
			}
			if attachment.Attachment.Title != "" {
				entry["title"] = attachment.Attachment.Title
			}
			if attachment.Attachment.Filename != "" {
				entry["filename"] = attachment.Attachment.Filename
			}
			if attachment.Attachment.ResolvedPath != "" {
				entry["resolved_path"] = attachment.Attachment.ResolvedPath
			}
			if attachment.Source != "" {
				entry["full_text_source"] = attachment.Source
			}
			if attachment.CacheHit {
				entry["full_text_cache_hit"] = true
			}
			attachments = append(attachments, entry)
		}
		data := map[string]any{
			"item_key": item.Key,
			"text":     result.Text,
		}
		if result.PrimaryAttachmentKey != "" {
			data["primary_attachment_key"] = result.PrimaryAttachmentKey
		}
		if len(attachments) > 0 {
			data["attachments"] = attachments
		}
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "extract-text",
			Data:    data,
			Meta:    meta,
		})
	}

	textReader, ok := localReader.(fullTextReader)
	if !ok {
		return c.printErr(fmt.Errorf("extract-text requires local full-text extraction support"))
	}
	text, err := textReader.ExtractItemFullText(context.Background(), item)
	if err != nil {
		return c.printErr(err)
	}
	readMeta := c.consumeReaderReadMetadata(localReader)
	c.warnIfSnapshotRead(readMeta)
	fmt.Fprintln(c.stdout, text)
	return 0
}

func (c *CLI) parseExtractTextArgs(args []string) (string, bool, bool) {
	itemKey := ""
	jsonOutput := false

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "--") || itemKey != "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return "", false, false
			}
			itemKey = arg
		}
	}

	if strings.TrimSpace(itemKey) == "" {
		fmt.Fprintln(c.stderr, usageExtractText)
		return "", false, false
	}
	return itemKey, jsonOutput, true
}

func filterPDFAttachments(attachments []domain.Attachment) []domain.Attachment {
	filtered := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		if strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			filtered = append(filtered, attachment)
		}
	}
	return filtered
}

// figureTaskResult holds the result of extracting figures from one item.
type figureTaskResult struct {
	itemKey string
	result  backend.ExtractFiguresResult
	err     error
}

// pdfJob wraps an item with its estimated page count for sorting.
type pdfJob struct {
	key     string
	item    domain.Item
	pageEst int // estimated page count for LPT (longest-processing-time) sorting
}

// estimatePages parses Zotero's "pages" field (e.g., "1-15", "12-20") or returns 0.
func estimatePages(item domain.Item) int {
	s := strings.TrimSpace(item.Pages)
	if s == "" {
		return 0
	}
	var start, end int
	if n, _ := fmt.Sscanf(s, "%d-%d", &start, &end); n == 2 && end >= start {
		return end - start + 1
	}
	if n, _ := fmt.Sscanf(s, "%d", &start); n == 1 {
		return start
	}
	return 0
}

// capFiguresPerPage removes excess figures when a page exceeds maxPerPage.
// It keeps the largest figures by pixel area and deletes truncated files from disk.
func capFiguresPerPage(result *backend.ExtractFiguresResult, outputDir string, maxPerPage int) {
	if maxPerPage <= 0 || len(result.Figures) == 0 {
		return
	}
	type pageGroup struct {
		figs []*backend.FigureInfo
	}
	pages := make(map[int]pageGroup)
	for i := range result.Figures {
		f := &result.Figures[i]
		pg := pages[f.Page]
		pg.figs = append(pg.figs, f)
		pages[f.Page] = pg
	}

	var kept []backend.FigureInfo
	var trimmed int
	for _, pg := range pages {
		if len(pg.figs) <= maxPerPage {
			for _, f := range pg.figs {
				kept = append(kept, *f)
			}
			continue
		}
		trimmed += len(pg.figs) - maxPerPage
		sort.Slice(pg.figs, func(i, j int) bool {
			a := parseArea(pg.figs[i].SizePx)
			b := parseArea(pg.figs[j].SizePx)
			return a > b
		})
		for i := 0; i < maxPerPage; i++ {
			kept = append(kept, *pg.figs[i])
		}
		for i := maxPerPage; i < len(pg.figs); i++ {
			f := pg.figs[i]
			fp := filepath.Join(outputDir, f.AttachmentKey, f.File)
			os.Remove(fp)
		}
	}
	result.Figures = kept
	if trimmed > 0 && result.Error != "" {
		result.Error += fmt.Sprintf("; capped %d excess figures (max %d/page)", trimmed, maxPerPage)
	} else if trimmed > 0 {
		result.Error = fmt.Sprintf("capped %d excess figures (max %d/page)", trimmed, maxPerPage)
	}
}

func parseArea(sizePx string) int64 {
	w, h, _ := strings.Cut(sizePx, "x")
	wi, _ := strconv.ParseInt(strings.TrimSpace(w), 10, 64)
	hi, _ := strconv.ParseInt(strings.TrimSpace(h), 10, 64)
	return wi * hi
}

func (c *CLI) runExtractFigures(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageExtractFigures)
	}

	itemKeys, outputDir, jsonOutput, workers, maxPerPage, ok := c.parseExtractFiguresArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		return c.printErr(err)
	}

	figExtractor, ok := localReader.(interface {
		ExtractFigures(ctx context.Context, item domain.Item, outputDir string) (backend.ExtractFiguresResult, error)
	})
	if !ok {
		return c.printErr(fmt.Errorf("extract-figures requires local reader with figure extraction support"))
	}

	// Resolve output directory
	if outputDir == "" {
		outputDir = filepath.Join(cfg.DataDir, ".zotero_cli", "figures")
	}
	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return c.printErr(err)
	}

	// Default workers: CPU count, min 2, max 8
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 8 {
			workers = 8
		}
		if workers < 2 {
			workers = 2
		}
	}

	ctx := context.Background()

	// Single item: run directly (no goroutine overhead)
	if len(itemKeys) == 1 {
		item, err := localReader.GetItem(ctx, itemKeys[0])
		if err != nil {
			return c.printErr(err)
		}
		res, err := figExtractor.ExtractFigures(ctx, item, absOutDir)
		if err != nil {
			res.Error = err.Error()
		}
		capFiguresPerPage(&res, absOutDir, maxPerPage)
		return c.outputFiguresResults([]figureTaskResult{{itemKey: itemKeys[0], result: res, err: err}}, jsonOutput)
	}

	// Multiple items: pre-fetch → filter PDF items → sort by page count (LPT) → parallel
	// Phase 1: pre-resolve all items and filter out those without PDFs
	type preloadResult struct {
		key  string
		item domain.Item
		err  error
	}
	preloads := make([]preloadResult, len(itemKeys))
	for i, key := range itemKeys {
		item, err := localReader.GetItem(ctx, key)
		preloads[i] = preloadResult{key: key, item: item, err: err}
	}

	// Phase 2: build job list — only items with resolvable PDF attachments
	jobs := make([]pdfJob, 0, len(itemKeys))
	skipResults := make([]figureTaskResult, 0)
	for _, p := range preloads {
		if p.err != nil {
			skipResults = append(skipResults, figureTaskResult{itemKey: p.key, err: p.err})
			continue
		}
		hasPDF := false
		for _, att := range p.item.Attachments {
			if strings.EqualFold(strings.TrimSpace(att.ContentType), "application/pdf") && att.Resolved && att.ResolvedPath != "" {
				hasPDF = true
				break
			}
		}
		if !hasPDF {
			skipResults = append(skipResults, figureTaskResult{
				itemKey: p.key,
				result:  backend.ExtractFiguresResult{ItemKey: p.key},
				err:     fmt.Errorf("no PDF attachment found for item %s", p.key),
			})
			continue
		}
		jobs = append(jobs, pdfJob{key: p.key, item: p.item, pageEst: estimatePages(p.item)})
	}

	// Phase 2b: get real page counts from PDF files via PyMuPDF (fast metadata-only read)
	pdfPaths := make([]string, len(jobs))
	pathToIdxs := make(map[string][]int, len(jobs))
	for i, job := range jobs {
		for _, att := range job.item.Attachments {
			if strings.EqualFold(strings.TrimSpace(att.ContentType), "application/pdf") && att.Resolved && att.ResolvedPath != "" {
				pdfPaths[i] = att.ResolvedPath
				pathToIdxs[att.ResolvedPath] = append(pathToIdxs[att.ResolvedPath], i)
				break
			}
		}
	}
	pageCounts, pageErr := backend.CountPDFPages(cfg.DataDir, pdfPaths)
	if pageErr == nil {
		for path, cnt := range pageCounts {
			if cnt <= 0 {
				continue
			}
			for _, idx := range pathToIdxs[path] {
				jobs[idx].pageEst = cnt
			}
		}
	}

	// Phase 3: sort by actual page count descending (LPT — longest jobs first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].pageEst > jobs[j].pageEst
	})

	// Phase 4: parallel extraction with semaphore
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, workers)
		results []figureTaskResult
	)
	results = append(results, skipResults...)

	for _, job := range jobs {
		wg.Add(1)
		j := job
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, extractErr := figExtractor.ExtractFigures(ctx, j.item, absOutDir)
			if extractErr != nil {
				res.Error = extractErr.Error()
			}

			mu.Lock()
			results = append(results, figureTaskResult{itemKey: j.key, result: res, err: extractErr})
			mu.Unlock()
		}()
	}

	wg.Wait()
	for i := range results {
		capFiguresPerPage(&results[i].result, absOutDir, maxPerPage)
	}

	return c.outputFiguresResults(results, jsonOutput)
}

func (c *CLI) outputFiguresResults(results []figureTaskResult, jsonOutput bool) int {
	if jsonOutput {
		allData := make([]map[string]any, 0, len(results))
		allFigs := 0
		var errs []string

		for _, r := range results {
			entry := map[string]any{
				"item_key": r.itemKey,
				"error":    r.result.Error,
			}
			if r.result.Error == "" || len(r.result.Figures) > 0 {
				if r.result.PDFPath != "" {
					entry["pdf"] = filepath.Base(r.result.PDFPath)
				}
				entry["total_pages"] = r.result.TotalPages
				figures := r.result.Figures
				if figures == nil {
					figures = []backend.FigureInfo{}
				}
				entry["figures"] = figures
				entry["elapsed_sec"] = r.result.ElapsedSec
				entry["method"] = r.result.Method
				allFigs += len(r.result.Figures)
			}
			allData = append(allData, entry)
			if r.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %s", r.itemKey, r.err.Error()))
			}
		}

		meta := map[string]any{
			"total_items":   len(results),
			"total_figures": allFigs,
		}
		if len(errs) > 0 {
			meta["errors"] = errs
		}

		return c.writeJSON(jsonResponse{
			OK:      len(errs) == 0,
			Command: "extract-figures",
			Data:    allData,
			Meta:    meta,
		})
	}

	hasAny := false
	for _, r := range results {
		if r.result.Error == "" && len(r.result.Figures) > 0 {
			hasAny = true
			fmt.Fprintf(c.stdout, "\n[%s] %d figure(s) in %.1fs\n",
				r.itemKey, len(r.result.Figures), r.result.ElapsedSec)
			for _, fig := range r.result.Figures {
				srcTag := "V"
				if fig.Source == "raster" {
					srcTag = "R"
				}
				capTag := ""
				if fig.HasCaption {
					capTag = " +caption"
				}
				fmt.Fprintf(c.stdout, "  [%s] %s  p.%d %s%s %s %.1fkB anchors=%d\n",
					fig.AttachmentKey, fig.File, fig.Page, srcTag,
					fig.SizePx, capTag, fig.KB, fig.Anchors)
			}
		} else if r.result.Error != "" {
			fmt.Fprintf(c.stderr, "[%s] error: %s\n", r.itemKey, r.result.Error)
		}
	}

	if !hasAny {
		for _, r := range results {
			if r.result.Error != "" {
				fmt.Fprintf(c.stderr, "[%s] error: %s\n", r.itemKey, r.result.Error)
			} else {
				fmt.Fprintf(c.stdout, "[%s] no figures found\n", r.itemKey)
			}
		}
	}

	return 0
}

func (c *CLI) parseExtractFiguresArgs(args []string) ([]string, string, bool, int, int, bool) {
	var itemKeys []string
	outputDir := ""
	jsonOutput := false
	workers := 0
	maxPerPage := 25
	expectOutputDir := false
	expectWorkers := false
	expectMaxPerPage := false

	for _, arg := range args {
		if expectOutputDir {
			outputDir = arg
			expectOutputDir = false
			continue
		}
		if expectWorkers {
			_, err := fmt.Sscanf(arg, "%d", &workers)
			if err != nil || workers <= 0 {
				fmt.Fprintf(c.stderr, "%s\ninvalid --workers value: %s\n", usageExtractFigures, arg)
				return nil, "", false, 0, 0, false
			}
			expectWorkers = false
			continue
		}
		if expectMaxPerPage {
			_, err := fmt.Sscanf(arg, "%d", &maxPerPage)
			if err != nil || maxPerPage < 1 {
				fmt.Fprintf(c.stderr, "%s\ninvalid --max-per-page value: %s\n", usageExtractFigures, arg)
				return nil, "", false, 0, 0, false
			}
			expectMaxPerPage = false
			continue
		}
		switch arg {
		case "--json", "-j":
			jsonOutput = true
		case "--output-dir", "-o":
			expectOutputDir = true
		case "--workers", "-w":
			expectWorkers = true
		case "--max-per-page", "-m":
			expectMaxPerPage = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintln(c.stderr, usageExtractFigures)
				return nil, "", false, 0, 0, false
			}
			itemKeys = append(itemKeys, arg)
		}
	}

	if len(itemKeys) == 0 {
		fmt.Fprintln(c.stderr, usageExtractFigures)
		return nil, "", false, 0, 0, false
	}
	return itemKeys, outputDir, jsonOutput, workers, maxPerPage, true
}
