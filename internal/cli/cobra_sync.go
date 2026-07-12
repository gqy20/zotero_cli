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
			if !opts.quiet {
				service.Progress = cmd.ErrOrStderr()
			}
			path := app.CommandPath{Resource: "sync"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.Sync(ctx, app.SyncRequest{Force: force})
			})
		},
	}
	syncCmd.Flags().BoolVar(&force, "force", false, "download files even when unchanged")
	return syncCmd
}
