package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"zotero_cli/internal/domain"
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
		openArgs := []string{"/c", "start", ""}
		if page > 0 {
			openArgs = append(openArgs, fmt.Sprintf("%s#page=%d", path, page))
		} else {
			openArgs = append(openArgs, path)
		}
		cmd = exec.Command("cmd", openArgs...)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		return c.printErr(fmt.Errorf("failed to open PDF: %w", err))
	}

	fmt.Fprintf(c.stdout, "Opened: %s\n", path)
	if page > 0 {
		fmt.Fprintf(c.stdout, "Page hint: %d\n", page)
	}
	fmt.Fprintf(c.stdout, "Item: %s (%s)\n", itemKey, item.Title)
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
		case "--attachment":
			nextFlag = "attachment"
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

func (c *CLI) openAttachment(att domain.Attachment, page int) error {
	if att.ResolvedPath == "" {
		return fmt.Errorf("attachment %s path not resolved", att.Key)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		arg := att.ResolvedPath
		if page > 0 {
			arg = fmt.Sprintf("%s#page=%d", att.ResolvedPath, page)
		}
		cmd = exec.Command("cmd", "/c", "start", "", arg)
	case "darwin":
		cmd = exec.Command("open", att.ResolvedPath)
	default:
		cmd = exec.Command("xdg-open", att.ResolvedPath)
	}
	return cmd.Start()
}

func parseIntArg(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
