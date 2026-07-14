package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/references"
)

func (c *CLI) referenceService() app.ReferenceService {
	service := app.NewReferenceService()
	service.NewReader = func(cfg config.Config) (backend.Reader, error) { return c.backendNewReader(cfg, nil) }
	return service
}

func (c *CLI) runReference(cmd *cobra.Command, opts *globalOptions, action string, run func(context.Context, app.ReferenceService) (app.Result, error)) error {
	path := app.CommandPath{Resource: "ref", Action: action}
	return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) { return run(ctx, c.referenceService()) })
}

func (c *CLI) newReferenceCommand(opts *globalOptions) *cobra.Command {
	ref := &cobra.Command{Use: "ref", Short: "Build and query the structured reference index"}
	var show app.ReferenceShowRequest
	show.Source = "auto"
	showCmd := &cobra.Command{Use: "show ITEM_KEY", Short: "Fetch and index references for one item", Args: cobra.ExactArgs(1)}
	showCmd.Flags().StringVar(&show.Source, "source", "auto", "auto, pmc, or pubmed")
	showCmd.Flags().BoolVar(&show.Refresh, "refresh", false, "bypass caches")
	showCmd.RunE = func(cmd *cobra.Command, args []string) error {
		show.Key = args[0]
		return c.runReference(cmd, opts, "show", func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			return service.Show(ctx, show)
		})
	}

	var find app.ReferenceFindRequest
	find.Options = references.SearchOptions{In: "all", Limit: 20}
	findCmd := &cobra.Command{Use: "find QUERY", Short: "Search references and citation contexts", Args: cobra.MinimumNArgs(1)}
	findCmd.Flags().StringVar(&find.Options.In, "in", "all", "search scope: all, references, contexts, or metadata")
	findCmd.Flags().StringVar(&find.Options.Field, "field", "", "metadata field")
	findCmd.Flags().StringVar(&find.Options.Section, "section", "", "citation context section")
	findCmd.Flags().StringVar(&find.Options.Source, "source", "", "pmc, pubmed, europepmc, or grobid")
	findCmd.Flags().StringVar(&find.Options.Target, "target", "", "target item key")
	findCmd.Flags().IntVar(&find.Options.Limit, "limit", 20, "maximum results")
	findCmd.RunE = func(cmd *cobra.Command, args []string) error {
		find.Options.Query = strings.Join(args, " ")
		find.Options.In = strings.ToLower(strings.TrimSpace(find.Options.In))
		if find.Options.In != "all" && find.Options.In != "references" && find.Options.In != "contexts" && find.Options.In != "metadata" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--in must be all, references, contexts, or metadata")}
		}
		if find.Options.Section != "" && find.Options.In != "all" && find.Options.In != "contexts" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--section requires --in contexts or --in all")}
		}
		if find.Options.Field != "" && find.Options.In != "all" && find.Options.In != "metadata" {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--field requires --in metadata or --in all")}
		}
		find.Options.Source = normalizeReferenceSearchSource(find.Options.Source)
		if find.Options.Limit < 1 || find.Options.Limit > 200 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--limit must be between 1 and 200")}
		}
		if !validReferenceField(find.Options.Field) {
			return &exitError{code: ExitUsage, err: fmt.Errorf("invalid --field %q", find.Options.Field)}
		}
		return c.runReference(cmd, opts, "find", func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			return service.Find(ctx, find)
		})
	}

	var build app.ReferenceBuildRequest
	build.Source = "auto"
	build.Workers = 3
	buildCmd := &cobra.Command{Use: "build", Short: "Build or backfill the reference index", Args: cobra.NoArgs}
	buildCmd.Flags().IntVar(&build.Workers, "workers", 3, "parallel workers")
	buildCmd.Flags().IntVar(&build.Limit, "limit", 0, "maximum pending items")
	buildCmd.Flags().StringVar(&build.Source, "source", "auto", "auto, pmc, or pubmed")
	buildCmd.Flags().BoolVar(&build.Force, "force", false, "reprocess fresh items")
	buildCmd.Flags().BoolVar(&build.Refresh, "refresh", false, "bypass network caches")
	buildCmd.Flags().BoolVar(&build.Failed, "failed", false, "retry only failed items")
	buildCmd.Flags().BoolVar(&build.Contexts, "contexts", false, "backfill PMC citation contexts")
	buildCmd.Flags().BoolVar(&build.Grobid, "grobid", false, "EXPERIMENTAL: process unsupported items through GROBID")
	buildCmd.Flags().BoolVar(&build.All, "all", false, "process every GROBID candidate")
	buildCmd.MarkFlagsMutuallyExclusive("failed", "contexts", "grobid")
	buildCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return c.runReference(cmd, opts, "build", func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			return service.Build(ctx, build)
		})
	}

	var status app.ReferenceStatusRequest
	statusCmd := &cobra.Command{Use: "status", Short: "Show reference index coverage", Args: cobra.NoArgs}
	statusCmd.Flags().BoolVar(&status.Failed, "failed", false, "list failed items")
	statusCmd.Flags().BoolVar(&status.Unsupported, "unsupported", false, "list unsupported items")
	statusCmd.Flags().BoolVar(&status.Grobid, "grobid", false, "check experimental GROBID fallback")
	statusCmd.MarkFlagsMutuallyExclusive("failed", "unsupported", "grobid")
	statusCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return c.runReference(cmd, opts, "status", func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			return service.Status(ctx, status)
		})
	}

	var resolveWorkers int
	resolve := &cobra.Command{Use: "resolve", Short: "Match indexed references to local items", Args: cobra.NoArgs}
	resolve.Flags().IntVar(&resolveWorkers, "workers", 0, "parallel workers")
	resolve.RunE = func(cmd *cobra.Command, _ []string) error {
		return c.runReference(cmd, opts, "resolve", func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			return service.Resolve(ctx, resolveWorkers)
		})
	}

	ref.AddCommand(showCmd, findCmd, buildCmd, statusCmd, resolve)
	ref.AddCommand(c.referenceDiscoveryCommand(opts, "related"), c.referenceDiscoveryCommand(opts, "cited"), c.referenceDiscoveryCommand(opts, "ctx"), c.referenceDiscoveryCommand(opts, "links"), c.referenceDiscoveryCommand(opts, "entities"), c.referenceDiscoveryCommand(opts, "profile"))
	return ref
}

func (c *CLI) referenceDiscoveryCommand(opts *globalOptions, action string) *cobra.Command {
	var req app.ReferenceDiscoveryRequest
	cmd := &cobra.Command{Use: action + " ITEM_KEY", Short: "Query reference " + action + " data", Args: cobra.ExactArgs(1)}
	if action == "related" || action == "cited" {
		cmd.Flags().IntVar(&req.Limit, "limit", 0, "maximum results")
	}
	if action == "related" {
		cmd.Flags().BoolVar(&req.AlsoViewed, "also-viewed", false, "use PubMed also-viewed ordering")
	}
	if action == "cited" {
		cmd.Flags().BoolVar(&req.External, "external", false, "query external Europe PMC citations")
	}
	if action != "ctx" {
		cmd.Flags().BoolVar(&req.Refresh, "refresh", false, "bypass network caches")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		req.Key = args[0]
		if req.Limit < 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("--limit must be non-negative")}
		}
		return c.runReference(cmd, opts, action, func(ctx context.Context, service app.ReferenceService) (app.Result, error) {
			switch action {
			case "related":
				return service.Related(ctx, req)
			case "cited":
				return service.Cited(ctx, req)
			case "ctx":
				return service.Contexts(ctx, req.Key)
			case "links":
				return service.Links(ctx, req)
			case "entities":
				return service.Entities(ctx, req)
			case "profile":
				return service.Profile(ctx, req)
			}
			return app.Result{}, fmt.Errorf("unsupported reference action %q", action)
		})
	}
	return cmd
}

func normalizeReferenceSearchSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "pmc":
		return string(references.SourcePMC)
	case "pubmed":
		return string(references.SourcePubMed)
	case "europepmc", "europe-pmc":
		return string(references.SourceEuropePMC)
	case "grobid":
		return string(references.SourceGROBID)
	}
	return value
}
func validReferenceField(value string) bool {
	return map[string]bool{"": true, "mesh": true, "publication_types": true, "keywords": true, "chemicals": true, "grants": true, "corrections": true, "annotations": true}[value]
}
