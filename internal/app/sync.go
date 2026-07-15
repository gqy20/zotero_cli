package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/safepath"
	"zotero_cli/internal/syncmirror"
)

type SyncRequest struct {
	Force bool
}

type SyncStats struct {
	Downloaded int64 `json:"downloaded"`
	Skipped    int64 `json:"skipped"`
	Bytes      int64 `json:"bytes"`
}

type SyncSummary struct {
	DataDir             string    `json:"data_dir"`
	LinkedAttachmentDir string    `json:"linked_attachment_dir"`
	SQLiteChanged       bool      `json:"sqlite_changed"`
	Fulltext            SyncStats `json:"fulltext"`
	Attachments         SyncStats `json:"attachments"`
	LinkedAttachments   SyncStats `json:"linked_attachments"`
	LinkedUnavailable   int64     `json:"linked_unavailable"`
	Storage             bool      `json:"storage"`
}

type SyncService struct {
	LoadConfig    func() (config.Config, string, error)
	DefaultPath   func() (string, error)
	Progress      io.Writer
	NewHTTPClient func() *http.Client
}

func NewSyncService() SyncService {
	return SyncService{LoadConfig: config.Load, DefaultPath: config.DefaultEnvPath, NewHTTPClient: newSyncHTTPClient}
}

func (s SyncService) Sync(ctx context.Context, req SyncRequest) (result Result, err error) {
	const concurrency = 8
	cfg, dataDir, err := s.loadSyncConfig()
	if err != nil {
		return Result{}, err
	}
	serverAddr := cfg.ServerAddr
	authKey := cfg.ServerAuthKey
	if serverAddr == "" {
		return Result{}, fmt.Errorf("no server address configured; run `zot config init` or set ZOT_SERVER_ADDR")
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "storage"), 0o755); err != nil {
		return Result{}, fmt.Errorf("create data-dir: %w", err)
	}
	cleanupStaleSQLiteStaging(dataDir)

	state, _, stateErr := readSyncState(dataDir)
	if stateErr != nil {
		return Result{}, fmt.Errorf("read sync state: %w", stateErr)
	}
	state.Version = syncStateVersion
	state.ServerAddr = serverAddr
	state.Status = syncStateRunning
	state.LastAttemptAt = time.Now().UTC()
	state.LastError = ""
	if stateErr := writeSyncState(dataDir, state); stateErr != nil {
		return Result{}, fmt.Errorf("write sync state: %w", stateErr)
	}
	defer func() {
		if err == nil {
			return
		}
		state.Status = syncStateFailed
		state.LastError = err.Error()
		_ = writeSyncState(dataDir, state)
	}()

	httpClient := s.NewHTTPClient
	if httpClient == nil {
		httpClient = newSyncHTTPClient
	}
	client := &syncClient{baseURL: strings.TrimRight(serverAddr, "/"), authKey: authKey, httpClient: httpClient()}

	if s.Progress != nil {
		fmt.Fprintf(s.Progress, "Fetching sync manifest from %s...\n", serverAddr)
	}
	manifest, err := client.getManifest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch manifest: %w", err)
	}
	linkedWarnings := syncLinkedAttachmentWarnings(manifest.Linked)
	progress := newSyncProgress(s.Progress, manifest)
	progress.Start()
	defer func() { progress.Stop(err) }()

	progress.SetPhase("SQLite")
	sqliteChanged, err := syncSqlite(ctx, client, manifest.SQLite, dataDir, req.Force, progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync sqlite: %w", err)
	}
	progress.SetPhase("SQLite verification")
	if err := checkSQLite(ctx, filepath.Join(dataDir, sqliteFileName), "quick_check"); err != nil {
		return Result{}, fmt.Errorf("validate synced sqlite: %w", err)
	}

	// fulltext index (.zotero_cli/fulltext) — always synced: small, and lets
	// full-text search work right away without a local 'zot index build'.
	progress.SetPhase("fulltext index")
	ftDownloaded, ftSkipped, ftBytes, err := syncFulltext(ctx, client, manifest.Fulltext, dataDir, req.Force, concurrency, progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync fulltext: %w", err)
	}

	progress.SetPhase("attachments")
	downloaded, skipped, bytes, err := syncStorage(ctx, client, manifest.Storage, dataDir, req.Force, concurrency, progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync storage: %w", err)
	}
	progress.SetPhase("linked attachments")
	linkedDownloaded, linkedSkipped, linkedBytes, linkedUnavailable, err := syncLinkedAttachments(ctx, client, manifest.Linked, dataDir, req.Force, concurrency, progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync linked attachments: %w", err)
	}
	progress.SetPhase("finalizing mirror")

	summary := SyncSummary{
		DataDir:             dataDir,
		LinkedAttachmentDir: filepath.Join(dataDir, syncmirror.AttachmentsDir),
		SQLiteChanged:       sqliteChanged,
		Fulltext:            SyncStats{ftDownloaded, ftSkipped, ftBytes},
		Attachments:         SyncStats{downloaded, skipped, bytes},
		LinkedAttachments:   SyncStats{linkedDownloaded, linkedSkipped, linkedBytes},
		LinkedUnavailable:   linkedUnavailable, Storage: true,
	}
	if err := writeSyncManifest(dataDir, manifest); err != nil {
		return Result{}, fmt.Errorf("write sync manifest: %w", err)
	}
	state.Status = syncStateSuccess
	state.LastSuccessAt = time.Now().UTC()
	state.LastError = ""
	state.Summary = &summary
	if err := writeSyncState(dataDir, state); err != nil {
		return Result{}, fmt.Errorf("write sync state: %w", err)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Synced to %s\n", dataDir)
	fmt.Fprintf(&text, "  linked attachment dir: %s\n", summary.LinkedAttachmentDir)
	if sqliteChanged {
		fmt.Fprintln(&text, "  zotero.sqlite: updated")
	} else {
		fmt.Fprintln(&text, "  zotero.sqlite: unchanged")
	}
	fmt.Fprintf(&text, "  fulltext index: %d downloaded, %d unchanged (%s)\n", ftDownloaded, ftSkipped, humanBytes(ftBytes))
	fmt.Fprintf(&text, "  attachments: %d downloaded, %d unchanged (%s)\n", downloaded, skipped, humanBytes(bytes))
	fmt.Fprintf(&text, "  linked attachments: %d downloaded, %d unchanged (%s), %d unavailable\n", linkedDownloaded, linkedSkipped, humanBytes(linkedBytes), linkedUnavailable)
	fmt.Fprintf(&text, "\nUse it:\n  zot --mode local find ...\n")
	return Result{Data: summary, Text: strings.TrimRight(text.String(), "\n"), Warnings: linkedWarnings}, nil
}

func syncLinkedAttachmentWarnings(entries []syncLinkedMeta) []Warning {
	warnings := make([]Warning, 0)
	for _, entry := range entries {
		if entry.Available {
			continue
		}
		reason := strings.TrimSpace(entry.Error)
		if reason == "" {
			reason = "source file is unavailable"
		}
		label := entry.Key
		if name := strings.TrimSpace(entry.Name); name != "" {
			label += " (" + name + ")"
		}
		warnings = append(warnings, Warning{
			Code:    "linked_attachment_skipped",
			Message: fmt.Sprintf("linked attachment %s skipped: %s", label, reason),
		})
	}
	return warnings
}

// --- manifest types (mirror server's syncManifest shape) ---

type syncFileMeta struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

type syncStorageMeta struct {
	Key   string         `json:"key"`
	Files []syncFileMeta `json:"files"`
}

type syncPathMeta struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

type syncLinkedMeta struct {
	Key          string `json:"key"`
	Name         string `json:"name,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Mtime        int64  `json:"mtime,omitempty"`
	Available    bool   `json:"available"`
	Error        string `json:"error,omitempty"`
}

type syncManifest struct {
	SQLite   []syncPathMeta    `json:"sqlite"`
	Storage  []syncStorageMeta `json:"storage"`
	Fulltext []syncPathMeta    `json:"fulltext"`
	Linked   []syncLinkedMeta  `json:"linked,omitempty"`
}

type syncProgress struct {
	writer io.Writer

	totalFiles       int64
	totalBytes       int64
	completedFiles   int64
	completedBytes   int64
	transferredBytes int64

	phaseMu sync.RWMutex
	phase   string
	writeMu sync.Mutex
	start   time.Time
	done    chan struct{}
	wg      sync.WaitGroup
	stop    sync.Once
}

func newSyncProgress(writer io.Writer, manifest syncManifest) *syncProgress {
	p := &syncProgress{writer: writer}
	add := func(size int64) {
		p.totalFiles++
		if size > 0 {
			p.totalBytes += size
		}
	}
	for _, entry := range manifest.SQLite {
		add(entry.Size)
	}
	for _, entry := range manifest.Fulltext {
		add(entry.Size)
	}
	for _, storage := range manifest.Storage {
		for _, file := range storage.Files {
			add(file.Size)
		}
	}
	for _, entry := range manifest.Linked {
		if entry.Available {
			add(entry.Size)
		}
	}
	return p
}

func (p *syncProgress) Start() {
	if p == nil || p.writer == nil {
		return
	}
	p.start = time.Now()
	p.done = make(chan struct{})
	p.write("Sync plan: %d files, %s total\n", p.totalFiles, humanBytes(p.totalBytes))
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.report("Syncing " + p.currentPhase())
			case <-p.done:
				return
			}
		}
	}()
}

func (p *syncProgress) SetPhase(phase string) {
	if p == nil {
		return
	}
	previous := p.currentPhase()
	if previous != "" && p.writer != nil {
		p.report("Completed " + previous)
	}
	p.phaseMu.Lock()
	p.phase = phase
	p.phaseMu.Unlock()
	if p.writer != nil {
		p.write("Starting %s...\n", phase)
	}
}

func (p *syncProgress) currentPhase() string {
	if p == nil {
		return ""
	}
	p.phaseMu.RLock()
	defer p.phaseMu.RUnlock()
	return p.phase
}

func (p *syncProgress) SkipFile(size int64) {
	if p == nil {
		return
	}
	atomic.AddInt64(&p.completedFiles, 1)
	if size > 0 {
		atomic.AddInt64(&p.completedBytes, size)
	}
}

func (p *syncProgress) ResumeBytes(size int64) {
	if p != nil && size > 0 {
		atomic.AddInt64(&p.completedBytes, size)
	}
}

func (p *syncProgress) TransferBytes(delta int64) {
	if p == nil || delta == 0 {
		return
	}
	atomic.AddInt64(&p.completedBytes, delta)
	if delta > 0 {
		atomic.AddInt64(&p.transferredBytes, delta)
	}
}

func (p *syncProgress) CompleteFile() {
	if p != nil {
		atomic.AddInt64(&p.completedFiles, 1)
	}
}

func (p *syncProgress) Stop(syncErr error) {
	if p == nil || p.writer == nil {
		return
	}
	p.stop.Do(func() {
		if p.done != nil {
			close(p.done)
			p.wg.Wait()
		}
		if syncErr != nil {
			p.report("Sync failed during " + p.currentPhase())
			return
		}
		p.report("Sync complete")
	})
}

func (p *syncProgress) report(label string) {
	if p == nil || p.writer == nil {
		return
	}
	files := atomic.LoadInt64(&p.completedFiles)
	readyBytes := atomic.LoadInt64(&p.completedBytes)
	transferred := atomic.LoadInt64(&p.transferredBytes)
	if readyBytes < 0 {
		readyBytes = 0
	}
	if readyBytes > p.totalBytes {
		readyBytes = p.totalBytes
	}
	percent := 100.0
	if p.totalBytes > 0 {
		percent = float64(readyBytes) * 100 / float64(p.totalBytes)
	} else if p.totalFiles > 0 {
		percent = float64(files) * 100 / float64(p.totalFiles)
	}
	if percent > 100 {
		percent = 100
	}

	elapsed := time.Since(p.start)
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(transferred) / elapsed.Seconds()
	}
	var suffix string
	if speed > 0 {
		suffix = fmt.Sprintf(", %s/s", humanBytes(int64(speed)))
		remaining := p.totalBytes - readyBytes
		if remaining > 0 {
			suffix += ", ETA " + humanDuration(time.Duration(float64(remaining)/speed*float64(time.Second)))
		}
	}
	p.write("%s: %d/%d files, %s/%s (%.1f%%)%s\n",
		label, files, p.totalFiles, humanBytes(readyBytes), humanBytes(p.totalBytes), percent, suffix)
}

func (p *syncProgress) write(format string, args ...any) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	fmt.Fprintf(p.writer, format, args...)
}

func humanDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// --- sync client ---

// newSyncHTTPClient returns an *http.Client tuned for many parallel file
// downloads: a generous per-host idle connection pool so keep-alive actually
// reuses connections across the concurrency fan-out (DefaultTransport caps
// MaxIdleConnsPerHost at 2, which defeats pooling when concurrency > 2).
func newSyncHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 32,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

type syncClient struct {
	baseURL    string
	authKey    string
	httpClient *http.Client
}

func (c *syncClient) newReq(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.authKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.authKey)
	}
	return req, nil
}

func (c *syncClient) getManifest(ctx context.Context) (syncManifest, error) {
	req, err := c.newReq(ctx, http.MethodGet, "/api/v1/sync/manifest")
	if err != nil {
		return syncManifest{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return syncManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return syncManifest{}, syncHTTPError(resp)
	}
	var api struct {
		Ok    bool         `json:"ok"`
		Data  syncManifest `json:"data"`
		Error string       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return syncManifest{}, err
	}
	if !api.Ok {
		return syncManifest{}, fmt.Errorf("%s", api.Error)
	}
	return api.Data, nil
}

func syncHTTPError(resp *http.Response) error {
	const maxErrorBody = 64 << 10
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if readErr != nil {
		return fmt.Errorf("HTTP %d (read error response: %v)", resp.StatusCode, readErr)
	}
	var api struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &api) == nil {
		if message := strings.TrimSpace(api.Error); message != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
		}
	}
	if message := strings.TrimSpace(string(body)); message != "" {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

func (c *syncClient) getStream(ctx context.Context, path string, offset int64) (io.ReadCloser, bool, error) {
	req, err := c.newReq(ctx, http.MethodGet, path)
	if err != nil {
		return nil, false, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, false, fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	return resp.Body, offset > 0 && resp.StatusCode == http.StatusPartialContent, nil
}

// cleanupStaleSQLiteStaging removes staging directories left by a process that
// terminated before its deferred cleanup ran. Resumable .part files remain.
func cleanupStaleSQLiteStaging(dataDir string) {
	if entries, err := os.ReadDir(dataDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), ".sqlite-staging-") {
				_ = os.RemoveAll(filepath.Join(dataDir, e.Name()))
			}
		}
	}
}

const sqliteFileName = "zotero.sqlite"

// syncSqlite fetches the SQLite main db + wal/shm/journal sidecars, per file
// incrementally. Unchanged files (matching size+mtime in dataDir) are skipped;
// changed files stage in a temp dir and swap in together (sidecars first, main
// last) so an interrupted sync never leaves a mismatched main db + wal. Under
// WAL mode this usually means only the small -wal is re-fetched.
func syncSqlite(ctx context.Context, client *syncClient, entries []syncPathMeta, dataDir string, force bool, progress *syncProgress) (changed bool, err error) {
	hasMain := false
	allowed := map[string]bool{
		sqliteFileName: true, sqliteFileName + "-wal": true,
		sqliteFileName + "-shm": true, sqliteFileName + "-journal": true,
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !allowed[entry.Path] {
			return false, fmt.Errorf("manifest contains invalid sqlite path %q", entry.Path)
		}
		if seen[entry.Path] {
			return false, fmt.Errorf("manifest contains duplicate sqlite path %q", entry.Path)
		}
		seen[entry.Path] = true
		if entry.Path == sqliteFileName {
			hasMain = true
		}
	}
	if !hasMain {
		return false, fmt.Errorf("manifest does not contain %s", sqliteFileName)
	}
	staging, err := os.MkdirTemp(dataDir, ".sqlite-staging-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(staging)

	var toSwap []string
	var downloaded, skipped int
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.Path] = true
		local, pathErr := safepath.JoinComponents(dataDir, e.Path)
		if pathErr != nil {
			return false, pathErr
		}
		if !force {
			if fi, perr := os.Stat(local); perr == nil && fi.Size() == e.Size && fi.ModTime().Unix() == e.Mtime {
				skipped++
				progress.SkipFile(e.Size)
				continue
			}
		}
		body, _, err := client.getStream(ctx, "/api/v1/sync/sqlite-file/"+url.PathEscape(e.Path), 0)
		if err != nil {
			return false, err
		}
		dest, pathErr := safepath.JoinComponents(staging, e.Path)
		if pathErr != nil {
			body.Close()
			return false, pathErr
		}
		f, err := os.Create(dest)
		if err != nil {
			body.Close()
			return false, err
		}
		if _, err := io.Copy(f, io.TeeReader(body, progressWriter{progress: progress})); err != nil {
			body.Close()
			f.Close()
			return false, err
		}
		body.Close()
		f.Close()
		if err := os.Chtimes(dest, time.Unix(e.Mtime, 0), time.Unix(e.Mtime, 0)); err != nil {
			return false, err
		}
		toSwap = append(toSwap, e.Path)
		downloaded++
		progress.CompleteFile()
	}

	var obsolete []string
	for _, name := range []string{sqliteFileName + "-wal", sqliteFileName + "-shm", sqliteFileName + "-journal"} {
		if present[name] {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(dataDir, name)); statErr == nil {
			obsolete = append(obsolete, name)
		} else if !os.IsNotExist(statErr) {
			return false, statErr
		}
	}

	if len(toSwap) == 0 && len(obsolete) == 0 {
		return false, nil
	}
	// SQLite sidecars are part of the current database generation rather than
	// retained library content. Remove sidecars absent from the server manifest;
	// storage and fulltext files intentionally remain additive-only.
	for _, name := range obsolete {
		if err := os.Remove(filepath.Join(dataDir, name)); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	// Swap sidecars first, main db last, so a reader never sees a new main db
	// without its matching wal/shm.
	for _, name := range toSwap {
		if name == sqliteFileName {
			continue
		}
		if err := os.Rename(filepath.Join(staging, name), filepath.Join(dataDir, name)); err != nil {
			return false, err
		}
	}
	for _, name := range toSwap {
		if name == sqliteFileName {
			if err := os.Rename(filepath.Join(staging, sqliteFileName), filepath.Join(dataDir, sqliteFileName)); err != nil {
				return false, err
			}
			break
		}
	}
	return true, nil
}

// fileDownload is a single file to fetch into targetDir, identified by a
// slash-separated path relative to targetDir.
type fileDownload struct {
	relPath string
	size    int64
	mtime   int64
}

// fetchFn opens a download stream for a file by its slash-separated relPath.
type fetchFn func(ctx context.Context, relPath string, offset int64) (io.ReadCloser, bool, error)

type progressWriter struct {
	progress *syncProgress
}

func (w progressWriter) Write(p []byte) (int, error) {
	w.progress.TransferBytes(int64(len(p)))
	return len(p), nil
}

// runDownloads fetches files into targetDir with incremental skip (size+mtime),
// bounded concurrency, resumable partial files, atomic rename, and mtime restore.
// Shared by storage and fulltext syncing.
func runDownloads(ctx context.Context, targetDir string, files []fileDownload, force bool, concurrency int, progress *syncProgress, fetch fetchFn) (downloaded, skipped, bytes int64, err error) {
	var jobs []fileDownload
	for _, f := range files {
		local, pathErr := safepath.JoinRelative(targetDir, f.relPath)
		if pathErr != nil {
			return 0, 0, 0, pathErr
		}
		if !force {
			if fi, e := os.Stat(local); e == nil && fi.Size() == f.size && fi.ModTime().Unix() == f.mtime {
				atomic.AddInt64(&skipped, 1)
				progress.SkipFile(f.size)
				continue
			}
		}
		jobs = append(jobs, f)
	}
	if len(jobs) == 0 {
		return 0, skipped, 0, nil
	}

	for _, job := range jobs {
		progress.ResumeBytes(resumablePartialSize(targetDir, job))
	}

	// Fail-fast: cancel on the first download error so in-flight requests abort
	// and remaining jobs are skipped (avoids hammering a broken/auth-failing
	// server with the whole queue).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, j := range jobs {
		if ctx.Err() != nil {
			break // a worker failed; stop launching new ones
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(j fileDownload) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if e := downloadOneWithProgress(ctx, targetDir, j, fetch, progress); e != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = e
					cancel()
				}
				mu.Unlock()
				return
			}
			atomic.AddInt64(&downloaded, 1)
			atomic.AddInt64(&bytes, j.size)
			progress.CompleteFile()
		}(j)
	}
	wg.Wait()
	return downloaded, skipped, bytes, firstErr
}

func downloadOne(ctx context.Context, targetDir string, f fileDownload, fetch fetchFn) error {
	return downloadOneWithProgress(ctx, targetDir, f, fetch, nil)
}

func downloadOneWithProgress(ctx context.Context, targetDir string, f fileDownload, fetch fetchFn, progress *syncProgress) error {
	dest, err := safepath.JoinRelative(targetDir, f.relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if !safepath.ExistingDirectoryWithin(targetDir, filepath.Dir(dest)) {
		return fmt.Errorf("destination directory for %q escapes its root", f.relPath)
	}
	part := fmt.Sprintf("%s.part-%d-%d", dest, f.size, f.mtime)
	partName := filepath.Base(part)
	partPrefix := filepath.Base(dest) + ".part-"
	if entries, err := os.ReadDir(filepath.Dir(dest)); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), partPrefix) && entry.Name() != partName {
				_ = os.Remove(filepath.Join(filepath.Dir(dest), entry.Name()))
			}
		}
	}
	offset := int64(0)
	if fi, err := os.Stat(part); err == nil {
		if fi.Size() > 0 && fi.Size() < f.size {
			offset = fi.Size()
		} else {
			_ = os.Remove(part)
		}
	}
	body, resumed, err := fetch(ctx, f.relPath, offset)
	if err != nil {
		return err
	}
	defer body.Close()
	if offset > 0 && !resumed {
		progress.TransferBytes(-offset)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if resumed {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.TeeReader(body, progressWriter{progress: progress})); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	fi, err := os.Stat(part)
	if err != nil {
		return err
	}
	if fi.Size() != f.size {
		return fmt.Errorf("downloaded %s: got %d bytes, want %d", f.relPath, fi.Size(), f.size)
	}
	_ = os.Chtimes(part, time.Unix(f.mtime, 0), time.Unix(f.mtime, 0))
	return os.Rename(part, dest)
}

func resumablePartialSize(targetDir string, f fileDownload) int64 {
	dest, err := safepath.JoinRelative(targetDir, f.relPath)
	if err != nil {
		return 0
	}
	part := fmt.Sprintf("%s.part-%d-%d", dest, f.size, f.mtime)
	if info, err := os.Stat(part); err == nil && info.Size() > 0 && info.Size() < f.size {
		return info.Size()
	}
	return 0
}

func syncStorage(ctx context.Context, client *syncClient, entries []syncStorageMeta, dataDir string, force bool, concurrency int, progress *syncProgress) (downloaded, skipped, bytes int64, err error) {
	var files []fileDownload
	for _, e := range entries {
		for _, f := range e.Files {
			if _, pathErr := safepath.JoinComponents(dataDir, e.Key, f.Name); pathErr != nil {
				return 0, 0, 0, fmt.Errorf("invalid storage manifest path: %w", pathErr)
			}
			files = append(files, fileDownload{relPath: e.Key + "/" + f.Name, size: f.Size, mtime: f.Mtime})
		}
	}
	fetch := func(ctx context.Context, relPath string, offset int64) (io.ReadCloser, bool, error) {
		key, name, _ := strings.Cut(relPath, "/")
		return client.getStream(ctx, "/api/v1/sync/storage/"+url.PathEscape(key)+"/"+url.PathEscape(name), offset)
	}
	return runDownloads(ctx, filepath.Join(dataDir, "storage"), files, force, concurrency, progress, fetch)
}

// syncFulltext pulls .zotero_cli/fulltext/ (FTS5 index + extracted-text cache)
// so full-text search works post-sync without a local 'zot index build'.
func syncFulltext(ctx context.Context, client *syncClient, entries []syncPathMeta, dataDir string, force bool, concurrency int, progress *syncProgress) (downloaded, skipped, bytes int64, err error) {
	files := make([]fileDownload, 0, len(entries))
	for _, e := range entries {
		files = append(files, fileDownload{relPath: e.Path, size: e.Size, mtime: e.Mtime})
	}
	fetch := func(ctx context.Context, relPath string, offset int64) (io.ReadCloser, bool, error) {
		segs := strings.Split(relPath, "/")
		for i, s := range segs {
			segs[i] = url.PathEscape(s)
		}
		return client.getStream(ctx, "/api/v1/sync/fulltext/"+strings.Join(segs, "/"), offset)
	}
	return runDownloads(ctx, filepath.Join(dataDir, ".zotero_cli", "fulltext"), files, force, concurrency, progress, fetch)
}

func syncLinkedAttachments(ctx context.Context, client *syncClient, entries []syncLinkedMeta, dataDir string, force bool, concurrency int, progress *syncProgress) (downloaded, skipped, bytes, unavailable int64, err error) {
	attachmentMap, _, err := syncmirror.Load(dataDir)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if attachmentMap.Attachments == nil {
		attachmentMap.Attachments = map[string]syncmirror.AttachmentEntry{}
	}

	type remoteFile struct{ key, name string }
	remote := make(map[string]remoteFile)
	seen := make(map[string]fileDownload)
	files := make([]fileDownload, 0, len(entries))
	for _, entry := range entries {
		if _, keyErr := safepath.JoinComponents(dataDir, entry.Key); keyErr != nil {
			return 0, 0, 0, unavailable, fmt.Errorf("invalid linked attachment key %q in manifest", entry.Key)
		}
		if !entry.Available {
			unavailable++
			old := attachmentMap.Attachments[entry.Key]
			old.Key = entry.Key
			old.Name = entry.Name
			old.SourceAvailable = false
			old.Stale = false
			if strings.TrimSpace(entry.RelativePath) != "" {
				if rel, pathErr := linkedAttachmentMirrorRelativePath(dataDir, entry); pathErr == nil {
					old.RelativePath = rel
				} else {
					if entry.Error != "" {
						entry.Error += "; "
					}
					entry.Error += pathErr.Error()
				}
			}
			old.Error = entry.Error
			if _, ok := syncmirror.Resolve(dataDir, old); ok {
				old.Stale = true
			} else {
				old.RelativePath = ""
				old.Size = 0
				old.Mtime = 0
			}
			attachmentMap.Attachments[entry.Key] = old
			continue
		}
		rel, pathErr := linkedAttachmentMirrorRelativePath(dataDir, entry)
		if pathErr != nil {
			return 0, 0, 0, unavailable, pathErr
		}
		file := fileDownload{relPath: rel, size: entry.Size, mtime: entry.Mtime}
		if existing, duplicate := seen[rel]; duplicate {
			if existing.size != file.size || existing.mtime != file.mtime {
				return 0, 0, 0, unavailable, fmt.Errorf("linked attachments conflict at %q", entry.RelativePath)
			}
			skipped++
			if progress != nil {
				progress.SkipFile(entry.Size)
			}
			continue
		}
		seen[rel] = file
		files = append(files, file)
		remote[rel] = remoteFile{key: entry.Key, name: entry.Name}
	}
	fetch := func(ctx context.Context, relPath string, offset int64) (io.ReadCloser, bool, error) {
		entry := remote[relPath]
		return client.getStream(ctx, "/api/v1/sync/linked/"+url.PathEscape(entry.key)+"/"+url.PathEscape(entry.name), offset)
	}
	var runSkipped int64
	downloaded, runSkipped, bytes, err = runDownloads(ctx, dataDir, files, force, concurrency, progress, fetch)
	skipped += runSkipped
	if err != nil {
		return downloaded, skipped, bytes, unavailable, err
	}
	for _, entry := range entries {
		if !entry.Available {
			continue
		}
		rel, pathErr := linkedAttachmentMirrorRelativePath(dataDir, entry)
		if pathErr != nil {
			return downloaded, skipped, bytes, unavailable, pathErr
		}
		attachmentMap.Attachments[entry.Key] = syncmirror.AttachmentEntry{
			Key: entry.Key, Name: entry.Name,
			RelativePath: rel,
			Size:         entry.Size, Mtime: entry.Mtime, SourceAvailable: true,
		}
	}
	attachmentMap.Version = syncmirror.AttachmentMapVersion
	if err := writeSyncJSON(syncmirror.MapPath(dataDir), attachmentMap); err != nil {
		return downloaded, skipped, bytes, unavailable, err
	}
	return downloaded, skipped, bytes, unavailable, nil
}

// linkedAttachmentMirrorRelativePath maps a server-side attachments: path to
// the sync mirror's portable attachments/ tree.
func linkedAttachmentMirrorRelativePath(dataDir string, entry syncLinkedMeta) (string, error) {
	if strings.TrimSpace(entry.RelativePath) == "" {
		return "", fmt.Errorf("linked attachment %q has no relative path", entry.Key)
	}
	attachmentsRoot := filepath.Join(dataDir, syncmirror.AttachmentsDir)
	abs, err := safepath.JoinRelative(attachmentsRoot, entry.RelativePath)
	if err != nil {
		return "", fmt.Errorf("invalid linked attachment relative path %q: %w", entry.RelativePath, err)
	}
	rel, err := filepath.Rel(dataDir, abs)
	if err != nil || !safepath.Within(dataDir, abs) {
		return "", fmt.Errorf("invalid linked attachment relative path %q", entry.RelativePath)
	}
	return filepath.ToSlash(rel), nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
