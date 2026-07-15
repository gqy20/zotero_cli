package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type recordingExportClient struct {
	batches [][]string
}

func (c *recordingExportClient) ExportItems(_ context.Context, keys []string, opts zoteroapi.ExportOptions) (zoteroapi.ExportResult, error) {
	c.batches = append(c.batches, append([]string(nil), keys...))
	if opts.Format == "csljson" {
		data := make([]any, 0, len(keys))
		for _, key := range keys {
			data = append(data, map[string]any{"id": key})
		}
		return zoteroapi.ExportResult{Format: opts.Format, Data: data}, nil
	}
	return zoteroapi.ExportResult{Format: opts.Format, Text: fmt.Sprintf("entries:%s", strings.Join(keys, ","))}, nil
}

func TestResolveExportKeysAcceptsFindEnvelopeFromStdin(t *testing.T) {
	keys, err := ResolveExportKeys("-", strings.NewReader(`{"ok":true,"data":[{"key":"ITEM0001"},{"key":"ITEM0002"},{"key":"ITEM0001"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "ITEM0001" || keys[1] != "ITEM0002" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestResolveExportKeysRejectsObjectsWithoutKeys(t *testing.T) {
	if _, err := ResolveExportKeys("-", strings.NewReader(`{"data":[{"title":"missing key"}]}`)); err == nil {
		t.Fatal("expected missing-key error")
	}
}

func TestExportBatchesRequestsAndMergesCSLJSON(t *testing.T) {
	client := &recordingExportClient{}
	service := ExportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{Mode: "web"}, "", nil },
		NewClient:  func(config.Config) (exportClient, error) { return client, nil },
	}
	result, err := service.Export(context.Background(), ItemExportRequest{Keys: []string{"A", "B", "C", "D", "E"}, Format: "csljson", BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.batches) != 3 || len(client.batches[0]) != 2 || len(client.batches[2]) != 1 {
		t.Fatalf("batches=%v", client.batches)
	}
	exported := result.Data.(zoteroapi.ExportResult)
	if values := exported.Data.([]any); len(values) != 5 {
		t.Fatalf("data=%v", values)
	}
}

func TestExportStreamWritesOneValidCSLJSONArray(t *testing.T) {
	client := &recordingExportClient{}
	service := ExportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{Mode: "web"}, "", nil },
		NewClient:  func(config.Config) (exportClient, error) { return client, nil },
	}
	var output bytes.Buffer
	if err := service.Stream(context.Background(), ItemExportRequest{Keys: []string{"A", "B", "C"}, Format: "csljson", BatchSize: 2}, &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "[{\"id\":\"A\"},{\"id\":\"B\"},{\"id\":\"C\"}]\n" {
		t.Fatalf("output=%q", got)
	}
}
