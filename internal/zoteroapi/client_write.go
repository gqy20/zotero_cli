package zoteroapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
)

func (c *Client) CreateItem(ctx context.Context, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	result, err := c.CreateItems(ctx, []map[string]any{data}, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	return firstWriteResult(result), nil
}

func (c *Client) UpdateItem(ctx context.Context, key string, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPatch, path.Join("items", key), data, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) DeleteItem(ctx context.Context, key string, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodDelete, path.Join("items", key), nil, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) DeleteItems(ctx context.Context, keys []string, ifUnmodifiedSinceVersion int) (BatchWriteResult, error) {
	resp, err := c.doDeleteByKeysRequest(ctx, "items", keys, ifUnmodifiedSinceVersion)
	if err != nil {
		return BatchWriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return BatchWriteResult{}, apiErrorFromResponse(resp)
	}

	successful := make([]WriteResult, 0, len(keys))
	for _, key := range keys {
		successful = append(successful, WriteResult{Key: key})
	}

	return BatchWriteResult{
		Successful:          successful,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) CreateItems(ctx context.Context, data []map[string]any, ifUnmodifiedSinceVersion int) (BatchWriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPost, "items", data, ifUnmodifiedSinceVersion)
	if err != nil {
		return BatchWriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BatchWriteResult{}, apiErrorFromResponse(resp)
	}

	var result apiWriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BatchWriteResult{}, err
	}

	return mapBatchWriteResult(result, resp.Header), nil
}

func (c *Client) UpdateItems(ctx context.Context, data []map[string]any, ifUnmodifiedSinceVersion int) (BatchWriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPatch, "items", data, ifUnmodifiedSinceVersion)
	if err != nil {
		return BatchWriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BatchWriteResult{}, apiErrorFromResponse(resp)
	}

	var result apiWriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BatchWriteResult{}, err
	}

	return mapBatchWriteResult(result, resp.Header), nil
}

func (c *Client) GetItemsByKeys(ctx context.Context, keys []string) ([]Item, error) {
	resp, err := c.doRequest(ctx, "items", FindOptions{}, map[string]string{
		"itemKey": strings.Join(keys, ","),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiErrorFromResponse(resp)
	}

	var raw []apiItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(raw))
	for _, item := range raw {
		items = append(items, mapItem(item))
	}
	return items, nil
}

func (c *Client) CreateCollection(ctx context.Context, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPost, "collections", []map[string]any{data}, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	var result apiWriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return WriteResult{}, err
	}

	writeResult := WriteResult{
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}
	if success, ok := result.Successful["0"]; ok {
		writeResult.Key = success.Key
	}
	return writeResult, nil
}

func (c *Client) UpdateCollection(ctx context.Context, key string, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPut, path.Join("collections", key), data, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) DeleteCollection(ctx context.Context, key string, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodDelete, path.Join("collections", key), nil, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) CreateSearch(ctx context.Context, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPost, "searches", []map[string]any{data}, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	var result apiWriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return WriteResult{}, err
	}

	writeResult := WriteResult{
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}
	if success, ok := result.Successful["0"]; ok {
		writeResult.Key = success.Key
	}
	return writeResult, nil
}

func (c *Client) UpdateSearch(ctx context.Context, key string, data map[string]any, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodPut, path.Join("searches", key), data, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}

func (c *Client) DeleteSearch(ctx context.Context, key string, ifUnmodifiedSinceVersion int) (WriteResult, error) {
	resp, err := c.doWriteRequest(ctx, http.MethodDelete, path.Join("searches", key), nil, ifUnmodifiedSinceVersion)
	if err != nil {
		return WriteResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return WriteResult{}, apiErrorFromResponse(resp)
	}

	return WriteResult{
		Key:                 key,
		LastModifiedVersion: parseLastModifiedVersion(resp.Header.Get("Last-Modified-Version")),
	}, nil
}
