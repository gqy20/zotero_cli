package app

import "testing"

func TestNormalizeItemTypeAliases(t *testing.T) {
	tests := map[string]string{"article": "journalArticle", "chapter": "bookSection", "conf": "conferencePaper", "web": "webpage", "blog": "blogPost", "journalArticle": "journalArticle"}
	for input, want := range tests {
		if got := NormalizeItemType(input); got != want {
			t.Fatalf("NormalizeItemType(%q) = %q; want %q", input, got, want)
		}
	}
}
