package backend

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zotero_cli/internal/domain"

	_ "modernc.org/sqlite"
)

type fullTextCache struct {
	rootDir string
}

type fullTextCacheMeta struct {
	AttachmentKey   string `json:"attachment_key"`
	ParentItemKey   string `json:"parent_item_key,omitempty"`
	ResolvedPath    string `json:"resolved_path,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	Title           string `json:"title,omitempty"`
	Creators        string `json:"creators,omitempty"`
	Tags            string `json:"tags,omitempty"`
	AttachmentTitle string `json:"attachment_title,omitempty"`
	AttachmentName  string `json:"attachment_name,omitempty"`
	AttachmentPath  string `json:"attachment_path,omitempty"`
	Extractor       string `json:"extractor,omitempty"`
	SourceMtimeUnix int64  `json:"source_mtime_unix,omitempty"`
	SourceSize      int64  `json:"source_size,omitempty"`
	TextHash        string `json:"text_hash,omitempty"`
	ExtractedAt     string `json:"extracted_at,omitempty"`
	Pages           int    `json:"pages,omitempty"`
	Chars           int    `json:"chars,omitempty"`
}

type FullTextDocument struct {
	Text     string
	Chunks   []chunk
	Meta     fullTextCacheMeta
	CacheHit bool
}

type chunk struct {
	Page       int        `json:"page"`
	BBox       [4]float64 `json:"bbox"`
	Text       string     `json:"text"`
	BlockCount int        `json:"block_count"`
}

type fullTextIndexMatch struct {
	ParentItemKey   string
	AttachmentKey   string
	Rank            float64
	Title           string
	AttachmentTitle string
	AttachmentName  string
	Body            string
	ChunkIndex      int
	ChunkPage       int
	ChunkBBox       [4]float64
}

func newFullTextCache(rootDir string) fullTextCache {
	return fullTextCache{rootDir: rootDir}
}

func (c fullTextCache) attachmentDir(attachmentKey string) string {
	return filepath.Join(c.rootDir, "cache", attachmentKey)
}

func (c fullTextCache) contentPath(attachmentKey string) string {
	return filepath.Join(c.attachmentDir(attachmentKey), "content.txt")
}

func (c fullTextCache) metaPath(attachmentKey string) string {
	return filepath.Join(c.attachmentDir(attachmentKey), "meta.json")
}

func (c fullTextCache) indexPath() string {
	return filepath.Join(c.rootDir, "index.sqlite")
}

func (c fullTextCache) Load(attachment domain.Attachment) (FullTextDocument, bool, error) {
	key := strings.TrimSpace(attachment.Key)
	if key == "" {
		return FullTextDocument{}, false, nil
	}
	content, err := os.ReadFile(c.contentPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{}, false, err
	}
	metaBytes, err := os.ReadFile(c.metaPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{}, false, err
	}
	var meta fullTextCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return FullTextDocument{}, false, err
	}
	if !c.IsFresh(meta, attachment) {
		return FullTextDocument{}, false, nil
	}
	return FullTextDocument{Text: string(content), Meta: meta, CacheHit: true}, true, nil
}

func (c fullTextCache) Save(doc FullTextDocument) error {
	key := strings.TrimSpace(doc.Meta.AttachmentKey)
	if key == "" {
		return nil
	}
	if err := os.MkdirAll(c.attachmentDir(key), 0o755); err != nil {
		return err
	}
	if doc.Meta.TextHash == "" && doc.Text != "" {
		hash := sha256.Sum256([]byte(doc.Text))
		doc.Meta.TextHash = "sha256:" + hex.EncodeToString(hash[:])
	}
	if doc.Meta.ExtractedAt == "" {
		doc.Meta.ExtractedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if doc.Meta.Chars == 0 && doc.Text != "" {
		doc.Meta.Chars = len([]rune(doc.Text))
	}
	if err := os.WriteFile(c.contentPath(key), []byte(doc.Text), 0o600); err != nil {
		return err
	}
	metaBytes, err := json.MarshalIndent(doc.Meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.metaPath(key), metaBytes, 0o600); err != nil {
		return err
	}
	return c.syncIndex(doc)
}

func (c fullTextCache) SaveBatch(docs []FullTextDocument) error {
	if len(docs) == 0 {
		return nil
	}

	for i := range docs {
		docs[i] = c.prepareDoc(docs[i])
		if err := c.writeFilesOnly(docs[i]); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite", c.indexPath())
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureFullTextIndexSchema(db); err != nil {
		return err
	}

	if err := c.writeMetaBatch(db, docs); err != nil {
		return err
	}

	return c.rebuildFTSIndex(db, docs)
}

func (c fullTextCache) writeMetaBatch(db *sql.DB, docs []FullTextDocument) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	metaStmt, err := tx.Prepare(`INSERT OR REPLACE INTO fulltext_meta (
		attachment_key, parent_item_key, resolved_path, content_type,
		title, creators, tags, attachment_title, attachment_name, attachment_path,
		extractor, source_mtime_unix, source_size, text_hash, extracted_at, pages, chars
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer metaStmt.Close()

	delMetaStmt, err := tx.Prepare(`DELETE FROM fulltext_meta WHERE attachment_key = ?`)
	if err != nil {
		return err
	}
	defer delMetaStmt.Close()

	for _, doc := range docs {
		key := doc.Meta.AttachmentKey
		if _, err := delMetaStmt.Exec(key); err != nil {
			return err
		}
		if _, err := metaStmt.Exec(
			doc.Meta.AttachmentKey,
			doc.Meta.ParentItemKey,
			doc.Meta.ResolvedPath,
			doc.Meta.ContentType,
			doc.Meta.Title,
			doc.Meta.Creators,
			doc.Meta.Tags,
			doc.Meta.AttachmentTitle,
			doc.Meta.AttachmentName,
			doc.Meta.AttachmentPath,
			doc.Meta.Extractor,
			doc.Meta.SourceMtimeUnix,
			doc.Meta.SourceSize,
			doc.Meta.TextHash,
			doc.Meta.ExtractedAt,
			doc.Meta.Pages,
			doc.Meta.Chars,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (c fullTextCache) rebuildFTSIndex(db *sql.DB, docs []FullTextDocument) error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS fulltext_documents;`,
		`DROP TABLE IF EXISTS fulltext_chunks;`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	for _, stmt := range []string{
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
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fulltext_chunks USING fts5(
			attachment_key UNINDEXED,
			parent_item_key UNINDEXED,
			chunk_index UNINDEXED,
			page UNINDEXED,
			bbox UNINDEXED,
			body
		);`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	docStmt, err := tx.Prepare(`INSERT INTO fulltext_documents (
		attachment_key, parent_item_key, content_type, resolved_path,
		title, creators, tags, attachment_title, attachment_name, attachment_path, body
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer docStmt.Close()

	chunkStmt, err := tx.Prepare(`INSERT INTO fulltext_chunks (
		attachment_key, parent_item_key, chunk_index, page, bbox, body
	) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer chunkStmt.Close()

	for _, doc := range docs {
		docChunks := doc.Chunks
		if len(docChunks) == 0 && doc.Text != "" {
			docChunks = []chunk{{Page: 1, Text: doc.Text, BlockCount: 1}}
		}
		if _, err := docStmt.Exec(
			doc.Meta.AttachmentKey,
			doc.Meta.ParentItemKey,
			doc.Meta.ContentType,
			doc.Meta.ResolvedPath,
			doc.Meta.Title,
			doc.Meta.Creators,
			doc.Meta.Tags,
			doc.Meta.AttachmentTitle,
			doc.Meta.AttachmentName,
			doc.Meta.AttachmentPath,
			doc.Text,
		); err != nil {
			return err
		}
		for ci, ch := range docChunks {
			bboxStr := fmt.Sprintf("[%g,%g,%g,%g]", ch.BBox[0], ch.BBox[1], ch.BBox[2], ch.BBox[3])
			if _, err := chunkStmt.Exec(
				doc.Meta.AttachmentKey,
				doc.Meta.ParentItemKey,
				ci,
				ch.Page,
				bboxStr,
				ch.Text,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (c fullTextCache) prepareDoc(doc FullTextDocument) FullTextDocument {
	if doc.Meta.TextHash == "" && doc.Text != "" {
		hash := sha256.Sum256([]byte(doc.Text))
		doc.Meta.TextHash = "sha256:" + hex.EncodeToString(hash[:])
	}
	if doc.Meta.ExtractedAt == "" {
		doc.Meta.ExtractedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if doc.Meta.Chars == 0 && doc.Text != "" {
		doc.Meta.Chars = len([]rune(doc.Text))
	}
	return doc
}

func (c fullTextCache) writeFilesOnly(doc FullTextDocument) error {
	key := strings.TrimSpace(doc.Meta.AttachmentKey)
	if key == "" {
		return nil
	}
	if err := os.MkdirAll(c.attachmentDir(key), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(c.contentPath(key), []byte(doc.Text), 0o600); err != nil {
		return err
	}
	metaBytes, err := json.MarshalIndent(doc.Meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.metaPath(key), metaBytes, 0o600)
}

func (c fullTextCache) IsFresh(meta fullTextCacheMeta, attachment domain.Attachment) bool {
	if strings.TrimSpace(meta.AttachmentKey) == "" || meta.AttachmentKey != attachment.Key {
		return false
	}
	if strings.TrimSpace(meta.ContentType) != strings.TrimSpace(attachment.ContentType) {
		return false
	}
	sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
	if !ok {
		return false
	}
	if filepath.Clean(meta.ResolvedPath) != filepath.Clean(sourcePath) {
		return false
	}
	return meta.SourceMtimeUnix == info.ModTime().Unix() && meta.SourceSize == info.Size()
}

func (c fullTextCache) IsMarkedFailed(key string) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(c.attachmentDir(key), ".failed"))
	return err == nil
}

func (c fullTextCache) MarkFailed(key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	dir := c.attachmentDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".failed"), []byte{}, 0o600)
}

func fullTextAttachmentSourceInfo(attachment domain.Attachment) (string, os.FileInfo, bool) {
	if attachment.Resolved && strings.TrimSpace(attachment.ResolvedPath) != "" {
		info, err := os.Stat(attachment.ResolvedPath)
		if err == nil {
			return attachment.ResolvedPath, info, true
		}
	}
	return "", nil, false
}

func (c fullTextCache) syncIndex(doc FullTextDocument) error {
	return c.syncIndexWithReset(doc, false)
}

func (c fullTextCache) syncIndexWithReset(doc FullTextDocument, reset bool) error {
	if reset {
		_ = os.Remove(c.indexPath())
	}
	if err := os.MkdirAll(c.rootDir, 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", c.indexPath())
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureFullTextIndexSchema(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM fulltext_meta WHERE attachment_key = ?`, doc.Meta.AttachmentKey); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO fulltext_meta (
		 attachment_key, parent_item_key, resolved_path, content_type,
		 title, creators, tags, attachment_title, attachment_name, attachment_path,
		 extractor, source_mtime_unix, source_size, text_hash, extracted_at, pages, chars
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.Meta.AttachmentKey,
		doc.Meta.ParentItemKey,
		doc.Meta.ResolvedPath,
		doc.Meta.ContentType,
		doc.Meta.Title,
		doc.Meta.Creators,
		doc.Meta.Tags,
		doc.Meta.AttachmentTitle,
		doc.Meta.AttachmentName,
		doc.Meta.AttachmentPath,
		doc.Meta.Extractor,
		doc.Meta.SourceMtimeUnix,
		doc.Meta.SourceSize,
		doc.Meta.TextHash,
		doc.Meta.ExtractedAt,
		doc.Meta.Pages,
		doc.Meta.Chars,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM fulltext_documents WHERE attachment_key = ?`, doc.Meta.AttachmentKey); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO fulltext_documents (
		 attachment_key, parent_item_key, content_type, resolved_path,
		 title, creators, tags, attachment_title, attachment_name, attachment_path, body
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.Meta.AttachmentKey,
		doc.Meta.ParentItemKey,
		doc.Meta.ContentType,
		doc.Meta.ResolvedPath,
		doc.Meta.Title,
		doc.Meta.Creators,
		doc.Meta.Tags,
		doc.Meta.AttachmentTitle,
		doc.Meta.AttachmentName,
		doc.Meta.AttachmentPath,
		doc.Text,
	); err != nil {
		if !reset && isFullTextIndexSchemaError(err) {
			_ = tx.Rollback()
			_ = db.Close()
			return c.syncIndexWithReset(doc, true)
		}
		return err
	}
	if _, err := tx.Exec(`DELETE FROM fulltext_chunks WHERE attachment_key = ?`, doc.Meta.AttachmentKey); err != nil {
		return err
	}
	chunks := doc.Chunks
	if len(chunks) == 0 && doc.Text != "" {
		chunks = []chunk{{Page: 1, Text: doc.Text, BlockCount: 1}}
	}
	if len(chunks) > 0 {
		chunkStmt, err := tx.Prepare(`INSERT INTO fulltext_chunks (
			attachment_key, parent_item_key, chunk_index, page, bbox, body
		) VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		for ci, ch := range chunks {
			bboxStr := fmt.Sprintf("[%g,%g,%g,%g]", ch.BBox[0], ch.BBox[1], ch.BBox[2], ch.BBox[3])
			if _, err := chunkStmt.Exec(
				doc.Meta.AttachmentKey,
				doc.Meta.ParentItemKey,
				ci,
				ch.Page,
				bboxStr,
				ch.Text,
			); err != nil {
				chunkStmt.Close()
				return err
			}
		}
		chunkStmt.Close()
	}
	if err := tx.Commit(); err != nil {
		if !reset && isFullTextIndexSchemaError(err) {
			_ = db.Close()
			return c.syncIndexWithReset(doc, true)
		}
		return err
	}
	return nil
}

func ensureFullTextIndexSchema(db *sql.DB) error {
	metaColumns := []string{
		"attachment_key", "parent_item_key", "resolved_path", "content_type",
		"title", "creators", "tags", "attachment_title", "attachment_name", "attachment_path",
		"extractor", "source_mtime_unix", "source_size", "text_hash", "extracted_at", "pages", "chars",
	}
	docColumns := []string{
		"attachment_key", "parent_item_key", "content_type", "resolved_path",
		"title", "creators", "tags", "attachment_title", "attachment_name", "attachment_path", "body",
	}
	chunkColumns := []string{
		"attachment_key", "parent_item_key", "chunk_index", "page", "bbox", "body",
	}
	metaOk, err := tableHasColumns(db, "fulltext_meta", metaColumns)
	if err != nil {
		return err
	}
	docOk, err := tableHasColumns(db, "fulltext_documents", docColumns)
	if err != nil {
		return err
	}
	_, chunksOk := true, true
	if rows, e := db.Query(`PRAGMA table_info(fulltext_chunks)`); e == nil {
		rows.Close()
		chunksOk, _ = tableHasColumns(db, "fulltext_chunks", chunkColumns)
	} else {
		rows.Close()
		chunksOk = false
	}
	if !metaOk || !docOk || !chunksOk {
		for _, stmt := range []string{
			`DROP TABLE IF EXISTS fulltext_documents;`,
			`DROP TABLE IF EXISTS fulltext_meta;`,
			`DROP TABLE IF EXISTS fulltext_chunks;`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS fulltext_meta (
		 attachment_key TEXT PRIMARY KEY,
		 parent_item_key TEXT,
		 resolved_path TEXT,
		 content_type TEXT,
		 title TEXT,
		 creators TEXT,
		 tags TEXT,
		 attachment_title TEXT,
		 attachment_name TEXT,
		 attachment_path TEXT,
		 extractor TEXT,
		 source_mtime_unix INTEGER,
		 source_size INTEGER,
		 text_hash TEXT,
		 extracted_at TEXT,
		 pages INTEGER,
		 chars INTEGER
		);`,
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
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fulltext_chunks USING fts5(
		 attachment_key UNINDEXED,
		 parent_item_key UNINDEXED,
		 chunk_index UNINDEXED,
		 page UNINDEXED,
		 bbox UNINDEXED,
		 body
		);`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func tableHasColumns(db *sql.DB, table string, required []string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	seen := make(map[string]struct{}, len(required))
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		seen[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	for _, column := range required {
		if _, ok := seen[column]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func isFullTextIndexSchemaError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no column named") || strings.Contains(message, "has no column named")
}

func (c fullTextCache) Search(query string, any bool, limit int) ([]fullTextIndexMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if _, err := os.Stat(c.indexPath()); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	db, err := sql.Open("sqlite", c.indexPath()+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	matchExpr := fullTextIndexMatchExpr(query, any)
	if matchExpr == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	fetchLimit := limit * 5
	if fetchLimit < 50 {
		fetchLimit = 50
	}

	return c.searchChunks(db, matchExpr, query, fetchLimit, limit)
}

func (c fullTextCache) searchChunks(db *sql.DB, matchExpr string, query string, fetchLimit int, limit int) ([]fullTextIndexMatch, error) {
	rows, err := db.Query(
		`SELECT fc.parent_item_key, fc.attachment_key,
		        bm25(fulltext_chunks, 1.0),
		        COALESCE(fm.title, ''),
		        COALESCE(fm.attachment_title, ''),
		        COALESCE(fm.attachment_name, ''),
		        COALESCE(fc.body, ''),
		        fc.chunk_index,
		        fc.page,
		        fc.bbox
		 FROM fulltext_chunks fc
		 LEFT JOIN fulltext_meta fm ON fm.attachment_key = fc.attachment_key
		 WHERE fulltext_chunks MATCH ?
		 ORDER BY bm25(fulltext_chunks, 1.0)
		 LIMIT ?`,
		matchExpr,
		fetchLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rawMatches := make([]fullTextIndexMatch, 0, fetchLimit)
	for rows.Next() {
		var match fullTextIndexMatch
		var bboxStr string
		if err := rows.Scan(
			&match.ParentItemKey,
			&match.AttachmentKey,
			&match.Rank,
			&match.Title,
			&match.AttachmentTitle,
			&match.AttachmentName,
			&match.Body,
			&match.ChunkIndex,
			&match.ChunkPage,
			&bboxStr,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(match.ParentItemKey) == "" {
			continue
		}
		parseBBoxString(bboxStr, &match.ChunkBBox)
		rawMatches = append(rawMatches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rankAndDedupeFullTextMatches(rawMatches, query, limit), nil
}

func parseBBoxString(s string, bbox *[4]float64) {
	if s == "" || len(s) < 5 {
		return
	}
	n, _ := fmt.Sscanf(s, "[%g,%g,%g,%g]", &bbox[0], &bbox[1], &bbox[2], &bbox[3])
	if n != 4 {
		*bbox = [4]float64{}
	}
}

func fullTextIndexMatchExpr(query string, any bool) string {
	tokens := fullTextQueryTokens(query)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, `"`+strings.ReplaceAll(token, `"`, `""`)+`"*`)
	}
	if any {
		return strings.Join(parts, " OR ")
	}
	return strings.Join(parts, " ")
}

func rankAndDedupeFullTextMatches(matches []fullTextIndexMatch, query string, limit int) []fullTextIndexMatch {
	if len(matches) == 0 {
		return nil
	}
	tokens := fullTextQueryTokens(query)
	queryLower := strings.ToLower(strings.TrimSpace(query))
	scored := make([]fullTextIndexMatch, 0, len(matches))
	for _, match := range matches {
		score := 1000.0 - match.Rank
		titleLower := strings.ToLower(match.Title)
		attachmentTitleLower := strings.ToLower(match.AttachmentTitle)
		attachmentNameLower := strings.ToLower(match.AttachmentName)
		bodyLower := strings.ToLower(match.Body)

		if queryLower != "" {
			switch {
			case strings.Contains(titleLower, queryLower):
				score += 500
			case strings.Contains(attachmentTitleLower, queryLower):
				score += 320
			case strings.Contains(attachmentNameLower, queryLower):
				score += 260
			case strings.Contains(bodyLower, queryLower):
				score += 120
			}
		}

		distinctCovered := 0
		for _, token := range tokens {
			if token == "" {
				continue
			}
			switch {
			case strings.Contains(titleLower, token):
				score += 120
				distinctCovered++
			case strings.Contains(attachmentTitleLower, token):
				score += 90
				distinctCovered++
			case strings.Contains(attachmentNameLower, token):
				score += 80
				distinctCovered++
			case strings.Contains(bodyLower, token):
				score += 30
				distinctCovered++
			}
		}
		score += float64(distinctCovered * 140)
		match.Rank = -score
		scored = append(scored, match)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Rank != scored[j].Rank {
			return scored[i].Rank < scored[j].Rank
		}
		if scored[i].ParentItemKey != scored[j].ParentItemKey {
			return scored[i].ParentItemKey < scored[j].ParentItemKey
		}
		return scored[i].AttachmentKey < scored[j].AttachmentKey
	})

	bestByParent := make(map[string]fullTextIndexMatch, len(scored))
	order := make([]string, 0, len(scored))
	for _, match := range scored {
		if _, ok := bestByParent[match.ParentItemKey]; ok {
			continue
		}
		bestByParent[match.ParentItemKey] = match
		order = append(order, match.ParentItemKey)
		if limit > 0 && len(order) >= limit {
			break
		}
	}

	result := make([]fullTextIndexMatch, 0, len(order))
	for _, parentKey := range order {
		result = append(result, bestByParent[parentKey])
	}
	return result
}
