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
		Short: "Inspect the Zotero metadata schema",
		Long:  "Inspect the Zotero metadata schema through the list and show subcommands.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return &exitError{code: ExitUsage, err: fmt.Errorf("schema requires a subcommand: list or show")}
		},
	}
	list := &cobra.Command{
		Use:   "list <types|fields|roles> [item-type]",
		Short: "List schema values",
		Args:  cobra.RangeArgs(1, 2),
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
		Use:   "show <item-type>",
		Short: "Show the creation template for an item type",
		Args:  cobra.ExactArgs(1),
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
