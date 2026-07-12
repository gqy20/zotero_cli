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
	DataDir       string    `json:"data_dir"`
	SQLiteChanged bool      `json:"sqlite_changed"`
	Fulltext      SyncStats `json:"fulltext"`
	Attachments   SyncStats `json:"attachments"`
	Storage       bool      `json:"storage"`
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

func (s SyncService) Sync(ctx context.Context, req SyncRequest) (Result, error) {
	const concurrency = 8
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	serverAddr := cfg.ServerAddr
	authKey := cfg.ServerAuthKey
	if serverAddr == "" {
		return Result{}, fmt.Errorf("no server address configured; run `zot config init` or set ZOT_SERVER_ADDR")
	}

	envPath, err := s.DefaultPath()
	if err != nil {
		return Result{}, fmt.Errorf("resolve default data-dir: %w", err)
	}
	dataDir := filepath.Join(filepath.Dir(envPath), "sync")
	if err := os.MkdirAll(filepath.Join(dataDir, "storage"), 0o755); err != nil {
		return Result{}, fmt.Errorf("create data-dir: %w", err)
	}
	cleanupStaleSQLiteStaging(dataDir)

	httpClient := s.NewHTTPClient
	if httpClient == nil {
		httpClient = newSyncHTTPClient
	}
	client := &syncClient{baseURL: strings.TrimRight(serverAddr, "/"), authKey: authKey, httpClient: httpClient()}

	manifest, err := client.getManifest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch manifest: %w", err)
	}

	sqliteChanged, err := syncSqlite(ctx, client, manifest.SQLite, dataDir, req.Force, s.Progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync sqlite: %w", err)
	}

	// fulltext index (.zotero_cli/fulltext) — always synced: small, and lets
	// full-text search work right away without a local 'zot index build'.
	ftDownloaded, ftSkipped, ftBytes, err := syncFulltext(ctx, client, manifest.Fulltext, dataDir, req.Force, concurrency, s.Progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync fulltext: %w", err)
	}

	downloaded, skipped, bytes, err := syncStorage(ctx, client, manifest.Storage, dataDir, req.Force, concurrency, s.Progress)
	if err != nil {
		return Result{}, fmt.Errorf("sync storage: %w", err)
	}

	summary := SyncSummary{DataDir: dataDir, SQLiteChanged: sqliteChanged, Fulltext: SyncStats{ftDownloaded, ftSkipped, ftBytes}, Attachments: SyncStats{downloaded, skipped, bytes}, Storage: true}
	var text strings.Builder
	fmt.Fprintf(&text, "Synced to %s\n", dataDir)
	if sqliteChanged {
		fmt.Fprintln(&text, "  zotero.sqlite: updated")
	} else {
		fmt.Fprintln(&text, "  zotero.sqlite: unchanged")
	}
	fmt.Fprintf(&text, "  fulltext index: %d downloaded, %d unchanged (%s)\n", ftDownloaded, ftSkipped, humanBytes(ftBytes))
	fmt.Fprintf(&text, "  attachments: %d downloaded, %d unchanged (%s)\n", downloaded, skipped, humanBytes(bytes))
	fmt.Fprintf(&text, "\nUse it:\n  ZOT_MODE=local ZOT_DATA_DIR=%s zot find ...\n", dataDir)
	return Result{Data: summary, Text: strings.TrimRight(text.String(), "\n")}, nil
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

type syncManifest struct {
	SQLite   []syncPathMeta    `json:"sqlite"`
	Storage  []syncStorageMeta `json:"storage"`
	Fulltext []syncPathMeta    `json:"fulltext"`
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
		return syncManifest{}, fmt.Errorf("HTTP %d", resp.StatusCode)
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
func syncSqlite(ctx context.Context, client *syncClient, entries []syncPathMeta, dataDir string, force bool, progress io.Writer) (changed bool, err error) {
	staging, err := os.MkdirTemp(dataDir, ".sqlite-staging-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(staging)

	var toSwap []string
	var downloaded, skipped int
	for _, e := range entries {
		local := filepath.Join(dataDir, e.Path)
		if !force {
			if fi, perr := os.Stat(local); perr == nil && fi.Size() == e.Size && fi.ModTime().Unix() == e.Mtime {
				skipped++
				continue
			}
		}
		body, _, err := client.getStream(ctx, "/api/v1/sync/sqlite-file/"+url.PathEscape(e.Path), 0)
		if err != nil {
			return false, err
		}
		dest := filepath.Join(staging, e.Path)
		f, err := os.Create(dest)
		if err != nil {
			body.Close()
			return false, err
		}
		if _, err := io.Copy(f, body); err != nil {
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
	}

	if len(toSwap) == 0 {
		if progress != nil {
			fmt.Fprintf(progress, "  sqlite: %d files up to date\n", skipped)
		}
		return false, nil
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
	if progress != nil {
		fmt.Fprintf(progress, "  sqlite: %d downloaded, %d up to date\n", downloaded, skipped)
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

// runDownloads fetches files into targetDir with incremental skip (size+mtime),
// bounded concurrency, resumable partial files, atomic rename, and mtime restore.
// Shared by storage and fulltext syncing.
func runDownloads(ctx context.Context, targetDir string, files []fileDownload, force bool, concurrency int, progress io.Writer, fetch fetchFn) (downloaded, skipped, bytes int64, err error) {
	var jobs []fileDownload
	for _, f := range files {
		if !force {
			local := filepath.Join(targetDir, filepath.FromSlash(f.relPath))
			if fi, e := os.Stat(local); e == nil && fi.Size() == f.size && fi.ModTime().Unix() == f.mtime {
				atomic.AddInt64(&skipped, 1)
				continue
			}
		}
		jobs = append(jobs, f)
	}
	if len(jobs) == 0 {
		if progress != nil {
			fmt.Fprintf(progress, "  %d files up to date\n", skipped)
		}
		return 0, skipped, 0, nil
	}

	if progress != nil {
		fmt.Fprintf(progress, "  downloading %d files (%d up to date)...\n", len(jobs), skipped)
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					fmt.Fprintf(progress, "    %d downloaded, %d unchanged (%s)\n",
						atomic.LoadInt64(&downloaded), atomic.LoadInt64(&skipped), humanBytes(atomic.LoadInt64(&bytes)))
				case <-done:
					return
				}
			}
		}()
		defer func() { close(done) }()
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
			if e := downloadOne(ctx, targetDir, j, fetch); e != nil {
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
		}(j)
	}
	wg.Wait()
	return downloaded, skipped, bytes, firstErr
}

func downloadOne(ctx context.Context, targetDir string, f fileDownload, fetch fetchFn) error {
	dest := filepath.Join(targetDir, filepath.FromSlash(f.relPath))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
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
	if _, err := io.Copy(out, body); err != nil {
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

func syncStorage(ctx context.Context, client *syncClient, entries []syncStorageMeta, dataDir string, force bool, concurrency int, progress io.Writer) (downloaded, skipped, bytes int64, err error) {
	var files []fileDownload
	for _, e := range entries {
		for _, f := range e.Files {
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
func syncFulltext(ctx context.Context, client *syncClient, entries []syncPathMeta, dataDir string, force bool, concurrency int, progress io.Writer) (downloaded, skipped, bytes int64, err error) {
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
