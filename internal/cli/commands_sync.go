package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zotero_cli/internal/config"
)

const usageSync = `usage: zot sync [--server-addr URL] [--data-dir DIR] [--force]
                [--concurrency N] [--no-storage]

Pull the remote library from a running 'zot server' into a local directory so
you can work offline in local mode afterwards, without keeping the server
running. Synced: zotero.sqlite (database) + storage/ (PDF/attachment files) +
.zotero_cli/fulltext/ (FTS5 full-text index + extracted text), so 'find
--fulltext' works right away with no local 'zot index build'. Subsequent runs
are incremental: files whose size + mtime are unchanged are skipped.

Flags:
  --server-addr URL   Remote 'zot server' URL (default: ZOT_SERVER_ADDR from config)
  --data-dir DIR      Local destination (default: ~/.zot/sync/)
  --force             Re-download everything, ignore incremental state
  --concurrency N     Parallel attachment downloads (default 8)
  --no-storage        Skip storage (PDF/attachments); sqlite + fulltext index still sync

Examples:
  zot sync                                        # uses ZOT_SERVER_ADDR, writes ~/.zot/sync/
  zot sync --server-addr http://192.168.1.50:8021 --data-dir ~/zotero-mirror
  zot sync --force                                 # full re-pull

After sync, point local mode at the copy:
  ZOT_MODE=local ZOT_DATA_DIR=~/.zot/sync zot find ...
  zot index build --data-dir ~/.zot/sync   # optional, build full-text index
`

type syncFlags struct {
	ServerAddr  string
	DataDir     string
	Concurrency int
	Force       bool
	NoStorage   bool
}

func parseSyncFlags(args []string) (syncFlags, error) {
	f := syncFlags{Concurrency: 8}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server-addr":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--server-addr requires a value")
			}
			f.ServerAddr = args[i+1]
			i++
		case "--data-dir":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--data-dir requires a value")
			}
			f.DataDir = args[i+1]
			i++
		case "--concurrency":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--concurrency requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				return f, fmt.Errorf("invalid --concurrency %q", args[i+1])
			}
			f.Concurrency = n
			i++
		case "--force":
			f.Force = true
		case "--no-storage":
			f.NoStorage = true
		default:
			return f, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return f, nil
}

func (c *CLI) runSync(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageSync)
	}
	flags, err := parseSyncFlags(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "%v\n\n%s", err, usageSync)
		return ExitUsage
	}

	// Resolve server addr + auth key: flags first, then config (.env + env).
	serverAddr := flags.ServerAddr
	authKey := ""
	if cfg, _, cfgErr := config.Load(); cfgErr == nil {
		if serverAddr == "" {
			serverAddr = cfg.ServerAddr
		}
		authKey = cfg.ServerAuthKey
	} else if cfgErr != config.ErrNotFound {
		fmt.Fprintf(c.stderr, "failed to load config: %v\n", cfgErr)
		return ExitConfig
	}
	if serverAddr == "" {
		fmt.Fprintf(c.stderr, "no server address: pass --server-addr or set ZOT_SERVER_ADDR\n(run 'zot init --mode remote --server-addr URL').\n\n%s", usageSync)
		return ExitConfig
	}

	dataDir := flags.DataDir
	if dataDir == "" {
		envPath, err := config.DefaultEnvPath()
		if err != nil {
			fmt.Fprintf(c.stderr, "resolve default data-dir: %v\n", err)
			return ExitConfig
		}
		dataDir = filepath.Join(filepath.Dir(envPath), "sync")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "storage"), 0o755); err != nil {
		fmt.Fprintf(c.stderr, "create data-dir: %v\n", err)
		return ExitError
	}
	cleanupStaleSync(dataDir)

	client := &syncClient{baseURL: strings.TrimRight(serverAddr, "/"), authKey: authKey, httpClient: newSyncHTTPClient()}
	ctx := context.Background()

	manifest, err := client.getManifest(ctx)
	if err != nil {
		fmt.Fprintf(c.stderr, "fetch manifest: %v\n", err)
		return ExitError
	}

	sqliteChanged, err := syncSqlite(ctx, client, manifest.SQLite, dataDir, flags.Force, c.stderr)
	if err != nil {
		fmt.Fprintf(c.stderr, "sync sqlite: %v\n", err)
		return ExitError
	}

	// fulltext index (.zotero_cli/fulltext) — always synced: small, and lets
	// full-text search work right away without a local 'zot index build'.
	ftDownloaded, ftSkipped, ftBytes, err := syncFulltext(ctx, client, manifest.Fulltext, dataDir, flags.Force, flags.Concurrency, c.stderr)
	if err != nil {
		fmt.Fprintf(c.stderr, "sync fulltext: %v\n", err)
		return ExitError
	}

	var downloaded, skipped, bytes int64
	if !flags.NoStorage {
		downloaded, skipped, bytes, err = syncStorage(ctx, client, manifest.Storage, dataDir, flags.Force, flags.Concurrency, c.stderr)
		if err != nil {
			fmt.Fprintf(c.stderr, "sync storage: %v\n", err)
			return ExitError
		}
	}

	fmt.Fprintf(c.stdout, "Synced to %s\n", dataDir)
	if sqliteChanged {
		fmt.Fprintf(c.stdout, "  zotero.sqlite: updated\n")
	} else {
		fmt.Fprintf(c.stdout, "  zotero.sqlite: unchanged\n")
	}
	fmt.Fprintf(c.stdout, "  fulltext index: %d downloaded, %d unchanged (%s)\n", ftDownloaded, ftSkipped, humanBytes(ftBytes))
	if !flags.NoStorage {
		fmt.Fprintf(c.stdout, "  attachments: %d downloaded, %d unchanged (%s)\n", downloaded, skipped, humanBytes(bytes))
	}
	fmt.Fprintf(c.stdout, "\nUse it:\n")
	fmt.Fprintf(c.stdout, "  ZOT_MODE=local ZOT_DATA_DIR=%s zot find ...\n", dataDir)
	fmt.Fprintf(c.stdout, "  (full-text index already synced; run 'zot index build --data-dir %s' only if you re-extract PDFs)\n", dataDir)
	fmt.Fprintf(c.stdout, "  note: only storage/ attachments are synced; linked_file attachments (external paths) are not.\n")
	return ExitOK
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

func (c *syncClient) getStream(ctx context.Context, path string) (io.ReadCloser, error) {
	req, err := c.newReq(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	return resp.Body, nil
}

// cleanupStaleSync removes leftovers from a previously interrupted sync:
// .sqlite-staging-* dirs (from syncSQLite) and stray *.tmp files under
// storage/ and .zotero_cli/fulltext/ (from interrupted downloads).
func cleanupStaleSync(dataDir string) {
	if entries, err := os.ReadDir(dataDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), ".sqlite-staging-") {
				_ = os.RemoveAll(filepath.Join(dataDir, e.Name()))
			}
		}
	}
	for _, sub := range []string{"storage", filepath.Join(".zotero_cli", "fulltext")} {
		root := filepath.Join(dataDir, sub)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".tmp") {
				_ = os.Remove(path)
			}
			return nil
		})
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
		body, err := client.getStream(ctx, "/api/v1/sync/sqlite-file/"+url.PathEscape(e.Path))
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
type fetchFn func(ctx context.Context, relPath string) (io.ReadCloser, error)

// runDownloads fetches files into targetDir with incremental skip (size+mtime),
// bounded concurrency, atomic write (*.tmp + rename), and mtime restore.
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
	body, err := fetch(ctx, f.relPath)
	if err != nil {
		return err
	}
	defer body.Close()

	dest := filepath.Join(targetDir, filepath.FromSlash(f.relPath))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()
	_ = os.Chtimes(tmp, time.Unix(f.mtime, 0), time.Unix(f.mtime, 0))
	return os.Rename(tmp, dest)
}

func syncStorage(ctx context.Context, client *syncClient, entries []syncStorageMeta, dataDir string, force bool, concurrency int, progress io.Writer) (downloaded, skipped, bytes int64, err error) {
	var files []fileDownload
	for _, e := range entries {
		for _, f := range e.Files {
			files = append(files, fileDownload{relPath: e.Key + "/" + f.Name, size: f.Size, mtime: f.Mtime})
		}
	}
	fetch := func(ctx context.Context, relPath string) (io.ReadCloser, error) {
		key, name, _ := strings.Cut(relPath, "/")
		return client.getStream(ctx, "/api/v1/sync/storage/"+url.PathEscape(key)+"/"+url.PathEscape(name))
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
	fetch := func(ctx context.Context, relPath string) (io.ReadCloser, error) {
		segs := strings.Split(relPath, "/")
		for i, s := range segs {
			segs[i] = url.PathEscape(s)
		}
		return client.getStream(ctx, "/api/v1/sync/fulltext/"+strings.Join(segs, "/"))
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
