package server

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type Handler struct {
	reader backend.Reader
}

func NewHandler(reader backend.Reader) *Handler {
	return &Handler{reader: reader}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health", h.healthCheck)
	mux.HandleFunc("GET /api/v1/stats", h.getStats)
	mux.HandleFunc("GET /api/v1/overview", h.getOverview)
	mux.HandleFunc("GET /api/v1/items", h.findItems)
	mux.HandleFunc("GET /api/v1/items/{key}", h.getItem)
	mux.HandleFunc("GET /api/v1/items/{key}/related", h.getRelated)
	mux.HandleFunc("GET /api/v1/collections", h.getCollections)
	mux.HandleFunc("GET /api/v1/tags", h.getTags)
	mux.HandleFunc("GET /api/v1/notes", h.getNotes)
	mux.HandleFunc("GET /api/v1/files/{key}", h.serveFile)
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, Meta{})
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.reader.GetLibraryStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats, Meta{})
}

func (h *Handler) findItems(w http.ResponseWriter, r *http.Request) {
	opts := parseFindOptions(r.URL.Query())
	items, err := h.reader.FindItems(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	meta := Meta{Total: len(items)}
	writeJSON(w, http.StatusOK, items, meta)
}

func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	item, err := h.reader.GetItem(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, item, Meta{})
}

func (h *Handler) getRelated(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	relations, err := h.reader.GetRelated(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	meta := Meta{Total: len(relations)}
	writeJSON(w, http.StatusOK, relations, meta)
}

func (h *Handler) getCollections(w http.ResponseWriter, r *http.Request) {
	collections, err := h.reader.ListCollections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, collections, Meta{})
}

func (h *Handler) getTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.reader.ListTags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tags, Meta{})
}

func (h *Handler) getNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := h.reader.ListNotes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, notes, Meta{})
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	filePath, contentType, err := h.reader.GetAttachmentFile(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, fmt.Errorf("file not found: %s", filePath))
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", formatContentDisposition(filepath.Base(filePath)))
	http.ServeFile(w, r, filePath)
}

type OverviewResponse struct {
	Stats       backend.LibraryStats `json:"stats"`
	RecentItems []domain.Item        `json:"recent_items"`
}

func (h *Handler) getOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := h.reader.GetLibraryStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	recentOpts := backend.FindOptions{Limit: 10, Sort: "dateAdded", Direction: "desc"}
	recentItems, _ := h.reader.FindItems(r.Context(), recentOpts)

	writeJSON(w, http.StatusOK, OverviewResponse{
		Stats:       stats,
		RecentItems: recentItems,
	}, Meta{})
}

func parseFindOptions(q url.Values) backend.FindOptions {
	return backend.ParseFindOptionsFromQuery(q)
}

func formatContentDisposition(filename string) string {
	mediatype, params, _ := mime.ParseMediaType(`inline; filename="x"`)
	params["filename"] = filename
	return mime.FormatMediaType(mediatype, params)
}

type LibraryStats = backend.LibraryStats
type Collection = backend.Collection
type Tag = backend.Tag

var _ = json.Marshal
var _ = domain.Item{}
