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
	usageExtractText = `usage: zot extract-text <item-key>|--all [--json] [--output-dir DIR] [--pages RANGE] [--max-chars N] [--grep TEXT] [--attachment KEY]

What: Extract plain text from a PDF attachment. Returns the concatenated text
content of all pages. Results are cached to disk (keyed by item key + file
mtime), so repeat calls are near-instant.

Output controls:
  --output-dir DIR Write Markdown files to DIR instead of printing text.
  --all            Extract all local items with PDF attachments to Markdown files.
  --pages RANGE      Return only selected PDF pages, e.g. 3 or 2,5-7.
  --max-chars N      Return at most N characters of text (applies per field).
  --grep TEXT        Return only lines containing TEXT (case-insensitive).
  --attachment KEY   Return text for one attachment key.

Modes:
  local/hybrid    Read PDF from local Zotero storage. Requires PyMuPDF.
  remote          Server reads PDF and returns text.
  web             Not supported (no local PDF to read).

Examples:
  zot extract-text ABCD --json
  zot extract-text ABCD --json --pages 3-8
  zot extract-text ABCD --json --max-chars 12000
  zot extract-text ABCD --json --grep methods --attachment ATT123
  zot extract-text ABCD -o ./markdown
  zot extract-text --all -o ./markdown --json

Notes:
  - Requires PyMuPDF; install via 'zot init --pdf' or 'pip install pymupdf'.
  - For snippet-level search use 'zot find ... --fulltext --snippet N'.
  - See also: extract-figures, find --has-pdf, annotations.`

	usageExtractFigures = `usage: zot extract-figures <item-key> [...]|--all [--output-dir DIR] [--json] [--workers N]

What: Extract scientific figures (charts, plots, diagrams) from PDF attachments
as PNG files. Filters cover pages, logos, and author headshots by default.

Options:
  --all                Extract all local items with PDF attachments.
  --output-dir DIR      Where to write PNGs. Default: {ZOT_DATA_DIR}/.zotero_cli/figures
                        (auto-created). Override with this flag.
  --workers N           Parallel workers. Default: CPU count (min 2, max 8).
  --json                Return JSON {key, page, file, ...} instead of writing.

Advanced:
  --max-per-page N      Stop after N figures per page to bound output (default 25).

Modes:
  local/hybrid    Reads from local Zotero storage. Requires PyMuPDF.
  remote          Server extracts via PyMuPDF (same backend).
  web             Not supported.

Examples:
  zot extract-figures ABCD --json
  zot extract-figures ABC1 ABC2 -o ./figs --workers 8 --json
  zot extract-figures --all -o ./figs --workers 8 --json

Notes:
  - Results are cached on disk; rerun skips already-extracted pages.
  - Multi-item runs sort by page count (longest first) for better parallelism.
  - Requires PyMuPDF. See 'zot init --pdf'.
  - See also: extract-text, annotations, open <key> (view in Zotero).`
)

type extractTextArgs struct {
	ItemKey       string
	All           bool
	JSONOutput    bool
	FileOutput    bool
	OutputDir     string
	MaxChars      int
	Grep          string
	AttachmentKey string
	PagesRaw      string
	PageRanges    []pageRange
}

type pageRange struct {
	Start int
	End   int
}

type textFieldView struct {
	Text          string
	Total         int
	ReturnedChars int
	Truncated     bool
}

type markdownTextResult struct {
	ItemKey     string `json:"item_key"`
	Title       string `json:"title,omitempty"`
	File        string `json:"file,omitempty"`
	Attachments int    `json:"attachments,omitempty"`
	Chars       int    `json:"chars,omitempty"`
	Error       string `json:"error,omitempty"`
}

type extractFiguresArgs struct {
	ItemKeys   []string
	All        bool
	OutputDir  string
	JSONOutput bool
	Workers    int
	MaxPerPage int
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

	if parsed.FileOutput {
		return c.runExtractTextMarkdown(reader, cfg, parsed)
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
		if parsed.PagesRaw != "" {
			pageReader, ok := reader.(attachmentPageTextReader)
			if !ok {
				return c.printErr(fmt.Errorf("extract-text --pages requires page-aware full-text extraction support"))
			}
			pageResult, err := pageReader.ExtractItemAttachmentPageTexts(context.Background(), item)
			if err != nil {
				return c.printErr(err)
			}
			readMeta := c.consumeReaderReadMetadata(reader)
			return c.writeExtractTextPagesJSON(item, pageResult, parsed, readMeta)
		}
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
	if parsed.PagesRaw != "" {
		return c.printErr(fmt.Errorf("--pages requires --json"))
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

func (c *CLI) runExtractTextMarkdown(reader backend.Reader, cfg config.Config, args extractTextArgs) int {
	outputDir := args.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(cfg.DataDir, ".zotero_cli", "markdown")
	}
	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return c.printErr(err)
	}
	if err := os.MkdirAll(absOutDir, 0o755); err != nil {
		return c.printErr(err)
	}

	ctx := context.Background()
	if args.All {
		if cfg.Mode == "remote" {
			return c.printErr(fmt.Errorf("extract-text --all is supported in local/hybrid mode"))
		}
		items, err := reader.FindItems(ctx, backend.FindOptions{All: true, Full: true, HasPDF: true})
		if err != nil {
			return c.printErr(err)
		}
		items = filterDefaultFindItems(items, backend.FindOptions{All: true, Full: true, HasPDF: true})
		return c.writeMarkdownForItems(ctx, reader, items, absOutDir, args)
	}

	item, err := reader.GetItem(ctx, args.ItemKey)
	if err != nil {
		return c.printErr(err)
	}
	results := []markdownTextResult{c.writeMarkdownForItem(ctx, reader, item, absOutDir, args)}
	return c.outputMarkdownTextResults(results, args.JSONOutput)
}

func (c *CLI) writeMarkdownForItems(ctx context.Context, reader backend.Reader, items []domain.Item, outputDir string, args extractTextArgs) int {
	results := make([]markdownTextResult, 0, len(items))
	for _, item := range items {
		results = append(results, c.writeMarkdownForItem(ctx, reader, item, outputDir, args))
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ItemKey < results[j].ItemKey
	})
	return c.outputMarkdownTextResults(results, args.JSONOutput)
}

func (c *CLI) writeMarkdownForItem(ctx context.Context, reader backend.Reader, item domain.Item, outputDir string, args extractTextArgs) markdownTextResult {
	result := markdownTextResult{ItemKey: item.Key, Title: item.Title}
	pageReader, pageAware := reader.(attachmentPageTextReader)
	if args.PagesRaw != "" {
		if !pageAware {
			result.Error = "extract-text --pages requires page-aware full-text extraction support"
			return result
		}
		pageResult, err := pageReader.ExtractItemAttachmentPageTexts(ctx, item)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		filtered := filterItemPageTextResult(pageResult, args)
		if args.AttachmentKey != "" && len(filtered.Attachments) == 0 {
			result.Error = fmt.Sprintf("attachment %s not found on item %s", args.AttachmentKey, item.Key)
			return result
		}
		content := markdownForPageTextItem(item, filtered, args)
		return writeMarkdownTextFile(outputDir, item, content, len(filtered.Attachments), result)
	}

	attachmentReader, ok := reader.(attachmentTextReader)
	if !ok {
		result.Error = "extract-text requires local full-text extraction support"
		return result
	}
	fullResult, err := attachmentReader.ExtractItemAttachmentTexts(ctx, item)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	filtered := filterItemFullTextResult(fullResult, args)
	if args.AttachmentKey != "" && len(filtered.Attachments) == 0 {
		result.Error = fmt.Sprintf("attachment %s not found on item %s", args.AttachmentKey, item.Key)
		return result
	}
	content := markdownForFullTextItem(item, filtered, args)
	return writeMarkdownTextFile(outputDir, item, content, len(filtered.Attachments), result)
}

func writeMarkdownTextFile(outputDir string, item domain.Item, content string, attachments int, result markdownTextResult) markdownTextResult {
	filename := markdownFilename(item)
	path := filepath.Join(outputDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		result.Error = err.Error()
		return result
	}
	result.File = path
	result.Attachments = attachments
	result.Chars = len([]rune(content))
	return result
}

func markdownForFullTextItem(item domain.Item, result backend.ItemFullTextResult, args extractTextArgs) string {
	var b strings.Builder
	writeMarkdownItemHeader(&b, item)
	for _, attachment := range result.Attachments {
		text := formatExtractedText(attachment.Text, args).Text
		if strings.TrimSpace(text) == "" {
			continue
		}
		writeMarkdownAttachmentHeader(&b, attachment.Attachment)
		b.WriteString(text)
		b.WriteString("\n")
	}
	return b.String()
}

func markdownForPageTextItem(item domain.Item, result backend.ItemPageTextResult, args extractTextArgs) string {
	var b strings.Builder
	writeMarkdownItemHeader(&b, item)
	for _, attachment := range result.Attachments {
		writeMarkdownAttachmentHeader(&b, attachment.Attachment)
		for _, page := range attachment.Pages {
			text := formatExtractedText(page.Text, args).Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			fmt.Fprintf(&b, "### Page %d\n\n", page.Page)
			b.WriteString(text)
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func writeMarkdownItemHeader(b *strings.Builder, item domain.Item) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = item.Key
	}
	fmt.Fprintf(b, "# %s\n\n", title)
	fmt.Fprintf(b, "- Key: `%s`\n", item.Key)
	if strings.TrimSpace(item.Date) != "" {
		fmt.Fprintf(b, "- Date: %s\n", strings.TrimSpace(item.Date))
	}
	if strings.TrimSpace(item.DOI) != "" {
		fmt.Fprintf(b, "- DOI: %s\n", strings.TrimSpace(item.DOI))
	}
	if strings.TrimSpace(item.URL) != "" {
		fmt.Fprintf(b, "- URL: %s\n", strings.TrimSpace(item.URL))
	}
	b.WriteString("\n")
}

func writeMarkdownAttachmentHeader(b *strings.Builder, attachment domain.Attachment) {
	label := firstNonEmptyPDFString(attachment.Title, attachment.Filename, attachment.Key)
	fmt.Fprintf(b, "## %s\n\n", label)
	fmt.Fprintf(b, "- Attachment key: `%s`\n", attachment.Key)
	if strings.TrimSpace(attachment.Filename) != "" {
		fmt.Fprintf(b, "- Filename: %s\n", strings.TrimSpace(attachment.Filename))
	}
	b.WriteString("\n")
}

func (c *CLI) outputMarkdownTextResults(results []markdownTextResult, jsonOutput bool) int {
	errors := make([]string, 0)
	written := 0
	for _, result := range results {
		if result.Error != "" {
			errors = append(errors, fmt.Sprintf("%s: %s", result.ItemKey, result.Error))
			continue
		}
		written++
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      len(errors) == 0,
			Command: "extract-text",
			Data:    results,
			Meta: map[string]any{
				"format":      "markdown",
				"total_items": len(results),
				"written":     written,
				"errors":      errors,
			},
		})
	}
	for _, result := range results {
		if result.Error != "" {
			fmt.Fprintf(c.stderr, "[%s] error: %s\n", result.ItemKey, result.Error)
			continue
		}
		fmt.Fprintf(c.stdout, "[%s] wrote %s\n", result.ItemKey, result.File)
	}
	if len(errors) > 0 {
		return ExitError
	}
	return ExitOK
}

func defaultPDFWorkers() int {
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 2 {
		workers = 2
	}
	return workers
}

func markdownFilename(item domain.Item) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "untitled"
	}
	title = sanitizeMarkdownFilename(title)
	if len([]rune(title)) > 90 {
		title = string([]rune(title)[:90])
	}
	return sanitizeMarkdownFilename(strings.TrimSpace(item.Key) + "-" + title + ".md")
}

func sanitizeMarkdownFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "untitled"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|', '\r', '\n', '\t':
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		default:
			b.WriteRune(r)
			lastUnderscore = false
		}
	}
	cleaned := strings.Trim(b.String(), " ._")
	if cleaned == "" {
		return "untitled"
	}
	return cleaned
}

func firstNonEmptyPDFString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func filterItemFullTextResult(result backend.ItemFullTextResult, args extractTextArgs) backend.ItemFullTextResult {
	if args.AttachmentKey == "" {
		return result
	}
	filtered := backend.ItemFullTextResult{}
	for _, attachment := range result.Attachments {
		if !strings.EqualFold(attachment.Attachment.Key, args.AttachmentKey) {
			continue
		}
		filtered.Attachments = append(filtered.Attachments, attachment)
		filtered.Text = attachment.Text
		filtered.PrimaryAttachmentKey = attachment.Attachment.Key
	}
	return filtered
}

func (c *CLI) parseExtractTextArgs(args []string) (extractTextArgs, bool) {
	parsed := extractTextArgs{}

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "--all":
			parsed.All = true
		case "--json":
			parsed.JSONOutput = true
		case "--output-dir", "-o":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			i++
			parsed.OutputDir = strings.TrimSpace(args[i])
			if parsed.OutputDir == "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
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
		case "--pages":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			i++
			ranges, err := parsePageRanges(args[i])
			if err != nil {
				fmt.Fprintln(c.stderr, "error:", err)
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			parsed.PagesRaw = strings.TrimSpace(args[i])
			parsed.PageRanges = ranges
		default:
			if strings.HasPrefix(arg, "--") || parsed.ItemKey != "" {
				fmt.Fprintln(c.stderr, usageExtractText)
				return extractTextArgs{}, false
			}
			parsed.ItemKey = arg
		}
	}

	if parsed.All && strings.TrimSpace(parsed.ItemKey) != "" {
		fmt.Fprintln(c.stderr, usageExtractText)
		return extractTextArgs{}, false
	}
	if parsed.All || parsed.OutputDir != "" {
		parsed.FileOutput = true
	}
	if !parsed.All && strings.TrimSpace(parsed.ItemKey) == "" {
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
	if args.PagesRaw != "" {
		filters["pages"] = args.PagesRaw
	}
	if len(filters) > 0 {
		meta["filters"] = filters
	}
}

func (c *CLI) writeExtractTextPagesJSON(item domain.Item, result backend.ItemPageTextResult, args extractTextArgs, readMeta backend.ReadMetadata) int {
	filtered := filterItemPageTextResult(result, args)
	if args.AttachmentKey != "" && len(filtered.Attachments) == 0 {
		return c.printErr(fmt.Errorf("attachment %s not found on item %s", args.AttachmentKey, item.Key))
	}
	dataTextView := formatExtractedText(filtered.Text, args)
	meta := map[string]any{
		"total":          dataTextView.Total,
		"returned_chars": dataTextView.ReturnedChars,
	}
	if dataTextView.Truncated {
		meta["truncated"] = true
	}
	appendExtractTextFilterMeta(meta, args)
	returnedPages := returnedPageNumbers(filtered.Attachments)
	if len(returnedPages) > 0 {
		meta["returned_pages"] = returnedPages
	}
	c.appendExplicitReadMetadata(meta, readMeta)

	attachments := make([]map[string]any, 0, len(filtered.Attachments))
	for _, attachment := range filtered.Attachments {
		text := joinCLIPageTexts(attachment.Pages)
		textView := formatExtractedText(text, args)
		entry := map[string]any{
			"attachment_key": attachment.Attachment.Key,
			"text":           textView.Text,
			"total":          textView.Total,
			"returned_chars": textView.ReturnedChars,
			"pages":          pageTextsToJSON(attachment.Pages),
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
	if filtered.PrimaryAttachmentKey != "" {
		data["primary_attachment_key"] = filtered.PrimaryAttachmentKey
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

func parsePageRanges(value string) ([]pageRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("missing value for --pages")
	}
	parts := strings.Split(value, ",")
	ranges := make([]pageRange, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid value for --pages")
		}
		if strings.Contains(part, "-") {
			ends := strings.Split(part, "-")
			if len(ends) != 2 {
				return nil, fmt.Errorf("invalid page range %q", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(ends[0]))
			if err != nil || start <= 0 {
				return nil, fmt.Errorf("invalid page range %q", part)
			}
			end, err := strconv.Atoi(strings.TrimSpace(ends[1]))
			if err != nil || end <= 0 || end < start {
				return nil, fmt.Errorf("invalid page range %q", part)
			}
			ranges = append(ranges, pageRange{Start: start, End: end})
			continue
		}
		page, err := strconv.Atoi(part)
		if err != nil || page <= 0 {
			return nil, fmt.Errorf("invalid page %q", part)
		}
		ranges = append(ranges, pageRange{Start: page, End: page})
	}
	return ranges, nil
}

func pageInRanges(page int, ranges []pageRange) bool {
	if len(ranges) == 0 {
		return true
	}
	for _, r := range ranges {
		if page >= r.Start && page <= r.End {
			return true
		}
	}
	return false
}

func filterItemPageTextResult(result backend.ItemPageTextResult, args extractTextArgs) backend.ItemPageTextResult {
	filtered := backend.ItemPageTextResult{PrimaryAttachmentKey: result.PrimaryAttachmentKey}
	textParts := []string{}
	for _, attachment := range result.Attachments {
		if args.AttachmentKey != "" && !strings.EqualFold(attachment.Attachment.Key, args.AttachmentKey) {
			continue
		}
		pages := make([]backend.PageText, 0, len(attachment.Pages))
		for _, page := range attachment.Pages {
			if pageInRanges(page.Page, args.PageRanges) {
				pages = append(pages, page)
			}
		}
		if len(pages) == 0 {
			continue
		}
		attachment.Pages = pages
		filtered.Attachments = append(filtered.Attachments, attachment)
		textParts = append(textParts, joinCLIPageTexts(pages))
		if args.AttachmentKey != "" {
			filtered.PrimaryAttachmentKey = attachment.Attachment.Key
		}
	}
	filtered.Text = strings.Join(textParts, "\n")
	if filtered.PrimaryAttachmentKey == "" && len(filtered.Attachments) > 0 {
		filtered.PrimaryAttachmentKey = filtered.Attachments[0].Attachment.Key
	}
	return filtered
}

func joinCLIPageTexts(pages []backend.PageText) string {
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		if strings.TrimSpace(page.Text) != "" {
			parts = append(parts, page.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func pageTextsToJSON(pages []backend.PageText) []map[string]any {
	out := make([]map[string]any, 0, len(pages))
	for _, page := range pages {
		out = append(out, map[string]any{
			"page": page.Page,
			"text": page.Text,
		})
	}
	return out
}

func returnedPageNumbers(attachments []backend.AttachmentPageText) []int {
	seen := map[int]struct{}{}
	for _, attachment := range attachments {
		for _, page := range attachment.Pages {
			seen[page.Page] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for page := range seen {
		out = append(out, page)
	}
	sort.Ints(out)
	return out
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
	pageNums := make([]int, 0, len(pages))
	for page := range pages {
		pageNums = append(pageNums, page)
	}
	sort.Ints(pageNums)
	for _, page := range pageNums {
		pg := pages[page]
		if len(pg.figs) <= maxPerPage {
			sort.Slice(pg.figs, func(i, j int) bool {
				return pg.figs[i].ID < pg.figs[j].ID
			})
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

	parsed, ok := c.parseExtractFiguresArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	// Remote mode: delegate to server API
	if cfg.Mode == "remote" {
		if parsed.All {
			return c.printErr(fmt.Errorf("extract-figures --all is supported in local/hybrid mode"))
		}
		return c.runExtractFiguresRemote(cfg, parsed.ItemKeys, parsed.OutputDir, parsed.JSONOutput)
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
	outputDir := parsed.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(cfg.DataDir, ".zotero_cli", "figures")
	}
	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return c.printErr(err)
	}

	// Default workers: CPU count, min 2, max 8
	workers := parsed.Workers
	if workers <= 0 {
		workers = defaultPDFWorkers()
	}
	maxPerPage := parsed.MaxPerPage

	ctx := context.Background()

	// Single item: run directly (no goroutine overhead)
	if !parsed.All && len(parsed.ItemKeys) == 1 {
		item, err := localReader.GetItem(ctx, parsed.ItemKeys[0])
		if err != nil {
			return c.printErr(err)
		}
		res, err := figExtractor.ExtractFigures(ctx, item, absOutDir)
		if err != nil {
			res.Error = err.Error()
		}
		capFiguresPerPage(&res, absOutDir, maxPerPage)
		return c.outputFiguresResults([]figureTaskResult{{itemKey: parsed.ItemKeys[0], result: res, err: err}}, parsed.JSONOutput)
	}

	// Multiple items: pre-fetch → filter PDF items → sort by page count (LPT) → parallel
	// Phase 1: pre-resolve all items and filter out those without PDFs
	type preloadResult struct {
		key  string
		item domain.Item
		err  error
	}
	var preloads []preloadResult
	if parsed.All {
		items, err := localReader.FindItems(ctx, backend.FindOptions{All: true, Full: true, HasPDF: true})
		if err != nil {
			return c.printErr(err)
		}
		items = filterDefaultFindItems(items, backend.FindOptions{All: true, Full: true, HasPDF: true})
		preloads = make([]preloadResult, 0, len(items))
		for _, item := range items {
			preloads = append(preloads, preloadResult{key: item.Key, item: item})
		}
	} else {
		preloads = make([]preloadResult, len(parsed.ItemKeys))
		for i, key := range parsed.ItemKeys {
			item, err := localReader.GetItem(ctx, key)
			preloads[i] = preloadResult{key: key, item: item, err: err}
		}
	}

	// Phase 2: build job list — only items with resolvable PDF attachments
	jobs := make([]pdfJob, 0, len(preloads))
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

	return c.outputFiguresResults(results, parsed.JSONOutput)
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

func (c *CLI) parseExtractFiguresArgs(args []string) (extractFiguresArgs, bool) {
	parsed := extractFiguresArgs{MaxPerPage: 25}
	expectOutputDir := false
	expectWorkers := false
	expectMaxPerPage := false

	for _, arg := range args {
		if expectOutputDir {
			parsed.OutputDir = arg
			expectOutputDir = false
			continue
		}
		if expectWorkers {
			_, err := fmt.Sscanf(arg, "%d", &parsed.Workers)
			if err != nil || parsed.Workers <= 0 {
				fmt.Fprintf(c.stderr, "%s\ninvalid --workers value: %s\n", usageExtractFigures, arg)
				return extractFiguresArgs{}, false
			}
			expectWorkers = false
			continue
		}
		if expectMaxPerPage {
			_, err := fmt.Sscanf(arg, "%d", &parsed.MaxPerPage)
			if err != nil || parsed.MaxPerPage < 1 {
				fmt.Fprintf(c.stderr, "%s\ninvalid --max-per-page value: %s\n", usageExtractFigures, arg)
				return extractFiguresArgs{}, false
			}
			expectMaxPerPage = false
			continue
		}
		switch arg {
		case "--all":
			parsed.All = true
		case "--json", "-j":
			parsed.JSONOutput = true
		case "--output-dir", "-o":
			expectOutputDir = true
		case "--workers", "-w":
			expectWorkers = true
		case "--max-per-page", "-m":
			expectMaxPerPage = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintln(c.stderr, usageExtractFigures)
				return extractFiguresArgs{}, false
			}
			parsed.ItemKeys = append(parsed.ItemKeys, arg)
		}
	}

	if expectOutputDir || expectWorkers || expectMaxPerPage {
		fmt.Fprintln(c.stderr, usageExtractFigures)
		return extractFiguresArgs{}, false
	}
	if parsed.All && len(parsed.ItemKeys) > 0 {
		fmt.Fprintln(c.stderr, usageExtractFigures)
		return extractFiguresArgs{}, false
	}
	if !parsed.All && len(parsed.ItemKeys) == 0 {
		fmt.Fprintln(c.stderr, usageExtractFigures)
		return extractFiguresArgs{}, false
	}
	return parsed, true
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
