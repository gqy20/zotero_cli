package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ClientConfig struct {
	BaseURL     string
	APIKey      string
	Email       string
	Tool        string
	CacheDir    string
	MinInterval time.Duration
	MaxAttempts int
	HTTPClient  *http.Client
}

type Client struct {
	cfg          ClientConfig
	mu           sync.Mutex
	lastRequest  time.Time
	cacheHits    atomic.Int64
	networkCalls atomic.Int64
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
	}
	if cfg.Tool == "" {
		cfg.Tool = "zotero_cli"
	}
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = 400 * time.Millisecond
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{cfg: cfg}
}

func (c *Client) Stats() (cacheHits, networkCalls int) {
	return int(c.cacheHits.Load()), int(c.networkCalls.Load())
}

func (c *Client) ResolveDOI(ctx context.Context, doi string, refresh bool) (string, error) {
	values := url.Values{"db": {"pubmed"}, "term": {fmt.Sprintf("%q[AID]", normalizeDOI(doi))}, "retmode": {"json"}, "retmax": {"2"}}
	data, err := c.get(ctx, "esearch.fcgi", values, refresh)
	if err != nil {
		return "", err
	}
	var response struct {
		Search struct {
			IDs []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("decode NCBI DOI search: %w", err)
	}
	if len(response.Search.IDs) == 0 {
		return "", nil
	}
	return response.Search.IDs[0], nil
}

func (c *Client) FetchPubMedArticle(ctx context.Context, pmid string, refresh bool) (pubmedRecord, error) {
	records, err := c.fetchPubMedRecords(ctx, []string{pmid}, refresh)
	if err != nil {
		return pubmedRecord{}, err
	}
	if len(records) == 0 {
		return pubmedRecord{}, fmt.Errorf("PMID %s not found", pmid)
	}
	return records[0], nil
}

func (c *Client) FetchPubMedReferences(ctx context.Context, pmid string, refresh bool) ([]Reference, error) {
	values := url.Values{"dbfrom": {"pubmed"}, "db": {"pubmed"}, "id": {pmid}, "linkname": {"pubmed_pubmed_refs"}, "retmode": {"json"}}
	data, err := c.get(ctx, "elink.fcgi", values, refresh)
	if err != nil {
		return nil, err
	}
	var response struct {
		LinkSets []struct {
			DBs []struct {
				Links []string `json:"links"`
			} `json:"linksetdbs"`
		} `json:"linksets"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode NCBI reference links: %w", err)
	}
	if len(response.LinkSets) == 0 || len(response.LinkSets[0].DBs) == 0 {
		return []Reference{}, nil
	}
	ids := response.LinkSets[0].DBs[0].Links
	records, err := c.fetchPubMedRecords(ctx, ids, refresh)
	if err != nil {
		return nil, err
	}
	refs := make([]Reference, 0, len(records))
	for i, record := range records {
		refs = append(refs, record.reference(i+1, SourcePubMed))
	}
	return refs, nil
}

func (c *Client) FetchPMCReferences(ctx context.Context, pmcid string, refresh bool) ([]Reference, error) {
	values := url.Values{"db": {"pmc"}, "id": {normalizePMCID(pmcid)}, "retmode": {"xml"}}
	data, err := c.get(ctx, "efetch.fcgi", values, refresh)
	if err != nil {
		return nil, err
	}
	return parseJATSReferences(data)
}

func (c *Client) fetchPubMedRecords(ctx context.Context, ids []string, refresh bool) ([]pubmedRecord, error) {
	if len(ids) == 0 {
		return []pubmedRecord{}, nil
	}
	byPMID := make(map[string]pubmedRecord, len(ids))
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		values := url.Values{"db": {"pubmed"}, "id": {strings.Join(ids[start:end], ",")}, "retmode": {"xml"}}
		data, err := c.get(ctx, "efetch.fcgi", values, refresh)
		if err != nil {
			return nil, err
		}
		var set pubmedArticleSet
		if err := xml.Unmarshal(data, &set); err != nil {
			return nil, fmt.Errorf("decode PubMed XML: %w", err)
		}
		for _, article := range set.Articles {
			record := article.record()
			byPMID[record.PMID] = record
		}
	}
	ordered := make([]pubmedRecord, 0, len(ids))
	for _, id := range ids {
		if record, ok := byPMID[id]; ok {
			ordered = append(ordered, record)
		}
	}
	return ordered, nil
}

func (c *Client) get(ctx context.Context, endpoint string, values url.Values, refresh bool) ([]byte, error) {
	values.Set("tool", c.cfg.Tool)
	if c.cfg.Email != "" {
		values.Set("email", c.cfg.Email)
	}
	if c.cfg.APIKey != "" {
		values.Set("api_key", c.cfg.APIKey)
	}
	rawURL := strings.TrimRight(c.cfg.BaseURL, "/") + "/" + endpoint + "?" + values.Encode()
	cachePath := c.cachePath(endpoint, values)
	if !refresh && cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil {
			c.cacheHits.Add(1)
			return data, nil
		}
	}

	var lastErr error
	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		c.wait(ctx)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "zotero-cli/0.1 (NCBI references)")
		resp, err := c.cfg.HTTPClient.Do(req)
		c.networkCalls.Add(1)
		if err != nil {
			lastErr = err
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
			resp.Body.Close()
			if readErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if cachePath != "" {
					_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
					_ = os.WriteFile(cachePath, data, 0o600)
				}
				return data, nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("NCBI %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(data)))
				if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
					return nil, lastErr
				}
			}
		}
		if attempt < c.cfg.MaxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * 500 * time.Millisecond):
			}
		}
	}
	return nil, lastErr
}

func (c *Client) wait(ctx context.Context) {
	c.mu.Lock()
	now := time.Now()
	scheduled := now
	if next := c.lastRequest.Add(c.cfg.MinInterval); next.After(scheduled) {
		scheduled = next
	}
	c.lastRequest = scheduled
	c.mu.Unlock()
	if wait := time.Until(scheduled); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	}
}

func (c *Client) cachePath(endpoint string, values url.Values) string {
	if c.cfg.CacheDir == "" {
		return ""
	}
	copyValues := url.Values{}
	for key, vals := range values {
		if key != "api_key" && key != "email" && key != "tool" {
			copyValues[key] = append([]string(nil), vals...)
		}
	}
	hash := sha256.Sum256([]byte(endpoint + "?" + copyValues.Encode()))
	ext := ".json"
	if values.Get("retmode") == "xml" {
		ext = ".xml"
	}
	return filepath.Join(c.cfg.CacheDir, strings.TrimSuffix(endpoint, ".fcgi"), hex.EncodeToString(hash[:])+ext)
}

func parseInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}
