package backend

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
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
	Extractor       string `json:"extractor,omitempty"`
	SourceMtimeUnix int64  `json:"source_mtime_unix,omitempty"`
	SourceSize      int64  `json:"source_size,omitempty"`
	TextHash        string `json:"text_hash,omitempty"`
	ExtractedAt     string `json:"extracted_at,omitempty"`
	Pages           int    `json:"pages,omitempty"`
	Chars           int    `json:"chars,omitempty"`
}

type fullTextDocument struct {
	Text string
	Meta fullTextCacheMeta
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

func (c fullTextCache) Load(attachment domain.Attachment) (fullTextDocument, bool, error) {
	key := strings.TrimSpace(attachment.Key)
	if key == "" {
		return fullTextDocument{}, false, nil
	}
	content, err := os.ReadFile(c.contentPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return fullTextDocument{}, false, nil
		}
		return fullTextDocument{}, false, err
	}
	metaBytes, err := os.ReadFile(c.metaPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return fullTextDocument{}, false, nil
		}
		return fullTextDocument{}, false, err
	}
	var meta fullTextCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return fullTextDocument{}, false, err
	}
	if !c.IsFresh(meta, attachment) {
		return fullTextDocument{}, false, nil
	}
	return fullTextDocument{Text: string(content), Meta: meta}, true, nil
}

func (c fullTextCache) Save(doc fullTextDocument) error {
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

func fullTextAttachmentSourceInfo(attachment domain.Attachment) (string, os.FileInfo, bool) {
	if attachment.Resolved && strings.TrimSpace(attachment.ResolvedPath) != "" {
		info, err := os.Stat(attachment.ResolvedPath)
		if err == nil {
			return attachment.ResolvedPath, info, true
		}
	}
	return "", nil, false
}

func (c fullTextCache) syncIndex(doc fullTextDocument) error {
	if err := os.MkdirAll(c.rootDir, 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", c.indexPath())
	if err != nil {
		return err
	}
	defer db.Close()

	schema := []string{
		`CREATE TABLE IF NOT EXISTS fulltext_meta (
		 attachment_key TEXT PRIMARY KEY,
		 parent_item_key TEXT,
		 resolved_path TEXT,
		 content_type TEXT,
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
		 body
		);`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
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
		 attachment_key, parent_item_key, resolved_path, content_type, extractor,
		 source_mtime_unix, source_size, text_hash, extracted_at, pages, chars
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.Meta.AttachmentKey,
		doc.Meta.ParentItemKey,
		doc.Meta.ResolvedPath,
		doc.Meta.ContentType,
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
		 attachment_key, parent_item_key, content_type, resolved_path, body
		) VALUES (?, ?, ?, ?, ?)`,
		doc.Meta.AttachmentKey,
		doc.Meta.ParentItemKey,
		doc.Meta.ContentType,
		doc.Meta.ResolvedPath,
		doc.Text,
	); err != nil {
		return err
	}
	return tx.Commit()
}
