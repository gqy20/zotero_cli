package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

const usageServer = "usage: zot server start [--port PORT]"

func (c *CLI) newServerCommand(opts *globalOptions) *cobra.Command {
	serverCmd := &cobra.Command{Use: "server", Short: "Run the remote-mode HTTP server"}
	var req app.ServerStartRequest
	start := &cobra.Command{
		Use:   "start",
		Short: "Start the HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			path := app.CommandPath{Resource: "server", Action: "start"}
			return c.renderResult(ctx, opts, path, func(ctx context.Context) (app.Result, error) {
				return app.NewServerService().Start(ctx, req)
			})
		},
	}
	start.Flags().StringVar(&req.Port, "port", "", "override the configured listen port")
	serverCmd.AddCommand(start)
	return serverCmd
}
