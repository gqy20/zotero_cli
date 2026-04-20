package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

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

	path := pdfs[0].ResolvedPath
	if path == "" {
		return c.printErr(fmt.Errorf("PDF path not resolved for item %s", itemKey))
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		return c.printErr(fmt.Errorf("failed to open PDF: %w", err))
	}

	fmt.Fprintf(c.stdout, "Opened: %s\n", path)
	fmt.Fprintf(c.stdout, "Item: %s (%s)\n", itemKey, item.Title)
	if page > 0 {
		fmt.Fprintf(c.stdout, "Page hint: %d (navigate manually in viewer)\n", page)
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
