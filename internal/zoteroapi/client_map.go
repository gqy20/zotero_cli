package zoteroapi

import (
	"context"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func (c *Client) getItems(ctx context.Context, relativePath string, opts FindOptions) ([]apiItem, error) {
	return c.fetchAllItems(ctx, relativePath, opts)
}

func (c *Client) getCollections(ctx context.Context) ([]apiCollection, error) {
	return c.fetchAllCollections(ctx, "collections")
}

func (c *Client) getNotes(ctx context.Context) ([]apiNoteItem, error) {
	return c.fetchAllNotes(ctx, "items", FindOptions{ItemType: "note"})
}

func (c *Client) getTags(ctx context.Context) ([]apiTagResponse, error) {
	return c.fetchAllTags(ctx, "tags")
}

func (c *Client) getSearches(ctx context.Context) ([]apiSearch, error) {
	return c.fetchAllSearches(ctx, "searches")
}

func (c *Client) fetchAllItems(ctx context.Context, relativePath string, opts FindOptions) ([]apiItem, error) {
	all := make([]apiItem, 0)
	current := opts

	for {
		resp, err := c.doRequest(ctx, relativePath, current, nil)
		if err != nil {
			return nil, err
		}

		page, total, err := decodeResponseWithTotal[apiItem](resp)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)
		if !shouldContinuePagination(len(page), len(all), total, current.Limit) {
			return all, nil
		}

		current.Start += len(page)
	}
}

func (c *Client) fetchAllCollections(ctx context.Context, relativePath string) ([]apiCollection, error) {
	all := make([]apiCollection, 0)
	opts := FindOptions{}

	for {
		resp, err := c.doRequest(ctx, relativePath, opts, nil)
		if err != nil {
			return nil, err
		}

		page, total, err := decodeResponseWithTotal[apiCollection](resp)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)
		if !shouldContinuePagination(len(page), len(all), total, 0) {
			return all, nil
		}

		opts.Start += len(page)
	}
}

func (c *Client) fetchAllNotes(ctx context.Context, relativePath string, opts FindOptions) ([]apiNoteItem, error) {
	all := make([]apiNoteItem, 0)
	current := opts

	for {
		resp, err := c.doRequest(ctx, relativePath, current, nil)
		if err != nil {
			return nil, err
		}

		page, total, err := decodeResponseWithTotal[apiNoteItem](resp)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)
		if !shouldContinuePagination(len(page), len(all), total, current.Limit) {
			return all, nil
		}

		current.Start += len(page)
	}
}

func (c *Client) fetchAllTags(ctx context.Context, relativePath string) ([]apiTagResponse, error) {
	all := make([]apiTagResponse, 0)
	opts := FindOptions{}

	for {
		resp, err := c.doRequest(ctx, relativePath, opts, nil)
		if err != nil {
			return nil, err
		}

		page, total, err := decodeResponseWithTotal[apiTagResponse](resp)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)
		if !shouldContinuePagination(len(page), len(all), total, 0) {
			return all, nil
		}

		opts.Start += len(page)
	}
}

func (c *Client) fetchAllSearches(ctx context.Context, relativePath string) ([]apiSearch, error) {
	all := make([]apiSearch, 0)
	opts := FindOptions{}

	for {
		resp, err := c.doRequest(ctx, relativePath, opts, nil)
		if err != nil {
			return nil, err
		}

		page, total, err := decodeResponseWithTotal[apiSearch](resp)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)
		if !shouldContinuePagination(len(page), len(all), total, 0) {
			return all, nil
		}

		opts.Start += len(page)
	}
}

func mapBatchWriteResult(result apiWriteResponse, header http.Header) BatchWriteResult {
	writeResult := BatchWriteResult{
		Failed:              result.Failed,
		LastModifiedVersion: parseLastModifiedVersion(header.Get("Last-Modified-Version")),
	}

	for _, index := range sortedMapKeys(result.Successful) {
		success := result.Successful[index]
		writeResult.Successful = append(writeResult.Successful, WriteResult{
			Key:                 success.Key,
			LastModifiedVersion: success.Version,
		})
	}
	writeResult.Unchanged = append(writeResult.Unchanged, sortedMapKeys(result.Unchanged)...)
	return writeResult
}

func firstWriteResult(result BatchWriteResult) WriteResult {
	writeResult := WriteResult{
		LastModifiedVersion: result.LastModifiedVersion,
	}
	if len(result.Successful) > 0 {
		writeResult.Key = result.Successful[0].Key
	}
	return writeResult
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, leftErr := strconv.Atoi(keys[i])
		right, rightErr := strconv.Atoi(keys[j])
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return keys[i] < keys[j]
	})
	return keys
}

func stripHTML(value string) string {
	return html.UnescapeString(htmlTagRe.ReplaceAllString(value, " "))
}

func compactWhitespace(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	replacer := strings.NewReplacer(
		" .", ".",
		" ,", ",",
		" ;", ";",
		" :", ":",
		" )", ")",
		"( ", "(",
	)
	return replacer.Replace(value)
}

func mapItem(item apiItem) Item {
	return Item{
		Version:   item.Version,
		Key:       item.Key,
		ItemType:  item.Data.ItemType,
		Title:     item.Data.Title,
		Date:      item.Data.Date,
		Creators:  mapCreators(item.Data.Creators),
		Container: firstNonEmpty(item.Data.PublicationTitle, item.Data.ProceedingsTitle, item.Data.BookTitle),
		Volume:    item.Data.Volume,
		Issue:     item.Data.Issue,
		Pages:     item.Data.Pages,
		DOI:       item.Data.DOI,
		URL:       item.Data.URL,
		Tags:      mapTags(item.Data.Tags),
	}
}

func mapCreators(creators []apiCreator) []Creator {
	out := make([]Creator, 0, len(creators))
	for _, creator := range creators {
		name := strings.TrimSpace(creator.Name)
		if name == "" {
			name = strings.TrimSpace(strings.TrimSpace(creator.FirstName + " " + creator.LastName))
		}
		out = append(out, Creator{
			Name:        name,
			CreatorType: creator.CreatorType,
		})
	}
	return out
}

func collectionParentKey(value interface{}) string {
	parent, ok := value.(string)
	if !ok {
		return ""
	}
	return parent
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mapTags(tags []apiTag) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag.Tag != "" {
			out = append(out, tag.Tag)
		}
	}
	return out
}

func mapAttachments(items []apiItem) []Attachment {
	out := make([]Attachment, 0, len(items))
	for _, item := range items {
		if item.Data.ItemType != "attachment" {
			continue
		}
		out = append(out, Attachment{
			Key:         item.Key,
			ItemType:    item.Data.ItemType,
			Title:       item.Data.Title,
			ContentType: item.Data.ContentType,
			LinkMode:    item.Data.LinkMode,
			Filename:    item.Data.Filename,
		})
	}
	return out
}
