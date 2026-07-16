package backend

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/syncmirror"

	_ "modernc.org/sqlite"
)

func TestFullTextCacheSaveAndLoad(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourcePath := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	doc := FullTextDocument{
		Text: "normalized text",
		Meta: fullTextCacheMeta{
			AttachmentKey:   "ATT123",
			ParentItemKey:   "ITEM123",
			ResolvedPath:    sourcePath,
			ContentType:     "application/pdf",
			Title:           "Normalized Title",
			Creators:        "Alice Bob",
			Tags:            "genomics plants",
			AttachmentTitle: "Supplement PDF",
			AttachmentName:  "paper.pdf",
			AttachmentPath:  sourcePath,
			Extractor:       "zotero_ft_cache",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
			TextHash:        "sha256:test",
			ExtractedAt:     "2026-04-16T00:00:00Z",
			Chars:           len("normalized text"),
		},
	}
	if err := cache.Save(doc); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, ok, err := cache.Load(domain.Attachment{Key: "ATT123", ResolvedPath: sourcePath, Resolved: true, ContentType: "application/pdf"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() ok = false, want true")
	}
	if got.Text != doc.Text {
		t.Fatalf("Load() text = %q, want %q", got.Text, doc.Text)
	}
	if got.Meta.AttachmentKey != doc.Meta.AttachmentKey {
		t.Fatalf("Load() meta = %#v, want attachment key %q", got.Meta, doc.Meta.AttachmentKey)
	}
	indexDB, err := sql.Open("sqlite", cache.indexPath())
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer indexDB.Close()
	var indexed string
	if err := indexDB.QueryRow(`SELECT body FROM fulltext_documents WHERE attachment_key = ?`, "ATT123").Scan(&indexed); err != nil {
		t.Fatalf("query fulltext_documents: %v", err)
	}
	if indexed != "normalized text" {
		t.Fatalf("indexed body = %q, want %q", indexed, "normalized text")
	}
	var attachmentName string
	if err := indexDB.QueryRow(`SELECT attachment_name FROM fulltext_documents WHERE attachment_key = ?`, "ATT123").Scan(&attachmentName); err != nil {
		t.Fatalf("query attachment_name: %v", err)
	}
	if attachmentName != "paper.pdf" {
		t.Fatalf("indexed attachment_name = %q, want %q", attachmentName, "paper.pdf")
	}
	var textHash, indexedTextHash string
	if err := indexDB.QueryRow(`SELECT text_hash, indexed_text_hash FROM fulltext_meta WHERE attachment_key = ?`, "ATT123").Scan(&textHash, &indexedTextHash); err != nil {
		t.Fatalf("query fulltext_meta index status: %v", err)
	}
	if textHash == "" || indexedTextHash != textHash {
		t.Fatalf("indexed_text_hash = %q, text_hash = %q, want matching non-empty hashes", indexedTextHash, textHash)
	}
}

func TestFullTextEvidenceWindowIncludesAdjacentChunks(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "evidence.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE fulltext_chunks (attachment_key TEXT, chunk_index INTEGER, page INTEGER, body TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		index int
		page  int
		body  string
	}{{0, 2, "context before"}, {1, 3, "matched evidence"}, {2, 3, "context after"}, {3, 4, "outside"}} {
		if _, err := db.Exec(`INSERT INTO fulltext_chunks VALUES ('ATT1', ?, ?, ?)`, row.index, row.page, row.body); err != nil {
			t.Fatal(err)
		}
	}
	text, pageEnd, err := fullTextEvidenceWindow(db, "ATT1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if text != "context before\nmatched evidence\ncontext after" || pageEnd != 3 {
		t.Fatalf("text=%q pageEnd=%d", text, pageEnd)
	}
}

func TestFullTextCacheLoadRejectsStaleEntry(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourcePath := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	doc := FullTextDocument{
		Text: "normalized text",
		Meta: fullTextCacheMeta{
			AttachmentKey:   "ATT123",
			ResolvedPath:    sourcePath,
			ContentType:     "application/pdf",
			Extractor:       "zotero_ft_cache",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
		},
	}
	if err := cache.Save(doc); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("new content"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, ok, err := cache.Load(domain.Attachment{Key: "ATT123", ResolvedPath: sourcePath, Resolved: true, ContentType: "application/pdf"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ok {
		t.Fatal("Load() ok = true, want false for stale cache")
	}
}

func TestFullTextCacheRejectsSameSizeReplacementWithinOneSecond(t *testing.T) {
	cache := newFullTextCache(t.TempDir())
	sourcePath := filepath.Join(t.TempDir(), "paper.pdf")
	oldTime := time.Unix(1_750_000_000, 100_000_000)
	newTime := time.Unix(1_750_000_000, 700_000_000)
	if err := os.WriteFile(sourcePath, []byte("old-pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sourcePath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	doc := FullTextDocument{
		Text:   "cached phrase from the original PDF",
		Chunks: []chunk{{Page: 1, Text: "cached phrase from the original PDF", BlockCount: 1}},
		Meta: fullTextCacheMeta{
			AttachmentKey: "ATT123",
			ParentItemKey: "ITEM123",
			ResolvedPath:  sourcePath,
			ContentType:   "application/pdf",
		},
	}
	if err := cache.Save(doc); err != nil {
		t.Fatalf("Save() error=%v", err)
	}
	attachment := domain.Attachment{Key: "ATT123", ResolvedPath: sourcePath, Resolved: true, ContentType: "application/pdf"}
	if _, ok, err := cache.Load(attachment); err != nil || !ok {
		t.Fatalf("initial Load() ok=%v error=%v", ok, err)
	}
	legacyInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	legacyMeta := fullTextCacheMeta{AttachmentKey: "ATT123", ResolvedPath: sourcePath, ContentType: "application/pdf", SourceMtimeUnix: legacyInfo.ModTime().Unix(), SourceSize: legacyInfo.Size()}
	if !cache.IsFresh(legacyMeta, attachment) {
		t.Fatal("legacy second-resolution cache metadata should remain compatible")
	}
	if matches, err := cache.Search("cached phrase", 10); err != nil || len(matches) != 1 {
		t.Fatalf("initial Search() matches=%v error=%v", matches, err)
	}
	reader := &LocalReader{FullTextCacheDir: cache.rootDir}
	if !reader.IsFullTextIndexed(attachment) {
		t.Fatal("initial IsFullTextIndexed()=false")
	}

	if err := os.WriteFile(sourcePath, []byte("new-pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sourcePath, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := cache.Load(attachment); err != nil || ok {
		t.Fatalf("stale Load() ok=%v error=%v", ok, err)
	}
	if matches, err := cache.Search("cached phrase", 10); err != nil || len(matches) != 0 {
		t.Fatalf("stale Search() matches=%v error=%v", matches, err)
	}
	if reader.IsFullTextIndexed(attachment) {
		t.Fatal("stale IsFullTextIndexed()=true")
	}
}

func TestNewLocalReaderConfiguresFullTextCacheDir(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "zotero.sqlite"), []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader, err := NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	want := filepath.Join(dataDir, ".zotero_cli", "fulltext")
	if reader.FullTextCacheDir != want {
		t.Fatalf("reader.FullTextCacheDir = %q, want %q", reader.FullTextCacheDir, want)
	}
}

func TestLocalFullTextPreviewCachesZoteroFTCacheContent(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "ATT123", ".zotero-ft-cache"), []byte("cached source text"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{
		DataDir:           dataDir,
		StorageDir:        storageDir,
		FullTextCacheDir:  filepath.Join(dataDir, ".zotero_cli", "fulltext"),
		AttachmentBaseDir: "",
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT123", ResolvedPath: resolvedPath, Resolved: true, ContentType: "application/pdf"},
		},
	}

	preview, err := reader.FullTextPreview(context.Background(), item)
	if err != nil {
		t.Fatalf("FullTextPreview() error = %v", err)
	}
	if preview != "cached source text" {
		t.Fatalf("FullTextPreview() = %q, want %q", preview, "cached source text")
	}
	readMeta := reader.ConsumeReadMetadata()
	if readMeta.FullTextSource != "zotero_ft_cache" || readMeta.FullTextAttachmentKey != "ATT123" || readMeta.FullTextCacheHit {
		t.Fatalf("ConsumeReadMetadata() = %#v, want zotero ft cache metadata", readMeta)
	}

	contentPath := filepath.Join(reader.FullTextCacheDir, "cache", "ATT123", "content.txt")
	metaPath := filepath.Join(reader.FullTextCacheDir, "cache", "ATT123", "meta.json")
	if _, err := os.Stat(contentPath); err != nil {
		t.Fatalf("content cache missing: %v", err)
	}
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var cacheMeta fullTextCacheMeta
	if err := json.Unmarshal(metaData, &cacheMeta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if cacheMeta.Extractor != "zotero_ft_cache" {
		t.Fatalf("meta.Extractor = %q, want zotero_ft_cache", cacheMeta.Extractor)
	}
	if err := os.Remove(filepath.Join(storageDir, "ATT123", ".zotero-ft-cache")); err != nil {
		t.Fatal(err)
	}

	preview, err = reader.FullTextPreview(context.Background(), item)
	if err != nil {
		t.Fatalf("FullTextPreview() second call error = %v", err)
	}
	if preview != "cached source text" {
		t.Fatalf("FullTextPreview() second call = %q, want cached source text", preview)
	}
	readMeta = reader.ConsumeReadMetadata()
	if readMeta.FullTextSource != "zotero_ft_cache" || readMeta.FullTextAttachmentKey != "ATT123" || !readMeta.FullTextCacheHit {
		t.Fatalf("ConsumeReadMetadata() second call = %#v, want cache-hit metadata", readMeta)
	}
}

func TestLocalFullTextPreviewFallsBackToPDFiumAndCachesResult(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	previous := extractFullTextWithPDFiumFunc
	t.Cleanup(func() { extractFullTextWithPDFiumFunc = previous })
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{
			Text: "pdfium extracted text",
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Extractor:       "pdfium",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
				Pages:           1,
				Chars:           len([]rune("pdfium extracted text")),
			},
		}, true, nil
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: filepath.Join(dataDir, ".zotero_cli", "fulltext"),
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT123", ResolvedPath: resolvedPath, Resolved: true, ContentType: "application/pdf"},
		},
	}

	preview, err := reader.FullTextPreview(context.Background(), item)
	if err != nil {
		t.Fatalf("FullTextPreview() error = %v", err)
	}
	if preview != "pdfium extracted text" {
		t.Fatalf("FullTextPreview() = %q, want pdfium extracted text", preview)
	}
	readMeta := reader.ConsumeReadMetadata()
	if readMeta.FullTextSource != "pdfium" || readMeta.FullTextAttachmentKey != "ATT123" || readMeta.FullTextCacheHit {
		t.Fatalf("ConsumeReadMetadata() = %#v, want pdfium metadata", readMeta)
	}

	metaPath := filepath.Join(reader.FullTextCacheDir, "cache", "ATT123", "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var cacheMeta fullTextCacheMeta
	if err := json.Unmarshal(metaData, &cacheMeta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if cacheMeta.Extractor != "pdfium" {
		t.Fatalf("meta.Extractor = %q, want pdfium", cacheMeta.Extractor)
	}
}

func TestNormalizeFullTextTextCleansWhitespaceAndHyphenation(t *testing.T) {
	input := "  This\t is   a   test.\r\n\r\ninforma-\n  tion retrieval \n\n\nNext\tparagraph.  "
	got := normalizeFullTextText(input)
	want := "This is a test.\n\ninformation retrieval\n\nNext paragraph."
	if got != want {
		t.Fatalf("normalizeFullTextText() = %q, want %q", got, want)
	}
}

func TestNormalizeFullTextTextRemovesHeadersAndMergesWrappedLines(t *testing.T) {
	input := strings.Join([]string{
		"Molecular Ecology. 2024;00:e17412. | 1 of 9 https://doi.org/10.1111/mec.17412",
		"wileyonlinelibrary.com/journal/mec",
		"1 | INTRODUCTION",
		"Speciation is often defined as a process in which one species splits",
		"into two. However, new species can also form as a result of hy",
		"bridization between different species.",
		"",
		"\f2 of 9 | LONG and RIESEBERG",
		"Downloaded from https://onlinelibrary.wiley.com/doi/10.1111/mec.17412",
		"See the Terms and Conditions on Wiley Online Library for rules of use;",
		"Abstract",
		"Homoploid hybrid speciation is challenging to document because hybridization can",
		"lead to outcomes other than speciation.",
	}, "\n")

	got := normalizeFullTextText(input)
	want := strings.Join([]string{
		"1 | INTRODUCTION",
		"Speciation is often defined as a process in which one species splits into two. However, new species can also form as a result of hybridization between different species.",
		"",
		"Abstract",
		"Homoploid hybrid speciation is challenging to document because hybridization can lead to outcomes other than speciation.",
	}, "\n")
	if got != want {
		t.Fatalf("normalizeFullTextText() = %q, want %q", got, want)
	}
}

func TestNormalizeFullTextTextRepairsCommonJoinedWords(t *testing.T) {
	input := strings.Join([]string{
		"Also,these outcomes are not mutually exclusive.",
		"Some authors used Wanget al. (2021), while others cited Sunet al. (2020).",
		"The estab lishment of reproductive isola tion may drive evolu tion.",
		"Next, whole-genome sequencing dataand standard analyses were used.",
		"The criteria maybe too strict, but the straight forward pipeline can didate genes in in dels data.",
		"Signals may remain if homop loid lineages have paren tal barriers thatthusmay persist.",
	}, "\n")

	got := normalizeFullTextText(input)
	want := "Also, these outcomes are not mutually exclusive. Some authors used Wang et al. (2021), while others cited Sun et al. (2020). The establishment of reproductive isolation may drive evolution. Next, whole-genome sequencing data and standard analyses were used. The criteria may be too strict, but the straightforward pipeline candidate genes in indels data. Signals may remain if homoploid lineages have parental barriers that thus may persist."
	if got != want {
		t.Fatalf("normalizeFullTextText() = %q, want %q", got, want)
	}
}

func TestBuildFullTextSnippetCentersMatch(t *testing.T) {
	text := strings.Repeat("preface context ", 100) + "Core section discusses speciation genome patterns in plants and gene flow. " + strings.Repeat("ending context ", 100)
	got := buildFullTextSnippet(text, "speciation genome")
	if !strings.Contains(got, "speciation genome patterns in plants") {
		t.Fatalf("buildFullTextSnippet() = %q, want centered match", got)
	}
	if len([]rune(got)) > 1206 {
		t.Fatalf("buildFullTextSnippet() length = %d, want about 1200 runes", len([]rune(got)))
	}
}

func TestCenterHighlightedEvidenceKeepsActualHit(t *testing.T) {
	value := strings.Repeat("before ", 300) + fullTextHitStart + "target phrase" + fullTextHitEnd + strings.Repeat(" after", 300)
	got := centerHighlightedEvidence(value, 1200)
	if !strings.Contains(got, "target phrase") {
		t.Fatalf("centerHighlightedEvidence() omitted hit: %q", got)
	}
	if strings.Contains(got, fullTextHitStart) || strings.Contains(got, fullTextHitEnd) {
		t.Fatalf("centerHighlightedEvidence() leaked markers: %q", got)
	}
}

func TestFullTextCacheSearchReturnsIndexedMatches(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourcePath := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	doc := FullTextDocument{
		Text: "Core section discusses speciation genome patterns in plants.",
		Meta: fullTextCacheMeta{
			AttachmentKey:   "ATT123",
			ParentItemKey:   "ITEM123",
			ResolvedPath:    sourcePath,
			ContentType:     "application/pdf",
			Title:           "Alpine Genome Study",
			Creators:        "Alice Bob",
			Tags:            "speciation plants",
			AttachmentTitle: "Genome Supplement",
			AttachmentName:  "genome.pdf",
			AttachmentPath:  sourcePath,
			Extractor:       "zotero_ft_cache",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
		},
	}
	if err := cache.Save(doc); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	matches, err := cache.Search("speciation genome", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() matches = %#v, want 1 match", matches)
	}
	if matches[0].ParentItemKey != "ITEM123" || matches[0].AttachmentKey != "ATT123" {
		t.Fatalf("Search() match = %#v, want ITEM123/ATT123", matches[0])
	}

	matches, err = cache.Search("genome patterns", 10)
	if err != nil {
		t.Fatalf("Search() field query error = %v", err)
	}
	if len(matches) != 1 || matches[0].AttachmentKey != "ATT123" {
		t.Fatalf("Search() field query matches = %#v, want ATT123", matches)
	}
}

func TestFullTextCacheSaveBatchPreservesExistingIndexEntries(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourceDir := t.TempDir()

	makeDoc := func(attachmentKey, parentKey, text string) FullTextDocument {
		sourcePath := filepath.Join(sourceDir, attachmentKey+".pdf")
		if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		return FullTextDocument{
			Text: text,
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachmentKey,
				ParentItemKey:   parentKey,
				ResolvedPath:    sourcePath,
				ContentType:     "application/pdf",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}
	}

	first := makeDoc("ATT123", "ITEM123", "first document mentions chestnut restoration")
	if err := cache.Save(first); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	second := makeDoc("ATT456", "ITEM456", "second document mentions wheat breeding")
	if err := cache.SaveBatch([]FullTextDocument{second}); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	matches, err := cache.Search("chestnut restoration", 10)
	if err != nil {
		t.Fatalf("Search() first error = %v", err)
	}
	if len(matches) != 1 || matches[0].AttachmentKey != "ATT123" {
		t.Fatalf("Search() first matches = %#v, want preserved ATT123", matches)
	}
	matches, err = cache.Search("wheat breeding", 10)
	if err != nil {
		t.Fatalf("Search() second error = %v", err)
	}
	if len(matches) != 1 || matches[0].AttachmentKey != "ATT456" {
		t.Fatalf("Search() second matches = %#v, want ATT456", matches)
	}
	statuses, err := cache.IndexStatuses()
	if err != nil {
		t.Fatalf("IndexStatuses() error = %v", err)
	}
	for _, key := range []string{"ATT123", "ATT456"} {
		status := statuses[key]
		if status.TextHash == "" || status.IndexedTextHash != status.TextHash {
			t.Fatalf("IndexStatuses()[%s] = %#v, want matching hashes", key, status)
		}
	}
}

func TestFullTextCacheSearchDedupesParentItems(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourceDir := t.TempDir()

	writeDoc := func(attachmentKey, attachmentName string) {
		sourcePath := filepath.Join(sourceDir, attachmentKey+".pdf")
		if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		doc := FullTextDocument{
			Text: "Hybridization and speciation in plants with genomic evidence.",
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachmentKey,
				ParentItemKey:   "ITEM123",
				ResolvedPath:    sourcePath,
				ContentType:     "application/pdf",
				Title:           "Hybridization and speciation",
				AttachmentName:  attachmentName,
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}
		if err := cache.Save(doc); err != nil {
			t.Fatalf("Save(%s) error = %v", attachmentKey, err)
		}
	}

	writeDoc("ATT123", "one.pdf")
	writeDoc("ATT456", "two.pdf")

	matches, err := cache.Search("hybridization speciation", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() matches = %#v, want 1 deduped parent match", matches)
	}
	if matches[0].ParentItemKey != "ITEM123" {
		t.Fatalf("Search() parent = %#v, want ITEM123", matches[0])
	}
}

func TestFullTextCacheSearchReturnsPhraseMatches(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourceDir := t.TempDir()

	writeDoc := func(attachmentKey, parentKey, title, text string) {
		sourcePath := filepath.Join(sourceDir, attachmentKey+".pdf")
		if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		doc := FullTextDocument{
			Text: text,
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachmentKey,
				ParentItemKey:   parentKey,
				ResolvedPath:    sourcePath,
				ContentType:     "application/pdf",
				Title:           title,
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}
		if err := cache.Save(doc); err != nil {
			t.Fatalf("Save(%s) error = %v", attachmentKey, err)
		}
	}

	writeDoc("ATT123", "ITEM123", "Plant extinction in the anthropocene", "Plant extinction in the anthropocene with discussion of extinction rates.")
	writeDoc("ATT456", "ITEM456", "Coral declines", "Plant extinction in the anthropocene is cited once in a broader coral vulnerability paper.")

	matches, err := cache.Search("Plant extinction in the anthropocene", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("Search() returned no matches")
	}
	if len(matches) != 2 {
		t.Fatalf("Search() returned %d matches, want 2", len(matches))
	}
}

func TestFullTextCacheSearchUsesNativeChunkBM25Ordering(t *testing.T) {
	rootDir := t.TempDir()
	cache := newFullTextCache(rootDir)
	sourceDir := t.TempDir()

	writeDoc := func(attachmentKey, parentKey, title, text string) {
		sourcePath := filepath.Join(sourceDir, attachmentKey+".pdf")
		if err := os.WriteFile(sourcePath, []byte("pdf"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		doc := FullTextDocument{
			Text: text,
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachmentKey,
				ParentItemKey:   parentKey,
				ResolvedPath:    sourcePath,
				ContentType:     "application/pdf",
				Title:           title,
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}
		if err := cache.Save(doc); err != nil {
			t.Fatalf("Save(%s) error = %v", attachmentKey, err)
		}
	}

	writeDoc("ATT123", "ITEM123", "Genome Architecture and Speciation in Plants and Animals", "Genome architecture and speciation are both central themes in this review.")
	writeDoc("ATT456", "ITEM456", "Changing views on speciation", "Speciation speciation speciation with no meaningful genome discussion.")

	matches, err := cache.Search("speciation genome", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("Search() returned no matches")
	}
	if matches[0].ParentItemKey != "ITEM456" {
		t.Fatalf("Search() top match = %#v, want native BM25 ordering without custom token balancing", matches[0])
	}
}

func TestLocalSQLiteDSNUsesReadOnlyPragmas(t *testing.T) {
	dsn := localSQLiteDSN(`D:\Zotero\zotero.sqlite`)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if u.Scheme != "file" {
		t.Fatalf("unexpected scheme: %q", u.Scheme)
	}
	if got := u.Query().Get("mode"); got != "ro" {
		t.Fatalf("unexpected mode query param: %q", got)
	}
	pragmas := u.Query()["_pragma"]
	if len(pragmas) != 2 {
		t.Fatalf("unexpected pragmas: %#v", pragmas)
	}
	if pragmas[0] != "busy_timeout=200" && pragmas[1] != "busy_timeout=200" {
		t.Fatalf("expected busy_timeout pragma, got %#v", pragmas)
	}
	if pragmas[0] != "query_only=1" && pragmas[1] != "query_only=1" {
		t.Fatalf("expected query_only pragma, got %#v", pragmas)
	}
}

func TestLocalSQLiteDSNRespectsBusyTimeoutOverride(t *testing.T) {
	t.Setenv("ZOT_LOCAL_BUSY_TIMEOUT_MS", "25")

	dsn := localSQLiteDSN(`D:\Zotero\zotero.sqlite`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	pragmas := u.Query()["_pragma"]
	found := false
	for _, pragma := range pragmas {
		if pragma == "busy_timeout=25" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected busy_timeout override, got %#v", pragmas)
	}
}

func TestCreateSQLiteSnapshotCopiesDatabaseAndSidecars(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	journalPath := sqlitePath + "-journal"
	walPath := sqlitePath + "-wal"

	if err := os.WriteFile(sqlitePath, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte("journal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, []byte("wal"), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshotDir, snapshotPath, err := createSQLiteSnapshot(sqlitePath)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	defer os.RemoveAll(snapshotDir)

	for path, want := range map[string]string{
		snapshotPath:              "db",
		snapshotPath + "-journal": "journal",
		snapshotPath + "-wal":     "wal",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read snapshot file %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("unexpected snapshot contents for %s: %q", path, string(data))
		}
	}
}

func TestCachedSnapshotReusesUnchangedGeneration(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.WriteFile(sqlitePath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, firstPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if !isSnapshotValid(sqlitePath, cacheDir) {
		t.Fatal("fresh snapshot should be valid")
	}
	_, secondPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath != secondPath {
		t.Fatalf("unchanged source should reuse generation: first=%q second=%q", firstPath, secondPath)
	}
}

func TestCachedSnapshotRefreshesWhenWALChanges(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.WriteFile(sqlitePath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-wal", []byte("wal-one"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, firstPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-wal", []byte("wal-two-with-new-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if isSnapshotValid(sqlitePath, cacheDir) {
		t.Fatal("WAL change must invalidate cached snapshot")
	}
	_, secondPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath == secondPath {
		t.Fatalf("WAL change should publish a new generation: %q", secondPath)
	}
	data, err := os.ReadFile(secondPath + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "wal-two-with-new-data" {
		t.Fatalf("new WAL contents=%q", got)
	}
	if _, err := os.Stat(filepath.Dir(firstPath)); !os.IsNotExist(err) {
		t.Fatalf("previous generation should be removed, stat err=%v", err)
	}
}

func TestCachedSnapshotRefreshesWhenJournalAppears(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.WriteFile(sqlitePath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, firstPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-journal", []byte("journal-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, secondPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath == secondPath {
		t.Fatalf("journal appearance should publish a new generation: %q", secondPath)
	}
	data, err := os.ReadFile(secondPath + "-journal")
	if err != nil || string(data) != "journal-data" {
		t.Fatalf("journal data=%q err=%v", data, err)
	}
}

func TestCachedSnapshotDoesNotRefreshForSHMOnlyChange(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.WriteFile(sqlitePath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-shm", []byte("shm-one"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, firstPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-shm", []byte("shm-two-with-lock-churn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !isSnapshotValid(sqlitePath, cacheDir) {
		t.Fatal("SHM-only changes must not invalidate data snapshot")
	}
	_, secondPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath != secondPath {
		t.Fatalf("SHM-only change should reuse generation: first=%q second=%q", firstPath, secondPath)
	}
}

func TestCachedSnapshotMigratesLegacyFlatCache(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath, []byte("current-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(cacheDir, "zotero.sqlite")
	if err := os.WriteFile(legacyPath, []byte("legacy-database"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, snapshotPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotPath == legacyPath || !strings.Contains(filepath.Base(filepath.Dir(snapshotPath)), "generation-") {
		t.Fatalf("legacy cache was not migrated: %q", snapshotPath)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy flat snapshot should be removed, stat err=%v", err)
	}
	if !isSnapshotValid(sqlitePath, cacheDir) {
		t.Fatal("migrated snapshot should be valid")
	}
}

func TestCachedSnapshotKeepsOldGenerationWhenSourceNeverStabilizes(t *testing.T) {
	sourceDir := t.TempDir()
	sqlitePath := filepath.Join(sourceDir, "zotero.sqlite")
	cacheDir := filepath.Join(sourceDir, ".zotero_cli", "snapshot")
	if err := os.WriteFile(sqlitePath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-wal", []byte("wal-one"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, oldPath, err := createOrReuseCachedSnapshot(sqlitePath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath+"-wal", []byte("wal-invalidates-old-generation"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyAttempts := 0
	unstableCopy := func(source, target string) error {
		copyAttempts++
		if err := copySQLiteSnapshotFiles(source, target); err != nil {
			return err
		}
		file, err := os.OpenFile(source+"-wal", os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, writeErr := file.Write([]byte{byte('0' + copyAttempts)})
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	_, fallbackPath, err := createOrReuseCachedSnapshotWithCopy(sqlitePath, cacheDir, unstableCopy)
	if err != nil {
		t.Fatal(err)
	}
	if copyAttempts != snapshotCopyMaxAttempts {
		t.Fatalf("copy attempts=%d want=%d", copyAttempts, snapshotCopyMaxAttempts)
	}
	if fallbackPath != oldPath {
		t.Fatalf("fallback path=%q want old generation %q", fallbackPath, oldPath)
	}
	if isSnapshotValid(sqlitePath, cacheDir) {
		t.Fatal("fallback generation must remain marked stale")
	}
	meta := snapshotReadMetadata(sqlitePath, cacheDir)
	if !meta.SQLiteFallback || !meta.SnapshotStale {
		t.Fatalf("metadata=%+v", meta)
	}
}

func TestWithReadableDBFallsBackToSnapshotWhenQueryHitsBusy(t *testing.T) {
	liveDB, err := sql.Open("sqlite", "file:live-fallback?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer liveDB.Close()

	snapshotDB, err := sql.Open("sqlite", "file:snapshot-fallback?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotDB.Close()

	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	cacheDir := filepath.Join(dataDir, ".zotero_cli", "snapshot")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlitePath, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}

	openDB := func(dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, filepath.ToSlash(cacheDir)) {
			return snapshotDB, nil
		}
		return liveDB, nil
	}

	reader := &LocalReader{
		SQLitePath:       sqlitePath,
		SnapshotCacheDir: cacheDir,
		openSQLiteDB:     openDB,
	}
	attempts := 0
	err = reader.withReadableDB(context.Background(), func(db *sql.DB) error {
		attempts++
		if db == liveDB {
			return errors.New("SQLITE_BUSY: database is locked")
		}
		if db != snapshotDB {
			t.Fatalf("unexpected db pointer %p", db)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withReadableDB() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("withReadableDB() attempts = %d, want 2", attempts)
	}
	meta := reader.ConsumeReadMetadata()
	if meta.ReadSource != "snapshot" || !meta.SQLiteFallback {
		t.Fatalf("ConsumeReadMetadata() = %#v, want snapshot metadata", meta)
	}
}

func TestNewLocalReaderLoadsDataDirAndAttachmentBaseDirFromPrefs(t *testing.T) {
	appData := t.TempDir()
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseAttachmentDir := t.TempDir()
	prefsPath := filepath.Join(appData, "Zotero", "Zotero", "Profiles", "abcd.default", "prefs.js")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	prefsContent := strings.Join([]string{
		`user_pref("extensions.zotero.baseAttachmentPath", "` + strings.ReplaceAll(baseAttachmentDir, `\`, `\\`) + `");`,
		`user_pref("extensions.zotero.dataDir", "` + strings.ReplaceAll(dataDir, `\`, `\\`) + `");`,
		"",
	}, "\n")
	if err := os.WriteFile(prefsPath, []byte(prefsContent), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", t.TempDir())

	reader, err := NewLocalReader(config.Config{})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	if reader.DataDir != dataDir {
		t.Fatalf("reader.DataDir = %q, want %q", reader.DataDir, dataDir)
	}
	if reader.AttachmentBaseDir != baseAttachmentDir {
		t.Fatalf("reader.AttachmentBaseDir = %q, want %q", reader.AttachmentBaseDir, baseAttachmentDir)
	}
}

func TestNewLocalReaderUsesSelfContainedSyncAttachmentDir(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, syncmirror.AttachmentsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "zotero.sqlite"), []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapJSON := `{"version":1,"attachments":{}}`
	if err := os.MkdirAll(filepath.Dir(syncmirror.MapPath(dataDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(syncmirror.MapPath(dataDir), []byte(mapJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	reader, err := NewLocalReader(config.Config{Mode: "local", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dataDir, syncmirror.AttachmentsDir)
	if reader.AttachmentBaseDir != want {
		t.Fatalf("AttachmentBaseDir = %q, want %q", reader.AttachmentBaseDir, want)
	}
}

func TestListSyncLinkedAttachmentsPreservesAttachmentsRelativePath(t *testing.T) {
	dataDir := t.TempDir()
	baseDir := t.TempDir()
	linkedPath := filepath.Join(baseDir, "Q_生物科学", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(linkedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT)`,
		`CREATE TABLE itemAttachments (itemID INTEGER, linkMode INTEGER, path TEXT)`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER)`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT)`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT)`,
		`INSERT INTO items VALUES (1, 'LINK1')`,
		`INSERT INTO itemAttachments VALUES (1, 2, 'attachments:Q_生物科学/paper.pdf')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{
		DataDir: dataDir, SQLitePath: sqlitePath, StorageDir: filepath.Join(dataDir, "storage"),
		AttachmentBaseDir: baseDir, openSQLiteDB: openSQLiteDB, createSnapshot: createSQLiteSnapshot,
	}
	entries, err := reader.ListSyncLinkedAttachments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Available || entries[0].RelativePath != "Q_生物科学/paper.pdf" {
		t.Fatalf("unexpected linked attachments: %#v", entries)
	}
}

func TestListSyncLinkedAttachmentsRejectsNonPortablePathsBeforeFileLookup(t *testing.T) {
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT)`,
		`CREATE TABLE itemAttachments (itemID INTEGER, linkMode INTEGER, path TEXT)`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER)`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT)`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT)`,
		`INSERT INTO items VALUES (1, 'ABS12345')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO itemAttachments VALUES (1, 2, ?)`, `Z:\offline-share\paper.pdf`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{
		DataDir: dataDir, SQLitePath: sqlitePath, StorageDir: filepath.Join(dataDir, "storage"),
		AttachmentBaseDir: t.TempDir(), openSQLiteDB: openSQLiteDB, createSnapshot: createSQLiteSnapshot,
	}
	entries, err := reader.ListSyncLinkedAttachments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Available || entries[0].RelativePath != "" || !strings.Contains(entries[0].Error, `attachments:`) || !strings.Contains(entries[0].Error, `relative path`) {
		t.Fatalf("unexpected absolute-path entry: %#v", entries)
	}

	db, err = sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE itemAttachments SET path = 'attachments:papers/paper.pdf'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reader.AttachmentBaseDir = ""
	entries, err = reader.ListSyncLinkedAttachments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Available || !strings.Contains(entries[0].Error, `base attachment directory is not configured`) {
		t.Fatalf("unexpected missing-base-directory entry: %#v", entries)
	}
}

func TestFindDefaultDataDirFallsBackToHomeZotero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("USERPROFILE", home)

	defaultDir := filepath.Join(home, "Zotero")
	storageDir := filepath.Join(defaultDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(defaultDir, "zotero.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader, err := NewLocalReader(config.Config{})
	if err != nil {
		t.Fatalf("NewLocalReader() error = %v", err)
	}
	if reader.DataDir != defaultDir {
		t.Fatalf("reader.DataDir = %q, want %q (default fallback)", reader.DataDir, defaultDir)
	}
}

func TestFindDefaultDataDirReturnsEmptyWhenNoSQLite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", t.TempDir())
	realHome, _ := os.UserHomeDir()

	result := findDefaultDataDir()
	if result != "" && !strings.HasPrefix(result, realHome) {
		t.Fatalf("findDefaultDataDir() = %q, want empty string (or real home fallback)", result)
	}
}

func TestNewLocalReaderRejectsRelativeDataDir(t *testing.T) {
	if _, err := NewLocalReader(config.Config{Mode: "local", DataDir: "relative/path"}); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("NewLocalReader() error = %v; want absolute-path error", err)
	}
}

func TestResolveAttachmentPathSupportsAttachmentsRelativeBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	relativePath := filepath.Join("papers", "example.pdf")
	absolutePath := filepath.Join(baseDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolutePath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{AttachmentBaseDir: baseDir}
	got, ok := reader.resolveAttachmentPath("ATTACH1", "attachments:papers/example.pdf", "example.pdf")
	if !ok {
		t.Fatal("resolveAttachmentPath() did not resolve attachments: path")
	}
	if got != absolutePath {
		t.Fatalf("resolveAttachmentPath() = %q, want %q", got, absolutePath)
	}
}

func TestResolveAttachmentPathRejectsAttachmentsPathEscape(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{AttachmentBaseDir: baseDir}
	if got, ok := reader.resolveAttachmentPath("ATTACH1", "attachments:../outside.pdf", "outside.pdf"); ok {
		t.Fatalf("escaping attachments path resolved to %q", got)
	}
}

func TestResolveAttachmentPathRejectsStorageFilenameEscape(t *testing.T) {
	root := t.TempDir()
	storageDir := filepath.Join(root, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{StorageDir: storageDir}
	if got, ok := reader.resolveAttachmentPath("ATTACH1", "storage:outside.pdf", "../../outside.pdf"); ok {
		t.Fatalf("escaping storage filename resolved to %q", got)
	}
}

func TestResolveAttachmentPathSupportsAbsolutePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absolute.pdf")
	if err := os.WriteFile(path, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := &LocalReader{}
	got, ok := reader.resolveAttachmentPath("ATTACH1", path, "absolute.pdf")
	if !ok {
		t.Fatal("resolveAttachmentPath() did not resolve absolute path")
	}
	if got != path {
		t.Fatalf("resolveAttachmentPath() = %q, want %q", got, path)
	}
}

func TestResolveAttachmentPathUsesSyncedAttachmentTree(t *testing.T) {
	dataDir := t.TempDir()
	mirrored := filepath.Join(dataDir, syncmirror.AttachmentsDir, "papers", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(mirrored), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mirrored, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &LocalReader{
		DataDir: dataDir,
		AttachmentMirror: map[string]syncmirror.AttachmentEntry{
			"ATTACH1": {Key: "ATTACH1", RelativePath: "attachments/papers/paper.pdf"},
		},
	}
	got, ok := reader.resolveAttachmentPath("ATTACH1", `C:\missing\paper.pdf`, "paper.pdf")
	if !ok || filepath.Clean(got) != filepath.Clean(mirrored) {
		t.Fatalf("resolveAttachmentPath() = %q, %v; want %q, true", got, ok, mirrored)
	}
}

func TestLocalExtractItemFullTextUsesPDFAttachment(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	previous := extractFullTextWithPDFiumFunc
	t.Cleanup(func() { extractFullTextWithPDFiumFunc = previous })
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{
			Text: "full extracted text",
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Extractor:       "pdfium",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}, true, nil
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: filepath.Join(dataDir, ".zotero_cli", "fulltext"),
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT999", ContentType: "text/plain", ResolvedPath: filepath.Join(storageDir, "ATT999", "note.txt"), Resolved: true},
			{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: resolvedPath, Resolved: true},
		},
	}

	text, err := reader.ExtractItemFullText(context.Background(), item)
	if err != nil {
		t.Fatalf("ExtractItemFullText() error = %v", err)
	}
	if text != "full extracted text" {
		t.Fatalf("ExtractItemFullText() = %q, want full extracted text", text)
	}

	meta := reader.ConsumeReadMetadata()
	if meta.FullTextSource != "pdfium" || meta.FullTextAttachmentKey != "ATT123" {
		t.Fatalf("ConsumeReadMetadata() = %#v, want pdfium attachment metadata", meta)
	}
}

func TestLocalExtractItemAttachmentTextsIncludesAllPDFAttachments(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	suppPath := filepath.Join(storageDir, "ATT456", "supplement.pdf")
	if err := os.MkdirAll(filepath.Dir(suppPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suppPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	previous := extractFullTextWithPDFiumFunc
	t.Cleanup(func() { extractFullTextWithPDFiumFunc = previous })
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{
			Text: attachment.Key + " text",
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Extractor:       "pdfium",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}, true, nil
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: filepath.Join(dataDir, ".zotero_cli", "fulltext"),
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: mainPath, Resolved: true},
			{Key: "ATT456", Title: "Supplementary PDF", ContentType: "application/pdf", ResolvedPath: suppPath, Resolved: true},
		},
	}

	result, err := reader.ExtractItemAttachmentTexts(context.Background(), item)
	if err != nil {
		t.Fatalf("ExtractItemAttachmentTexts() error = %v", err)
	}
	if result.Text != "ATT123 text" || result.PrimaryAttachmentKey != "ATT123" {
		t.Fatalf("ExtractItemAttachmentTexts() primary = %#v", result)
	}
	if len(result.Attachments) != 2 {
		t.Fatalf("ExtractItemAttachmentTexts() attachments = %#v, want 2 entries", result.Attachments)
	}
	if result.Attachments[0].Attachment.Key != "ATT123" || result.Attachments[0].Text != "ATT123 text" {
		t.Fatalf("unexpected first attachment result: %#v", result.Attachments[0])
	}
	if result.Attachments[0].ContentPath == "" {
		t.Fatalf("expected cache content path: %#v", result.Attachments[0])
	}
	if _, err := os.Stat(result.Attachments[0].ContentPath); err != nil {
		t.Fatalf("cache content path is not readable: %v", err)
	}
	if result.Attachments[1].Attachment.Key != "ATT456" || result.Attachments[1].Text != "ATT456 text" {
		t.Fatalf("unexpected second attachment result: %#v", result.Attachments[1])
	}

	meta := reader.ConsumeReadMetadata()
	if meta.FullTextSource != "pdfium" || meta.FullTextAttachmentKey != "ATT123" {
		t.Fatalf("ConsumeReadMetadata() = %#v, want primary attachment metadata", meta)
	}
}

func TestFullTextCacheSaveLoadPreservesChunks(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := newFullTextCache(filepath.Join(dataDir, ".zotero_cli", "fulltext"))
	doc := FullTextDocument{
		Text: "page one\npage two",
		Chunks: []chunk{
			{Page: 1, Text: "page one", BlockCount: 1},
			{Page: 2, Text: "page two", BlockCount: 1},
		},
		Meta: fullTextCacheMeta{
			AttachmentKey:   "ATT123",
			ResolvedPath:    resolvedPath,
			ContentType:     "application/pdf",
			Extractor:       "pymupdf",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
		},
	}
	if err := cache.Save(doc); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, ok, err := cache.Load(domain.Attachment{Key: "ATT123", ContentType: "application/pdf", ResolvedPath: resolvedPath, Resolved: true})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() missed fresh cache")
	}
	if len(loaded.Chunks) != 2 || loaded.Chunks[1].Page != 2 || loaded.Chunks[1].Text != "page two" {
		t.Fatalf("Load() chunks = %#v", loaded.Chunks)
	}
}

func TestLocalExtractItemAttachmentPageTextsUsesChunks(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	previous := extractFullTextWithPDFiumFunc
	t.Cleanup(func() { extractFullTextWithPDFiumFunc = previous })
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return FullTextDocument{}, false, nil
		}
		return FullTextDocument{
			Text: "page one text\npage two methods\npage two results",
			Chunks: []chunk{
				{Page: 1, Text: "page one text", BlockCount: 1},
				{Page: 2, Text: "page two methods", BlockCount: 1},
				{Page: 2, Text: "page two results", BlockCount: 1},
			},
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Extractor:       "pdfium",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}, true, nil
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: filepath.Join(dataDir, ".zotero_cli", "fulltext"),
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: resolvedPath, Resolved: true},
		},
	}
	result, err := reader.ExtractItemAttachmentPageTexts(context.Background(), item)
	if err != nil {
		t.Fatalf("ExtractItemAttachmentPageTexts() error = %v", err)
	}
	if result.PrimaryAttachmentKey != "ATT123" || len(result.Attachments) != 1 {
		t.Fatalf("unexpected result metadata: %#v", result)
	}
	pages := result.Attachments[0].Pages
	if len(pages) != 2 || pages[0].Page != 1 || pages[1].Page != 2 {
		t.Fatalf("unexpected pages: %#v", pages)
	}
	if !strings.Contains(pages[1].Text, "page two methods") || !strings.Contains(pages[1].Text, "page two results") {
		t.Fatalf("page 2 text did not combine chunks: %#v", pages[1])
	}
}

func TestLocalExtractItemAttachmentPageTextsRejectsLegacyCacheWithoutChunks(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	resolvedPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolvedPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dataDir, ".zotero_cli", "fulltext")
	cache := newFullTextCache(cacheDir)
	if err := cache.Save(FullTextDocument{
		Text: "legacy cache text without page chunks",
		Meta: fullTextCacheMeta{
			AttachmentKey:   "ATT123",
			ResolvedPath:    resolvedPath,
			ContentType:     "application/pdf",
			Extractor:       "zotero_ft_cache",
			SourceMtimeUnix: info.ModTime().Unix(),
			SourceSize:      info.Size(),
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: cacheDir,
	}
	item := domain.Item{
		Key: "ITEM123",
		Attachments: []domain.Attachment{
			{Key: "ATT123", Title: "Paper PDF", ContentType: "application/pdf", ResolvedPath: resolvedPath, Resolved: true},
		},
	}
	_, err = reader.ExtractItemAttachmentPageTexts(context.Background(), item)
	if err == nil || !strings.Contains(err.Error(), "page-aware full-text cache") {
		t.Fatalf("expected page-aware cache error, got %v", err)
	}
}

func TestLocalExtractItemAttachmentTextsPrefersMainPDFOverSupplement(t *testing.T) {
	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	suppPath := filepath.Join(storageDir, "ATT456", "supplement.pdf")
	if err := os.MkdirAll(filepath.Dir(suppPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suppPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(storageDir, "ATT123", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	previous := extractFullTextWithPDFiumFunc
	t.Cleanup(func() { extractFullTextWithPDFiumFunc = previous })
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (FullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return FullTextDocument{}, false, nil
		}
		text := "Abstract\nHybrid speciation via inheritance of alternate alleles."
		if attachment.Key == "ATT456" {
			text = "Supplementary Information\nAdditional methods and figures."
		}
		return FullTextDocument{
			Text: text,
			Meta: fullTextCacheMeta{
				AttachmentKey:   attachment.Key,
				ResolvedPath:    sourcePath,
				ContentType:     attachment.ContentType,
				Extractor:       "pdfium",
				SourceMtimeUnix: info.ModTime().Unix(),
				SourceSize:      info.Size(),
			},
		}, true, nil
	}

	reader := &LocalReader{
		DataDir:          dataDir,
		StorageDir:       storageDir,
		FullTextCacheDir: filepath.Join(dataDir, ".zotero_cli", "fulltext"),
	}
	item := domain.Item{
		Key:   "ITEM123",
		Title: "Hybrid speciation via inheritance of alternate alleles",
		Attachments: []domain.Attachment{
			{Key: "ATT456", Title: "Supplementary Information", Filename: "supplement.pdf", ContentType: "application/pdf", ResolvedPath: suppPath, Resolved: true},
			{Key: "ATT123", Title: "Main Article PDF", Filename: "paper.pdf", ContentType: "application/pdf", ResolvedPath: mainPath, Resolved: true},
		},
	}

	result, err := reader.ExtractItemAttachmentTexts(context.Background(), item)
	if err != nil {
		t.Fatalf("ExtractItemAttachmentTexts() error = %v", err)
	}
	if result.PrimaryAttachmentKey != "ATT123" {
		t.Fatalf("ExtractItemAttachmentTexts() primary = %q, want ATT123", result.PrimaryAttachmentKey)
	}

	meta := reader.ConsumeReadMetadata()
	if meta.FullTextAttachmentKey != "ATT123" {
		t.Fatalf("ConsumeReadMetadata() = %#v, want ATT123", meta)
	}
}
