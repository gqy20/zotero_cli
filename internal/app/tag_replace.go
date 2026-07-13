package app

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

const tagReplaceBatchSize = 50

type TagReplaceRequest struct {
	Match   string        `json:"match"`
	Replace string        `json:"replace"`
	Safety  SafetyOptions `json:"-"`
}

type TagReplacement struct {
	From     string `json:"from"`
	To       string `json:"to"`
	NumItems int    `json:"num_items"`
}

type TagReplaceReport struct {
	Match              string           `json:"match"`
	Replace            string           `json:"replace"`
	Replacements       []TagReplacement `json:"replacements"`
	MatchedTags        int              `json:"matched_tags"`
	MatchedAssignments int              `json:"matched_assignments"`
	AffectedItems      int              `json:"affected_items,omitempty"`
	UpdatedItems       int              `json:"updated_items,omitempty"`
	VerifiedItems      int              `json:"verified_items,omitempty"`
	Applied            bool             `json:"applied"`
	LastLibraryVersion int              `json:"last_library_version,omitempty"`
}

func (s WriteService) ReplaceTags(ctx context.Context, req TagReplaceRequest) (Result, error) {
	if strings.TrimSpace(req.Match) == "" {
		return Result{}, fmt.Errorf("--match is required")
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

	replacements := make([]TagReplacement, 0)
	assignments := 0
	for _, tag := range libraryTags {
		if !re.MatchString(tag.Name) {
			continue
		}
		to := re.ReplaceAllString(tag.Name, req.Replace)
		if to == "" {
			return Result{}, fmt.Errorf("replacement turns tag %q into an empty tag", tag.Name)
		}
		if to == tag.Name {
			continue
		}
		replacements = append(replacements, TagReplacement{From: tag.Name, To: to, NumItems: tag.NumItems})
		assignments += tag.NumItems
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].From < replacements[j].From })
	report := TagReplaceReport{
		Match: req.Match, Replace: req.Replace, Replacements: replacements,
		MatchedTags: len(replacements), MatchedAssignments: assignments,
	}
	if !req.Safety.Yes || len(replacements) == 0 {
		return tagReplaceResult(report, true), nil
	}

	_, client, version, err := s.open(ctx, false, req.Safety)
	if err != nil {
		return Result{}, err
	}
	itemsByKey := make(map[string]zoteroapi.Item)
	for _, replacement := range replacements {
		items, err := client.FindItems(ctx, zoteroapi.FindOptions{Tag: replacement.From})
		if err != nil {
			return Result{Data: report}, fmt.Errorf("find items tagged %q: %w", replacement.From, err)
		}
		for _, item := range items {
			if itemHasExactTag(item, replacement.From) {
				itemsByKey[item.Key] = item
			}
		}
	}

	keys := make([]string, 0, len(itemsByKey))
	for key := range itemsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report.AffectedItems = len(keys)
	expected := make(map[string][]zoteroapi.ItemTag, len(keys))
	payload := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		item := itemsByKey[key]
		tags, changed, err := replaceItemTags(item, re, req.Replace)
		if err != nil {
			return Result{Data: report}, fmt.Errorf("item %s: %w", key, err)
		}
		if !changed {
			continue
		}
		expected[key] = tags
		payload = append(payload, map[string]any{"key": key, "version": item.Version, "tags": tags})
	}

	for start := 0; start < len(payload); start += tagReplaceBatchSize {
		end := min(start+tagReplaceBatchSize, len(payload))
		batch, err := client.UpdateItems(ctx, payload[start:end], version)
		if err != nil {
			return Result{Data: report}, err
		}
		if len(batch.Failed) > 0 {
			return Result{Data: report}, fmt.Errorf("replace tags failed for %d item(s)", len(batch.Failed))
		}
		report.UpdatedItems += len(batch.Successful)
		if batch.LastModifiedVersion > 0 {
			version = batch.LastModifiedVersion
		}
	}

	verifyKeys := make([]string, 0, len(expected))
	for key := range expected {
		verifyKeys = append(verifyKeys, key)
	}
	sort.Strings(verifyKeys)
	for start := 0; start < len(verifyKeys); start += tagReplaceBatchSize {
		end := min(start+tagReplaceBatchSize, len(verifyKeys))
		items, err := client.GetItemsByKeys(ctx, verifyKeys[start:end])
		if err != nil {
			return Result{Data: report}, fmt.Errorf("verify replaced tags: %w", err)
		}
		for _, item := range items {
			want, ok := expected[item.Key]
			if !ok || !equalTagSets(itemTagObjects(item), want) {
				return Result{Data: report}, fmt.Errorf("verification failed for item %s", item.Key)
			}
			report.VerifiedItems++
		}
	}
	if report.VerifiedItems != len(expected) {
		return Result{Data: report}, fmt.Errorf("verification returned %d of %d updated item(s)", report.VerifiedItems, len(expected))
	}

	report.Applied = true
	report.LastLibraryVersion = version
	return tagReplaceResult(report, false), nil
}

func replaceItemTags(item zoteroapi.Item, re *regexp.Regexp, replacement string) ([]zoteroapi.ItemTag, bool, error) {
	original := itemTagObjects(item)
	result := make([]zoteroapi.ItemTag, 0, len(original))
	seen := make(map[string]struct{}, len(original))
	changed := false
	for _, tag := range original {
		updated := tag
		if re.MatchString(tag.Tag) {
			updated.Tag = re.ReplaceAllString(tag.Tag, replacement)
			if updated.Tag == "" {
				return nil, false, fmt.Errorf("replacement turns tag %q into an empty tag", tag.Tag)
			}
			if updated.Tag != tag.Tag {
				updated.Type = nil
				changed = true
			}
		}
		if _, exists := seen[updated.Tag]; exists {
			changed = true
			continue
		}
		seen[updated.Tag] = struct{}{}
		result = append(result, updated)
	}
	return result, changed, nil
}

func itemTagObjects(item zoteroapi.Item) []zoteroapi.ItemTag {
	if len(item.TagObjects) > 0 {
		return append([]zoteroapi.ItemTag(nil), item.TagObjects...)
	}
	tags := make([]zoteroapi.ItemTag, 0, len(item.Tags))
	for _, tag := range item.Tags {
		tags = append(tags, zoteroapi.ItemTag{Tag: tag})
	}
	return tags
}

func itemHasExactTag(item zoteroapi.Item, target string) bool {
	for _, tag := range itemTagObjects(item) {
		if tag.Tag == target {
			return true
		}
	}
	return false
}

func equalTagSets(left, right []zoteroapi.ItemTag) bool {
	if len(left) != len(right) {
		return false
	}
	normalize := func(tags []zoteroapi.ItemTag) []string {
		values := make([]string, 0, len(tags))
		for _, tag := range tags {
			tagType := 0
			if tag.Type != nil {
				tagType = *tag.Type
			}
			values = append(values, fmt.Sprintf("%s\x00%d", tag.Tag, tagType))
		}
		sort.Strings(values)
		return values
	}
	return strings.Join(normalize(left), "\x01") == strings.Join(normalize(right), "\x01")
}

func tagReplaceResult(report TagReplaceReport, preview bool) Result {
	lines := make([]string, 0, len(report.Replacements)+2)
	for _, replacement := range report.Replacements {
		lines = append(lines, fmt.Sprintf("%s -> %s  items=%d", replacement.From, replacement.To, replacement.NumItems))
	}
	if len(lines) == 0 {
		lines = append(lines, "no tags matched with a changed result")
	}
	if preview {
		lines = append(lines, fmt.Sprintf("preview: %d tag(s), %d tag assignment(s); add --yes to apply", report.MatchedTags, report.MatchedAssignments))
	} else {
		lines = append(lines, fmt.Sprintf("updated and verified %d item(s)", report.VerifiedItems))
	}
	meta := map[string]any{"preview": preview, "matched_tags": report.MatchedTags, "matched_assignments": report.MatchedAssignments}
	return Result{Data: report, Meta: meta, Text: strings.Join(lines, "\n")}
}
