package backend

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

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

func (r *LocalReader) CollectionItemKeys(ctx context.Context, collectionKey string, limit int) ([]string, error) {
	db, cleanup, err := r.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	query := `
		SELECT i.key
		FROM collections c
		JOIN collectionItems ci ON ci.collectionID = c.collectionID
		JOIN items i ON i.itemID = ci.itemID
		JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
		WHERE c.key = ?
		AND NOT EXISTS (SELECT 1 FROM itemAttachments ia WHERE ia.itemID = i.itemID)
		AND NOT EXISTS (SELECT 1 FROM itemNotes n WHERE n.itemID = i.itemID)
		AND NOT EXISTS (SELECT 1 FROM itemAnnotations a WHERE a.itemID = i.itemID)
		AND it.typeName <> 'annotation'
		ORDER BY ci.itemID
	`
	rows, err := db.QueryContext(ctx, query, collectionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
		if limit > 0 && len(keys) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
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
