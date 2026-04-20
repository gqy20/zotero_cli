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
		cmd = openWithZotero(path, page)
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
		fmt.Fprintf(c.stdout, "Page hint: %d (press Ctrl+G in reader)\n", page)
	}
	return 0
}

func openWithZotero(path string, page int) *exec.Cmd {
	if zoteroExePath == "" {
		return exec.Command("cmd", "/c", "start", "", path)
	}
	fileURI := toFileURI(path)
	if page > 0 {
		fileURI += fmt.Sprintf("#page=%d", page)
	}
	return exec.Command(zoteroExePath, "--browser", fileURI)
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

func toFileURI(path string) string {
	s := strings.ReplaceAll(path, "\\", "/")
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	s = strings.ReplaceAll(s, " ", "%20")
	return "file://" + s
}
