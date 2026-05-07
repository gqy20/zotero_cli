package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"zotero_cli/internal/domain"
)

type RemoteReader struct {
	baseURL          string
	httpClient       *http.Client
	lastReadMetadata ReadMetadata
}

func NewRemoteReader(baseURL string, httpClient *http.Client) *RemoteReader {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RemoteReader{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (r *RemoteReader) markRead() {
	r.lastReadMetadata = ReadMetadata{ReadSource: "remote"}
}

func (r *RemoteReader) ConsumeReadMetadata() ReadMetadata {
	meta := r.lastReadMetadata
	r.lastReadMetadata = ReadMetadata{}
	return meta
}

func (r *RemoteReader) buildURL(path string) string {
	return r.baseURL + path
}

func (r *RemoteReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items"), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = EncodeFindOptions(opts).Encode()

	var resp apiResponse[[]domain.Item]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+key), nil)
	if err != nil {
		return domain.Item{}, err
	}
	var resp apiResponse[domain.Item]
	if err := r.doJSON(req, &resp); err != nil {
		return domain.Item{}, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+key+"/related"), nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]domain.Relation]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/stats"), nil)
	if err != nil {
		return LibraryStats{}, err
	}
	var resp apiResponse[LibraryStats]
	if err := r.doJSON(req, &resp); err != nil {
		return LibraryStats{}, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ListTags(ctx context.Context) ([]Tag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/tags"), nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Tag]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ListCollections(ctx context.Context) ([]Collection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/collections"), nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Collection]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/notes"), nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]domain.Note]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) GetAttachmentFile(ctx context.Context, key string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/files/"+key), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("remote server returned %d for file %s", resp.StatusCode, key)
	}

	filename := key
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		if _, params, err := mime.ParseMediaType(disp); err == nil {
			if n, ok := params["filename"]; ok {
				filename = n
			}
		}
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("zot-remote-%s-*%s", key, ext))
	if err != nil {
		return "", "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", "", fmt.Errorf("download file: %w", err)
	}
	tmpFile.Close()

	contentType := resp.Header.Get("Content-Type")
	r.markRead()
	return tmpFile.Name(), contentType, nil
}

type apiResponse[T any] struct {
	Ok    bool   `json:"ok"`
	Data  T      `json:"data"`
	Error string `json:"error"`
	Meta  struct {
		Total int `json:"total"`
	} `json:"meta"`
}

func (r *RemoteReader) doJSON(req *http.Request, result any) error {
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp apiResponse[any]
		json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Error
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return fmt.Errorf("remote error: %s", msg)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// EncodeFindOptions converts FindOptions to URL query parameters for HTTP transport.
func EncodeFindOptions(opts FindOptions) url.Values {
	q := url.Values{}
	setString(q, "q", opts.Query)
	setString(q, "item_type", opts.ItemType)
	setString(q, "tag", opts.Tag)
	setString(q, "sort", opts.Sort)
	setString(q, "direction", opts.Direction)
	setString(q, "qmode", opts.QMode)
	setString(q, "date_after", opts.DateAfter)
	setString(q, "date_before", opts.DateBefore)
	setString(q, "attachment_name", opts.AttachmentName)
	setString(q, "attachment_path", opts.AttachmentPath)
	setString(q, "attachment_type", opts.AttachmentType)
	setString(q, "exclude_item_type", opts.ExcludeItemType)
	setString(q, "date_modified_after", opts.DateModifiedAfter)
	setString(q, "date_added_after", opts.DateAddedAfter)
	setBool(q, "full_text", opts.FullText)
	setBool(q, "full_text_any", opts.FullTextAny)
	setBool(q, "all", opts.All)
	setBool(q, "full", opts.Full)
	setBool(q, "tag_any", opts.TagAny)
	setBool(q, "include_trashed", opts.IncludeTrashed)
	setBool(q, "has_pdf", opts.HasPDF)
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Start > 0 {
		q.Set("start", strconv.Itoa(opts.Start))
	}
	setCommaList(q, "tags", opts.Tags)
	setCommaList(q, "include_fields", opts.IncludeFields)
	setCommaList(q, "collection", opts.Collection)
	setCommaList(q, "no_collection", opts.NoCollection)
	setCommaList(q, "tag_contains", opts.TagContains)
	setCommaList(q, "exclude_tags", opts.ExcludeTags)
	return q
}

// ParseFindOptionsFromQuery reconstructs FindOptions from URL query parameters.
func ParseFindOptionsFromQuery(q url.Values) FindOptions {
	opts := FindOptions{
		Limit: 25,
		Start: 0,
	}
	opts.Query = q.Get("q")
	opts.ItemType = q.Get("item_type")
	opts.Tag = q.Get("tag")
	opts.Sort = q.Get("sort")
	opts.Direction = q.Get("direction")
	opts.QMode = q.Get("qmode")
	opts.DateAfter = q.Get("date_after")
	opts.DateBefore = q.Get("date_before")
	opts.AttachmentName = q.Get("attachment_name")
	opts.AttachmentPath = q.Get("attachment_path")
	opts.AttachmentType = q.Get("attachment_type")
	opts.ExcludeItemType = q.Get("exclude_item_type")
	opts.DateModifiedAfter = q.Get("date_modified_after")
	opts.DateAddedAfter = q.Get("date_added_after")
	opts.FullText = q.Get("full_text") == "true"
	opts.FullTextAny = q.Get("full_text_any") == "true"
	opts.All = q.Get("all") == "true"
	opts.Full = q.Get("full") == "true"
	opts.TagAny = q.Get("tag_any") == "true"
	opts.IncludeTrashed = q.Get("include_trashed") == "true"
	opts.HasPDF = q.Get("has_pdf") == "true"
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := q.Get("start"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.Start = n
		}
	}
	opts.Tags = parseCommaList(q.Get("tags"))
	opts.IncludeFields = parseCommaList(q.Get("include_fields"))
	opts.Collection = parseCommaList(q.Get("collection"))
	opts.NoCollection = parseCommaList(q.Get("no_collection"))
	opts.TagContains = parseCommaList(q.Get("tag_contains"))
	opts.ExcludeTags = parseCommaList(q.Get("exclude_tags"))
	return opts
}

func setString(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setBool(q url.Values, key string, value bool) {
	if value {
		q.Set(key, "true")
	}
}

func setCommaList(q url.Values, key string, values []string) {
	if len(values) > 0 {
		q.Set(key, strings.Join(values, ","))
	}
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
