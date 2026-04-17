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

func TestNormalizeFindOptionsTrimsAndDedupesSharedFindInputs(t *testing.T) {
	got := NormalizeFindOptions(FindOptions{
		Query:          "  hybrid speciation  ",
		ItemType:       " journalArticle ",
		Tags:           []string{" AI ", "survey", "ai", ""},
		Sort:           " date ",
		Direction:      " desc ",
		QMode:          " everything ",
		DateAfter:      " 2024-01 ",
		DateBefore:     " 2024-12-31 ",
		AttachmentName: " paper ",
		AttachmentPath: " storage ",
		AttachmentType: " pdf ",
	})

	if got.Query != "hybrid speciation" || got.ItemType != "journalArticle" {
		t.Fatalf("NormalizeFindOptions() did not trim shared fields: %#v", got)
	}
	if got.Sort != "date" || got.Direction != "desc" || got.QMode != "everything" {
		t.Fatalf("NormalizeFindOptions() did not normalize sort/direction/qmode: %#v", got)
	}
	if got.DateAfter != "2024-01" || got.DateBefore != "2024-12-31" {
		t.Fatalf("NormalizeFindOptions() did not normalize dates: %#v", got)
	}
	if got.AttachmentName != "paper" || got.AttachmentPath != "storage" || got.AttachmentType != "pdf" {
		t.Fatalf("NormalizeFindOptions() did not normalize attachment filters: %#v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "ai" || got.Tags[1] != "survey" {
		t.Fatalf("NormalizeFindOptions() tags = %#v, want [ai survey]", got.Tags)
	}
	if got.Tag != "" {
		t.Fatalf("NormalizeFindOptions() single-tag shortcut should stay empty for multiple tags: %#v", got.Tag)
	}
}

func TestSupportsWebFindRejectsLocalOnlyFindCapabilities(t *testing.T) {
	tests := []struct {
		name string
		opts FindOptions
		want bool
	}{
		{name: "plain query", opts: FindOptions{Query: "hybrid"}, want: true},
		{name: "qmode still web-capable", opts: FindOptions{Query: "hybrid", QMode: "everything"}, want: true},
		{name: "fulltext requires local", opts: FindOptions{Query: "hybrid", FullText: true}, want: false},
		{name: "attachment name requires local", opts: FindOptions{Query: "hybrid", AttachmentName: "pdf"}, want: false},
		{name: "has pdf requires local", opts: FindOptions{Query: "hybrid", HasPDF: true}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsWebFind(tt.opts); got != tt.want {
				t.Fatalf("SupportsWebFind(%#v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
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
