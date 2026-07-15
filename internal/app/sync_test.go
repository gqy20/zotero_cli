package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/syncmirror"
)

func TestSyncUsesConfiguredServerAndDefaultMirror(t *testing.T) {
	configDir := t.TempDir()
	sourceDB := filepath.Join(configDir, "source.sqlite")
	db, err := sql.Open("sqlite", sourceDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourceDB)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sync/manifest":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{
				"sqlite":   []any{map[string]any{"path": sqliteFileName, "size": info.Size(), "mtime": info.ModTime().Unix()}},
				"storage":  []any{},
				"fulltext": []any{},
			}})
		case "/api/v1/sync/sqlite-file/zotero.sqlite":
			http.ServeFile(w, r, sourceDB)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := filepath.Join(configDir, "sync")
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{ServerAddr: server.URL}, "", nil
	}
	service.DefaultPath = func() (string, error) { return filepath.Join(configDir, ".env"), nil }
	var progressOutput bytes.Buffer
	service.Progress = &progressOutput
	result, err := service.Sync(context.Background(), SyncRequest{})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := result.Data.(SyncSummary)
	if !ok || summary.DataDir != dir || !summary.Storage {
		t.Fatalf("unexpected summary: %#v", result.Data)
	}
	if summary.LinkedAttachmentDir != filepath.Join(dir, syncmirror.AttachmentsDir) {
		t.Fatalf("linked attachment dir = %q", summary.LinkedAttachmentDir)
	}
	state, present, err := readSyncState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !present || state.Status != syncStateSuccess || state.LastSuccessAt.IsZero() {
		t.Fatalf("unexpected persisted sync state: present=%v state=%#v", present, state)
	}
	if _, err := os.Stat(syncManifestPath(dir)); err != nil {
		t.Fatalf("sync manifest was not persisted: %v", err)
	}
	for _, want := range []string{"Fetching sync manifest", "Sync plan:", "Starting SQLite", "Sync complete:", "(100.0%)"} {
		if !strings.Contains(progressOutput.String(), want) {
			t.Fatalf("progress output %q does not contain %q", progressOutput.String(), want)
		}
	}
	if !strings.Contains(result.Text, "linked attachment dir:") {
		t.Fatalf("sync result text %q does not report linked attachment directory", result.Text)
	}
}

func TestSyncLinkedAttachmentsDownloadsAndRetainsUnavailableMirror(t *testing.T) {
	dataDir := t.TempDir()
	content := []byte("linked pdf")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/linked/LINK1/paper.pdf" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	client := &syncClient{baseURL: server.URL, httpClient: server.Client()}
	entries := []syncLinkedMeta{{
		Key: "LINK1", Name: "paper.pdf", RelativePath: "Q_生物科学/Q-3_研究方法与技术/paper.pdf",
		Size: int64(len(content)), Mtime: 123, Available: true,
	}}
	downloaded, skipped, _, unavailable, err := syncLinkedAttachments(context.Background(), client, entries, dataDir, false, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if downloaded != 1 || skipped != 0 || unavailable != 0 {
		t.Fatalf("unexpected stats: downloaded=%d skipped=%d unavailable=%d", downloaded, skipped, unavailable)
	}
	attachmentMap, _, err := syncmirror.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	entry := attachmentMap.Attachments["LINK1"]
	wantRelative := filepath.ToSlash(filepath.Join(syncmirror.AttachmentsDir, "Q_生物科学", "Q-3_研究方法与技术", "paper.pdf"))
	if entry.RelativePath != wantRelative {
		t.Fatalf("relative path = %q, want %q", entry.RelativePath, wantRelative)
	}
	resolved, ok := syncmirror.Resolve(dataDir, entry)
	if !ok {
		t.Fatalf("linked attachment did not resolve: %#v", entry)
	}
	if got, _ := os.ReadFile(resolved); !bytes.Equal(got, content) {
		t.Fatalf("linked content = %q", got)
	}

	missing := []syncLinkedMeta{{
		Key: "LINK1", Name: "paper.pdf", RelativePath: "Q_生物科学/Q-3_研究方法与技术/paper.pdf",
		Available: false, Error: "source file is unavailable",
	}}
	_, _, _, unavailable, err = syncLinkedAttachments(context.Background(), client, missing, dataDir, false, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	attachmentMap, _, _ = syncmirror.Load(dataDir)
	entry = attachmentMap.Attachments["LINK1"]
	if unavailable != 1 || !entry.Stale || entry.SourceAvailable {
		t.Fatalf("unavailable mirror was not retained as stale: %#v", entry)
	}
	if _, ok := syncmirror.Resolve(dataDir, entry); !ok {
		t.Fatal("stale local copy should remain usable")
	}
}

func TestSyncLinkedAttachmentsRejectsUnsafeRelativePath(t *testing.T) {
	dataDir := t.TempDir()
	client := &syncClient{baseURL: "http://example.test", httpClient: http.DefaultClient}
	entries := []syncLinkedMeta{{
		Key: "LINK1", Name: "paper.pdf", RelativePath: "../outside.pdf",
		Size: 1, Mtime: 123, Available: true,
	}}
	if _, _, _, _, err := syncLinkedAttachments(context.Background(), client, entries, dataDir, false, 1, nil); err == nil {
		t.Fatal("expected unsafe linked attachment path to be rejected")
	}
}

func TestSyncLinkedAttachmentsRejectsManifestWithoutRelativePath(t *testing.T) {
	dataDir := t.TempDir()
	client := &syncClient{baseURL: "http://example.test", httpClient: http.DefaultClient}
	entries := []syncLinkedMeta{{Key: "LINK1", Name: "paper.pdf", Size: 1, Mtime: 123, Available: true}}
	if _, _, _, _, err := syncLinkedAttachments(context.Background(), client, entries, dataDir, false, 1, nil); err == nil {
		t.Fatal("expected missing linked attachment relative path to be rejected")
	}
}

func TestDownloadOneResumesMatchingPartialFile(t *testing.T) {
	target := t.TempDir()
	content := []byte("0123456789")
	file := fileDownload{relPath: "KEY/file.pdf", size: int64(len(content)), mtime: 123}
	dest := filepath.Join(target, "KEY", "file.pdf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	part := dest + ".part-10-123"
	if err := os.WriteFile(part, content[:4], 0o644); err != nil {
		t.Fatal(err)
	}
	fetch := func(_ context.Context, _ string, offset int64) (io.ReadCloser, bool, error) {
		if offset != 4 {
			t.Fatalf("offset=%d, want 4", offset)
		}
		return io.NopCloser(bytes.NewReader(content[offset:])), true, nil
	}
	if err := downloadOne(context.Background(), target, file, fetch); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content=%q, want %q", got, content)
	}
}

func TestDownloadOneReportsResumedAndTransferredBytes(t *testing.T) {
	target := t.TempDir()
	content := []byte("0123456789")
	file := fileDownload{relPath: "KEY/file.pdf", size: int64(len(content)), mtime: 123}
	dest := filepath.Join(target, "KEY", "file.pdf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	part := dest + ".part-10-123"
	if err := os.WriteFile(part, content[:4], 0o644); err != nil {
		t.Fatal(err)
	}
	progress := &syncProgress{totalFiles: 1, totalBytes: int64(len(content))}
	progress.ResumeBytes(4)
	fetch := func(_ context.Context, _ string, offset int64) (io.ReadCloser, bool, error) {
		return io.NopCloser(bytes.NewReader(content[offset:])), true, nil
	}
	if err := downloadOneWithProgress(context.Background(), target, file, fetch, progress); err != nil {
		t.Fatal(err)
	}
	if progress.completedBytes != 10 || progress.transferredBytes != 6 {
		t.Fatalf("progress bytes = ready %d, transferred %d", progress.completedBytes, progress.transferredBytes)
	}
}

func TestSyncProgressReportsPercentageSpeedAndETA(t *testing.T) {
	var output bytes.Buffer
	manifest := syncManifest{Storage: []syncStorageMeta{{
		Key: "KEY1",
		Files: []syncFileMeta{
			{Name: "one.pdf", Size: 100},
			{Name: "two.pdf", Size: 100},
		},
	}}}
	progress := newSyncProgress(&output, manifest)
	progress.start = time.Now().Add(-2 * time.Second)
	progress.TransferBytes(100)
	progress.CompleteFile()
	progress.report("Syncing attachments")
	got := output.String()
	for _, want := range []string{"Syncing attachments", "1/2 files", "(50.0%)", "/s", "ETA"} {
		if !strings.Contains(got, want) {
			t.Fatalf("progress output %q does not contain %q", got, want)
		}
	}
}

func TestDownloadOneRejectsEscapingPath(t *testing.T) {
	target := t.TempDir()
	fetched := false
	err := downloadOne(context.Background(), target, fileDownload{relPath: "../outside.txt", size: 1}, func(context.Context, string, int64) (io.ReadCloser, bool, error) {
		fetched = true
		return io.NopCloser(bytes.NewReader([]byte("x"))), false, nil
	})
	if err == nil {
		t.Fatal("expected unsafe relative path to be rejected")
	}
	if fetched {
		t.Fatal("unsafe path should be rejected before download")
	}
}

func TestSyncStorageRejectsUnsafeManifestComponents(t *testing.T) {
	entries := []syncStorageMeta{{Key: "..", Files: []syncFileMeta{{Name: "outside.pdf"}}}}
	if _, _, _, err := syncStorage(context.Background(), nil, entries, t.TempDir(), false, 1, nil); err == nil {
		t.Fatal("expected unsafe storage key to be rejected")
	}
}

func TestSyncRequiresConfiguredServer(t *testing.T) {
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{}, "", nil }
	if _, err := service.Sync(context.Background(), SyncRequest{}); err == nil {
		t.Fatal("expected missing server error")
	}
}

func TestSyncPersistsFailedAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	configDir := t.TempDir()
	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{ServerAddr: server.URL}, "", nil
	}
	service.DefaultPath = func() (string, error) { return filepath.Join(configDir, ".env"), nil }
	if _, err := service.Sync(context.Background(), SyncRequest{}); err == nil {
		t.Fatal("expected sync failure")
	}
	state, present, err := readSyncState(filepath.Join(configDir, "sync"))
	if err != nil {
		t.Fatal(err)
	}
	if !present || state.Status != syncStateFailed || state.LastError == "" {
		t.Fatalf("unexpected failed state: present=%v state=%#v", present, state)
	}
}

func TestSyncSqliteRemovesSidecarsAbsentFromManifest(t *testing.T) {
	dataDir := t.TempDir()
	main := filepath.Join(dataDir, sqliteFileName)
	mtime := time.Unix(123, 0)
	if err := os.WriteFile(main, []byte("sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(main, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := os.WriteFile(main+suffix, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	changed, err := syncSqlite(context.Background(), nil, []syncPathMeta{{Path: sqliteFileName, Size: 6, Mtime: 123}}, dataDir, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected removal of stale sidecars to count as a SQLite change")
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(main + suffix); !os.IsNotExist(err) {
			t.Fatalf("sidecar %s still exists or stat failed: %v", suffix, err)
		}
	}
}

func TestSyncSqliteRejectsManifestWithoutMainDatabase(t *testing.T) {
	if _, err := syncSqlite(context.Background(), nil, nil, t.TempDir(), false, nil); err == nil {
		t.Fatal("expected missing main database error")
	}
}

func TestSyncSqliteRejectsUnexpectedPath(t *testing.T) {
	entries := []syncPathMeta{{Path: sqliteFileName}, {Path: "../outside"}}
	if _, err := syncSqlite(context.Background(), nil, entries, t.TempDir(), false, nil); err == nil {
		t.Fatal("expected unsafe sqlite path to be rejected")
	}
}

func TestSyncStatusFullVerifiesLastManifest(t *testing.T) {
	configDir := t.TempDir()
	dataDir := filepath.Join(configDir, "sync")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, sqliteFileName)
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := syncManifest{SQLite: []syncPathMeta{{Path: sqliteFileName, Size: info.Size(), Mtime: info.ModTime().Unix()}}}
	if err := writeSyncManifest(dataDir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncState(dataDir, SyncState{Version: syncStateVersion, ServerAddr: "http://example.test", Status: syncStateSuccess, LastAttemptAt: time.Now(), LastSuccessAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	service := NewSyncService()
	service.LoadConfig = func() (config.Config, string, error) {
		return config.Config{ServerAddr: "http://example.test"}, "", nil
	}
	service.DefaultPath = func() (string, error) { return filepath.Join(configDir, ".env"), nil }
	result, err := service.Status(context.Background(), SyncStatusRequest{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	status := result.Data.(SyncStatusSummary)
	if !status.Ready || !status.Healthy || status.Manifest == nil || status.Manifest.Verified != 1 {
		t.Fatalf("unexpected healthy status: %#v", status)
	}

	if err := os.WriteFile(sqlitePath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = service.Status(context.Background(), SyncStatusRequest{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	status = result.Data.(SyncStatusSummary)
	if status.Healthy || status.Manifest == nil || status.Manifest.Changed != 1 || status.SQLite.Error == "" {
		t.Fatalf("unexpected degraded status: %#v", status)
	}
}

func TestVerifySyncManifestRejectsEscapingPath(t *testing.T) {
	dataDir := t.TempDir()
	manifest := syncManifest{Fulltext: []syncPathMeta{{Path: "../../outside", Size: 1}}}
	if err := writeSyncManifest(dataDir, manifest); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifySyncManifest(dataDir); err == nil {
		t.Fatal("expected unsafe manifest path to be rejected")
	}
}

func TestVerifySyncManifestChecksLinkedAttachmentRelativeTree(t *testing.T) {
	dataDir := t.TempDir()
	relativePath := "Q_生物科学/Q-3_研究方法与技术/paper.pdf"
	path := filepath.Join(dataDir, syncmirror.AttachmentsDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest := syncManifest{Linked: []syncLinkedMeta{{
		Key: "LINK1", Name: "paper.pdf", RelativePath: relativePath,
		Size: info.Size(), Mtime: info.ModTime().Unix(), Available: true,
	}}}
	if err := writeSyncManifest(dataDir, manifest); err != nil {
		t.Fatal(err)
	}
	status, _, err := verifySyncManifest(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if status.Expected != 1 || status.Verified != 1 || status.Missing != 0 || status.Changed != 0 {
		t.Fatalf("unexpected manifest status: %#v", status)
	}
}
