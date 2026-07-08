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
	authKey          string
	httpClient       *http.Client
	lastReadMetadata ReadMetadata
}

func NewRemoteReader(baseURL string, httpClient *http.Client) *RemoteReader {
	return NewRemoteReaderWithAuth(baseURL, "", httpClient)
}

func NewRemoteReaderWithAuth(baseURL, authKey string, httpClient *http.Client) *RemoteReader {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RemoteReader{baseURL: strings.TrimRight(baseURL, "/"), authKey: authKey, httpClient: httpClient}
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

func (r *RemoteReader) setAuth(req *http.Request) {
	if r.authKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.authKey)
	}
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

func (r *RemoteReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+item.Key+"/preview"), nil)
	if err != nil {
		return "", err
	}
	var resp apiResponse[string]
	if err := r.doJSON(req, &resp); err != nil {
		return "", err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+item.Key+"/snippet"), nil)
	if err != nil {
		return "", err
	}
	if query != "" {
		values := url.Values{}
		values.Set("q", query)
		req.URL.RawQuery = values.Encode()
	}
	var resp apiResponse[string]
	if err := r.doJSON(req, &resp); err != nil {
		return "", err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ExtractItemFullText(ctx context.Context, item domain.Item) (string, error) {
	result, err := r.ExtractItemAttachmentTexts(ctx, item)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (r *RemoteReader) ExtractItemAttachmentTexts(ctx context.Context, item domain.Item) (ItemFullTextResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+item.Key+"/text"), nil)
	if err != nil {
		return ItemFullTextResult{}, err
	}
	var resp apiResponse[ItemFullTextResult]
	if err := r.doJSON(req, &resp); err != nil {
		return ItemFullTextResult{}, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ExtractItemAttachmentPageTexts(ctx context.Context, item domain.Item) (ItemPageTextResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+item.Key+"/text"), nil)
	if err != nil {
		return ItemPageTextResult{}, err
	}
	values := url.Values{}
	values.Set("pages", "true")
	req.URL.RawQuery = values.Encode()
	var resp apiResponse[ItemPageTextResult]
	if err := r.doJSON(req, &resp); err != nil {
		return ItemPageTextResult{}, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) ReadItemAnnotations(ctx context.Context, item domain.Item) (ItemAnnotationsResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+item.Key+"/annotations"), nil)
	if err != nil {
		return ItemAnnotationsResult{}, err
	}
	var resp apiResponse[ItemAnnotationsResult]
	if err := r.doJSON(req, &resp); err != nil {
		return ItemAnnotationsResult{}, err
	}
	r.markRead()
	return resp.Data, nil
}

func (r *RemoteReader) AnnotateItem(ctx context.Context, item domain.Item, reqBody AnnotateRequest) (AnnotateResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return AnnotateResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.buildURL("/api/v1/items/"+item.Key+"/annotate"), strings.NewReader(string(body)))
	if err != nil {
		return AnnotateResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var resp apiResponse[AnnotateResult]
	if err := r.doJSON(req, &resp); err != nil {
		return AnnotateResult{}, err
	}
	r.markRead()
	resp.Data.PDFPath = ""
	return resp.Data, nil
}

func (r *RemoteReader) ClearItemAnnotations(ctx context.Context, item domain.Item, reqBody DeleteAnnotationsRequest) (ItemAnnotationClearResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return ItemAnnotationClearResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.buildURL("/api/v1/items/"+item.Key+"/annotations/clear"), strings.NewReader(string(body)))
	if err != nil {
		return ItemAnnotationClearResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var resp apiResponse[ItemAnnotationClearResult]
	if err := r.doJSON(req, &resp); err != nil {
		return ItemAnnotationClearResult{}, err
	}
	r.markRead()
	resp.Data.PDFPath = ""
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
	r.setAuth(req)
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
	r.setAuth(req)
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

// ExtractFigures calls the remote server to extract figures for an item,
// then downloads each figure PNG to outputDir/{attachmentKey}/.
func (r *RemoteReader) ExtractFigures(ctx context.Context, itemKey string, outputDir string) (ExtractFiguresResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.buildURL("/api/v1/items/"+itemKey+"/figures"), nil)
	if err != nil {
		return ExtractFiguresResult{}, err
	}

	var resp apiResponse[ExtractFiguresResult]
	if err := r.doJSON(req, &resp); err != nil {
		return ExtractFiguresResult{}, err
	}
	r.markRead()

	result := resp.Data
	var errs []string

	for _, fig := range result.Figures {
		if fig.AttachmentKey == "" || fig.File == "" {
			continue
		}

		attDir := filepath.Join(outputDir, fig.AttachmentKey)
		if err := os.MkdirAll(attDir, 0o755); err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: mkdir: %v", fig.AttachmentKey, fig.File, err))
			continue
		}

		figURL := r.buildURL("/api/v1/figures/" + fig.AttachmentKey + "/" + fig.File)
		dlReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, figURL, nil)
		r.setAuth(dlReq)
		dlResp, err := r.httpClient.Do(dlReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: download: %v", fig.AttachmentKey, fig.File, err))
			continue
		}

		if dlResp.StatusCode != http.StatusOK {
			dlResp.Body.Close()
			errs = append(errs, fmt.Sprintf("%s/%s: HTTP %d", fig.AttachmentKey, fig.File, dlResp.StatusCode))
			continue
		}

		dstPath := filepath.Join(attDir, fig.File)
		f, err := os.Create(dstPath)
		if err != nil {
			dlResp.Body.Close()
			errs = append(errs, fmt.Sprintf("%s/%s: create: %v", fig.AttachmentKey, fig.File, err))
			continue
		}
		_, copyErr := io.Copy(f, dlResp.Body)
		f.Close()
		dlResp.Body.Close()
		if copyErr != nil {
			os.Remove(dstPath)
			errs = append(errs, fmt.Sprintf("%s/%s: write: %v", fig.AttachmentKey, fig.File, copyErr))
		}
	}

	if len(errs) > 0 {
		result.Error = strings.Join(errs, "; ")
	}
	return result, nil
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
