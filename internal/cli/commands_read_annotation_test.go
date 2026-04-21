package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"

	_ "modernc.org/sqlite"
)

// --- Annotation tests (TDD: write failing tests first) ---

func buildLocalAnnotationFixture(t *testing.T, sqlitePath string, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE itemTypes (itemTypeID INTEGER PRIMARY KEY, typeName TEXT);`,
		`CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, version INTEGER, itemTypeID INTEGER, dateAdded TEXT);`,
		`CREATE TABLE fieldsCombined (fieldID INTEGER PRIMARY KEY, fieldName TEXT);`,
		`CREATE TABLE itemDataValues (valueID INTEGER PRIMARY KEY, value TEXT);`,
		`CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);`,
		`CREATE TABLE creators (creatorID INTEGER PRIMARY KEY, firstName TEXT, lastName TEXT, fieldMode INT);`,
		`CREATE TABLE creatorTypes (creatorTypeID INTEGER PRIMARY KEY, creatorType TEXT);`,
		`CREATE TABLE itemCreators (itemID INTEGER, creatorID INTEGER, creatorTypeID INTEGER, orderIndex INTEGER);`,
		`CREATE TABLE tags (tagID INTEGER PRIMARY KEY, name TEXT);`,
		`CREATE TABLE itemTags (itemID INTEGER, tagID INTEGER);`,
		`CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, key TEXT, collectionName TEXT);`,
		`CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);`,
		`CREATE TABLE itemAttachments (itemID INTEGER, parentItemID INTEGER, contentType TEXT, linkMode INTEGER, path TEXT);`,
		`CREATE TABLE itemNotes (itemID INTEGER, parentItemID INTEGER, note TEXT, title TEXT);`,
		`CREATE TABLE itemAnnotations (
			itemID INTEGER PRIMARY KEY,
			parentItemID INT NOT NULL,
			type INT NOT NULL,
			authorName TEXT,
			text TEXT,
			comment TEXT,
			color TEXT,
			pageLabel TEXT,
			sortIndex TEXT NOT NULL,
			position TEXT NOT NULL,
			isExternal INT NOT NULL
		);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	inserts := []string{
		// itemTypes
		`INSERT INTO itemTypes(itemTypeID, typeName) VALUES (1, 'journalArticle'), (2, 'attachment'), (3, 'annotation');`,
		// items: 1=article, 2=PDF attachment, 3=note
		`INSERT INTO items(itemID, key, version, itemTypeID) VALUES (1, 'ITEM1234', 7, 1), (2, 'ATTACHPDF', 1, 2), (3, 'NOTE1234', 1, 2), (10, 'ANNO1', 1, 3), (11, 'ANNO2', 1, 3), (12, 'ANNO3', 1, 3), (13, 'ANNO4', 1, 3);`,
		// fields + data for article title
		`INSERT INTO fieldsCombined(fieldID, fieldName) VALUES (1, 'title');`,
		`INSERT INTO itemDataValues(valueID, value) VALUES (1, 'Annotated Paper');`,
		`INSERT INTO itemData(itemID, fieldID, valueID) VALUES (1, 1, 1);`,
		// creator
		`INSERT INTO creators(creatorID, firstName, lastName, fieldMode) VALUES (1, 'Jane', 'Doe', 0);`,
		`INSERT INTO creatorTypes(creatorTypeID, creatorType) VALUES (1, 'author');`,
		`INSERT INTO itemCreators(itemID, creatorID, creatorTypeID, orderIndex) VALUES (1, 1, 1, 0);`,
		// tag
		`INSERT INTO tags(tagID, name) VALUES (1, 'genomics');`,
		`INSERT INTO itemTags(itemID, tagID) VALUES (1, 1);`,
		// collection
		`INSERT INTO collections(collectionID, key, collectionName) VALUES (1, 'COLL1', 'Research');`,
		`INSERT INTO collectionItems(collectionID, itemID) VALUES (1, 1);`,
		// PDF attachment (parentItemID=1 means child of article)
		`INSERT INTO itemAttachments(itemID, parentItemID, contentType, linkMode, path) VALUES (2, 1, 'application/pdf', 0, 'storage:paper.pdf');`,
		// note
		`INSERT INTO itemNotes(itemID, parentItemID, note, title) VALUES (3, 1, '<p>A regular note</p>', 'Note');`,
		// === Annotations on the PDF attachment (parentItemID = 2, the PDF's itemID) ===
		// Annotation 1: highlight with comment on page 2
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (10, 2, 1, '', 'Key finding about genome assembly', 'This is important for the discussion section', '#ffd400', '2', '00001|00000|00000', '{"pageIndex":1,"rects":[{"left":100,"top":200,"right":500,"bottom":250}]}', 0)`,
		// Annotation 2: note-only on page 3
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (11, 2, 1, '', 'Another highlighted passage', 'Need to verify this claim', '#ff6666', '3', '00002|00000|00000', '{"pageIndex":2,"rects":[{"left":50,"top":300,"right":450,"bottom":330}]}', 0)`,
		// Annotation 3: highlight without comment on page 5
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (12, 2, 0, '', 'A pure highlight text', '', '#5c9eff', '5', '00003|00000|00000', '{"pageIndex":4,"rects":[{"left":200,"top":100,"right":600,"bottom":140}]}', 0)`,
		// Annotation 4: ink/drawing on page 1
		`INSERT INTO itemAnnotations(itemID, parentItemID, type, authorName, text, comment, color, pageLabel, sortIndex, position, isExternal) VALUES (13, 2, 3, '', '', 'A hand-drawn sketch note', '#000000', '1', '00004|00000|00000', '{"pageIndex":0}', 0)`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	// Create storage dir with a dummy PDF so attachment resolves
	attachmentDir := filepath.Join(storageDir, "ATTACHPDF")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "paper.pdf"), []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAnnotationsReturnsAllTypes(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	reader, err := backend.NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error: %v", err)
	}

	// Load the article item which has a PDF attachment with annotations
	item, err := reader.GetItem(context.Background(), "ITEM1234")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}

	// Verify annotations are loaded on the item
	if len(item.Annotations) == 0 {
		t.Fatalf("expected annotations on ITEM1234, got 0")
	}

	// Should have exactly 4 annotations
	if len(item.Annotations) != 4 {
		t.Fatalf("expected 4 annotations, got %d", len(item.Annotations))
	}

	// Annotation 1: note type with text+comment
	a0 := item.Annotations[0]
	if a0.Key != "10" { // itemIDs are auto-generated in this fixture
		// Key comes from items table, let's just check content
	}
	// Find by text to verify regardless of key ordering
	foundHighlightNote := false
	foundPureHighlight := false
	foundInk := false
	for _, a := range item.Annotations {
		if a.Text == "Key finding about genome assembly" {
			foundHighlightNote = true
			if a.Type != "note" {
				t.Errorf("expected type=note for highlight-note annotation, got %s", a.Type)
			}
			if a.Comment != "This is important for the discussion section" {
				t.Errorf("unexpected comment: %q", a.Comment)
			}
			if a.Color != "#ffd400" {
				t.Errorf("unexpected color: %q", a.Color)
			}
			if a.PageLabel != "2" {
				t.Errorf("unexpected pageLabel: %q", a.PageLabel)
			}
			if a.PageIndex != 1 {
				t.Errorf("expected pageIndex=1, got %d", a.PageIndex)
			}
			if a.IsExternal {
				t.Error("expected IsExternal=false")
			}
		}
		if a.Text == "A pure highlight text" {
			foundPureHighlight = true
			if a.Type != "highlight" {
				t.Errorf("expected type=highlight, got %s", a.Type)
			}
			if a.Comment != "" {
				t.Errorf("expected empty comment for pure highlight, got %q", a.Comment)
			}
			if a.Color != "#5c9eff" {
				t.Errorf("unexpected color: %q", a.Color)
			}
			if a.PageLabel != "5" {
				t.Errorf("unexpected pageLabel: %q", a.PageLabel)
			}
		}
		if a.Type == "ink" {
			foundInk = true
			if a.Comment != "A hand-drawn sketch note" {
				t.Errorf("unexpected ink comment: %q", a.Comment)
			}
		}
	}
	if !foundHighlightNote {
		t.Error("could not find the highlight-note annotation")
	}
	if !foundPureHighlight {
		t.Error("could not find the pure highlight annotation")
	}
	if !foundInk {
		t.Error("could not find the ink annotation")
	}
}

func TestLoadAnnotationsEmptyWhenNoneExist(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalShowFixture(t, sqlitePath, storageDir) // standard fixture has no real annotations
	t.Setenv("ZOT_DATA_DIR", dataDir)

	reader, err := backend.NewLocalReader(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewLocalReader() error: %v", err)
	}

	item, err := reader.GetItem(context.Background(), "ITEM1234")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}

	if len(item.Annotations) != 0 {
		t.Fatalf("expected 0 annotations for item without annotations, got %d", len(item.Annotations))
	}
}

func TestShowLocalJSONIncludesAnnotations(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234", "--json"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be an object, got %#v", got["data"])
	}

	annos, ok := data["annotations"].([]any)
	if !ok {
		t.Fatalf("expected annotations array in show output, got %#v", data["annotations"])
	}
	if len(annos) != 4 {
		t.Fatalf("expected 4 annotations, got %d", len(annos))
	}

	// Verify first annotation structure
	first := annos[0].(map[string]any)
	requiredFields := []string{"key", "type", "text", "comment", "color", "page_label", "page_index", "position", "is_external"}
	existingKeys := make([]string, 0, len(first))
	for k := range first {
		existingKeys = append(existingKeys, k)
	}
	for _, f := range requiredFields {
		if _, exists := first[f]; !exists {
			t.Errorf("annotation missing required field %q; available keys: %v", f, existingKeys)
		}
	}
}

func TestShowLocalTextOutputIncludesAnnotations(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)
	t.Setenv("ZOT_MODE", "local")

	dataDir := t.TempDir()
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.Mkdir(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	buildLocalAnnotationFixture(t, sqlitePath, storageDir)
	t.Setenv("ZOT_DATA_DIR", dataDir)

	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "ITEM1234"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Annotations: 4",
		"color=#ffd400",
		"page=2",
		"Key finding about genome assembly",
		"This is important for the discussion section",
		"color=#5c9eff",
		"A pure highlight text",
		"page=5",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in show text output, got:\n%s", want, got)
		}
	}
}
