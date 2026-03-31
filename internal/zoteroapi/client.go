package zoteroapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"zotero_cli/internal/config"
)

const defaultBaseURL = "https://api.zotero.org"

type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        config.Config
	sleep      func(time.Duration)
}

type FindOptions struct {
	Query          string
	All            bool
	ItemType       string
	Limit          int
	Start          int
	Tag            string
	Tags           []string
	Sort           string
	Direction      string
	QMode          string
	IncludeTrashed bool
	DateAfter      string
	DateBefore     string
}

type CitationOptions struct {
	Format string
	Style  string
	Locale string
}

type Collection struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	ParentKey      string `json:"parent_key,omitempty"`
	NumCollections int    `json:"num_collections,omitempty"`
	NumItems       int    `json:"num_items,omitempty"`
}

type Note struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type Tag struct {
	Name     string `json:"name"`
	NumItems int    `json:"num_items,omitempty"`
}

type Search struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	NumConditions int    `json:"num_conditions,omitempty"`
}

type Deleted struct {
	Collections []string `json:"collections,omitempty"`
	Searches    []string `json:"searches,omitempty"`
	Items       []string `json:"items,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type VersionsOptions struct {
	ObjectType             string
	Since                  int
	IncludeTrashed         bool
	IfModifiedSinceVersion int
}

type VersionsResult struct {
	Versions            map[string]int `json:"versions"`
	LastModifiedVersion int            `json:"last_modified_version,omitempty"`
	NotModified         bool           `json:"not_modified,omitempty"`
}

type ExportOptions struct {
	Format string
	Style  string
	Locale string
}

type ExportResult struct {
	Format string `json:"format"`
	Text   string `json:"text,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type WriteResult struct {
	Key                 string `json:"key,omitempty"`
	LastModifiedVersion int    `json:"last_modified_version,omitempty"`
}

type BatchWriteResult struct {
	Successful          []WriteResult  `json:"successful,omitempty"`
	Unchanged           []string       `json:"unchanged,omitempty"`
	Failed              map[string]any `json:"failed,omitempty"`
	LastModifiedVersion int            `json:"last_modified_version,omitempty"`
}

type LocalizedValue struct {
	ID        string `json:"id"`
	Localized string `json:"localized"`
}

type KeyInfo struct {
	UserID int            `json:"user_id"`
	Access map[string]any `json:"access,omitempty"`
}

type GroupInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ValidationResult struct {
	LibraryType string `json:"library_type"`
	LibraryID   string `json:"library_id"`
	KeyUserID   int    `json:"key_user_id"`
	GroupFound  bool   `json:"group_found,omitempty"`
}

type LibraryStats struct {
	LibraryType      string `json:"library_type"`
	LibraryID        string `json:"library_id"`
	TotalItems       int    `json:"total_items"`
	TotalCollections int    `json:"total_collections"`
	TotalSearches    int    `json:"total_searches"`
}

type Item struct {
	Version     int          `json:"version,omitempty"`
	Key         string       `json:"key"`
	ItemType    string       `json:"item_type"`
	Title       string       `json:"title"`
	Date        string       `json:"date"`
	Creators    []Creator    `json:"creators"`
	Container   string       `json:"container,omitempty"`
	DOI         string       `json:"doi,omitempty"`
	URL         string       `json:"url,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Creator struct {
	Name        string `json:"name"`
	CreatorType string `json:"creator_type"`
}

type Attachment struct {
	Key         string `json:"key"`
	ItemType    string `json:"item_type"`
	Title       string `json:"title,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	LinkMode    string `json:"link_mode,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

type apiItem struct {
	Key     string      `json:"key"`
	Version int         `json:"version"`
	Data    apiItemData `json:"data"`
}

type apiItemData struct {
	ItemType         string       `json:"itemType"`
	Title            string       `json:"title"`
	Date             string       `json:"date"`
	DOI              string       `json:"DOI"`
	URL              string       `json:"url"`
	ContentType      string       `json:"contentType"`
	LinkMode         string       `json:"linkMode"`
	Filename         string       `json:"filename"`
	PublicationTitle string       `json:"publicationTitle"`
	ProceedingsTitle string       `json:"proceedingsTitle"`
	BookTitle        string       `json:"bookTitle"`
	Creators         []apiCreator `json:"creators"`
	Tags             []apiTag     `json:"tags"`
}

type apiCreator struct {
	CreatorType string `json:"creatorType"`
	Name        string `json:"name"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
}

type apiTag struct {
	Tag string `json:"tag"`
}

type apiCitationResponse struct {
	Key      string `json:"key"`
	Citation string `json:"citation"`
	Bib      string `json:"bib"`
}

type apiCollection struct {
	Key  string            `json:"key"`
	Data apiCollectionData `json:"data"`
	Meta apiCollectionMeta `json:"meta"`
}

type apiCollectionData struct {
	Name   string      `json:"name"`
	Parent interface{} `json:"parentCollection"`
}

type apiCollectionMeta struct {
	NumCollections int `json:"numCollections"`
	NumItems       int `json:"numItems"`
}

type apiNoteData struct {
	ItemType string `json:"itemType"`
	Note     string `json:"note"`
}

type apiNoteItem struct {
	Key  string      `json:"key"`
	Data apiNoteData `json:"data"`
}

type apiTagResponse struct {
	Tag  string     `json:"tag"`
	Meta apiTagMeta `json:"meta"`
}

type apiTagMeta struct {
	NumItems int `json:"numItems"`
}

type apiSearch struct {
	Key  string        `json:"key"`
	Data apiSearchData `json:"data"`
}

type apiSearchData struct {
	Name       string               `json:"name"`
	Conditions []apiSearchCondition `json:"conditions"`
}

type apiSearchCondition struct {
	Condition string `json:"condition"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}

type apiLocalizedItemType struct {
	ItemType  string `json:"itemType"`
	Localized string `json:"localized"`
}

type apiLocalizedField struct {
	Field     string `json:"field"`
	Localized string `json:"localized"`
}

type apiLocalizedCreatorType struct {
	CreatorType string `json:"creatorType"`
	Localized   string `json:"localized"`
}

type apiKeyInfo struct {
	UserID int            `json:"userID"`
	Access map[string]any `json:"access"`
}

type apiGroup struct {
	ID   int          `json:"id"`
	Data apiGroupData `json:"data"`
}

type apiGroupData struct {
	Name string `json:"name"`
}

type apiWriteResponse struct {
	Successful map[string]apiWriteSuccess `json:"successful"`
	Unchanged  map[string]int             `json:"unchanged"`
	Failed     map[string]any             `json:"failed"`
}

type apiWriteSuccess struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
}

type CitationResult struct {
	Key    string `json:"key"`
	Format string `json:"format"`
	Style  string `json:"style,omitempty"`
	Locale string `json:"locale,omitempty"`
	Text   string `json:"text"`
	HTML   string `json:"html,omitempty"`
}

type APIError struct {
	StatusCode int
	RetryAfter string
	Body       string
}

func (e *APIError) Error() string {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return "zotero api unauthorized (401): check library id and api key"
	case http.StatusNotFound:
		return "zotero api not found (404)"
	case http.StatusConflict:
		return formatAPIError("zotero api conflict (409): request conflicts with existing data", e.Body)
	case http.StatusPreconditionFailed:
		return formatAPIError("zotero api precondition failed (412): library version changed; refresh and retry", e.Body)
	case http.StatusTooManyRequests:
		if e.RetryAfter != "" {
			return fmt.Sprintf("zotero api rate limited (429): retry after %ss", e.RetryAfter)
		}
		return "zotero api rate limited (429)"
	default:
		return fmt.Sprintf("zotero api returned status %d", e.StatusCode)
	}
}

func formatAPIError(prefix string, body string) string {
	detail := strings.TrimSpace(body)
	if detail == "" {
		return prefix
	}
	return fmt.Sprintf("%s: %s", prefix, detail)
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func New(cfg config.Config, baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = defaultHTTPClient(cfg)
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		cfg:        cfg,
		sleep:      time.Sleep,
	}
}

func defaultHTTPClient(cfg config.Config) *http.Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	return &http.Client{
		Timeout: timeout,
	}
}

func (c *Client) FindItems(ctx context.Context, opts FindOptions) ([]Item, error) {
	raw, err := c.getItems(ctx, "items", opts)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(raw))
	for _, item := range raw {
		items = append(items, mapItem(item))
	}

	return items, nil
}

func (c *Client) ListTrashItems(ctx context.Context, opts FindOptions) ([]Item, error) {
	raw, err := c.getItems(ctx, path.Join("items", "trash"), opts)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(raw))
	for _, item := range raw {
		items = append(items, mapItem(item))
	}

	return items, nil
}

func (c *Client) ListPublicationsItems(ctx context.Context, opts FindOptions) ([]Item, error) {
	raw, err := c.getItems(ctx, path.Join("publications", "items"), opts)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(raw))
	for _, item := range raw {
		items = append(items, mapItem(item))
	}

	return items, nil
}

func (c *Client) GetItem(ctx context.Context, key string) (Item, error) {
	resp, err := c.doRequest(ctx, path.Join("items", key), FindOptions{}, nil)
	if err != nil {
		return Item{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Item{}, apiErrorFromResponse(resp)
	}

	var raw apiItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Item{}, err
	}

	item := mapItem(raw)

	children, err := c.getItems(ctx, path.Join("items", key, "children"), FindOptions{})
	if err != nil {
		return Item{}, err
	}
	item.Attachments = mapAttachments(children)

	return item, nil
}

func (c *Client) GetCitation(ctx context.Context, key string, opts CitationOptions) (CitationResult, error) {
	include := opts.Format
	if include == "" {
		include = "citation"
	}

	resp, err := c.doRequest(ctx, path.Join("items", key), FindOptions{}, map[string]string{
		"format":  "json",
		"include": include,
		"style":   opts.Style,
		"locale":  opts.Locale,
	})
	if err != nil {
		return CitationResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CitationResult{}, apiErrorFromResponse(resp)
	}

	var raw apiCitationResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return CitationResult{}, err
	}

	htmlValue := raw.Citation
	if include == "bib" {
		htmlValue = raw.Bib
	}

	return CitationResult{
		Key:    raw.Key,
		Format: include,
		Style:  opts.Style,
		Locale: opts.Locale,
		Text:   compactWhitespace(stripHTML(htmlValue)),
		HTML:   htmlValue,
	}, nil
}

func (c *Client) ExportItems(ctx context.Context, keys []string, opts ExportOptions) (ExportResult, error) {
	format := opts.Format
	if format == "" {
		format = "bib"
	}

	if format == "bib" {
		results := make([]CitationResult, 0, len(keys))
		for _, key := range keys {
			result, err := c.GetCitation(ctx, key, CitationOptions{
				Format: "bib",
				Style:  opts.Style,
				Locale: opts.Locale,
			})
			if err != nil {
				return ExportResult{}, err
			}
			results = append(results, result)
		}

		texts := make([]string, 0, len(results))
		for _, result := range results {
			texts = append(texts, result.Text)
		}
		return ExportResult{
			Format: "bib",
			Text:   strings.Join(texts, "\n\n"),
			Data:   results,
		}, nil
	}

	resp, err := c.doRequest(ctx, "items", FindOptions{}, map[string]string{
		"itemKey": strings.Join(keys, ","),
		"format":  format,
	})
	if err != nil {
		return ExportResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ExportResult{}, apiErrorFromResponse(resp)
	}

	if format == "csljson" {
		var payload any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return ExportResult{}, err
		}
		return ExportResult{
			Format: format,
			Data:   payload,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExportResult{}, err
	}

	return ExportResult{
		Format: format,
		Text:   string(body),
	}, nil
}

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

func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	raw, err := c.getCollections(ctx)
	if err != nil {
		return nil, err
	}

	collections := make([]Collection, 0, len(raw))
	for _, collection := range raw {
		collections = append(collections, Collection{
			Key:            collection.Key,
			Name:           collection.Data.Name,
			ParentKey:      collectionParentKey(collection.Data.Parent),
			NumCollections: collection.Meta.NumCollections,
			NumItems:       collection.Meta.NumItems,
		})
	}

	return collections, nil
}

func (c *Client) ListTopCollections(ctx context.Context) ([]Collection, error) {
	raw, err := c.fetchAllCollections(ctx, path.Join("collections", "top"))
	if err != nil {
		return nil, err
	}

	collections := make([]Collection, 0, len(raw))
	for _, collection := range raw {
		collections = append(collections, Collection{
			Key:            collection.Key,
			Name:           collection.Data.Name,
			ParentKey:      collectionParentKey(collection.Data.Parent),
			NumCollections: collection.Meta.NumCollections,
			NumItems:       collection.Meta.NumItems,
		})
	}

	return collections, nil
}

func (c *Client) ListNotes(ctx context.Context) ([]Note, error) {
	raw, err := c.getNotes(ctx)
	if err != nil {
		return nil, err
	}

	notes := make([]Note, 0, len(raw))
	for _, note := range raw {
		notes = append(notes, Note{
			Key:     note.Key,
			Content: compactWhitespace(stripHTML(note.Data.Note)),
		})
	}

	return notes, nil
}

func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	raw, err := c.getTags(ctx)
	if err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(raw))
	for _, tag := range raw {
		tags = append(tags, Tag{
			Name:     tag.Tag,
			NumItems: tag.Meta.NumItems,
		})
	}

	return tags, nil
}

func (c *Client) ListSearches(ctx context.Context) ([]Search, error) {
	raw, err := c.getSearches(ctx)
	if err != nil {
		return nil, err
	}

	searches := make([]Search, 0, len(raw))
	for _, search := range raw {
		searches = append(searches, Search{
			Key:           search.Key,
			Name:          search.Data.Name,
			NumConditions: len(search.Data.Conditions),
		})
	}

	return searches, nil
}

func (c *Client) GetDeleted(ctx context.Context) (Deleted, error) {
	resp, err := c.doRequest(ctx, "deleted", FindOptions{}, nil)
	if err != nil {
		return Deleted{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Deleted{}, apiErrorFromResponse(resp)
	}

	var deleted Deleted
	if err := json.NewDecoder(resp.Body).Decode(&deleted); err != nil {
		return Deleted{}, err
	}

	return deleted, nil
}

func (c *Client) GetVersions(ctx context.Context, opts VersionsOptions) (map[string]int, error) {
	result, err := c.GetVersionsResult(ctx, opts)
	if err != nil {
		return nil, err
	}
	return result.Versions, nil
}

func (c *Client) GetVersionsResult(ctx context.Context, opts VersionsOptions) (VersionsResult, error) {
	relativePath, err := versionsPath(opts.ObjectType)
	if err != nil {
		return VersionsResult{}, err
	}

	extra := map[string]string{
		"format": "versions",
		"since":  strconv.Itoa(opts.Since),
	}
	if opts.IncludeTrashed {
		extra["includeTrashed"] = "1"
	}

	headers := map[string]string{}
	if opts.IfModifiedSinceVersion > 0 {
		headers["If-Modified-Since-Version"] = strconv.Itoa(opts.IfModifiedSinceVersion)
	}

	resp, err := c.doRequest(ctx, relativePath, FindOptions{}, extra, headers)
	if err != nil {
		return VersionsResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return VersionsResult{
			Versions:    map[string]int{},
			NotModified: true,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return VersionsResult{}, apiErrorFromResponse(resp)
	}

	var versions map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return VersionsResult{}, err
	}

	result := VersionsResult{
		Versions: versions,
	}
	if value := resp.Header.Get("Last-Modified-Version"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			result.LastModifiedVersion = parsed
		}
	}

	return result, nil
}

func (c *Client) ListItemTypes(ctx context.Context, locale string) ([]LocalizedValue, error) {
	var raw []apiLocalizedItemType
	if err := c.doGlobalJSONRequest(ctx, "itemTypes", map[string]string{"locale": locale}, &raw); err != nil {
		return nil, err
	}

	out := make([]LocalizedValue, 0, len(raw))
	for _, value := range raw {
		out = append(out, LocalizedValue{
			ID:        value.ItemType,
			Localized: value.Localized,
		})
	}
	return out, nil
}

func (c *Client) ListItemFields(ctx context.Context, locale string) ([]LocalizedValue, error) {
	var raw []apiLocalizedField
	if err := c.doGlobalJSONRequest(ctx, "itemFields", map[string]string{"locale": locale}, &raw); err != nil {
		return nil, err
	}

	out := make([]LocalizedValue, 0, len(raw))
	for _, value := range raw {
		out = append(out, LocalizedValue{
			ID:        value.Field,
			Localized: value.Localized,
		})
	}
	return out, nil
}

func (c *Client) ListCreatorFields(ctx context.Context, locale string) ([]LocalizedValue, error) {
	var raw []apiLocalizedField
	if err := c.doGlobalJSONRequest(ctx, "creatorFields", map[string]string{"locale": locale}, &raw); err != nil {
		return nil, err
	}

	out := make([]LocalizedValue, 0, len(raw))
	for _, value := range raw {
		out = append(out, LocalizedValue{
			ID:        value.Field,
			Localized: value.Localized,
		})
	}
	return out, nil
}

func (c *Client) ListItemTypeFields(ctx context.Context, itemType string, locale string) ([]LocalizedValue, error) {
	var raw []apiLocalizedField
	if err := c.doGlobalJSONRequest(ctx, "itemTypeFields", map[string]string{
		"itemType": itemType,
		"locale":   locale,
	}, &raw); err != nil {
		return nil, err
	}

	out := make([]LocalizedValue, 0, len(raw))
	for _, value := range raw {
		out = append(out, LocalizedValue{
			ID:        value.Field,
			Localized: value.Localized,
		})
	}
	return out, nil
}

func (c *Client) ListItemTypeCreatorTypes(ctx context.Context, itemType string, locale string) ([]LocalizedValue, error) {
	var raw []apiLocalizedCreatorType
	if err := c.doGlobalJSONRequest(ctx, "itemTypeCreatorTypes", map[string]string{
		"itemType": itemType,
		"locale":   locale,
	}, &raw); err != nil {
		return nil, err
	}

	out := make([]LocalizedValue, 0, len(raw))
	for _, value := range raw {
		out = append(out, LocalizedValue{
			ID:        value.CreatorType,
			Localized: value.Localized,
		})
	}
	return out, nil
}

func (c *Client) GetItemTemplate(ctx context.Context, itemType string) (map[string]any, error) {
	var raw map[string]any
	if err := c.doGlobalJSONRequest(ctx, path.Join("items", "new"), map[string]string{
		"itemType": itemType,
	}, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) GetKeyInfo(ctx context.Context, key string) (KeyInfo, error) {
	var raw apiKeyInfo
	if err := c.doGlobalJSONRequest(ctx, path.Join("keys", key), nil, &raw); err != nil {
		return KeyInfo{}, err
	}

	return KeyInfo{
		UserID: raw.UserID,
		Access: raw.Access,
	}, nil
}

func (c *Client) ListGroupsForUser(ctx context.Context, userID string) ([]GroupInfo, error) {
	var raw []apiGroup
	if err := c.doGlobalJSONRequest(ctx, path.Join("users", userID, "groups"), nil, &raw); err != nil {
		return nil, err
	}

	out := make([]GroupInfo, 0, len(raw))
	for _, group := range raw {
		out = append(out, GroupInfo{
			ID:   group.ID,
			Name: group.Data.Name,
		})
	}
	return out, nil
}

func (c *Client) ValidateLibraryAccess(ctx context.Context) (ValidationResult, error) {
	info, err := c.GetKeyInfo(ctx, c.cfg.APIKey)
	if err != nil {
		return ValidationResult{}, err
	}

	result := ValidationResult{
		LibraryType: c.cfg.LibraryType,
		LibraryID:   c.cfg.LibraryID,
		KeyUserID:   info.UserID,
	}

	switch c.cfg.LibraryType {
	case "user":
		if strconv.Itoa(info.UserID) != c.cfg.LibraryID {
			return ValidationResult{}, fmt.Errorf("configured user library %s does not match api key owner %d", c.cfg.LibraryID, info.UserID)
		}
		return result, nil
	case "group":
		groups, err := c.ListGroupsForUser(ctx, strconv.Itoa(info.UserID))
		if err != nil {
			return ValidationResult{}, err
		}
		for _, group := range groups {
			if strconv.Itoa(group.ID) == c.cfg.LibraryID {
				result.GroupFound = true
				return result, nil
			}
		}
		return ValidationResult{}, fmt.Errorf("configured group library %s is not accessible to api key owner %d", c.cfg.LibraryID, info.UserID)
	default:
		return ValidationResult{}, fmt.Errorf("unsupported library_type %q", c.cfg.LibraryType)
	}
}

func (c *Client) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	itemVersions, err := c.GetVersionsResult(ctx, VersionsOptions{
		ObjectType: "items",
		Since:      0,
	})
	if err != nil {
		return LibraryStats{}, err
	}

	collections, err := c.ListCollections(ctx)
	if err != nil {
		return LibraryStats{}, err
	}

	searches, err := c.ListSearches(ctx)
	if err != nil {
		return LibraryStats{}, err
	}

	return LibraryStats{
		LibraryType:      c.cfg.LibraryType,
		LibraryID:        c.cfg.LibraryID,
		TotalItems:       len(itemVersions.Versions),
		TotalCollections: len(collections),
		TotalSearches:    len(searches),
	}, nil
}

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

func decodeResponseWithTotal[T any](resp *http.Response) ([]T, int, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, apiErrorFromResponse(resp)
	}

	var raw []T
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, err
	}

	total := 0
	if value := resp.Header.Get("Total-Results"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			total = parsed
		}
	}

	return raw, total, nil
}

func shouldContinuePagination(pageLen int, accumulated int, total int, requestedLimit int) bool {
	if requestedLimit > 0 {
		return false
	}
	if pageLen == 0 {
		return false
	}
	if total > 0 {
		return accumulated < total
	}
	return pageLen == 25
}

func apiErrorFromResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RetryAfter: resp.Header.Get("Retry-After"),
		Body:       strings.TrimSpace(string(body)),
	}
	if apiErr.StatusCode == 0 {
		return errors.New("zotero api request failed")
	}
	return apiErr
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

func parseLastModifiedVersion(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func versionsPath(objectType string) (string, error) {
	switch objectType {
	case "collections":
		return "collections", nil
	case "searches":
		return "searches", nil
	case "items":
		return "items", nil
	case "items-top":
		return path.Join("items", "top"), nil
	default:
		return "", fmt.Errorf("unsupported object type %q", objectType)
	}
}

func (c *Client) doGlobalJSONRequest(ctx context.Context, relativePath string, query map[string]string, target any) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, relativePath)

	values := u.Query()
	for key, value := range query {
		if value != "" {
			values.Set(key, value)
		}
	}
	u.RawQuery = values.Encode()

	resp, err := c.doHTTPRequest(ctx, http.MethodGet, u.String(), nil, true, map[string]string{
		"Zotero-API-Version": "3",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apiErrorFromResponse(resp)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) doRequest(ctx context.Context, relativePath string, opts FindOptions, extraQuery map[string]string, extraHeaders ...map[string]string) (*http.Response, error) {
	if c.cfg.Mode != "" && c.cfg.Mode != "web" {
		return nil, fmt.Errorf("unsupported mode %q", c.cfg.Mode)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	switch c.cfg.LibraryType {
	case "user":
		u.Path = path.Join(u.Path, "users", c.cfg.LibraryID, relativePath)
	case "group":
		u.Path = path.Join(u.Path, "groups", c.cfg.LibraryID, relativePath)
	default:
		return nil, fmt.Errorf("unsupported library_type %q", c.cfg.LibraryType)
	}

	if opts.Query != "" || opts.ItemType != "" || opts.Limit > 0 || opts.Start > 0 || opts.Tag != "" || opts.Sort != "" || opts.Direction != "" || opts.QMode != "" || opts.IncludeTrashed || len(extraQuery) > 0 {
		values := u.Query()
		if opts.Query != "" {
			values.Set("q", opts.Query)
		}
		if opts.ItemType != "" {
			values.Set("itemType", opts.ItemType)
		}
		if opts.Limit > 0 {
			values.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.Start > 0 {
			values.Set("start", fmt.Sprintf("%d", opts.Start))
		}
		if opts.Tag != "" {
			values.Set("tag", opts.Tag)
		}
		if opts.Sort != "" {
			values.Set("sort", opts.Sort)
		}
		if opts.Direction != "" {
			values.Set("direction", opts.Direction)
		}
		if opts.QMode != "" {
			values.Set("qmode", opts.QMode)
		}
		if opts.IncludeTrashed {
			values.Set("includeTrashed", "1")
		}
		for key, value := range extraQuery {
			if value != "" {
				values.Set(key, value)
			}
		}
		u.RawQuery = values.Encode()
	}

	headers := map[string]string{
		"Zotero-API-Key":     c.cfg.APIKey,
		"Zotero-API-Version": "3",
	}
	for _, headerSet := range extraHeaders {
		for key, value := range headerSet {
			if value != "" {
				headers[key] = value
			}
		}
	}

	resp, err := c.doHTTPRequest(ctx, http.MethodGet, u.String(), nil, true, headers)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) doWriteRequest(ctx context.Context, method string, relativePath string, body any, ifUnmodifiedSinceVersion int) (*http.Response, error) {
	if c.cfg.Mode != "" && c.cfg.Mode != "web" {
		return nil, fmt.Errorf("unsupported mode %q", c.cfg.Mode)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	switch c.cfg.LibraryType {
	case "user":
		u.Path = path.Join(u.Path, "users", c.cfg.LibraryID, relativePath)
	case "group":
		u.Path = path.Join(u.Path, "groups", c.cfg.LibraryID, relativePath)
	default:
		return nil, fmt.Errorf("unsupported library_type %q", c.cfg.LibraryType)
	}

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	headers := map[string]string{
		"Zotero-API-Key":     c.cfg.APIKey,
		"Zotero-API-Version": "3",
	}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	if ifUnmodifiedSinceVersion > 0 {
		headers["If-Unmodified-Since-Version"] = strconv.Itoa(ifUnmodifiedSinceVersion)
	}

	return c.doHTTPRequest(ctx, method, u.String(), payload, false, headers)
}

func (c *Client) doDeleteByKeysRequest(ctx context.Context, relativePath string, keys []string, ifUnmodifiedSinceVersion int) (*http.Response, error) {
	if c.cfg.Mode != "" && c.cfg.Mode != "web" {
		return nil, fmt.Errorf("unsupported mode %q", c.cfg.Mode)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	switch c.cfg.LibraryType {
	case "user":
		u.Path = path.Join(u.Path, "users", c.cfg.LibraryID, relativePath)
	case "group":
		u.Path = path.Join(u.Path, "groups", c.cfg.LibraryID, relativePath)
	default:
		return nil, fmt.Errorf("unsupported library_type %q", c.cfg.LibraryType)
	}

	values := u.Query()
	values.Set("itemKey", strings.Join(keys, ","))
	u.RawQuery = values.Encode()

	headers := map[string]string{
		"Zotero-API-Key":     c.cfg.APIKey,
		"Zotero-API-Version": "3",
	}
	if ifUnmodifiedSinceVersion > 0 {
		headers["If-Unmodified-Since-Version"] = strconv.Itoa(ifUnmodifiedSinceVersion)
	}

	return c.doHTTPRequest(ctx, http.MethodDelete, u.String(), nil, false, headers)
}

func (c *Client) doHTTPRequest(ctx context.Context, method string, rawURL string, body []byte, retryable bool, headers map[string]string) (*http.Response, error) {
	attempts := 1
	if retryable && c.cfg.RetryMaxAttempts > 1 {
		attempts = c.cfg.RetryMaxAttempts
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		var reader io.Reader
		if body != nil {
			reader = strings.NewReader(string(body))
		}

		req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			if value != "" {
				req.Header.Set(key, value)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if !retryable || attempt == attempts || !shouldRetryError(err) {
				return nil, err
			}
			c.pauseBeforeRetry(attempt, 0)
			continue
		}

		if !retryable || attempt == attempts || !shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		resp.Body.Close()
		c.pauseBeforeRetry(attempt, retryAfter)
	}

	return nil, errors.New("zotero api request failed after retries")
}

func (c *Client) pauseBeforeRetry(attempt int, retryAfter time.Duration) {
	delay := retryAfter
	if delay <= 0 {
		baseDelay := c.cfg.RetryBaseDelayMilliseconds
		if baseDelay <= 0 {
			baseDelay = 250
		}
		multiplier := math.Pow(2, float64(attempt-1))
		delay = time.Duration(float64(baseDelay)*multiplier) * time.Millisecond
	}
	if c.sleep != nil && delay > 0 {
		c.sleep(delay)
	}
}

func shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func shouldRetryError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
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
