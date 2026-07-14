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
	GetLibraryVersion(context.Context) (int, error)
	ListTags(context.Context) ([]zoteroapi.Tag, error)
	FindItems(context.Context, zoteroapi.FindOptions) ([]zoteroapi.Item, error)
	CreateItem(context.Context, map[string]any, int) (zoteroapi.WriteResult, error)
	UpdateItem(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error)
	DeleteItems(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
	DeleteCollections(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
	DeleteSearches(context.Context, []string, int) (zoteroapi.BatchWriteResult, error)
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
		current, err := client.GetLibraryVersion(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve current library version: %w", err)
		}
		version = current
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
		keys, err := uniqueKeys(req.Keys)
		if err != nil {
			return Result{}, err
		}
		result, err := deleteKeysInBatches(ctx, client.DeleteItems, keys, version)
		if err != nil {
			return Result{Data: result}, err
		}
		text := fmt.Sprintf("deleted %d %s objects at library version %d", len(keys), displayResource(kind), result.LastModifiedVersion)
		if len(keys) == 1 {
			text = fmt.Sprintf("deleted %s %s at library version %d", displayResource(kind), keys[0], result.LastModifiedVersion)
		}
		return Result{Data: result, Meta: map[string]any{"deleted": len(keys)}, Text: text}, nil
	}
	keys, err := uniqueKeys(req.Keys)
	if err != nil {
		return Result{}, err
	}
	if len(keys) == 1 {
		var single zoteroapi.WriteResult
		switch kind {
		case "coll":
			single, err = client.DeleteCollection(ctx, keys[0], version)
		case "search":
			single, err = client.DeleteSearch(ctx, keys[0], version)
		default:
			return Result{}, fmt.Errorf("unsupported delete resource %q", kind)
		}
		if err != nil {
			return Result{}, err
		}
		result := zoteroapi.BatchWriteResult{Successful: []zoteroapi.WriteResult{single}, LastModifiedVersion: single.LastModifiedVersion}
		return Result{Data: result, Meta: map[string]any{"deleted": 1}, Text: fmt.Sprintf("deleted %s %s at library version %d", displayResource(kind), keys[0], single.LastModifiedVersion)}, nil
	}
	var deleteBatch batchDeleteFunc
	switch kind {
	case "coll":
		deleteBatch = client.DeleteCollections
	case "search":
		deleteBatch = client.DeleteSearches
	default:
		return Result{}, fmt.Errorf("unsupported delete resource %q", kind)
	}
	result, err := deleteKeysInBatches(ctx, deleteBatch, keys, version)
	if err != nil {
		return Result{Data: result}, err
	}
	text := fmt.Sprintf("deleted %d %s objects at library version %d", len(keys), displayResource(kind), result.LastModifiedVersion)
	if len(keys) == 1 {
		text = fmt.Sprintf("deleted %s %s at library version %d", displayResource(kind), keys[0], result.LastModifiedVersion)
	}
	return Result{Data: result, Meta: map[string]any{"deleted": len(keys)}, Text: text}, nil
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
	keys, err := uniqueKeys(req.Keys)
	if err != nil {
		return Result{}, err
	}
	items, err := getItemsByKeysInBatches(ctx, client, keys)
	if err != nil {
		return Result{}, err
	}
	if missing := missingItemKeys(keys, items); len(missing) > 0 {
		return Result{}, fmt.Errorf("item keys not found: %s", formatMissingKeys(missing))
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		tags := updateItemTags(item, req.Tag, req.Add)
		payload = append(payload, map[string]any{"key": item.Key, "version": item.Version, "tags": tags})
	}
	result, err := updateItemsInBatches(ctx, client, payload, version)
	if err != nil {
		return Result{Data: result}, err
	}
	action := "added"
	if !req.Add {
		action = "removed"
	}
	return Result{Data: result, Text: fmt.Sprintf("%s tag %q on %d item(s), %d unchanged, at library version %d", action, req.Tag, len(result.Successful), len(result.Unchanged), result.LastModifiedVersion)}, nil
}

func updateItemTags(item zoteroapi.Item, target string, add bool) []zoteroapi.ItemTag {
	tags := append([]zoteroapi.ItemTag(nil), item.TagObjects...)
	if len(tags) == 0 && len(item.Tags) > 0 {
		tags = make([]zoteroapi.ItemTag, 0, len(item.Tags))
		for _, tag := range item.Tags {
			tags = append(tags, zoteroapi.ItemTag{Tag: tag})
		}
	}

	result := make([]zoteroapi.ItemTag, 0, len(tags)+1)
	found := false
	for _, tag := range tags {
		if tag.Tag == target {
			found = true
			if !add {
				continue
			}
		}
		result = append(result, tag)
	}
	if add && !found {
		result = append(result, zoteroapi.ItemTag{Tag: target})
	}
	return result
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
	keys, err := uniqueKeys(req.ItemKeys)
	if err != nil {
		return Result{}, err
	}
	items, err := getItemsByKeysInBatches(ctx, client, keys)
	if err != nil {
		return Result{}, err
	}
	if missing := missingItemKeys(keys, items); len(missing) > 0 {
		return Result{}, fmt.Errorf("item keys not found: %s", formatMissingKeys(missing))
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		collections := updateStringSet(item.Collections, req.CollectionKey, req.Add)
		payload = append(payload, map[string]any{"key": item.Key, "version": item.Version, "collections": collections})
	}
	result, err := updateItemsInBatches(ctx, client, payload, version)
	if err != nil {
		return Result{Data: result}, err
	}
	action := "added to"
	if !req.Add {
		action = "removed from"
	}
	return Result{Data: result, Text: fmt.Sprintf("%d items %s collection %s at library version %d", len(keys), action, req.CollectionKey, result.LastModifiedVersion)}, nil
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
