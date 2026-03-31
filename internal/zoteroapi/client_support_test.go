package zoteroapi

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
