package backend

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type zoteroPrefs struct {
	DataDir            string
	BaseAttachmentPath string
}

func loadMatchingZoteroPrefs(dataDir string) (zoteroPrefs, string, error) {
	paths, err := findZoteroPrefsFunc()
	if err != nil {
		return zoteroPrefs{}, "", err
	}
	if len(paths) == 0 {
		return zoteroPrefs{}, "", nil
	}

	targetDataDir := ""
	if dataDir != "" {
		targetDataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return zoteroPrefs{}, "", err
		}
	}

	var fallback zoteroPrefs
	var fallbackPath string
	for _, prefsPath := range paths {
		prefs, err := parseZoteroPrefs(prefsPath)
		if err != nil {
			continue
		}
		if fallbackPath == "" {
			fallback = prefs
			fallbackPath = prefsPath
		}
		if targetDataDir == "" {
			continue
		}
		prefsDataDir := prefs.DataDir
		if prefsDataDir == "" {
			continue
		}
		prefsDataDir, err = filepath.Abs(prefsDataDir)
		if err != nil {
			continue
		}
		if sameFilePath(prefsDataDir, targetDataDir) {
			return prefs, prefsPath, nil
		}
	}
	if targetDataDir != "" {
		return zoteroPrefs{}, "", nil
	}
	return fallback, fallbackPath, nil
}

func findZoteroPrefs() ([]string, error) {
	patterns := []string{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		patterns = append(patterns, filepath.Join(appData, "Zotero", "Zotero", "Profiles", "*", "prefs.js"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		patterns = append(patterns,
			filepath.Join(home, ".zotero", "zotero", "Profiles", "*", "prefs.js"),
			filepath.Join(home, "Library", "Application Support", "Zotero", "Profiles", "*", "prefs.js"),
		)
	}
	seen := map[string]struct{}{}
	paths := []string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func parseZoteroPrefs(path string) (zoteroPrefs, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return zoteroPrefs{}, err
	}
	prefs := zoteroPrefs{}
	pattern := regexp.MustCompile(`^user_pref\("([^"]+)",\s*(.+)\);$`)
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		key := matches[1]
		value := strings.TrimSpace(matches[2])
		switch key {
		case "extensions.zotero.dataDir":
			parsed, err := parseZoteroPrefString(value)
			if err != nil {
				continue
			}
			prefs.DataDir = parsed
		case "extensions.zotero.baseAttachmentPath":
			parsed, err := parseZoteroPrefString(value)
			if err != nil {
				continue
			}
			prefs.BaseAttachmentPath = parsed
		}
	}
	return prefs, nil
}

func parseZoteroPrefString(value string) (string, error) {
	return strconv.Unquote(value)
}

func sameFilePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
