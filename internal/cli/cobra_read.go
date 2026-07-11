package cli

import (
	"context"
	"fmt"

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
	cmd := &cobra.Command{Use: "list", Short: "List items in a library scope", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if list.Scope == "" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--scope is required (trash or pubs)")}
		}
		return c.runRead(cmd, opts, app.CommandPath{Resource: "item", Action: "list"}, func(ctx context.Context, s app.ReadService) (app.Result, error) { return s.Items(ctx, list) })
	}}
	addListFlags(cmd, &list)
	cmd.Flags().StringVar(&list.Scope, "scope", "", "trash or pubs")
	item.AddCommand(cmd)
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
