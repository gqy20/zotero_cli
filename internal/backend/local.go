package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type LocalReader struct {
	DataDir    string
	SQLitePath string
	StorageDir string
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
		DataDir:    dataDir,
		SQLitePath: sqlitePath,
		StorageDir: storageDir,
	}, nil
}

func (r *LocalReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	return nil, fmt.Errorf("local find is not implemented yet")
}

func (r *LocalReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	db, err := sql.Open("sqlite", r.SQLitePath)
	if err != nil {
		return domain.Item{}, err
	}
	defer db.Close()

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

	item.Creators = creators
	item.Tags = tags
	item.Collections = collections
	item.Attachments = attachments
	return item, nil
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
		&item.DOI,
		&item.URL,
		&publicationTitle,
		&proceedingsTitle,
		&bookTitle,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Item{}, 0, fmt.Errorf("item not found: %s", key)
		}
		return domain.Item{}, 0, err
	}
	item.Container = firstNonEmptyString(publicationTitle, proceedingsTitle, bookTitle)
	return item, itemID, nil
}

func (r *LocalReader) loadCreators(ctx context.Context, db *sql.DB, itemID int64) ([]domain.Creator, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			TRIM(COALESCE(cd.firstName, '') || ' ' || COALESCE(cd.lastName, '')),
			COALESCE(ct.typeName, '')
		FROM itemCreators ic
		JOIN creators c ON c.creatorID = ic.creatorID
		JOIN creatorData cd ON cd.creatorDataID = c.creatorDataID
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
