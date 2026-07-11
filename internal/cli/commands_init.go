package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const usageInit = `usage: zot config init [--mode MODE] [--library-type TYPE] [--library-id ID] [--api-key KEY] [--data-dir PATH] [--pdf] [--no-pdf] [--check-pdf]

Initialize ~/.zot/.env. The legacy shortcut 'zot init' maps to this command.`

func discoverDataDir() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	prefsDir := os.Getenv("APPDATA")
	if prefsDir == "" {
		return ""
	}
	pattern := filepath.Join(prefsDir, "Zotero", "Zotero", "Profiles", "*", "prefs.js")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		re := regexp.MustCompile(`^user_pref\("extensions\.zotero\.dataDir",\s*(.+)\);$`)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			m := re.FindStringSubmatch(line)
			if len(m) == 2 {
				unquoted, err := strconv.Unquote(strings.TrimSpace(m[1]))
				if err == nil && unquoted != "" {
					if _, err := os.Stat(filepath.Join(unquoted, "zotero.sqlite")); err == nil {
						return unquoted
					}
				}
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaultDir := filepath.Join(home, "Zotero")
	if _, err := os.Stat(filepath.Join(defaultDir, "zotero.sqlite")); err == nil {
		return defaultDir
	}
	return ""
}
