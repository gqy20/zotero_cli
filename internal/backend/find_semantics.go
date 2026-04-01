package backend

import (
	"strings"
	"time"
)

func ShouldIncludeFindItem(itemType string, itemTags []string, itemDate string, requestedType string, requiredTags []string, anyMode bool, after string, before string) bool {
	if requestedType == "" && (itemType == "attachment" || itemType == "note" || itemType == "annotation") {
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
