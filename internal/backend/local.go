package backend

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type LocalReader struct {
	LibraryType       string
	LibraryID         string
	DataDir           string
	SQLitePath        string
	StorageDir        string
	FullTextCacheDir  string
	AttachmentBaseDir string
	lastReadMetadata  ReadMetadata
	openSQLiteDB      func(string) (*sql.DB, error)
	createSnapshot    func(string) (string, string, error)
	findZoteroPrefs   func() ([]string, error)
}

func NewLocalReader(cfg config.Config) (*LocalReader, error) {
	prefs, _, err := loadMatchingZoteroPrefs(cfg.DataDir, findZoteroPrefs)
	if err != nil {
		return nil, err
	}

	dataDirInput := cfg.DataDir
	if dataDirInput == "" {
		dataDirInput = prefs.DataDir
	}
	if dataDirInput == "" {
		dataDirInput = findDefaultDataDir()
	}
	if dataDirInput == "" {
		return nil, fmt.Errorf("local mode requires data_dir (set ZOT_DATA_DIR or configure in Zotero)")
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
		FullTextCacheDir:  filepath.Join(dataDir, ".zotero_cli", "fulltext"),
		AttachmentBaseDir: attachmentBaseDir,
		openSQLiteDB:      openSQLiteDB,
		createSnapshot:    createSQLiteSnapshot,
		findZoteroPrefs:   findZoteroPrefs,
	}, nil
}

func (r *LocalReader) IsFullTextCached(attachment domain.Attachment) bool {
	cache := newFullTextCache(r.FullTextCacheDir)
	_, ok, err := cache.Load(attachment)
	return err == nil && ok
}

func (r *LocalReader) IsMarkedFailed(key string) bool {
	return newFullTextCache(r.FullTextCacheDir).IsMarkedFailed(key)
}

func (r *LocalReader) MarkExtractFailed(key string) error {
	return newFullTextCache(r.FullTextCacheDir).MarkFailed(key)
}

func (r *LocalReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	opts = NormalizeFindOptions(opts)
	if opts.IncludeTrashed {
		return nil, newUnsupportedFeatureErrorWithHint("local", "find --include-trashed", "set ZOT_MODE=web or ZOT_MODE=hybrid to use this feature")
	}
	if opts.QMode != "" {
		return nil, newUnsupportedFeatureErrorWithHint("local", "find --qmode", "set ZOT_MODE=web or ZOT_MODE=hybrid to use this feature")
	}
	if opts.FullText {
		return r.findItemsFromFullTextIndex(ctx, opts)
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
			if opts.FullText {
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
					&item.SearchScore,
				); err != nil {
					return err
				}
			} else if err := rows.Scan(
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

		var (
			creatorsByItemID      map[int64][]domain.Creator
			tagsByItemID          map[int64][]string
			attachmentsByParentID map[int64][]domain.Attachment
			loadErr               error
			wg                    sync.WaitGroup
		)
		wg.Add(3)
		go func() {
			defer wg.Done()
			creatorsByItemID, loadErr = r.loadCreatorsByItemIDs(ctx, db, itemIDs)
		}()
		go func() {
			defer wg.Done()
			tagsByItemID, loadErr = r.loadTagsByItemIDs(ctx, db, itemIDs)
		}()
		go func() {
			defer wg.Done()
			attachmentsByParentID, loadErr = r.loadAttachmentsByParentItemIDs(ctx, db, itemIDs)
		}()
		wg.Wait()
		if loadErr != nil {
			return loadErr
		}
		for index, itemID := range itemIDs {
			items[index].Creators = creatorsByItemID[itemID]
			items[index].Tags = tagsByItemID[itemID]
			items[index].Attachments = attachmentsByParentID[itemID]
			items[index].MatchedOn = localMatchedOn(items[index], opts)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if opts.FullText {
		r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{FullTextEngine: "zotero_fulltext"})
	}
	items = localFilterAndOrderItems(items, opts)
	items = paginateItems(items, opts.Start, opts.Limit)
	if !opts.Full && !findFieldIncluded(opts.IncludeFields, "attachments") {
		for i := range items {
			items[i].Attachments = nil
		}
	}
	return items, nil
}

func (r *LocalReader) findItemsFromFullTextIndex(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	matches, err := newFullTextCache(r.FullTextCacheDir).Search(opts.Query, opts.FullTextAny, opts.Limit)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return []domain.Item{}, nil
	}

	var items []domain.Item
	err = r.withReadableDB(ctx, func(db *sql.DB) error {
		parentKeys := make([]string, 0, len(matches))
		for _, m := range matches {
			parentKeys = append(parentKeys, m.ParentItemKey)
		}

		itemMap, idMap, err := r.batchLoadItemsByKeys(ctx, db, parentKeys)
		if err != nil {
			return err
		}

		itemIDs := make([]int64, 0, len(idMap))
		for _, id := range idMap {
			itemIDs = append(itemIDs, id)
		}

		creatorsMap, _ := r.loadCreatorsByItemIDs(ctx, db, itemIDs)
		tagsMap, _ := r.loadTagsByItemIDs(ctx, db, itemIDs)
		attachMap, _ := r.loadAttachmentsByParentItemIDs(ctx, db, itemIDs)

		items = make([]domain.Item, 0, len(matches))
		for idx, match := range matches {
			item, ok := itemMap[match.ParentItemKey]
			if !ok {
				continue
			}
			itemID := idMap[match.ParentItemKey]
			item.Creators = creatorsMap[itemID]
			item.Tags = tagsMap[itemID]
			item.Attachments = attachMap[itemID]
			item.MatchedOn = []string{"fulltext_attachment"}
			item.SearchScore = len(matches) - idx
			item.SnippetAttachmentKey = match.AttachmentKey
			if match.ChunkIndex >= 0 && match.Body != "" {
				item.MatchedChunk = &domain.MatchedChunkInfo{
					Text:          match.Body,
					Page:          match.ChunkPage,
					BBox:          match.ChunkBBox,
					AttachmentKey: match.AttachmentKey,
				}
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{FullTextEngine: "index_sqlite"})

	items = localFilterAndOrderItems(items, opts)
	items = paginateItems(items, opts.Start, opts.Limit)
	if !opts.Full && !findFieldIncluded(opts.IncludeFields, "attachments") {
		for i := range items {
			items[i].Attachments = nil
		}
	}
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

		var allAnnotations []domain.Annotation
		childAttIDs, cerr := r.loadChildAttachmentIDs(ctx, db, itemID)
		if cerr != nil {
			return cerr
		}
		annosMap, aerr := r.loadAnnotationsByItemIDs(ctx, db, childAttIDs)
		if aerr != nil {
			return aerr
		}
		for _, attID := range childAttIDs {
			allAnnotations = append(allAnnotations, annosMap[attID]...)
		}

		loadedItem.Annotations = allAnnotations
		item = loadedItem
		return nil
	})
	if err != nil {
		return domain.Item{}, err
	}
	return item, nil
}

func (r *LocalReader) CiteItem(ctx context.Context, key string, opts domain.CitationOptions) (domain.CitationResult, error) {
	var result domain.CitationResult
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		item, itemID, err := r.loadItem(ctx, db, key)
		if err != nil {
			return err
		}
		creators, err := r.loadCreators(ctx, db, itemID)
		if err != nil {
			return err
		}

		result.Key = key
		result.Format = opts.Format
		result.Style = opts.Style
		result.Text = formatCitation(item, creators, opts.Format)
		return nil
	})
	if err != nil {
		return domain.CitationResult{}, err
	}
	return result, nil
}

func formatCitation(item domain.Item, creators []domain.Creator, format string) string {
	year := extractYear(item.Date)
	authorStr := formatAuthors(creators)

	switch format {
	case "bib":
		parts := make([]string, 0, 6)
		if authorStr != "" {
			parts = append(parts, authorStr+".")
		}
		if item.Title != "" {
			parts = append(parts, item.Title+".")
		}
		if item.Container != "" {
			parts = append(parts, item.Container+".")
		}
		volIssue := ""
		if item.Volume != "" {
			volIssue = item.Volume
		}
		if item.Issue != "" {
			if volIssue != "" {
				volIssue += "(" + item.Issue + ")"
			} else {
				volIssue = item.Issue
			}
		}
		if volIssue != "" {
			parts = append(parts, volIssue)
		}
		if item.Pages != "" {
			parts = append(parts, ":"+item.Pages)
		}
		if year != "" {
			parts = append(parts, year)
		}
		return joinNonEmpty(" ", parts...)
	default: // "citation"
		if authorStr != "" && year != "" {
			if strings.HasSuffix(authorStr, " et al") {
				return authorStr + "., " + year
			}
			return authorStr + ", " + year
		} else if authorStr != "" {
			if strings.HasSuffix(authorStr, " et al") {
				return authorStr + "."
			}
			return authorStr
		} else if item.Title != "" {
			return truncateTitle(item.Title) + ", " + year
		}
		return keyOrTitle(item.Key, item.Title) + ", " + year
	}
}

func formatAuthors(creators []domain.Creator) string {
	n := len(creators)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return lastNameOnly(creators[0].Name)
	}
	if n == 2 {
		return lastNameOnly(creators[0].Name) + " & " + lastNameOnly(creators[1].Name)
	}
	names := make([]string, 0, n)
	for _, c := range creators {
		names = append(names, lastNameOnly(c.Name))
	}
	return names[0] + " et al"
}

func yearFromCreators(creators []domain.Creator) string {
	for _, c := range creators {
		if y := extractYearFromName(c.Name); y != "" {
			return y
		}
	}
	return ""
}

func extractYear(date string) string {
	if date == "" {
		return ""
	}
	parts := strings.Fields(date)
	if len(parts) > 0 {
		y := parts[0]
		if len(y) >= 4 && y[0] >= '0' && y[0] <= '9' {
			return y[:4]
		}
	}
	return ""
}

func extractYearFromName(name string) string {
	re := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := re.FindStringSubmatch(name)
	if matches != nil && len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func lastNameOnly(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return fullName
	}
	return parts[len(parts)-1]
}

func truncateTitle(title string) string {
	if len(title) <= 60 {
		return title
	}
	return title[:57] + "..."
}

func keyOrTitle(key, title string) string {
	if title != "" {
		return truncateTitle(title)
	}
	return "[" + key + "]"
}

func (r *LocalReader) GetAttachmentFile(ctx context.Context, key string) (string, string, error) {
	var contentType, zoteroPath, filename string
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
			SELECT COALESCE(ia.contentType, ''), COALESCE(ia.path, ''),
			       COALESCE(MAX(CASE WHEN f.fieldName = 'filename' THEN v.value END), '')
			FROM itemAttachments ia
			JOIN items i ON i.itemID = ia.itemID
			LEFT JOIN itemData d ON d.itemID = i.itemID
			LEFT JOIN itemDataValues v ON v.valueID = d.valueID
			LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
			WHERE i.key = ?
			GROUP BY ia.contentType, ia.path
		`, key)
		err := row.Scan(&contentType, &zoteroPath, &filename)
		if err == sql.ErrNoRows {
			return fmt.Errorf("attachment not found: %s", key)
		}
		return err
	})
	if err != nil {
		return "", "", err
	}
	resolved, ok := r.resolveAttachmentPath(key, zoteroPath, filename)
	if !ok {
		return "", "", fmt.Errorf("attachment file not resolved: %s", key)
	}
	return resolved, contentType, nil
}

func (r *LocalReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	var relations []domain.Relation
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		_, itemID, err := r.loadItemRefByKey(ctx, db, key)
		if err != nil {
			return err
		}

		var outgoing, incoming []domain.Relation
		var relErr error
		var relWg sync.WaitGroup
		relWg.Add(2)
		go func() {
			defer relWg.Done()
			outgoing, relErr = r.loadOutgoingRelations(ctx, db, itemID)
		}()
		go func() {
			defer relWg.Done()
			incoming, relErr = r.loadIncomingRelations(ctx, db, key)
		}()
		relWg.Wait()
		if relErr != nil {
			return relErr
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

func (r *LocalReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	var notes []domain.Note
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `
			SELECT i.key, COALESCE(pi.key, ''), COALESCE(n.note, '')
			FROM itemNotes n
			JOIN items i ON i.itemID = n.itemID
			LEFT JOIN items pi ON pi.itemID = n.parentItemID
			ORDER BY i.key
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var note domain.Note
			var content string
			if err := rows.Scan(&note.Key, &note.ParentItemKey, &content); err != nil {
				return err
			}
			note.Content = content
			note.Preview = notePreview(content)
			notes = append(notes, note)
		}
		return rows.Err()
	})
	return notes, err
}

func (r *LocalReader) ListTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `
			SELECT t.name, COUNT(it.itemID) as cnt
			FROM tags t
			JOIN itemTags it ON it.tagID = t.tagID
			GROUP BY t.name
			ORDER BY cnt DESC, t.name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t Tag
			if err := rows.Scan(&t.Name, &t.NumItems); err != nil {
				return err
			}
			tags = append(tags, t)
		}
		return rows.Err()
	})
	return tags, err
}

func (r *LocalReader) ListCollections(ctx context.Context) ([]Collection, error) {
	var collections []Collection
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `
			SELECT c.key, c.collectionName, COUNT(ci.itemID) as cnt
			FROM collections c
			LEFT JOIN collectionItems ci ON ci.collectionID = c.collectionID
			GROUP BY c.collectionID
			ORDER BY cnt DESC, c.collectionName
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var col Collection
			if err := rows.Scan(&col.Key, &col.Name, &col.NumItems); err != nil {
				return err
			}
			collections = append(collections, col)
		}
		return rows.Err()
	})
	return collections, err
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

func (r *LocalReader) openLiveDB() (*sql.DB, func(), error) {
	db, err := r.sqliteOpenFunc()(localSQLiteDSN(r.SQLitePath))
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
	snapshotDir, snapshotPath, err := r.snapshotFunc()(r.SQLitePath)
	if err != nil {
		return nil, nil, err
	}
	db, err := r.sqliteOpenFunc()(localSQLiteDSN(snapshotPath))
	if err != nil {
		_ = os.RemoveAll(snapshotDir)
		return nil, nil, err
	}
	return db, func() {
		_ = os.RemoveAll(snapshotDir)
	}, nil
}

func (r *LocalReader) sqliteOpenFunc() func(string) (*sql.DB, error) {
	if r != nil && r.openSQLiteDB != nil {
		return r.openSQLiteDB
	}
	return openSQLiteDB
}

func (r *LocalReader) snapshotFunc() func(string) (string, string, error) {
	if r != nil && r.createSnapshot != nil {
		return r.createSnapshot
	}
	return createSQLiteSnapshot
}

func findDefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			filepath.Join(home, "Zotero"),
		}
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Zotero"),
		}
	default:
		candidates = []string{
			filepath.Join(home, "Zotero"),
		}
	}
	for _, c := range candidates {
		sqlitePath := filepath.Join(c, "zotero.sqlite")
		if _, err := os.Stat(sqlitePath); err == nil {
			return c
		}
	}
	return ""
}

type DeleteDBAnnotationsResult struct {
	Deleted int `json:"deleted"`
}

func (r *LocalReader) DeleteDBAnnotations(ctx context.Context, itemKey string, req DeleteAnnotationsRequest) (DeleteDBAnnotationsResult, error) {
	db, err := r.sqliteOpenFunc()(localSQLiteDSNReadWrite(r.SQLitePath))
	if err != nil {
		return DeleteDBAnnotationsResult{}, err
	}
	defer db.Close()

	args := []any{itemKey}
	query := "DELETE FROM itemAnnotations WHERE parentItemID IN (SELECT ia.itemID FROM itemAttachments ia WHERE ia.parentItemID = (SELECT itemID FROM items WHERE key = ?))"

	if req.Page > 0 {
		pageFilter := fmt.Sprintf("%%\"pageIndex\":%d%%", req.Page)
		query += " AND position LIKE ?"
		args = append(args, pageFilter)
	}

	if req.Type != "" {
		typeMap := map[string]int{"highlight": 0, "note": 7, "image": 8}
		if typeVal, ok := typeMap[req.Type]; ok {
			query += " AND type = ?"
			args = append(args, typeVal)
		}
	}

	if req.Author != "" {
		query += " AND authorName = ?"
		args = append(args, req.Author)
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return DeleteDBAnnotationsResult{}, err
	}
	deleted, _ := result.RowsAffected()
	return DeleteDBAnnotationsResult{Deleted: int(deleted)}, nil
}
