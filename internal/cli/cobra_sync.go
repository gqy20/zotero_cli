package cli

import (
	"context"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

func (c *CLI) newSyncCommand(opts *globalOptions) *cobra.Command {
	var force bool
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize the configured remote library for offline use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service := app.NewSyncService()
			service.Progress = cmd.ErrOrStderr()
			path := app.CommandPath{Resource: "sync"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.Sync(ctx, app.SyncRequest{Force: force})
			})
		},
	}
	syncCmd.Flags().BoolVar(&force, "force", false, "download files even when unchanged")

	var full bool
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show mirror status and verify local sync data",
		Long: "Show the health and location of the offline mirror. The data directory contains\n" +
			"the mirrored SQLite database, storage/ attachments, and attachments/ linked files.\n" +
			"Use `zot file path KEY` to locate attachments for an item or attachment key.",
		Example: "  zot sync status\n  zot sync status --full\n  zot sync status --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service := app.NewSyncService()
			path := app.CommandPath{Resource: "sync", Action: "status"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.Status(ctx, app.SyncStatusRequest{Full: full})
			})
		},
	}
	statusCmd.Flags().BoolVar(&full, "full", false, "run full SQLite and last-manifest verification")
	syncCmd.AddCommand(statusCmd)
	return syncCmd
}
