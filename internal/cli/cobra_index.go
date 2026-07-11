package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) indexService() app.IndexService {
	service := app.NewIndexService()
	service.NewReader = func(cfg config.Config) (backend.Reader, error) { return c.newLocalReader(cfg) }
	return service
}

func (c *CLI) newIndexCommand(opts *globalOptions) *cobra.Command {
	index := &cobra.Command{Use: "index", Short: "Build and inspect the PDF full-text index"}
	var build app.IndexBuildRequest
	buildCmd := &cobra.Command{Use: "build", Short: "Build the PDF full-text index", Args: cobra.NoArgs}
	buildCmd.Flags().BoolVar(&build.Force, "force", false, "rebuild cached attachments")
	buildCmd.Flags().IntVar(&build.Workers, "workers", 0, "parallel workers")
	buildCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if build.Workers < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--workers must be non-negative")}
		}
		return c.renderResult(cmd.Context(), opts, app.CommandPath{Resource: "index", Action: "build"}, func(ctx context.Context) (app.Result, error) { return c.indexService().Build(ctx, build) })
	}
	status := &cobra.Command{Use: "status", Short: "Show PDF full-text index status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return c.renderResult(cmd.Context(), opts, app.CommandPath{Resource: "index", Action: "status"}, func(ctx context.Context) (app.Result, error) { return c.indexService().Status(ctx) })
	}}
	index.AddCommand(buildCmd, status)
	return index
}
