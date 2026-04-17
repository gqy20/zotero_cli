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

	"zotero_cli/internal/domain"
)

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

func (r *LocalReader) loadAttachmentsByParentItemIDs(ctx context.Context, db *sql.DB, itemIDs []int64) (map[int64][]domain.Attachment, error) {
	result := make(map[int64][]domain.Attachment, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		args = append(args, itemID)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			ia.parentItemID,
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
		WHERE ia.parentItemID IN (`+placeholders(len(itemIDs))+`)
		GROUP BY ia.parentItemID, i.itemID, i.key, it.typeName, ia.contentType, ia.linkMode, ia.path
		ORDER BY ia.parentItemID, i.key
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var parentItemID int64
		var attachment domain.Attachment
		var linkMode int
		if err := rows.Scan(
			&parentItemID,
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
		result[parentItemID] = append(result[parentItemID], attachment)
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
	if after, ok := strings.CutPrefix(zoteroPath, "storage:"); ok {
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
	if after, ok := strings.CutPrefix(zoteroPath, "attachments:"); ok && r.AttachmentBaseDir != "" {
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
