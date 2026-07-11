package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type ListOptions struct {
	Limit int
	Query string
	Top   bool
	Scope string
}

type LogOptions struct {
	Kind              string
	Since             int
	Deleted           bool
	IncludeTrashed    bool
	IfModifiedVersion int
}

type ReadService struct {
	LoadConfig func() (config.Config, string, error)
	NewReader  func(config.Config) (backend.Reader, error)
	NewClient  func(config.Config) (*zoteroapi.Client, error)
}

func NewReadService() ReadService {
	return ReadService{
		LoadConfig: config.Load,
		NewReader: func(cfg config.Config) (backend.Reader, error) {
			return backend.NewReader(cfg, nil)
		},
		NewClient: func(cfg config.Config) (*zoteroapi.Client, error) {
			normalized := cfg
			switch cfg.Mode {
			case "", "web", "hybrid":
				normalized.Mode = "web"
			case "remote":
				if cfg.APIKey == "" || cfg.LibraryID == "" {
					return nil, fmt.Errorf("web API commands are not available in remote mode without API key")
				}
				normalized.Mode = "web"
			case "local":
				return nil, fmt.Errorf("web API commands are not available in local mode; use web or hybrid mode")
			default:
				return nil, fmt.Errorf("unsupported mode %q", cfg.Mode)
			}
			return zoteroapi.New(normalized, os.Getenv("ZOT_BASE_URL"), http.DefaultClient), nil
		},
	}
}

func (s ReadService) reader() (config.Config, backend.Reader, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	r, err := s.NewReader(cfg)
	return cfg, r, err
}

func (s ReadService) client() (config.Config, *zoteroapi.Client, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	c, err := s.NewClient(cfg)
	return cfg, c, err
}

func readMeta(reader backend.Reader) map[string]any {
	meta := map[string]any{}
	if reporter, ok := reader.(interface{ ConsumeReadMetadata() backend.ReadMetadata }); ok {
		m := reporter.ConsumeReadMetadata()
		if m.ReadSource != "" {
			meta["read_source"] = m.ReadSource
		}
		if m.SQLiteFallback {
			meta["sqlite_fallback"] = true
		}
		if m.SnapshotStale {
			meta["snapshot_stale"] = true
		}
	}
	return meta
}

func limitSlice[T any](values []T, limit int) []T {
	if limit > 0 && len(values) > limit {
		return values[:limit]
	}
	return values
}

func (s ReadService) Collections(ctx context.Context, opts ListOptions) (Result, error) {
	if opts.Top {
		_, client, err := s.client()
		if err != nil {
			return Result{}, err
		}
		rows, err := client.ListTopCollections(ctx)
		if err != nil {
			return Result{}, err
		}
		rows = limitSlice(rows, opts.Limit)
		return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web"}, Text: collectionText(rows)}, nil
	}
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	rows, err := reader.ListCollections(ctx)
	if err != nil {
		return Result{}, err
	}
	rows = limitSlice(rows, opts.Limit)
	meta := readMeta(reader)
	meta["total"] = len(rows)
	return Result{Data: rows, Meta: meta, Text: backendCollectionText(rows), Warnings: readWarnings(meta)}, nil
}

func (s ReadService) Tags(ctx context.Context, opts ListOptions) (Result, error) {
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	rows, err := reader.ListTags(ctx)
	if err != nil {
		return Result{}, err
	}
	rows = limitSlice(rows, opts.Limit)
	meta := readMeta(reader)
	meta["total"] = len(rows)
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-20s  items=%d", row.Name, row.NumItems))
	}
	return Result{Data: rows, Meta: meta, Text: strings.Join(lines, "\n"), Warnings: readWarnings(meta)}, nil
}

func (s ReadService) Notes(ctx context.Context, opts ListOptions) (Result, error) {
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	rows, err := reader.ListNotes(ctx)
	if err != nil {
		return Result{}, err
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Query)); q != "" {
		filtered := rows[:0]
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.Preview), q) || strings.Contains(strings.ToLower(row.Content), q) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	rows = limitSlice(rows, opts.Limit)
	meta := readMeta(reader)
	meta["total"] = len(rows)
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %s", row.Key, row.Preview))
	}
	return Result{Data: rows, Meta: meta, Text: strings.Join(lines, "\n"), Warnings: readWarnings(meta)}, nil
}

func (s ReadService) Searches(ctx context.Context, opts ListOptions) (Result, error) {
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	rows, err := client.ListSearches(ctx)
	if err != nil {
		return Result{}, err
	}
	rows = limitSlice(rows, opts.Limit)
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-24s  conditions=%d", row.Key, row.Name, row.NumConditions))
	}
	return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web"}, Text: strings.Join(lines, "\n")}, nil
}

func (s ReadService) Groups(ctx context.Context, _ ListOptions) (Result, error) {
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	key, err := client.GetCurrentKeyInfo(ctx)
	if err != nil {
		return Result{}, err
	}
	rows, err := client.ListGroupsForUser(ctx, fmt.Sprintf("%d", key.UserID))
	if err != nil {
		return Result{}, err
	}
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-8d  %s", row.ID, row.Name))
	}
	return Result{Data: rows, Meta: map[string]any{"total": len(rows)}, Text: strings.Join(lines, "\n")}, nil
}

func (s ReadService) Stats(ctx context.Context) (Result, error) {
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	stats, err := reader.GetLibraryStats(ctx)
	if err != nil {
		return Result{}, err
	}
	meta := readMeta(reader)
	meta["total"] = stats.TotalItems
	text := fmt.Sprintf("library=%s:%s\nitems=%d\ncollections=%d\nsearches=%d", stats.LibraryType, stats.LibraryID, stats.TotalItems, stats.TotalCollections, stats.TotalSearches)
	return Result{Data: stats, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func (s ReadService) Overview(ctx context.Context) (Result, error) {
	cfg, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	type aggregate struct {
		stats       backend.LibraryStats
		collections []backend.Collection
		tags        []backend.Tag
		items       []domain.Item
		err         error
	}
	var a aggregate
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); a.stats, a.err = reader.GetLibraryStats(ctx) }()
	go func() { defer wg.Done(); a.collections, _ = reader.ListCollections(ctx) }()
	go func() { defer wg.Done(); a.tags, _ = reader.ListTags(ctx) }()
	go func() { defer wg.Done(); a.items, _ = reader.FindItems(ctx, backend.FindOptions{Limit: 5}) }()
	wg.Wait()
	if a.err != nil {
		return Result{}, a.err
	}
	a.collections = limitSlice(a.collections, 5)
	a.tags = limitSlice(a.tags, 10)
	indexStatus := "unavailable"
	if cfg.Mode == "local" || cfg.Mode == "hybrid" {
		if cfg.DataDir == "" {
			indexStatus = "not_configured"
		} else if _, err := os.Stat(cfg.DataDir); err == nil {
			indexStatus = "available"
		} else {
			indexStatus = "data_dir_missing"
		}
	}
	data := map[string]any{"stats": a.stats, "collections": a.collections, "tags": a.tags, "recent_items": a.items}
	meta := readMeta(reader)
	meta["index_status"] = indexStatus
	meta["total_items"] = a.stats.TotalItems
	text := fmt.Sprintf("Library: %s:%s\nItems: %d | Collections: %d | Searches: %d\nIndex: %s", a.stats.LibraryType, a.stats.LibraryID, a.stats.TotalItems, a.stats.TotalCollections, a.stats.TotalSearches, indexStatus)
	return Result{Data: data, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func (s ReadService) Items(ctx context.Context, opts ListOptions) (Result, error) {
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	var rows []zoteroapi.Item
	switch opts.Scope {
	case "trash":
		rows, err = client.ListTrashItems(ctx, zoteroapi.FindOptions{Limit: opts.Limit})
	case "pubs":
		rows, err = client.ListPublicationsItems(ctx, zoteroapi.FindOptions{Limit: opts.Limit})
	default:
		return Result{}, fmt.Errorf("unsupported item scope %q", opts.Scope)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web", "scope": opts.Scope}, Text: itemText(rows)}, nil
}

func (s ReadService) Log(ctx context.Context, opts LogOptions) (Result, error) {
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	if opts.Deleted {
		d, err := client.GetDeleted(ctx)
		if err != nil {
			return Result{}, err
		}
		total := len(d.Items) + len(d.Collections) + len(d.Searches) + len(d.Tags)
		return Result{Data: d, Meta: map[string]any{"total": total, "kind": "deleted", "read_source": "web"}, Text: fmt.Sprintf("items=%d\ncollections=%d\nsearches=%d\ntags=%d", len(d.Items), len(d.Collections), len(d.Searches), len(d.Tags))}, nil
	}
	r, err := client.GetVersionsResult(ctx, zoteroapi.VersionsOptions{ObjectType: opts.Kind, Since: opts.Since, IncludeTrashed: opts.IncludeTrashed, IfModifiedSinceVersion: opts.IfModifiedVersion})
	if err != nil {
		return Result{}, err
	}
	keys := make([]string, 0, len(r.Versions))
	for k := range r.Versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%-10s  %d", k, r.Versions[k]))
	}
	meta := map[string]any{"total": len(r.Versions), "kind": opts.Kind}
	if r.NotModified {
		meta["not_modified"] = true
	}
	if r.LastModifiedVersion > 0 {
		meta["last_modified_version"] = r.LastModifiedVersion
	}
	return Result{Data: r.Versions, Meta: meta, Text: strings.Join(lines, "\n")}, nil
}

func readWarnings(meta map[string]any) []Warning {
	if stale, _ := meta["snapshot_stale"].(bool); stale {
		return []Warning{{Code: "snapshot_stale", Message: "snapshot may be stale"}}
	}
	if fallback, _ := meta["sqlite_fallback"].(bool); fallback {
		return []Warning{{Code: "sqlite_fallback", Message: "using snapshot fallback for local Zotero data"}}
	}
	return nil
}
func backendCollectionText(rows []backend.Collection) string {
	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-20s  items=%d", r.Key, r.Name, r.NumItems))
	}
	return strings.Join(lines, "\n")
}
func collectionText(rows []zoteroapi.Collection) string {
	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-20s  items=%d", r.Key, r.Name, r.NumItems))
	}
	return strings.Join(lines, "\n")
}
func itemText(rows []zoteroapi.Item) string {
	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-16s  %s", r.Key, r.ItemType, r.Title))
	}
	return strings.Join(lines, "\n")
}
