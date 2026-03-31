package cli

import (
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

func filterDefaultFindItems(items []domain.Item, opts backend.FindOptions) []domain.Item {
	filtered := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if !shouldIncludeFindItem(item.ItemType, item.Tags, item.Date, opts.ItemType, opts.Tags, opts.TagAny, opts.DateAfter, opts.DateBefore) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func matchesTags(itemTags []string, required []string, anyMode bool) bool {
	if len(required) == 0 {
		return true
	}

	tagSet := make(map[string]struct{}, len(itemTags))
	for _, tag := range itemTags {
		normalized := strings.TrimSpace(strings.ToLower(tag))
		if normalized != "" {
			tagSet[normalized] = struct{}{}
		}
	}

	for _, tag := range required {
		normalized := strings.TrimSpace(strings.ToLower(tag))
		if normalized == "" {
			continue
		}
		_, ok := tagSet[normalized]
		if anyMode && ok {
			return true
		}
		if !anyMode && !ok {
			return false
		}
	}
	return !anyMode
}

func matchesDateRange(itemDate string, after string, before string) bool {
	if after == "" && before == "" {
		return true
	}

	itemStart, itemEnd, ok := parseDateRange(itemDate)
	if !ok {
		return false
	}

	if after != "" {
		afterStart, _, ok := parseDateRange(after)
		if !ok || itemStart.Before(afterStart) {
			return false
		}
	}

	if before != "" {
		_, beforeEnd, ok := parseDateRange(before)
		if !ok || itemEnd.After(beforeEnd) {
			return false
		}
	}

	return true
}

func parseDateRange(value string) (time.Time, time.Time, bool) {
	value = strings.TrimSpace(value)
	switch len(value) {
	case len("2006"):
		start, err := time.Parse("2006", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end := time.Date(start.Year(), time.December, 31, 23, 59, 59, 0, time.UTC)
		return start, end, true
	case len("2006-01"):
		start, err := time.Parse("2006-01", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end := time.Date(start.Year(), start.Month()+1, 0, 23, 59, 59, 0, time.UTC)
		return start, end, true
	default:
		if len(value) >= len("2006-01-02") {
			start, err := time.Parse("2006-01-02", value[:10])
			if err != nil {
				return time.Time{}, time.Time{}, false
			}
			end := time.Date(start.Year(), start.Month(), start.Day(), 23, 59, 59, 0, time.UTC)
			return start, end, true
		}
		return time.Time{}, time.Time{}, false
	}
}

func shortDate(value string) string {
	if len(value) >= 4 {
		return value[:4]
	}
	return value
}

func shortCreators(creators []domain.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	return shortCreatorLabel(creators[0].Name, len(creators))
}

func shortCreatorsAPI(creators []zoteroapi.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	return shortCreatorLabel(creators[0].Name, len(creators))
}

func filterDefaultFindItemsAPI(items []zoteroapi.Item, opts zoteroapi.FindOptions) []zoteroapi.Item {
	filtered := make([]zoteroapi.Item, 0, len(items))
	for _, item := range items {
		if !shouldIncludeFindItem(item.ItemType, item.Tags, item.Date, opts.ItemType, opts.Tags, opts.TagAny, opts.DateAfter, opts.DateBefore) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func shouldIncludeFindItem(itemType string, itemTags []string, itemDate string, requestedType string, requiredTags []string, anyMode bool, after string, before string) bool {
	if requestedType == "" && (itemType == "attachment" || itemType == "note" || itemType == "annotation") {
		return false
	}
	if !matchesTags(itemTags, requiredTags, anyMode) {
		return false
	}
	if !matchesDateRange(itemDate, after, before) {
		return false
	}
	return true
}

func shortCreatorLabel(name string, count int) string {
	if count <= 1 {
		return name
	}
	return name + " et al."
}

func filterVisibleNotes(notes []zoteroapi.Note) []zoteroapi.Note {
	filtered := make([]zoteroapi.Note, 0, len(notes))
	for _, note := range notes {
		if isMachineNote(note.Content) {
			continue
		}
		filtered = append(filtered, note)
	}
	return filtered
}

func isMachineNote(content string) bool {
	content = strings.TrimSpace(content)
	return strings.Contains(content, "{\"readingTime\":")
}

func notePreview(content string) string {
	content = strings.TrimSpace(content)
	const limit = 96
	if len(content) <= limit {
		return content
	}
	return strings.TrimSpace(content[:limit-3]) + "..."
}

func mutateTags(existing []string, tag string, add bool) []string {
	seen := make(map[string]struct{}, len(existing)+1)
	out := make([]string, 0, len(existing)+1)
	for _, current := range existing {
		if current == "" {
			continue
		}
		if !add && current == tag {
			continue
		}
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		out = append(out, current)
	}
	if add {
		if _, ok := seen[tag]; !ok {
			out = append(out, tag)
		}
	}
	return out
}

func toAPITags(tags []string) []map[string]string {
	out := make([]map[string]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, map[string]string{"tag": tag})
	}
	return out
}
