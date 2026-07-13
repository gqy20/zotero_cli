package server

import (
	"context"
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
	reader      backend.Reader
	dataDir     string
	allowWrite  bool
	allowDelete bool
}

func NewHandler(reader backend.Reader) *Handler {
	return &Handler{reader: reader}
}

func NewHandlerWithDir(reader backend.Reader, dataDir string) *Handler {
	return NewHandlerWithPermissions(reader, dataDir, false, false)
}

func NewHandlerWithPermissions(reader backend.Reader, dataDir string, allowWrite bool, allowDelete bool) *Handler {
	return &Handler{reader: reader, dataDir: dataDir, allowWrite: allowWrite, allowDelete: allowDelete}
}

// FigureExtractor is implemented by readers that support figure extraction.
type FigureExtractor interface {
	ExtractFigures(ctx context.Context, item domain.Item, outputDir string) (backend.ExtractFiguresResult, error)
}

type previewReader interface {
	FullTextPreview(context.Context, domain.Item) (string, error)
}

type snippetReader interface {
	FullTextSnippet(context.Context, domain.Item, string) (string, error)
}

type attachmentTextReader interface {
	ExtractItemAttachmentTexts(context.Context, domain.Item) (backend.ItemFullTextResult, error)
}

type attachmentPageTextReader interface {
	ExtractItemAttachmentPageTexts(context.Context, domain.Item) (backend.ItemPageTextResult, error)
}

type itemAnnotationsReader interface {
	ReadItemAnnotations(context.Context, domain.Item, string) (backend.ItemAnnotationsResult, error)
}

type itemAnnotator interface {
	AnnotateItem(context.Context, domain.Item, backend.AnnotateRequest) (backend.AnnotateResult, error)
}

type itemAnnotationClearer interface {
	ClearItemAnnotations(context.Context, domain.Item, backend.DeleteAnnotationsRequest) (backend.ItemAnnotationClearResult, error)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health", h.healthCheck)
	mux.HandleFunc("GET /api/v1/stats", h.getStats)
	mux.HandleFunc("GET /api/v1/overview", h.getOverview)
	mux.HandleFunc("GET /api/v1/items", h.findItems)
	mux.HandleFunc("GET /api/v1/items/{key}", h.getItem)
	mux.HandleFunc("GET /api/v1/items/{key}/preview", h.getItemPreview)
	mux.HandleFunc("GET /api/v1/items/{key}/snippet", h.getItemSnippet)
	mux.HandleFunc("GET /api/v1/items/{key}/text", h.getItemText)
	mux.HandleFunc("GET /api/v1/items/{key}/annotations", h.getItemAnnotations)
	mux.HandleFunc("POST /api/v1/items/{key}/annotate", h.annotateItem)
	mux.HandleFunc("POST /api/v1/items/{key}/annotations/clear", h.clearItemAnnotations)
	mux.HandleFunc("GET /api/v1/items/{key}/related", h.getRelated)
	mux.HandleFunc("GET /api/v1/items/{key}/figures", h.extractFigures)
	mux.HandleFunc("GET /api/v1/collections", h.getCollections)
	mux.HandleFunc("GET /api/v1/tags", h.getTags)
	mux.HandleFunc("GET /api/v1/notes", h.getNotes)
	mux.HandleFunc("GET /api/v1/files/{key}", h.serveFile)
	mux.HandleFunc("GET /api/v1/figures/{attachmentKey}/{filename}", h.serveFigure)

	// Sync: pull raw zotero.sqlite + storage/ for offline local-mode use.
	mux.HandleFunc("GET /api/v1/sync/manifest", h.syncManifest)
	mux.HandleFunc("GET /api/v1/sync/sqlite-file/{name}", h.syncSqliteFile)
	mux.HandleFunc("GET /api/v1/sync/storage/{key}/{file}", h.syncStorageFile)
	mux.HandleFunc("GET /api/v1/sync/fulltext/{path...}", h.syncFulltextFile)
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
	writeJSON(w, http.StatusOK, sanitizeItemsForRemote(items), meta)
}

func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	item, err := h.reader.GetItem(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, sanitizeItemForRemote(item), Meta{})
}

func (h *Handler) getItemPreview(w http.ResponseWriter, r *http.Request) {
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	previewer, ok := h.reader.(previewReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("full-text preview not available on this server"))
		return
	}
	preview, err := previewer.FullTextPreview(r.Context(), item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, preview, Meta{})
}

func (h *Handler) getItemSnippet(w http.ResponseWriter, r *http.Request) {
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	snippeter, ok := h.reader.(snippetReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("full-text snippet not available on this server"))
		return
	}
	snippet, err := snippeter.FullTextSnippet(r.Context(), item, r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snippet, Meta{})
}

func (h *Handler) getItemText(w http.ResponseWriter, r *http.Request) {
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("pages") == "true" {
		reader, ok := h.reader.(attachmentPageTextReader)
		if !ok {
			writeError(w, http.StatusNotImplemented, fmt.Errorf("page-aware full-text extraction not available on this server"))
			return
		}
		result, err := reader.ExtractItemAttachmentPageTexts(r.Context(), item)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for i := range result.Attachments {
			result.Attachments[i].Attachment = sanitizeAttachmentForRemote(result.Attachments[i].Attachment)
		}
		writeJSON(w, http.StatusOK, result, Meta{Total: len([]rune(result.Text))})
		return
	}
	reader, ok := h.reader.(attachmentTextReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("full-text extraction not available on this server"))
		return
	}
	result, err := reader.ExtractItemAttachmentTexts(r.Context(), item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for i := range result.Attachments {
		result.Attachments[i].Attachment = sanitizeAttachmentForRemote(result.Attachments[i].Attachment)
	}
	writeJSON(w, http.StatusOK, result, Meta{Total: len([]rune(result.Text))})
}

func (h *Handler) getItemAnnotations(w http.ResponseWriter, r *http.Request) {
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	reader, ok := h.reader.(itemAnnotationsReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("annotations not available on this server"))
		return
	}
	result, err := reader.ReadItemAnnotations(r.Context(), item, r.URL.Query().Get("attachment"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result.PDFPath = ""
	writeJSON(w, http.StatusOK, result, Meta{})
}

func (h *Handler) annotateItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	annotator, ok := h.reader.(itemAnnotator)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("annotation writing not available on this server"))
		return
	}
	var req backend.AnnotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}
	if !req.DryRun && !h.allowWrite {
		writeError(w, http.StatusForbidden, fmt.Errorf("annotation writing is disabled on this server"))
		return
	}
	result, err := annotator.AnnotateItem(r.Context(), item, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result.PDFPath = ""
	writeJSON(w, http.StatusOK, result, Meta{Total: len(result.Matches)})
}

func (h *Handler) clearItemAnnotations(w http.ResponseWriter, r *http.Request) {
	if !h.allowDelete {
		writeError(w, http.StatusForbidden, fmt.Errorf("annotation deletion is disabled on this server"))
		return
	}
	item, err := h.loadItem(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	clearer, ok := h.reader.(itemAnnotationClearer)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("annotation deletion not available on this server"))
		return
	}
	var req backend.DeleteAnnotationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}
	result, err := clearer.ClearItemAnnotations(r.Context(), item, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result.PDFPath = ""
	writeJSON(w, http.StatusOK, result, Meta{Total: result.Deleted})
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
		RecentItems: sanitizeItemsForRemote(recentItems),
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

func (h *Handler) extractFigures(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	extractor, ok := h.reader.(FigureExtractor)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("figure extraction not available on this server"))
		return
	}

	item, err := h.reader.GetItem(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	outputDir := filepath.Join(h.dataDir, ".zotero_cli", "figures")
	result, err := extractor.ExtractFigures(r.Context(), item, outputDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Populate URL field for each figure
	for i := range result.Figures {
		fig := &result.Figures[i]
		if fig.AttachmentKey != "" && fig.File != "" {
			fig.URL = fmt.Sprintf("/api/v1/figures/%s/%s", fig.AttachmentKey, fig.File)
		}
	}

	writeJSON(w, http.StatusOK, result, Meta{})
}

func (h *Handler) serveFigure(w http.ResponseWriter, r *http.Request) {
	attKey := r.PathValue("attachmentKey")
	filename := r.PathValue("filename")

	// Prevent path traversal
	cleanAttKey := filepath.Base(attKey)
	cleanFilename := filepath.Base(filename)
	if cleanAttKey != attKey || cleanFilename != filename {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid path"))
		return
	}

	figurePath := filepath.Join(h.dataDir, ".zotero_cli", "figures", cleanAttKey, cleanFilename)
	if _, err := os.Stat(figurePath); err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("figure not found"))
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, figurePath)
}

func (h *Handler) loadItem(ctx context.Context, key string) (domain.Item, error) {
	return h.reader.GetItem(ctx, key)
}

func sanitizeItemsForRemote(items []domain.Item) []domain.Item {
	out := make([]domain.Item, 0, len(items))
	for _, item := range items {
		out = append(out, sanitizeItemForRemote(item))
	}
	return out
}

func sanitizeItemForRemote(item domain.Item) domain.Item {
	item.Attachments = sanitizeAttachmentsForRemote(item.Attachments)
	return item
}

func sanitizeAttachmentsForRemote(attachments []domain.Attachment) []domain.Attachment {
	out := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		out = append(out, sanitizeAttachmentForRemote(attachment))
	}
	return out
}

func sanitizeAttachmentForRemote(attachment domain.Attachment) domain.Attachment {
	attachment.ResolvedPath = ""
	attachment.ZoteroPath = ""
	return attachment
}

type LibraryStats = backend.LibraryStats
type Collection = backend.Collection
type Tag = backend.Tag

var _ = json.Marshal
var _ = domain.Item{}
