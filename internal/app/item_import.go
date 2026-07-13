package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
	"zotero_cli/internal/zoteroconnector"
)

type ItemImportRequest struct {
	Path       string
	Collection string
	DryRun     bool
}

type ItemImportResult struct {
	File               string                  `json:"file"`
	Size               int64                   `json:"size"`
	Accepted           bool                    `json:"accepted"`
	CanRecognize       bool                    `json:"can_recognize,omitempty"`
	RecognitionQueued  bool                    `json:"recognition_queued,omitempty"`
	DryRun             bool                    `json:"dry_run,omitempty"`
	CollectionKey      string                  `json:"collection_key,omitempty"`
	CollectionName     string                  `json:"collection_name,omitempty"`
	CollectionAssigned bool                    `json:"collection_assigned,omitempty"`
	DuplicateCleanup   *DuplicateCleanupResult `json:"duplicate_cleanup,omitempty"`
}

type DuplicateCleanupResult struct {
	Detected []string `json:"detected,omitempty"`
	Kept     string   `json:"kept,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
}

type ItemImportConnector interface {
	Ping(context.Context) error
	ImportPDF(context.Context, zoteroconnector.ImportPDFRequest) (zoteroconnector.ImportPDFResult, error)
	UpdateSession(context.Context, zoteroconnector.UpdateSessionRequest) error
	WaitForRecognizedItem(context.Context, string) (zoteroconnector.RecognizedItem, bool, error)
}

type itemImportCollectionResolver interface {
	CollectionTarget(context.Context, string) (backend.CollectionTarget, error)
	ImportedPDFAttachments(context.Context, string) ([]backend.ImportedAttachment, error)
}

type itemImportDeleteClient interface {
	GetLibraryStats(context.Context) (zoteroapi.LibraryStats, error)
	DeleteItems(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
}

type ItemImportService struct {
	LoadConfig      func() (config.Config, string, error)
	NewClient       func(config.Config) ItemImportConnector
	NewResolver     func(config.Config) (itemImportCollectionResolver, error)
	NewDeleteClient func(config.Config) (itemImportDeleteClient, error)
}

func NewItemImportService() ItemImportService {
	return ItemImportService{
		LoadConfig: config.Load,
		NewClient: func(cfg config.Config) ItemImportConnector {
			timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 20 * time.Second
			}
			return zoteroconnector.New(os.Getenv("ZOT_CONNECTOR_URL"), &http.Client{Timeout: timeout})
		},
		NewResolver: func(cfg config.Config) (itemImportCollectionResolver, error) {
			return backend.NewLocalReader(cfg)
		},
		NewDeleteClient: func(cfg config.Config) (itemImportDeleteClient, error) {
			read := NewReadService()
			return read.NewClient(cfg)
		},
	}
}

func (s ItemImportService) Import(ctx context.Context, req ItemImportRequest) (Result, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	if !cfg.AllowWrite {
		return Result{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
	}

	absPath, info, err := validateImportPDF(req.Path)
	if err != nil {
		return Result{}, err
	}
	client := s.NewClient(cfg)
	if err := client.Ping(ctx); err != nil {
		return Result{}, err
	}

	var collection backend.CollectionTarget
	var resolver itemImportCollectionResolver
	if strings.TrimSpace(req.Collection) != "" {
		resolver, err = s.NewResolver(cfg)
		if err != nil {
			return Result{}, fmt.Errorf("resolve import collection: %w", err)
		}
		collection, err = resolver.CollectionTarget(ctx, req.Collection)
		if err != nil {
			return Result{}, err
		}
	}
	data := ItemImportResult{File: absPath, Size: info.Size(), DryRun: req.DryRun, CollectionKey: collection.Key, CollectionName: collection.Name}
	if req.DryRun {
		return Result{Data: data, Meta: map[string]any{"dry_run": true}, Text: fmt.Sprintf("dry run: %s is ready to import into Zotero desktop", absPath)}, nil
	}

	file, err := os.Open(absPath)
	if err != nil {
		return Result{}, fmt.Errorf("open PDF %q: %w", absPath, err)
	}
	defer file.Close()
	sessionID := uuid.NewString()
	imported, err := client.ImportPDF(ctx, zoteroconnector.ImportPDFRequest{
		SessionID:     sessionID,
		Title:         filepath.Base(absPath),
		SourceURL:     importFileURL(absPath),
		Content:       file,
		ContentLength: info.Size(),
	})
	if err != nil {
		return Result{}, err
	}
	data.Accepted = true
	if collection.ID > 0 {
		if err := client.UpdateSession(ctx, zoteroconnector.UpdateSessionRequest{SessionID: sessionID, Target: fmt.Sprintf("C%d", collection.ID)}); err != nil {
			return Result{Data: data}, err
		}
		data.CollectionAssigned = true
	}
	data.CanRecognize = imported.CanRecognize
	data.RecognitionQueued = imported.CanRecognize
	text := fmt.Sprintf("imported %s into Zotero desktop", absPath)
	if imported.CanRecognize {
		text += "; metadata recognition queued"
	} else {
		text += "; Zotero did not queue metadata recognition"
	}
	if data.CollectionAssigned {
		text += fmt.Sprintf("; assigned to collection %s", data.CollectionName)
	}
	result := Result{Data: data, Meta: map[string]any{"write_source": "zotero_connector"}, Text: text}
	if imported.CanRecognize {
		if _, recognized, waitErr := client.WaitForRecognizedItem(ctx, sessionID); waitErr != nil {
			result.Warnings = append(result.Warnings, Warning{Code: "recognition_wait_failed", Message: waitErr.Error()})
		} else if recognized {
			time.Sleep(2 * time.Second)
			if resolver == nil {
				resolver, err = s.NewResolver(cfg)
			}
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{Code: "duplicate_cleanup_failed", Message: err.Error()})
			} else {
				cleanup, cleanupErr := s.cleanupDuplicateAttachments(ctx, cfg, resolver, importFileURL(absPath))
				data.DuplicateCleanup = cleanup
				result.Data = data
				if cleanupErr != nil {
					result.Warnings = append(result.Warnings, Warning{Code: "duplicate_cleanup_failed", Message: cleanupErr.Error()})
				}
			}
		}
	}
	return result, nil
}

func (s ItemImportService) cleanupDuplicateAttachments(ctx context.Context, cfg config.Config, resolver itemImportCollectionResolver, sourceURL string) (*DuplicateCleanupResult, error) {
	attachments, err := resolver.ImportedPDFAttachments(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	groups := map[string][]string{}
	for _, attachment := range attachments {
		if attachment.LinkMode != 2 || attachment.ParentKey == "" || attachment.Path == "" {
			continue
		}
		group := attachment.ParentKey + "\x00" + strings.ToLower(filepath.Clean(attachment.Path))
		groups[group] = append(groups[group], attachment.Key)
	}
	cleanup := &DuplicateCleanupResult{}
	for _, keys := range groups {
		if len(keys) < 2 {
			continue
		}
		sort.Strings(keys)
		cleanup.Kept = keys[0]
		cleanup.Detected = append(cleanup.Detected, keys...)
		cleanup.Deleted = append(cleanup.Deleted, keys[1:]...)
	}
	if len(cleanup.Deleted) == 0 {
		return nil, nil
	}
	sort.Strings(cleanup.Detected)
	sort.Strings(cleanup.Deleted)
	client, err := s.NewDeleteClient(cfg)
	if err != nil {
		cleanup.Deleted = nil
		return cleanup, err
	}
	stats, err := client.GetLibraryStats(ctx)
	if err != nil {
		cleanup.Deleted = nil
		return cleanup, err
	}
	deleted := append([]string(nil), cleanup.Deleted...)
	batch, err := client.DeleteItems(ctx, deleted, stats.LastLibraryVersion)
	if err != nil {
		cleanup.Deleted = nil
		return cleanup, err
	}
	if len(batch.Failed) > 0 {
		cleanup.Deleted = nil
		return cleanup, fmt.Errorf("delete duplicate attachments failed for %d item(s)", len(batch.Failed))
	}
	cleanup.Deleted = deleted
	return cleanup, nil
}

func validateImportPDF(path string) (string, os.FileInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil, fmt.Errorf("PDF path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve PDF path %q: %w", path, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", nil, fmt.Errorf("inspect PDF %q: %w", absPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("PDF path is not a regular file: %s", absPath)
	}
	if !strings.EqualFold(filepath.Ext(absPath), ".pdf") {
		return "", nil, fmt.Errorf("only PDF files are supported: %s", absPath)
	}
	return absPath, info, nil
}

func importFileURL(path string) string {
	slash := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash}).String()
}
