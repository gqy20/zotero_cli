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
	options []zoteroapi.ExportOptions
}

func (c *recordingExportClient) ExportItems(_ context.Context, keys []string, opts zoteroapi.ExportOptions) (zoteroapi.ExportResult, error) {
	c.batches = append(c.batches, append([]string(nil), keys...))
	c.options = append(c.options, opts)
	if opts.Format == "csljson" {
		data := make([]any, 0, len(keys))
		for _, key := range keys {
			data = append(data, map[string]any{"id": key})
		}
		return zoteroapi.ExportResult{Format: opts.Format, Data: data}, nil
	}
	if opts.Format == "bib" {
		return zoteroapi.ExportResult{Format: opts.Format, Text: "1. First\n2. Second", HTML: "<div class=\"csl-bib-body\">raw</div>"}, nil
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

func TestExportBibliographyUsesOneRequestAndStyleOverride(t *testing.T) {
	client := &recordingExportClient{}
	service := ExportService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "web", Style: "apa", Locale: "en-US"}, "", nil
		},
		NewClient: func(config.Config) (exportClient, error) { return client, nil },
	}
	result, err := service.Export(context.Background(), ItemExportRequest{
		Keys:      []string{"A", "B"},
		Format:    "bibliography",
		Style:     "nature",
		BatchSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.batches) != 1 || len(client.batches[0]) != 2 {
		t.Fatalf("bibliography must use one request, batches=%v", client.batches)
	}
	if got := client.options[0]; got.Format != "bib" || got.Style != "nature" || got.Locale != "en-US" {
		t.Fatalf("options=%+v", got)
	}
	exported := result.Data.(zoteroapi.ExportResult)
	if exported.Format != "bibliography" || exported.Style != "nature" || exported.Text != "1. First\n2. Second" {
		t.Fatalf("exported=%+v", exported)
	}
	if result.Meta["batches"] != 1 || result.Meta["style"] != "nature" {
		t.Fatalf("meta=%v", result.Meta)
	}
}

func TestStreamBibliographyWritesRawHTML(t *testing.T) {
	client := &recordingExportClient{}
	service := ExportService{
		LoadConfig: func() (config.Config, string, error) {
			return config.Config{Mode: "web", Style: "nature", Locale: "en-US"}, "", nil
		},
		NewClient: func(config.Config) (exportClient, error) { return client, nil },
	}
	var output bytes.Buffer
	if err := service.Stream(context.Background(), ItemExportRequest{Keys: []string{"A", "B"}, Format: "bibliography"}, &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "<div class=\"csl-bib-body\">raw</div>" {
		t.Fatalf("output=%q", got)
	}
}

func TestExportBibliographyRejectsLocalModeAndOversizedSets(t *testing.T) {
	local := ExportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{Mode: "local"}, "", nil },
	}
	if _, err := local.Export(context.Background(), ItemExportRequest{Keys: []string{"A"}, Format: "bibliography"}); err == nil || !strings.Contains(err.Error(), "requires Zotero Web API") {
		t.Fatalf("expected local-mode error, got %v", err)
	}

	keys := make([]string, maxBibliographyItems+1)
	for index := range keys {
		keys[index] = fmt.Sprintf("K%03d", index)
	}
	web := ExportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{Mode: "web"}, "", nil },
	}
	if _, err := web.Export(context.Background(), ItemExportRequest{Keys: keys, Format: "bibliography"}); err == nil || !strings.Contains(err.Error(), "at most 100 items") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}
