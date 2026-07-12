package backend

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"zotero_cli/internal/domain"
)

func localExactKeyCandidateQuery(opts FindOptions) (string, []any) {
	query := `
		SELECT i.itemID, i.key
		FROM items i
		JOIN itemTypes it ON it.itemTypeID=i.itemTypeID
		WHERE ` + localVisibleItemClause(opts) + `
		AND i.key = ?
		LIMIT 1
	`
	return query, []any{strings.TrimSpace(opts.Query)}
}

func (r *LocalReader) findItemFromExactKey(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	var items []domain.Item
	err := r.withReadableDB(ctx, func(db *sql.DB) error {
		query, args := localExactKeyCandidateQuery(opts)
		var itemID int64
		var key string
		if err := db.QueryRowContext(ctx, query, args...).Scan(&itemID, &key); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		loaded, _, err := r.batchLoadItemsByKeys(ctx, db, []string{key})
		if err != nil {
			return err
		}
		item := loaded[key]
		item.SearchScore = metadataFindScore(item, opts.Query)
		items = []domain.Item{item}
		if err := r.enrichCandidateItems(ctx, db, items, []int64{itemID}, opts); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !opts.Full && !findFieldIncluded(opts.IncludeFields, "attachments") {
		for i := range items {
			items[i].Attachments = nil
		}
	}
	return items, nil
}

func (r *LocalReader) enrichCandidateItems(ctx context.Context, db *sql.DB, items []domain.Item, itemIDs []int64, opts FindOptions) error {
	var creators map[int64][]domain.Creator
	var tags map[int64][]string
	var attachments map[int64][]domain.Attachment
	var creatorsErr, tagsErr, attachmentsErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); creators, creatorsErr = r.loadCreatorsByItemIDs(ctx, db, itemIDs) }()
	go func() { defer wg.Done(); tags, tagsErr = r.loadTagsByItemIDs(ctx, db, itemIDs) }()
	go func() {
		defer wg.Done()
		attachments, attachmentsErr = r.loadAttachmentsByParentItemIDs(ctx, db, itemIDs)
	}()
	wg.Wait()
	if creatorsErr != nil {
		return creatorsErr
	}
	if tagsErr != nil {
		return tagsErr
	}
	if attachmentsErr != nil {
		return attachmentsErr
	}
	for index, itemID := range itemIDs {
		items[index].Creators = creators[itemID]
		items[index].Tags = tags[itemID]
		items[index].Attachments = attachments[itemID]
		items[index].MatchedOn = localMatchedOn(items[index], opts)
	}
	return nil
}
