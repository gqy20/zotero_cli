package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"zotero_cli/internal/config"
)

type ItemImportBatchResult struct {
	Total      int                   `json:"total"`
	Ready      int                   `json:"ready,omitempty"`
	Success    int                   `json:"success,omitempty"`
	Existing   int                   `json:"existing,omitempty"`
	Partial    int                   `json:"partial,omitempty"`
	Failed     int                   `json:"failed,omitempty"`
	TotalBytes int64                 `json:"total_bytes,omitempty"`
	DryRun     bool                  `json:"dry_run,omitempty"`
	Items      []ItemImportBatchItem `json:"items"`
}

type ItemImportBatchItem struct {
	InputIndex int                   `json:"input_index"`
	Input      string                `json:"input"`
	Status     string                `json:"status"`
	Data       *ItemImportResult     `json:"data,omitempty"`
	Warnings   []Warning             `json:"warnings,omitempty"`
	Error      *ItemImportBatchError `json:"error,omitempty"`
}

type ItemImportBatchError struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type ItemImportProgress struct {
	Completed int
	Total     int
	Input     string
	Status    string
}

func (s ItemImportService) Import(ctx context.Context, req ItemImportRequest) (Result, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	sources, metadataData, err := resolveItemImportInputs(req)
	if err != nil {
		return Result{}, err
	}
	if metadataData != nil {
		req.FromData = metadataData
		return s.importOne(ctx, cfg, req, "")
	}
	if len(sources) == 1 {
		req.FromData = nil
		req.FromName = ""
		return s.importOne(ctx, cfg, req, sources[0])
	}
	return s.importBatch(ctx, cfg, req, sources)
}

func resolveItemImportInputs(req ItemImportRequest) ([]string, []byte, error) {
	if len(req.Sources) > 0 && (strings.TrimSpace(req.FromName) != "" || len(req.FromData) > 0) {
		return nil, nil, fmt.Errorf("SOURCE and --from cannot be combined")
	}
	if strings.TrimSpace(req.FromName) == "" && len(req.FromData) == 0 {
		sources := normalizeImportSources(req.Sources)
		if len(sources) == 0 {
			return nil, nil, fmt.Errorf("SOURCE or --from is required")
		}
		return sources, nil, nil
	}
	if len(req.FromData) == 0 {
		return nil, nil, fmt.Errorf("import JSON is empty")
	}
	var value any
	if err := json.Unmarshal(req.FromData, &value); err != nil {
		return nil, nil, fmt.Errorf("decode import JSON: %w", err)
	}
	paths, classified, err := importPathsFromJSON(value)
	if err != nil {
		return nil, nil, err
	}
	if classified {
		paths = normalizeImportSources(paths)
		if len(paths) == 0 {
			return nil, nil, fmt.Errorf("import JSON contains no PDF paths")
		}
		return paths, nil, nil
	}
	return nil, req.FromData, nil
}

func importPathsFromJSON(value any) ([]string, bool, error) {
	switch typed := value.(type) {
	case string:
		return []string{typed}, true, nil
	case map[string]any:
		if data, ok := typed["data"]; ok {
			return importPathsFromJSON(data)
		}
		if path, ok := typed["path"].(string); ok {
			return []string{path}, true, nil
		}
		return nil, false, nil
	case []any:
		paths := make([]string, 0, len(typed))
		pathEntries := 0
		metadataEntries := 0
		for index, entry := range typed {
			entryPaths, classified, err := importPathsFromJSON(entry)
			if err != nil {
				return nil, false, fmt.Errorf("import input entry %d: %w", index+1, err)
			}
			if classified {
				pathEntries++
				paths = append(paths, entryPaths...)
			} else {
				metadataEntries++
			}
		}
		if pathEntries > 0 && metadataEntries > 0 {
			return nil, false, fmt.Errorf("import JSON cannot mix PDF paths with metadata records")
		}
		if pathEntries > 0 {
			return paths, true, nil
		}
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("import JSON must be a path, path array, metadata object, or CLI data envelope")
	}
}

func normalizeImportSources(sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		if source = strings.TrimSpace(source); source != "" {
			out = append(out, source)
		}
	}
	return out
}

func (s ItemImportService) importBatch(ctx context.Context, cfg config.Config, req ItemImportRequest, sources []string) (Result, error) {
	if !req.DryRun && !cfg.AllowWrite {
		return Result{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
	}
	pdfOnly := true
	for _, source := range sources {
		if isMetadataImportSource(source) {
			pdfOnly = false
			break
		}
	}
	var prepared preparedPDFImport
	var err error
	if pdfOnly {
		prepared, err = s.preparePDFImport(ctx, cfg, req.Collection)
		if err != nil {
			return Result{}, err
		}
	}
	batch := ItemImportBatchResult{Total: len(sources), DryRun: req.DryRun, Items: make([]ItemImportBatchItem, 0, len(sources))}
	seen := map[string]int{}
	for index, source := range sources {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		row := ItemImportBatchItem{InputIndex: index, Input: source}
		identity := importSourceIdentity(source)
		if first, duplicate := seen[identity]; duplicate {
			row.Status = "failed"
			row.Error = &ItemImportBatchError{Stage: "validation", Message: fmt.Sprintf("duplicate input; first occurrence is at index %d", first)}
			batch.Failed++
			batch.Items = append(batch.Items, row)
			s.reportImportProgress(index+1, len(sources), source, row.Status)
			continue
		}
		seen[identity] = index

		oneReq := req
		oneReq.Sources = nil
		oneReq.FromData = nil
		oneReq.FromName = ""
		var result Result
		if pdfOnly {
			result, err = s.importPreparedPDF(ctx, cfg, oneReq, source, prepared)
		} else {
			result, err = s.importOne(ctx, cfg, oneReq, source)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return Result{}, err
			}
			row.Status = "failed"
			row.Error = &ItemImportBatchError{Stage: importBatchErrorStage(source, err), Message: err.Error()}
			batch.Failed++
			batch.Items = append(batch.Items, row)
			s.reportImportProgress(index+1, len(sources), source, row.Status)
			continue
		}
		data, ok := result.Data.(ItemImportResult)
		if !ok {
			row.Status = "failed"
			row.Error = &ItemImportBatchError{Stage: "result", Message: fmt.Sprintf("unexpected import result %T", result.Data)}
			batch.Failed++
			batch.Items = append(batch.Items, row)
			s.reportImportProgress(index+1, len(sources), source, row.Status)
			continue
		}
		row.Status = data.Status
		row.Data = &data
		row.Warnings = append([]Warning(nil), result.Warnings...)
		batch.TotalBytes += data.Size
		batch.addStatus(data.Status)
		batch.Items = append(batch.Items, row)
		s.reportImportProgress(index+1, len(sources), source, row.Status)
	}
	text := itemImportBatchText(batch)
	return Result{Data: batch, Meta: map[string]any{"batch": true, "total": batch.Total, "total_bytes": batch.TotalBytes, "has_failures": batch.Failed > 0 || batch.Partial > 0}, Text: text}, nil
}

func itemImportBatchText(batch ItemImportBatchResult) string {
	var text strings.Builder
	fmt.Fprintf(&text, "processed %d import inputs: ready=%d success=%d existing=%d partial=%d failed=%d", batch.Total, batch.Ready, batch.Success, batch.Existing, batch.Partial, batch.Failed)
	for _, row := range batch.Items {
		fmt.Fprintf(&text, "\n[%d] %-8s %s", row.InputIndex, row.Status, row.Input)
		if row.Data != nil && row.Data.ItemKey != "" {
			fmt.Fprintf(&text, " item=%s", row.Data.ItemKey)
		}
		if row.Error != nil {
			fmt.Fprintf(&text, ": %s", row.Error.Message)
		}
	}
	return text.String()
}

func (s ItemImportService) reportImportProgress(completed, total int, input, status string) {
	if s.OnProgress != nil {
		s.OnProgress(ItemImportProgress{Completed: completed, Total: total, Input: input, Status: status})
	}
}

func (b *ItemImportBatchResult) addStatus(status string) {
	switch status {
	case "ready":
		b.Ready++
	case "success", "created":
		b.Success++
	case "existing":
		b.Existing++
	case "partial":
		b.Partial++
	default:
		b.Failed++
	}
}

func importSourceIdentity(source string) string {
	trimmed := strings.TrimSpace(source)
	if isMetadataImportSource(trimmed) {
		return "metadata:" + strings.ToLower(trimmed)
	}
	if abs, err := filepath.Abs(trimmed); err == nil {
		trimmed = filepath.Clean(abs)
	}
	if runtime.GOOS == "windows" {
		trimmed = strings.ToLower(trimmed)
	}
	return "file:" + trimmed
}

func importBatchErrorStage(source string, err error) string {
	message := strings.ToLower(err.Error())
	if !isMetadataImportSource(source) && (strings.Contains(message, "pdf path") || strings.Contains(message, "only pdf") || strings.Contains(message, "inspect pdf")) {
		return "validation"
	}
	if strings.Contains(message, "connector") || strings.Contains(message, "zotero desktop") {
		return "connector"
	}
	if strings.Contains(message, "metadata") || strings.Contains(message, "pmid") || strings.Contains(message, "doi") {
		return "metadata"
	}
	return "import"
}
