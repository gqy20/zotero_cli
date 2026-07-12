package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

func (c *CLI) newServeCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Share this Zotero library on the local network",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			path := app.CommandPath{Resource: "serve"}
			return c.renderResult(ctx, opts, path, func(ctx context.Context) (app.Result, error) {
				return app.NewServerService().Start(ctx)
			})
		},
	}
}
