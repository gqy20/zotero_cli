package references

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type statusCacheEntry struct {
	Fingerprint string    `json:"fingerprint"`
	GeneratedAt time.Time `json:"generated_at"`
	Status      Status    `json:"status"`
}

func (s *Store) CachedStatus(ctx context.Context) (Status, bool, error) {
	fingerprint, err := storeFingerprint(s.path)
	if err != nil {
		return Status{}, false, err
	}
	cachePath := s.path + ".status.json"
	if data, readErr := os.ReadFile(cachePath); readErr == nil {
		var entry statusCacheEntry
		if json.Unmarshal(data, &entry) == nil && entry.Fingerprint == fingerprint {
			return entry.Status, true, nil
		}
	}
	status, err := s.Status(ctx)
	if err != nil {
		return Status{}, false, err
	}
	entry := statusCacheEntry{Fingerprint: fingerprint, GeneratedAt: time.Now().UTC(), Status: status}
	if data, marshalErr := json.Marshal(entry); marshalErr == nil {
		_ = writeStatusCache(cachePath, data)
	}
	return status, false, nil
}

func storeFingerprint(path string) (string, error) {
	parts := make([]string, 0, 2)
	for _, candidate := range []string{path, path + "-wal"} {
		info, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) && candidate != path {
				parts = append(parts, filepath.Base(candidate)+":missing")
				continue
			}
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%d", filepath.Base(candidate), info.Size(), info.ModTime().UnixNano()))
	}
	return strings.Join(parts, "|"), nil
}

func writeStatusCache(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ref-status-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, path)
}
