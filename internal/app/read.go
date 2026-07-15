package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type ListOptions struct {
	Limit    int
	Offset   int
	Query    string
	Top      bool
	Scope    string
	Sort     string
	Order    string
	ItemType string
	Tags     []string
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
	OpenFile   func(string) error
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
		OpenFile: openSystemFile,
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
		if m.FullTextEngine != "" {
			meta["full_text_engine"] = m.FullTextEngine
		}
		if m.FullTextSource != "" {
			meta["full_text_source"] = m.FullTextSource
		}
		if m.FullTextAttachmentKey != "" {
			meta["full_text_attachment_key"] = m.FullTextAttachmentKey
		}
		if m.FullTextCacheHit {
			meta["full_text_cache_hit"] = true
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
		text := collectionText(rows)
		if len(rows) == 0 {
			text = "no top-level collections found"
		}
		return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web"}, Text: text}, nil
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
	text := backendCollectionText(rows)
	if len(rows) == 0 {
		text = "no collections found"
	}
	return Result{Data: rows, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func (s ReadService) ShowCollection(ctx context.Context, key string) (Result, error) {
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	rows, err := reader.ListCollections(ctx)
	if err != nil {
		return Result{}, err
	}
	for _, row := range rows {
		if row.Key == key {
			meta := readMeta(reader)
			meta["total"] = 1
			return Result{Data: row, Meta: meta, Text: fmt.Sprintf("Key: %s\nName: %s\nItems: %d", row.Key, row.Name, row.NumItems), Warnings: readWarnings(meta)}, nil
		}
	}
	return Result{}, fmt.Errorf("collection %q not found", key)
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
		tagType := "manual"
		if row.Type == 1 {
			tagType = "automatic"
		}
		lines = append(lines, fmt.Sprintf("%-20s  items=%d  type=%s", row.Name, row.NumItems, tagType))
	}
	text := strings.Join(lines, "\n")
	if len(rows) == 0 {
		text = "no tags found"
	}
	return Result{Data: rows, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
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
	if q := strings.TrimSpace(opts.Query); q != "" {
		pattern, err := regexp.Compile("(?i:" + q + ")")
		if err != nil {
			return Result{}, fmt.Errorf("invalid note regular expression: %w", err)
		}
		filtered := rows[:0]
		for _, row := range rows {
			if pattern.MatchString(row.Preview) || pattern.MatchString(row.Content) {
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
		if isMachineNote(row.Content) {
			continue
		}
		parent := ""
		if row.ParentItemKey != "" {
			parent = fmt.Sprintf("  [%s]", row.ParentItemKey)
		}
		lines = append(lines, fmt.Sprintf("%-10s%s  %s", row.Key, parent, row.Preview))
	}
	text := strings.Join(lines, "\n")
	if len(lines) == 0 {
		text = "no readable notes found in text mode; use --json to inspect all notes"
	}
	return Result{Data: rows, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func (s ReadService) ShowNote(ctx context.Context, key string) (Result, error) {
	result, err := s.Notes(ctx, ListOptions{})
	if err != nil {
		return Result{}, err
	}
	rows, _ := result.Data.([]domain.Note)
	for _, row := range rows {
		if row.Key == key {
			result.Data = row
			result.Meta["total"] = 1
			result.Text = fmt.Sprintf("Key: %s\nParent: %s\n%s", row.Key, row.ParentItemKey, backend.StripHTMLTags(row.Content))
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("note %q not found", key)
}

func (s ReadService) Searches(ctx context.Context, opts ListOptions) (Result, error) {
	cfg, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	searchReader, ok := reader.(backend.SavedSearchReader)
	if !ok {
		client, clientErr := s.NewClient(cfg)
		if clientErr != nil {
			return Result{}, fmt.Errorf("configured backend does not support saved searches: %w", clientErr)
		}
		raw, clientErr := client.ListSearches(ctx)
		if clientErr != nil {
			return Result{}, clientErr
		}
		rows := make([]backend.SavedSearch, 0, len(raw))
		for _, search := range raw {
			conditions := make([]backend.SearchCondition, 0, len(search.Conditions))
			for _, condition := range search.Conditions {
				conditions = append(conditions, backend.SearchCondition{Condition: condition.Condition, Operator: condition.Operator, Value: condition.Value})
			}
			rows = append(rows, backend.SavedSearch{Key: search.Key, Name: search.Name, NumConditions: len(conditions), Conditions: conditions})
		}
		rows = limitSlice(rows, opts.Limit)
		return savedSearchResult(rows, map[string]any{"read_source": "web"}), nil
	}
	rows, err := searchReader.ListSavedSearches(ctx)
	if err != nil {
		return Result{}, err
	}
	rows = limitSlice(rows, opts.Limit)
	return savedSearchResult(rows, readMeta(reader)), nil
}

func savedSearchResult(rows []backend.SavedSearch, meta map[string]any) Result {
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-24s  conditions=%d", row.Key, row.Name, row.NumConditions))
	}
	text := strings.Join(lines, "\n")
	if len(rows) == 0 {
		text = "no saved searches found"
	}
	meta["total"] = len(rows)
	return Result{Data: rows, Meta: meta, Text: text, Warnings: readWarnings(meta)}
}

func (s ReadService) ShowSearch(ctx context.Context, key string) (Result, error) {
	result, err := s.Searches(ctx, ListOptions{})
	if err != nil {
		return Result{}, err
	}
	rows, _ := result.Data.([]backend.SavedSearch)
	for _, row := range rows {
		if row.Key == key {
			result.Data = row
			result.Meta["total"] = 1
			result.Text = fmt.Sprintf("Key: %s\nName: %s\nConditions: %d", row.Key, row.Name, row.NumConditions)
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("saved search %q not found", key)
}

func (s ReadService) Groups(ctx context.Context, _ ListOptions) (Result, error) {
	cfg, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	userID := ""
	if cfg.LibraryType == "user" {
		userID = strings.TrimSpace(cfg.LibraryID)
	}
	if userID == "" {
		key, err := client.GetCurrentKeyInfo(ctx)
		if err != nil {
			return Result{}, err
		}
		userID = fmt.Sprintf("%d", key.UserID)
	}
	rows, err := client.ListGroupsForUser(ctx, userID)
	if err != nil {
		return Result{}, err
	}
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-8d  %s", row.ID, row.Name))
	}
	text := strings.Join(lines, "\n")
	if len(rows) == 0 {
		text = "no groups found for the current api key"
	}
	return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web"}, Text: text}, nil
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
	localData := inspectLocalData(reader)
	fullTextIndex := inspectFullTextIndex(cfg, reader, false)
	_, configPath, _ := s.LoadConfig()
	taste, tasteErr := LoadLibraryTaste(cfg, configPath)
	if tasteErr != nil {
		return Result{}, tasteErr
	}
	tasteStatus := LibraryTaste{Path: taste.Path, Exists: taste.Exists}
	data := map[string]any{"stats": a.stats, "collections": a.collections, "tags": a.tags, "recent_items": a.items, "local_data": localData, "fulltext_index": fullTextIndex, "taste": tasteStatus}
	meta := readMeta(reader)
	meta["total_items"] = a.stats.TotalItems
	meta["taste_path"] = taste.Path
	meta["taste_exists"] = taste.Exists
	text := overviewText(a.stats, a.collections, a.tags, a.items, localData, fullTextIndex)
	warnings := readWarnings(meta)
	if taste.Exists {
		text += "\nTaste: configured (" + taste.Path + ")"
	} else {
		text += "\nTaste: not configured\nCreate: zot lib taste --init\nPath: " + taste.Path
		warnings = append(warnings, Warning{Code: "taste_missing", Message: "library taste is not configured; run `zot lib taste --init`"})
	}
	return Result{Data: data, Meta: meta, Text: text, Warnings: warnings}, nil
}

func (s ReadService) Items(ctx context.Context, opts ListOptions) (Result, error) {
	if opts.Scope == "" {
		_, reader, err := s.reader()
		if err != nil {
			return Result{}, err
		}
		rows, err := reader.FindItems(ctx, backend.FindOptions{All: true, Limit: opts.Limit, Start: opts.Offset, Sort: opts.Sort, Direction: opts.Order, ItemType: opts.ItemType, Tags: opts.Tags})
		if err != nil {
			return Result{}, err
		}
		meta := readMeta(reader)
		meta["total"] = len(rows)
		text := domainItemText(rows)
		if len(rows) == 0 {
			text = "no items found"
		}
		return Result{Data: rows, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
	}
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	var rows []zoteroapi.Item
	find := zoteroapi.FindOptions{Limit: opts.Limit, Start: opts.Offset, Sort: opts.Sort, Direction: opts.Order, ItemType: opts.ItemType, Tags: opts.Tags}
	switch opts.Scope {
	case "trash":
		rows, err = client.ListTrashItems(ctx, find)
	case "pubs":
		rows, err = client.ListPublicationsItems(ctx, find)
	default:
		return Result{}, fmt.Errorf("unsupported item scope %q", opts.Scope)
	}
	if err != nil {
		return Result{}, err
	}
	text := itemText(rows)
	if len(rows) == 0 {
		if opts.Scope == "trash" {
			text = "trash is empty"
		} else {
			text = "no publications found"
		}
	}
	return Result{Data: rows, Meta: map[string]any{"total": len(rows), "read_source": "web", "scope": opts.Scope}, Text: text}, nil
}

func domainItemText(rows []domain.Item) string {
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s  %-16s  %s", row.Key, row.ItemType, row.Title))
	}
	return strings.Join(lines, "\n")
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
		return Result{Data: r.Versions, Meta: meta, Text: fmt.Sprintf("not modified since version %d", opts.IfModifiedVersion)}, nil
	}
	if r.LastModifiedVersion > 0 {
		meta["last_modified_version"] = r.LastModifiedVersion
	}
	return Result{Data: r.Versions, Meta: meta, Text: strings.Join(lines, "\n")}, nil
}

func isMachineNote(content string) bool {
	return strings.Contains(strings.TrimSpace(content), `{"readingTime":`)
}

func overviewText(stats backend.LibraryStats, collections []backend.Collection, tags []backend.Tag, items []domain.Item, localData LocalDataStatus, fullTextIndex FullTextIndexStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Library: %s:%s\n", stats.LibraryType, stats.LibraryID)
	fmt.Fprintf(&b, "Items: %d | Collections: %d | Searches: %d\n", stats.TotalItems, stats.TotalCollections, stats.TotalSearches)
	if stats.LastLibraryVersion > 0 {
		fmt.Fprintf(&b, "Version: %d\n", stats.LastLibraryVersion)
	}
	if len(collections) > 0 {
		b.WriteString("\nTop Collections:\n")
		for _, collection := range collections {
			fmt.Fprintf(&b, "  %s (%d items)\n", collection.Name, collection.NumItems)
		}
	}
	if len(tags) > 0 {
		b.WriteString("Top Tags:\n")
		for _, tag := range tags {
			fmt.Fprintf(&b, "  %s (%d items)\n", tag.Name, tag.NumItems)
		}
	}
	if len(items) > 0 {
		b.WriteString("Recent Items:\n")
		for _, item := range items {
			fmt.Fprintf(&b, "  %-10s  %s\n", item.Key, item.Title)
		}
	}
	fmt.Fprintf(&b, "\nLocal data: %s", localData.Status)
	if localData.Path != "" {
		fmt.Fprintf(&b, "\nData dir: %s", localData.Path)
	}
	fmt.Fprintf(&b, "\nFull-text index: %s", fullTextIndex.Status)
	if fullTextIndex.Path != "" {
		fmt.Fprintf(&b, "\nIndex path: %s", fullTextIndex.Path)
	}
	if fullTextIndex.Status == "unavailable" {
		b.WriteString("\nBuild: zot index build")
	}
	return b.String()
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
