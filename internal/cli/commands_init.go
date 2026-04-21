package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) runInit(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageInit)
	}

	flags, rest := parseInitFlags(args)
	if len(rest) > 0 {
		fmt.Fprintf(c.stderr, "unknown init flag: %s\n\n", rest[0])
		return c.printCommandUsage(usageInit)
	}

	path, err := config.DefaultPath()
	if err != nil {
		return c.printErr(err)
	}

	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(c.stderr, "config already exists at %s\n", path)
		fmt.Fprintf(c.stderr, "edit it manually, or remove it before re-running init\n")
		return 3
	} else if !os.IsNotExist(err) {
		return c.printErr(err)
	}

	cfg := config.Default()
	if flags.Mode != "" {
		cfg.Mode = flags.Mode
	}
	if flags.LibraryType != "" {
		cfg.LibraryType = flags.LibraryType
	}
	if flags.LibraryID != "" {
		cfg.LibraryID = flags.LibraryID
	}
	if flags.APIKey != "" {
		cfg.APIKey = flags.APIKey
	}
	if flags.DataDir != "" {
		cfg.DataDir = flags.DataDir
	}

	provided := map[string]bool{
		"mode":         flags.Mode != "",
		"library_type": flags.LibraryType != "",
		"library_id":   flags.LibraryID != "",
		"api_key":      flags.APIKey != "",
		"data_dir":     flags.DataDir != "",
	}

	isNonInteractive := provided["mode"] && provided["library_type"] &&
		provided["library_id"] && provided["api_key"]

	reader := bufio.NewReader(c.stdin)

	if isNonInteractive && (cfg.Mode == "web" || provided["data_dir"]) {
		if err := config.Save(cfg); err != nil {
			return c.printErr(err)
		}
		fmt.Fprintf(c.stdout, "created config at %s\n", path)
	} else {
		cfg, err = c.promptInitSetup(cfg, provided, reader)
		if err != nil {
			return c.printErr(err)
		}
		if err := config.Save(cfg); err != nil {
			return c.printErr(err)
		}
		fmt.Fprintf(c.stdout, "created config at %s\n", path)
		fmt.Fprintln(c.stdout, "you can edit ~/.zot/.env later if you want to change keys or permissions")
	}

	if flags.NoPDF {
		return 0
	}
	if cfg.Mode != "local" && cfg.Mode != "hybrid" {
		if flags.SetupPDF {
			fmt.Fprintln(c.stderr, "warning: --pdf flag has no effect in web mode; PyMuPDF is only used for local/hybrid modes")
		}
		return 0
	}
	if cfg.DataDir == "" {
		fmt.Fprintln(c.stdout, "\nTip: run 'zot setup pdf-extract' to set up PyMuPDF after configuring ZOT_DATA_DIR.")
		return 0
	}

	wantPDF := flags.SetupPDF
	if !wantPDF && !isNonInteractive {
		wantPDF, err = c.promptBool(reader, "Set up PyMuPDF for PDF extraction now? [Y/n]: ", true)
		if err != nil {
			return c.printErr(err)
		}
	}
	if !wantPDF {
		fmt.Fprintln(c.stdout, "\nYou can set up PyMuPDF later with: zot setup pdf-extract")
		return 0
	}

	fmt.Fprintln(c.stdout, "\nSetting up PyMuPDF PDF extraction...")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, timeoutCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer timeoutCancel()

	if err := backend.SetupVenv(ctx, cfg.DataDir); err != nil {
		return c.printErr(fmt.Errorf("PyMuPDF setup failed: %w", err))
	}

	status := backend.CheckVenvStatus(cfg.DataDir)
	if !status.HasPyMuPDF {
		return c.printErr(fmt.Errorf("setup completed but PyMuPDF verification failed"))
	}
	fmt.Fprintf(c.stdout, "PyMuPDF setup complete. Python: %s\n", status.PythonPath)
	return 0
}

type initFlags struct {
	Mode        string
	LibraryType string
	LibraryID   string
	APIKey      string
	DataDir     string
	SetupPDF    bool
	NoPDF       bool
}

func parseInitFlags(args []string) (initFlags, []string) {
	var f initFlags
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			if i+1 >= len(args) {
				return f, []string{"--mode"}
			}
			f.Mode = args[i+1]
			i++
		case "--library-type":
			if i+1 >= len(args) {
				return f, []string{"--library-type"}
			}
			f.LibraryType = args[i+1]
			i++
		case "--library-id":
			if i+1 >= len(args) {
				return f, []string{"--library-id"}
			}
			f.LibraryID = args[i+1]
			i++
		case "--api-key":
			if i+1 >= len(args) {
				return f, []string{"--api-key"}
			}
			f.APIKey = args[i+1]
			i++
		case "--data-dir":
			if i+1 >= len(args) {
				return f, []string{"--data-dir"}
			}
			f.DataDir = args[i+1]
			i++
		case "--pdf":
			f.SetupPDF = true
		case "--no-pdf":
			f.NoPDF = true
		default:
			rest = append(rest, args[i])
		}
	}
	return f, rest
}

const usageInit = `usage: zot init [--mode MODE] [--library-type TYPE] [--library-id ID] [--api-key KEY] [--data-dir PATH] [--pdf] [--no-pdf]

Initialize ~/.zot/.env with a streamlined interactive setup.

Options:
  --mode MODE           Operating mode: web | local | hybrid (default: web)
  --library-type TYPE   Library type: user | group
  --library-id ID       Your Zotero library numeric ID
  --api-key KEY         Zotero Web API key
  --data-dir PATH       Zotero local data directory (required for local/hybrid)
  --pdf                 Force PyMuPDF setup after config creation
  --no-pdf              Skip PyMuPDF setup

Provide all required flags for non-interactive mode.
Omit flags for interactive mode with guided prompts.

Examples:
  zot init                              # Interactive guided setup
  zot init --mode hybrid --library-id 123 --api-key abc  # Partial flags, prompts for the rest
  zot init --mode web --library-type user --library-id 123 --api-key key  # Fully non-interactive
`
