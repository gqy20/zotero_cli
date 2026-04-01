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
	"strings"

	_ "modernc.org/sqlite"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type LocalReader struct {
	LibraryType string
	LibraryID   string
	DataDir     string
	SQLitePath  string
	StorageDir  string
}

func NewLocalReader(cfg config.Config) (*LocalReader, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("local mode requires data_dir")
	}

	dataDir, err := filepath.Abs(cfg.DataDir)
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

	return &LocalReader{
		LibraryType: cfg.LibraryType,
		LibraryID:   cfg.LibraryID,
		DataDir:     dataDir,
		SQLitePath:  sqlitePath,
		StorageDir:  storageDir,
	}, nil
}

func (r *LocalReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	if opts.IncludeTrashed {
		return nil, newUnsupportedFeatureError("local", "find --include-trashed")
	}
	if opts.QMode != "" {
		return nil, newUnsupportedFeatureError("local", "find --qmode")
	}

	db, cleanup, err := r.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	query, args := localFindQuery(opts)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Item{}
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
			return nil, err
		}
		item.Container = firstNonEmptyString(publicationTitle, proceedingsTitle, bookTitle)
		item.Date = normalizeLocalDate(item.Date)

		item.Creators, err = r.loadCreators(ctx, db, itemID)
		if err != nil {
			return nil, err
		}
		item.Tags, err = r.loadTags(ctx, db, itemID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items = localFilterAndOrderItems(items, opts)
	items = paginateItems(items, opts.Start, opts.Limit)
	return items, nil
}

func (r *LocalReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return domain.Item{}, err
	}
	defer db.Close()
	defer cleanup()

	item, itemID, err := r.loadItem(ctx, db, key)
	if err != nil {
		return domain.Item{}, err
	}

	creators, err := r.loadCreators(ctx, db, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	tags, err := r.loadTags(ctx, db, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	collections, err := r.loadCollections(ctx, db, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	attachments, err := r.loadAttachments(ctx, db, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	notes, err := r.loadNotes(ctx, db, itemID)
	if err != nil {
		return domain.Item{}, err
	}

	item.Creators = creators
	item.Tags = tags
	item.Collections = collections
	item.Attachments = attachments
	item.Notes = notes
	return item, nil
}

func (r *LocalReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	_, itemID, err := r.loadItemRefByKey(ctx, db, key)
	if err != nil {
		return nil, err
	}

	outgoing, err := r.loadOutgoingRelations(ctx, db, itemID)
	if err != nil {
		return nil, err
	}
	incoming, err := r.loadIncomingRelations(ctx, db, key)
	if err != nil {
		return nil, err
	}
	relations := append(outgoing, incoming...)
	sort.SliceStable(relations, func(i int, j int) bool {
		if relations[i].Predicate != relations[j].Predicate {
			return relations[i].Predicate < relations[j].Predicate
		}
		if relations[i].Direction != relations[j].Direction {
			return relations[i].Direction < relations[j].Direction
		}
		return relations[i].Target.Key < relations[j].Target.Key
	})
	return relations, nil
}

func (r *LocalReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return LibraryStats{}, err
	}
	defer db.Close()
	defer cleanup()

	totalItems, err := countRows(ctx, db, `SELECT COUNT(*) FROM items`)
	if err != nil {
		return LibraryStats{}, err
	}
	totalCollections, err := countRows(ctx, db, `SELECT COUNT(*) FROM collections`)
	if err != nil {
		return LibraryStats{}, err
	}
	totalSearches, err := countRows(ctx, db, `SELECT COUNT(*) FROM savedSearches`)
	if err != nil {
		return LibraryStats{}, err
	}

	return LibraryStats{
		LibraryType:      r.LibraryType,
		LibraryID:        r.LibraryID,
		TotalItems:       totalItems,
		TotalCollections: totalCollections,
		TotalSearches:    totalSearches,
	}, nil
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

func (r *LocalReader) openDB() (*sql.DB, func(), error) {
	db, err := openSQLiteDB(localSQLiteDSN(r.SQLitePath))
	if err != nil {
		if !isSQLiteBusy(err) {
			return nil, nil, err
		}
		snapshotDir, snapshotPath, snapshotErr := createSQLiteSnapshot(r.SQLitePath)
		if snapshotErr != nil {
			return nil, nil, err
		}
		db, err = openSQLiteDB(localSQLiteDSN(snapshotPath))
		if err != nil {
			_ = os.RemoveAll(snapshotDir)
			return nil, nil, err
		}
		return db, func() {
			_ = os.RemoveAll(snapshotDir)
		}, nil
	}
	return db, func() {}, nil
}

func localSQLiteDSN(path string) string {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: "mode=ro&_pragma=busy_timeout=5000&_pragma=query_only=1",
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

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SQLITE_BUSY")
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
