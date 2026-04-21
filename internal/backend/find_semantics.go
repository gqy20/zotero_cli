package backend

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NormalizeFindOptions(opts FindOptions) FindOptions {
	opts.Query = strings.TrimSpace(opts.Query)
	opts.ItemType = strings.TrimSpace(opts.ItemType)
	opts.Sort = strings.TrimSpace(opts.Sort)
	opts.Direction = strings.TrimSpace(opts.Direction)
	opts.QMode = strings.TrimSpace(opts.QMode)
	opts.DateAfter = strings.TrimSpace(opts.DateAfter)
	opts.DateBefore = strings.TrimSpace(opts.DateBefore)
	opts.AttachmentName = strings.TrimSpace(opts.AttachmentName)
	opts.AttachmentPath = strings.TrimSpace(opts.AttachmentPath)
	opts.AttachmentType = strings.TrimSpace(opts.AttachmentType)
	opts.ExcludeItemType = strings.TrimSpace(opts.ExcludeItemType)
	opts.DateModifiedAfter = strings.TrimSpace(opts.DateModifiedAfter)
	opts.DateAddedAfter = strings.TrimSpace(opts.DateAddedAfter)
	opts.Collection = normalizeFindTags(opts.Collection)
	opts.NoCollection = normalizeFindTags(opts.NoCollection)
	opts.TagContains = normalizeFindTags(opts.TagContains)
	opts.ExcludeTags = normalizeFindTags(opts.ExcludeTags)
	opts.Tags = normalizeFindTags(opts.Tags)
	opts.Tag = ""
	if len(opts.Tags) == 1 {
		opts.Tag = opts.Tags[0]
	}
	return opts
}

func SupportsWebFind(opts FindOptions) bool {
	opts = NormalizeFindOptions(opts)
	return !requiresLocalFindSupport(opts)
}

func ShouldIncludeFindItem(itemType string, itemTags []string, itemDate string, requestedType string, requiredTags []string, anyMode bool, after string, before string) bool {
	if requestedType == "" && !isDefaultFindVisibleItemType(itemType) {
		return false
	}
	if !matchesTags(itemTags, requiredTags, anyMode) {
		return false
	}
	if !MatchesDateRange(itemDate, after, before) {
		return false
	}
	return true
}

func isDefaultFindVisibleItemType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "attachment", "note", "annotation":
		return false
	default:
		return true
	}
}

func requiresLocalFindSupport(opts FindOptions) bool {
	if opts.FullText || hasAttachmentFindFilters(opts) {
		return true
	}
	if len(opts.Collection) > 0 || len(opts.NoCollection) > 0 {
		return true
	}
	if len(opts.TagContains) > 0 || len(opts.ExcludeTags) > 0 {
		return true
	}
	if opts.ExcludeItemType != "" {
		return true
	}
	if opts.DateModifiedAfter != "" || opts.DateAddedAfter != "" {
		return true
	}
	return false
}

func hasAttachmentFindFilters(opts FindOptions) bool {
	return opts.HasPDF ||
		strings.TrimSpace(opts.AttachmentName) != "" ||
		strings.TrimSpace(opts.AttachmentPath) != "" ||
		strings.TrimSpace(opts.AttachmentType) != ""
}

func normalizeFindTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalized := strings.TrimSpace(strings.ToLower(tag))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func MatchesDateRange(itemDate string, after string, before string) bool {
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

func compareFindDates(left string, right string) int {
	leftStart, _, leftOK := parseDateRange(left)
	rightStart, _, rightOK := parseDateRange(right)
	if leftOK && rightOK {
		if leftStart.Before(rightStart) {
			return -1
		}
		if leftStart.After(rightStart) {
			return 1
		}
		return 0
	}
	if leftOK {
		return -1
	}
	if rightOK {
		return 1
	}
	return strings.Compare(left, right)
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

func parseRelativeDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(s[:len(s)-1])
		if err != nil || days < 0 {
			return "", false
		}
		return fmt.Sprintf("datetime('now', '-%d days', 'start of day')", days), true
	}
	switch len(s) {
	case len("2006"):
		if _, err := time.Parse("2006", s); err != nil {
			return "", false
		}
		return fmt.Sprintf("datetime('%s-01-01')", s), true
	case len("2006-01"):
		if _, err := time.Parse("2006-01", s); err != nil {
			return "", false
		}
		return fmt.Sprintf("datetime('%s-01')", s), true
	default:
		if len(s) >= len("2006-01-02") {
			if _, err := time.Parse("2006-01-02", s[:10]); err != nil {
				return "", false
			}
			return fmt.Sprintf("datetime('%s')", s[:10]), true
		}
		return "", false
	}
}
