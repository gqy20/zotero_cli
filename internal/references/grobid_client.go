package references

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GrobidClient struct {
	BaseURL, CacheDir, Token string
	HTTPClient               *http.Client
	MaxAttempts              int
}

func NewGrobidClient(baseURL, cacheDir, token string, timeout time.Duration) *GrobidClient {
	return &GrobidClient{BaseURL: strings.TrimRight(baseURL, "/"), CacheDir: cacheDir, Token: token, HTTPClient: &http.Client{Timeout: timeout}, MaxAttempts: 3}
}
func (c *GrobidClient) Health(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/isalive", nil)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != "true" {
		return fmt.Errorf("GROBID health HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *GrobidClient) Process(ctx context.Context, pdfPath, cacheKey string, refresh bool) ([]byte, bool, error) {
	cache := filepath.Join(c.CacheDir, cacheKey+".tei.xml")
	if !refresh {
		if data, err := os.ReadFile(cache); err == nil {
			return data, true, nil
		}
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return nil, false, err
	}
	for attempt := 1; attempt <= c.MaxAttempts; attempt++ {
		data, status, retryAfter, err := c.processOnce(ctx, pdfPath)
		if err == nil && status == 200 {
			tmp := cache + ".tmp"
			if e := os.WriteFile(tmp, data, 0o600); e == nil {
				_ = os.Remove(cache)
				_ = os.Rename(tmp, cache)
			}
			return data, false, nil
		}
		if err == nil && status == 204 {
			return nil, false, fmt.Errorf("GROBID extracted no content")
		}
		if err != nil {
			if attempt == c.MaxAttempts {
				return nil, false, err
			}
			delay := time.Duration(attempt*5) * time.Second
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if status != 429 && status != 503 && status < 500 {
			return nil, false, fmt.Errorf("GROBID returned HTTP %d: %s", status, strings.TrimSpace(string(data)))
		}
		delay := time.Duration(attempt*5) * time.Second
		if retryAfter > 0 {
			delay = retryAfter
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, false, fmt.Errorf("GROBID request exhausted retries")
}
func (c *GrobidClient) processOnce(ctx context.Context, pdfPath string) ([]byte, int, time.Duration, error) {
	file, err := os.Open(pdfPath)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		part, e := writer.CreateFormFile("input", filepath.Base(pdfPath))
		if e == nil {
			_, e = io.Copy(part, file)
		}
		if e == nil {
			e = writer.WriteField("includeRawCitations", "1")
		}
		if e == nil {
			e = writer.WriteField("consolidateCitations", "2")
		}
		if closeErr := writer.Close(); e == nil {
			e = closeErr
		}
		_ = pw.CloseWithError(e)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/processFulltextDocument", pr)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/xml")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	delay := time.Duration(0)
	if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil {
		delay = time.Duration(n) * time.Second
	}
	return data, resp.StatusCode, delay, err
}
