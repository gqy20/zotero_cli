package app

import (
	"context"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type pdfScopeTestReader struct {
	backend.Reader
	target   backend.CollectionTarget
	findOpts backend.FindOptions
}

func (r *pdfScopeTestReader) CollectionTarget(context.Context, string) (backend.CollectionTarget, error) {
	return r.target, nil
}

func (r *pdfScopeTestReader) FindItems(_ context.Context, opts backend.FindOptions) ([]domain.Item, error) {
	r.findOpts = opts
	return []domain.Item{{Key: "ITEM1", Title: "Scoped paper"}}, nil
}

type pdfPageTestExtractor struct {
	result backend.ItemPageTextResult
}

func (e pdfPageTestExtractor) ExtractItemAttachmentPageTexts(context.Context, domain.Item) (backend.ItemPageTextResult, error) {
	return e.result, nil
}

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
	pattern, err := compilePDFTextPattern("methods")
	if err != nil {
		t.Fatal(err)
	}
	got, total, matches, truncated := filterPDFText("intro\nMethods: sampling and analysis\nresults", pattern, 14)
	if got != "intro\nMethods:" || total != 44 || matches != 1 || !truncated {
		t.Fatalf("got=%q total=%d truncated=%t", got, total, truncated)
	}
}

func TestFilterPDFTextUsesUnicodeCharactersAndContext(t *testing.T) {
	pattern, err := compilePDFTextPattern("泛基因组")
	if err != nil {
		t.Fatal(err)
	}
	got, total, matches, truncated := filterPDFText("背景\n方法：泛基因组分析\n结果", pattern, 8)
	if got != "背景\n方法：泛基" || total != 15 || matches != 1 || !truncated {
		t.Fatalf("got=%q total=%d truncated=%t", got, total, truncated)
	}
}

func TestPDFTextGrepUsesCaseInsensitiveRegularExpression(t *testing.T) {
	pattern, err := compilePDFTextPattern(`gene\s+flow|introgression`)
	if err != nil {
		t.Fatal(err)
	}
	got, _, matches, _ := filterPDFText("before\nGene flow was detected\nafter\nIntrogression occurred", pattern, 0)
	if matches != 2 || got == "" {
		t.Fatalf("matches=%d got=%q", matches, got)
	}
	if _, err := compilePDFTextPattern("["); err == nil {
		t.Fatal("expected invalid regular expression error")
	}
}

func TestPDFTextItemsResolveCollectionAndApplyItsKey(t *testing.T) {
	reader := &pdfScopeTestReader{target: backend.CollectionTarget{Key: "COLL1", Name: "Genetics", Path: "Research/Genetics"}}
	items, target, err := pdfTextItems(context.Background(), reader, nil, false, "Research/Genetics")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || target == nil || target.Key != "COLL1" {
		t.Fatalf("items=%#v target=%#v", items, target)
	}
	if len(reader.findOpts.Collection) != 1 || reader.findOpts.Collection[0] != "COLL1" || !reader.findOpts.All || !reader.findOpts.HasPDF {
		t.Fatalf("find options=%+v", reader.findOpts)
	}
}

func TestPDFTextPageEvidenceIncludesOnlyRegexMatches(t *testing.T) {
	pattern, err := compilePDFTextPattern(`gene\s+flow|introgression`)
	if err != nil {
		t.Fatal(err)
	}
	extractor := pdfPageTestExtractor{result: backend.ItemPageTextResult{Attachments: []backend.AttachmentPageText{{
		Attachment: domain.Attachment{Key: "PDF1"},
		Pages: []backend.PageText{
			{Page: 1, Text: "background only"},
			{Page: 2, Text: "Gene flow was detected twice: gene flow."},
			{Page: 3, Text: "Introgression was inferred."},
		},
	}}}}
	entry, _, available, err := extractPDFTextByPages(context.Background(), extractor, domain.Item{Key: "ITEM1", Title: "Paper"}, PDFTextRequest{Grep: `gene\s+flow|introgression`}, nil, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if !available || entry["match_count"] != 3 {
		t.Fatalf("entry=%#v available=%t", entry, available)
	}
	pages := entry["returned_pages"].([]int)
	if len(pages) != 2 || pages[0] != 2 || pages[1] != 3 {
		t.Fatalf("returned pages=%v", pages)
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
