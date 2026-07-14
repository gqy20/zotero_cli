package syncmirror

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MetadataDir          = ".zotero_cli"
	LinkedDir            = "linked"
	AttachmentMapFile    = "attachment-map.json"
	AttachmentMapVersion = 1
)

type AttachmentEntry struct {
	Key             string `json:"key"`
	Name            string `json:"name,omitempty"`
	RelativePath    string `json:"relative_path,omitempty"`
	Size            int64  `json:"size,omitempty"`
	Mtime           int64  `json:"mtime,omitempty"`
	SourceAvailable bool   `json:"source_available"`
	Stale           bool   `json:"stale,omitempty"`
	Error           string `json:"error,omitempty"`
}

type AttachmentMap struct {
	Version     int                        `json:"version"`
	Attachments map[string]AttachmentEntry `json:"attachments"`
}

func MapPath(dataDir string) string {
	return filepath.Join(dataDir, MetadataDir, AttachmentMapFile)
}

func LinkedRelativePath(key, name string) string {
	return filepath.ToSlash(filepath.Join(MetadataDir, LinkedDir, key, name))
}

func Load(dataDir string) (AttachmentMap, bool, error) {
	data, err := os.ReadFile(MapPath(dataDir))
	if os.IsNotExist(err) {
		return AttachmentMap{Version: AttachmentMapVersion, Attachments: map[string]AttachmentEntry{}}, false, nil
	}
	if err != nil {
		return AttachmentMap{}, false, err
	}
	var manifest AttachmentMap
	if err := json.Unmarshal(data, &manifest); err != nil {
		return AttachmentMap{}, false, err
	}
	if manifest.Attachments == nil {
		manifest.Attachments = map[string]AttachmentEntry{}
	}
	if manifest.Version != 0 && manifest.Version != AttachmentMapVersion {
		return AttachmentMap{}, false, fmt.Errorf("unsupported attachment map version %d", manifest.Version)
	}
	if manifest.Version == 0 {
		manifest.Version = AttachmentMapVersion
	}
	for key, entry := range manifest.Attachments {
		if err := ValidateEntry(key, entry); err != nil {
			return AttachmentMap{}, false, err
		}
	}
	return manifest, true, nil
}

func Resolve(dataDir string, entry AttachmentEntry) (string, bool) {
	if strings.TrimSpace(entry.RelativePath) == "" {
		return "", false
	}
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return "", false
	}
	path, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(entry.RelativePath)))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func ValidateEntry(key string, entry AttachmentEntry) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(entry.Key) == "" || key != entry.Key {
		return fmt.Errorf("attachment map key mismatch for %q", key)
	}
	return nil
}
