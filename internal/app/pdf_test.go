package app

import (
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

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
	if got != "intro\nMethods:" || total != 44 || !truncated {
		t.Fatalf("got=%q total=%d truncated=%t", got, total, truncated)
	}
}

func TestFilterPDFTextUsesUnicodeCharactersAndContext(t *testing.T) {
	got, total, truncated := filterPDFText("背景\n方法：泛基因组分析\n结果", "泛基因组", 8)
	if got != "背景\n方法：泛基" || total != 15 || !truncated {
		t.Fatalf("got=%q total=%d truncated=%t", got, total, truncated)
	}
}

func TestPDFTextCachePathMode(t *testing.T) {
	if !shouldReturnPDFCachePaths("local", PDFTextRequest{}) || !shouldReturnPDFCachePaths("hybrid", PDFTextRequest{All: true}) {
		t.Fatal("local and hybrid unfiltered reads should return cache paths")
	}
	for _, request := range []PDFTextRequest{{Grep: "method"}, {Pages: "2"}, {MaxChars: 100}, {OutputDir: "out"}} {
		if shouldReturnPDFCachePaths("local", request) {
			t.Fatalf("filtered/export request unexpectedly uses path mode: %+v", request)
		}
	}
	if shouldReturnPDFCachePaths("remote", PDFTextRequest{}) {
		t.Fatal("remote reads cannot return server-local cache paths")
	}
}

func TestPDFTextCachePathEntryOmitsFullText(t *testing.T) {
	entry, text, err := pdfTextCachePathEntry(domain.Item{Key: "ITEM1"}, backend.ItemFullTextResult{
		PrimaryAttachmentKey: "ATT1",
		Attachments: []backend.AttachmentFullText{{
			Attachment:  domain.Attachment{Key: "ATT1"},
			Text:        "完整正文",
			Source:      "pymupdf",
			CacheHit:    true,
			ContentPath: `D:\\cache\\ATT1\\content.txt`,
			ChunksPath:  `D:\\cache\\ATT1\\chunks.json`,
		}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if text != `D:\\cache\\ATT1\\content.txt` || entry["content_path"] != text || entry["chunks_path"] != `D:\\cache\\ATT1\\chunks.json` {
		t.Fatalf("entry=%+v text=%q", entry, text)
	}
	if _, ok := entry["text"]; ok {
		t.Fatalf("full text leaked into path response: %+v", entry)
	}
}
