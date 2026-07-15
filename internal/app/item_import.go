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
	"zotero_cli/internal/references"
	"zotero_cli/internal/zoteroapi"
	"zotero_cli/internal/zoteroconnector"
)

type ItemImportRequest struct {
	Source     string
	FromData   []byte
	FromName   string
	Collection string
	DryRun     bool
}

type ItemImportResult struct {
	Status              string                  `json:"status"`
	SourceType          string                  `json:"source_type"`
	Identifier          string                  `json:"identifier,omitempty"`
	Mode                string                  `json:"mode,omitempty"`
	Stages              map[string]string       `json:"stages,omitempty"`
	PlannedActions      []string                `json:"planned_actions,omitempty"`
	DuplicateCandidates []DuplicateCandidate    `json:"duplicate_candidates,omitempty"`
	File                string                  `json:"file,omitempty"`
	Size                int64                   `json:"size,omitempty"`
	Accepted            bool                    `json:"accepted"`
	CanRecognize        bool                    `json:"can_recognize,omitempty"`
	RecognitionQueued   bool                    `json:"recognition_queued,omitempty"`
	DryRun              bool                    `json:"dry_run,omitempty"`
	CollectionKey       string                  `json:"collection_key,omitempty"`
	CollectionName      string                  `json:"collection_name,omitempty"`
	CollectionPath      string                  `json:"collection_path,omitempty"`
	CollectionAssigned  bool                    `json:"collection_assigned,omitempty"`
	DuplicateCleanup    *DuplicateCleanupResult `json:"duplicate_cleanup,omitempty"`
	ItemKey             string                  `json:"item_key,omitempty"`
	AttachmentKey       string                  `json:"attachment_key,omitempty"`
	FullTextIndexed     bool                    `json:"full_text_indexed,omitempty"`
}

type DuplicateCandidate struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Match string `json:"match"`
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
	GetLibraryVersion(context.Context) (int, error)
	DeleteItems(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
}

type itemImportWriteClient interface {
	GetLibraryVersion(context.Context) (int, error)
	CreateItem(context.Context, map[string]any, int) (zoteroapi.WriteResult, error)
}

type itemImportIndexBuilder interface {
	Build(context.Context, IndexBuildRequest) (Result, error)
}

type ItemImportService struct {
	LoadConfig      func() (config.Config, string, error)
	NewClient       func(config.Config) ItemImportConnector
	NewResolver     func(config.Config) (itemImportCollectionResolver, error)
	NewDeleteClient func(config.Config) (itemImportDeleteClient, error)
	NewWriteClient  func(config.Config) (itemImportWriteClient, error)
	NewReader       func(config.Config) (backend.Reader, error)
	ResolveArticle  func(context.Context, config.Config, references.Identifiers) (references.Article, error)
	NewIndexBuilder func() itemImportIndexBuilder
	PollInterval    time.Duration
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
		NewWriteClient: func(cfg config.Config) (itemImportWriteClient, error) {
			read := NewReadService()
			return read.NewClient(cfg)
		},
		NewReader: func(cfg config.Config) (backend.Reader, error) {
			read := NewReadService()
			return read.NewReader(cfg)
		},
		ResolveArticle: func(ctx context.Context, cfg config.Config, ids references.Identifiers) (references.Article, error) {
			return newReferenceClient(cfg).ResolveArticle(ctx, ids, false)
		},
		NewIndexBuilder: func() itemImportIndexBuilder {
			service := NewIndexService()
			return service
		},
		PollInterval: 400 * time.Millisecond,
	}
}

func (s ItemImportService) Import(ctx context.Context, req ItemImportRequest) (Result, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	source := strings.TrimSpace(req.Source)
	if strings.TrimSpace(req.FromName) != "" || len(req.FromData) > 0 || isMetadataImportSource(source) {
		return s.importMetadata(ctx, cfg, req, source)
	}
	absPath, info, err := validateImportPDF(source)
	if err != nil {
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
	client := s.NewClient(cfg)
	if err := client.Ping(ctx); err != nil {
		return Result{}, err
	}
	data := ItemImportResult{Status: "ready", SourceType: "pdf", Mode: cfg.Mode, Stages: map[string]string{"validation": "success", "connector": "success"}, File: absPath, Size: info.Size(), DryRun: req.DryRun, CollectionKey: collection.Key, CollectionName: collection.Name, CollectionPath: collection.Path}
	if req.DryRun {
		data.PlannedActions = []string{"upload PDF through Zotero desktop connector", "queue metadata recognition when supported"}
		if collection.Key != "" {
			data.PlannedActions = append(data.PlannedActions, "assign item to collection "+collection.Key)
		}
		return Result{Data: data, Meta: map[string]any{"dry_run": true}, Text: fmt.Sprintf("dry run: %s is ready to import into Zotero desktop", absPath)}, nil
	}
	if !cfg.AllowWrite {
		return Result{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
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
	data.Stages["upload"] = "success"
	if collection.ID > 0 {
		if err := client.UpdateSession(ctx, zoteroconnector.UpdateSessionRequest{SessionID: sessionID, Target: fmt.Sprintf("C%d", collection.ID)}); err != nil {
			data.Status = "partial"
			data.Stages["collection"] = "failed"
			return Result{Data: data, Meta: map[string]any{"write_source": "zotero_connector"}, Text: fmt.Sprintf("imported %s into Zotero desktop", absPath), Warnings: []Warning{{Code: "collection_assignment_failed", Message: err.Error()}}}, nil
		}
		data.CollectionAssigned = true
		data.Stages["collection"] = "success"
	}
	data.CanRecognize = imported.CanRecognize
	data.RecognitionQueued = imported.CanRecognize
	text := fmt.Sprintf("imported %s into Zotero desktop", absPath)
	if imported.CanRecognize {
		data.Stages["recognition"] = "queued"
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
			data.Stages["recognition"] = "warning"
			result.Warnings = append(result.Warnings, Warning{Code: "recognition_wait_failed", Message: waitErr.Error()})
		} else if recognized {
			data.Stages["recognition"] = "success"
			if resolver == nil {
				resolver, err = s.NewResolver(cfg)
			}
			if err != nil {
				data.Stages["attachment_resolution"] = "failed"
				result.Warnings = append(result.Warnings, Warning{Code: "duplicate_cleanup_failed", Message: err.Error()})
			} else {
				attachments, waitAttachmentsErr := s.waitForImportedAttachments(ctx, resolver, importFileURL(absPath))
				if waitAttachmentsErr != nil {
					data.Stages["attachment_resolution"] = "failed"
					result.Warnings = append(result.Warnings, Warning{Code: "import_attachment_wait_failed", Message: waitAttachmentsErr.Error()})
				} else {
					data.Stages["attachment_resolution"] = "success"
				}
				cleanup, cleanupErr := s.cleanupDuplicateAttachments(ctx, cfg, resolver, importFileURL(absPath))
				data.DuplicateCleanup = cleanup
				data.ItemKey, data.AttachmentKey = importedAttachmentTarget(attachments, cleanup)
				result.Data = data
				if cleanupErr != nil {
					data.Stages["duplicate_cleanup"] = "failed"
					result.Warnings = append(result.Warnings, Warning{Code: "duplicate_cleanup_failed", Message: cleanupErr.Error()})
				} else {
					data.Stages["duplicate_cleanup"] = "success"
				}
				if data.ItemKey != "" && data.AttachmentKey != "" && s.NewIndexBuilder != nil {
					indexed, indexErr := s.NewIndexBuilder().Build(ctx, IndexBuildRequest{Workers: 1, ItemKeys: []string{data.ItemKey}, AttachmentKeys: []string{data.AttachmentKey}})
					if indexErr != nil {
						data.Stages["full_text_index"] = "failed"
						result.Warnings = append(result.Warnings, Warning{Code: "full_text_index_failed", Message: indexErr.Error()})
					} else if indexResult, ok := indexed.Data.(IndexBuildResult); ok {
						data.FullTextIndexed = indexResult.Indexed+indexResult.Skipped > 0 && indexResult.Failed == 0
						if data.FullTextIndexed {
							data.Stages["full_text_index"] = "success"
						} else {
							data.Stages["full_text_index"] = "failed"
						}
						result.Data = data
					}
				}
			}
		} else {
			data.Stages["recognition"] = "not_completed"
		}
	} else {
		data.Stages["recognition"] = "not_supported"
	}
	if data.Status != "partial" {
		if len(result.Warnings) > 0 {
			data.Status = "partial"
		} else {
			data.Status = "success"
		}
		result.Data = data
	}
	return result, nil
}

func (s ItemImportService) waitForImportedAttachments(ctx context.Context, resolver itemImportCollectionResolver, sourceURL string) ([]backend.ImportedAttachment, error) {
	interval := s.PollInterval
	if interval <= 0 {
		interval = 400 * time.Millisecond
	}
	var previous string
	var latest []backend.ImportedAttachment
	for attempt := 0; attempt < 20; attempt++ {
		attachments, err := resolver.ImportedPDFAttachments(ctx, sourceURL)
		if err != nil {
			return nil, err
		}
		if importedAttachmentsReady(attachments) {
			signature := importedAttachmentSignature(attachments)
			latest = attachments
			if signature == previous {
				return latest, nil
			}
			previous = signature
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
	if len(latest) > 0 {
		return latest, nil
	}
	return nil, fmt.Errorf("Zotero did not expose the imported PDF attachment before timeout")
}

func importedAttachmentsReady(attachments []backend.ImportedAttachment) bool {
	for _, attachment := range attachments {
		if attachment.Key != "" && attachment.ParentKey != "" && attachment.Path != "" {
			return true
		}
	}
	return false
}

func importedAttachmentSignature(attachments []backend.ImportedAttachment) string {
	parts := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		parts = append(parts, attachment.Key+"\x00"+attachment.ParentKey+"\x00"+attachment.Path)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

func importedAttachmentTarget(attachments []backend.ImportedAttachment, cleanup *DuplicateCleanupResult) (string, string) {
	keep := ""
	if cleanup != nil {
		keep = cleanup.Kept
	}
	candidates := append([]backend.ImportedAttachment(nil), attachments...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].LinkMode != candidates[j].LinkMode {
			return candidates[i].LinkMode == 2
		}
		return candidates[i].Key < candidates[j].Key
	})
	for _, attachment := range candidates {
		if attachment.ParentKey == "" || attachment.Path == "" || (keep != "" && attachment.Key != keep) {
			continue
		}
		return attachment.ParentKey, attachment.Key
	}
	return "", ""
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
	version, err := client.GetLibraryVersion(ctx)
	if err != nil {
		cleanup.Deleted = nil
		return cleanup, err
	}
	deleted := append([]string(nil), cleanup.Deleted...)
	batch, err := deleteKeysInBatches(ctx, client.DeleteItems, deleted, version)
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
