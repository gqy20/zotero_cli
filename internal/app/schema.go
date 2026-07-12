package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

const schemaCacheTTL = 7 * 24 * time.Hour

type SchemaOptions struct {
	Refresh bool
}

type SchemaService struct {
	LoadConfig func() (config.Config, string, error)
	NewClient  func(config.Config) (*zoteroapi.Client, error)
	CacheDir   func(string) string
	Now        func() time.Time
}

func NewSchemaService() SchemaService {
	read := NewReadService()
	return SchemaService{
		LoadConfig: read.LoadConfig,
		NewClient:  read.NewClient,
		CacheDir: func(configPath string) string {
			return filepath.Join(filepath.Dir(configPath), "cache", "schema")
		},
		Now: time.Now,
	}
}

func (s SchemaService) List(ctx context.Context, category, itemType string) (Result, error) {
	return s.ListWithOptions(ctx, category, itemType, SchemaOptions{})
}

func (s SchemaService) ListWithOptions(ctx context.Context, category, itemType string, opts SchemaOptions) (Result, error) {
	if category == "types" && itemType != "" {
		return Result{}, fmt.Errorf("schema types does not accept an item type")
	}
	if category != "types" && category != "fields" && category != "roles" {
		return Result{}, fmt.Errorf("unknown schema category %q", category)
	}
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	key := strings.Join([]string{"list", cfg.Locale, category, itemType}, "|")
	var values []zoteroapi.LocalizedValue
	state, err := s.cached(ctx, configPath, key, opts.Refresh, &values, func() (any, error) {
		client, err := s.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		switch category {
		case "types":
			return client.ListItemTypes(ctx, cfg.Locale)
		case "fields":
			if itemType == "" {
				return client.ListItemFields(ctx, cfg.Locale)
			}
			return client.ListItemTypeFields(ctx, itemType, cfg.Locale)
		case "roles":
			if itemType == "" {
				return client.ListCreatorFields(ctx, cfg.Locale)
			}
			return client.ListItemTypeCreatorTypes(ctx, itemType, cfg.Locale)
		}
		return nil, fmt.Errorf("unknown schema category %q", category)
	})
	if err != nil {
		return Result{}, err
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("%-18s  %s", value.ID, value.Localized))
	}
	meta := schemaMeta(state)
	meta["total"], meta["category"], meta["item_type"] = len(values), category, itemType
	return Result{Data: values, Meta: meta, Text: strings.Join(lines, "\n"), Warnings: schemaWarnings(state)}, nil
}

func (s SchemaService) Show(ctx context.Context, itemType string) (Result, error) {
	return s.ShowWithOptions(ctx, itemType, SchemaOptions{})
}

func (s SchemaService) ShowWithOptions(ctx context.Context, itemType string, opts SchemaOptions) (Result, error) {
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	var template map[string]any
	key := strings.Join([]string{"show", itemType}, "|")
	state, err := s.cached(ctx, configPath, key, opts.Refresh, &template, func() (any, error) {
		client, err := s.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return client.GetItemTemplate(ctx, itemType)
	})
	if err != nil {
		return Result{}, err
	}
	formatted, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return Result{}, err
	}
	meta := schemaMeta(state)
	meta["item_type"] = itemType
	return Result{Data: template, Meta: meta, Text: string(formatted), Warnings: schemaWarnings(state)}, nil
}
