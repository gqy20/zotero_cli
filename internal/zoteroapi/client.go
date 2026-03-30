package zoteroapi

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"zotero_cli/internal/config"
)

const defaultBaseURL = "https://api.zotero.org"

type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        config.Config
}

type FindOptions struct {
	Query    string
	ItemType string
	Limit    int
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

type CitationResult struct {
	Key    string `json:"key"`
	Format string `json:"format"`
	Style  string `json:"style,omitempty"`
	Locale string `json:"locale,omitempty"`
	Text   string `json:"text"`
	HTML   string `json:"html,omitempty"`
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func New(cfg config.Config, baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		cfg:        cfg,
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
		return Item{}, fmt.Errorf("zotero api returned status %d", resp.StatusCode)
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
		return CitationResult{}, fmt.Errorf("zotero api returned status %d", resp.StatusCode)
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
	resp, err := c.doRequest(ctx, "collections", FindOptions{}, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zotero api returned status %d", resp.StatusCode)
	}

	var raw []apiCollection
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
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
	resp, err := c.doRequest(ctx, "items", FindOptions{ItemType: "note"}, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zotero api returned status %d", resp.StatusCode)
	}

	var raw []apiNoteItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
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

func (c *Client) getItems(ctx context.Context, relativePath string, opts FindOptions) ([]apiItem, error) {
	resp, err := c.doRequest(ctx, relativePath, opts, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zotero api returned status %d", resp.StatusCode)
	}

	var raw []apiItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return raw, nil
}

func (c *Client) doRequest(ctx context.Context, relativePath string, opts FindOptions, extraQuery map[string]string) (*http.Response, error) {
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

	if opts.Query != "" || opts.ItemType != "" || opts.Limit > 0 || len(extraQuery) > 0 {
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
