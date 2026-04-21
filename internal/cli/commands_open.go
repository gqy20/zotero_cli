package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zotero_cli/internal/config"
)

var zoteroExePath = ""
var zoteroResolved bool

func init() {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			fileExists("C:\\Program Files\\Zotero\\zotero.exe"),
			fileExists(os.Getenv("ProgramFiles") + "\\Zotero\\zotero.exe"),
			fileExists(os.Getenv("ProgramFiles(x86)") + "\\Zotero\\zotero.exe"),
		}
	case "darwin":
		candidates = []string{
			fileExists("/Applications/Zotero.app/Contents/MacOS/zotero"),
			fileExists(filepath.Join(os.Getenv("HOME"), "Applications", "Zotero.app", "Contents", "MacOS", "zotero")),
		}
	default:
		candidates = []string{
			fileExists("/usr/bin/zotero"),
			fileExists("/opt/zotero/zotero"),
			fileExists("/usr/local/bin/zotero"),
			fileExists(filepath.Join(os.Getenv("HOME"), ".local", "share", "zotero", "zotero")),
			fileExists("/snap/bin/zotero"),
			fileExists("/var/lib/flatpak/exports/bin/org.zotero.Zotero"),
		}
	}
	for _, c := range candidates {
		if c != "" {
			zoteroExePath = c
			zoteroResolved = true
			break
		}
	}
}

func resolveZoteroExePath(cfg config.Config) {
	if zoteroResolved {
		return
	}
	if path := findFromRegistry(); path != "" {
		zoteroExePath = path
		zoteroResolved = true
		return
	}
	if path := findFromDataDir(cfg); path != "" {
		zoteroExePath = path
		zoteroResolved = true
		return
	}
	if path := findFromPATH(); path != "" {
		zoteroExePath = path
		zoteroResolved = true
		return
	}
}

func findFromRegistry() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	uninstallPaths := []string{
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, base := range uninstallPaths {
		if path := searchUninstallKey(base); path != "" {
			return path
		}
	}
	return ""
}

func searchUninstallKey(base string) string {
	out, err := exec.Command("reg", "query", base).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "Zotero") || !strings.HasPrefix(base+"\\", line) {
			continue
		}
		subkey := line
		locOut, err := exec.Command("reg", "query", subkey, "/v", "InstallLocation").Output()
		if err != nil {
			continue
		}
		dir := parseRegValue(locOut)
		if dir == "" {
			continue
		}
		for _, candidate := range []string{
			filepath.Join(dir, "zotero.exe"),
			filepath.Join(dir, "Zotero", "zotero.exe"),
		} {
			if fileExists(candidate) != "" {
				return candidate
			}
		}
	}
	return ""
}

func parseRegValue(out []byte) string {
	s := strings.TrimSpace(string(out))
	idx := strings.LastIndex(s, "REG_SZ")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(s[idx+len("REG_SZ"):])
}

func findFromDataDir(cfg config.Config) string {
	if cfg.DataDir == "" {
		return ""
	}
	dataDir := cfg.DataDir
	parent := filepath.Dir(dataDir)
	for _, candidate := range []string{
		filepath.Join(parent, "zotero.exe"),
		filepath.Join(parent, "Zotero", "zotero.exe"),
		filepath.Join(dataDir, "zotero.exe"),
	} {
		if fileExists(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func findFromPATH() string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("where", "zotero.exe")
	default:
		cmd = exec.Command("which", "zotero")
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

	resolveZoteroExePath(cfg)

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
