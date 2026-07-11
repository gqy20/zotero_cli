package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type SchemaService struct {
	LoadConfig func() (config.Config, string, error)
	NewClient  func(config.Config) (*zoteroapi.Client, error)
}

func NewSchemaService() SchemaService {
	read := NewReadService()
	return SchemaService{LoadConfig: read.LoadConfig, NewClient: read.NewClient}
}

func (s SchemaService) client() (config.Config, *zoteroapi.Client, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	client, err := s.NewClient(cfg)
	return cfg, client, err
}

func (s SchemaService) List(ctx context.Context, category, itemType string) (Result, error) {
	cfg, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	var values []zoteroapi.LocalizedValue
	switch category {
	case "types":
		if itemType != "" {
			return Result{}, fmt.Errorf("schema types does not accept an item type")
		}
		values, err = client.ListItemTypes(ctx, cfg.Locale)
	case "fields":
		if itemType == "" {
			values, err = client.ListItemFields(ctx, cfg.Locale)
		} else {
			values, err = client.ListItemTypeFields(ctx, itemType, cfg.Locale)
		}
	case "roles":
		if itemType == "" {
			values, err = client.ListCreatorFields(ctx, cfg.Locale)
		} else {
			values, err = client.ListItemTypeCreatorTypes(ctx, itemType, cfg.Locale)
		}
	default:
		return Result{}, fmt.Errorf("unknown schema category %q", category)
	}
	if err != nil {
		return Result{}, err
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("%-18s  %s", value.ID, value.Localized))
	}
	return Result{Data: values, Meta: map[string]any{"total": len(values), "category": category, "item_type": itemType}, Text: strings.Join(lines, "\n")}, nil
}

func (s SchemaService) Show(ctx context.Context, itemType string) (Result, error) {
	_, client, err := s.client()
	if err != nil {
		return Result{}, err
	}
	template, err := client.GetItemTemplate(ctx, itemType)
	if err != nil {
		return Result{}, err
	}
	formatted, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{Data: template, Meta: map[string]any{"item_type": itemType}, Text: string(formatted)}, nil
}
