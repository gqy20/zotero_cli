package cli

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) readService() app.ReadService {
	service := app.NewReadService()
	service.NewReader = func(cfg config.Config) (backend.Reader, error) { return c.backendNewReader(cfg, nil) }
	return service
}

func (c *CLI) addReadCommands(root *cobra.Command, opts *globalOptions) {
	root.AddCommand(
		c.newLibCommand(opts), c.newItemListCommand(opts), c.newCollCommand(opts),
		c.newTagCommand(opts), c.newNoteListCommand(opts), c.newSearchListCommand(opts), c.newGroupCommand(opts),
	)
}

func (c *CLI) runRead(cmd *cobra.Command, opts *globalOptions, path app.CommandPath, run func(context.Context, app.ReadService) (app.Result, error)) error {
	return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return run(ctx, c.readService()) })
}

func (c *CLI) newLibCommand(opts *globalOptions) *cobra.Command {
	lib := &cobra.Command{Use: "lib", Short: "Inspect library state"}
	show := &cobra.Command{Use: "show", Short: "Show a library overview", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "lib", Action: "show"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Overview(ctx) })
	}}
	stats := &cobra.Command{Use: "stats", Short: "Show library counters", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "lib", Action: "stats"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Stats(ctx) })
	}}
	var logOpts app.LogOptions
	logCmd := &cobra.Command{Use: "log", Short: "Show changed or deleted objects", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !logOpts.Deleted && logOpts.Kind == "" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--kind is required unless --deleted is set")}
		}
		if logOpts.Kind != "" && !slices.Contains([]string{"items", "items-top", "collections", "searches"}, logOpts.Kind) {
			return &exitError{code: ExitUsage, err: fmt.Errorf("unsupported object type %q", logOpts.Kind)}
		}
		if !logOpts.Deleted && !cmd.Flags().Changed("since") {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--since is required")}
		}
		if logOpts.IfModifiedVersion < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("invalid value for --if-modified-version: must be non-negative")}
		}
		return c.runRead(cmd, opts, app.CommandPath{Resource: "lib", Action: "log"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Log(ctx, logOpts) })
	}}
	logCmd.Flags().StringVar(&logOpts.Kind, "kind", "", "items, items-top, collections, or searches")
	logCmd.Flags().IntVar(&logOpts.Since, "since", 0, "return changes since library version")
	logCmd.Flags().BoolVar(&logOpts.Deleted, "deleted", false, "show deleted object keys")
	logCmd.Flags().BoolVar(&logOpts.IncludeTrashed, "include-trashed", false, "include trashed items")
	logCmd.Flags().IntVar(&logOpts.IfModifiedVersion, "if-modified-version", 0, "conditional request version")
	lib.AddCommand(show, stats, logCmd)
	return lib
}

func addListFlags(cmd *cobra.Command, opts *app.ListOptions) {
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "maximum results")
}

func (c *CLI) newItemListCommand(opts *globalOptions) *cobra.Command {
	item := &cobra.Command{Use: "item", Short: "Work with library items"}
	var list app.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List library items", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "item", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Items(ctx, list) })
	}}
	addListFlags(cmd, &list)
	cmd.Flags().StringVar(&list.Scope, "scope", "", "trash or pubs")
	cmd.Flags().IntVar(&list.Offset, "offset", 0, "result offset")
	cmd.Flags().StringVar(&list.Sort, "sort", "", "sort field")
	cmd.Flags().StringVar(&list.Order, "order", "", "asc or desc")
	cmd.Flags().StringVar(&list.ItemType, "type", "", "item type, such as article or journalArticle")
	cmd.Flags().StringArrayVar(&list.Tags, "tag", nil, "require a tag")
	previousRun := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		list.ItemType = app.NormalizeItemType(list.ItemType)
		if list.Limit < 0 || list.Offset < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--limit and --offset must be non-negative")}
		}
		if list.Order != "" && list.Order != "asc" && list.Order != "desc" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--order must be asc or desc")}
		}
		return previousRun(cmd, args)
	}
	item.AddCommand(cmd)
	c.addItemReadCommands(item, opts)
	c.addItemWriteCommands(item, opts)
	return item
}

func (c *CLI) newCollCommand(opts *globalOptions) *cobra.Command {
	coll := &cobra.Command{Use: "coll", Short: "Work with collections"}
	var list app.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List collections", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "coll", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Collections(ctx, list) })
	}}
	addListFlags(cmd, &list)
	cmd.Flags().BoolVar(&list.Top, "top", false, "only top-level collections")
	coll.AddCommand(cmd)
	show := &cobra.Command{Use: "show KEY", Short: "Show one collection", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "coll", Action: "show"}, func(ctx context.Context, s app.ReadService) (app.Result, error) {
			return s.ShowCollection(ctx, args[0])
		})
	}}
	coll.AddCommand(show)
	c.addCollectionWriteCommands(coll, opts)
	return coll
}

func (c *CLI) newTagCommand(opts *globalOptions) *cobra.Command {
	tag := &cobra.Command{Use: "tag", Short: "Work with tags"}
	var list app.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List tags", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "tag", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Tags(ctx, list) })
	}}
	addListFlags(cmd, &list)
	tag.AddCommand(cmd)
	return tag
}
func (c *CLI) newNoteListCommand(opts *globalOptions) *cobra.Command {
	note := &cobra.Command{Use: "note", Short: "Work with notes"}
	var list app.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List notes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "note", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Notes(ctx, list) })
	}}
	addListFlags(cmd, &list)
	cmd.Flags().StringVar(&list.Query, "query", "", "filter note text")
	note.AddCommand(cmd)
	find := &cobra.Command{Use: "find QUERY", Short: "Find notes by content", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "note", Action: "find"}, func(ctx context.Context, s app.ReadService) (app.Result, error) {
			return s.Notes(ctx, app.ListOptions{Query: args[0], Limit: list.Limit})
		})
	}}
	addListFlags(find, &list)
	show := &cobra.Command{Use: "show KEY", Short: "Show one note", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "note", Action: "show"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.ShowNote(ctx, args[0]) })
	}}
	note.AddCommand(find, show)
	c.addObjectWriteCommands(note, opts, "note")
	return note
}
func (c *CLI) newSearchListCommand(opts *globalOptions) *cobra.Command {
	search := &cobra.Command{Use: "search", Short: "Work with saved searches"}
	var list app.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List saved searches", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "search", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Searches(ctx, list) })
	}}
	addListFlags(cmd, &list)
	search.AddCommand(cmd)
	show := &cobra.Command{Use: "show KEY", Short: "Show one saved search", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "search", Action: "show"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.ShowSearch(ctx, args[0]) })
	}}
	search.AddCommand(show)
	c.addObjectWriteCommands(search, opts, "search")
	return search
}
func (c *CLI) newGroupCommand(opts *globalOptions) *cobra.Command {
	group := &cobra.Command{Use: "group", Short: "Work with groups"}
	cmd := &cobra.Command{Use: "list", Short: "List accessible groups", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.runRead(cmd, opts, app.CommandPath{Resource: "group", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) {
			return s.Groups(ctx, app.ListOptions{})
		})
	}}
	group.AddCommand(cmd)
	return group
}
