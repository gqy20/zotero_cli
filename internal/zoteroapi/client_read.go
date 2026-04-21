package zoteroapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
)

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

func (c *Client) ListCollectionItems(ctx context.Context, key string, opts FindOptions) ([]Item, error) {
	raw, err := c.getItems(ctx, path.Join("collections", key, "items"), opts)
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

	extra := map[string]string{
		"itemKey": strings.Join(keys, ","),
		"format":  format,
	}
	if format == "bib" {
		if opts.Style != "" {
			extra["style"] = opts.Style
		}
		if opts.Locale != "" {
			extra["locale"] = opts.Locale
		}
	}

	resp, err := c.doRequest(ctx, "items", FindOptions{}, extra)
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

func (c *Client) GetCurrentKeyInfo(ctx context.Context) (KeyInfo, error) {
	var raw apiKeyInfo
	if err := c.doGlobalJSONRequest(ctx, path.Join("keys", "current"), nil, &raw); err != nil {
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
	info, err := c.GetCurrentKeyInfo(ctx)
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
		LibraryType:        c.cfg.LibraryType,
		LibraryID:          c.cfg.LibraryID,
		TotalItems:         len(itemVersions.Versions),
		TotalCollections:   len(collections),
		TotalSearches:      len(searches),
		LastLibraryVersion: itemVersions.LastModifiedVersion,
	}, nil
}
