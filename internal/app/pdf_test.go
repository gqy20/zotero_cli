package app

import "testing"

func TestParsePageRanges(t *testing.T) {
	ranges, err := parsePageRanges("1-3,7")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []int{1, 2, 3, 7} {
		if !pageInPDFRanges(page, ranges) {
			t.Fatalf("page %d not selected", page)
		}
	}
	if pageInPDFRanges(4, ranges) {
		t.Fatal("page 4 unexpectedly selected")
	}
	if _, err := parsePageRanges("3-1"); err == nil {
		t.Fatal("expected invalid descending range")
	}
}

func TestFilterPDFText(t *testing.T) {
	got, total, truncated := filterPDFText("intro\nMethods: sampling and analysis\nresults", "methods", 14)
	if got != "Methods: sampl" || total != 44 || !truncated {
		t.Fatalf("got=%q total=%d truncated=%t", got, total, truncated)
	}
}
