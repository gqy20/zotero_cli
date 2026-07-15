package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

func (c *CLI) newSchemaCommand(opts *globalOptions) *cobra.Command {
	service := app.NewSchemaService()
	var listRefresh bool
	var showRefresh bool
	schema := &cobra.Command{
		Use:   "schema",
		Short: "Discover Zotero item types, fields, creator roles, and templates",
		Long: "Inspect the Zotero metadata schema through read-only, cached subcommands used to\n" +
			"create and edit items. `list` discovers\n" +
			"valid item types, fields, and creator roles; `show` returns Zotero's official\n" +
			"creation template for an item type. Schema commands are read-only and cached.",
		Example: "  zot schema list types\n  zot schema list fields journalArticle\n  zot schema list roles journalArticle\n  zot schema show journalArticle --json",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return &exitError{code: ExitUsage, err: fmt.Errorf("schema requires a subcommand: list or show")}
		},
	}
	list := &cobra.Command{
		Use:     "list <types|fields|roles> [item-type]",
		Short:   "List valid item types, fields, or creator roles",
		Long:    "List localized Zotero schema values. Add an item type when listing fields or\ncreator roles to restrict the result. --refresh bypasses the local schema cache.",
		Example: "  zot schema list types\n  zot schema list fields journalArticle\n  zot schema list roles journalArticle\n  zot schema list fields journalArticle --refresh",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "types" && args[0] != "fields" && args[0] != "roles" {
				return &exitError{code: ExitUsage, err: fmt.Errorf("unknown schema category %q; use types, fields, or roles", args[0])}
			}
			itemType := ""
			if len(args) == 2 {
				itemType = args[1]
			}
			path := app.CommandPath{Resource: "schema", Action: "list"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.ListWithOptions(ctx, args[0], itemType, app.SchemaOptions{Refresh: listRefresh})
			})
		},
	}
	list.Flags().BoolVar(&listRefresh, "refresh", false, "bypass the schema cache")
	show := &cobra.Command{
		Use:     "show <item-type>",
		Short:   "Show the creation template for an item type",
		Long:    "Return Zotero's official JSON creation template for one item type. The template\nis a starting payload: fill in the desired values before passing it to `zot item new`.",
		Example: "  zot schema show journalArticle\n  zot schema show journalArticle --json\n  zot schema show book --refresh",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := app.CommandPath{Resource: "schema", Action: "show"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.ShowWithOptions(ctx, args[0], app.SchemaOptions{Refresh: showRefresh})
			})
		},
	}
	show.Flags().BoolVar(&showRefresh, "refresh", false, "bypass the schema cache")
	schema.AddCommand(list, show)
	return schema
}
