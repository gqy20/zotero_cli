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
	case "completion":
		return args, true
	case "lib", "item", "coll", "tag", "note", "search", "group", "file", "pdf", "ann", "index":
		return args, true
	case "schema":
		return translateLegacySchema(args), true
	case "server":
		if len(args) > 1 && args[1] == "start" {
			return args, true
		}
		return append([]string{"server", "start"}, args[1:]...), true
	case "sync":
		if len(args) > 1 && args[1] == "pull" {
			return args, true
		}
		return append([]string{"sync", "pull"}, args[1:]...), true
	case "ref":
		return translateReferenceArgs(args)
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
		return append(translated, translateChangesFlags(args[2:])...), true
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
	case "find":
		return append([]string{"item", "find"}, translateFindFlags(args[1:])...), true
	case "show":
		return append([]string{"item", "show"}, args[1:]...), true
	case "supplements":
		return append([]string{"item", "supp"}, args[1:]...), true
	case "export":
		return translateLegacyExport(args[1:]), true
	case "inspect-attachment":
		return translateLegacyFile(args[1:]), true
	case "annotations":
		return translateLegacyAnnotations(args[1:]), true
	case "annotate":
		return translateLegacyAnnotate(args[1:]), true
	case "extract-text":
		return append([]string{"pdf", "text"}, args[1:]...), true
	case "extract-figures":
		return append([]string{"pdf", "figs"}, args[1:]...), true
	case "open":
		return append([]string{"pdf", "open"}, args[1:]...), true
	case "create-item":
		return append([]string{"item", "new"}, translateWriteFlags(args[1:])...), true
	case "update-item":
		return append([]string{"item", "edit"}, translateWriteFlags(args[1:])...), true
	case "delete-item":
		return append([]string{"item", "delete"}, translateWriteFlags(args[1:])...), true
	case "add-tag":
		return translateLegacyTag("tag", args[1:]), true
	case "remove-tag":
		return translateLegacyTag("untag", args[1:]), true
	case "create-collection":
		return append([]string{"coll", "new"}, translateWriteFlags(args[1:])...), true
	case "update-collection":
		return append([]string{"coll", "edit"}, translateWriteFlags(args[1:])...), true
	case "delete-collection":
		return append([]string{"coll", "delete"}, translateWriteFlags(args[1:])...), true
	case "create-search":
		return append([]string{"search", "new"}, translateWriteFlags(args[1:])...), true
	case "update-search":
		return append([]string{"search", "edit"}, translateWriteFlags(args[1:])...), true
	case "delete-search":
		return append([]string{"search", "delete"}, translateWriteFlags(args[1:])...), true
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

func translateReferenceArgs(args []string) ([]string, bool) {
	if len(args) < 2 {
		return args, true
	}
	action := args[1]
	rest := args[2:]
	switch action {
	case "show", "find", "related", "cited", "ctx", "links", "entities", "profile", "build", "resolve", "status":
		return args, true
	case "search":
		return append([]string{"ref", "find"}, rest...), true
	case "cited-by":
		return append([]string{"ref", "cited"}, rest...), true
	case "annotations":
		return append([]string{"ref", "entities"}, rest...), true
	case "retry":
		return append([]string{"ref", "build", "--failed"}, rest...), true
	case "failed":
		return append([]string{"ref", "status", "--failed"}, rest...), true
	case "unsupported":
		return append([]string{"ref", "status", "--unsupported"}, rest...), true
	case "contexts":
		if len(rest) > 0 && rest[0] == "build" {
			return append([]string{"ref", "build", "--contexts"}, rest[1:]...), true
		}
		return append([]string{"ref", "ctx"}, rest...), true
	case "grobid":
		if len(rest) == 0 {
			return []string{"ref", "status", "--grobid"}, true
		}
		switch rest[0] {
		case "status":
			return append([]string{"ref", "status", "--grobid"}, rest[1:]...), true
		case "build":
			return append([]string{"ref", "build", "--grobid"}, rest[1:]...), true
		default:
			return append([]string{"ref", "status", "--grobid"}, rest...), true
		}
	default:
		if strings.HasPrefix(action, "-") {
			return args, true
		}
		return append([]string{"ref", "show"}, args[1:]...), true
	}
}

func translateLegacySchema(args []string) []string {
	if len(args) < 2 || args[1] == "list" || args[1] == "show" {
		return args
	}
	rest := args[2:]
	switch args[1] {
	case "types":
		return append([]string{"schema", "list", "types"}, rest...)
	case "fields":
		return append([]string{"schema", "list", "fields"}, rest...)
	case "creator-types":
		return append([]string{"schema", "list", "roles"}, rest...)
	case "fields-for":
		if len(rest) == 0 {
			return args
		}
		return append([]string{"schema", "list", "fields"}, rest...)
	case "creator-types-for":
		if len(rest) == 0 {
			return args
		}
		return append([]string{"schema", "list", "roles"}, rest...)
	case "template":
		if len(rest) == 0 {
			return args
		}
		return append([]string{"schema", "show"}, rest...)
	default:
		return args
	}
}

func translateLegacyFile(args []string) []string {
	action := "show"
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--health" {
			action = "check"
			continue
		}
		rest = append(rest, arg)
	}
	return append([]string{"file", action}, rest...)
}

func translateLegacyAnnotations(args []string) []string {
	action := "list"
	confirmed := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--clear" {
			action = "delete"
			confirmed = true
			continue
		}
		rest = append(rest, arg)
	}
	if confirmed {
		rest = append(rest, "--yes")
	}
	return append([]string{"ann", action}, rest...)
}

func translateLegacyAnnotate(args []string) []string {
	action := "new"
	confirmed := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "--clear":
			action = "delete"
			confirmed = true
		case arg == "--from-file":
			rest = append(rest, "--from")
		case strings.HasPrefix(arg, "--from-file="):
			rest = append(rest, "--from="+strings.TrimPrefix(arg, "--from-file="))
		default:
			rest = append(rest, arg)
		}
	}
	if confirmed {
		rest = append(rest, "--yes")
	}
	return append([]string{"ann", action}, rest...)
}

func translateLegacyExport(args []string) []string {
	translated := []string{"item", "export"}
	query := make([]string, 0)
	fromFind := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--item-key":
			if i+1 < len(args) {
				i++
				translated = append(translated, args[i])
			}
		case "--format", "-f":
			translated = append(translated, "--as")
			if i+1 < len(args) {
				i++
				translated = append(translated, args[i])
			}
		case "--from-find":
			fromFind = true
		case "--collection", "--limit", "--item-type", "--tag", "--date-after", "--date-before", "--start", "--sort", "--direction", "--qmode", "--include-fields", "--no-type", "--no-collection", "--tag-contains", "--exclude-tag", "--modified-within", "--added-since", "--attachment-name":
			mapped := translateFindFlags([]string{arg})[0]
			translated = append(translated, mapped)
			if i+1 < len(args) {
				i++
				translated = append(translated, args[i])
			}
		default:
			if strings.HasPrefix(arg, "--item-key=") {
				translated = append(translated, strings.TrimPrefix(arg, "--item-key="))
			} else if strings.HasPrefix(arg, "--format=") {
				translated = append(translated, "--as="+strings.TrimPrefix(arg, "--format="))
			} else if strings.HasPrefix(arg, "-") {
				translated = append(translated, translateFindFlags([]string{arg})...)
			} else {
				query = append(query, arg)
			}
		}
	}
	if len(query) > 0 {
		translated = append(translated, "--query", strings.Join(query, " "))
	} else if fromFind {
		translated = append(translated, "--all")
	}
	return translated
}

func translateFindFlags(args []string) []string {
	translated := make([]string, len(args))
	for i, arg := range args {
		switch {
		case arg == "--item-type":
			translated[i] = "--type"
		case strings.HasPrefix(arg, "--item-type="):
			translated[i] = "--type=" + strings.TrimPrefix(arg, "--item-type=")
		case arg == "--start":
			translated[i] = "--offset"
		case strings.HasPrefix(arg, "--start="):
			translated[i] = "--offset=" + strings.TrimPrefix(arg, "--start=")
		case arg == "--direction":
			translated[i] = "--order"
		case strings.HasPrefix(arg, "--direction="):
			translated[i] = "--order=" + strings.TrimPrefix(arg, "--direction=")
		default:
			translated[i] = arg
		}
	}
	return translated
}

func translateWriteFlags(args []string) []string {
	translated := make([]string, len(args))
	for i, arg := range args {
		switch {
		case arg == "--from-file":
			translated[i] = "--from"
		case strings.HasPrefix(arg, "--from-file="):
			translated[i] = "--from=" + strings.TrimPrefix(arg, "--from-file=")
		case arg == "--if-unmodified-since-version":
			translated[i] = "--if-version"
		case strings.HasPrefix(arg, "--if-unmodified-since-version="):
			translated[i] = "--if-version=" + strings.TrimPrefix(arg, "--if-unmodified-since-version=")
		default:
			translated[i] = arg
		}
	}
	return translated
}

func translateLegacyTag(action string, args []string) []string {
	translated := []string{"item", action}
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--items" && i+1 < len(args) {
			i++
			for _, key := range strings.Split(args[i], ",") {
				if key = strings.TrimSpace(key); key != "" {
					translated = append(translated, key)
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "--items=") {
			for _, key := range strings.Split(strings.TrimPrefix(arg, "--items="), ",") {
				if key = strings.TrimSpace(key); key != "" {
					translated = append(translated, key)
				}
			}
			continue
		}
		rest = append(rest, arg)
	}
	return append(translated, translateWriteFlags(rest)...)
}

func translateChangesFlags(args []string) []string {
	translated := make([]string, len(args))
	for i, arg := range args {
		switch {
		case arg == "--if-modified-since-version":
			translated[i] = "--if-modified-version"
		case strings.HasPrefix(arg, "--if-modified-since-version="):
			translated[i] = "--if-modified-version=" + strings.TrimPrefix(arg, "--if-modified-since-version=")
		default:
			translated[i] = arg
		}
	}
	return translated
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
	c.addContentCommands(root, opts)
	root.AddCommand(c.newReferenceCommand(opts))
	root.AddCommand(c.newIndexCommand(opts))
	root.AddCommand(c.newSchemaCommand(opts))
	root.AddCommand(c.newServerCommand(opts))
	root.AddCommand(c.newSyncCommand(opts))
	root.AddCommand(c.newCompletionCommand())
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
