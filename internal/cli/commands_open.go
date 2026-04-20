package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var zoteroExePath = ""

func init() {
	candidates := []string{
		fileExists("C:\\Program Files\\Zotero\\zotero.exe"),
		fileExists(os.Getenv("ProgramFiles") + "\\Zotero\\zotero.exe"),
	}
	for _, c := range candidates {
		if c != "" {
			zoteroExePath = c
			break
		}
	}
}

func fileExists(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func isZoteroRunning() bool {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq zotero.exe", "/NH").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "zotero.exe")
	case "darwin":
		out, err := exec.Command("pgrep", "-x", "Zotero").Output()
		return err == nil && len(out) > 0
	default:
		out, err := exec.Command("pgrep", "-x", "zotero").Output()
		return err == nil && len(out) > 0
	}
}

func (c *CLI) runOpen(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageOpen)
	}

	itemKey, page, ok := c.parseOpenArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		return c.printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}

	pdfs := filterPDFAttachments(item.Attachments)
	if len(pdfs) == 0 {
		return c.printErr(fmt.Errorf("item %s has no PDF attachment", itemKey))
	}

	attachmentKey := pdfs[0].Key
	path := pdfs[0].ResolvedPath
	if path == "" {
		return c.printErr(fmt.Errorf("PDF path not resolved for item %s", itemKey))
	}

	var cmd *exec.Cmd
	if isZoteroRunning() {
		uri := fmt.Sprintf("zotero://open-pdf/library/items/%s", attachmentKey)
		if page > 0 {
			uri += fmt.Sprintf("?page=%d", page)
		}
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", uri)
		case "darwin":
			cmd = exec.Command("open", uri)
		default:
			cmd = exec.Command("xdg-open", uri)
		}
	} else {
		switch runtime.GOOS {
		case "windows":
			if zoteroExePath != "" {
				cmd = exec.Command(zoteroExePath, "--browser", path)
			} else {
				cmd = exec.Command("cmd", "/c", "start", "", path)
			}
		case "darwin":
			cmd = exec.Command("open", path)
		default:
			cmd = exec.Command("xdg-open", path)
		}
	}

	if err := cmd.Start(); err != nil {
		return c.printErr(fmt.Errorf("failed to open PDF: %w", err))
	}

	fmt.Fprintf(c.stdout, "Opened: %s\n", path)
	fmt.Fprintf(c.stdout, "Item: %s (%s)\n", itemKey, item.Title)
	if page > 0 {
		fmt.Fprintf(c.stdout, "Page hint: %d\n", page)
	}
	return 0
}

func (c *CLI) parseOpenArgs(args []string) (string, int, bool) {
	itemKey := ""
	page := 0
	nextFlag := ""

	for _, arg := range args {
		if nextFlag != "" {
			if nextFlag == "page" {
				n, err := parseIntArg(arg)
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageOpen)
					return "", 0, false
				}
				page = n
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--page":
			nextFlag = "page"
		default:
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				fmt.Fprintln(c.stderr, usageOpen)
				return "", 0, false
			}
			if strings.HasPrefix(arg, "--page=") {
				n, err := parseIntArg(strings.TrimPrefix(arg, "--page="))
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageOpen)
					return "", 0, false
				}
				page = n
			} else if itemKey != "" {
				fmt.Fprintln(c.stderr, usageOpen)
				return "", 0, false
			} else {
				itemKey = arg
			}
		}
	}

	if itemKey == "" {
		fmt.Fprintln(c.stderr, usageOpen)
		return "", 0, false
	}
	return itemKey, page, true
}

func parseIntArg(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
