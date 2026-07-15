package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type IndexBuildRequest struct {
	Force          bool
	Workers        int
	ItemKeys       []string
	AttachmentKeys []string
}

type IndexBuildResult struct {
	TotalItems       int      `json:"total_items_with_pdf"`
	TotalAttachments int      `json:"total_attachments"`
	Indexed          int      `json:"indexed"`
	Skipped          int      `json:"skipped"`
	Failed           int      `json:"failed"`
	Errors           []string `json:"errors,omitempty"`
	Elapsed          float64  `json:"elapsed_seconds"`
}

type LocalDataStatus struct {
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
}

type FullTextIndexStatus struct {
	Status        string `json:"status"`
	Path          string `json:"path,omitempty"`
	IndexBytes    int64  `json:"index_bytes,omitempty"`
	CacheBytes    int64  `json:"cache_bytes,omitempty"`
	TotalBytes    int64  `json:"total_bytes,omitempty"`
	UsageMeasured bool   `json:"usage_measured,omitempty"`
}

type IndexService struct {
	LoadConfig func() (config.Config, string, error)
	NewReader  func(config.Config) (backend.Reader, error)
}

func NewIndexService() IndexService {
	return IndexService{LoadConfig: config.Load, NewReader: func(cfg config.Config) (backend.Reader, error) { return backend.NewLocalReader(cfg) }}
}

type indexExtractor interface {
	ExtractAttachmentFullTextOnly(context.Context, domain.Item, domain.Attachment) (backend.FullTextDocument, bool, error)
}
type indexWriter interface {
	SaveFullText(backend.FullTextDocument) error
}
type indexBatchWriter interface {
	SaveFullTextBatch([]backend.FullTextDocument) error
}
type indexCacheChecker interface {
	IsFullTextCached(domain.Attachment) bool
	IsMarkedFailed(string) bool
}
type indexEntryChecker interface{ IsFullTextIndexed(domain.Attachment) bool }
type indexFailureMarker interface{ MarkExtractFailed(string) error }

func (s IndexService) open() (config.Config, backend.Reader, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	reader, err := s.NewReader(cfg)
	return cfg, reader, err
}

func (s IndexService) Build(ctx context.Context, req IndexBuildRequest) (Result, error) {
	_, reader, err := s.open()
	if err != nil {
		return Result{}, err
	}
	if req.Workers == 0 {
		req.Workers = min(runtime.NumCPU(), 10)
	}
	if req.Workers < 1 {
		return Result{}, fmt.Errorf("--workers must be positive")
	}
	if req.Workers > 20 {
		req.Workers = 20
	}
	extractor, ok := reader.(indexExtractor)
	if !ok {
		return Result{}, fmt.Errorf("index build requires local full-text extraction support")
	}
	writer, hasWriter := reader.(indexWriter)
	batchWriter, hasBatchWriter := reader.(indexBatchWriter)
	cache, hasCache := reader.(indexCacheChecker)
	entries, hasEntries := reader.(indexEntryChecker)
	marker, hasMarker := reader.(indexFailureMarker)
	started := time.Now()
	items, err := indexBuildItems(ctx, reader, req.ItemKeys)
	if err != nil {
		return Result{}, err
	}
	attachmentKeys := make(map[string]struct{}, len(req.AttachmentKeys))
	for _, key := range req.AttachmentKeys {
		attachmentKeys[key] = struct{}{}
	}
	type task struct {
		item       domain.Item
		attachment domain.Attachment
	}
	var tasks []task
	result := IndexBuildResult{TotalItems: len(items)}
	for _, item := range items {
		for _, attachment := range item.Attachments {
			if attachment.ContentType != "application/pdf" {
				continue
			}
			if len(attachmentKeys) > 0 {
				if _, ok := attachmentKeys[attachment.Key]; !ok {
					continue
				}
			}
			result.TotalAttachments++
			if !req.Force && hasCache && cache.IsFullTextCached(attachment) && (!hasEntries || entries.IsFullTextIndexed(attachment)) {
				result.Skipped++
				continue
			}
			if !req.Force && hasCache && cache.IsMarkedFailed(attachment.Key) {
				result.Skipped++
				continue
			}
			tasks = append(tasks, task{item, attachment})
		}
	}
	type outcome struct {
		doc  backend.FullTextDocument
		ok   bool
		err  error
		task task
	}
	results := make(chan outcome, len(tasks))
	sem := make(chan struct{}, req.Workers)
	var wg sync.WaitGroup
	for _, current := range tasks {
		current := current
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			doc, ok, extractErr := extractor.ExtractAttachmentFullTextOnly(ctx, current.item, current.attachment)
			if (!ok || extractErr != nil) && hasMarker {
				_ = marker.MarkExtractFailed(current.attachment.Key)
			}
			results <- outcome{doc, ok && extractErr == nil, extractErr, current}
		}()
	}
	wg.Wait()
	close(results)
	var docs []backend.FullTextDocument
	var indexed, failed int64
	for outcome := range results {
		if outcome.ok {
			docs = append(docs, outcome.doc)
			atomic.AddInt64(&indexed, 1)
		} else {
			atomic.AddInt64(&failed, 1)
			if outcome.err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", outcome.task.item.Key, outcome.task.attachment.Key, outcome.err))
			}
		}
	}
	if len(docs) > 0 {
		if hasBatchWriter {
			err = batchWriter.SaveFullTextBatch(docs)
		} else if hasWriter {
			for _, doc := range docs {
				if err = writer.SaveFullText(doc); err != nil {
					break
				}
			}
		} else {
			err = fmt.Errorf("index writer is unavailable")
		}
		if err != nil {
			return Result{}, err
		}
	}
	result.Indexed = int(indexed)
	result.Failed = int(failed)
	result.Elapsed = time.Since(started).Seconds()
	text := fmt.Sprintf("Index complete: %d indexed, %d skipped, %d failed in %.1fs", result.Indexed, result.Skipped, result.Failed, result.Elapsed)
	return Result{Data: result, Meta: map[string]any{"elapsed": result.Elapsed, "workers": req.Workers}, Text: text}, nil
}

func indexBuildItems(ctx context.Context, reader backend.Reader, keys []string) ([]domain.Item, error) {
	if len(keys) == 0 {
		return reader.FindItems(ctx, backend.FindOptions{HasPDF: true, IncludeFields: []string{"attachments"}})
	}
	items := make([]domain.Item, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		item, err := reader.GetItem(ctx, key)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s IndexService) Status(_ context.Context) (Result, error) {
	cfg, reader, err := s.open()
	if err != nil {
		return Result{}, err
	}
	status := inspectFullTextIndex(cfg, reader, true)
	text := fmt.Sprintf("Full-text index: %s", status.Status)
	if status.Path != "" {
		text += fmt.Sprintf("\nPath: %s", status.Path)
	}
	if status.Status == "available" {
		text += fmt.Sprintf("\nIndex database: %s\nExtracted-text cache: %s\nTotal derived data: %s", humanBytes(status.IndexBytes), humanBytes(status.CacheBytes), humanBytes(status.TotalBytes))
	} else if status.Status == "unavailable" {
		text += "\nBuild: zot index build"
	}
	text += "\nSource PDFs: use `zot file path KEY` (typically under storage/ or attachments/)"
	return Result{Data: status, Meta: map[string]any{"available": status.Status == "available"}, Text: text}, nil
}

func inspectLocalData(reader backend.Reader) LocalDataStatus {
	if local := localReader(reader); local != nil {
		return LocalDataStatus{Status: "available", Path: local.DataDir}
	}
	return LocalDataStatus{Status: "not_applicable"}
}

func inspectFullTextIndex(cfg config.Config, reader backend.Reader, measureUsage bool) FullTextIndexStatus {
	if cfg.Mode == "remote" {
		return FullTextIndexStatus{Status: "server_managed"}
	}
	local := localReader(reader)
	if local == nil {
		return FullTextIndexStatus{Status: "not_applicable"}
	}
	indexPath := filepath.Join(local.FullTextCacheDir, "index.sqlite")
	status := FullTextIndexStatus{Status: "unavailable", Path: indexPath}
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		status.Status = "available"
		status.IndexBytes = info.Size()
	}
	if measureUsage {
		status.CacheBytes = directoryBytes(filepath.Join(local.FullTextCacheDir, "cache"))
		status.TotalBytes = status.IndexBytes + status.CacheBytes
		status.UsageMeasured = true
	}
	return status
}

func localReader(reader backend.Reader) *backend.LocalReader {
	switch value := reader.(type) {
	case *backend.LocalReader:
		return value
	case *backend.HybridReader:
		return value.LocalReader()
	default:
		return nil
	}
}

func directoryBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
