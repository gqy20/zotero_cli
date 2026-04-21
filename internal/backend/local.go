package backend

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
