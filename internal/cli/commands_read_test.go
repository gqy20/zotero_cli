package cli

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"

	_ "modernc.org/sqlite"
)

type stubMetadataReader struct {
	items     []domain.Item
	item      domain.Item
	relations []domain.Relation
	stats     backend.LibraryStats
	meta      backend.ReadMetadata
}

func (r stubMetadataReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	return append([]domain.Item(nil), r.items...), nil
}

func (r stubMetadataReader) GetItem(context.Context, string) (domain.Item, error) {
	return r.item, nil
}

func (r stubMetadataReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return append([]domain.Relation(nil), r.relations...), nil
}

func (r stubMetadataReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return r.stats, nil
}

func (r stubMetadataReader) ListNotes(context.Context) ([]domain.Note, error) {
	return nil, nil
}

func (r stubMetadataReader) ListTags(context.Context) ([]backend.Tag, error) {
	return nil, nil
}

func (r stubMetadataReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

type stubLocalExportReader struct {
	keys      []string
	payload   []map[string]any
	meta      backend.ReadMetadata
	findErr   error
	keysErr   error
	exportErr error
}

type stubLocalTextReader struct {
	item        domain.Item
	text        string
	attachments []backend.AttachmentFullText
	meta        backend.ReadMetadata
}

func (r stubLocalExportReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	items := make([]domain.Item, 0, len(r.keys))
	for _, key := range r.keys {
		items = append(items, domain.Item{Key: key})
	}
	return items, nil
}

func (r stubLocalExportReader) GetItem(context.Context, string) (domain.Item, error) {
	return domain.Item{}, nil
}

func (r stubLocalExportReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return nil, nil
}

func (r stubLocalExportReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}

func (r stubLocalExportReader) CollectionItemKeys(context.Context, string, int) ([]string, error) {
	if r.keysErr != nil {
		return nil, r.keysErr
	}
	return append([]string(nil), r.keys...), nil
}

func (r stubLocalExportReader) ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error) {
	if r.exportErr != nil {
		return nil, r.exportErr
	}
	return append([]map[string]any(nil), r.payload...), nil
}

func (r stubLocalExportReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

func (r stubLocalExportReader) ListNotes(context.Context) ([]domain.Note, error) {
	return nil, nil
}

func (r stubLocalExportReader) ListTags(context.Context) ([]backend.Tag, error) {
	return nil, nil
}

func (r stubLocalTextReader) GetItem(context.Context, string) (domain.Item, error) {
	return r.item, nil
}

func (r stubLocalTextReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	return nil, nil
}

func (r stubLocalTextReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return nil, nil
}

func (r stubLocalTextReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}

func (r stubLocalTextReader) ExtractItemFullText(context.Context, domain.Item) (string, error) {
	return r.text, nil
}

func (r stubLocalTextReader) ExtractItemAttachmentTexts(context.Context, domain.Item) (backend.ItemFullTextResult, error) {
	return backend.ItemFullTextResult{
		Text:                 r.text,
		PrimaryAttachmentKey: r.meta.FullTextAttachmentKey,
		Attachments:          append([]backend.AttachmentFullText(nil), r.attachments...),
	}, nil
}

func (r stubLocalTextReader) ConsumeReadMetadata() backend.ReadMetadata {
	return r.meta
}

func (r stubLocalTextReader) ListNotes(context.Context) ([]domain.Note, error) {
	return nil, nil
}

func (r stubLocalTextReader) ListTags(context.Context) ([]backend.Tag, error) {
	return nil, nil
}

func TestHoldSQLiteExclusiveLockHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SQLITE_LOCK_HELPER") != "1" {
		return
	}
	dbPath := os.Getenv("LOCK_DB_PATH")
	readyPath := os.Getenv("LOCK_READY_PATH")
	releasePath := os.Getenv("LOCK_RELEASE_PATH")
	if dbPath == "" || readyPath == "" || releasePath == "" {
		os.Exit(2)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA locking_mode=EXCLUSIVE;`); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`BEGIN EXCLUSIVE;`); err != nil {
		panic(err)
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		panic(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(releasePath); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := db.Exec(`ROLLBACK;`); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func startSQLiteLockHelper(t *testing.T, sqlitePath string) func() {
	t.Helper()

	readyPath := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	cmd := exec.Command(os.Args[0], "-test.run=TestHoldSQLiteExclusiveLockHelper")
	cmd.Env = append(os.Environ(),
		"GO_WANT_SQLITE_LOCK_HELPER=1",
		"LOCK_DB_PATH="+sqlitePath,
		"LOCK_READY_PATH="+readyPath,
		"LOCK_RELEASE_PATH="+releasePath,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start lock helper: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("lock helper did not become ready")
		}
		time.Sleep(50 * time.Millisecond)
	}

	return func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o600)
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("lock helper wait: %v", err)
			}
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("lock helper did not exit")
		}
	}
}

func buildLocalShowFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ATTACHPDF', 1, 2), (3, 'ATTACHURL', 1, 2), (4, 'NOTE1234', 1, 2);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'publicationTitle'), (4, 'DOI'), (5, 'url'), (6, 'filename'), (7, 'note'), (8, 'volume'), (9, 'issue'), (10, 'pages');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Attention Is All You Need'), (2, '2024-01-08 2024-01-08 00:00:00'), (3, 'NeurIPS'), (4, '10.1/example'), (5, 'https://example.com/paper'), (6, 'attention.pdf'), (7, 'Web Snapshot'), (8, '<p>Local note summary</p>'), (9, '37'), (10, '11'), (11, '1234-1248');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (1, 3, 3), (1, 4, 4), (1, 5, 5), (1, 8, 9), (1, 9, 10), (1, 10, 11), (2, 1, 1), (2, 6, 6), (3, 1, 7), (4, 7, 8);`,
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Ashish', 'Vaswani', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0);`,
		`INSERT INTO tags(tagID, name) VALUES (1, 'transformers');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 0, 'storage:attention.pdf'), (3, 1, 'text/html', 3, 'attachments:snapshots/page.html');`,
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (4, 1, '<p>Local note summary</p>', 'Local note');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	attachmentDir := filepath.Join(storageDir, "ATTACHPDF")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "attention.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, ".zotero-ft-cache"), []byte("Attention is all you need full text preview from zotero cache."), 0o600); err != nil {
		t.Fatal(err)
	}
}

func buildLocalRelateFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE relationPredicates (predicateID INTEGER PRIMARY KEY, predicate TEXT);`,
		`CREATE TABLE itemRelations (itemID INTEGER, predicateID INTEGER, object TEXT);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'RELA1111', 1, 1), (3, 'RELB2222', 1, 1);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Primary Item'), (2, 'Related Incoming'), (3, 'Related Outgoing');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (2, 1, 2), (3, 1, 3);`,
		`INSERT INTO relationPredicates(predicateID, predicate) VALUES (1, 'dc:relation');`,
		`INSERT INTO itemRelations(itemID, predicateID, object) VALUES (1, 1, 'http://zotero.org/users/123456/items/RELB2222'), (2, 1, 'http://zotero.org/users/123456/items/ITEM1234');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func buildLocalLinkedAttachmentFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEMLINK', 3, 1), (2, 'ATTLINK', 1, 2);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'filename');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Linked PDF Item'), (2, '2024-06-01'), (3, 'linked.pdf');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (2, 1, 1), (2, 3, 3);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 2, 'attachments:papers/linked.pdf');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func writeZoteroPrefsFixture(t *testing.T, root string, dataDir string, baseAttachmentDir string) {
	t.Helper()

	prefsPath := filepath.Join(root, "Zotero", "Zotero", "Profiles", "abcd.default", "prefs.js")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`user_pref("extensions.zotero.baseAttachmentPath", "` + strings.ReplaceAll(baseAttachmentDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.dataDir", "` + strings.ReplaceAll(dataDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.saveRelativeAttachmentPath", true);`,
		"",
	}, "\n")
	if err := os.WriteFile(prefsPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func buildLocalStatsFixture(t *testing.T, sqlitePath string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE savedSearches (savedSearchID INTEGER PRIMARY KEY, savedSearchName TEXT);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'book');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ART67890', 3, 1), (3, 'BOOK1234', 2, 2);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning'), (2, 'COLL5678', 'Chestnut');`,
		`INSERT INTO savedSearches(savedSearchID, savedSearchName) VALUES (1, 'Unread PDFs');`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

func buildLocalFindFixture(t *testing.T, dataDir string, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
		itemID INTEGER PRIMARY KEY,
		parentItemID INT NOT NULL,
		type INT NOT NULL,
		authorName TEXT,
		text TEXT,
		comment TEXT,
		color TEXT,
		pageLabel TEXT,
		sortIndex TEXT NOT NULL,
		position TEXT NOT NULL,
		isExternal INT NOT NULL
	);`,
		`CREATE TABLE fulltextItems (itemID INTEGER PRIMARY KEY, indexedChars INTEGER, totalChars INTEGER, indexedPages INTEGER, totalPages INTEGER, version INTEGER, synced INTEGER);`,
		`CREATE TABLE fulltextWords (wordID INTEGER PRIMARY KEY, word TEXT);`,
		`CREATE TABLE fulltextItemWords (wordID INTEGER, itemID INTEGER);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'book'), (3, 'attachment'), (4, 'note'), (5, 'annotation');`,
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ART67890', 3, 1), (3, 'BOOK1234', 2, 2), (4, 'ATTA1111', 1, 3), (5, 'NOTE1111', 1, 4), (6, 'ANNO1111', 1, 5), (7, 'ARTFULL2', 4, 1), (8, 'ATTB2222', 1, 3);`,
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title'), (2, 'date'), (3, 'publicationTitle'), (4, 'DOI'), (5, 'url'), (6, 'filename'), (7, 'note'), (8, 'volume'), (9, 'issue'), (10, 'pages');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Attention Is All You Need'), (2, '2024-01-08 2024-01-08 00:00:00'), (3, 'NeurIPS'), (4, '10.1/example'), (5, 'https://example.com/paper'), (6, 'Mixed Survey'), (7, '2024-05-03'), (8, 'Mixed Book'), (9, '2023'), (10, 'mixed.pdf'), (11, '<p>Mixed note</p>'), (12, 'Mixed Attachment'), (13, '37'), (14, '11'), (15, '1234-1248'), (16, '29'), (17, '20'), (18, 'R1094-R1103'), (19, 'Prefix Match Article'), (20, '2024-06-07'), (21, 'prefix.pdf'), (22, 'Prefix Attachment');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1), (1, 2, 2), (1, 3, 3), (1, 4, 4), (1, 5, 5), (1, 8, 13), (1, 9, 14), (1, 10, 15), (2, 1, 6), (2, 2, 7), (2, 8, 16), (2, 9, 17), (2, 10, 18), (3, 1, 8), (3, 2, 9), (4, 1, 12), (4, 6, 10), (5, 7, 11), (7, 1, 19), (7, 2, 20), (8, 1, 22), (8, 6, 21);`,
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Ashish', 'Vaswani', 0), (2, 'Jane', 'Roe', 0), (3, 'John', 'Doe', 0), (4, 'Alex', 'Smith', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0), (2, 2, 1, 0), (3, 3, 1, 0), (7, 4, 1, 0);`,
		`INSERT INTO tags(tagID, name) VALUES (1, 'transformers'), (2, 'ai'), (3, 'survey'), (4, 'classic'), (5, 'genomics');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1), (2, 2), (2, 3), (3, 4), (7, 5);`,
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1234', 'Machine Learning'), (2, 'COLL5678', 'Books');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1), (1, 2), (2, 3);`,
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (4, 2, 'application/pdf', 0, 'storage:mixed.pdf'), (8, 7, 'text/plain', 0, 'storage:prefix.pdf');`,
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (5, 2, '<p>Mixed note</p>', 'Mixed note');`,
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (6, 4, 0, '', '', '', '#ffd400', '1', '00001|00000|00000', '{"pageIndex":0}', 0);`,
		`INSERT INTO fulltextItems(itemID, indexedPages, totalPages, version, synced) VALUES (4, 5, 5, 1, 1), (8, 4, 4, 1, 1);`,
		`INSERT INTO fulltextWords(wordID, word) VALUES (1, 'speciation'), (2, 'genome'), (3, 'genomic'), (4, 'species');`,
		`INSERT INTO fulltextItemWords(wordID, itemID) VALUES (1, 4), (2, 4), (3, 8), (4, 8);`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	attachmentDir := filepath.Join(storageDir, "ATTA1111")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "mixed.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, ".zotero-ft-cache"), []byte("Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow."), 0o600); err != nil {
		t.Fatal(err)
	}
	attachmentDir = filepath.Join(storageDir, "ATTB2222")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "prefix.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
}

type ftsCacheRow struct {
	AttachmentKey   string
	ParentItemKey   string
	Title           string
	AttachmentTitle string
	Body            string
}

func buildGlobalFTSCacheForTest(t *testing.T, dataDir string, rows []ftsCacheRow) {
	t.Helper()
	cacheDir := filepath.Join(dataDir, ".zotero_cli", "fulltext")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(cacheDir, "index.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS fulltext_meta(
			attachment_key TEXT PRIMARY KEY,
			parent_item_key TEXT,
			title TEXT,
			attachment_title TEXT,
			attachment_name TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fulltext_documents USING fts5(
			attachment_key UNINDEXED,
			parent_item_key UNINDEXED,
			content_type UNINDEXED,
			resolved_path UNINDEXED,
			title,
			creators,
			tags,
			attachment_title,
			attachment_name,
			attachment_path,
			body
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fulltext_chunks USING fts5(
			attachment_key UNINDEXED,
			parent_item_key UNINDEXED,
			chunk_index UNINDEXED,
			page UNINDEXED,
			bbox UNINDEXED,
			body
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	for _, row := range rows {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO fulltext_meta(attachment_key, parent_item_key, title, attachment_title) VALUES (?, ?, ?, ?)`,
			row.AttachmentKey, row.ParentItemKey, row.Title, row.AttachmentTitle,
		); err != nil {
			t.Fatalf("insert fts meta: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO fulltext_documents(attachment_key, parent_item_key, title, body) VALUES (?, ?, ?, ?)`,
			row.AttachmentKey, row.ParentItemKey, row.Title, row.Body,
		); err != nil {
			t.Fatalf("insert fts cache: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO fulltext_chunks(attachment_key, parent_item_key, chunk_index, page, bbox, body) VALUES (?, ?, 1, 1, '[0,0,0,0]', ?)`,
			row.AttachmentKey, row.ParentItemKey, row.Body,
		); err != nil {
			t.Fatalf("insert fts chunks: %v", err)
		}
	}
}
