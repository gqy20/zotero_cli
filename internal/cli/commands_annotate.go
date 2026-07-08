package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
)

const usageAnnotate = `usage: zot annotate <item-key> (--text TEXT | --page N (--rect x0,y0,x1,y2 | --point x,y)) [--color COLOR] [--comment TEXT] [--type TYPE] [--dry-run] [--clear] [--author AUTHOR] [--json]
       zot annotate [<default-item-key>] --from-file PATH [--dry-run] [--json]

What: Add highlights/underlines/notes to a PDF. Three modes:

  Mode 1     --text TEXT
             Plain text search. Zotero locates the first match on the
             page (auto-resolved). Works without geometry knowledge.
             zot annotate KEY --text "GATK" --color yellow

  Mode 1.5   --text TEXT --page N       (recommended)
             Text + page constraint. Faster + more precise than Mode 1.
             zot annotate KEY --page 4 --text "GATK" --color red

  Mode 2     --page N --rect/--point
             Pure geometry. No text match. For image-only or exact regions.
             zot annotate KEY --page 4 --rect 100,200,400,250 --color blue

Options:
  --color       yellow | red | blue | green | magenta | cyan | orange
  --type        highlight (default) | underline | note | image
  --comment     Sticky note text (when --type=note)
  --author      Annotation author (defaults to current Zotero user)
  --dry-run     Preview matched pages/rectangles without writing the PDF.
  --from-file   Batch annotations from a JSON array. Each entry accepts
                item_key, text, page, rect, point, color, comment, type,
                and dry_run. item_key may be omitted when <default-item-key>
                is provided on the command line.
  --clear       Remove existing annotations. By default clears both API and
                local DB; the DB layer is only removable when Zotero is closed.
                Combine with --type / --page to scope.

Examples:
  zot annotate ABCD --page 4 --text "GATK" --color yellow --json
  zot annotate ABCD --page 4 --text "GATK" --dry-run --json
  zot annotate ABCD --from-file annotations.json --dry-run --json
  zot annotate ABCD --page 1 --rect 50,100,300,150 --color red --json
  zot annotate ABCD --type note --page 5 --comment "TODO" --json
  zot annotate ABCD --clear --page 4 --type highlight

Notes:
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - In local/hybrid mode annotations are written to the local SQLite when
    Zotero is not running (~50ms), else via the Web API (~2s).
  - See also: annotations (read back), extract-text (read PDF body).`

type annotateParsedArgs struct {
	itemKey      string
	req          backend.AnnotateRequest
	clearMode    bool
	authorFilter string
	jsonOutput   bool
	fromFile     string
}

type annotateBatchEntry struct {
	ItemKey string    `json:"item_key,omitempty"`
	Text    string    `json:"text,omitempty"`
	Color   string    `json:"color,omitempty"`
	Comment string    `json:"comment,omitempty"`
	Type    string    `json:"type,omitempty"`
	Page    int       `json:"page,omitempty"`
	Rect    []float64 `json:"rect,omitempty"`
	Point   []float64 `json:"point,omitempty"`
	DryRun  *bool     `json:"dry_run,omitempty"`
}

type annotateBatchResult struct {
	Index         int                     `json:"index"`
	OK            bool                    `json:"ok"`
	ItemKey       string                  `json:"item_key,omitempty"`
	AttachmentKey string                  `json:"attachment_key,omitempty"`
	PDFPath       string                  `json:"pdf_path,omitempty"`
	Matches       []backend.AnnotateMatch `json:"matches,omitempty"`
	TotalMatches  int                     `json:"total_matches,omitempty"`
	DryRun        bool                    `json:"dry_run,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

func (c *CLI) runAnnotate(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageAnnotate)
	}

	parsed, ok := c.parseAnnotateArgs(args)
	if !ok {
		return 2
	}
	if parsed.fromFile != "" {
		return c.runAnnotateBatch(parsed)
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if cfg.Mode != "remote" && !parsed.req.DryRun {
		if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
			return exitCode
		}
	}

	item, err := reader.GetItem(context.Background(), parsed.itemKey)
	if err != nil {
		return c.printErr(err)
	}

	// Handle --clear mode: delete annotations instead of creating
	if parsed.clearMode {
		delReq := backend.DeleteAnnotationsRequest{
			Page:   parsed.req.Page,
			Type:   parsed.req.Type,
			Author: parsed.authorFilter,
		}
		return c.runAnnotationClear(reader, parsed.itemKey, delReq, parsed.jsonOutput, "annotate")
	}

	lr, ok := reader.(itemAnnotator)
	if !ok {
		return c.printErr(fmt.Errorf("annotation writing is not available for the current reader"))
	}

	result, err := lr.AnnotateItem(context.Background(), item, parsed.req)
	if err != nil {
		return c.printErr(err)
	}

	if parsed.jsonOutput {
		data := map[string]any{
			"item_key":       parsed.itemKey,
			"attachment_key": result.AttachmentKey,
			"pdf_path":       result.PDFPath,
			"matches":        result.Matches,
			"total_matches":  len(result.Matches),
			"dry_run":        result.DryRun,
		}
		meta := map[string]any{
			"total_matches": len(result.Matches),
			"dry_run":       result.DryRun,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "annotate",
			Data:    data,
			Meta:    meta,
		})
	}

	if result.DryRun {
		fmt.Fprintf(c.stdout, "[dry-run] Would annotate %s (%s)\n", parsed.itemKey, result.AttachmentKey)
	} else {
		fmt.Fprintf(c.stdout, "Annotated %s (%s)\n", parsed.itemKey, result.AttachmentKey)
	}
	if strings.TrimSpace(result.PDFPath) != "" {
		fmt.Fprintf(c.stdout, "PDF: %s\n", result.PDFPath)
	}
	fmt.Fprintf(c.stdout, "Matches: %d\n\n", len(result.Matches))
	for _, m := range result.Matches {
		fmt.Fprintf(c.stdout, "  Page %d [%s %s]: \"%s\"\n", m.Page, m.Type, m.Color, m.Text)
	}
	return 0
}

func (c *CLI) runAnnotateBatch(parsed annotateParsedArgs) int {
	entries, err := loadAnnotateBatchEntries(parsed.fromFile)
	if err != nil {
		return c.printErr(err)
	}
	if len(entries) == 0 {
		return c.printErr(fmt.Errorf("no annotations in batch file"))
	}

	requests := make([]struct {
		itemKey string
		req     backend.AnnotateRequest
	}, 0, len(entries))
	needsWrite := false
	for i, entry := range entries {
		itemKey, req, err := annotateBatchEntryRequest(parsed, entry)
		if err != nil {
			return c.printErr(fmt.Errorf("annotation %d: %w", i+1, err))
		}
		if !req.DryRun {
			needsWrite = true
		}
		requests = append(requests, struct {
			itemKey string
			req     backend.AnnotateRequest
		}{itemKey: itemKey, req: req})
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}
	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	if cfg.Mode != "remote" && needsWrite {
		if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
			return exitCode
		}
	}

	annotator, ok := reader.(itemAnnotator)
	if !ok {
		return c.printErr(fmt.Errorf("annotation writing is not available for the current reader"))
	}

	ctx := context.Background()
	results := make([]annotateBatchResult, 0, len(requests))
	failed := 0
	for i, op := range requests {
		result := annotateBatchResult{
			Index:   i + 1,
			ItemKey: op.itemKey,
			DryRun:  op.req.DryRun,
		}
		item, err := reader.GetItem(ctx, op.itemKey)
		if err != nil {
			result.Error = err.Error()
			failed++
			results = append(results, result)
			continue
		}
		annotateResult, err := annotator.AnnotateItem(ctx, item, op.req)
		if err != nil {
			result.Error = err.Error()
			failed++
			results = append(results, result)
			continue
		}
		result.OK = true
		result.AttachmentKey = annotateResult.AttachmentKey
		result.PDFPath = annotateResult.PDFPath
		result.Matches = annotateResult.Matches
		result.TotalMatches = len(annotateResult.Matches)
		result.DryRun = annotateResult.DryRun
		results = append(results, result)
	}

	if parsed.jsonOutput {
		meta := map[string]any{
			"total":  len(results),
			"failed": failed,
			"ok":     len(results) - failed,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      failed == 0,
			Command: "annotate",
			Data:    results,
			Meta:    meta,
		})
	}

	for _, result := range results {
		prefix := "annotated"
		if result.DryRun {
			prefix = "[dry-run] would annotate"
		}
		if result.Error != "" {
			fmt.Fprintf(c.stdout, "FAIL #%d %s: %s\n", result.Index, result.ItemKey, result.Error)
			continue
		}
		fmt.Fprintf(c.stdout, "%s #%d %s (%s), matches=%d\n", prefix, result.Index, result.ItemKey, result.AttachmentKey, result.TotalMatches)
	}
	fmt.Fprintf(c.stdout, "\n%d annotations processed (%d ok, %d failed)\n", len(results), len(results)-failed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

func (c *CLI) parseAnnotateArgs(args []string) (annotateParsedArgs, bool) {
	var itemKey string
	req := backend.AnnotateRequest{
		Type:  "highlight",
		Color: "yellow",
	}
	clearMode := false
	authorFilter := ""
	jsonOutput := false
	fromFile := ""
	nextFlag := ""

	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "text":
				req.Text = arg
			case "color":
				req.Color = arg
			case "comment":
				req.Comment = arg
			case "type":
				req.Type = arg
			case "page":
				n, err := strconv.Atoi(arg)
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				req.Page = n
			case "rect":
				parts := strings.Split(arg, ",")
				if len(parts) != 4 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				var rc [4]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return annotateParsedArgs{}, false
					}
					rc[i] = v
				}
				req.Rect = &rc
			case "point":
				parts := strings.Split(arg, ",")
				if len(parts) != 2 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				var pt [2]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return annotateParsedArgs{}, false
					}
					pt[i] = v
				}
				req.Point = &pt
			case "author":
				authorFilter = arg
			case "from-file":
				fromFile = arg
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--json":
			jsonOutput = true
		case "--clear":
			clearMode = true
		case "--dry-run", "-n":
			req.DryRun = true
		case "--text":
			nextFlag = "text"
		case "--color":
			nextFlag = "color"
		case "--comment":
			nextFlag = "comment"
		case "--type":
			nextFlag = "type"
		case "--page":
			nextFlag = "page"
		case "--rect":
			nextFlag = "rect"
		case "--point":
			nextFlag = "point"
		case "--author":
			nextFlag = "author"
		case "--from-file":
			nextFlag = "from-file"
		default:
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				fmt.Fprintln(c.stderr, usageAnnotate)
				return annotateParsedArgs{}, false
			}
			if strings.HasPrefix(arg, "--text=") {
				req.Text = strings.TrimPrefix(arg, "--text=")
			} else if strings.HasPrefix(arg, "--color=") {
				req.Color = strings.TrimPrefix(arg, "--color=")
			} else if strings.HasPrefix(arg, "--comment=") {
				req.Comment = strings.TrimPrefix(arg, "--comment=")
			} else if strings.HasPrefix(arg, "--type=") {
				req.Type = strings.TrimPrefix(arg, "--type=")
			} else if strings.HasPrefix(arg, "--page=") {
				n, err := strconv.Atoi(strings.TrimPrefix(arg, "--page="))
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				req.Page = n
			} else if strings.HasPrefix(arg, "--rect=") {
				parts := strings.Split(strings.TrimPrefix(arg, "--rect="), ",")
				if len(parts) != 4 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				var rc [4]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return annotateParsedArgs{}, false
					}
					rc[i] = v
				}
				req.Rect = &rc
			} else if strings.HasPrefix(arg, "--point=") {
				parts := strings.Split(strings.TrimPrefix(arg, "--point="), ",")
				if len(parts) != 2 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return annotateParsedArgs{}, false
				}
				var pt [2]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return annotateParsedArgs{}, false
					}
					pt[i] = v
				}
				req.Point = &pt
			} else if strings.HasPrefix(arg, "--author=") {
				authorFilter = strings.TrimPrefix(arg, "--author=")
			} else if strings.HasPrefix(arg, "--from-file=") {
				fromFile = strings.TrimPrefix(arg, "--from-file=")
			} else if itemKey != "" {
				fmt.Fprintln(c.stderr, usageAnnotate)
				return annotateParsedArgs{}, false
			} else {
				itemKey = arg
			}
		}
	}

	if nextFlag != "" {
		fmt.Fprintln(c.stderr, usageAnnotate)
		return annotateParsedArgs{}, false
	}

	parsed := annotateParsedArgs{
		itemKey:      itemKey,
		req:          req,
		clearMode:    clearMode,
		authorFilter: authorFilter,
		jsonOutput:   jsonOutput,
		fromFile:     fromFile,
	}
	if fromFile != "" {
		if clearMode {
			fmt.Fprintln(c.stderr, usageAnnotate)
			return annotateParsedArgs{}, false
		}
		return parsed, true
	}

	// In clear mode, only itemKey is required (page/type/author are optional filters)
	if clearMode {
		if itemKey == "" {
			fmt.Fprintln(c.stderr, usageAnnotate)
			return annotateParsedArgs{}, false
		}
		req.Type = "" // reset default "highlight" so clear deletes all types
		parsed.req = req
		parsed.clearMode = true
		return parsed, true
	}

	hasText := req.Text != ""
	hasRect := req.Page > 0 && req.Rect != nil
	hasPoint := req.Page > 0 && req.Point != nil

	if itemKey == "" || (!hasText && !hasRect && !hasPoint) {
		fmt.Fprintln(c.stderr, usageAnnotate)
		return annotateParsedArgs{}, false
	}

	if hasPoint && req.Comment == "" {
		req.Comment = "Note"
	}

	parsed.req = req
	return parsed, true
}

func loadAnnotateBatchEntries(path string) ([]annotateBatchEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errReadFromFile(err)
	}
	var entries []annotateBatchEntry
	if err := json.Unmarshal(content, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w (expected JSON array)", path, err)
	}
	return entries, nil
}

func annotateBatchEntryRequest(parsed annotateParsedArgs, entry annotateBatchEntry) (string, backend.AnnotateRequest, error) {
	itemKey := strings.TrimSpace(entry.ItemKey)
	if itemKey == "" {
		itemKey = strings.TrimSpace(parsed.itemKey)
	}
	if itemKey == "" {
		return "", backend.AnnotateRequest{}, fmt.Errorf("missing item_key")
	}

	req := parsed.req
	if strings.TrimSpace(entry.Text) != "" {
		req.Text = entry.Text
	}
	if strings.TrimSpace(entry.Color) != "" {
		req.Color = entry.Color
	}
	if strings.TrimSpace(entry.Comment) != "" {
		req.Comment = entry.Comment
	}
	if strings.TrimSpace(entry.Type) != "" {
		req.Type = entry.Type
	}
	if entry.Page > 0 {
		req.Page = entry.Page
	}
	if len(entry.Rect) > 0 {
		if len(entry.Rect) != 4 {
			return "", backend.AnnotateRequest{}, fmt.Errorf("rect must have 4 numbers")
		}
		req.Rect = &[4]float64{entry.Rect[0], entry.Rect[1], entry.Rect[2], entry.Rect[3]}
	}
	if len(entry.Point) > 0 {
		if len(entry.Point) != 2 {
			return "", backend.AnnotateRequest{}, fmt.Errorf("point must have 2 numbers")
		}
		req.Point = &[2]float64{entry.Point[0], entry.Point[1]}
	}
	if entry.DryRun != nil {
		req.DryRun = *entry.DryRun
	}
	if parsed.req.DryRun {
		req.DryRun = true
	}
	if req.Type == "" {
		req.Type = "highlight"
	}
	if req.Color == "" {
		req.Color = "yellow"
	}
	if req.Point != nil && req.Comment == "" {
		req.Comment = "Note"
	}
	if err := validateAnnotateRequest(req); err != nil {
		return "", backend.AnnotateRequest{}, err
	}
	return itemKey, req, nil
}

func validateAnnotateRequest(req backend.AnnotateRequest) error {
	hasText := strings.TrimSpace(req.Text) != ""
	hasRect := req.Page > 0 && req.Rect != nil
	hasPoint := req.Page > 0 && req.Point != nil
	if !hasText && !hasRect && !hasPoint {
		return fmt.Errorf("missing annotation target (use text, page+rect, or page+point)")
	}
	if req.Rect != nil && req.Point != nil {
		return fmt.Errorf("cannot combine rect and point")
	}
	if req.Page < 0 {
		return fmt.Errorf("invalid page")
	}
	return nil
}
