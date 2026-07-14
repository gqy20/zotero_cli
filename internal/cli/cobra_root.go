package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/config"
	appRender "zotero_cli/internal/render"
)

type globalOptions struct {
	format  string
	json    bool
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

func expandShortcutArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	if len(args) >= 3 && args[0] == "lib" && args[1] == "taste" {
		switch args[2] {
		case "init":
			return append([]string{"lib", "taste", "--init"}, args[3:]...)
		case "path":
			return append([]string{"lib", "taste", "--path"}, args[3:]...)
		}
	}
	switch args[0] {
	case "find":
		return append([]string{"item", "find"}, args[1:]...)
	case "show":
		return append([]string{"item", "show"}, args[1:]...)
	case "export":
		return append([]string{"item", "export"}, args[1:]...)
	default:
		return args
	}
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
	} else if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "requires") || strings.Contains(err.Error(), "accepts") {
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
		Use:   "zot",
		Short: "Work with a Zotero library",
		Long: `Work with a Zotero library.

Common shortcuts:
  zot find QUERY       Same as zot item find QUERY
  zot show KEY         Same as zot item show KEY
  zot export [KEY...]  Same as zot item export [KEY...]

Library preferences:
  zot lib taste        Show the current library taste
  zot lib taste init   Create a starter taste.md
  zot lib taste path   Show the resolved taste.md path`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonOutputRequested(opts) {
				return &exitError{code: ExitUsage, err: app.NewUsageError("root help is text-only; run `zot --help` without JSON output")}
			}
			return cmd.Help()
		},
	}
	root.SetIn(c.stdin)
	root.SetOut(c.stdout)
	root.SetErr(c.stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&opts.format, "format", opts.format, "output format: text or json")
	flags.BoolVar(&opts.json, "json", false, "output JSON")
	flags.BoolVarP(&opts.verbose, "verbose", "v", false, "show diagnostics")
	flags.BoolVar(&opts.noColor, "no-color", false, "disable colored output")
	flags.StringVar(&opts.mode, "mode", "", "override configured mode for this invocation")
	flags.StringVar(&opts.timeout, "timeout", "", "override timeout for this invocation")

	root.AddCommand(c.newVersionCommand(opts), c.newConfigCommand(opts))
	c.addReadCommands(root, opts)
	c.addContentCommands(root, opts)
	root.AddCommand(c.newReferenceCommand(opts))
	root.AddCommand(c.newIndexCommand(opts))
	root.AddCommand(c.newSchemaCommand(opts))
	root.AddCommand(c.newServeCommand(opts))
	root.AddCommand(c.newSyncCommand(opts))
	root.AddCommand(c.newCompletionCommand(opts))
	installGroupHelpRunners(root, opts)
	return root
}

func jsonOutputRequested(opts *globalOptions) bool {
	output, err := outputOptions(opts)
	return err == nil && output.Format == "json"
}

func installGroupHelpRunners(root *cobra.Command, opts *globalOptions) {
	for _, cmd := range root.Commands() {
		installGroupHelpRunners(cmd, opts)
		if !cmd.HasSubCommands() || cmd.Run != nil || cmd.RunE != nil {
			continue
		}
		cmd.RunE = func(current *cobra.Command, _ []string) error {
			if jsonOutputRequested(opts) {
				return &exitError{code: ExitUsage, err: app.NewUsageError(current.CommandPath() + " help is text-only; use --help without JSON output")}
			}
			return current.Help()
		}
	}
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
	return app.OutputOptions{Format: format, Verbose: opts.verbose, Color: !opts.noColor}, nil
}

func (c *CLI) renderResult(ctx context.Context, opts *globalOptions, path app.CommandPath, run func(context.Context) (app.Result, error)) error {
	output, err := outputOptions(opts)
	if err != nil {
		return err
	}
	if rawMode := strings.ToLower(strings.TrimSpace(opts.mode)); rawMode != "" {
		switch rawMode {
		case "web", "local", "hybrid", "remote":
		default:
			return &exitError{code: ExitUsage, err: fmt.Errorf("invalid mode %q; expected web, local, hybrid, or remote", rawMode)}
		}
		oldMode, hadMode := os.LookupEnv("ZOT_MODE")
		if err := os.Setenv("ZOT_MODE", rawMode); err != nil {
			return err
		}
		defer func() {
			if hadMode {
				_ = os.Setenv("ZOT_MODE", oldMode)
			} else {
				_ = os.Unsetenv("ZOT_MODE")
			}
		}()
	}
	if raw := strings.TrimSpace(opts.timeout); raw != "" {
		timeout, parseErr := time.ParseDuration(raw)
		if parseErr != nil || timeout <= 0 {
			return &exitError{code: ExitUsage, err: fmt.Errorf("invalid timeout %q; use a positive duration such as 30s or 10m", raw)}
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	result, err := run(ctx)
	if err != nil {
		code := ExitError
		if app.IsConfigNotFound(err) {
			code = ExitConfig
			err = fmt.Errorf("%w.\nrequired fields: library_type, library_id, api_key\nrun `zot init` to set them up interactively in ~/.zot/.env", err)
		} else if strings.Contains(err.Error(), "config already exists") {
			code = ExitConfig
		} else if app.IsUsageError(err) {
			code = ExitUsage
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
			output, err := outputOptions(opts)
			if err != nil {
				return err
			}
			initReq.Prompt = nil
			initReq.ConfirmPDF = nil
			if output.Format != "json" {
				reader := bufio.NewReader(cmd.InOrStdin())
				initReq.Prompt = func(cfg config.Config, provided map[string]bool) (config.Config, error) {
					return c.promptInitSetup(cfg, provided, reader)
				}
				initReq.ConfirmPDF = func() (bool, error) {
					return c.promptBool(reader, "Set up PyMuPDF for PDF extraction now? [Y/n]: ", true)
				}
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
		Use:   "check",
		Short: "Validate configuration and library access",
		Args:  cobra.NoArgs,
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
