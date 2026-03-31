package zoteroapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"zotero_cli/internal/config"
)

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
