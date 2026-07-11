package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type ItemExportRequest struct {
	Keys       []string
	Collection string
	Find       backend.FindOptions
	Format     string
}

type exportClient interface {
	FindItems(context.Context, zoteroapi.FindOptions) ([]zoteroapi.Item, error)
	ListCollectionItems(context.Context, string, zoteroapi.FindOptions) ([]zoteroapi.Item, error)
	ExportItems(context.Context, []string, zoteroapi.ExportOptions) (zoteroapi.ExportResult, error)
}

type localCSLExporter interface {
	ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error)
}

type collectionKeyReader interface {
	CollectionItemKeys(context.Context, string, int) ([]string, error)
}

type ExportService struct {
	LoadConfig     func() (config.Config, string, error)
	NewReader      func(config.Config) (backend.Reader, error)
	NewLocalReader func(config.Config) (backend.Reader, error)
	NewClient      func(config.Config) (exportClient, error)
}

func NewExportService() ExportService {
	read := NewReadService()
	return ExportService{
		LoadConfig: config.Load,
		NewReader:  read.NewReader,
		NewLocalReader: func(cfg config.Config) (backend.Reader, error) {
			return backend.NewLocalReader(cfg)
		},
		NewClient: func(cfg config.Config) (exportClient, error) { return read.NewClient(cfg) },
	}
}

func (s ExportService) Export(ctx context.Context, req ItemExportRequest) (Result, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "bibtex"
	}
	if format != "bibtex" && format != "biblatex" && format != "csljson" && format != "ris" {
		return Result{}, fmt.Errorf("unsupported export format %q", format)
	}
	keys := append([]string(nil), req.Keys...)
	readMeta := map[string]any{}
	if len(keys) == 0 && req.Collection == "" {
		reader, err := s.NewReader(cfg)
		if err != nil {
			return Result{}, err
		}
		items, err := reader.FindItems(ctx, req.Find)
		if err != nil {
			return Result{}, err
		}
		for _, item := range filterItems(items, backend.NormalizeFindOptions(req.Find)) {
			keys = append(keys, item.Key)
		}
		readMeta = readMetaFromReader(reader)
	}
	if format == "csljson" && cfg.Mode != "web" && cfg.Mode != "remote" {
		local, localErr := s.NewLocalReader(cfg)
		if localErr == nil {
			if req.Collection != "" {
				if collectionReader, ok := local.(collectionKeyReader); ok {
					keys, localErr = collectionReader.CollectionItemKeys(ctx, req.Collection, req.Find.Limit)
				} else {
					localErr = fmt.Errorf("local export requires collection access support")
				}
			}
			if exporter, ok := local.(localCSLExporter); ok && localErr == nil {
				payload, exportErr := exporter.ExportItemsCSLJSON(ctx, keys)
				if exportErr == nil {
					meta := readMetaFromReader(local)
					meta["total"] = len(keys)
					result := zoteroapi.ExportResult{Format: "csljson", Data: payload}
					return Result{Data: result, Meta: meta, Text: jsonText(payload), Warnings: readWarnings(meta)}, nil
				}
				localErr = exportErr
			}
		}
		if cfg.Mode == "local" {
			return Result{}, localErr
		}
		if localErr != nil && !isExpectedLocalExportFallback(localErr) {
			return Result{}, localErr
		}
	}
	client, err := s.NewClient(cfg)
	if err != nil {
		return Result{}, err
	}
	if req.Collection != "" {
		items, err := client.ListCollectionItems(ctx, req.Collection, zoteroapi.FindOptions{Limit: req.Find.Limit})
		if err != nil {
			return Result{}, err
		}
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}
	if len(keys) == 0 {
		return Result{}, fmt.Errorf("no items matched export source")
	}
	exported, err := client.ExportItems(ctx, keys, zoteroapi.ExportOptions{Format: format, Style: cfg.Style, Locale: cfg.Locale})
	if err != nil {
		return Result{}, err
	}
	readMeta["total"] = len(keys)
	readMeta["read_source"] = "web"
	text := exported.Text
	if text == "" {
		text = jsonText(exported.Data)
	}
	return Result{Data: exported, Meta: readMeta, Text: text}, nil
}

func isExpectedLocalExportFallback(err error) bool {
	return errors.Is(err, backend.ErrItemNotFound) || errors.Is(err, backend.ErrUnsupportedFeature) || errors.Is(err, backend.ErrLocalTemporarilyUnavailable)
}

func readMetaFromReader(reader backend.Reader) map[string]any {
	return readMeta(reader)
}

func jsonText(value any) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded)
}
