package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) addItemReadCommands(item *cobra.Command, opts *globalOptions) {
	var find backend.FindOptions
	var snippet bool
	findCmd := &cobra.Command{Use: "find QUERY", Short: "Find library items", Args: cobra.ArbitraryArgs}
	flags := findCmd.Flags()
	flags.BoolVar(&find.All, "all", false, "disable the default result limit; also permits an empty query")
	flags.BoolVar(&find.FullText, "fulltext", false, "search metadata and full text")
	flags.BoolVar(&find.FullTextOnly, "fulltext-only", false, "search only full text")
	flags.BoolVar(&find.MetadataOnly, "metadata-only", false, "search only metadata")
	flags.BoolVar(&find.FullTextAny, "fulltext-any", false, "match any full-text term")
	flags.BoolVar(&find.Full, "full", false, "return full item data")
	flags.BoolVar(&snippet, "snippet", false, "include a matching full-text preview")
	flags.StringVar(&find.ItemType, "type", "", "item type, such as article or journalArticle")
	flags.StringArrayVar(&find.Tags, "tag", nil, "require a tag (repeatable)")
	flags.BoolVar(&find.TagAny, "tag-any", false, "match any supplied tag")
	flags.StringSliceVar(&find.IncludeFields, "include-fields", nil, "extra text fields")
	flags.StringVar(&find.DateAfter, "date-after", "", "minimum publication date")
	flags.StringVar(&find.DateBefore, "date-before", "", "maximum publication date")
	flags.IntVarP(&find.Limit, "limit", "l", 0, "maximum results (default 100, or 20 with --snippet/--full; use --all for unlimited)")
	flags.IntVar(&find.Start, "offset", 0, "result offset")
	flags.StringVar(&find.Sort, "sort", "", "sort field")
	flags.StringVar(&find.Direction, "order", "", "asc or desc")
	flags.StringVar(&find.QMode, "qmode", "", "titleCreatorYear or everything")
	flags.BoolVar(&find.HasPDF, "has-pdf", false, "only items with PDF attachments")
	flags.StringVar(&find.AttachmentName, "attachment-name", "", "filter attachment filenames")
	flags.StringVar(&find.AttachmentPath, "attachment-path", "", "filter attachment paths")
	flags.StringVar(&find.AttachmentType, "attachment-type", "", "filter attachment content types")
	flags.BoolVar(&find.MissingAttachment, "missing-attachment", false, "only items with missing attachments")
	flags.BoolVar(&find.BadAttachmentName, "bad-attachment-name", false, "only items with unhealthy attachment names")
	flags.StringVar(&find.AttachmentHealth, "attachment-health", "", "critical, error, warning, or info")
	flags.StringArrayVar(&find.Collection, "collection", nil, "require collection membership")
	flags.StringArrayVar(&find.NoCollection, "no-collection", nil, "exclude collection membership")
	flags.StringArrayVar(&find.TagContains, "tag-contains", nil, "match a tag fragment")
	flags.StringArrayVar(&find.ExcludeTags, "exclude-tag", nil, "exclude a tag")
	flags.StringVar(&find.ExcludeItemType, "no-type", "", "exclude an item type")
	flags.StringVar(&find.DateModifiedAfter, "modified-within", "", "modified within a duration")
	flags.StringVar(&find.DateAddedAfter, "added-since", "", "added within a duration")
	flags.BoolVar(&find.IncludeTrashed, "include-trashed", false, "include trashed items")
	findCmd.RunE = func(cmd *cobra.Command, args []string) error {
		find.Query = strings.TrimSpace(strings.Join(args, " "))
		find.ItemType = app.NormalizeItemType(find.ItemType)
		find.ExcludeItemType = app.NormalizeItemType(find.ExcludeItemType)
		if find.Limit < 0 || find.Start < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--limit and --offset must be non-negative")}
		}
		if cmd.Flags().Changed("limit") && find.Limit == 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--limit must be positive; use --all to remove the result limit")}
		}
		if find.Direction != "" && find.Direction != "asc" && find.Direction != "desc" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--order must be asc or desc")}
		}
		if find.QMode != "" && find.QMode != "titleCreatorYear" && find.QMode != "everything" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("invalid value for --qmode")}
		}
		if find.FullTextOnly && find.MetadataOnly {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--fulltext-only and --metadata-only are mutually exclusive")}
		}
		if find.FullTextAny && !find.FullText && !find.FullTextOnly {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--fulltext-any requires --fulltext")}
		}
		if find.Query == "" && len(args) == 0 && !find.All && !hasFindFilters(find) {
			return &exitError{code: ExitUsage, err: fmt.Errorf("QUERY, --all, or a filter is required")}
		}
		if find.Query == "" && hasFindFilters(find) {
			find.All = true
		}
		if err := validateIncludeFields(find.IncludeFields); err != nil {
			return &exitError{code: ExitUsage, err: err}
		}
		path := app.CommandPath{Resource: "item", Action: "find"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.FindItems(ctx, app.ItemFindRequest{
				Options:     find,
				Snippet:     snippet,
				ExplicitAll: cmd.Flags().Changed("all") && find.All,
			})
		})
	}

	var full, showSnippet bool
	show := &cobra.Command{Use: "show KEY", Short: "Show one library item", Args: cobra.ExactArgs(1)}
	show.Flags().BoolVar(&full, "full", false, "return full item data")
	show.Flags().BoolVar(&showSnippet, "snippet", false, "include a full-text preview")
	show.RunE = func(cmd *cobra.Command, args []string) error {
		path := app.CommandPath{Resource: "item", Action: "show"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.ShowItem(ctx, app.ItemShowRequest{Key: args[0], Full: full, Snippet: showSnippet})
		})
	}
	var supp app.SupplementsRequest
	suppCmd := &cobra.Command{Use: "supp [KEY]", Short: "Find supplementary material", Args: cobra.MaximumNArgs(1)}
	suppCmd.Flags().BoolVar(&supp.All, "all", false, "scan every local item")
	suppCmd.Flags().BoolVar(&supp.Online, "online", false, "query public online providers")
	suppCmd.Flags().IntVar(&supp.Limit, "limit", 0, "maximum supplement records")
	suppCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			supp.Key = args[0]
		}
		if supp.All == (supp.Key != "") {
			return &exitError{code: ExitUsage, err: fmt.Errorf("provide exactly one item key or --all")}
		}
		path := app.CommandPath{Resource: "item", Action: "supp"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.Supplements(ctx, supp)
		})
	}
	var exportReq app.ItemExportRequest
	var exportQuery, exportType string
	var exportTags []string
	var exportAll bool
	exportCmd := &cobra.Command{Use: "export [KEY...]", Short: "Export item citations", Args: cobra.ArbitraryArgs}
	exportCmd.Flags().StringVar(&exportReq.Format, "as", "bibtex", "bibtex, biblatex, csljson, or ris")
	exportCmd.Flags().StringVar(&exportReq.Collection, "collection", "", "export items in a collection")
	exportCmd.Flags().StringVar(&exportQuery, "query", "", "export items matching a query")
	exportCmd.Flags().BoolVar(&exportAll, "all", false, "export all items matching filters")
	exportCmd.Flags().IntVar(&exportReq.Find.Limit, "limit", 0, "maximum matched items")
	exportCmd.Flags().StringVar(&exportType, "type", "", "item type filter")
	exportCmd.Flags().StringArrayVar(&exportTags, "tag", nil, "tag filter")
	exportCmd.Flags().StringVar(&exportReq.Find.AttachmentName, "attachment-name", "", "attachment filename filter")
	exportCmd.Flags().BoolVar(&exportReq.Find.HasPDF, "has-pdf", false, "only items with PDF attachments")
	exportCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if exportReq.Format != "bibtex" && exportReq.Format != "biblatex" && exportReq.Format != "csljson" && exportReq.Format != "ris" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("unsupported export format %q", exportReq.Format)}
		}
		exportReq.Keys = args
		exportReq.Find.Query = strings.TrimSpace(exportQuery)
		exportReq.Find.All = exportAll
		exportReq.Find.ItemType = app.NormalizeItemType(exportType)
		exportReq.Find.Tags = exportTags
		sources := 0
		if len(args) > 0 {
			sources++
		}
		if exportReq.Collection != "" {
			sources++
		}
		if exportReq.Find.Query != "" || exportAll || exportType != "" || len(exportTags) > 0 || exportReq.Find.AttachmentName != "" || exportReq.Find.HasPDF {
			sources++
		}
		if sources != 1 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("provide exactly one export source: item keys, --collection, --query, or --all/filters")}
		}
		service := app.NewExportService()
		service.NewReader = func(cfg config.Config) (backend.Reader, error) { return c.backendNewReader(cfg, nil) }
		service.NewLocalReader = c.newLocalReader
		path := app.CommandPath{Resource: "item", Action: "export"}
		return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return service.Export(ctx, exportReq) })
	}
	item.AddCommand(findCmd, show, suppCmd, exportCmd)
}

func validateIncludeFields(fields []string) error {
	allowed := map[string]bool{"key": true, "version": true, "item_type": true, "title": true, "date": true, "date_added": true, "abstract": true, "creators": true, "container": true, "volume": true, "issue": true, "pages": true, "doi": true, "url": true, "tags": true, "collections": true, "attachments": true, "notes": true, "matched_on": true, "full_text_preview": true}
	for _, field := range fields {
		if !allowed[strings.ToLower(strings.TrimSpace(field))] {
			return fmt.Errorf("invalid value for --include-fields: %s", field)
		}
	}
	return nil
}

func hasFindFilters(opts backend.FindOptions) bool {
	return opts.ItemType != "" || len(opts.Tags) > 0 || opts.DateAfter != "" || opts.DateBefore != "" || opts.HasPDF || len(opts.Collection) > 0 || len(opts.NoCollection) > 0 || len(opts.TagContains) > 0 || len(opts.ExcludeTags) > 0 || opts.ExcludeItemType != "" || opts.MissingAttachment || opts.BadAttachmentName || opts.AttachmentName != "" || opts.AttachmentPath != "" || opts.AttachmentType != "" || opts.AttachmentHealth != "" || opts.DateModifiedAfter != "" || opts.DateAddedAfter != ""
}
