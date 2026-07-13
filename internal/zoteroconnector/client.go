package zoteroconnector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultBaseURL = "http://127.0.0.1:23119"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type ImportPDFRequest struct {
	SessionID     string
	Title         string
	SourceURL     string
	Content       io.Reader
	ContentLength int64
}

type ImportPDFResult struct {
	CanRecognize bool `json:"can_recognize"`
}

type UpdateSessionRequest struct {
	SessionID string
	Target    string
}

type RecognizedItem struct {
	Title    string `json:"title"`
	ItemType string `json:"itemType"`
}

func New(baseURL string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/connector/ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect to Zotero desktop: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return connectorError("ping Zotero desktop", resp)
	}
	return nil
}

func (c *Client) ImportPDF(ctx context.Context, input ImportPDFRequest) (ImportPDFResult, error) {
	metadata, err := json.Marshal(map[string]string{
		"sessionID": input.SessionID,
		"title":     input.Title,
		"url":       input.SourceURL,
	})
	if err != nil {
		return ImportPDFResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/connector/saveStandaloneAttachment", input.Content)
	if err != nil {
		return ImportPDFResult{}, err
	}
	req.ContentLength = input.ContentLength
	req.Header.Set("Content-Type", "application/pdf")
	req.Header.Set("X-Metadata", string(metadata))
	req.Header.Set("X-Zotero-Connector-API-Version", "3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ImportPDFResult{}, fmt.Errorf("submit PDF to Zotero desktop: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return ImportPDFResult{}, connectorError("import PDF into Zotero desktop", resp)
	}

	var raw struct {
		CanRecognize bool `json:"canRecognize"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ImportPDFResult{}, fmt.Errorf("decode Zotero import response: %w", err)
	}
	return ImportPDFResult{CanRecognize: raw.CanRecognize}, nil
}

func (c *Client) UpdateSession(ctx context.Context, input UpdateSessionRequest) error {
	body, err := json.Marshal(map[string]string{"sessionID": input.SessionID, "target": input.Target})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/connector/updateSession", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zotero-Connector-API-Version", "3")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("set Zotero import collection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return connectorError("set Zotero import collection", resp)
	}
	return nil
}

func (c *Client) WaitForRecognizedItem(ctx context.Context, sessionID string) (RecognizedItem, bool, error) {
	body, err := json.Marshal(map[string]string{"sessionID": sessionID})
	if err != nil {
		return RecognizedItem{}, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/connector/getRecognizedItem", strings.NewReader(string(body)))
	if err != nil {
		return RecognizedItem{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zotero-Connector-API-Version", "3")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RecognizedItem{}, false, fmt.Errorf("wait for Zotero metadata recognition: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return RecognizedItem{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return RecognizedItem{}, false, connectorError("wait for Zotero metadata recognition", resp)
	}
	var item RecognizedItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return RecognizedItem{}, false, fmt.Errorf("decode Zotero recognition response: %w", err)
	}
	return item, true, nil
}

func connectorError(action string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", action, resp.StatusCode, message)
}
