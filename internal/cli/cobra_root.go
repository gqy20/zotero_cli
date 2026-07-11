package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/config"
	appRender "zotero_cli/internal/render"
)

type globalOptions struct {
	format  string
	json    bool
	quiet   bool
	verbose bool
	noColor bool
	mode    string
	timeout string
	path    app.CommandPath
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func translateStageOneArgs(args []string) ([]string, bool) {
	if len(args) == 0 {
		return nil, false
	}
	switch args[0] {
	case "version":
		return args, true
	case "lib", "item", "coll", "tag", "note", "search", "group":
		return args, true
	case "init":
		return append([]string{"config", "init"}, args[1:]...), true
	case "overview":
		return append([]string{"lib", "show"}, args[1:]...), true
	case "stats":
		return append([]string{"lib", "stats"}, args[1:]...), true
	case "deleted":
		return append([]string{"lib", "log", "--deleted"}, args[1:]...), true
	case "changes":
		if len(args) < 2 {
			return []string{"lib", "log"}, true
		}
		translated := []string{"lib", "log", "--kind", args[1]}
		return append(translated, args[2:]...), true
	case "collections":
		return append([]string{"coll", "list"}, args[1:]...), true
	case "collections-top":
		return append([]string{"coll", "list", "--top"}, args[1:]...), true
	case "tags":
		return append([]string{"tag", "list"}, args[1:]...), true
	case "notes":
		return append([]string{"note", "list"}, args[1:]...), true
	case "searches":
		return append([]string{"search", "list"}, args[1:]...), true
	case "groups":
		return append([]string{"group", "list"}, args[1:]...), true
	case "trash":
		return append([]string{"item", "list", "--scope", "trash"}, args[1:]...), true
	case "publications":
		return append([]string{"item", "list", "--scope", "pubs"}, args[1:]...), true
	case "config":
		if len(args) < 2 {
			return args, true
		}
		switch args[1] {
		case "show", "check", "init":
			return args, true
		case "validate":
			translated := append([]string{"config", "check"}, args[2:]...)
			return translated, true
		case "path":
			translated := append([]string{"config", "show", "--path"}, args[2:]...)
			return translated, true
		}
	}
	return nil, false
}

func (c *CLI) runCobra(args []string) int {
	root := c.newRootCommand()
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return ExitOK
	}
	code := ExitError
	var ee *exitError
	if errors.As(err, &ee) {
		code = ee.code
	} else if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "requires") || strings.Contains(err.Error(), "accepts") {
		code = ExitUsage
	}
	path := app.CommandPath{Resource: "zot"}
	if cmd != nil {
		parts := strings.Fields(cmd.CommandPath())
		if len(parts) > 1 {
			path.Resource = parts[1]
		}
		if len(parts) > 2 {
			path.Action = parts[2]
		}
	}
	format := root.Flag("format").Value.String()
	if jsonFlag := root.Flag("json"); jsonFlag != nil && jsonFlag.Value.String() == "true" {
		format = "json"
	}
	_ = (appRender.Renderer{Out: c.stdout, Err: c.stderr}).Error(path, err, code, format)
	return code
}

func (c *CLI) newRootCommand() *cobra.Command {
	opts := &globalOptions{format: defaultOutputFormat()}
	root := &cobra.Command{
		Use:           "zot",
		Short:         "Work with a Zotero library",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(c.stdin)
	root.SetOut(c.stdout)
	root.SetErr(c.stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&opts.format, "format", opts.format, "output format: text or json")
	flags.BoolVar(&opts.json, "json", false, "output JSON")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "suppress non-result output")
	flags.BoolVarP(&opts.verbose, "verbose", "v", false, "show diagnostics")
	flags.BoolVar(&opts.noColor, "no-color", false, "disable colored output")
	flags.StringVar(&opts.mode, "mode", "", "override configured mode for this invocation")
	flags.StringVar(&opts.timeout, "timeout", "", "override timeout for this invocation")

	root.AddCommand(c.newVersionCommand(opts), c.newConfigCommand(opts))
	c.addReadCommands(root, opts)
	return root
}

func defaultOutputFormat() string {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("ZOT_OUTPUT")))
	if format == "json" {
		return "json"
	}
	return "text"
}

func outputOptions(opts *globalOptions) (app.OutputOptions, error) {
	format := strings.ToLower(strings.TrimSpace(opts.format))
	if opts.json {
		format = "json"
	}
	if format != "text" && format != "json" {
		return app.OutputOptions{}, &exitError{code: ExitUsage, err: fmt.Errorf("invalid output format %q", format)}
	}
	return app.OutputOptions{Format: format, Quiet: opts.quiet, Verbose: opts.verbose, Color: !opts.noColor}, nil
}

func (c *CLI) renderResult(ctx context.Context, opts *globalOptions, path app.CommandPath, run func(context.Context) (app.Result, error)) error {
	output, err := outputOptions(opts)
	if err != nil {
		return err
	}
	result, err := run(ctx)
	if err != nil {
		code := ExitError
		if app.IsConfigNotFound(err) {
			code = ExitConfig
			err = fmt.Errorf("%w.\nrequired fields: library_type, library_id, api_key\nrun `zot init` to set them up interactively in ~/.zot/.env", err)
		} else if strings.Contains(err.Error(), "config already exists") {
			code = ExitConfig
		}
		return &exitError{code: code, err: err}
	}
	return (appRender.Renderer{Out: c.stdout, Err: c.stderr}).Result(path, result, output)
}

func (c *CLI) newVersionCommand(opts *globalOptions) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := app.CommandPath{Resource: "version"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return (app.VersionService{Current: version, Commit: commit, BuildDate: buildDate}).Show(ctx, check)
			})
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "check GitHub for the latest release")
	return cmd
}

func (c *CLI) newConfigCommand(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect and validate configuration"}
	var initReq app.ConfigInitRequest
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize ~/.zot/.env",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			reader := bufio.NewReader(cmd.InOrStdin())
			initReq.Prompt = func(cfg config.Config, provided map[string]bool) (config.Config, error) {
				return c.promptInitSetup(cfg, provided, reader)
			}
			initReq.ConfirmPDF = func() (bool, error) {
				return c.promptBool(reader, "Set up PyMuPDF for PDF extraction now? [Y/n]: ", true)
			}
			path := app.CommandPath{Resource: "config", Action: "init"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return (app.ConfigService{}).Init(ctx, initReq)
			})
		},
	}
	initFlags := initCmd.Flags()
	initFlags.StringVar(&initReq.Mode, "mode", "", "web, local, hybrid, or remote")
	initFlags.StringVar(&initReq.LibraryType, "library-type", "", "user or group")
	initFlags.StringVar(&initReq.LibraryID, "library-id", "", "Zotero library ID")
	initFlags.StringVar(&initReq.APIKey, "api-key", "", "Zotero Web API key")
	initFlags.StringVar(&initReq.DataDir, "data-dir", "", "Zotero data directory")
	initFlags.StringVar(&initReq.ServerAddr, "server-addr", "", "remote zot server address")
	initFlags.BoolVar(&initReq.SetupPDF, "pdf", false, "set up PyMuPDF")
	initFlags.BoolVar(&initReq.NoPDF, "no-pdf", false, "skip PyMuPDF setup")
	initFlags.BoolVar(&initReq.CheckPDF, "check-pdf", false, "check PyMuPDF status")
	initCmd.MarkFlagsMutuallyExclusive("pdf", "no-pdf", "check-pdf")
	var pathOnly bool
	show := &cobra.Command{
		Use:   "show",
		Short: "Show the active masked configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := app.CommandPath{Resource: "config", Action: "show"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return (app.ConfigService{}).Show(ctx, pathOnly)
			})
		},
	}
	show.Flags().BoolVar(&pathOnly, "path", false, "print only the config file path")
	check := &cobra.Command{
		Use:     "check",
		Aliases: []string{"validate"},
		Short:   "Validate configuration and library access",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := app.CommandPath{Resource: "config", Action: "check"}
			return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
				return (app.ConfigService{}).Check(ctx)
			})
		},
	}
	cmd.AddCommand(initCmd, show, check)
	return cmd
}
