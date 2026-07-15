package app

import (
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
	Keys      []string
	Format    string
	BatchSize int
}

const defaultExportBatchSize = 100

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
	batchSize := exportBatchSize(req.BatchSize)
	readMeta := map[string]any{"total": len(keys), "batch_size": batchSize, "batches": batchCount(len(keys), batchSize)}
	if format == "csljson" && cfg.Mode != "web" && cfg.Mode != "remote" {
		local, localErr := s.NewLocalReader(cfg)
		if localErr == nil {
			if exporter, ok := local.(localCSLExporter); ok {
				payload := make([]map[string]any, 0, len(keys))
				for index, batch := range exportKeyBatches(keys, batchSize) {
					part, exportErr := exporter.ExportItemsCSLJSON(ctx, batch)
					if exportErr != nil {
						localErr = exportBatchError(index, len(keys), batchSize, batch, exportErr)
						break
					}
					payload = append(payload, part...)
				}
				if localErr == nil {
					meta := readMetaFromReader(local)
					meta["total"] = len(keys)
					meta["batch_size"] = batchSize
					meta["batches"] = batchCount(len(keys), batchSize)
					result := zoteroapi.ExportResult{Format: "csljson", Data: payload}
					return Result{Data: result, Meta: meta, Text: jsonText(payload), Warnings: readWarnings(meta)}, nil
				}
			} else {
				localErr = backend.ErrUnsupportedFeature
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
	exported := zoteroapi.ExportResult{Format: format}
	var text strings.Builder
	var data []any
	for index, batch := range exportKeyBatches(keys, batchSize) {
		part, exportErr := client.ExportItems(ctx, batch, zoteroapi.ExportOptions{Format: format, Style: cfg.Style, Locale: cfg.Locale})
		if exportErr != nil {
			return Result{}, exportBatchError(index, len(keys), batchSize, batch, exportErr)
		}
		if format == "csljson" {
			values, convertErr := exportDataSlice(part.Data)
			if convertErr != nil {
				return Result{}, exportBatchError(index, len(keys), batchSize, batch, convertErr)
			}
			data = append(data, values...)
		} else if part.Text != "" {
			if text.Len() > 0 && !strings.HasSuffix(text.String(), "\n") {
				text.WriteByte('\n')
			}
			text.WriteString(part.Text)
		}
	}
	exported.Text = text.String()
	if format == "csljson" {
		exported.Data = data
	}
	readMeta["read_source"] = "web"
	resultText := exported.Text
	if resultText == "" {
		resultText = jsonText(exported.Data)
	}
	return Result{Data: exported, Meta: readMeta, Text: resultText}, nil
}

func (s ExportService) Stream(ctx context.Context, req ItemExportRequest, out io.Writer) error {
	if out == nil {
		return fmt.Errorf("export stream requires an output writer")
	}
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return err
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "bibtex"
	}
	if format != "bibtex" && format != "biblatex" && format != "csljson" && format != "ris" {
		return fmt.Errorf("unsupported export format %q", format)
	}
	keys := uniqueExportKeys(req.Keys)
	if len(keys) == 0 {
		return fmt.Errorf("item export requires item keys or --from")
	}
	batchSize := exportBatchSize(req.BatchSize)
	if format == "csljson" && cfg.Mode != "web" && cfg.Mode != "remote" {
		local, localErr := s.NewLocalReader(cfg)
		if localErr == nil {
			if exporter, ok := local.(localCSLExporter); ok {
				return streamLocalCSLJSON(ctx, exporter, keys, batchSize, out)
			}
			localErr = backend.ErrUnsupportedFeature
		}
		if cfg.Mode == "local" || (localErr != nil && !isExpectedLocalExportFallback(localErr)) {
			return localErr
		}
	}
	client, err := s.NewClient(cfg)
	if err != nil {
		return err
	}
	if format == "csljson" {
		return streamWebCSLJSON(ctx, client, cfg, keys, batchSize, out)
	}
	for index, batch := range exportKeyBatches(keys, batchSize) {
		part, exportErr := client.ExportItems(ctx, batch, zoteroapi.ExportOptions{Format: format, Style: cfg.Style, Locale: cfg.Locale})
		if exportErr != nil {
			return exportBatchError(index, len(keys), batchSize, batch, exportErr)
		}
		if index > 0 {
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, part.Text); err != nil {
			return err
		}
	}
	return nil
}

func ResolveExportKeys(from string, stdin io.Reader) ([]string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return nil, fmt.Errorf("--from requires a JSON file path or - for stdin")
	}
	var input io.Reader
	var closeInput io.Closer
	if from == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("--from - requires stdin")
		}
		input = stdin
	} else {
		file, err := os.Open(from)
		if err != nil {
			return nil, fmt.Errorf("read --from %q: %w", from, err)
		}
		input = file
		closeInput = file
	}
	if closeInput != nil {
		defer closeInput.Close()
	}
	keys, err := decodeExportKeys(json.NewDecoder(input))
	if err != nil {
		return nil, fmt.Errorf("parse --from %q: %w", from, err)
	}
	keys = uniqueExportKeys(keys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("--from input contains no item keys")
	}
	return keys, nil
}

func decodeExportKeys(decoder *json.Decoder) ([]string, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	return decodeExportKeysToken(decoder, token)
}

func decodeExportKeysToken(decoder *json.Decoder, token json.Token) ([]string, error) {
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '[':
			var keys []string
			for decoder.More() {
				entry, err := decodeExportKeys(decoder)
				if err != nil {
					return nil, fmt.Errorf("export input entry %d: %w", len(keys)+1, err)
				}
				keys = append(keys, entry...)
			}
			_, err := decoder.Token()
			return keys, err
		case '{':
			var keys []string
			recognized := false
			for decoder.More() {
				nameToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, _ := nameToken.(string)
				switch name {
				case "data":
					recognized = true
					entry, err := decodeExportKeys(decoder)
					if err != nil {
						return nil, err
					}
					keys = append(keys, entry...)
				case "key", "item_key":
					recognized = true
					var key string
					if err := decoder.Decode(&key); err != nil {
						return nil, fmt.Errorf("%s must be a string", name)
					}
					if strings.TrimSpace(key) != "" {
						keys = append(keys, key)
					}
				default:
					if err := skipJSONValue(decoder); err != nil {
						return nil, err
					}
				}
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			if !recognized {
				return nil, fmt.Errorf("object must contain key, item_key, or data")
			}
			return keys, nil
		}
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("item key is empty")
		}
		return []string{value}, nil
	}
	return nil, fmt.Errorf("expected a key, item object, array, or CLI JSON envelope")
}

func skipJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok || (delim != '[' && delim != '{') {
		return nil
	}
	for decoder.More() {
		if delim == '{' {
			if _, err := decoder.Token(); err != nil {
				return err
			}
		}
		if err := skipJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func exportBatchSize(value int) int {
	if value <= 0 {
		return defaultExportBatchSize
	}
	return value
}

func batchCount(total, size int) int {
	if total == 0 {
		return 0
	}
	return (total + size - 1) / size
}

func exportKeyBatches(keys []string, size int) [][]string {
	batches := make([][]string, 0, batchCount(len(keys), size))
	for start := 0; start < len(keys); start += size {
		end := start + size
		if end > len(keys) {
			end = len(keys)
		}
		batches = append(batches, keys[start:end])
	}
	return batches
}

func exportBatchError(index, total, size int, keys []string, err error) error {
	return fmt.Errorf("export batch %d/%d (%s..%s): %w", index+1, batchCount(total, size), keys[0], keys[len(keys)-1], err)
}

func exportDataSlice(value any) ([]any, error) {
	switch typed := value.(type) {
	case []any:
		return typed, nil
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, entry := range typed {
			result = append(result, entry)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("CSL JSON export returned %T instead of an array", value)
	}
}

func streamLocalCSLJSON(ctx context.Context, exporter localCSLExporter, keys []string, batchSize int, out io.Writer) error {
	if _, err := io.WriteString(out, "["); err != nil {
		return err
	}
	written := 0
	for index, batch := range exportKeyBatches(keys, batchSize) {
		part, err := exporter.ExportItemsCSLJSON(ctx, batch)
		if err != nil {
			return exportBatchError(index, len(keys), batchSize, batch, err)
		}
		for _, entry := range part {
			if written > 0 {
				if _, err := io.WriteString(out, ","); err != nil {
					return err
				}
			}
			encoded, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			if _, err := out.Write(encoded); err != nil {
				return err
			}
			written++
		}
	}
	_, err := io.WriteString(out, "]\n")
	return err
}

func streamWebCSLJSON(ctx context.Context, client exportClient, cfg config.Config, keys []string, batchSize int, out io.Writer) error {
	if _, err := io.WriteString(out, "["); err != nil {
		return err
	}
	written := 0
	for index, batch := range exportKeyBatches(keys, batchSize) {
		part, err := client.ExportItems(ctx, batch, zoteroapi.ExportOptions{Format: "csljson", Style: cfg.Style, Locale: cfg.Locale})
		if err != nil {
			return exportBatchError(index, len(keys), batchSize, batch, err)
		}
		values, err := exportDataSlice(part.Data)
		if err != nil {
			return exportBatchError(index, len(keys), batchSize, batch, err)
		}
		for _, entry := range values {
			if written > 0 {
				if _, err := io.WriteString(out, ","); err != nil {
					return err
				}
			}
			encoded, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			if _, err := out.Write(encoded); err != nil {
				return err
			}
			written++
		}
	}
	_, err := io.WriteString(out, "]\n")
	return err
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
