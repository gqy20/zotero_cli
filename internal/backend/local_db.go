package backend

import (
	"context"
	"database/sql"
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

	snapshotPath := filepath.Join(snapshotDir, filepath.Base(sqlitePath))
	for _, sourcePath := range []string{
		sqlitePath,
		sqlitePath + "-journal",
		sqlitePath + "-wal",
		sqlitePath + "-shm",
	} {
		if err := copySQLiteFileIfExists(sourcePath, filepath.Join(snapshotDir, filepath.Base(sourcePath))); err != nil {
			_ = os.RemoveAll(snapshotDir)
			return "", "", err
		}
	}
	return snapshotDir, snapshotPath, nil
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

func isSnapshotValid(sqlitePath string, cacheDir string) bool {
	cachedPath := filepath.Join(cacheDir, filepath.Base(sqlitePath))
	cachedInfo, err := os.Stat(cachedPath)
	if err != nil {
		return false
	}
	srcInfo, err := os.Stat(sqlitePath)
	if err != nil {
		return false
	}
	return !srcInfo.ModTime().After(cachedInfo.ModTime())
}

func createOrReuseCachedSnapshot(sqlitePath string, cacheDir string) (string, string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	if isSnapshotValid(sqlitePath, cacheDir) {
		return cacheDir, filepath.Join(cacheDir, filepath.Base(sqlitePath)), nil
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	for _, sourcePath := range []string{
		sqlitePath,
		sqlitePath + "-journal",
		sqlitePath + "-wal",
		sqlitePath + "-shm",
	} {
		if err := copySQLiteFileIfExists(sourcePath, filepath.Join(cacheDir, filepath.Base(sourcePath))); err != nil {
			os.RemoveAll(cacheDir)
			return "", "", err
		}
	}
	return cacheDir, filepath.Join(cacheDir, filepath.Base(sqlitePath)), nil
}

func countRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
