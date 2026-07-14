package backend

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type CollectionTarget struct {
	ID   int64
	Key  string
	Name string
	Path string
}

type collectionTargetRow struct {
	target   CollectionTarget
	parentID int64
}

type ImportedAttachment struct {
	Key       string
	ParentKey string
	LinkMode  int
	Path      string
}

func (r *LocalReader) ImportedPDFAttachments(ctx context.Context, sourceURL string) ([]ImportedAttachment, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	rows, err := db.QueryContext(ctx, `
		SELECT a.key, COALESCE(p.key, ''), COALESCE(ia.linkMode, 0), COALESCE(ia.path, '')
		FROM itemAttachments ia
		JOIN items a ON a.itemID = ia.itemID
		LEFT JOIN items p ON p.itemID = ia.parentItemID
		LEFT JOIN itemData d ON d.itemID = a.itemID
		LEFT JOIN itemDataValues v ON v.valueID = d.valueID
		LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
		WHERE COALESCE(ia.contentType, '') = 'application/pdf'
		GROUP BY a.itemID, a.key, p.key, ia.linkMode, ia.path
		HAVING MAX(CASE WHEN f.fieldName = 'url' THEN v.value END) = ?
		ORDER BY a.key
	`, sourceURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImportedAttachment
	for rows.Next() {
		var attachment ImportedAttachment
		if err := rows.Scan(&attachment.Key, &attachment.ParentKey, &attachment.LinkMode, &attachment.Path); err != nil {
			return nil, err
		}
		out = append(out, attachment)
	}
	return out, rows.Err()
}

func (r *LocalReader) CollectionTarget(ctx context.Context, selector string) (CollectionTarget, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return CollectionTarget{}, err
	}
	defer db.Close()
	defer cleanup()

	rows, err := db.QueryContext(ctx, `SELECT collectionID, key, collectionName, parentCollectionID FROM collections`)
	if err != nil {
		return CollectionTarget{}, err
	}
	defer rows.Close()

	collections := map[int64]collectionTargetRow{}
	for rows.Next() {
		var row collectionTargetRow
		var parent sql.NullInt64
		if err := rows.Scan(&row.target.ID, &row.target.Key, &row.target.Name, &parent); err != nil {
			return CollectionTarget{}, err
		}
		if parent.Valid {
			row.parentID = parent.Int64
		}
		collections[row.target.ID] = row
	}
	if err := rows.Err(); err != nil {
		return CollectionTarget{}, err
	}

	value := strings.TrimSpace(selector)
	for id, row := range collections {
		path, err := collectionTargetPath(id, collections)
		if err != nil {
			return CollectionTarget{}, err
		}
		row.target.Path = path
		collections[id] = row
		if strings.EqualFold(row.target.Key, value) {
			return row.target, nil
		}
	}

	normalizedPath := normalizeCollectionTargetPath(value)
	pathMatches := matchingCollectionTargets(collections, func(target CollectionTarget) bool {
		return strings.EqualFold(target.Path, normalizedPath)
	})
	if len(pathMatches) == 1 {
		return pathMatches[0], nil
	}
	if len(pathMatches) > 1 {
		return CollectionTarget{}, ambiguousCollectionTargetError(selector, pathMatches)
	}

	nameMatches := matchingCollectionTargets(collections, func(target CollectionTarget) bool {
		return strings.EqualFold(target.Name, value)
	})
	if len(nameMatches) == 1 {
		return nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		return CollectionTarget{}, ambiguousCollectionTargetError(selector, nameMatches)
	}
	return CollectionTarget{}, fmt.Errorf("collection %q not found; use `zot coll list` to inspect collection keys and names", selector)
}

func collectionTargetPath(id int64, collections map[int64]collectionTargetRow) (string, error) {
	parts := []string{}
	seen := map[int64]bool{}
	for id != 0 {
		if seen[id] {
			return "", fmt.Errorf("collection hierarchy contains a cycle at id %d", id)
		}
		seen[id] = true
		row, ok := collections[id]
		if !ok {
			return "", fmt.Errorf("collection hierarchy references missing parent id %d", id)
		}
		parts = append(parts, row.target.Name)
		id = row.parentID
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, "/"), nil
}

func normalizeCollectionTargetPath(value string) string {
	parts := strings.FieldsFunc(strings.ReplaceAll(value, `\`, "/"), func(r rune) bool { return r == '/' })
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, "/")
}

func matchingCollectionTargets(collections map[int64]collectionTargetRow, match func(CollectionTarget) bool) []CollectionTarget {
	result := []CollectionTarget{}
	for _, row := range collections {
		if match(row.target) {
			result = append(result, row.target)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Key < result[j].Key
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func ambiguousCollectionTargetError(selector string, matches []CollectionTarget) error {
	candidates := make([]string, 0, len(matches))
	for _, target := range matches {
		candidates = append(candidates, fmt.Sprintf("%s (%s)", target.Path, target.Key))
	}
	return fmt.Errorf("collection %q is ambiguous; use a collection key or full path: %s", selector, strings.Join(candidates, ", "))
}

func (r *LocalReader) ExportItemsCSLJSON(ctx context.Context, keys []string) ([]map[string]any, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		item, itemID, err := r.loadItem(ctx, db, key)
		if err != nil {
			return nil, err
		}
		authors, err := r.loadCSLCreators(ctx, db, itemID)
		if err != nil {
			return nil, err
		}

		entry := map[string]any{
			"id":    item.Key,
			"type":  cslTypeForItem(item.ItemType),
			"title": item.Title,
		}
		if len(authors) > 0 {
			entry["author"] = authors
		}
		if issued := cslIssued(item.Date); len(issued) > 0 {
			entry["issued"] = map[string]any{"date-parts": []any{issued}}
		}
		if item.Container != "" {
			entry["container-title"] = item.Container
		}
		if item.Volume != "" {
			entry["volume"] = item.Volume
		}
		if item.Issue != "" {
			entry["issue"] = item.Issue
		}
		if item.Pages != "" {
			entry["page"] = item.Pages
		}
		if item.DOI != "" {
			entry["DOI"] = item.DOI
		}
		if item.URL != "" {
			entry["URL"] = item.URL
		}
		out = append(out, entry)
	}
	return out, nil
}

func (r *LocalReader) loadCSLCreators(ctx context.Context, db *sql.DB, itemID int64) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(c.firstName, ''),
			COALESCE(c.lastName, ''),
			COALESCE(c.fieldMode, 0)
		FROM itemCreators ic
		JOIN creators c ON c.creatorID = ic.creatorID
		WHERE ic.itemID = ?
		ORDER BY ic.orderIndex
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	creators := []map[string]any{}
	for rows.Next() {
		var firstName string
		var lastName string
		var fieldMode int
		if err := rows.Scan(&firstName, &lastName, &fieldMode); err != nil {
			return nil, err
		}
		firstName = strings.TrimSpace(firstName)
		lastName = strings.TrimSpace(lastName)
		switch {
		case fieldMode == 1 && lastName != "":
			creators = append(creators, map[string]any{"literal": lastName})
		case firstName != "" || lastName != "":
			creator := map[string]any{}
			if firstName != "" {
				creator["given"] = firstName
			}
			if lastName != "" {
				creator["family"] = lastName
			}
			creators = append(creators, creator)
		}
	}
	return creators, rows.Err()
}

func cslTypeForItem(itemType string) string {
	switch itemType {
	case "journalArticle":
		return "article-journal"
	case "book":
		return "book"
	case "conferencePaper":
		return "paper-conference"
	case "thesis":
		return "thesis"
	default:
		return "article"
	}
}

func cslIssued(date string) []any {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil
	}
	parts := strings.Split(strings.Fields(date)[0], "-")
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "00" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
