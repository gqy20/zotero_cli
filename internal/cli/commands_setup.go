package cli

import (
	"fmt"

	"zotero_cli/internal/backend"
)

func (c *CLI) runSetup(args []string) int {
	if len(args) == 0 || args[0] != "pdf-extract" {
		fmt.Fprintln(c.stderr, "`zot setup pdf-extract` has been replaced by `zot init --pdf`")
		fmt.Fprintln(c.stderr, "run `zot init --mode hybrid --api-key KEY --library-id ID` to set up config and PyMuPDF in one step")
		return 2
	}
	return c.runSetupPdfExtract(args[1:])
}

func (c *CLI) runSetupPdfExtract(args []string) int {
	if isHelpOnly(args) {
		fmt.Fprintln(c.stdout, "usage: zot setup pdf-extract [--check]")
		fmt.Fprintln(c.stdout, "")
		fmt.Fprintln(c.stdout, "Note: this command is deprecated. Use `zot init --pdf` for installation,")
		fmt.Fprintln(c.stdout, "      or `zot init --check-pdf` to check PyMuPDF status.")
		return 0
	}

	checkOnly := false
	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		default:
			fmt.Fprintf(c.stderr, "unknown flag: %s\n", arg)
			return 2
		}
	}

	if !checkOnly {
		fmt.Fprintln(c.stderr, "`zot setup pdf-extract` (install mode) has been replaced by `zot init --pdf`")
		fmt.Fprintln(c.stderr, "run `zot init --pdf` to install PyMuPDF as part of initialization.")
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}
	if cfg.DataDir == "" {
		fmt.Fprintln(c.stderr, "error: ZOT_DATA_DIR is required; run 'zot init' first")
		return 3
	}

	return c.reportPdfExtractStatus(cfg.DataDir)
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
		fmt.Fprintln(c.stdout, "\nRun 'zot init --pdf' to install PyMuPDF.")
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
