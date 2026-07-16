package backend

import (
	"context"
	cryptoRand "crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

func closeDBAndCleanup(db *sql.DB, cleanup func()) error {
	err := db.Close()
	cleanup()
	return err
}

func localSQLiteDSN(path string) string {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	busyTimeout := 200
	if value := strings.TrimSpace(os.Getenv("ZOT_LOCAL_BUSY_TIMEOUT_MS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			busyTimeout = parsed
		}
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: fmt.Sprintf("mode=ro&_pragma=busy_timeout=%d&_pragma=query_only=1", busyTimeout),
	}).String()
}

func localSQLiteDSNReadWrite(path string) string {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	busyTimeout := 200
	if value := strings.TrimSpace(os.Getenv("ZOT_LOCAL_BUSY_TIMEOUT_MS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			busyTimeout = parsed
		}
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: fmt.Sprintf("mode=rwc&_pragma=busy_timeout=%d&_pragma=journal_mode=WAL", busyTimeout),
	}).String()
}

func openSQLiteDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func isSQLiteRetryableReadError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "SQLITE_BUSY") ||
		strings.Contains(message, "SQLITE_LOCKED") ||
		strings.Contains(strings.ToLower(message), "database is locked") ||
		strings.Contains(strings.ToLower(message), "database is busy")
}

func createSQLiteSnapshot(sqlitePath string) (string, string, error) {
	snapshotDir, err := os.MkdirTemp("", "zot-local-snapshot-*")
	if err != nil {
		return "", "", err
	}

	if err := copySQLiteSnapshotFiles(sqlitePath, snapshotDir); err != nil {
		_ = os.RemoveAll(snapshotDir)
		return "", "", err
	}
	return snapshotDir, filepath.Join(snapshotDir, filepath.Base(sqlitePath)), nil
}

func copySQLiteFileIfExists(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := target.ReadFrom(source); err != nil {
		return err
	}
	return target.Close()
}

const (
	snapshotManifestVersion = 1
	snapshotCopyMaxAttempts = 3
	snapshotManifestName    = "manifest.json"
)

type snapshotFileFingerprint struct {
	Exists          bool  `json:"exists"`
	Size            int64 `json:"size,omitempty"`
	ModTimeUnixNano int64 `json:"mtime_unix_nano,omitempty"`
}

type snapshotSourceFingerprint struct {
	Database snapshotFileFingerprint `json:"database"`
	WAL      snapshotFileFingerprint `json:"wal"`
	Journal  snapshotFileFingerprint `json:"journal"`
}

type snapshotManifest struct {
	Version     int                       `json:"version"`
	SourcePath  string                    `json:"source_path"`
	Generation  string                    `json:"generation"`
	Fingerprint snapshotSourceFingerprint `json:"fingerprint"`
}

func copySQLiteSnapshotFiles(sqlitePath, targetDir string) error {
	for _, sourcePath := range []string{
		sqlitePath,
		sqlitePath + "-journal",
		sqlitePath + "-wal",
		sqlitePath + "-shm",
	} {
		if err := copySQLiteFileIfExists(sourcePath, filepath.Join(targetDir, filepath.Base(sourcePath))); err != nil {
			return err
		}
	}
	return nil
}

func sourceSnapshotFingerprint(sqlitePath string) (snapshotSourceFingerprint, error) {
	database, err := snapshotFileFingerprintFor(sqlitePath, true)
	if err != nil {
		return snapshotSourceFingerprint{}, err
	}
	wal, err := snapshotFileFingerprintFor(sqlitePath+"-wal", false)
	if err != nil {
		return snapshotSourceFingerprint{}, err
	}
	journal, err := snapshotFileFingerprintFor(sqlitePath+"-journal", false)
	if err != nil {
		return snapshotSourceFingerprint{}, err
	}
	return snapshotSourceFingerprint{Database: database, WAL: wal, Journal: journal}, nil
}

func snapshotFileFingerprintFor(path string, required bool) (snapshotFileFingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return snapshotFileFingerprint{}, nil
		}
		return snapshotFileFingerprint{}, err
	}
	if info.IsDir() {
		return snapshotFileFingerprint{}, fmt.Errorf("snapshot source is a directory: %s", path)
	}
	return snapshotFileFingerprint{Exists: true, Size: info.Size(), ModTimeUnixNano: info.ModTime().UnixNano()}, nil
}

func readSnapshotManifest(cacheDir string) (snapshotManifest, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, snapshotManifestName))
	if err != nil {
		return snapshotManifest{}, err
	}
	var manifest snapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return snapshotManifest{}, err
	}
	if manifest.Version != snapshotManifestVersion {
		return snapshotManifest{}, fmt.Errorf("unsupported snapshot manifest version %d", manifest.Version)
	}
	if !validSnapshotGeneration(manifest.Generation) {
		return snapshotManifest{}, fmt.Errorf("invalid snapshot generation %q", manifest.Generation)
	}
	return manifest, nil
}

func validSnapshotGeneration(generation string) bool {
	return strings.HasPrefix(generation, "generation-") && filepath.Base(generation) == generation
}

func snapshotPathForManifest(sqlitePath, cacheDir string, manifest snapshotManifest) string {
	return filepath.Join(cacheDir, manifest.Generation, filepath.Base(sqlitePath))
}

func cachedSnapshotPath(sqlitePath, cacheDir string) (string, string, bool) {
	if manifest, err := readSnapshotManifest(cacheDir); err == nil {
		path := snapshotPathForManifest(sqlitePath, cacheDir, manifest)
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			return filepath.Dir(path), path, true
		}
	}
	legacyPath := filepath.Join(cacheDir, filepath.Base(sqlitePath))
	if info, err := os.Stat(legacyPath); err == nil && !info.IsDir() {
		return cacheDir, legacyPath, true
	}
	return "", "", false
}

func isSnapshotValid(sqlitePath string, cacheDir string) bool {
	manifest, err := readSnapshotManifest(cacheDir)
	if err != nil || filepath.Clean(manifest.SourcePath) != filepath.Clean(sqlitePath) {
		return false
	}
	current, err := sourceSnapshotFingerprint(sqlitePath)
	if err != nil || current != manifest.Fingerprint {
		return false
	}
	return snapshotFilesMatchFingerprint(snapshotPathForManifest(sqlitePath, cacheDir, manifest), manifest.Fingerprint)
}

func snapshotFilesMatchFingerprint(snapshotPath string, fingerprint snapshotSourceFingerprint) bool {
	checks := []struct {
		path string
		want snapshotFileFingerprint
	}{
		{path: snapshotPath, want: fingerprint.Database},
		{path: snapshotPath + "-wal", want: fingerprint.WAL},
		{path: snapshotPath + "-journal", want: fingerprint.Journal},
	}
	for _, check := range checks {
		info, err := os.Stat(check.path)
		if !check.want.Exists {
			if err == nil || !os.IsNotExist(err) {
				return false
			}
			continue
		}
		if err != nil || info.IsDir() || info.Size() != check.want.Size {
			return false
		}
	}
	return true
}

func createOrReuseCachedSnapshot(sqlitePath string, cacheDir string) (string, string, error) {
	return createOrReuseCachedSnapshotWithCopy(sqlitePath, cacheDir, copySQLiteSnapshotFiles)
}

func createOrReuseCachedSnapshotWithCopy(sqlitePath string, cacheDir string, copyFiles func(string, string) error) (string, string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	if isSnapshotValid(sqlitePath, cacheDir) {
		manifest, _ := readSnapshotManifest(cacheDir)
		path := snapshotPathForManifest(sqlitePath, cacheDir, manifest)
		return filepath.Dir(path), path, nil
	}

	oldDir, oldPath, hasOldSnapshot := cachedSnapshotPath(sqlitePath, cacheDir)
	oldManifest, _ := readSnapshotManifest(cacheDir)
	var lastErr error
	for attempt := 1; attempt <= snapshotCopyMaxAttempts; attempt++ {
		before, err := sourceSnapshotFingerprint(sqlitePath)
		if err != nil {
			lastErr = err
			break
		}
		stagingDir, err := os.MkdirTemp(cacheDir, ".staging-")
		if err != nil {
			lastErr = err
			break
		}
		if err := copyFiles(sqlitePath, stagingDir); err != nil {
			lastErr = err
			_ = os.RemoveAll(stagingDir)
			continue
		}
		after, err := sourceSnapshotFingerprint(sqlitePath)
		if err != nil {
			lastErr = err
			_ = os.RemoveAll(stagingDir)
			continue
		}
		if before != after {
			lastErr = fmt.Errorf("SQLite source changed while creating snapshot (attempt %d/%d)", attempt, snapshotCopyMaxAttempts)
			_ = os.RemoveAll(stagingDir)
			continue
		}

		generation := "generation-" + strings.TrimPrefix(filepath.Base(stagingDir), ".staging-")
		generationDir := filepath.Join(cacheDir, generation)
		if err := os.Rename(stagingDir, generationDir); err != nil {
			lastErr = err
			_ = os.RemoveAll(stagingDir)
			continue
		}
		manifest := snapshotManifest{
			Version:     snapshotManifestVersion,
			SourcePath:  filepath.Clean(sqlitePath),
			Generation:  generation,
			Fingerprint: after,
		}
		if err := writeSnapshotManifest(cacheDir, manifest); err != nil {
			lastErr = err
			_ = os.RemoveAll(generationDir)
			continue
		}
		cleanupPreviousSnapshot(sqlitePath, cacheDir, oldDir, oldManifest.Generation, generation)
		path := filepath.Join(generationDir, filepath.Base(sqlitePath))
		return generationDir, path, nil
	}

	if hasOldSnapshot {
		return oldDir, oldPath, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to create SQLite snapshot")
	}
	return "", "", lastErr
}

func writeSnapshotManifest(cacheDir string, manifest snapshotManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(cacheDir, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(cacheDir, snapshotManifestName)
	if err := os.Rename(tmpPath, manifestPath); err == nil {
		return nil
	}
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, manifestPath)
}

func cleanupPreviousSnapshot(sqlitePath, cacheDir, oldDir, oldGeneration, newGeneration string) {
	if validSnapshotGeneration(oldGeneration) && oldGeneration != newGeneration {
		_ = os.RemoveAll(filepath.Join(cacheDir, oldGeneration))
	} else if oldDir == cacheDir {
		for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
			_ = os.Remove(filepath.Join(cacheDir, filepath.Base(sqlitePath)+suffix))
		}
	}
}

func countRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

var base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func generateItemKey() (string, error) {
	b := make([]byte, 5)
	if _, err := cryptoRand.Read(b); err != nil {
		return "", err
	}
	result := make([]byte, 8)
	for i := range result {
		if i < 5 {
			result[i] = base32Alphabet[b[i]&31]
		} else {
			// Combine remaining bytes for the last 3 chars (Zotero uses 5 bytes → 8 chars)
			combined := uint(b[3])<<10 | uint(b[4])<<2
			shift := uint((i - 5) * 5)
			result[i] = base32Alphabet[(combined>>shift)&31]
		}
	}
	return string(result), nil
}
