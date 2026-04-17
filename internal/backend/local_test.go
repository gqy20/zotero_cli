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

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"

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

	doc := fullTextDocument{
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
	doc := fullTextDocument{
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
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (fullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return fullTextDocument{}, false, nil
		}
		return fullTextDocument{
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
	text := "Preface words. Mixed survey full text preview from zotero cache. Core section discusses speciation genome patterns in plants and gene flow. Ending notes."
	got := buildFullTextSnippet(text, "speciation genome")
	if !strings.Contains(got, "speciation genome patterns in plants") {
		t.Fatalf("buildFullTextSnippet() = %q, want centered match", got)
	}
	if strings.Contains(got, "Preface words.") && strings.Contains(got, "Ending notes.") {
		t.Fatalf("buildFullTextSnippet() = %q, want trimmed snippet instead of full preview", got)
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
	doc := fullTextDocument{
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

	matches, err := cache.Search("speciation genome", false, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() matches = %#v, want 1 match", matches)
	}
	if matches[0].ParentItemKey != "ITEM123" || matches[0].AttachmentKey != "ATT123" {
		t.Fatalf("Search() match = %#v, want ITEM123/ATT123", matches[0])
	}

	matches, err = cache.Search("genome supplement", false, 10)
	if err != nil {
		t.Fatalf("Search() field query error = %v", err)
	}
	if len(matches) != 1 || matches[0].AttachmentKey != "ATT123" {
		t.Fatalf("Search() field query matches = %#v, want ATT123", matches)
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
		doc := fullTextDocument{
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

	matches, err := cache.Search("hybridization speciation", false, 10)
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

func TestFullTextCacheSearchPrefersExactTitlePhrase(t *testing.T) {
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
		doc := fullTextDocument{
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

	matches, err := cache.Search("Plant extinction in the anthropocene", false, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("Search() returned no matches")
	}
	if matches[0].ParentItemKey != "ITEM123" {
		t.Fatalf("Search() top match = %#v, want ITEM123 exact title hit first", matches[0])
	}
}

func TestFullTextCacheSearchPrefersBalancedTokenCoverage(t *testing.T) {
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
		doc := fullTextDocument{
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

	matches, err := cache.Search("speciation genome", false, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("Search() returned no matches")
	}
	if matches[0].ParentItemKey != "ITEM123" {
		t.Fatalf("Search() top match = %#v, want ITEM123 balanced two-token hit first", matches[0])
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
	if pragmas[0] != "busy_timeout=5000" && pragmas[1] != "busy_timeout=5000" {
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

	previousOpen := openSQLiteDBFunc
	previousSnapshot := createSQLiteSnapshotFunc
	t.Cleanup(func() {
		openSQLiteDBFunc = previousOpen
		createSQLiteSnapshotFunc = previousSnapshot
	})

	openSQLiteDBFunc = func(dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "snapshot.sqlite") {
			return snapshotDB, nil
		}
		return liveDB, nil
	}
	createSQLiteSnapshotFunc = func(string) (string, string, error) {
		snapshotDir := t.TempDir()
		return snapshotDir, filepath.Join(snapshotDir, "snapshot.sqlite"), nil
	}

	reader := &LocalReader{SQLitePath: filepath.Join(t.TempDir(), "zotero.sqlite")}
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
	extractFullTextWithPDFiumFunc = func(_ context.Context, _ *LocalReader, attachment domain.Attachment) (fullTextDocument, bool, error) {
		sourcePath, info, ok := fullTextAttachmentSourceInfo(attachment)
		if !ok {
			return fullTextDocument{}, false, nil
		}
		return fullTextDocument{
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
