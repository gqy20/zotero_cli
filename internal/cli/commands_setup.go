package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zotero_cli/internal/backend"
)

func (c *CLI) runSetup(args []string) int {
	if len(args) == 0 {
		c.printSetupUsage()
		return 0
	}
	switch args[0] {
	case "pdf-extract":
		return c.runSetupPdfExtract(args[1:])
	case "help", "-h", "--help":
		c.printSetupUsage()
		return 0
	default:
		fmt.Fprintf(c.stderr, "unknown setup command: %s\n\n", args[0])
		c.printSetupUsage()
		return 2
	}
}

func (c *CLI) printSetupUsage() {
	fmt.Fprint(c.stdout, `Usage:
  zot setup pdf-extract [--check]

Setup Commands:
  pdf-extract   Set up Python venv with PyMuPDF for high-quality PDF text extraction

Options:
  --check       Report current status without making changes

Examples:
  zot setup pdf-extract          # Create venv and install pymupdf
  zot setup pdf-extract --check  # Show current PyMuPDF readiness status
`)
}

func (c *CLI) runSetupPdfExtract(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageSetupPdfExtract)
	}

	checkOnly := false
	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		default:
			fmt.Fprintf(c.stderr, "%s\n", usageSetupPdfExtract)
			return 2
		}
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}
	if cfg.DataDir == "" {
		fmt.Fprintln(c.stderr, "error: ZOT_DATA_DIR is required; run 'zot config init' first")
		return 3
	}

	if checkOnly {
		return c.reportPdfExtractStatus(cfg.DataDir)
	}
	return c.performPdfExtractSetup(cfg.DataDir)
}

func (c *CLI) reportPdfExtractStatus(dataDir string) int {
	status := backend.CheckVenvStatus(dataDir)

	fmt.Fprintln(c.stdout, "PDF Text Extraction Status:")
	fmt.Fprintf(c.stdout, "  Data directory:    %s\n", dataDir)
	fmt.Fprintf(c.stdout, "  Venv path:         %s\n", status.VenvPath)
	fmt.Fprintf(c.stdout, "  uv available:      %t\n", status.HasUV)

	if status.PythonPath != "" {
		fmt.Fprintf(c.stdout, "  Python:            %s\n", status.PythonPath)
		fmt.Fprintf(c.stdout, "  PyMuPDF installed: %t\n", status.HasPyMuPDF)
	} else {
		fmt.Fprintln(c.stdout, "  Python:            (not found)")
		fmt.Fprintln(c.stdout, "  PyMuPDF installed: false")
	}

	if status.Error != "" {
		fmt.Fprintf(c.stdout, "  Error:             %s\n", status.Error)
	}

	if status.SetupNeeded {
		fmt.Fprintln(c.stdout, "\nRun 'zot setup pdf-extract' to set up PyMuPDF.")
		if !status.HasUV {
			fmt.Fprintln(c.stdout, "Tip: install 'uv' from https://docs.astral.sh/uv/ for faster setup.")
		}
	} else {
		fmt.Fprintln(c.stdout, "\nPyMuPDF extraction is ready.")
	}

	return c.writeJSON(map[string]any{
		"ok":           !status.SetupNeeded,
		"command":      "setup-pdf-extract-check",
		"data_dir":     dataDir,
		"venv_path":    status.VenvPath,
		"python_path":  status.PythonPath,
		"has_uv":       status.HasUV,
		"has_pymupdf":  status.HasPyMuPDF,
		"setup_needed": status.SetupNeeded,
		"error":        status.Error,
	})
}

func (c *CLI) performPdfExtractSetup(dataDir string) int {
	fmt.Fprintf(c.stdout, "Setting up PyMuPDF PDF extraction...\n")
	fmt.Fprintf(c.stdout, "  Data dir: %s\n", dataDir)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer timeoutCancel()

	if err := backend.SetupVenv(ctx, dataDir); err != nil {
		return c.printErr(fmt.Errorf("setup failed: %w", err))
	}

	status := backend.CheckVenvStatus(dataDir)
	if !status.HasPyMuPDF {
		return c.printErr(fmt.Errorf("setup completed but PyMuPDF verification failed"))
	}

	fmt.Fprintln(c.stdout, "PyMuPDF setup complete.")
	fmt.Fprintf(c.stdout, "  Python: %s\n", status.PythonPath)
	fmt.Fprintln(c.stdout, "PDF text extraction will now use PyMuPDF as the default extractor.")

	return 0
}
