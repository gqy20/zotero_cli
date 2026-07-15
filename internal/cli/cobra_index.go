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
	index := &cobra.Command{
		Use:   "index",
		Short: "Build and inspect the derived PDF full-text index",
		Long: "Build and inspect searchable text derived from local PDF attachments. The index\n" +
			"does not contain PDF binaries; it stores extracted-text caches and a SQLite FTS\n" +
			"index below <data-dir>/.zotero_cli/fulltext/. Search it with\n" +
			"`zot find QUERY --in fulltext` and locate source PDFs with `zot file path`.",
		Example: "  zot index build\n  zot index status\n  zot find '\"gene flow\"' --in fulltext --snippet",
	}
	var build app.IndexBuildRequest
	buildCmd := &cobra.Command{Use: "build", Short: "Extract and index text from locally available PDFs", Long: "Scan locally available PDF attachments, reuse fresh extracted-text caches, and\nadd their text to the SQLite full-text index. --force re-extracts cached attachments.\nUse `zot index status` afterward to see the index path and total derived-data size.", Example: "  zot index build\n  zot index build --workers 4\n  zot index build --force", Args: cobra.NoArgs}
	buildCmd.Flags().BoolVar(&build.Force, "force", false, "rebuild cached attachments")
	buildCmd.Flags().IntVar(&build.Workers, "workers", 0, "parallel workers")
	buildCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if build.Workers < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--workers must be non-negative")}
		}
		return c.renderResult(cmd.Context(), opts, app.CommandPath{Resource: "index", Action: "build"}, func(ctx context.Context) (app.Result, error) { return c.indexService().Build(ctx, build) })
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Show extracted PDF full-text index status",
		Long: "Show the location and availability of the derived full-text index.\n\n" +
			"The index does not contain PDF binaries. Source attachments may resolve under\n" +
			"storage/ or attachments/; use `zot file path ATTACHMENT_KEY` for an exact path.\n" +
			"Reported storage includes both index.sqlite and extracted-text cache files.",
		Example: "  zot index status\n  zot index status --json",
		Args:    cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return c.renderResult(cmd.Context(), opts, app.CommandPath{Resource: "index", Action: "status"}, func(ctx context.Context) (app.Result, error) { return c.indexService().Status(ctx) })
		}}
	index.AddCommand(buildCmd, status)
	return index
}
