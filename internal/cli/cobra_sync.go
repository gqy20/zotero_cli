package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

func (c *CLI) newSyncCommand(opts *globalOptions) *cobra.Command {
	syncCmd := &cobra.Command{Use: "sync", Short: "Synchronize a remote library for offline use"}
	req := app.SyncPullRequest{Concurrency: 8}
	pull := &cobra.Command{
		Use:   "pull",
		Short: "Pull a remote library into a local mirror",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if req.Concurrency < 1 {
				return &exitError{code: ExitUsage, err: fmt.Errorf("--concurrency must be at least 1")}
			}
			service := app.NewSyncService()
			if !opts.quiet {
				service.Progress = cmd.ErrOrStderr()
			}
			path := app.CommandPath{Resource: "sync", Action: "pull"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return service.Pull(ctx, req)
			})
		},
	}
	flags := pull.Flags()
	flags.StringVar(&req.ServerAddr, "server-addr", "", "remote zot server URL")
	flags.StringVar(&req.DataDir, "data-dir", "", "local destination directory")
	flags.IntVar(&req.Concurrency, "concurrency", 8, "parallel downloads")
	flags.BoolVar(&req.Force, "force", false, "download files even when unchanged")
	flags.BoolVar(&req.NoStorage, "no-storage", false, "skip PDF and attachment storage")
	syncCmd.AddCommand(pull)
	return syncCmd
}
