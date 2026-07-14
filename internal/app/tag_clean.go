package app

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

type TagCleanRequest struct {
	Match    string
	MaxItems int
	Safety   SafetyOptions
}

type TagCleanCandidate struct {
	Name     string `json:"name"`
	NumItems int    `json:"num_items"`
}

type TagCleanReport struct {
	Match              string              `json:"match"`
	MaxItems           int                 `json:"max_items"`
	Candidates         []TagCleanCandidate `json:"candidates"`
	MatchedTags        int                 `json:"matched_tags"`
	MatchedAssignments int                 `json:"matched_assignments"`
	AffectedItems      int                 `json:"affected_items,omitempty"`
	UpdatedItems       int                 `json:"updated_items,omitempty"`
	VerifiedItems      int                 `json:"verified_items,omitempty"`
	Applied            bool                `json:"applied"`
	LastLibraryVersion int                 `json:"last_library_version,omitempty"`
}

func (s WriteService) CleanAutomaticTags(ctx context.Context, req TagCleanRequest) (Result, error) {
	if strings.TrimSpace(req.Match) == "" {
		return Result{}, fmt.Errorf("--match is required")
	}
	if req.MaxItems < 1 {
		return Result{}, fmt.Errorf("--max-items must be at least 1")
	}
	re, err := regexp.Compile(req.Match)
	if err != nil {
		return Result{}, fmt.Errorf("invalid --match regular expression: %w", err)
	}
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	previewClient, err := s.NewClient(cfg)
	if err != nil {
		return Result{}, err
	}
	libraryTags, err := previewClient.ListTags(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("list tags: %w", err)
	}

	candidates := make([]TagCleanCandidate, 0)
	replacements := make([]TagReplacement, 0)
	assignments := 0
	for _, tag := range libraryTags {
		if tag.Type != 1 || tag.NumItems > req.MaxItems || !re.MatchString(tag.Name) {
			continue
		}
		candidates = append(candidates, TagCleanCandidate{Name: tag.Name, NumItems: tag.NumItems})
		replacements = append(replacements, TagReplacement{From: tag.Name, NumItems: tag.NumItems})
		assignments += tag.NumItems
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].From < replacements[j].From })
	report := TagCleanReport{Match: req.Match, MaxItems: req.MaxItems, Candidates: candidates, MatchedTags: len(candidates), MatchedAssignments: assignments}
	if !req.Safety.Yes || len(candidates) == 0 {
		return tagCleanResult(report, true), nil
	}

	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	itemsByKey, err := findItemsForTagReplacements(ctx, client, replacements)
	if err != nil {
		return Result{Data: report}, err
	}
	keys := make([]string, 0, len(itemsByKey))
	for key := range itemsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report.AffectedItems = len(keys)

	matched := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		matched[candidate.Name] = struct{}{}
	}
	expected := make(map[string][]zoteroapi.ItemTag, len(keys))
	payload := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		item := itemsByKey[key]
		tags := make([]zoteroapi.ItemTag, 0, len(itemTagObjects(item)))
		changed := false
		for _, tag := range itemTagObjects(item) {
			_, selected := matched[tag.Tag]
			if selected && tag.Type != nil && *tag.Type == 1 {
				changed = true
				continue
			}
			tags = append(tags, tag)
		}
		expected[key] = tags
		if changed {
			payload = append(payload, map[string]any{"key": key, "version": item.Version, "tags": tags})
		}
	}

	batch, err := updateItemsInBatches(ctx, client, payload, version)
	report.UpdatedItems = len(batch.Successful)
	if batch.LastModifiedVersion > 0 {
		version = batch.LastModifiedVersion
	}
	if err != nil {
		return Result{Data: report}, err
	}
	verified, err := getItemsByKeysInBatches(ctx, client, keys)
	if err != nil {
		return Result{Data: report}, fmt.Errorf("verify cleaned tags: %w", err)
	}
	for _, item := range verified {
		if !equalTagSets(itemTagObjects(item), expected[item.Key]) {
			return Result{Data: report}, fmt.Errorf("verification failed for item %s", item.Key)
		}
		report.VerifiedItems++
	}
	if report.VerifiedItems != len(expected) {
		return Result{Data: report}, fmt.Errorf("verification returned %d of %d updated item(s)", report.VerifiedItems, len(expected))
	}
	report.Applied = true
	report.LastLibraryVersion = version
	return tagCleanResult(report, false), nil
}

func tagCleanResult(report TagCleanReport, preview bool) Result {
	lines := make([]string, 0, len(report.Candidates)+1)
	for _, candidate := range report.Candidates {
		lines = append(lines, fmt.Sprintf("%s  items=%d", candidate.Name, candidate.NumItems))
	}
	if len(lines) == 0 {
		lines = append(lines, "no automatic tags matched")
	}
	if preview {
		lines = append(lines, fmt.Sprintf("preview: %d automatic tag(s), %d assignment(s); add --yes to apply", report.MatchedTags, report.MatchedAssignments))
	} else {
		lines = append(lines, fmt.Sprintf("removed automatic tags and verified %d item(s)", report.VerifiedItems))
	}
	return Result{Data: report, Meta: map[string]any{"preview": preview, "matched_tags": report.MatchedTags, "matched_assignments": report.MatchedAssignments}, Text: strings.Join(lines, "\n")}
}
