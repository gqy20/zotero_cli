package zoteroapi

import (
	"fmt"
	"net/http"
	"regexp"
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
	Full           bool
	ItemType       string
	Limit          int
	Start          int
	Tag            string
	Tags           []string
	TagAny         bool
	IncludeFields  []string
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
	LibraryType         string `json:"library_type"`
	LibraryID           string `json:"library_id"`
	TotalItems          int    `json:"total_items"`
	TotalCollections    int    `json:"total_collections"`
	TotalSearches       int    `json:"total_searches"`
	LastLibraryVersion  int    `json:"last_library_version,omitempty"`
}

type Item struct {
	Version     int          `json:"version,omitempty"`
	Key         string       `json:"key"`
	ItemType    string       `json:"item_type"`
	Title       string       `json:"title"`
	Date        string       `json:"date"`
	Creators    []Creator    `json:"creators"`
	Container   string       `json:"container,omitempty"`
	Volume      string       `json:"volume,omitempty"`
	Issue       string       `json:"issue,omitempty"`
	Pages       string       `json:"pages,omitempty"`
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
	Volume           string       `json:"volume"`
	Issue            string       `json:"issue"`
	Pages            string       `json:"pages"`
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
	case http.StatusForbidden:
		detail := strings.TrimSpace(strings.ToLower(e.Body))
		if strings.Contains(detail, "invalid key") {
			return formatAPIError("zotero api forbidden (403): invalid api key; update ZOT_API_KEY", e.Body)
		}
		return formatAPIError("zotero api forbidden (403)", e.Body)
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
