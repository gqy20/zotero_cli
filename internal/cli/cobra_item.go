package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
)

func (c *CLI) addItemReadCommands(item *cobra.Command, opts *globalOptions) {
	var find backend.FindOptions
	var snippet bool
	find.In = "metadata"
	findCmd := &cobra.Command{Use: "find [QUERY]", Short: "Find or browse library items", Args: cobra.ArbitraryArgs}
	flags := findCmd.Flags()
	flags.BoolVar(&find.All, "all", false, "remove the result limit")
	flags.StringVar(&find.In, "in", "metadata", "search scope: metadata or fulltext")
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
		if find.All && cmd.Flags().Changed("limit") {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--all and --limit are mutually exclusive")}
		}
		if find.Full && snippet {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--full and --snippet are mutually exclusive")}
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
		find.In = strings.ToLower(strings.TrimSpace(find.In))
		if find.In != "metadata" && find.In != "fulltext" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--in must be metadata or fulltext")}
		}
		if find.In != "metadata" && find.Query == "" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--in %s requires QUERY", find.In)}
		}
		if err := validateIncludeFields(find.IncludeFields); err != nil {
			return &exitError{code: ExitUsage, err: err}
		}
		path := app.CommandPath{Resource: "item", Action: "find"}
		return c.runRead(cmd, opts, path, func(ctx context.Context, service app.ReadService) (app.Result, error) {
			return service.FindItems(ctx, app.ItemFindRequest{
				Options: find,
				Snippet: snippet,
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
	var exportFrom string
	exportCmd := &cobra.Command{Use: "export [KEY...]", Short: "Export item citations", Args: cobra.ArbitraryArgs}
	exportCmd.Flags().StringVar(&exportReq.Format, "as", "bibtex", "bibtex, biblatex, csljson, or ris")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "read item keys or find JSON from a file, or - for stdin")
	exportCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if exportReq.Format != "bibtex" && exportReq.Format != "biblatex" && exportReq.Format != "csljson" && exportReq.Format != "ris" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("unsupported export format %q", exportReq.Format)}
		}
		exportReq.Keys = args
		if len(args) > 0 && strings.TrimSpace(exportFrom) != "" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("item keys and --from are mutually exclusive")}
		}
		if strings.TrimSpace(exportFrom) != "" {
			keys, err := app.ResolveExportKeys(exportFrom, c.stdin)
			if err != nil {
				return &exitError{code: ExitUsage, err: err}
			}
			exportReq.Keys = keys
		}
		if len(exportReq.Keys) == 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("provide item keys or --from")}
		}
		service := app.NewExportService()
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
