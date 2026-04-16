package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

var (
	openSQLiteDBFunc         = openSQLiteDB
	createSQLiteSnapshotFunc = createSQLiteSnapshot
	findZoteroPrefsFunc      = findZoteroPrefs
)

type LocalReader struct {
	LibraryType       string
	LibraryID         string
	DataDir           string
	SQLitePath        string
	StorageDir        string
	AttachmentBaseDir string
	lastReadMetadata  ReadMetadata
}

func NewLocalReader(cfg config.Config) (*LocalReader, error) {
	prefs, _, err := loadMatchingZoteroPrefs(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	dataDirInput := cfg.DataDir
	if dataDirInput == "" {
		dataDirInput = prefs.DataDir
	}
	if dataDirInput == "" {
		return nil, fmt.Errorf("local mode requires data_dir")
	}

	dataDir, err := filepath.Abs(dataDirInput)
	if err != nil {
		return nil, err
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	storageDir := filepath.Join(dataDir, "storage")
	if err := requireDir(dataDir, "data_dir"); err != nil {
		return nil, err
	}
	if err := requireFile(sqlitePath, "zotero.sqlite"); err != nil {
		return nil, err
	}
	if err := requireDir(storageDir, "storage"); err != nil {
		return nil, err
	}

	attachmentBaseDir := ""
	if prefs.BaseAttachmentPath != "" {
		attachmentBaseDir, err = filepath.Abs(prefs.BaseAttachmentPath)
		if err != nil {
			return nil, err
		}
	}

	return &LocalReader{
		LibraryType:       cfg.LibraryType,
		LibraryID:         cfg.LibraryID,
		DataDir:           dataDir,
		SQLitePath:        sqlitePath,
		StorageDir:        storageDir,
		AttachmentBaseDir: attachmentBaseDir,
	}, nil
}

type zoteroPrefs struct {
	DataDir            string
	BaseAttachmentPath string
}

func loadMatchingZoteroPrefs(dataDir string) (zoteroPrefs, string, error) {
	paths, err := findZoteroPrefsFunc()
	if err != nil {
		return zoteroPrefs{}, "", err
	}
	if len(paths) == 0 {
		return zoteroPrefs{}, "", nil
	}

	targetDataDir := ""
	if dataDir != "" {
		targetDataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return zoteroPrefs{}, "", err
		}
	}

	var fallback zoteroPrefs
	var fallbackPath string
	for _, prefsPath := range paths {
		prefs, err := parseZoteroPrefs(prefsPath)
		if err != nil {
			continue
		}
		if fallbackPath == "" {
			fallback = prefs
			fallbackPath = prefsPath
		}
		if targetDataDir == "" {
			continue
		}
		prefsDataDir := prefs.DataDir
		if prefsDataDir == "" {
			continue
		}
		prefsDataDir, err = filepath.Abs(prefsDataDir)
		if err != nil {
			continue
		}
		if sameFilePath(prefsDataDir, targetDataDir) {
			return prefs, prefsPath, nil
		}
	}
	if targetDataDir != "" {
		return zoteroPrefs{}, "", nil
	}
	return fallback, fallbackPath, nil
}

func findZoteroPrefs() ([]string, error) {
	patterns := []string{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		patterns = append(patterns, filepath.Join(appData, "Zotero", "Zotero", "Profiles", "*", "prefs.js"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		patterns = append(patterns,
			filepath.Join(home, ".zotero", "zotero", "Profiles", "*", "prefs.js"),
			filepath.Join(home, "Library", "Application Support", "Zotero", "Profiles", "*", "prefs.js"),
		)
	}
	seen := map[string]struct{}{}
	paths := []string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func parseZoteroPrefs(path string) (zoteroPrefs, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return zoteroPrefs{}, err
	}
	prefs := zoteroPrefs{}
	pattern := regexp.MustCompile(`^user_pref\("([^"]+)",\s*(.+)\);$`)
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		key := matches[1]
		value := strings.TrimSpace(matches[2])
		switch key {
		case "extensions.zotero.dataDir":
			parsed, err := parseZoteroPrefString(value)
			if err != nil {
				continue
			}
			prefs.DataDir = parsed
		case "extensions.zotero.baseAttachmentPath":
			parsed, err := parseZoteroPrefString(value)
			if err != nil {
				continue
			}
			prefs.BaseAttachmentPath = parsed
		}
	}
	return prefs, nil
}

func parseZoteroPrefString(value string) (string, error) {
	return strconv.Unquote(value)
}

func sameFilePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func (r *LocalReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	if opts.IncludeTrashed {
		return nil, newUnsupportedFeatureErrorWithHint("local", "find --include-trashed", "set ZOT_MODE=web or ZOT_MODE=hybrid to use this feature")
	}
	if opts.QMode != "" {
		return nil, newUnsupportedFeatureErrorWithHint("local", "find --qmode", "set ZOT_MODE=web or ZOT_MODE=hybrid to use this feature")
	}

	items := []domain.Item{}
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		query, args := localFindQuery(opts)
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		items = items[:0]
		itemIDs := make([]int64, 0, 32)
		for rows.Next() {
			var (
				item             domain.Item
				itemID           int64
				publicationTitle string
				proceedingsTitle string
				bookTitle        string
			)
			if err := rows.Scan(
				&itemID,
				&item.Key,
				&item.Version,
				&item.ItemType,
				&item.Title,
				&item.Date,
				&item.Volume,
				&item.Issue,
				&item.Pages,
				&item.DOI,
				&item.URL,
				&publicationTitle,
				&proceedingsTitle,
				&bookTitle,
			); err != nil {
				return err
			}
			item.Container = firstNonEmptyString(publicationTitle, proceedingsTitle, bookTitle)
			item.Date = normalizeLocalDate(item.Date)
			items = append(items, item)
			itemIDs = append(itemIDs, itemID)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		creatorsByItemID, err := r.loadCreatorsByItemIDs(ctx, db, itemIDs)
		if err != nil {
			return err
		}
		tagsByItemID, err := r.loadTagsByItemIDs(ctx, db, itemIDs)
		if err != nil {
			return err
		}
		for index, itemID := range itemIDs {
			items[index].Creators = creatorsByItemID[itemID]
			items[index].Tags = tagsByItemID[itemID]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	items = localFilterAndOrderItems(items, opts)
	items = paginateItems(items, opts.Start, opts.Limit)
	return items, nil
}

func (r *LocalReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	var item domain.Item
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		loadedItem, itemID, err := r.loadItem(ctx, db, key)
		if err != nil {
			return err
		}

		creators, err := r.loadCreators(ctx, db, itemID)
		if err != nil {
			return err
		}
		tags, err := r.loadTags(ctx, db, itemID)
		if err != nil {
			return err
		}
		collections, err := r.loadCollections(ctx, db, itemID)
		if err != nil {
			return err
		}
		attachments, err := r.loadAttachments(ctx, db, itemID)
		if err != nil {
			return err
		}
		notes, err := r.loadNotes(ctx, db, itemID)
		if err != nil {
			return err
		}

		loadedItem.Creators = creators
		loadedItem.Tags = tags
		loadedItem.Collections = collections
		loadedItem.Attachments = attachments
		loadedItem.Notes = notes
		item = loadedItem
		return nil
	})
	if err != nil {
		return domain.Item{}, err
	}
	return item, nil
}

func (r *LocalReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	var relations []domain.Relation
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		_, itemID, err := r.loadItemRefByKey(ctx, db, key)
		if err != nil {
			return err
		}

		outgoing, err := r.loadOutgoingRelations(ctx, db, itemID)
		if err != nil {
			return err
		}
		incoming, err := r.loadIncomingRelations(ctx, db, key)
		if err != nil {
			return err
		}
		relations = append(outgoing, incoming...)
		sort.SliceStable(relations, func(i int, j int) bool {
			if relations[i].Predicate != relations[j].Predicate {
				return relations[i].Predicate < relations[j].Predicate
			}
			if relations[i].Direction != relations[j].Direction {
				return relations[i].Direction < relations[j].Direction
			}
			return relations[i].Target.Key < relations[j].Target.Key
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *LocalReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	var stats LibraryStats
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		totalItems, err := countRows(ctx, db, `SELECT COUNT(*) FROM items`)
		if err != nil {
			return err
		}
		totalCollections, err := countRows(ctx, db, `SELECT COUNT(*) FROM collections`)
		if err != nil {
			return err
		}
		totalSearches, err := countRows(ctx, db, `SELECT COUNT(*) FROM savedSearches`)
		if err != nil {
			return err
		}

		stats = LibraryStats{
			LibraryType:      r.LibraryType,
			LibraryID:        r.LibraryID,
			TotalItems:       totalItems,
			TotalCollections: totalCollections,
			TotalSearches:    totalSearches,
		}
		return nil
	})
	if err != nil {
		return LibraryStats{}, err
	}
	return stats, nil
}

func localFindQuery(opts FindOptions) (string, []any) {
	query := `
		SELECT
			i.itemID,
			i.key,
			COALESCE(i.version, 0),
			it.typeName,
			COALESCE(MAX(CASE WHEN f.fieldName = 'title' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'date' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'volume' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'issue' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'pages' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'DOI' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'url' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'publicationTitle' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'proceedingsTitle' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'bookTitle' THEN v.value END), '')
		FROM items i
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		LEFT JOIN itemData d ON d.itemID = i.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE ` + localVisibleItemClause(opts.ItemType) + `
		AND ` + localQueryMatchClause() + `
		` + localTagFilterClause(opts) + `
		GROUP BY i.itemID, i.key, i.version, it.typeName
		ORDER BY i.key
	`
	args := localFindArgs(opts)
	return query, args
}

func localFindArgs(opts FindOptions) []any {
	args := []any{}
	if opts.ItemType != "" {
		args = append(args, opts.ItemType)
	}
	query := strings.TrimSpace(strings.ToLower(opts.Query))
	queryLike := "%" + query + "%"
	args = append(args,
		query,
		queryLike,
		queryLike,
		queryLike,
		queryLike,
		queryLike,
		queryLike,
		queryLike,
	)
	for _, tag := range normalizedTags(opts.Tags) {
		args = append(args, tag)
	}
	return args
}

func localVisibleItemClause(itemType string) string {
	if itemType != "" {
		return `it.typeName = ?`
	}
	return `
		NOT EXISTS (SELECT 1 FROM itemAttachments ia WHERE ia.itemID = i.itemID)
		AND NOT EXISTS (SELECT 1 FROM itemNotes n WHERE n.itemID = i.itemID)
		AND NOT EXISTS (SELECT 1 FROM itemAnnotations a WHERE a.itemID = i.itemID)
		AND it.typeName <> 'annotation'
	`
}

func localQueryMatchClause() string {
	return `(
		? = ''
		OR LOWER(i.key) LIKE ?
		OR EXISTS (
			SELECT 1
			FROM itemData d2
			JOIN itemDataValues v2 ON v2.valueID = d2.valueID
			JOIN fieldsCombined f2 ON f2.fieldID = d2.fieldID
			WHERE d2.itemID = i.itemID
			AND f2.fieldName IN ('title', 'shortTitle', 'publicationTitle', 'bookTitle', 'proceedingsTitle', 'date')
			AND LOWER(v2.value) LIKE ?
		)
		OR EXISTS (
			SELECT 1
			FROM itemCreators ic2
			JOIN creators c2 ON c2.creatorID = ic2.creatorID
			WHERE ic2.itemID = i.itemID
			AND LOWER(TRIM(COALESCE(c2.firstName, '') || ' ' || COALESCE(c2.lastName, ''))) LIKE ?
		)
		OR EXISTS (
			SELECT 1
			FROM itemTags it2
			JOIN tags t2 ON t2.tagID = it2.tagID
			WHERE it2.itemID = i.itemID
			AND LOWER(t2.name) LIKE ?
		)
		OR EXISTS (
			SELECT 1
			FROM itemAttachments ia2
			LEFT JOIN itemData d3 ON d3.itemID = ia2.itemID
			LEFT JOIN itemDataValues v3 ON v3.valueID = d3.valueID
			LEFT JOIN fieldsCombined f3 ON f3.fieldID = d3.fieldID
			WHERE (ia2.itemID = i.itemID OR ia2.parentItemID = i.itemID)
			AND (
				LOWER(COALESCE(ia2.path, '')) LIKE ?
				OR LOWER(COALESCE(ia2.contentType, '')) LIKE ?
				OR (f3.fieldName IN ('title', 'filename') AND LOWER(v3.value) LIKE ?)
			)
		)
	)`
}

func localTagFilterClause(opts FindOptions) string {
	tags := normalizedTags(opts.Tags)
	if len(tags) == 0 {
		return ""
	}
	if opts.TagAny {
		return `
		AND EXISTS (
			SELECT 1
			FROM itemTags it3
			JOIN tags t3 ON t3.tagID = it3.tagID
			WHERE it3.itemID = i.itemID
			AND LOWER(t3.name) IN (` + placeholders(len(tags)) + `)
		)
		`
	}
	clause := ""
	for range tags {
		clause += `
		AND EXISTS (
			SELECT 1
			FROM itemTags it3
			JOIN tags t3 ON t3.tagID = it3.tagID
			WHERE it3.itemID = i.itemID
			AND LOWER(t3.name) = ?
		)
		`
	}
	return clause
}

func normalizedTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalized := strings.TrimSpace(strings.ToLower(tag))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	result := "?"
	for i := 1; i < n; i++ {
		result += ", ?"
	}
	return result
}

func localFilterAndOrderItems(items []domain.Item, opts FindOptions) []domain.Item {
	filtered := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if !MatchesDateRange(item.Date, opts.DateAfter, opts.DateBefore) {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i int, j int) bool {
		cmp := compareFindItems(filtered[i], filtered[j], opts.Sort)
		if opts.Direction == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
	return filtered
}

func compareFindItems(left domain.Item, right domain.Item, sortBy string) int {
	switch sortBy {
	case "title":
		if cmp := strings.Compare(strings.ToLower(left.Title), strings.ToLower(right.Title)); cmp != 0 {
			return cmp
		}
	case "date":
		if cmp := compareFindDates(left.Date, right.Date); cmp != 0 {
			return cmp
		}
	}
	return strings.Compare(left.Key, right.Key)
}

func paginateItems(items []domain.Item, start int, limit int) []domain.Item {
	if start >= len(items) {
		return []domain.Item{}
	}
	if start > 0 {
		items = items[start:]
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

func (r *LocalReader) loadItem(ctx context.Context, db *sql.DB, key string) (domain.Item, int64, error) {
	query := `
		SELECT
			i.itemID,
			i.key,
			COALESCE(i.version, 0),
			it.typeName,
			COALESCE(MAX(CASE WHEN f.fieldName = 'title' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'date' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'volume' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'issue' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'pages' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'DOI' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'url' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'publicationTitle' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'proceedingsTitle' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'bookTitle' THEN v.value END), '')
		FROM items i
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		LEFT JOIN itemData d ON d.itemID = i.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE i.key = ?
		GROUP BY i.itemID, i.key, i.version, it.typeName
	`

	var (
		itemID           int64
		item             domain.Item
		publicationTitle string
		proceedingsTitle string
		bookTitle        string
	)
	err := db.QueryRowContext(ctx, query, key).Scan(
		&itemID,
		&item.Key,
		&item.Version,
		&item.ItemType,
		&item.Title,
		&item.Date,
		&item.Volume,
		&item.Issue,
		&item.Pages,
		&item.DOI,
		&item.URL,
		&publicationTitle,
		&proceedingsTitle,
		&bookTitle,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Item{}, 0, newItemNotFoundError("item", key)
		}
		return domain.Item{}, 0, err
	}
	item.Container = firstNonEmptyString(publicationTitle, proceedingsTitle, bookTitle)
	item.Date = normalizeLocalDate(item.Date)
	return item, itemID, nil
}

func (r *LocalReader) withReadableDB(_ context.Context, fn func(*sql.DB) error) error {
	db, cleanup, err := r.openLiveDB()
	if err == nil {
		runErr := fn(db)
		closeErr := closeDBAndCleanup(db, cleanup)
		if runErr == nil {
			r.lastReadMetadata = ReadMetadata{ReadSource: "live"}
			return closeErr
		}
		if !isSQLiteRetryableReadError(runErr) {
			return runErr
		}
		if closeErr != nil && !isSQLiteRetryableReadError(closeErr) {
			return closeErr
		}
	} else if !isSQLiteRetryableReadError(err) {
		return err
	}

	db, cleanup, err = r.openSnapshotDB()
	if err != nil {
		return newLocalTemporarilyUnavailableError(err)
	}
	runErr := fn(db)
	closeErr := closeDBAndCleanup(db, cleanup)
	if runErr != nil {
		if isSQLiteRetryableReadError(runErr) {
			return newLocalTemporarilyUnavailableError(runErr)
		}
		return runErr
	}
	if closeErr != nil {
		return newLocalTemporarilyUnavailableError(closeErr)
	}
	r.lastReadMetadata = ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true}
	return nil
}

func (r *LocalReader) ConsumeReadMetadata() ReadMetadata {
	meta := r.lastReadMetadata
	r.lastReadMetadata = ReadMetadata{}
	return meta
}

func closeDBAndCleanup(db *sql.DB, cleanup func()) error {
	err := db.Close()
	cleanup()
	return err
}

func (r *LocalReader) openLiveDB() (*sql.DB, func(), error) {
	db, err := openSQLiteDBFunc(localSQLiteDSN(r.SQLitePath))
	if err != nil {
		return nil, nil, err
	}
	return db, func() {}, nil
}

func (r *LocalReader) openDB() (*sql.DB, func(), error) {
	db, cleanup, err := r.openLiveDB()
	if err == nil {
		r.lastReadMetadata = ReadMetadata{ReadSource: "live"}
		return db, cleanup, nil
	}
	if !isSQLiteRetryableReadError(err) {
		return nil, nil, err
	}
	db, cleanup, err = r.openSnapshotDB()
	if err != nil {
		return nil, nil, newLocalTemporarilyUnavailableError(err)
	}
	r.lastReadMetadata = ReadMetadata{ReadSource: "snapshot", SQLiteFallback: true}
	return db, cleanup, nil
}

func (r *LocalReader) openSnapshotDB() (*sql.DB, func(), error) {
	snapshotDir, snapshotPath, err := createSQLiteSnapshotFunc(r.SQLitePath)
	if err != nil {
		return nil, nil, err
	}
	db, err := openSQLiteDBFunc(localSQLiteDSN(snapshotPath))
	if err != nil {
		_ = os.RemoveAll(snapshotDir)
		return nil, nil, err
	}
	return db, func() {
		_ = os.RemoveAll(snapshotDir)
	}, nil
}

func localSQLiteDSN(path string) string {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	busyTimeout := 5000
	if value := strings.TrimSpace(os.Getenv("ZOT_LOCAL_BUSY_TIMEOUT_MS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			busyTimeout = parsed
		}
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: fmt.Sprintf("mode=ro&_pragma=busy_timeout=%d&_pragma=query_only=1", busyTimeout),
	}).String()
}

func openSQLiteDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func isSQLiteRetryableReadError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "SQLITE_BUSY") ||
		strings.Contains(message, "SQLITE_LOCKED") ||
		strings.Contains(strings.ToLower(message), "database is locked") ||
		strings.Contains(strings.ToLower(message), "database is busy")
}

func createSQLiteSnapshot(sqlitePath string) (string, string, error) {
	snapshotDir, err := os.MkdirTemp("", "zot-local-snapshot-*")
	if err != nil {
		return "", "", err
	}

	snapshotPath := filepath.Join(snapshotDir, filepath.Base(sqlitePath))
	for _, sourcePath := range []string{
		sqlitePath,
		sqlitePath + "-journal",
		sqlitePath + "-wal",
		sqlitePath + "-shm",
	} {
		if err := copySQLiteFileIfExists(sourcePath, filepath.Join(snapshotDir, filepath.Base(sourcePath))); err != nil {
			_ = os.RemoveAll(snapshotDir)
			return "", "", err
		}
	}
	return snapshotDir, snapshotPath, nil
}

func copySQLiteFileIfExists(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := target.ReadFrom(source); err != nil {
		return err
	}
	return target.Close()
}

func countRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LocalReader) loadItemRefByKey(ctx context.Context, db *sql.DB, key string) (domain.ItemRef, int64, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			i.itemID,
			i.key,
			it.typeName,
			COALESCE(MAX(CASE WHEN f.fieldName = 'title' THEN v.value END), '')
		FROM items i
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		LEFT JOIN itemData d ON d.itemID = i.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE i.key = ?
		GROUP BY i.itemID, i.key, it.typeName
	`, key)

	var itemID int64
	var ref domain.ItemRef
	if err := row.Scan(&itemID, &ref.Key, &ref.ItemType, &ref.Title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ItemRef{}, 0, newItemNotFoundError("item", key)
		}
		return domain.ItemRef{}, 0, err
	}
	return ref, itemID, nil
}

func (r *LocalReader) loadItemRefByID(ctx context.Context, db *sql.DB, itemID int64) (domain.ItemRef, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			i.key,
			it.typeName,
			COALESCE(MAX(CASE WHEN f.fieldName = 'title' THEN v.value END), '')
		FROM items i
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		LEFT JOIN itemData d ON d.itemID = i.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE i.itemID = ?
		GROUP BY i.key, it.typeName
	`, itemID)

	var ref domain.ItemRef
	if err := row.Scan(&ref.Key, &ref.ItemType, &ref.Title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ItemRef{}, fmt.Errorf("item id not found: %d", itemID)
		}
		return domain.ItemRef{}, err
	}
	return ref, nil
}

func (r *LocalReader) loadOutgoingRelations(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Relation, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rp.predicate, ir.object
		FROM itemRelations ir
		JOIN relationPredicates rp ON rp.predicateID = ir.predicateID
		WHERE ir.itemID = ?
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := []domain.Relation{}
	for rows.Next() {
		var predicate string
		var object string
		if err := rows.Scan(&predicate, &object); err != nil {
			return nil, err
		}
		targetKey := relationObjectKey(object)
		target := domain.ItemRef{Key: targetKey}
		if targetKey != "" {
			if ref, _, err := r.loadItemRefByKey(ctx, db, targetKey); err == nil {
				target = ref
			}
		}
		relations = append(relations, domain.Relation{
			Predicate: predicate,
			Direction: "outgoing",
			Target:    target,
		})
	}
	return relations, rows.Err()
}

func (r *LocalReader) loadIncomingRelations(ctx context.Context, db *sql.DB, key string) ([]domain.Relation, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ir.itemID, rp.predicate
		FROM itemRelations ir
		JOIN relationPredicates rp ON rp.predicateID = ir.predicateID
		WHERE ir.object LIKE ?
	`, "%/items/"+key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := []domain.Relation{}
	for rows.Next() {
		var itemID int64
		var predicate string
		if err := rows.Scan(&itemID, &predicate); err != nil {
			return nil, err
		}
		target, err := r.loadItemRefByID(ctx, db, itemID)
		if err != nil {
			return nil, err
		}
		relations = append(relations, domain.Relation{
			Predicate: predicate,
			Direction: "incoming",
			Target:    target,
		})
	}
	return relations, rows.Err()
}

func relationObjectKey(value string) string {
	idx := strings.LastIndex(value, "/items/")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(value[idx+len("/items/"):])
}

func (r *LocalReader) loadCreators(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Creator, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			TRIM(COALESCE(c.firstName, '') || ' ' || COALESCE(c.lastName, '')),
			COALESCE(ct.creatorType, '')
		FROM itemCreators ic
		JOIN creators c ON c.creatorID = ic.creatorID
		LEFT JOIN creatorTypes ct ON ct.creatorTypeID = ic.creatorTypeID
		WHERE ic.itemID = ?
		ORDER BY ic.orderIndex
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	creators := []domain.Creator{}
	for rows.Next() {
		var name string
		var creatorType string
		if err := rows.Scan(&name, &creatorType); err != nil {
			return nil, err
		}
		name = normalizeWhitespace(name)
		if name == "" {
			continue
		}
		creators = append(creators, domain.Creator{Name: name, CreatorType: creatorType})
	}
	return creators, rows.Err()
}

func (r *LocalReader) loadCreatorsByItemIDs(ctx context.Context, db *sql.DB, itemIDs []int64) (map[int64][]domain.Creator, error) {
	result := make(map[int64][]domain.Creator, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		args = append(args, itemID)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			ic.itemID,
			TRIM(COALESCE(c.firstName, '') || ' ' || COALESCE(c.lastName, '')),
			COALESCE(ct.creatorType, '')
		FROM itemCreators ic
		JOIN creators c ON c.creatorID = ic.creatorID
		LEFT JOIN creatorTypes ct ON ct.creatorTypeID = ic.creatorTypeID
		WHERE ic.itemID IN (`+placeholders(len(itemIDs))+`)
		ORDER BY ic.itemID, ic.orderIndex
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID int64
		var name string
		var creatorType string
		if err := rows.Scan(&itemID, &name, &creatorType); err != nil {
			return nil, err
		}
		name = normalizeWhitespace(name)
		if name == "" {
			continue
		}
		result[itemID] = append(result[itemID], domain.Creator{Name: name, CreatorType: creatorType})
	}
	return result, rows.Err()
}

func (r *LocalReader) loadTags(ctx context.Context, db *sql.DB, itemID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.name
		FROM itemTags it
		JOIN tags t ON t.tagID = it.tagID
		WHERE it.itemID = ?
		ORDER BY t.name
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, rows.Err()
}

func (r *LocalReader) loadTagsByItemIDs(ctx context.Context, db *sql.DB, itemIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		args = append(args, itemID)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT it.itemID, t.name
		FROM itemTags it
		JOIN tags t ON t.tagID = it.tagID
		WHERE it.itemID IN (`+placeholders(len(itemIDs))+`)
		ORDER BY it.itemID, t.name
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID int64
		var tag string
		if err := rows.Scan(&itemID, &tag); err != nil {
			return nil, err
		}
		if tag == "" {
			continue
		}
		result[itemID] = append(result[itemID], tag)
	}
	return result, rows.Err()
}

func (r *LocalReader) loadCollections(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Collection, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.key, c.collectionName
		FROM collectionItems ci
		JOIN collections c ON c.collectionID = ci.collectionID
		WHERE ci.itemID = ?
		ORDER BY c.collectionName
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collections := []domain.Collection{}
	for rows.Next() {
		var collection domain.Collection
		if err := rows.Scan(&collection.Key, &collection.Name); err != nil {
			return nil, err
		}
		collections = append(collections, collection)
	}
	return collections, rows.Err()
}

func (r *LocalReader) loadAttachments(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Attachment, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			i.key,
			it.typeName,
			COALESCE(MAX(CASE WHEN f.fieldName = 'title' THEN v.value END), ''),
			COALESCE(MAX(CASE WHEN f.fieldName = 'filename' THEN v.value END), ''),
			COALESCE(ia.contentType, ''),
			COALESCE(ia.linkMode, 0),
			COALESCE(ia.path, '')
		FROM itemAttachments ia
		JOIN items i ON i.itemID = ia.itemID
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		LEFT JOIN itemData d ON d.itemID = i.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE ia.parentItemID = ?
		GROUP BY i.itemID, i.key, it.typeName, ia.contentType, ia.linkMode, ia.path
		ORDER BY i.key
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := []domain.Attachment{}
	for rows.Next() {
		var attachment domain.Attachment
		var linkMode int
		if err := rows.Scan(
			&attachment.Key,
			&attachment.ItemType,
			&attachment.Title,
			&attachment.Filename,
			&attachment.ContentType,
			&linkMode,
			&attachment.ZoteroPath,
		); err != nil {
			return nil, err
		}
		attachment.LinkMode = formatAttachmentLinkMode(linkMode)
		attachment.ResolvedPath, attachment.Resolved = r.resolveAttachmentPath(attachment.Key, attachment.ZoteroPath, attachment.Filename)
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (r *LocalReader) loadNotes(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Note, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			i.key,
			COALESCE(n.note, '')
		FROM itemNotes n
		JOIN items i ON i.itemID = n.itemID
		WHERE n.parentItemID = ?
		ORDER BY i.key
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []domain.Note{}
	for rows.Next() {
		var note domain.Note
		var content string
		if err := rows.Scan(&note.Key, &content); err != nil {
			return nil, err
		}
		note.Preview = notePreview(content)
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func (r *LocalReader) resolveAttachmentPath(key string, zoteroPath string, filename string) (string, bool) {
	if zoteroPath == "" {
		return "", false
	}
	if after, ok := stringsCutPrefix(zoteroPath, "storage:"); ok {
		name := filename
		if name == "" {
			name = path.Base(after)
		}
		if name == "" || name == "." {
			return "", false
		}
		resolved := filepath.Join(r.StorageDir, key, filepath.FromSlash(name))
		if _, err := os.Stat(resolved); err == nil {
			return resolved, true
		}
	}
	if after, ok := stringsCutPrefix(zoteroPath, "attachments:"); ok && r.AttachmentBaseDir != "" {
		resolved := filepath.Join(r.AttachmentBaseDir, filepath.FromSlash(after))
		if _, err := os.Stat(resolved); err == nil {
			return resolved, true
		}
	}
	if filepath.IsAbs(zoteroPath) {
		if _, err := os.Stat(zoteroPath); err == nil {
			return zoteroPath, true
		}
	}
	return "", false
}

func formatAttachmentLinkMode(mode int) string {
	switch mode {
	case 0:
		return "imported_file"
	case 1:
		return "imported_url"
	case 2:
		return "linked_file"
	case 3:
		return "linked_url"
	default:
		return fmt.Sprintf("mode_%d", mode)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return stringsJoinFields(value)
}

func notePreview(value string) string {
	text := stripHTMLTags(value)
	text = normalizeWhitespace(text)
	if len(text) <= 80 {
		return text
	}
	return text[:77] + "..."
}

func stripHTMLTags(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		case '\n', '\r', '\t':
			if !inTag {
				b.WriteRune(' ')
			}
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

var (
	localDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	localTimePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
)

func normalizeLocalDate(value string) string {
	value = normalizeWhitespace(value)
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) >= 3 && localDatePattern.MatchString(parts[0]) && parts[1] == parts[0] && localTimePattern.MatchString(parts[2]) {
		return parts[0]
	}
	if len(parts) >= 2 && localDatePattern.MatchString(parts[0]) && parts[1] == parts[0] {
		return parts[0]
	}
	return value
}

func stringsJoinFields(value string) string {
	parts := stringsFields(value)
	if len(parts) == 0 {
		return ""
	}
	return joinWithSpace(parts)
}

func stringsFields(value string) []string {
	return strings.Fields(value)
}

func joinWithSpace(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += " " + part
	}
	return result
}

func stringsCutPrefix(value string, prefix string) (string, bool) {
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return "", false
	}
	return value[len(prefix):], true
}

func requireDir(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", label, path)
	}
	return nil
}

func requireFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is not a file: %s", label, path)
	}
	return nil
}
