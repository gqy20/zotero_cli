package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/safepath"
	"zotero_cli/internal/syncmirror"

	_ "modernc.org/sqlite"
)

const (
	syncStateVersion = 1
	syncStateFile    = "sync-state.json"
	syncManifestFile = "sync-manifest.json"
	syncMetadataDir  = ".zotero_cli"
	syncStateRunning = "running"
	syncStateSuccess = "success"
	syncStateFailed  = "failed"
	maxSyncProblems  = 20
)

type SyncStatusRequest struct {
	Full bool
}

type SyncState struct {
	Version       int          `json:"version"`
	ServerAddr    string       `json:"server_addr"`
	Status        string       `json:"status"`
	LastAttemptAt time.Time    `json:"last_attempt_at"`
	LastSuccessAt time.Time    `json:"last_success_at,omitempty"`
	LastError     string       `json:"last_error,omitempty"`
	Summary       *SyncSummary `json:"summary,omitempty"`
}

type SyncSQLiteStatus struct {
	Exists bool   `json:"exists"`
	Bytes  int64  `json:"bytes,omitempty"`
	Check  string `json:"check"`
	Error  string `json:"error,omitempty"`
}

type SyncManifestStatus struct {
	Present  bool     `json:"present"`
	Expected int64    `json:"expected"`
	Verified int64    `json:"verified"`
	Missing  int64    `json:"missing"`
	Changed  int64    `json:"changed"`
	Problems []string `json:"problems,omitempty"`
}

type SyncStatusSummary struct {
	DataDir             string              `json:"data_dir"`
	LinkedAttachmentDir string              `json:"linked_attachment_dir"`
	ServerAddr          string              `json:"server_addr,omitempty"`
	ConfigError         string              `json:"config_error,omitempty"`
	Ready               bool                `json:"ready"`
	Healthy             bool                `json:"healthy"`
	StatePresent        bool                `json:"state_present"`
	LastSync            *SyncState          `json:"last_sync,omitempty"`
	SQLite              SyncSQLiteStatus    `json:"sqlite"`
	Full                bool                `json:"full"`
	Manifest            *SyncManifestStatus `json:"manifest,omitempty"`
	PartialFiles        int64               `json:"partial_files,omitempty"`
	StagingDirs         int64               `json:"staging_dirs,omitempty"`
}

func (s SyncService) loadSyncConfig() (config.Config, string, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, "", err
	}
	envPath, err := s.DefaultPath()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("resolve default data-dir: %w", err)
	}
	return cfg, filepath.Join(filepath.Dir(envPath), "sync"), nil
}

func (s SyncService) Status(ctx context.Context, req SyncStatusRequest) (Result, error) {
	envPath, err := s.DefaultPath()
	if err != nil {
		return Result{}, fmt.Errorf("resolve default data-dir: %w", err)
	}
	dataDir := filepath.Join(filepath.Dir(envPath), "sync")
	cfg, _, configErr := s.LoadConfig()

	state, statePresent, err := readSyncState(dataDir)
	if err != nil {
		return Result{}, fmt.Errorf("read sync state: %w", err)
	}
	serverAddr := cfg.ServerAddr
	if serverAddr == "" && statePresent {
		serverAddr = state.ServerAddr
	}

	summary := SyncStatusSummary{
		DataDir:             dataDir,
		LinkedAttachmentDir: filepath.Join(dataDir, syncmirror.AttachmentsDir),
		ServerAddr:          serverAddr,
		StatePresent:        statePresent,
		Full:                req.Full,
		StagingDirs:         countSQLiteStagingDirs(dataDir),
	}
	if configErr != nil {
		summary.ConfigError = configErr.Error()
	}
	if statePresent {
		summary.LastSync = &state
	}

	sqlitePath := filepath.Join(dataDir, sqliteFileName)
	if info, statErr := os.Stat(sqlitePath); statErr == nil && !info.IsDir() {
		summary.SQLite.Exists = true
		summary.SQLite.Bytes = info.Size()
		check := "quick_check"
		if req.Full {
			check = "integrity_check"
		}
		summary.SQLite.Check = check
		if checkErr := checkSQLite(ctx, sqlitePath, check); checkErr != nil {
			summary.SQLite.Error = checkErr.Error()
		} else {
			summary.Ready = true
		}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		summary.SQLite.Check = "unavailable"
		summary.SQLite.Error = statErr.Error()
	} else {
		summary.SQLite.Check = "missing"
	}

	if req.Full {
		manifestStatus, partialFiles, verifyErr := verifySyncManifest(dataDir)
		if verifyErr != nil {
			return Result{}, verifyErr
		}
		summary.Manifest = &manifestStatus
		summary.PartialFiles = partialFiles
	}

	summary.Healthy = summary.Ready
	if summary.ConfigError != "" {
		summary.Healthy = false
	}
	if statePresent && state.Status == syncStateFailed {
		summary.Healthy = false
	}
	if summary.Manifest != nil && (!summary.Manifest.Present || summary.Manifest.Missing > 0 || summary.Manifest.Changed > 0) {
		summary.Healthy = false
	}
	if summary.PartialFiles > 0 || summary.StagingDirs > 0 {
		summary.Healthy = false
	}

	status := "ready"
	if !summary.Ready {
		status = "not ready"
	} else if !summary.Healthy {
		status = "degraded"
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Sync status: %s\n", status)
	fmt.Fprintf(&text, "  data dir: %s\n", dataDir)
	fmt.Fprintf(&text, "  linked attachment dir: %s\n", summary.LinkedAttachmentDir)
	if serverAddr != "" {
		fmt.Fprintf(&text, "  server: %s\n", serverAddr)
	}
	if summary.ConfigError != "" {
		fmt.Fprintf(&text, "  config: %s\n", summary.ConfigError)
	}
	if summary.SQLite.Error != "" {
		fmt.Fprintf(&text, "  sqlite: %s (%s)\n", summary.SQLite.Check, summary.SQLite.Error)
	} else if summary.SQLite.Exists {
		fmt.Fprintf(&text, "  sqlite: ok via %s (%s)\n", summary.SQLite.Check, humanBytes(summary.SQLite.Bytes))
	} else {
		fmt.Fprintln(&text, "  sqlite: missing")
	}
	if statePresent {
		fmt.Fprintf(&text, "  last sync: %s at %s\n", state.Status, state.LastAttemptAt.Format(time.RFC3339))
		if state.LastError != "" {
			fmt.Fprintf(&text, "  last error: %s\n", state.LastError)
		}
	}
	if summary.Manifest != nil {
		fmt.Fprintf(&text, "  manifest: %d verified, %d missing, %d changed\n", summary.Manifest.Verified, summary.Manifest.Missing, summary.Manifest.Changed)
		fmt.Fprintf(&text, "  partial files: %d\n", summary.PartialFiles)
	}
	fmt.Fprintln(&text, "  attachment paths: zot file path ATTACHMENT_KEY")
	return Result{Data: summary, Text: strings.TrimRight(text.String(), "\n")}, nil
}

func syncStatePath(dataDir string) string {
	return filepath.Join(dataDir, syncMetadataDir, syncStateFile)
}

func syncManifestPath(dataDir string) string {
	return filepath.Join(dataDir, syncMetadataDir, syncManifestFile)
}

func readSyncState(dataDir string) (SyncState, bool, error) {
	data, err := os.ReadFile(syncStatePath(dataDir))
	if os.IsNotExist(err) {
		return SyncState{}, false, nil
	}
	if err != nil {
		return SyncState{}, false, err
	}
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return SyncState{}, false, err
	}
	return state, true, nil
}

func writeSyncState(dataDir string, state SyncState) error {
	return writeSyncJSON(syncStatePath(dataDir), state)
}

func writeSyncManifest(dataDir string, manifest syncManifest) error {
	return writeSyncJSON(syncManifestPath(dataDir), manifest)
}

func writeSyncJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func checkSQLite(ctx context.Context, path, check string) error {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro&_pragma=query_only=1&_pragma=busy_timeout=5000"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "PRAGMA "+check)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return err
		}
		if result != "ok" {
			return fmt.Errorf("%s", result)
		}
	}
	return rows.Err()
}

func countSQLiteStagingDirs(dataDir string) int64 {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return 0
	}
	var count int64
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".sqlite-staging-") {
			count++
		}
	}
	return count
}

func verifySyncManifest(dataDir string) (SyncManifestStatus, int64, error) {
	status := SyncManifestStatus{}
	data, err := os.ReadFile(syncManifestPath(dataDir))
	if os.IsNotExist(err) {
		partial, walkErr := countPartialFiles(dataDir)
		return status, partial, walkErr
	}
	if err != nil {
		return status, 0, fmt.Errorf("read sync manifest: %w", err)
	}
	var manifest syncManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return status, 0, fmt.Errorf("parse sync manifest: %w", err)
	}
	status.Present = true
	for _, entry := range manifest.SQLite {
		path, pathErr := safepath.JoinComponents(dataDir, entry.Path)
		if pathErr != nil {
			return status, 0, fmt.Errorf("invalid sqlite manifest path: %w", pathErr)
		}
		verifyManifestFile(&status, path, "sqlite/"+entry.Path, entry.Size, entry.Mtime)
	}
	for _, group := range manifest.Storage {
		for _, entry := range group.Files {
			rel := filepath.ToSlash(filepath.Join(group.Key, entry.Name))
			path, pathErr := safepath.JoinComponents(filepath.Join(dataDir, "storage"), group.Key, entry.Name)
			if pathErr != nil {
				return status, 0, fmt.Errorf("invalid storage manifest path: %w", pathErr)
			}
			verifyManifestFile(&status, path, "storage/"+rel, entry.Size, entry.Mtime)
		}
	}
	for _, entry := range manifest.Fulltext {
		path, pathErr := safepath.JoinRelative(filepath.Join(dataDir, syncMetadataDir, "fulltext"), entry.Path)
		if pathErr != nil {
			return status, 0, fmt.Errorf("invalid fulltext manifest path: %w", pathErr)
		}
		verifyManifestFile(&status, path, "fulltext/"+entry.Path, entry.Size, entry.Mtime)
	}
	for _, entry := range manifest.Linked {
		if !entry.Available {
			continue
		}
		if _, keyErr := safepath.JoinComponents(dataDir, entry.Key); keyErr != nil {
			return status, 0, fmt.Errorf("invalid linked manifest key %q", entry.Key)
		}
		rel, relErr := linkedAttachmentMirrorRelativePath(dataDir, entry)
		if relErr != nil {
			return status, 0, relErr
		}
		path, pathErr := safepath.JoinRelative(dataDir, rel)
		if pathErr != nil {
			return status, 0, fmt.Errorf("invalid linked manifest path: %w", pathErr)
		}
		verifyManifestFile(&status, path, rel, entry.Size, entry.Mtime)
	}
	partial, err := countPartialFiles(dataDir)
	return status, partial, err
}

func verifyManifestFile(status *SyncManifestStatus, path, label string, size, mtime int64) {
	status.Expected++
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		status.Missing++
		appendSyncProblem(status, label+": missing")
		return
	}
	if err != nil {
		status.Changed++
		appendSyncProblem(status, label+": "+err.Error())
		return
	}
	if info.Size() != size || info.ModTime().Unix() != mtime {
		status.Changed++
		appendSyncProblem(status, label+": size or mtime changed")
		return
	}
	status.Verified++
}

func appendSyncProblem(status *SyncManifestStatus, problem string) {
	if len(status.Problems) < maxSyncProblems {
		status.Problems = append(status.Problems, problem)
	}
}

func countPartialFiles(dataDir string) (int64, error) {
	var count int64
	err := filepath.WalkDir(dataDir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(entry.Name(), ".part-") {
			count++
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return count, err
}
