package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

type TagApplyOperation struct {
	Keys   []string `json:"keys"`
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

type TagApplyRequest struct {
	Operations []TagApplyOperation `json:"operations"`
	Safety     SafetyOptions       `json:"-"`
}

type TagApplyReport struct {
	Operations         int                        `json:"operations"`
	Items              int                        `json:"items"`
	AddAssignments     int                        `json:"add_assignments"`
	RemoveAssignments  int                        `json:"remove_assignments"`
	UpdatedItems       int                        `json:"updated_items"`
	UnchangedItems     int                        `json:"unchanged_items"`
	VerifiedItems      int                        `json:"verified_items"`
	LastLibraryVersion int                        `json:"last_library_version,omitempty"`
	DryRun             bool                       `json:"dry_run"`
	WriteResult        zoteroapi.BatchWriteResult `json:"write_result,omitempty"`
}

type normalizedTagChange struct {
	add       []string
	remove    []string
	addSet    map[string]struct{}
	removeSet map[string]struct{}
}

func ResolveTagApplyOperations(from string, stdin io.Reader) ([]TagApplyOperation, error) {
	if strings.TrimSpace(from) == "" {
		return nil, fmt.Errorf("--from is required")
	}
	var raw []byte
	var err error
	if from == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("--from - requires stdin")
		}
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(from)
	}
	if err != nil {
		return nil, fmt.Errorf("read --from %q: %w", from, err)
	}
	var operations []TagApplyOperation
	if err := json.Unmarshal(raw, &operations); err != nil {
		return nil, fmt.Errorf("invalid tag operation JSON: %w", err)
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("tag operation list must not be empty")
	}
	return operations, nil
}

func (s WriteService) ApplyTags(ctx context.Context, req TagApplyRequest) (Result, error) {
	keys, changes, report, err := normalizeTagApplyOperations(req.Operations)
	if err != nil {
		return Result{}, err
	}
	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	if req.Safety.DryRun {
		report.DryRun = true
		return Result{Data: report, Meta: map[string]any{"dry_run": true, "items": report.Items}, Text: fmt.Sprintf("dry run: %d item(s), %d tag addition(s), %d tag removal(s)", report.Items, report.AddAssignments, report.RemoveAssignments)}, nil
	}

	items, err := getItemsByKeysInBatches(ctx, client, keys)
	if err != nil {
		return Result{}, err
	}
	if missing := missingItemKeys(keys, items); len(missing) > 0 {
		return Result{}, fmt.Errorf("item keys not found: %s", formatMissingKeys(missing))
	}

	payload := make([]map[string]any, 0, len(items))
	expected := make(map[string][]zoteroapi.ItemTag, len(items))
	for _, item := range items {
		change := changes[item.Key]
		tags, changed := applyItemTagChanges(item, change)
		expected[item.Key] = tags
		if changed {
			payload = append(payload, map[string]any{"key": item.Key, "version": item.Version, "tags": tags})
		}
	}

	result, err := updateItemsInBatches(ctx, client, payload, version)
	report.WriteResult = result
	report.UpdatedItems = len(result.Successful)
	report.UnchangedItems = len(items) - len(payload) + len(result.Unchanged)
	if result.LastModifiedVersion > 0 {
		version = result.LastModifiedVersion
	}
	report.LastLibraryVersion = version
	if err != nil {
		return Result{Data: report}, err
	}

	verified, err := getItemsByKeysInBatches(ctx, client, keys)
	if err != nil {
		return Result{Data: report}, fmt.Errorf("verify applied tags: %w", err)
	}
	for _, item := range verified {
		if !equalTagSets(itemTagObjects(item), expected[item.Key]) {
			return Result{Data: report}, fmt.Errorf("verification failed for item %s", item.Key)
		}
		report.VerifiedItems++
	}
	if report.VerifiedItems != len(expected) {
		return Result{Data: report}, fmt.Errorf("verification returned %d of %d item(s)", report.VerifiedItems, len(expected))
	}

	return Result{Data: report, Meta: map[string]any{"items": report.Items, "updated": report.UpdatedItems, "verified": report.VerifiedItems}, Text: fmt.Sprintf("updated %d item(s), %d unchanged; verified %d item(s) at library version %d", report.UpdatedItems, report.UnchangedItems, report.VerifiedItems, report.LastLibraryVersion)}, nil
}

func normalizeTagApplyOperations(operations []TagApplyOperation) ([]string, map[string]*normalizedTagChange, TagApplyReport, error) {
	report := TagApplyReport{Operations: len(operations)}
	changes := make(map[string]*normalizedTagChange)
	keys := make([]string, 0)
	for index, operation := range operations {
		operationKeys, err := uniqueKeys(operation.Keys)
		if err != nil || len(operationKeys) == 0 {
			return nil, nil, report, fmt.Errorf("operation %d requires non-empty keys", index)
		}
		if len(operation.Add) == 0 && len(operation.Remove) == 0 {
			return nil, nil, report, fmt.Errorf("operation %d requires add or remove tags", index)
		}
		for _, key := range operationKeys {
			change, exists := changes[key]
			if !exists {
				change = &normalizedTagChange{addSet: map[string]struct{}{}, removeSet: map[string]struct{}{}}
				changes[key] = change
				keys = append(keys, key)
			}
			for _, tag := range operation.Add {
				if err := change.addTag(strings.TrimSpace(tag), true); err != nil {
					return nil, nil, report, fmt.Errorf("operation %d item %s: %w", index, key, err)
				}
			}
			for _, tag := range operation.Remove {
				if err := change.addTag(strings.TrimSpace(tag), false); err != nil {
					return nil, nil, report, fmt.Errorf("operation %d item %s: %w", index, key, err)
				}
			}
		}
	}
	report.Items = len(keys)
	for _, change := range changes {
		report.AddAssignments += len(change.add)
		report.RemoveAssignments += len(change.remove)
	}
	return keys, changes, report, nil
}

func (c *normalizedTagChange) addTag(tag string, add bool) error {
	if tag == "" {
		return fmt.Errorf("tag names must not be empty")
	}
	if add {
		if _, conflict := c.removeSet[tag]; conflict {
			return fmt.Errorf("tag %q cannot be both added and removed", tag)
		}
		if _, exists := c.addSet[tag]; !exists {
			c.addSet[tag] = struct{}{}
			c.add = append(c.add, tag)
		}
		return nil
	}
	if _, conflict := c.addSet[tag]; conflict {
		return fmt.Errorf("tag %q cannot be both added and removed", tag)
	}
	if _, exists := c.removeSet[tag]; !exists {
		c.removeSet[tag] = struct{}{}
		c.remove = append(c.remove, tag)
	}
	return nil
}

func applyItemTagChanges(item zoteroapi.Item, change *normalizedTagChange) ([]zoteroapi.ItemTag, bool) {
	original := itemTagObjects(item)
	result := make([]zoteroapi.ItemTag, 0, len(original)+len(change.add))
	present := make(map[string]struct{}, len(original)+len(change.add))
	changed := false
	for _, tag := range original {
		if _, remove := change.removeSet[tag.Tag]; remove {
			changed = true
			continue
		}
		if _, duplicate := present[tag.Tag]; duplicate {
			changed = true
			continue
		}
		present[tag.Tag] = struct{}{}
		result = append(result, tag)
	}
	for _, tag := range change.add {
		if _, exists := present[tag]; exists {
			continue
		}
		present[tag] = struct{}{}
		result = append(result, zoteroapi.ItemTag{Tag: tag})
		changed = true
	}
	return result, changed
}
