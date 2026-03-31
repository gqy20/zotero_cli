package zoteroapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
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
}

type FindOptions struct {
	Query     string
	ItemType  string
	Limit     int
	Start     int
	Tag       string
	Sort      string
	Direction string
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

type LocalizedValue struct {
	ID        string `json:"id"`
	Localized string `json:"localized"`
}

type Item struct {
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
	Key  string      `json:"key"`
	Data apiItemData `json:"data"`
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
	case http.StatusTooManyRequests:
		if e.RetryAfter != "" {
			return fmt.Sprintf("zotero api rate limited (429): retry after %ss", e.RetryAfter)
		}
		return "zotero api rate limited (429)"
	default:
		return fmt.Sprintf("zotero api returned status %d", e.StatusCode)
	}
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Zotero-API-Version", "3")

	resp, err := c.httpClient.Do(req)
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

	if opts.Query != "" || opts.ItemType != "" || opts.Limit > 0 || opts.Start > 0 || opts.Tag != "" || opts.Sort != "" || opts.Direction != "" || len(extraQuery) > 0 {
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
		for key, value := range extraQuery {
			if value != "" {
				values.Set(key, value)
			}
		}
		u.RawQuery = values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Zotero-API-Key", c.cfg.APIKey)
	req.Header.Set("Zotero-API-Version", "3")
	for _, headerSet := range extraHeaders {
		for key, value := range headerSet {
			if value != "" {
				req.Header.Set(key, value)
			}
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
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
