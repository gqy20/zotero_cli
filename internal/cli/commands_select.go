package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const usageSelect = `usage: zot select <item-key>

What: Highlight the item in the Zotero desktop UI (Windows / macOS). If Zotero
is not running, this command exits with an error. The remote Zotero web
interface is NOT targeted by this command — use 'open' for the PDF.

Notes:
  - Requires Zotero desktop to be running.
  - See also: open <key> (opens the attached PDF in the system viewer).`

func (c *CLI) runSelect(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageSelect)
	}

	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		fmt.Fprintln(c.stderr, usageSelect)
		return 2
	}

	itemKey := args[0]
	zoteroURI := fmt.Sprintf("zotero://select/library/items/%s", itemKey)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", zoteroURI)
	case "darwin":
		cmd = exec.Command("open", zoteroURI)
	default:
		cmd = exec.Command("xdg-open", zoteroURI)
	}

	if err := cmd.Start(); err != nil {
		return c.printErr(fmt.Errorf("failed to select item: %w", err))
	}

	fmt.Fprintf(c.stdout, "Selected: %s\n", itemKey)
	return 0
}
