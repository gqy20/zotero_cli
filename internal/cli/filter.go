package cli

import (
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

func filterDefaultFindItems(items []domain.Item, opts backend.FindOptions) []domain.Item {
	opts = backend.NormalizeFindOptions(opts)
	filtered := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if !backend.ShouldIncludeFindItem(item.ItemType, item.Tags, item.Date, opts.ItemType, opts.Tags, opts.TagAny, opts.DateAfter, opts.DateBefore) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
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
	normalized := backend.NormalizeFindOptions(backend.FindOptions{
		ItemType:   opts.ItemType,
		Tags:       opts.Tags,
		TagAny:     opts.TagAny,
		DateAfter:  opts.DateAfter,
		DateBefore: opts.DateBefore,
	})
	filtered := make([]zoteroapi.Item, 0, len(items))
	for _, item := range items {
		if !backend.ShouldIncludeFindItem(item.ItemType, item.Tags, item.Date, normalized.ItemType, normalized.Tags, normalized.TagAny, normalized.DateAfter, normalized.DateBefore) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
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
