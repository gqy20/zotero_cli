package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type ItemExportRequest struct {
	Keys   []string
	Format string
}

type exportClient interface {
	ExportItems(context.Context, []string, zoteroapi.ExportOptions) (zoteroapi.ExportResult, error)
}

type localCSLExporter interface {
	ExportItemsCSLJSON(context.Context, []string) ([]map[string]any, error)
}

type ExportService struct {
	LoadConfig     func() (config.Config, string, error)
	NewLocalReader func(config.Config) (backend.Reader, error)
	NewClient      func(config.Config) (exportClient, error)
}

func NewExportService() ExportService {
	read := NewReadService()
	return ExportService{
		LoadConfig: config.Load,
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
	keys := uniqueExportKeys(req.Keys)
	if len(keys) == 0 {
		return Result{}, fmt.Errorf("item export requires item keys or --from")
	}
	readMeta := map[string]any{}
	if format == "csljson" && cfg.Mode != "web" && cfg.Mode != "remote" {
		local, localErr := s.NewLocalReader(cfg)
		if localErr == nil {
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

func ResolveExportKeys(from string, stdin io.Reader) ([]string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return nil, fmt.Errorf("--from requires a JSON file path or - for stdin")
	}
	var content []byte
	var err error
	if from == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("--from - requires stdin")
		}
		content, err = io.ReadAll(bufio.NewReader(stdin))
	} else {
		content, err = os.ReadFile(from)
	}
	if err != nil {
		return nil, fmt.Errorf("read --from %q: %w", from, err)
	}
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return nil, fmt.Errorf("parse --from %q: %w", from, err)
	}
	keys, err := exportKeysFromJSON(value)
	if err != nil {
		return nil, err
	}
	keys = uniqueExportKeys(keys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("--from input contains no item keys")
	}
	return keys, nil
}

func exportKeysFromJSON(value any) ([]string, error) {
	switch typed := value.(type) {
	case []any:
		keys := make([]string, 0, len(typed))
		for index, entry := range typed {
			entryKeys, err := exportKeysFromJSON(entry)
			if err != nil {
				return nil, fmt.Errorf("export input entry %d: %w", index+1, err)
			}
			keys = append(keys, entryKeys...)
		}
		return keys, nil
	case map[string]any:
		if data, ok := typed["data"]; ok {
			return exportKeysFromJSON(data)
		}
		if key, ok := typed["key"].(string); ok && strings.TrimSpace(key) != "" {
			return []string{key}, nil
		}
		if key, ok := typed["item_key"].(string); ok && strings.TrimSpace(key) != "" {
			return []string{key}, nil
		}
		return nil, fmt.Errorf("object must contain key, item_key, or data")
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, fmt.Errorf("item key is empty")
		}
		return []string{typed}, nil
	default:
		return nil, fmt.Errorf("expected a key, item object, array, or CLI JSON envelope")
	}
}

func uniqueExportKeys(keys []string) []string {
	result := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized := strings.ToUpper(key)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, key)
	}
	return result
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
