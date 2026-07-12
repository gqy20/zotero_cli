package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/references"
)

type ReferenceShowRequest struct {
	Key     string
	Source  string
	Refresh bool
}

type ReferenceBuildRequest struct {
	Workers  int
	Limit    int
	Source   string
	Force    bool
	Refresh  bool
	Failed   bool
	Contexts bool
	Grobid   bool
	All      bool
}

type ReferenceStatusRequest struct {
	Failed      bool
	Unsupported bool
	Grobid      bool
}

type ReferenceFindRequest struct {
	Options references.SearchOptions
}

type ReferenceDiscoveryRequest struct {
	Key        string
	Limit      int
	Refresh    bool
	External   bool
	AlsoViewed bool
}

type ReferenceService struct {
	LoadConfig        func() (config.Config, string, error)
	NewReader         func(config.Config) (backend.Reader, error)
	OpenStore         func(config.Config) (*references.Store, error)
	OpenReadOnlyStore func(config.Config) (*references.Store, error)
	NewClient         func(config.Config) *references.Client
}

func NewReferenceService() ReferenceService {
	read := NewReadService()
	return ReferenceService{LoadConfig: config.Load, NewReader: read.NewReader, OpenStore: openReferenceStore, OpenReadOnlyStore: openReferenceStoreReadOnly, NewClient: newReferenceClient}
}

func (s ReferenceService) config() (config.Config, error) {
	cfg, _, err := s.LoadConfig()
	return cfg, err
}

func (s ReferenceService) reader(cfg config.Config) (backend.Reader, error) { return s.NewReader(cfg) }

func validReferenceSource(source string) bool {
	return source == "auto" || source == "pmc" || source == "pubmed"
}

func (s ReferenceService) Show(ctx context.Context, req ReferenceShowRequest) (Result, error) {
	if req.Source == "" {
		req.Source = "auto"
	}
	if !validReferenceSource(req.Source) {
		return Result{}, fmt.Errorf("invalid reference source %q", req.Source)
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return Result{}, err
	}
	item, err := reader.GetItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	indexHit := false
	var result references.Result
	if !req.Refresh && req.Source == "auto" {
		if stored, ok, loadErr := store.LoadResult(ctx, req.Key); loadErr != nil {
			return Result{}, loadErr
		} else if ok {
			result = stored
			result.CacheHits++
			indexHit = true
		}
	}
	if !indexHit {
		result, err = references.NewService(s.NewClient(cfg)).References(ctx, item, references.Options{Source: req.Source, Refresh: req.Refresh})
		if err != nil {
			return Result{}, err
		}
		if err := store.SaveResult(ctx, result, references.Fingerprint(item)); err != nil {
			return Result{}, err
		}
	}
	meta := map[string]any{"total": len(result.References), "strategy": result.Strategy, "index_hit": indexHit, "cache_hits": result.CacheHits, "network_calls": result.NetworkCalls, "elapsed_ms": result.ElapsedMS}
	return Result{Data: result, Meta: meta, Text: referenceResultText(result)}, nil
}

func (s ReferenceService) Build(ctx context.Context, req ReferenceBuildRequest) (Result, error) {
	if req.Grobid {
		return s.GrobidBuild(ctx, req)
	}
	if req.Workers == 0 {
		req.Workers = 3
	}
	if req.Source == "" {
		req.Source = "auto"
	}
	if req.Workers < 1 || req.Workers > 16 {
		return Result{}, fmt.Errorf("--workers must be between 1 and 16")
	}
	if req.Limit < 0 || !validReferenceSource(req.Source) {
		return Result{}, fmt.Errorf("invalid reference build options")
	}
	if req.Failed && req.Contexts {
		return Result{}, fmt.Errorf("--failed and --contexts are mutually exclusive")
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	var items []domain.Item
	if req.Failed {
		failed, err := store.Failed(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, row := range failed {
			if item, getErr := reader.GetItem(ctx, row.ItemKey); getErr == nil {
				items = append(items, item)
			}
		}
		req.Force = true
	} else if req.Contexts {
		pending, err := store.ContextPending(ctx, req.Limit)
		if err != nil {
			return Result{}, err
		}
		for _, row := range pending {
			item, getErr := reader.GetItem(ctx, row.ItemKey)
			if getErr != nil {
				return Result{}, getErr
			}
			items = append(items, item)
		}
		req.Force = true
		req.Source = "pmc"
	} else {
		items, err = reader.FindItems(ctx, backend.FindOptions{All: true, Full: true})
		if err != nil {
			return Result{}, err
		}
	}
	report, err := references.NewBuilder(references.NewService(s.NewClient(cfg)), store).Build(ctx, items, references.BuildOptions{Workers: req.Workers, Force: req.Force, Refresh: req.Refresh || req.Force, Source: req.Source, Limit: req.Limit})
	if err != nil {
		return Result{}, err
	}
	meta := map[string]any{"processed": report.Processed, "succeeded": report.Succeeded, "unsupported": report.Unsupported, "failed": report.Failed, "elapsed_ms": report.ElapsedMS, "workers": req.Workers, "scope_failed": req.Failed, "scope_contexts": req.Contexts}
	return Result{Data: report, Meta: meta, Text: fmt.Sprintf("Reference build: %d succeeded, %d unsupported, %d failed, %d skipped; %d references (%s)", report.Succeeded, report.Unsupported, report.Failed, report.Skipped, report.References, time.Duration(report.ElapsedMS)*time.Millisecond)}, nil
}

func (s ReferenceService) Status(ctx context.Context, req ReferenceStatusRequest) (Result, error) {
	if req.Grobid {
		return s.GrobidStatus(ctx)
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	indexPath := referenceStorePath(cfg)
	store, err := s.OpenReadOnlyStore(cfg)
	if os.IsNotExist(err) {
		status := references.Status{}
		text := fmt.Sprintf("Reference index is not initialized\nPath: %s", indexPath)
		return Result{Data: status, Meta: map[string]any{"index_path": indexPath, "initialized": false, "read_mode": "none"}, Text: text}, nil
	}
	if err != nil {
		return Result{}, err
	}
	readMode := "read_only"
	closeStore := func() { _ = store.Close() }
	defer func() { closeStore() }()
	reopenForMigration := func() error {
		closeStore()
		migrated, openErr := s.OpenStore(cfg)
		if openErr != nil {
			return openErr
		}
		store = migrated
		closeStore = func() { _ = store.Close() }
		readMode = "migrated"
		return nil
	}
	if req.Failed {
		rows, err := store.Failed(ctx)
		if references.IsSchemaError(err) {
			if err = reopenForMigration(); err == nil {
				rows, err = store.Failed(ctx)
			}
		}
		if err != nil {
			return Result{}, err
		}
		return Result{Data: rows, Meta: map[string]any{"total": len(rows), "index_path": store.Path(), "initialized": true, "read_mode": readMode}, Text: failedReferenceText(rows)}, nil
	}
	if req.Unsupported {
		rows, err := store.Unsupported(ctx)
		if references.IsSchemaError(err) {
			if err = reopenForMigration(); err == nil {
				rows, err = store.Unsupported(ctx)
			}
		}
		if err != nil {
			return Result{}, err
		}
		return Result{Data: rows, Meta: map[string]any{"total": len(rows), "index_path": store.Path(), "initialized": true, "read_mode": readMode}, Text: failedReferenceText(rows)}, nil
	}
	status, cacheHit, err := store.CachedStatus(ctx)
	if references.IsSchemaError(err) {
		if err = reopenForMigration(); err == nil {
			status, cacheHit, err = store.CachedStatus(ctx)
		}
	}
	if err != nil {
		return Result{}, err
	}
	text := fmt.Sprintf("Reference index: %d items (%d successful, %d unsupported, %d failed), %d references\nResolved: %d  Unresolved: %d  Contexts: %d\nPath: %s", status.IndexedItems, status.SuccessfulItems, status.UnsupportedItems, status.FailedItems, status.TotalReferences, status.ResolvedReferences, status.UnresolvedReferences, status.CitationContexts, store.Path())
	return Result{Data: status, Meta: map[string]any{"index_path": store.Path(), "initialized": true, "read_mode": readMode, "status_cache_hit": cacheHit}, Text: text}, nil
}

func (s ReferenceService) Resolve(ctx context.Context, workers int) (Result, error) {
	if workers == 0 {
		workers = min(runtime.NumCPU(), 16)
	}
	if workers < 1 || workers > 32 {
		return Result{}, fmt.Errorf("--workers must be between 1 and 32")
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return Result{}, err
	}
	items, err := reader.FindItems(ctx, backend.FindOptions{All: true, Full: true})
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	report, err := store.Resolve(ctx, references.NewResolver(items), workers)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: report, Meta: map[string]any{"library_items": len(items), "workers": workers, "elapsed_ms": report.ElapsedMS}, Text: fmt.Sprintf("Resolved %d/%d references in %s", report.Resolved, report.Total, time.Duration(report.ElapsedMS)*time.Millisecond)}, nil
}

func (s ReferenceService) Find(ctx context.Context, req ReferenceFindRequest) (Result, error) {
	if req.Options.Limit == 0 {
		req.Options.Limit = 20
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		return Result{}, fmt.Errorf("reference query is required")
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	hits, err := store.Search(ctx, req.Options)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: hits, Meta: map[string]any{"query": req.Options.Query, "scope": req.Options.In, "total": len(hits), "limit": req.Options.Limit}, Text: referenceSearchText(hits)}, nil
}

func (s ReferenceService) Related(ctx context.Context, req ReferenceDiscoveryRequest) (Result, error) {
	if req.Limit == 0 {
		req.Limit = 20
	}
	cfg, item, err := s.referenceItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	rows, ids, err := references.NewService(s.NewClient(cfg)).Related(ctx, item, req.Limit, req.AlsoViewed, req.Refresh)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: rows, Meta: map[string]any{"item_key": req.Key, "pmid": ids.PMID, "total": len(rows), "limit": req.Limit, "also_viewed": req.AlsoViewed}, Text: fmt.Sprintf("PubMed related articles for %s: %d", req.Key, len(rows))}, nil
}

func (s ReferenceService) Links(ctx context.Context, req ReferenceDiscoveryRequest) (Result, error) {
	cfg, item, err := s.referenceItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	rows, ids, err := references.NewService(s.NewClient(cfg)).Links(ctx, item, req.Refresh)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: rows, Meta: map[string]any{"item_key": req.Key, "pmid": ids.PMID, "total": len(rows)}, Text: fmt.Sprintf("NCBI resource links for %s: %d type(s)", req.Key, len(rows))}, nil
}

func (s ReferenceService) Entities(ctx context.Context, req ReferenceDiscoveryRequest) (Result, error) {
	cfg, item, err := s.referenceItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	rows, ids, err := references.NewService(s.NewClient(cfg)).Annotations(ctx, item, req.Refresh)
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	if err := store.SaveAnnotations(ctx, req.Key, rows); err != nil {
		return Result{}, err
	}
	return Result{Data: rows, Meta: map[string]any{"item_key": req.Key, "pmid": ids.PMID, "total": len(rows), "source": "europe_pmc"}, Text: fmt.Sprintf("Europe PMC entities for %s: %d", req.Key, len(rows))}, nil
}

func (s ReferenceService) Profile(ctx context.Context, req ReferenceDiscoveryRequest) (Result, error) {
	cfg, item, err := s.referenceItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	profile, err := references.NewService(s.NewClient(cfg)).Profile(ctx, item, req.Refresh)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: profile, Meta: map[string]any{"item_key": req.Key, "source": "europe_pmc"}, Text: fmt.Sprintf("Europe PMC profile for %s: %s/%s, cited by %d, OA %v (%s)", req.Key, profile.Source, profile.ID, profile.CitedByCount, profile.OpenAccess, profile.License)}, nil
}

func (s ReferenceService) Cited(ctx context.Context, req ReferenceDiscoveryRequest) (Result, error) {
	if req.External {
		if req.Limit == 0 {
			req.Limit = 100
		}
		cfg, item, err := s.referenceItem(ctx, req.Key)
		if err != nil {
			return Result{}, err
		}
		rows, total, ids, err := references.NewService(s.NewClient(cfg)).ExternalCitations(ctx, item, req.Limit, req.Refresh)
		if err != nil {
			return Result{}, err
		}
		return Result{Data: rows, Meta: map[string]any{"target_item_key": req.Key, "pmid": ids.PMID, "returned": len(rows), "total": total, "source": "europe_pmc"}, Text: fmt.Sprintf("Cited by %d external Europe PMC item(s), showing %d", total, len(rows))}, nil
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	rows, err := store.CitedBy(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	return Result{Data: rows, Meta: map[string]any{"target_item_key": req.Key, "total": len(rows)}, Text: fmt.Sprintf("Cited by %d indexed item(s)", len(rows))}, nil
}

func (s ReferenceService) Contexts(ctx context.Context, key string) (Result, error) {
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	rows, err := store.Contexts(ctx, key)
	if err != nil {
		return Result{}, err
	}
	summary, found, err := store.ContextSummary(ctx, key)
	if err != nil {
		return Result{}, err
	}
	meta := map[string]any{"item_key": key, "total": len(rows), "context_summary": summary, "summary_found": found}
	return Result{Data: rows, Meta: meta, Text: fmt.Sprintf("Citation contexts for %s: %d (status %s, coverage %.1f%%)", key, len(rows), summary.Status, summary.Coverage*100)}, nil
}

func (s ReferenceService) referenceItem(ctx context.Context, key string) (config.Config, domain.Item, error) {
	cfg, err := s.config()
	if err != nil {
		return config.Config{}, domain.Item{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return config.Config{}, domain.Item{}, err
	}
	item, err := reader.GetItem(ctx, key)
	return cfg, item, err
}

func openReferenceStore(cfg config.Config) (*references.Store, error) {
	return references.OpenStore(referenceStorePath(cfg))
}

func openReferenceStoreReadOnly(cfg config.Config) (*references.Store, error) {
	return references.OpenStoreReadOnly(referenceStorePath(cfg))
}

func referenceStorePath(cfg config.Config) string {
	return filepath.Join(referenceRoot(cfg), "index.sqlite")
}
func referenceRoot(cfg config.Config) string {
	if cfg.DataDir != "" {
		return filepath.Join(cfg.DataDir, ".zotero_cli", "ref")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zot", ".zotero_cli", "ref")
}

func newReferenceClient(cfg config.Config) *references.Client {
	root := referenceRoot(cfg)
	interval := 350 * time.Millisecond
	apiKey := strings.TrimSpace(os.Getenv("ZOT_NCBI_API_KEY"))
	if apiKey != "" {
		interval = 110 * time.Millisecond
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return references.NewClient(references.ClientConfig{BaseURL: os.Getenv("ZOT_NCBI_BASE_URL"), APIKey: apiKey, Email: os.Getenv("ZOT_NCBI_EMAIL"), CacheDir: filepath.Join(root, "ncbi"), MinInterval: interval, MaxAttempts: cfg.RetryMaxAttempts, HTTPClient: &http.Client{Timeout: timeout}})
}

func referenceResultText(result references.Result) string {
	return fmt.Sprintf("References for %s: %d via %s", result.ItemKey, len(result.References), result.Strategy)
}
func failedReferenceText(rows []references.FailedItem) string {
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%-10s attempts=%d %s %s", row.ItemKey, row.Attempts, row.Title, row.Error)
	}
	return b.String()
}
func referenceSearchText(hits []references.SearchHit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reference search: %d result(s)", len(hits))
	for _, hit := range hits {
		fmt.Fprintf(&b, "\n%-10s ref %-4d %s", hit.SourceItemKey, hit.Reference.Index, hit.Reference.Title)
	}
	return b.String()
}
