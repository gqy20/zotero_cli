package app

import "strings"

var itemTypeAliases = map[string]string{
	"article": "journalArticle",
	"chapter": "bookSection",
	"conf":    "conferencePaper",
	"web":     "webpage",
	"blog":    "blogPost",
}

func NormalizeItemType(value string) string {
	trimmed := strings.TrimSpace(value)
	if canonical, ok := itemTypeAliases[strings.ToLower(trimmed)]; ok {
		return canonical
	}
	return trimmed
}
