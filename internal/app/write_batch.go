package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

const zoteroWriteBatchSize = 50

type batchItemReader interface {
	GetItemsByKeys(context.Context, []string) ([]zoteroapi.Item, error)
}

type batchItemUpdater interface {
	UpdateItems(context.Context, []map[string]any, int) (zoteroapi.BatchWriteResult, error)
}

type batchDeleteFunc func(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)

func uniqueKeys(keys []string) ([]string, error) {
	result := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("item keys must not be empty")
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result, nil
}

func getItemsByKeysInBatches(ctx context.Context, client batchItemReader, keys []string) ([]zoteroapi.Item, error) {
	items := make([]zoteroapi.Item, 0, len(keys))
	for start := 0; start < len(keys); start += zoteroWriteBatchSize {
		end := min(start+zoteroWriteBatchSize, len(keys))
		batch, err := client.GetItemsByKeys(ctx, keys[start:end])
		if err != nil {
			return nil, err
		}
		items = append(items, batch...)
	}
	return items, nil
}

func missingItemKeys(keys []string, items []zoteroapi.Item) []string {
	found := make(map[string]struct{}, len(items))
	for _, item := range items {
		found[item.Key] = struct{}{}
	}
	missing := make([]string, 0)
	for _, key := range keys {
		if _, ok := found[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func updateItemsInBatches(ctx context.Context, client batchItemUpdater, payload []map[string]any, version int) (zoteroapi.BatchWriteResult, error) {
	var result zoteroapi.BatchWriteResult
	for start := 0; start < len(payload); start += zoteroWriteBatchSize {
		end := min(start+zoteroWriteBatchSize, len(payload))
		batch, err := client.UpdateItems(ctx, payload[start:end], version)
		if err != nil {
			return result, err
		}
		mergeBatchWriteResult(&result, batch, start)
		if len(batch.Failed) > 0 {
			return result, fmt.Errorf("batch update failed for %d item(s)", len(batch.Failed))
		}
		if batch.LastModifiedVersion > 0 {
			version = batch.LastModifiedVersion
		}
	}
	return result, nil
}

func deleteKeysInBatches(ctx context.Context, deleteBatch batchDeleteFunc, keys []string, version int) (zoteroapi.BatchWriteResult, error) {
	var result zoteroapi.BatchWriteResult
	for start := 0; start < len(keys); start += zoteroWriteBatchSize {
		end := min(start+zoteroWriteBatchSize, len(keys))
		batch, err := deleteBatch(ctx, keys[start:end], version)
		if err != nil {
			return result, err
		}
		mergeBatchWriteResult(&result, batch, start)
		if len(batch.Failed) > 0 {
			return result, fmt.Errorf("batch delete failed for %d object(s)", len(batch.Failed))
		}
		if batch.LastModifiedVersion > 0 {
			version = batch.LastModifiedVersion
		}
	}
	return result, nil
}

func mergeBatchWriteResult(target *zoteroapi.BatchWriteResult, batch zoteroapi.BatchWriteResult, offset int) {
	target.Successful = append(target.Successful, batch.Successful...)
	for _, index := range batch.Unchanged {
		target.Unchanged = append(target.Unchanged, offsetBatchIndex(index, offset))
	}
	if len(batch.Failed) > 0 && target.Failed == nil {
		target.Failed = make(map[string]any, len(batch.Failed))
	}
	for index, failure := range batch.Failed {
		target.Failed[offsetBatchIndex(index, offset)] = failure
	}
	if batch.LastModifiedVersion > 0 {
		target.LastModifiedVersion = batch.LastModifiedVersion
	}
}

func offsetBatchIndex(index string, offset int) string {
	parsed, err := strconv.Atoi(index)
	if err != nil {
		return fmt.Sprintf("%d:%s", offset/zoteroWriteBatchSize, index)
	}
	return strconv.Itoa(offset + parsed)
}

func formatMissingKeys(keys []string) string {
	keys = append([]string(nil), keys...)
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
