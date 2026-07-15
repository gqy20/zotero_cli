package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/safepath"
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

type syncLinkedEntry struct {
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Mtime     int64  `json:"mtime,omitempty"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

type syncManifest struct {
	SQLite   []syncPathEntry    `json:"sqlite"`
	Storage  []syncStorageEntry `json:"storage"`
	Fulltext []syncPathEntry    `json:"fulltext"`
	Linked   []syncLinkedEntry  `json:"linked,omitempty"`
}

const sqliteFileName = "zotero.sqlite"

// syncManifest reports file sizes + mtimes so the client can skip unchanged
// files on subsequent syncs (incremental).
func (h *Handler) syncManifest(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}

	// SQLite main db + wal/shm/journal sidecars, listed individually so the
	// client syncs incrementally per file: in WAL mode most Zotero writes only
	// change -wal (small), leaving the ~hundred-MB main db untouched.
	manifest := syncManifest{}
	for _, name := range []string{sqliteFileName, sqliteFileName + "-wal", sqliteFileName + "-shm", sqliteFileName + "-journal"} {
		full := filepath.Join(h.dataDir, name)
		if entry, lstatErr := os.Lstat(full); lstatErr == nil && entry.Mode().IsRegular() && safepath.ExistingRegularFileWithin(h.dataDir, full) {
			fi, err := os.Stat(full)
			if err != nil {
				continue
			}
			manifest.SQLite = append(manifest.SQLite, syncPathEntry{
				Path:  name,
				Size:  fi.Size(),
				Mtime: fi.ModTime().Unix(),
			})
		}
	}
	if len(manifest.SQLite) == 0 || manifest.SQLite[0].Path != sqliteFileName {
		writeError(w, http.StatusNotFound, fmt.Errorf("zotero.sqlite not found in data_dir"))
		return
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
				if f.IsDir() || f.Type()&os.ModeSymlink != 0 {
					continue
				}
				full := filepath.Join(storageDir, e.Name(), f.Name())
				if info, err := f.Info(); err == nil && info.Mode().IsRegular() && safepath.ExistingRegularFileWithin(storageDir, full) {
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

	if lister, ok := h.reader.(backend.SyncLinkedAttachmentLister); ok {
		linked, err := lister.ListSyncLinkedAttachments(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("list linked attachments: %w", err))
			return
		}
		manifest.Linked = make([]syncLinkedEntry, 0, len(linked))
		for _, entry := range linked {
			manifest.Linked = append(manifest.Linked, syncLinkedEntry{
				Key: entry.Key, Name: entry.Name, Size: entry.Size, Mtime: entry.Mtime,
				Available: entry.Available, Error: entry.Error,
			})
		}
	}

	writeJSON(w, http.StatusOK, manifest, Meta{})
}

// syncLinkedFile serves a linked-file attachment through the reader so its
// original absolute/base-directory path never crosses the wire.
func (h *Handler) syncLinkedFile(w http.ResponseWriter, r *http.Request) {
	if h.reader == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no local reader; cannot sync linked attachments"))
		return
	}
	key := strings.TrimSpace(r.PathValue("key"))
	name := r.PathValue("file")
	if key == "" || name == "" || filepath.Base(name) != name {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid linked attachment path"))
		return
	}
	filePath, contentType, err := h.reader.GetAttachmentFile(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if filepath.Base(filePath) != name {
		writeError(w, http.StatusNotFound, fmt.Errorf("linked attachment filename does not match"))
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeFile(w, r, filePath)
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
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() || !safepath.ExistingRegularFileWithin(root, full) {
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
	abs, err := safepath.JoinRelative(fulltextDir, rel)
	if err != nil || !safepath.ExistingRegularFileWithin(fulltextDir, abs) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}
	http.ServeFile(w, r, abs)
}

// syncSqliteFile serves a single sqlite file (zotero.sqlite or a wal/shm/
// journal sidecar) for per-file incremental sync. WAL mode means most writes
// only touch -wal, so the client skips the large main db and re-fetches only
// the changed sidecar.
func (h *Handler) syncSqliteFile(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}
	name := r.PathValue("name")
	allowed := name == sqliteFileName || name == sqliteFileName+"-wal" || name == sqliteFileName+"-shm" || name == sqliteFileName+"-journal"
	if !allowed {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}
	abs, err := safepath.JoinComponents(h.dataDir, name)
	if err != nil || !safepath.ExistingRegularFileWithin(h.dataDir, abs) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}
	http.ServeFile(w, r, abs)
}

// syncStorageFile serves a single storage/{key}/{file}, looked up by path
// (no SQLite query) so the client can fetch exactly the files the manifest
// listed. Path components are basename-cleaned to prevent traversal.
func (h *Handler) syncStorageFile(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server has no data_dir; cannot sync"))
		return
	}
	key := r.PathValue("key")
	file := r.PathValue("file")
	storageDir := filepath.Join(h.dataDir, "storage")
	abs, err := safepath.JoinComponents(storageDir, key, file)
	if err != nil || !safepath.ExistingRegularFileWithin(storageDir, abs) {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}

	http.ServeFile(w, r, abs)
}

// pathIsWithin reports whether path resolves to inside dir (no ../ escape).
func pathIsWithin(path, dir string) bool {
	return safepath.Within(dir, path)
}
