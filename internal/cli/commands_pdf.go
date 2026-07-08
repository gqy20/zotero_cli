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
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

const (
	usageExtractText = `usage: zot extract-text <item-key> [--json] [--max-chars N] [--grep TEXT] [--attachment KEY]

What: Extract plain text from a PDF attachment. Returns the concatenated text
content of all pages. Results are cached to disk (keyed by item key + file
mtime), so repeat calls are near-instant.

Output controls:
  --max-chars N      Return at most N characters of text (applies per field).
  --grep TEXT        Return only lines containing TEXT (case-insensitive).
  --attachment KEY   Return text for one attachment key.

Modes:
  local/hybrid    Read PDF from local Zotero storage. Requires PyMuPDF.
  remote          Server reads PDF and returns text.
  web             Not supported (no local PDF to read).

Examples:
  zot extract-text ABCD --json
  zot extract-text ABCD --json --max-chars 12000
  zot extract-text ABCD --json --grep methods --attachment ATT123

Notes:
  - Requires PyMuPDF; install via 'zot init --pdf' or 'pip install pymupdf'.
  - For snippet-level search use 'zot find ... --fulltext --snippet N'.
  - See also: extract-figures, find --has-pdf, annotations.`

	usageExtractFigures = `usage: zot extract-figures <item-key> [...] [--output-dir DIR] [--json] [--workers N] [--max-per-page N]

What: Extract scientific figures (charts, plots, diagrams) from PDF attachments
as PNG files. Filters cover pages, logos, and author headshots by default.

Options:
  --output-dir DIR      Where to write PNGs. Default: {ZOT_DATA_DIR}/.zotero_cli/figures
                        (auto-created). Override with this flag.
  --workers N           Parallel workers. Default: CPU count (min 2, max 8).
  --max-per-page N      Stop after N figures per page to bound output (default 25).
  --json                Return JSON {key, page, file, ...} instead of writing.

Modes:
  local/hybrid    Reads from local Zotero storage. Requires PyMuPDF.
  remote          Server extracts via PyMuPDF (same backend).
  web             Not supported.

Examples:
  zot extract-figures ABCD --json
  zot extract-figures ABC1 ABC2 -o ./figs --workers 8 --json
  zot extract-figures ABCD --max-per-page 5 --json

Notes:
  - Results are cached on disk; rerun skips already-extracted pages.
  - Multi-item runs sort by page count (longest first) for better parallelism.
  - Requires PyMuPDF. See 'zot init --pdf'.
  - See also: extract-text, annotations, open <key> (view in Zotero).`
)

type extractTextArgs struct {
	ItemKey       string
	JSONOutput    bool
	MaxChars      int
	Grep          string
	AttachmentKey string
}

type textFieldView struct {
	Text          string
	Total         int
	ReturnedChars int
	Truncated     bool
}

func (c *CLI) runExtractText(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageExtractText)
	}

	parsed, ok := c.parseExtractTextArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	var (
		reader backend.Reader
		err    error
	)
	if cfg.Mode == "remote" {
		reader, err = c.backendNewReader(cfg, nil)
	} else {
		reader, err = c.newLocalReader(cfg)
	}
	if err != nil {
		return c.printErr(err)
	}

	item, err := reader.GetItem(context.Background(), parsed.ItemKey)
	if err != nil {
		return c.printErr(err)
	}
	if parsed.JSONOutput {
		var (
			result backend.ItemFullTextResult
			err    error
		)
		if attachmentReader, ok := reader.(attachmentTextReader); ok {
			result, err = attachmentReader.ExtractItemAttachmentTexts(context.Background(), item)
		} else {
			textReader, ok := reader.(fullTextReader)
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

		readMeta := c.consumeReaderReadMetadata(reader)
		dataText := result.Text
		if parsed.AttachmentKey != "" {
			selected := make([]string, 0, 1)
			for _, attachment := range result.Attachments {
				if strings.EqualFold(attachment.Attachment.Key, parsed.AttachmentKey) {
					selected = append(selected, attachment.Text)
				}
			}
			if len(selected) == 0 {
				return c.printErr(fmt.Errorf("attachment %s not found on item %s", parsed.AttachmentKey, item.Key))
			}
			dataText = strings.Join(selected, "\n")
		}
		dataTextView := formatExtractedText(dataText, parsed)
		meta := map[string]any{
			"total":          dataTextView.Total,
			"returned_chars": dataTextView.ReturnedChars,
		}
		if dataTextView.Truncated {
			meta["truncated"] = true
		}
		appendExtractTextFilterMeta(meta, parsed)
		c.appendExplicitReadMetadata(meta, readMeta)
		attachments := make([]map[string]any, 0, len(result.Attachments))
		for _, attachment := range result.Attachments {
			if parsed.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, parsed.AttachmentKey) {
				continue
			}
			textView := formatExtractedText(attachment.Text, parsed)
			entry := map[string]any{
				"attachment_key": attachment.Attachment.Key,
				"text":           textView.Text,
				"total":          textView.Total,
				"returned_chars": textView.ReturnedChars,
			}
			if textView.Truncated {
				entry["truncated"] = true
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
			"text":     dataTextView.Text,
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

	if parsed.AttachmentKey != "" {
		return c.printErr(fmt.Errorf("--attachment requires --json"))
	}
	textReader, ok := reader.(fullTextReader)
	if !ok {
		return c.printErr(fmt.Errorf("extract-text requires local full-text extraction support"))
	}
	text, err := textReader.ExtractItemFullText(context.Background(), item)
	if err != nil {
		return c.printErr(err)
	}
	readMeta := c.consumeReaderReadMetadata(reader)
	c.warnIfSnapshotRead(readMeta)
	text = formatExtractedText(text, parsed).Text
	fmt.Fprintln(c.stdout, text)
	return 0
}

func (c *CLI) parseExtractTextArgs(args []string) (extractTextArgs, bool) {
	parsed := extractTextArgs{}

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "--json":
			parsed.JSONOutput = true
		case "--max-chars":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			i++
			maxChars, err := strconv.Atoi(args[i])
			if err != nil || maxChars <= 0 {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			parsed.MaxChars = maxChars
		case "--grep":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			i++
			parsed.Grep = strings.TrimSpace(args[i])
			if parsed.Grep == "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
		case "--attachment":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			i++
			parsed.AttachmentKey = strings.TrimSpace(args[i])
			if parsed.AttachmentKey == "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
		default:
			if strings.HasPrefix(arg, "--") || parsed.ItemKey != "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			parsed.ItemKey = arg
		}
	}

	if strings.TrimSpace(parsed.ItemKey) == "" {
		fmt.Fprintln(c.stderr, usageExtractText)
		return extractTextArgs{}, false
	}
	return parsed, true
}

func appendExtractTextFilterMeta(meta map[string]any, args extractTextArgs) {
	filters := map[string]any{}
	if args.MaxChars > 0 {
		filters["max_chars"] = args.MaxChars
	}
	if args.Grep != "" {
		filters["grep"] = args.Grep
	}
	if args.AttachmentKey != "" {
		filters["attachment_key"] = args.AttachmentKey
	}
	if len(filters) > 0 {
		meta["filters"] = filters
	}
}

func formatExtractedText(text string, args extractTextArgs) textFieldView {
	originalTotal := len([]rune(text))
	if args.Grep != "" {
		text = grepTextLines(text, args.Grep)
	}
	view := textFieldView{Text: text, Total: originalTotal}
	if args.MaxChars > 0 {
		runes := []rune(view.Text)
		if len(runes) > args.MaxChars {
			view.Text = string(runes[:args.MaxChars])
			view.Truncated = true
		}
	}
	view.ReturnedChars = len([]rune(view.Text))
	return view
}

func grepTextLines(text string, needle string) string {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	matches := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), needle) {
			matches = append(matches, line)
		}
	}
	return strings.Join(matches, "\n")
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

	// Remote mode: delegate to server API
	if cfg.Mode == "remote" {
		return c.runExtractFiguresRemote(cfg, itemKeys, outputDir, jsonOutput)
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

func (c *CLI) runExtractFiguresRemote(cfg config.Config, itemKeys []string, outputDir string, jsonOutput bool) int {
	if outputDir == "" {
		outputDir = filepath.Join(".", "figures")
	}
	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return c.printErr(err)
	}

	reader, err := c.backendNewReader(cfg, nil)
	if err != nil {
		return c.printErr(err)
	}
	remoteReader, ok := reader.(*backend.RemoteReader)
	if !ok {
		return c.printErr(fmt.Errorf("extract-figures in remote mode requires a remote reader"))
	}

	ctx := context.Background()
	var results []figureTaskResult
	for _, key := range itemKeys {
		res, extractErr := remoteReader.ExtractFigures(ctx, key, absOutDir)
		results = append(results, figureTaskResult{itemKey: key, result: res, err: extractErr})
	}

	return c.outputFiguresResults(results, jsonOutput)
}
