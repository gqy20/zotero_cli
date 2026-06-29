package server

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
)

// Sync endpoints let a client pull the raw Zotero data files (zotero.sqlite +
// storage/) once and then work offline in local mode, instead of keeping the
// server running. All three live under authMiddleware (ZOT_SERVER_AUTH_KEY).

type syncFileEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"` // unix seconds — client round-trips this for incremental sync
}

type syncStorageEntry struct {
	Key   string          `json:"key"`
	Files []syncFileEntry `json:"files"`
}

// syncPathEntry describes a file under a sync section root (e.g. .zotero_cli/
// fulltext), identified by a slash-separated relative path.
type syncPathEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

type syncManifest struct {
	SQLite   syncFileEntry      `json:"sqlite"`
	Storage  []syncStorageEntry `json:"storage"`
	Fulltext []syncPathEntry    `json:"fulltext"`
}

const sqliteFileName = "zotero.sqlite"

// syncManifest reports file sizes + mtimes so the client can skip unchanged
// files on subsequent syncs (incremental).
func (h *Handler) syncManifest(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}

	sqlitePath := filepath.Join(h.dataDir, sqliteFileName)
	fi, err := os.Stat(sqlitePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("zotero.sqlite not found in data_dir"))
		return
	}

	// mtime is the max over the main file and its wal/shm/journal sidecars, so
	// the client re-syncs when Zotero appends to the WAL even if the main file
	// size is unchanged.
	sqliteMtime := fi.ModTime().Unix()
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if sfi, err := os.Stat(sqlitePath + suffix); err == nil {
			if m := sfi.ModTime().Unix(); m > sqliteMtime {
				sqliteMtime = m
			}
		}
	}

	manifest := syncManifest{
		SQLite: syncFileEntry{Name: sqliteFileName, Size: fi.Size(), Mtime: sqliteMtime},
	}

	storageDir := filepath.Join(h.dataDir, "storage")
	if entries, err := os.ReadDir(storageDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			files, err := os.ReadDir(filepath.Join(storageDir, e.Name()))
			if err != nil {
				continue
			}
			se := syncStorageEntry{Key: e.Name()}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				if info, err := f.Info(); err == nil {
					se.Files = append(se.Files, syncFileEntry{
						Name:  f.Name(),
						Size:  info.Size(),
						Mtime: info.ModTime().Unix(),
					})
				}
			}
			if len(se.Files) > 0 {
				manifest.Storage = append(manifest.Storage, se)
			}
		}
	}

	// .zotero_cli/fulltext — zot-cli's FTS5 index + extracted-text cache, so
	// clients can use full-text search right after sync without rebuilding.
	// (snapshot/figures_cache/venv are intentionally excluded: runtime-built
	// or platform-specific.)
	fulltextDir := filepath.Join(h.dataDir, ".zotero_cli", "fulltext")
	manifest.Fulltext = collectTree(fulltextDir, fulltextDir)

	writeJSON(w, http.StatusOK, manifest, Meta{})
}

// collectTree walks dir recursively and returns every regular file as a
// syncPathEntry with a slash-separated path relative to root (which stays
// fixed across recursion so nested paths keep their full prefix).
func collectTree(root, dir string) []syncPathEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []syncPathEntry
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			out = append(out, collectTree(root, full)...)
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		out = append(out, syncPathEntry{
			Path:  filepath.ToSlash(rel),
			Size:  fi.Size(),
			Mtime: fi.ModTime().Unix(),
		})
	}
	return out
}

// syncFulltextFile serves a single file under .zotero_cli/fulltext/ by relative
// path (e.g. cache/KEY/content.txt). Used to ship the FTS index + text cache.
func (h *Handler) syncFulltextFile(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}
	rel := r.PathValue("path")
	fulltextDir := filepath.Join(h.dataDir, ".zotero_cli", "fulltext")
	abs := filepath.Join(fulltextDir, filepath.FromSlash(rel))
	if rel == "" || !pathIsWithin(abs, fulltextDir) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}
	if fi, err := os.Stat(abs); err != nil || fi.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}
	http.ServeFile(w, r, abs)
}

// syncSQLite streams a tar of a consistent SQLite snapshot: zotero.sqlite plus
// its -wal/-shm/-journal sidecars when present. The snapshot is a read-only
// copy (see backend.CreateSQLiteSnapshot), safe to produce while Zotero runs.
func (h *Handler) syncSQLite(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}
	sqlitePath := filepath.Join(h.dataDir, sqliteFileName)
	if _, err := os.Stat(sqlitePath); err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("zotero.sqlite not found"))
		return
	}

	snapshotDir, _, err := backend.CreateSQLiteSnapshot(sqlitePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("snapshot sqlite: %w", err))
		return
	}
	defer os.RemoveAll(snapshotDir)

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("read snapshot: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", `attachment; filename="zotero.sqlite.tar"`)

	tw := tar.NewWriter(w)
	defer tw.Close()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		hdr := &tar.Header{
			Name:    e.Name(),
			Size:    info.Size(),
			Mode:    int64(info.Mode().Perm()),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return
		}
		f, err := os.Open(filepath.Join(snapshotDir, e.Name()))
		if err != nil {
			return
		}
		io.Copy(tw, f) // best-effort; client failure surfaces as short write
		f.Close()
	}
}

// syncStorageFile serves a single storage/{key}/{file}, looked up by path
// (no SQLite query) so the client can fetch exactly the files the manifest
// listed. Path components are basename-cleaned to prevent traversal.
func (h *Handler) syncStorageFile(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}
	key := filepath.Base(r.PathValue("key"))
	file := filepath.Base(r.PathValue("file"))
	if key == "." || key == string(os.PathSeparator) || file == "." || file == string(os.PathSeparator) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}

	storageDir := filepath.Join(h.dataDir, "storage")
	abs := filepath.Join(storageDir, key, file)
	if !pathIsWithin(abs, storageDir) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}
	if fi, err := os.Stat(abs); err != nil || fi.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}

	http.ServeFile(w, r, abs)
}

// pathIsWithin reports whether path resolves to inside dir (no ../ escape).
func pathIsWithin(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
