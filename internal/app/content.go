package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type FileRequest struct {
	AttachmentKey string
	ItemKey       string
	Sheet         string
	Head          int
	MaxSheets     int
	MaxColumns    int
	Health        bool
}

type FileResult struct {
	AttachmentKey string                    `json:"attachment_key"`
	ItemKey       string                    `json:"item_key,omitempty"`
	Label         string                    `json:"label,omitempty"`
	ContentType   string                    `json:"content_type,omitempty"`
	LocalPath     string                    `json:"local_path"`
	Health        *backend.AttachmentHealth `json:"health,omitempty"`
	Workbook      *backend.TableWorkbook    `json:"workbook,omitempty"`
}

func (s ReadService) Files(ctx context.Context, req FileRequest) (Result, error) {
	cfg, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	if cfg.Mode == "web" {
		return Result{}, fmt.Errorf("file inspection requires local, hybrid, or remote mode with attachment access")
	}
	if req.AttachmentKey == "" && req.ItemKey == "" {
		return Result{}, fmt.Errorf("attachment key or --item is required")
	}
	if req.AttachmentKey != "" && req.ItemKey != "" {
		return Result{}, fmt.Errorf("attachment key and --item are mutually exclusive")
	}
	if req.Sheet != "" && req.ItemKey != "" {
		return Result{}, fmt.Errorf("--sheet requires one attachment key")
	}
	opts := backend.TableInspectOptions{Sheet: req.Sheet, Head: positiveDefault(req.Head, 5), MaxSheets: positiveDefault(req.MaxSheets, 5), MaxColumns: positiveDefault(req.MaxColumns, 12)}
	results, err := inspectFileTargets(ctx, reader, req, opts)
	if err != nil {
		return Result{}, err
	}
	meta := readMeta(reader)
	meta["total"] = len(results)
	meta["head"] = opts.Head
	meta["max_sheets"] = opts.MaxSheets
	meta["max_columns"] = opts.MaxColumns
	return Result{Data: results, Meta: meta, Text: fileResultsText(results), Warnings: readWarnings(meta)}, nil
}

func inspectFileTargets(ctx context.Context, reader backend.Reader, req FileRequest, opts backend.TableInspectOptions) ([]FileResult, error) {
	if req.AttachmentKey != "" {
		path, contentType, err := reader.GetAttachmentFile(ctx, req.AttachmentKey)
		if err != nil {
			return nil, err
		}
		attachment := domain.Attachment{Key: req.AttachmentKey, ContentType: contentType, ResolvedPath: path, Resolved: true}
		result := FileResult{AttachmentKey: req.AttachmentKey, ContentType: contentType, LocalPath: path}
		if req.Health {
			health := backend.InspectAttachmentHealth(attachment)
			result.Health = &health
		}
		if !req.Health || isWorkbookPath(path) {
			workbook, err := backend.InspectTableFile(path, opts)
			if err != nil {
				return nil, err
			}
			result.Workbook = &workbook
		}
		return []FileResult{result}, nil
	}
	item, err := reader.GetItem(ctx, req.ItemKey)
	if err != nil {
		return nil, err
	}
	attachments := workbookAttachments(item.Attachments)
	if req.Health {
		attachments = item.Attachments
	}
	results := make([]FileResult, 0, len(attachments))
	for _, attachment := range attachments {
		if !attachment.Resolved && !req.Health {
			continue
		}
		result := FileResult{AttachmentKey: attachment.Key, ItemKey: item.Key, Label: firstNonEmpty(attachment.Title, attachment.Filename, filepath.Base(attachment.ResolvedPath), attachment.Key), ContentType: attachment.ContentType, LocalPath: attachment.ResolvedPath}
		if req.Health {
			health := backend.InspectAttachmentHealth(attachment)
			result.Health = &health
		}
		if attachment.Resolved && isWorkbookPath(firstNonEmpty(attachment.Filename, attachment.ZoteroPath, attachment.ResolvedPath, attachment.Title)) {
			workbook, err := backend.InspectTableFile(attachment.ResolvedPath, opts)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", attachment.Key, err)
			}
			result.Workbook = &workbook
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no matching attachments found for item %s", req.ItemKey)
	}
	return results, nil
}

func positiveDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func workbookAttachments(values []domain.Attachment) []domain.Attachment {
	result := make([]domain.Attachment, 0)
	for _, value := range values {
		if isWorkbookPath(firstNonEmpty(value.Filename, value.ZoteroPath, value.ResolvedPath, value.Title)) {
			result = append(result, value)
		}
	}
	return result
}

func isWorkbookPath(value string) bool {
	switch strings.ToLower(filepath.Ext(value)) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fileResultsText(results []FileResult) string {
	var b strings.Builder
	for i, result := range results {
		fmt.Fprintf(&b, "Attachment: %s\nPath: %s", result.AttachmentKey, result.LocalPath)
		if result.Health != nil {
			fmt.Fprintf(&b, "\nHealth: %s", result.Health.Status)
			for _, issue := range result.Health.Issues {
				fmt.Fprintf(&b, "\n  - %s %s: %s", issue.Severity, issue.Code, issue.Message)
			}
		}
		if result.Workbook != nil {
			fmt.Fprintf(&b, "\nWorkbook: %s, sheets=%d", result.Workbook.FileType, result.Workbook.SheetCount)
			for _, sheet := range result.Workbook.Sheets {
				fmt.Fprintf(&b, "\n  - %s rows=%d cols=%d", sheet.Name, sheet.Rows, sheet.Columns)
			}
		}
		if i < len(results)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

type AnnotationFilter struct {
	Page   int
	Type   string
	Author string
}

type AnnotationTarget struct {
	ItemKey string
	Request backend.AnnotateRequest
}

type AnnotationService struct {
	LoadConfig      func() (config.Config, string, error)
	NewReader       func(config.Config) (backend.Reader, error)
	NewDeleteClient func(config.Config) (annotationDeleteClient, error)
}

func NewAnnotationService() AnnotationService {
	read := NewReadService()
	return AnnotationService{
		LoadConfig: config.Load,
		NewReader:  read.NewReader,
		NewDeleteClient: func(cfg config.Config) (annotationDeleteClient, error) {
			return read.NewClient(cfg)
		},
	}
}

type annotationDeleteClient interface {
	GetLibraryStats(context.Context) (zoteroapi.LibraryStats, error)
	DeleteItems(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
}

type annotationReader interface {
	ReadItemAnnotations(context.Context, domain.Item) (backend.ItemAnnotationsResult, error)
}

type annotationWriter interface {
	AnnotateItem(context.Context, domain.Item, backend.AnnotateRequest) (backend.AnnotateResult, error)
}

type annotationDeleter interface {
	ClearItemAnnotations(context.Context, domain.Item, backend.DeleteAnnotationsRequest) (backend.ItemAnnotationClearResult, error)
}

func (s AnnotationService) open() (config.Config, backend.Reader, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	reader, err := s.NewReader(cfg)
	return cfg, reader, err
}

func (s AnnotationService) loadConfig() (config.Config, error) {
	cfg, _, err := s.LoadConfig()
	return cfg, err
}

func (s AnnotationService) List(ctx context.Context, itemKey string, filter AnnotationFilter) (Result, error) {
	_, reader, err := s.open()
	if err != nil {
		return Result{}, err
	}
	item, err := reader.GetItem(ctx, itemKey)
	if err != nil {
		return Result{}, err
	}
	capability, ok := reader.(annotationReader)
	if !ok {
		return Result{}, fmt.Errorf("annotations are not available for the current backend")
	}
	annotations, err := capability.ReadItemAnnotations(ctx, item)
	if err != nil {
		return Result{}, err
	}
	pdf := filterPDFAnnotations(annotations.PDFAnnotations, filter)
	db := filterDBAnnotations(annotations.DBAnnotations, filter)
	data := map[string]any{"item_key": itemKey, "attachment_key": annotations.AttachmentKey, "pdf_path": annotations.PDFPath, "pdf_annotations": pdf, "db_annotations": db, "total_pdf": len(pdf), "total_db": len(db)}
	meta := readMeta(reader)
	meta["total_pdf"] = len(pdf)
	meta["total_db"] = len(db)
	warnings := readWarnings(meta)
	if annotations.PDFError != "" {
		data["pdf_error"] = annotations.PDFError
		warnings = append(warnings, Warning{Code: "pdf_annotation_read_failed", Message: annotations.PDFError})
	}
	return Result{Data: data, Meta: meta, Text: annotationListText(item, annotations.PDFPath, pdf, db), Warnings: warnings}, nil
}

func (s AnnotationService) Create(ctx context.Context, targets []AnnotationTarget) (Result, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return Result{}, err
	}
	needsWrite := false
	for _, target := range targets {
		needsWrite = needsWrite || !target.Request.DryRun
	}
	if cfg.Mode != "remote" && needsWrite && !cfg.AllowWrite {
		return Result{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
	}
	reader, err := s.NewReader(cfg)
	if err != nil {
		return Result{}, err
	}
	capability, ok := reader.(annotationWriter)
	if !ok {
		return Result{}, fmt.Errorf("annotation writing is not available for the current backend")
	}
	results := make([]map[string]any, 0, len(targets))
	failed := 0
	for _, target := range targets {
		item, err := reader.GetItem(ctx, target.ItemKey)
		if err != nil {
			failed++
			results = append(results, map[string]any{"item_key": target.ItemKey, "error": err.Error()})
			continue
		}
		created, err := capability.AnnotateItem(ctx, item, target.Request)
		if err != nil {
			failed++
			results = append(results, map[string]any{"item_key": target.ItemKey, "error": err.Error()})
			continue
		}
		results = append(results, map[string]any{"item_key": target.ItemKey, "attachment_key": created.AttachmentKey, "pdf_path": created.PDFPath, "matches": created.Matches, "total_matches": len(created.Matches), "dry_run": created.DryRun})
	}
	meta := readMeta(reader)
	meta["total"] = len(results)
	meta["failed"] = failed
	return Result{Data: results, Meta: meta, Text: fmt.Sprintf("%d annotations processed (%d ok, %d failed)", len(results), len(results)-failed, failed), Warnings: readWarnings(meta)}, nil
}

func (s AnnotationService) Delete(ctx context.Context, itemKey string, filter AnnotationFilter, source string, safety SafetyOptions) (Result, error) {
	source, err := normalizeAnnotationDeleteSource(source)
	if err != nil {
		return Result{}, err
	}
	if err := validateAnnotationFilter(filter); err != nil {
		return Result{}, err
	}
	if source == "zotero" && safety.IfVersion < 0 {
		return Result{}, fmt.Errorf("--if-version must be non-negative")
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return Result{}, err
	}
	if !safety.DryRun && (cfg.Mode != "remote" || source == "zotero") && !cfg.AllowDelete {
		return Result{}, fmt.Errorf("delete operations are disabled; set ZOT_ALLOW_DELETE=1")
	}
	reader, err := s.NewReader(cfg)
	if err != nil {
		return Result{}, err
	}
	item, err := reader.GetItem(ctx, itemKey)
	if err != nil {
		return Result{}, err
	}
	readCapability, ok := reader.(annotationReader)
	if !ok {
		return Result{}, fmt.Errorf("annotations are not available for the current backend")
	}
	annotations, err := readCapability.ReadItemAnnotations(ctx, item)
	if err != nil {
		return Result{}, err
	}
	if source == "pdf" && annotations.PDFError != "" {
		return Result{}, fmt.Errorf("cannot safely select embedded PDF annotations: %s", annotations.PDFError)
	}
	pdfCandidates := filterPDFAnnotations(annotations.PDFAnnotations, filter)
	zoteroCandidates := filterDBAnnotations(annotations.DBAnnotations, filter)
	data := map[string]any{
		"item_key": itemKey, "source": source, "attachment_key": annotations.AttachmentKey,
		"pdf_candidates": pdfCandidates, "zotero_candidates": zoteroCandidates,
		"total_pdf_candidates": len(pdfCandidates), "total_zotero_candidates": len(zoteroCandidates),
	}
	selected := len(pdfCandidates)
	if source == "zotero" {
		selected = len(zoteroCandidates)
	}
	data["selected"] = selected
	if safety.DryRun {
		data["dry_run"] = true
		return Result{Data: data, Meta: map[string]any{"dry_run": true, "selected": selected}, Text: fmt.Sprintf("dry run: %d %s annotation(s) selected from %s", selected, source, itemKey)}, nil
	}
	if selected == 0 {
		return Result{Data: data, Meta: map[string]any{"deleted": 0}, Text: fmt.Sprintf("no matching %s annotations found for %s", source, itemKey)}, nil
	}
	if !safety.Yes && (safety.Confirm == nil || !safety.Confirm(fmt.Sprintf("delete %d %s annotation(s) from %s", selected, source, itemKey))) {
		return Result{}, ErrCancelled
	}

	if source == "zotero" {
		client, err := s.NewDeleteClient(cfg)
		if err != nil {
			return Result{}, fmt.Errorf("safe Zotero annotation deletion requires Web API access: %w", err)
		}
		version := safety.IfVersion
		if version == 0 {
			stats, err := client.GetLibraryStats(ctx)
			if err != nil {
				return Result{}, fmt.Errorf("resolve current library version: %w", err)
			}
			version = stats.LastLibraryVersion
			if version <= 0 {
				return Result{}, fmt.Errorf("library did not provide a usable current version")
			}
		}
		keys := make([]string, 0, len(zoteroCandidates))
		for _, annotation := range zoteroCandidates {
			keys = append(keys, annotation.Key)
		}
		batch, err := client.DeleteItems(ctx, keys, version)
		if err != nil {
			return Result{}, err
		}
		if len(batch.Failed) > 0 {
			return Result{}, fmt.Errorf("delete Zotero annotations failed for %d item(s)", len(batch.Failed))
		}
		data["deleted"] = len(keys)
		data["deleted_keys"] = keys
		data["last_modified_version"] = batch.LastModifiedVersion
		return Result{Data: data, Meta: map[string]any{"deleted": len(keys), "delete_source": "zotero_web_api"}, Text: fmt.Sprintf("deleted %d Zotero annotation(s) from %s", len(keys), itemKey)}, nil
	}

	capability, ok := reader.(annotationDeleter)
	if !ok {
		return Result{}, fmt.Errorf("PDF annotation deletion is not available for the current backend")
	}
	xrefs := make([]int, 0, len(pdfCandidates))
	for _, annotation := range pdfCandidates {
		if annotation.XRef <= 0 {
			return Result{}, fmt.Errorf("PDF annotation on page %d has no stable xref; refusing deletion", annotation.Page)
		}
		xrefs = append(xrefs, annotation.XRef)
	}
	deleted, err := capability.ClearItemAnnotations(ctx, item, backend.DeleteAnnotationsRequest{Page: filter.Page, Type: filter.Type, Author: filter.Author, PDFXRefs: xrefs})
	if err != nil {
		return Result{}, err
	}
	data["pdf_path"] = deleted.PDFPath
	data["deleted"] = deleted.PDFDeleted
	data["deleted_xrefs"] = xrefs
	return Result{Data: data, Meta: map[string]any{"deleted": deleted.PDFDeleted, "delete_source": "pdf_embedded"}, Text: fmt.Sprintf("deleted %d embedded PDF annotation(s) from %s", deleted.PDFDeleted, itemKey)}, nil
}

func normalizeAnnotationDeleteSource(source string) (string, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return "", fmt.Errorf("--source is required; choose zotero or pdf")
	}
	if source != "zotero" && source != "pdf" {
		return "", fmt.Errorf("unsupported annotation source %q; choose zotero or pdf", source)
	}
	return source, nil
}

func validateAnnotationFilter(filter AnnotationFilter) error {
	if filter.Page < 0 {
		return fmt.Errorf("--page must be non-negative")
	}
	if filter.Type == "" {
		return nil
	}
	valid := map[string]bool{
		"highlight": true, "note": true, "image": true, "ink": true, "area": true,
		"underline": true, "link": true, "freetext": true, "line": true, "square": true,
		"circle": true, "polygon": true, "polyline": true, "stamp": true, "caret": true,
		"attachment": true, "screen": true, "strikeout": true, "squiggly": true,
		"redact": true, "popup": true, "sound": true, "movie": true, "richmedia": true,
		"widget": true, "printermark": true, "trapnet": true, "watermark": true,
		"3d": true, "projection": true,
	}
	if !valid[strings.ToLower(strings.TrimSpace(filter.Type))] {
		return fmt.Errorf("unsupported annotation type %q", filter.Type)
	}
	return nil
}

func filterPDFAnnotations(values []backend.PDFAnnotation, filter AnnotationFilter) []backend.PDFAnnotation {
	result := make([]backend.PDFAnnotation, 0, len(values))
	for _, value := range values {
		if filter.Page > 0 && value.Page != filter.Page {
			continue
		}
		if filter.Type != "" && !strings.EqualFold(value.Type, filter.Type) {
			continue
		}
		if filter.Author != "" && !strings.EqualFold(value.Author, filter.Author) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func filterDBAnnotations(values []domain.Annotation, filter AnnotationFilter) []domain.Annotation {
	result := make([]domain.Annotation, 0, len(values))
	for _, value := range values {
		if filter.Page > 0 && value.PageIndex+1 != filter.Page {
			continue
		}
		if filter.Type != "" && !strings.EqualFold(value.Type, filter.Type) {
			continue
		}
		if filter.Author != "" && !strings.EqualFold(value.Author, filter.Author) {
			continue
		}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DateAdded > result[j].DateAdded })
	return result
}

func annotationListText(item domain.Item, pdfPath string, pdf []backend.PDFAnnotation, db []domain.Annotation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Annotations for %s (%s)", item.Key, item.Title)
	if pdfPath != "" {
		fmt.Fprintf(&b, "\nPDF: %s", pdfPath)
	}
	for _, value := range db {
		fmt.Fprintf(&b, "\n  [db:%s page=%d] %s%s", value.Type, value.PageIndex+1, value.Text, value.Comment)
	}
	for _, value := range pdf {
		fmt.Fprintf(&b, "\n  [pdf:%s page=%d] %s%s", value.Type, value.Page, value.Text, value.Comment)
	}
	fmt.Fprintf(&b, "\nTotal: %d (db:%d + pdf:%d)", len(db)+len(pdf), len(db), len(pdf))
	return b.String()
}
