package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

var ErrCancelled = errors.New("operation cancelled")

type PayloadInput struct {
	Data string
	From string
	Set  []string
}

type ObjectWriteRequest struct {
	Keys    []string
	Payload map[string]any
	Safety  SafetyOptions
}

type TagWriteRequest struct {
	Keys   []string
	Tag    string
	Add    bool
	Safety SafetyOptions
}

type MembershipRequest struct {
	CollectionKey string
	ItemKeys      []string
	Add           bool
	Safety        SafetyOptions
}

type WriteClient interface {
	GetLibraryStats(context.Context) (zoteroapi.LibraryStats, error)
	CreateItem(context.Context, map[string]any, int) (zoteroapi.WriteResult, error)
	UpdateItem(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error)
	DeleteItems(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
	GetItemsByKeys(context.Context, []string) ([]zoteroapi.Item, error)
	UpdateItems(context.Context, []map[string]any, int) (zoteroapi.BatchWriteResult, error)
	CreateCollection(context.Context, map[string]any, int) (zoteroapi.WriteResult, error)
	UpdateCollection(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error)
	DeleteCollection(context.Context, string, int) (zoteroapi.WriteResult, error)
	CreateSearch(context.Context, map[string]any, int) (zoteroapi.WriteResult, error)
	UpdateSearch(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error)
	DeleteSearch(context.Context, string, int) (zoteroapi.WriteResult, error)
}

type WriteService struct {
	LoadConfig func() (config.Config, string, error)
	NewClient  func(config.Config) (WriteClient, error)
}

func NewWriteService() WriteService {
	read := NewReadService()
	return WriteService{
		LoadConfig: config.Load,
		NewClient: func(cfg config.Config) (WriteClient, error) {
			return read.NewClient(cfg)
		},
	}
}

func ResolvePayload(input PayloadInput, stdin io.Reader) (map[string]any, error) {
	selected := 0
	if strings.TrimSpace(input.Data) != "" {
		selected++
	}
	if strings.TrimSpace(input.From) != "" {
		selected++
	}
	if len(input.Set) > 0 {
		selected++
	}
	if selected == 0 {
		return nil, fmt.Errorf("one of --data, --from, or --set is required")
	}
	if selected > 1 {
		return nil, fmt.Errorf("--data, --from, and --set are mutually exclusive")
	}

	if len(input.Set) > 0 {
		payload := make(map[string]any, len(input.Set))
		for _, assignment := range input.Set {
			name, value, ok := strings.Cut(assignment, "=")
			name = strings.TrimSpace(name)
			if !ok || name == "" {
				return nil, fmt.Errorf("invalid --set %q; expected FIELD=VALUE", assignment)
			}
			payload[name] = parseScalar(value)
		}
		return payload, nil
	}

	raw := input.Data
	if input.From != "" {
		var content []byte
		var err error
		if input.From == "-" {
			if stdin == nil {
				return nil, fmt.Errorf("--from - requires stdin")
			}
			content, err = io.ReadAll(bufio.NewReader(stdin))
		} else {
			content, err = os.ReadFile(input.From)
		}
		if err != nil {
			return nil, fmt.Errorf("read --from %q: %w", input.From, err)
		}
		raw = string(content)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	return payload, nil
}

func parseScalar(value string) any {
	value = strings.TrimSpace(value)
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if value == "null" {
		return nil
	}
	if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		return number
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number
	}
	return value
}

func (s WriteService) open(ctx context.Context, delete bool, safety SafetyOptions) (config.Config, WriteClient, int, error) {
	cfg, err := s.authorize(delete)
	if err != nil {
		return config.Config{}, nil, 0, err
	}
	if safety.DryRun {
		if safety.IfVersion < 0 {
			return config.Config{}, nil, 0, fmt.Errorf("--if-version must be non-negative")
		}
		return cfg, nil, safety.IfVersion, nil
	}
	client, version, err := s.clientAndVersion(ctx, cfg, safety)
	return cfg, client, version, err
}

func (s WriteService) authorize(delete bool) (config.Config, error) {
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return config.Config{}, err
	}
	if delete {
		if !cfg.AllowDelete {
			return config.Config{}, fmt.Errorf("delete operations are disabled; set ZOT_ALLOW_DELETE=1")
		}
	} else if !cfg.AllowWrite {
		return config.Config{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
	}
	return cfg, nil
}

func (s WriteService) clientAndVersion(ctx context.Context, cfg config.Config, safety SafetyOptions) (WriteClient, int, error) {
	client, err := s.NewClient(cfg)
	if err != nil {
		return nil, 0, err
	}
	version := safety.IfVersion
	if version < 0 {
		return nil, 0, fmt.Errorf("--if-version must be non-negative")
	}
	if version == 0 {
		stats, err := client.GetLibraryStats(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve current library version: %w", err)
		}
		version = stats.LastLibraryVersion
		if version <= 0 {
			return nil, 0, fmt.Errorf("library did not provide a usable current version; pass --if-version")
		}
	}
	return client, version, nil
}

func confirmDelete(kind string, keys []string, safety SafetyOptions) error {
	if safety.DryRun || safety.Yes {
		return nil
	}
	if safety.Confirm == nil || !safety.Confirm(fmt.Sprintf("delete %s %s", kind, strings.Join(keys, ", "))) {
		return ErrCancelled
	}
	return nil
}

func dryRunResult(action string, data any, version int) Result {
	return Result{Data: data, Meta: map[string]any{"dry_run": true, "if_version": version}, Text: "dry run: " + action}
}

func (s WriteService) Create(ctx context.Context, kind string, req ObjectWriteRequest) (Result, error) {
	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		return dryRunResult("create "+kind, req.Payload, version), nil
	}
	var result zoteroapi.WriteResult
	switch kind {
	case "item", "note":
		result, err = client.CreateItem(ctx, req.Payload, version)
	case "coll":
		result, err = client.CreateCollection(ctx, req.Payload, version)
	case "search":
		result, err = client.CreateSearch(ctx, req.Payload, version)
	default:
		return Result{}, fmt.Errorf("unsupported create resource %q", kind)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Data: result, Meta: map[string]any{"write_source": "web"}, Text: fmt.Sprintf("created %s %s at library version %d", displayResource(kind), result.Key, result.LastModifiedVersion)}, nil
}

func (s WriteService) Update(ctx context.Context, kind string, req ObjectWriteRequest) (Result, error) {
	if len(req.Keys) != 1 {
		return Result{}, fmt.Errorf("%s edit requires exactly one key", kind)
	}
	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		return dryRunResult("edit "+kind+" "+req.Keys[0], req.Payload, version), nil
	}
	var result zoteroapi.WriteResult
	switch kind {
	case "item", "note":
		result, err = client.UpdateItem(ctx, req.Keys[0], req.Payload, version)
	case "coll":
		result, err = client.UpdateCollection(ctx, req.Keys[0], req.Payload, version)
	case "search":
		result, err = client.UpdateSearch(ctx, req.Keys[0], req.Payload, version)
	default:
		return Result{}, fmt.Errorf("unsupported edit resource %q", kind)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Data: result, Meta: map[string]any{"write_source": "web"}, Text: fmt.Sprintf("updated %s %s at library version %d", displayResource(kind), result.Key, result.LastModifiedVersion)}, nil
}

func (s WriteService) Delete(ctx context.Context, kind string, req ObjectWriteRequest) (Result, error) {
	if len(req.Keys) == 0 {
		return Result{}, fmt.Errorf("%s delete requires at least one key", kind)
	}
	cfg, err := s.authorize(true)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		return dryRunResult("delete "+kind+" "+strings.Join(req.Keys, ", "), req.Keys, req.Safety.IfVersion), nil
	}
	if err := confirmDelete(displayResource(kind), req.Keys, req.Safety); err != nil {
		return Result{}, err
	}
	client, version, err := s.clientAndVersion(ctx, cfg, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if kind == "item" || kind == "note" {
		result, err := client.DeleteItems(ctx, req.Keys, version)
		if err != nil {
			return Result{}, err
		}
		text := fmt.Sprintf("deleted %d %s objects at library version %d", len(req.Keys), displayResource(kind), result.LastModifiedVersion)
		if len(req.Keys) == 1 {
			text = fmt.Sprintf("deleted %s %s at library version %d", displayResource(kind), req.Keys[0], result.LastModifiedVersion)
		}
		return Result{Data: result, Meta: map[string]any{"deleted": len(req.Keys)}, Text: text}, nil
	}
	results := make([]zoteroapi.WriteResult, 0, len(req.Keys))
	for _, key := range req.Keys {
		var result zoteroapi.WriteResult
		switch kind {
		case "coll":
			result, err = client.DeleteCollection(ctx, key, version)
		case "search":
			result, err = client.DeleteSearch(ctx, key, version)
		default:
			return Result{}, fmt.Errorf("unsupported delete resource %q", kind)
		}
		if err != nil {
			return Result{}, err
		}
		results = append(results, result)
		if result.LastModifiedVersion > 0 {
			version = result.LastModifiedVersion
		}
	}
	text := fmt.Sprintf("deleted %d %s objects", len(results), displayResource(kind))
	if len(results) == 1 {
		text = fmt.Sprintf("deleted %s %s at library version %d", displayResource(kind), results[0].Key, results[0].LastModifiedVersion)
	}
	return Result{Data: results, Meta: map[string]any{"deleted": len(results)}, Text: text}, nil
}

func displayResource(kind string) string {
	if kind == "coll" {
		return "collection"
	}
	return kind
}

func (s WriteService) Tags(ctx context.Context, req TagWriteRequest) (Result, error) {
	if len(req.Keys) == 0 || strings.TrimSpace(req.Tag) == "" {
		return Result{}, fmt.Errorf("item keys and --tag are required")
	}
	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		return dryRunResult("update item tags", req, version), nil
	}
	items, err := client.GetItemsByKeys(ctx, req.Keys)
	if err != nil {
		return Result{}, err
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		tags := updateStringSet(item.Tags, req.Tag, req.Add)
		apiTags := make([]map[string]string, 0, len(tags))
		for _, tag := range tags {
			apiTags = append(apiTags, map[string]string{"tag": tag})
		}
		payload = append(payload, map[string]any{"key": item.Key, "version": item.Version, "tags": apiTags})
	}
	result, err := client.UpdateItems(ctx, payload, version)
	if err != nil {
		return Result{}, err
	}
	action := "added"
	if !req.Add {
		action = "removed"
	}
	return Result{Data: result, Text: fmt.Sprintf("%s tag %q on %d items at library version %d", action, req.Tag, len(req.Keys), result.LastModifiedVersion)}, nil
}

func (s WriteService) Membership(ctx context.Context, req MembershipRequest) (Result, error) {
	if req.CollectionKey == "" || len(req.ItemKeys) == 0 {
		return Result{}, fmt.Errorf("collection key and item keys are required")
	}
	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		return dryRunResult("update collection membership", req, version), nil
	}
	items, err := client.GetItemsByKeys(ctx, req.ItemKeys)
	if err != nil {
		return Result{}, err
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		collections := updateStringSet(item.Collections, req.CollectionKey, req.Add)
		payload = append(payload, map[string]any{"key": item.Key, "version": item.Version, "collections": collections})
	}
	result, err := client.UpdateItems(ctx, payload, version)
	if err != nil {
		return Result{}, err
	}
	action := "added to"
	if !req.Add {
		action = "removed from"
	}
	return Result{Data: result, Text: fmt.Sprintf("%d items %s collection %s at library version %d", len(req.ItemKeys), action, req.CollectionKey, result.LastModifiedVersion)}, nil
}

func updateStringSet(values []string, target string, add bool) []string {
	result := make([]string, 0, len(values)+1)
	found := false
	for _, value := range values {
		if value == target {
			found = true
			if !add {
				continue
			}
		}
		result = append(result, value)
	}
	if add && !found {
		result = append(result, target)
	}
	return result
}
