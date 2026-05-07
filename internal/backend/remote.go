package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"zotero_cli/internal/domain"
)

type RemoteReader struct {
	baseURL    string
	httpClient *http.Client
}

func NewRemoteReader(baseURL string, httpClient *http.Client) *RemoteReader {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RemoteReader{baseURL: baseURL, httpClient: httpClient}
}

type apiResponse[T any] struct {
	Ok    bool   `json:"ok"`
	Data  T      `json:"data"`
	Error string `json:"error"`
	Meta  struct {
		Total int `json:"total"`
	} `json:"meta"`
}

func (r *RemoteReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/items", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	if opts.Query != "" {
		q.Set("q", opts.Query)
	}
	if opts.ItemType != "" {
		q.Set("item_type", opts.ItemType)
	}
	if opts.Tag != "" {
		q.Set("tag", opts.Tag)
	}
	if len(opts.Tags) > 0 {
		q.Set("tags", joinStrings(opts.Tags, ","))
	}
	if len(opts.Collection) > 0 {
		q.Set("collection", opts.Collection[0])
	}
	if opts.DateAfter != "" {
		q.Set("date_after", opts.DateAfter)
	}
	if opts.DateBefore != "" {
		q.Set("date_before", opts.DateBefore)
	}
	if opts.HasPDF {
		q.Set("has_pdf", "true")
	}
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", opts.Start))
	}
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	if opts.Direction != "" {
		q.Set("direction", opts.Direction)
	}
	if opts.Full {
		q.Set("full", "true")
	}
	req.URL.RawQuery = q.Encode()

	var resp apiResponse[[]domain.Item]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/items/"+key, nil)
	if err != nil {
		return domain.Item{}, err
	}
	var resp apiResponse[domain.Item]
	if err := r.doJSON(req, &resp); err != nil {
		return domain.Item{}, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/items/"+key+"/related", nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]domain.Relation]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/stats", nil)
	if err != nil {
		return LibraryStats{}, err
	}
	var resp apiResponse[LibraryStats]
	if err := r.doJSON(req, &resp); err != nil {
		return LibraryStats{}, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) ListTags(ctx context.Context) ([]Tag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/tags", nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Tag]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) ListCollections(ctx context.Context) ([]Collection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/collections", nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Collection]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/notes", nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]domain.Note]
	if err := r.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *RemoteReader) GetAttachmentFile(ctx context.Context, key string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/files/"+key, nil)
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
		filename = extractFilename(disp)
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
	return tmpFile.Name(), contentType, nil
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

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func extractFilename(disp string) string {
	for _, prefix := range []string{`filename="`, "filename="} {
		idx := indexOf(disp, prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		rest := disp[start:]
		if prefix[len(prefix)-1] == '"' {
			if end := indexOf(rest, `"`); end >= 0 {
				return rest[:end]
			}
		}
		return rest
	}
	return ""
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
