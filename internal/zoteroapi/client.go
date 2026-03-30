package zoteroapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"zotero_cli/internal/config"
)

const defaultBaseURL = "https://api.zotero.org"

type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        config.Config
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

func (c *Client) FindItems(ctx context.Context, query string) ([]Item, error) {
	raw, err := c.getItems(ctx, "items", query)
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
	resp, err := c.doRequest(ctx, path.Join("items", key), "")
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

	children, err := c.getItems(ctx, path.Join("items", key, "children"), "")
	if err != nil {
		return Item{}, err
	}
	item.Attachments = mapAttachments(children)

	return item, nil
}

func (c *Client) getItems(ctx context.Context, relativePath, query string) ([]apiItem, error) {
	resp, err := c.doRequest(ctx, relativePath, query)
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

func (c *Client) doRequest(ctx context.Context, relativePath, query string) (*http.Response, error) {
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

	if query != "" {
		values := u.Query()
		values.Set("q", query)
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
