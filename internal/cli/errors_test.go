package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestRunUnknownCommandReturnsUsageError(t *testing.T) {
	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"wat"})

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command: wat") {
		t.Fatalf("expected unknown command message, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage text on stdout, got %q", stdout.String())
	}
}

func TestHelpIncludesDeleteWarnings(t *testing.T) {
	stdout, stderr := captureOutput(t)
	exitCode := Run([]string{"help"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Delete Warnings:") {
		t.Fatalf("expected delete warning section, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "If you are an agent or automation tool, stop and think before deleting anything.") {
		t.Fatalf("expected agent warning, got %q", stdout.String())
	}
}

func TestSubcommandHelpSkipsConfigLoading(t *testing.T) {
	testCases := []struct {
		name      string
		args      []string
		wantUsage string
	}{
		{name: "export help", args: []string{"export", "--help"}, wantUsage: usageExport},
		{name: "stats help", args: []string{"stats", "--help"}, wantUsage: usageStats},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configRoot := t.TempDir()
			setTestConfigDir(t, configRoot)

			stdout, stderr := captureOutput(t)
			exitCode := Run(tc.args)

			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d; stderr=%q", exitCode, stderr.String())
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("expected empty stderr, got %q", got)
			}
			if !strings.Contains(stdout.String(), tc.wantUsage) {
				t.Fatalf("expected usage %q in stdout, got %q", tc.wantUsage, stdout.String())
			}
		})
	}
}

func TestRunCommandsReturnConfigErrorWhenConfigMissing(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "find", args: []string{"find", "attention"}},
		{name: "config validate", args: []string{"config", "validate"}},
		{name: "show", args: []string{"show", "X42A7DEE"}},
		{name: "relate", args: []string{"relate", "X42A7DEE"}},
		{name: "cite", args: []string{"cite", "X42A7DEE"}},
		{name: "export query", args: []string{"export", "attention"}},
		{name: "export item key", args: []string{"export", "--item-key", "X42A7DEE"}},
		{name: "collections", args: []string{"collections"}},
		{name: "notes", args: []string{"notes"}},
		{name: "tags", args: []string{"tags"}},
		{name: "searches", args: []string{"searches"}},
		{name: "deleted", args: []string{"deleted"}},
		{name: "stats", args: []string{"stats"}},
		{name: "versions", args: []string{"versions", "items", "--since", "1"}},
		{name: "item types", args: []string{"item-types"}},
		{name: "item fields", args: []string{"item-fields"}},
		{name: "creator fields", args: []string{"creator-fields"}},
		{name: "item type fields", args: []string{"item-type-fields", "book"}},
		{name: "item type creator types", args: []string{"item-type-creator-types", "book"}},
		{name: "item template", args: []string{"item-template", "book"}},
		{name: "groups", args: []string{"groups"}},
		{name: "trash", args: []string{"trash"}},
		{name: "collections top", args: []string{"collections-top"}},
		{name: "publications", args: []string{"publications"}},
		{name: "create item", args: []string{"create-item", "--data", `{"itemType":"book"}`, "--if-unmodified-since-version", "1"}},
		{name: "update item", args: []string{"update-item", "ABCD2345", "--data", `{"title":"Updated"}`, "--if-unmodified-since-version", "1"}},
		{name: "delete item", args: []string{"delete-item", "ABCD2345", "--if-unmodified-since-version", "1"}},
		{name: "create items", args: []string{"create-items", "--data", `[{"itemType":"book"}]`, "--if-unmodified-since-version", "1"}},
		{name: "update items", args: []string{"update-items", "--data", `[{"key":"ABCD2345","version":1,"title":"Updated"}]`}},
		{name: "delete items", args: []string{"delete-items", "--items", "ABCD2345,EFGH6789"}},
		{name: "add tag", args: []string{"add-tag", "--items", "ABCD2345", "--tag", "ai"}},
		{name: "remove tag", args: []string{"remove-tag", "--items", "ABCD2345", "--tag", "ai"}},
		{name: "create collection", args: []string{"create-collection", "--data", `{"name":"New Collection"}`, "--if-unmodified-since-version", "1"}},
		{name: "update collection", args: []string{"update-collection", "COLL1234", "--data", `{"key":"COLL1234","version":1,"name":"Renamed"}`}},
		{name: "delete collection", args: []string{"delete-collection", "COLL1234", "--if-unmodified-since-version", "1"}},
		{name: "create search", args: []string{"create-search", "--data", `{"name":"Unread PDFs"}`, "--if-unmodified-since-version", "1"}},
		{name: "update search", args: []string{"update-search", "SCH12345", "--data", `{"key":"SCH12345","version":1,"name":"Renamed Search"}`}},
		{name: "delete search", args: []string{"delete-search", "SCH12345", "--if-unmodified-since-version", "1"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configRoot := t.TempDir()
			setTestConfigDir(t, configRoot)

			_, stderr := captureOutput(t)
			exitCode := Run(tc.args)

			if exitCode != 3 {
				t.Fatalf("expected exit code 3, got %d; stderr=%q", exitCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), "config not found.") || !strings.Contains(stderr.String(), "run `zot config init`") {
				t.Fatalf("expected config-not-found message, got %q", stderr.String())
			}
		})
	}
}

func TestRunShowRejectsExtraPositionalArgs(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "X42A7DEE", "extra"})

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), usageShow) {
		t.Fatalf("expected usage message, got %q", stderr.String())
	}
}

func TestRunRelateRejectsExtraPositionalArgs(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	serverURL, cleanup := newTestAPI(t)
	defer cleanup()
	t.Setenv("ZOT_BASE_URL", serverURL)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"relate", "X42A7DEE", "extra"})

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), usageRelate) {
		t.Fatalf("expected usage message, got %q", stderr.String())
	}
}

func TestRunArgumentValidationReturnsUsageError(t *testing.T) {
	testCases := []struct {
		name       string
		args       []string
		wantUsage  string
		wantStderr string
	}{
		{
			name:      "find missing query",
			args:      []string{"find"},
			wantUsage: usageFind,
		},
		{
			name:       "cite invalid format",
			args:       []string{"cite", "X42A7DEE", "--format", "bad"},
			wantUsage:  usageCite,
			wantStderr: "error: unsupported format",
		},
		{
			name:       "export conflicting args",
			args:       []string{"export", "mixed", "--item-key", "X42A7DEE"},
			wantUsage:  usageExport,
			wantStderr: "error: cannot combine query, --item-key, and --collection",
		},
		{
			name:       "export conflicting collection args",
			args:       []string{"export", "--collection", "COLL1234", "--item-key", "X42A7DEE"},
			wantUsage:  usageExport,
			wantStderr: "error: cannot combine query, --item-key, and --collection",
		},
		{
			name:       "export invalid format",
			args:       []string{"export", "--item-key", "X42A7DEE", "--format", "bad"},
			wantUsage:  usageExport,
			wantStderr: "error: unsupported format",
		},
		{
			name:      "collections extra arg",
			args:      []string{"collections", "extra"},
			wantUsage: usageCollections,
		},
		{
			name:      "notes extra arg",
			args:      []string{"notes", "extra"},
			wantUsage: usageNotes,
		},
		{
			name:      "tags extra arg",
			args:      []string{"tags", "extra"},
			wantUsage: usageTags,
		},
		{
			name:      "searches extra arg",
			args:      []string{"searches", "extra"},
			wantUsage: usageSearches,
		},
		{
			name:      "deleted extra arg",
			args:      []string{"deleted", "extra"},
			wantUsage: usageDeleted,
		},
		{
			name:      "stats extra arg",
			args:      []string{"stats", "extra"},
			wantUsage: usageStats,
		},
		{
			name:       "versions missing since",
			args:       []string{"versions", "items"},
			wantUsage:  usageVersions,
			wantStderr: "error: missing value for --since",
		},
		{
			name:       "versions invalid object",
			args:       []string{"versions", "bad", "--since", "1"},
			wantUsage:  usageVersions,
			wantStderr: "error: unsupported object type",
		},
		{
			name:       "versions invalid if-modified-since-version",
			args:       []string{"versions", "items", "--since", "1", "--if-modified-since-version", "-1"},
			wantUsage:  usageVersions,
			wantStderr: "error: invalid value for --if-modified-since-version",
		},
		{
			name:      "item type fields missing type",
			args:      []string{"item-type-fields"},
			wantUsage: usageItemTypeFields,
		},
		{
			name:      "item type creator types missing type",
			args:      []string{"item-type-creator-types"},
			wantUsage: usageItemTypeCreatorTypes,
		},
		{
			name:      "item template missing type",
			args:      []string{"item-template"},
			wantUsage: usageItemTemplate,
		},
		{
			name:      "key info missing key",
			args:      []string{"key-info"},
			wantUsage: usageKeyInfo,
		},
		{
			name:      "groups extra arg",
			args:      []string{"groups", "extra"},
			wantUsage: usageGroups,
		},
		{
			name:       "find invalid qmode",
			args:       []string{"find", "test", "--qmode", "bad"},
			wantUsage:  usageFind,
			wantStderr: "error: invalid value for --qmode",
		},
		{
			name:       "find invalid include fields",
			args:       []string{"find", "test", "--include-fields", "url,bad"},
			wantUsage:  usageFind,
			wantStderr: "error: invalid value for --include-fields: bad",
		},
		{
			name:      "trash extra arg",
			args:      []string{"trash", "extra"},
			wantUsage: usageTrash,
		},
		{
			name:      "collections top extra arg",
			args:      []string{"collections-top", "extra"},
			wantUsage: usageCollectionsTop,
		},
		{
			name:      "publications extra arg",
			args:      []string{"publications", "extra"},
			wantUsage: usagePublications,
		},
		{
			name:      "create item missing data",
			args:      []string{"create-item"},
			wantUsage: usageCreateItem,
		},
		{
			name:       "create item conflicting data sources",
			args:       []string{"create-item", "--data", `{"itemType":"book"}`, "--from-file", "item.json", "--if-unmodified-since-version", "1"},
			wantUsage:  usageCreateItem,
			wantStderr: "error: cannot combine --data and --from-file",
		},
		{
			name:       "create item missing file",
			args:       []string{"create-item", "--from-file", "missing.json", "--if-unmodified-since-version", "1"},
			wantUsage:  usageCreateItem,
			wantStderr: "error: could not read --from-file",
		},
		{
			name:       "create item invalid json",
			args:       []string{"create-item", "--data", `{"itemType":`, "--if-unmodified-since-version", "1"},
			wantUsage:  usageCreateItem,
			wantStderr: "error: invalid JSON payload",
		},
		{
			name:      "update item missing args",
			args:      []string{"update-item", "ABCD2345"},
			wantUsage: usageUpdateItem,
		},
		{
			name:       "update item missing version",
			args:       []string{"update-item", "ABCD2345", "--data", `{"title":"Updated"}`},
			wantUsage:  usageUpdateItem,
			wantStderr: "error: missing value for --if-unmodified-since-version",
		},
		{
			name:      "delete item missing version",
			args:      []string{"delete-item", "ABCD2345"},
			wantUsage: usageDeleteItem,
		},
		{
			name:      "create items missing data",
			args:      []string{"create-items"},
			wantUsage: usageCreateItems,
		},
		{
			name:       "create items invalid json",
			args:       []string{"create-items", "--data", `[`, "--if-unmodified-since-version", "1"},
			wantUsage:  usageCreateItems,
			wantStderr: "error: invalid JSON payload",
		},
		{
			name:      "update items missing data",
			args:      []string{"update-items"},
			wantUsage: usageUpdateItems,
		},
		{
			name:      "delete items missing keys",
			args:      []string{"delete-items"},
			wantUsage: usageDeleteItems,
		},
		{
			name:      "add tag missing tag",
			args:      []string{"add-tag", "--items", "ITEMA001"},
			wantUsage: usageAddTag,
		},
		{
			name:      "remove tag missing items",
			args:      []string{"remove-tag", "--tag", "ai"},
			wantUsage: usageRemoveTag,
		},
		{
			name:      "create collection missing data",
			args:      []string{"create-collection"},
			wantUsage: usageCreateCollection,
		},
		{
			name:      "update collection missing args",
			args:      []string{"update-collection", "COLL1234"},
			wantUsage: usageUpdateCollection,
		},
		{
			name:      "delete collection missing version",
			args:      []string{"delete-collection", "COLL1234"},
			wantUsage: usageDeleteCollection,
		},
		{
			name:      "create search missing data",
			args:      []string{"create-search"},
			wantUsage: usageCreateSearch,
		},
		{
			name:      "update search missing args",
			args:      []string{"update-search", "SCH12345"},
			wantUsage: usageUpdateSearch,
		},
		{
			name:      "delete search missing version",
			args:      []string{"delete-search", "SCH12345"},
			wantUsage: usageDeleteSearch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configRoot := t.TempDir()
			setTestConfigDir(t, configRoot)
			writeTestConfig(t, configRoot)

			_, stderr := captureOutput(t)
			exitCode := Run(tc.args)

			if exitCode != 2 {
				t.Fatalf("expected exit code 2, got %d; stderr=%q", exitCode, stderr.String())
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("expected %q in stderr, got %q", tc.wantStderr, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantUsage) {
				t.Fatalf("expected usage %q in stderr, got %q", tc.wantUsage, stderr.String())
			}
		})
	}
}

func TestRunShowSurfacesReadableNotFoundError(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	server := newErrorAPI(t, http.StatusNotFound, "")
	defer server.cleanup()
	t.Setenv("ZOT_BASE_URL", server.url)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"show", "MISSING"})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "zotero api not found (404)") {
		t.Fatalf("expected readable 404 message, got %q", stderr.String())
	}
}

func TestRunFindSurfacesRetryAfterOnRateLimit(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	server := newErrorAPI(t, http.StatusTooManyRequests, "5")
	defer server.cleanup()
	t.Setenv("ZOT_BASE_URL", server.url)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"find", "attention"})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "zotero api rate limited (429): retry after 5s") {
		t.Fatalf("expected readable 429 message, got %q", stderr.String())
	}
}

func TestRunUpdateItemSurfacesReadablePreconditionError(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	server := newErrorAPIWithBody(t, http.StatusPreconditionFailed, "", "library version advanced to 88")
	defer server.cleanup()
	t.Setenv("ZOT_BASE_URL", server.url)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"update-item", "ABCD2345", "--data", `{"title":"Updated"}`, "--if-unmodified-since-version", "7"})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "zotero api precondition failed (412): library version changed; refresh and retry") {
		t.Fatalf("expected readable 412 message, got %q", stderr.String())
	}
}

func TestRunCreateItemSurfacesReadableConflictError(t *testing.T) {
	configRoot := t.TempDir()
	setTestConfigDir(t, configRoot)
	writeTestConfig(t, configRoot)

	server := newErrorAPIWithBody(t, http.StatusConflict, "", "collection key already exists")
	defer server.cleanup()
	t.Setenv("ZOT_BASE_URL", server.url)

	_, stderr := captureOutput(t)
	exitCode := Run([]string{"create-item", "--data", `{"itemType":"book"}`, "--if-unmodified-since-version", "41"})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "zotero api conflict (409): request conflicts with existing data") {
		t.Fatalf("expected readable 409 message, got %q", stderr.String())
	}
}
