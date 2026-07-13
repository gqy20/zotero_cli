package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) addContentCommands(root *cobra.Command, opts *globalOptions) {
	root.AddCommand(c.newFileCommand(opts), c.newPDFCommand(opts), c.newAnnotationCommand(opts))
}

func (c *CLI) pdfService() app.PDFService {
	service := app.NewPDFService()
	service.NewReader = func(cfg config.Config) (backend.Reader, error) {
		if cfg.Mode == "remote" {
			return c.backendNewReader(cfg, nil)
		}
		return c.newLocalReader(cfg)
	}
	return service
}

func (c *CLI) newPDFCommand(opts *globalOptions) *cobra.Command {
	pdf := &cobra.Command{Use: "pdf", Short: "Extract and open PDF content"}
	var textReq app.PDFTextRequest
	textCmd := &cobra.Command{Use: "text [ITEM_KEY...]", Short: "Prepare PDF full-text cache paths or filtered text", Args: cobra.ArbitraryArgs}
	textCmd.Flags().BoolVar(&textReq.All, "all", false, "prepare every item with a PDF")
	textCmd.Flags().StringVarP(&textReq.OutputDir, "output-dir", "o", "", "write Markdown files to this directory")
	textCmd.Flags().StringVar(&textReq.Pages, "pages", "", "page ranges such as 1-3,7")
	textCmd.Flags().IntVar(&textReq.MaxChars, "max-chars", 0, "maximum returned characters")
	textCmd.Flags().StringVar(&textReq.Grep, "grep", "", "return matching text with adjacent context")
	textCmd.Flags().StringVar(&textReq.AttachmentKey, "attachment", "", "extract one attachment")
	textCmd.RunE = func(cmd *cobra.Command, args []string) error {
		textReq.Keys = args
		if textReq.All == (len(args) > 0) {
			return &exitError{code: ExitUsage, err: fmt.Errorf("provide item keys or --all")}
		}
		if textReq.MaxChars < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--max-chars must be non-negative")}
		}
		path := app.CommandPath{Resource: "pdf", Action: "text"}
		return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return c.pdfService().Text(ctx, textReq) })
	}

	var figuresReq app.PDFFiguresRequest
	figuresCmd := &cobra.Command{Use: "figs [ITEM_KEY...]", Short: "Extract figures from PDFs", Args: cobra.ArbitraryArgs}
	figuresCmd.Flags().BoolVar(&figuresReq.All, "all", false, "extract figures from every PDF item")
	figuresCmd.Flags().StringVarP(&figuresReq.OutputDir, "output-dir", "o", "", "figure output directory")
	figuresCmd.Flags().IntVar(&figuresReq.Workers, "workers", 0, "parallel workers")
	figuresCmd.Flags().IntVar(&figuresReq.MaxPerPage, "max-per-page", 0, "keep at most N figures per page")
	figuresCmd.RunE = func(cmd *cobra.Command, args []string) error {
		figuresReq.Keys = args
		if figuresReq.All == (len(args) > 0) {
			return &exitError{code: ExitUsage, err: fmt.Errorf("provide item keys or --all")}
		}
		if figuresReq.Workers < 0 || figuresReq.MaxPerPage < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--workers and --max-per-page must be non-negative")}
		}
		path := app.CommandPath{Resource: "pdf", Action: "figs"}
		return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return c.pdfService().Figures(ctx, figuresReq) })
	}

	var page int
	openCmd := &cobra.Command{Use: "open ITEM_KEY", Short: "Open an item's PDF", Args: cobra.ExactArgs(1)}
	openCmd.Flags().IntVar(&page, "page", 0, "one-based page hint")
	openCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if page < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--page must be non-negative")}
		}
		path := app.CommandPath{Resource: "pdf", Action: "open"}
		return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
			return c.pdfService().Open(ctx, app.PDFOpenRequest{Key: args[0], Page: page})
		})
	}
	pdf.AddCommand(textCmd, figuresCmd, openCmd)
	return pdf
}

func (c *CLI) newFileCommand(opts *globalOptions) *cobra.Command {
	file := &cobra.Command{Use: "file", Short: "Inspect library attachments"}

	var showReq app.FileRequest
	show := &cobra.Command{Use: "show [ATTACHMENT_KEY]", Short: "Preview a spreadsheet attachment", Args: cobra.MaximumNArgs(1)}
	show.Flags().StringVar(&showReq.ItemKey, "item", "", "inspect attachments belonging to one item")
	show.Flags().StringVar(&showReq.Sheet, "sheet", "", "inspect one workbook sheet")
	show.Flags().IntVar(&showReq.Head, "head", 5, "preview non-empty rows per sheet")
	show.Flags().IntVar(&showReq.MaxSheets, "max-sheets", 5, "maximum workbook sheets")
	show.Flags().IntVar(&showReq.MaxColumns, "max-columns", 12, "maximum preview cells per row")
	show.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			showReq.AttachmentKey = args[0]
		}
		if showReq.Head <= 0 || showReq.MaxSheets <= 0 || showReq.MaxColumns <= 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--head, --max-sheets, and --max-columns must be positive")}
		}
		path := app.CommandPath{Resource: "file", Action: "show"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.Files(ctx, showReq)
		})
	}

	var checkReq app.FileRequest
	check := &cobra.Command{Use: "check [ATTACHMENT_KEY]", Short: "Check attachment health", Args: cobra.MaximumNArgs(1)}
	check.Flags().StringVar(&checkReq.ItemKey, "item", "", "inspect attachments belonging to one item")
	check.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			checkReq.AttachmentKey = args[0]
		}
		checkReq.Health = true
		path := app.CommandPath{Resource: "file", Action: "check"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.Files(ctx, checkReq)
		})
	}

	file.AddCommand(show, check)
	return file
}

func (c *CLI) annotationService() app.AnnotationService {
	service := app.NewAnnotationService()
	service.NewReader = func(cfg config.Config) (backend.Reader, error) { return c.backendNewReader(cfg, nil) }
	return service
}

func (c *CLI) runAnnotation(cmd *cobra.Command, opts *globalOptions, path app.CommandPath, run func(context.Context, app.AnnotationService) (app.Result, error)) error {
	err := c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return run(ctx, c.annotationService()) })
	if errors.Is(err, app.ErrCancelled) {
		return &exitError{code: 130, err: app.ErrCancelled}
	}
	return err
}

func (c *CLI) newAnnotationCommand(opts *globalOptions) *cobra.Command {
	ann := &cobra.Command{Use: "ann", Short: "Read and modify PDF annotations"}
	var listFilter app.AnnotationFilter
	list := &cobra.Command{Use: "list ITEM_KEY", Short: "List annotations", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return c.runAnnotation(cmd, opts, app.CommandPath{Resource: "ann", Action: "list"}, func(ctx context.Context, service app.AnnotationService) (app.Result, error) {
			return service.List(ctx, args[0], listFilter)
		})
	}}
	addAnnotationFilterFlags(list, &listFilter)

	var create backend.AnnotateRequest
	var from string
	var rect, point string
	create.Type = "highlight"
	create.Color = "yellow"
	newCmd := &cobra.Command{Use: "new [ITEM_KEY]", Short: "Create annotations", Args: cobra.MaximumNArgs(1)}
	flags := newCmd.Flags()
	flags.StringVar(&create.Text, "text", "", "text to locate")
	flags.StringVar(&create.AttachmentKey, "attachment", "", "target PDF attachment key")
	flags.StringVar(&create.Color, "color", "yellow", "annotation color")
	flags.StringVar(&create.Comment, "comment", "", "annotation comment")
	flags.StringVar(&create.Type, "type", "highlight", "highlight, underline, or note")
	flags.IntVar(&create.Page, "page", 0, "one-based page number")
	flags.StringVar(&rect, "rect", "", "x0,y0,x1,y1 rectangle")
	flags.StringVar(&point, "point", "", "x,y note point")
	flags.BoolVar(&create.DryRun, "dry-run", false, "preview without writing")
	flags.StringVar(&from, "from", "", "read annotation requests from a JSON file")
	newCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if rect != "" {
			values, err := parseFloatTuple(rect, 4)
			if err != nil {
				return &exitError{code: ExitUsage, err: fmt.Errorf("invalid --rect: %w", err)}
			}
			create.Rect = &[4]float64{values[0], values[1], values[2], values[3]}
		}
		if point != "" {
			values, err := parseFloatTuple(point, 2)
			if err != nil {
				return &exitError{code: ExitUsage, err: fmt.Errorf("invalid --point: %w", err)}
			}
			create.Point = &[2]float64{values[0], values[1]}
			if !flags.Changed("type") {
				create.Type = "note"
			}
		}
		defaultKey := ""
		if len(args) == 1 {
			defaultKey = args[0]
		}
		targets, err := annotationTargets(defaultKey, create, from)
		if err != nil {
			return &exitError{code: ExitUsage, err: err}
		}
		return c.runAnnotation(cmd, opts, app.CommandPath{Resource: "ann", Action: "new"}, func(ctx context.Context, service app.AnnotationService) (app.Result, error) {
			return service.Create(ctx, targets)
		})
	}

	var deleteFilter app.AnnotationFilter
	var safety app.SafetyOptions
	var deleteSource string
	deleteCmd := &cobra.Command{Use: "delete ITEM_KEY", Short: "Permanently delete annotations", Args: cobra.ExactArgs(1)}
	addAnnotationFilterFlags(deleteCmd, &deleteFilter)
	deleteCmd.Flags().StringVar(&deleteSource, "source", "", "required annotation source: zotero or pdf")
	deleteCmd.Flags().BoolVar(&safety.DryRun, "dry-run", false, "preview exact deletion candidates")
	deleteCmd.Flags().IntVar(&safety.IfVersion, "if-version", 0, "expected Zotero library version (zotero source)")
	deleteCmd.Flags().BoolVarP(&safety.Yes, "yes", "y", false, "confirm destructive operation")
	deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
		safety.Confirm = c.confirm
		return c.runAnnotation(cmd, opts, app.CommandPath{Resource: "ann", Action: "delete"}, func(ctx context.Context, service app.AnnotationService) (app.Result, error) {
			return service.Delete(ctx, args[0], deleteFilter, deleteSource, safety)
		})
	}
	ann.AddCommand(list, newCmd, deleteCmd)
	return ann
}

func addAnnotationFilterFlags(cmd *cobra.Command, filter *app.AnnotationFilter) {
	cmd.Flags().StringVar(&filter.AttachmentKey, "attachment", "", "target PDF attachment key")
	cmd.Flags().IntVar(&filter.Page, "page", 0, "one-based page number")
	cmd.Flags().StringVar(&filter.Type, "type", "", "annotation type")
	cmd.Flags().StringVar(&filter.Author, "author", "", "annotation author")
}

type annotationFileEntry struct {
	ItemKey       string    `json:"item_key"`
	AttachmentKey string    `json:"attachment_key"`
	Text          string    `json:"text"`
	Color         string    `json:"color"`
	Comment       string    `json:"comment"`
	Type          string    `json:"type"`
	Page          int       `json:"page"`
	Rect          []float64 `json:"rect"`
	Point         []float64 `json:"point"`
	DryRun        *bool     `json:"dry_run"`
}

func annotationTargets(defaultKey string, defaults backend.AnnotateRequest, from string) ([]app.AnnotationTarget, error) {
	if from == "" {
		if defaultKey == "" {
			return nil, fmt.Errorf("ITEM_KEY or --from is required")
		}
		defaults.Type = strings.ToLower(strings.TrimSpace(defaults.Type))
		if err := validateAnnotationRequest(defaults); err != nil {
			return nil, err
		}
		return []app.AnnotationTarget{{ItemKey: defaultKey, Request: defaults}}, nil
	}
	content, err := os.ReadFile(from)
	if err != nil {
		return nil, fmt.Errorf("read --from %q: %w", from, err)
	}
	var entries []annotationFileEntry
	if err := json.Unmarshal(content, &entries); err != nil {
		return nil, fmt.Errorf("parse --from %q: %w", from, err)
	}
	targets := make([]app.AnnotationTarget, 0, len(entries))
	for i, entry := range entries {
		key := entry.ItemKey
		if key == "" {
			key = defaultKey
		}
		request := defaults
		if entry.AttachmentKey != "" {
			request.AttachmentKey = entry.AttachmentKey
		}
		if entry.Text != "" {
			request.Text = entry.Text
		}
		if entry.Color != "" {
			request.Color = entry.Color
		}
		if entry.Comment != "" {
			request.Comment = entry.Comment
		}
		if entry.Type != "" {
			request.Type = entry.Type
		}
		if entry.Page > 0 {
			request.Page = entry.Page
		}
		if len(entry.Rect) > 0 {
			if len(entry.Rect) != 4 {
				return nil, fmt.Errorf("annotation %d rect must contain 4 numbers", i+1)
			}
			request.Rect = &[4]float64{entry.Rect[0], entry.Rect[1], entry.Rect[2], entry.Rect[3]}
		}
		if len(entry.Point) > 0 {
			if len(entry.Point) != 2 {
				return nil, fmt.Errorf("annotation %d point must contain 2 numbers", i+1)
			}
			request.Point = &[2]float64{entry.Point[0], entry.Point[1]}
			if entry.Type == "" && request.Type == "highlight" {
				request.Type = "note"
			}
		}
		if entry.DryRun != nil {
			request.DryRun = *entry.DryRun
		}
		if defaults.DryRun {
			request.DryRun = true
		}
		if key == "" {
			return nil, fmt.Errorf("annotation %d is missing item_key", i+1)
		}
		request.Type = strings.ToLower(strings.TrimSpace(request.Type))
		if err := validateAnnotationRequest(request); err != nil {
			return nil, fmt.Errorf("annotation %d: %w", i+1, err)
		}
		targets = append(targets, app.AnnotationTarget{ItemKey: key, Request: request})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no annotations in --from file")
	}
	return targets, nil
}

func validateAnnotationRequest(req backend.AnnotateRequest) error {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "highlight" && req.Type != "underline" && req.Type != "note" {
		return fmt.Errorf("unsupported annotation type %q; choose highlight, underline, or note", req.Type)
	}
	hasText := strings.TrimSpace(req.Text) != ""
	hasRect := req.Page > 0 && req.Rect != nil
	hasPoint := req.Page > 0 && req.Point != nil
	if !hasText && !hasRect && !hasPoint {
		return fmt.Errorf("missing annotation target (use --text, --page+--rect, or --page+--point)")
	}
	if req.Rect != nil && req.Point != nil {
		return fmt.Errorf("--rect and --point are mutually exclusive")
	}
	if hasText && (req.Rect != nil || req.Point != nil) {
		return fmt.Errorf("--text, --rect, and --point target modes are mutually exclusive")
	}
	if req.Page < 0 {
		return fmt.Errorf("--page must be non-negative")
	}
	if req.Type == "note" && !hasPoint {
		return fmt.Errorf("note annotations require --page and --point")
	}
	if req.Type != "note" && hasPoint {
		return fmt.Errorf("--point creates a note annotation; use --type note")
	}
	return nil
}

func parseFloatTuple(value string, count int) ([]float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) != count {
		return nil, fmt.Errorf("expected %d comma-separated numbers", count)
	}
	result := make([]float64, count)
	for i, part := range parts {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, err
		}
		result[i] = parsed
	}
	return result, nil
}
