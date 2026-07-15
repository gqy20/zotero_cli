package app

import (
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

func TestMachineNoteDetection(t *testing.T) {
	if !isMachineNote(` {"readingTime":88,"progress":0.5}`) {
		t.Fatal("expected Zotero reading-time payload to be classified as a machine note")
	}
	if isMachineNote("A normal research note") {
		t.Fatal("normal note was classified as machine-generated")
	}
}

func TestOverviewTextIncludesEveryAvailableSection(t *testing.T) {
	got := overviewText(
		backend.LibraryStats{LibraryType: "user", LibraryID: "42", TotalItems: 3, TotalCollections: 1, TotalSearches: 2, LastLibraryVersion: 9},
		[]backend.Collection{{Key: "C1", Name: "Projects", NumItems: 3}},
		[]backend.Tag{{Name: "transformers", NumItems: 2}},
		[]domain.Item{{Key: "I1", Title: "Attention Is All You Need"}},
		LocalDataStatus{Status: "available", Path: "/data/zotero"},
		FullTextIndexStatus{Status: "unavailable", Path: "/data/zotero/.zotero_cli/fulltext/index.sqlite"},
	)
	for _, want := range []string{"Library: user:42", "Version: 9", "Top Collections:", "Projects", "Top Tags:", "transformers", "Recent Items:", "I1", "Local data: available", "Data dir: /data/zotero", "Full-text index: unavailable", "Build: zot index build"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overview text missing %q:\n%s", want, got)
		}
	}
}

func TestLimitSlice(t *testing.T) {
	values := []int{1, 2, 3}
	if got := limitSlice(values, 2); len(got) != 2 || got[1] != 2 {
		t.Fatalf("limitSlice = %v", got)
	}
	if got := limitSlice(values, 0); len(got) != 3 {
		t.Fatalf("unlimited limitSlice = %v", got)
	}
}
