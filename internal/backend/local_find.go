package backend

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"zotero_cli/internal/domain"
)

func localFindQuery(opts FindOptions) (string, []any) {
	if opts.FullText {
		return localFullTextFindQuery(opts)
	}
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

func localFullTextFindQuery(opts FindOptions) (string, []any) {
	tokens := localFullTextTokens(opts.Query)
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
		JOIN itemAttachments iaf ON iaf.parentItemID = i.itemID
		JOIN fulltextItemWords fiw ON fiw.itemID = iaf.itemID
		JOIN fulltextWords fw ON fw.wordID = fiw.wordID
		WHERE ` + localVisibleItemClause(opts.ItemType) + `
		` + localFullTextMatchClause(tokens) + `
		` + localTagFilterClause(opts) + `
		GROUP BY i.itemID, i.key, i.version, it.typeName
		` + localFullTextHavingClause(tokens) + `
		ORDER BY i.key
	`
	args := localFullTextArgs(opts, tokens)
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
		queryLike,
		queryLike,
		queryLike,
	)
	for _, tag := range normalizedTags(opts.Tags) {
		args = append(args, tag)
	}
	return args
}

func localFullTextArgs(opts FindOptions, tokens []string) []any {
	args := []any{}
	if opts.ItemType != "" {
		args = append(args, opts.ItemType)
	}
	for _, token := range tokens {
		args = append(args, token)
	}
	for _, tag := range normalizedTags(opts.Tags) {
		args = append(args, tag)
	}
	if len(tokens) > 0 {
		args = append(args, len(tokens))
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
		OR EXISTS (
			SELECT 1
			FROM itemAttachments ia2
			LEFT JOIN itemData d3 ON d3.itemID = ia2.itemID
			LEFT JOIN itemDataValues v3 ON v3.valueID = d3.valueID
			LEFT JOIN fieldsCombined f3 ON f3.fieldID = d3.fieldID
			WHERE (ia2.itemID = i.itemID OR ia2.parentItemID = i.itemID)
			AND (
				LOWER(COALESCE(ia2.path, '')) LIKE ?
				OR LOWER(COALESCE(ia2.contentType, '')) LIKE ?
				OR (f3.fieldName IN ('title', 'filename') AND LOWER(v3.value) LIKE ?)
			)
		)
	)`
}

func localFullTextMatchClause(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return `
		AND LOWER(fw.word) IN (` + placeholders(len(tokens)) + `)
	`
}

func localFullTextHavingClause(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return `
		HAVING COUNT(DISTINCT LOWER(fw.word)) = ?
	`
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

func findFieldIncluded(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func localFilterAndOrderItems(items []domain.Item, opts FindOptions) []domain.Item {
	filtered := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if !MatchesDateRange(item.Date, opts.DateAfter, opts.DateBefore) {
			continue
		}
		if !matchesAttachmentFilters(item.Attachments, opts) {
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

func matchesAttachmentFilters(attachments []domain.Attachment, opts FindOptions) bool {
	if !opts.HasPDF && strings.TrimSpace(opts.AttachmentName) == "" && strings.TrimSpace(opts.AttachmentPath) == "" && strings.TrimSpace(opts.AttachmentType) == "" {
		return true
	}
	nameNeedle := strings.ToLower(strings.TrimSpace(opts.AttachmentName))
	pathNeedle := strings.ToLower(strings.TrimSpace(opts.AttachmentPath))
	typeNeedle := strings.ToLower(strings.TrimSpace(opts.AttachmentType))
	for _, attachment := range attachments {
		if opts.HasPDF && attachment.ContentType != "application/pdf" {
			continue
		}
		if nameNeedle != "" {
			name := strings.ToLower(firstNonEmptyString(attachment.Filename, attachment.Title))
			if !strings.Contains(name, nameNeedle) {
				continue
			}
		}
		if pathNeedle != "" {
			if !attachmentPathMatchesFilter(attachment, pathNeedle) {
				continue
			}
		}
		if typeNeedle != "" {
			if !strings.Contains(strings.ToLower(attachment.ContentType), typeNeedle) {
				continue
			}
		}
		return true
	}
	return false
}

func attachmentPathMatchesFilter(attachment domain.Attachment, needle string) bool {
	return strings.Contains(strings.ToLower(attachment.ZoteroPath), needle) ||
		strings.Contains(strings.ToLower(attachment.ResolvedPath), needle)
}

func localMatchedOn(item domain.Item, opts FindOptions) []string {
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	if query == "" {
		return nil
	}
	if opts.FullText {
		return []string{"fulltext_attachment"}
	}
	matched := []string{}
	add := func(reason string) {
		for _, existing := range matched {
			if existing == reason {
				return
			}
		}
		matched = append(matched, reason)
	}

	if strings.Contains(strings.ToLower(item.Key), query) {
		add("key")
	}
	if localItemMetadataMatches(item, query) {
		add("metadata")
	}
	for _, creator := range item.Creators {
		if strings.Contains(strings.ToLower(strings.TrimSpace(creator.Name)), query) {
			add("creator")
			break
		}
	}
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			add("tag")
			break
		}
	}
	for _, attachment := range item.Attachments {
		if strings.Contains(strings.ToLower(attachment.Title), query) {
			add("attachment_title")
		}
		if strings.Contains(strings.ToLower(attachment.Filename), query) {
			add("attachment_filename")
		}
		if strings.Contains(strings.ToLower(attachment.ZoteroPath), query) || strings.Contains(strings.ToLower(attachment.ResolvedPath), query) {
			add("attachment_path")
		}
		if strings.Contains(strings.ToLower(attachment.ContentType), query) {
			add("attachment_content_type")
		}
	}
	return matched
}

func localFullTextTokens(query string) []string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	tokens := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		tokens = append(tokens, part)
	}
	return tokens
}

func localItemMetadataMatches(item domain.Item, query string) bool {
	for _, value := range []string{
		item.Title,
		item.Container,
		item.Date,
	} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
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
