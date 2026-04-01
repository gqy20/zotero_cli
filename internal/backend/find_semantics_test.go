package backend

import "testing"

func TestShouldIncludeFindItemFiltersNonTopItemsByDefault(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		want     bool
	}{
		{name: "primary item", itemType: "journalArticle", want: true},
		{name: "attachment", itemType: "attachment", want: false},
		{name: "note", itemType: "note", want: false},
		{name: "annotation", itemType: "annotation", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludeFindItem(tt.itemType, nil, "", "", nil, false, "", "")
			if got != tt.want {
				t.Fatalf("ShouldIncludeFindItem(%q) = %v, want %v", tt.itemType, got, tt.want)
			}
		})
	}
}

func TestShouldIncludeFindItemAllowsRequestedType(t *testing.T) {
	if !ShouldIncludeFindItem("attachment", nil, "", "attachment", nil, false, "", "") {
		t.Fatalf("expected explicitly requested item type to remain visible")
	}
}

func TestShouldIncludeFindItemSupportsTagSemantics(t *testing.T) {
	if !ShouldIncludeFindItem("journalArticle", []string{"AI", "survey"}, "", "", []string{"ai", "survey"}, false, "", "") {
		t.Fatalf("expected AND tag semantics to match")
	}
	if ShouldIncludeFindItem("journalArticle", []string{"AI"}, "", "", []string{"ai", "survey"}, false, "", "") {
		t.Fatalf("expected missing AND tag to fail")
	}
	if !ShouldIncludeFindItem("journalArticle", []string{"classic"}, "", "", []string{"ai", "classic"}, true, "", "") {
		t.Fatalf("expected OR tag semantics to match")
	}
}

func TestMatchesDateRangeSupportsYearMonthAndDate(t *testing.T) {
	tests := []struct {
		name     string
		itemDate string
		after    string
		before   string
		want     bool
	}{
		{name: "year after", itemDate: "2024", after: "2023", before: "", want: true},
		{name: "month after", itemDate: "2024-05", after: "2024-04", before: "", want: true},
		{name: "date bounded", itemDate: "2024-05-03", after: "2024-05-01", before: "2024-12-31", want: true},
		{name: "before fails", itemDate: "2023", after: "2024", before: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesDateRange(tt.itemDate, tt.after, tt.before); got != tt.want {
				t.Fatalf("MatchesDateRange(%q, %q, %q) = %v, want %v", tt.itemDate, tt.after, tt.before, got, tt.want)
			}
		})
	}
}
