package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type CLI struct {
	stdout           io.Writer
	stderr           io.Writer
	stdin            io.Reader
	backendNewReader func(config.Config, *http.Client) (backend.Reader, error)
	newLocalReader   func(config.Config) (backend.Reader, error)
	citeCache        map[string]zoteroapi.CitationResult
	citeCacheMu      sync.RWMutex
}

var (
	version   = "0.0.5"
	commit    = "unknown"
	buildDate = "unknown"
)

func New() *CLI {
	return &CLI{
		stdout:           os.Stdout,
		stderr:           os.Stderr,
		stdin:            os.Stdin,
		backendNewReader: backend.NewReader,
		newLocalReader: func(cfg config.Config) (backend.Reader, error) {
			return backend.NewLocalReader(cfg)
		},
		citeCache: make(map[string]zoteroapi.CitationResult),
	}
}

func (c *CLI) Run(args []string) int {
	if len(args) == 0 {
		c.printUsage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		c.printUsage()
		return 0
	case "version":
		c.printVersion()
		return 0
	case "config":
		return c.runConfig(args[1:])
	case "find":
		return c.runFind(args[1:])
	case "show":
		return c.runShow(args[1:])
	case "extract-text":
		return c.runExtractText(args[1:])
	case "annotate":
		return c.runAnnotate(args[1:])
	case "open":
		return c.runOpen(args[1:])
	case "select":
		return c.runSelect(args[1:])
	case "annotations":
		return c.runAnnotations(args[1:])
	case "relate":
		return c.runRelate(args[1:])
	case "cite":
		return c.runCite(args[1:])
	case "export":
		return c.runExport(args[1:])
	case "collections":
		return c.runCollections(args[1:])
	case "notes":
		return c.runNotes(args[1:])
	case "tags":
		return c.runTags(args[1:])
	case "searches":
		return c.runSearches(args[1:])
	case "deleted":
		return c.runDeleted(args[1:])
	case "stats":
		return c.runStats(args[1:])
	case "versions":
		return c.runVersions(args[1:])
	case "schema":
		return c.runSchema(args[1:])
	case "key-info":
		return c.runKeyInfo(args[1:])
	case "groups":
		return c.runGroups(args[1:])
	case "trash":
		return c.runTrash(args[1:])
	case "collections-top":
		return c.runCollectionsTop(args[1:])
	case "publications":
		return c.runPublications(args[1:])
	case "create-item":
		return c.runCreateItem(args[1:])
	case "update-item":
		return c.runUpdateItem(args[1:])
	case "delete-item":
		return c.runDeleteItem(args[1:])
	case "add-tag":
		return c.runAddTag(args[1:])
	case "remove-tag":
		return c.runRemoveTag(args[1:])
	case "create-collection":
		return c.runCreateCollection(args[1:])
	case "update-collection":
		return c.runUpdateCollection(args[1:])
	case "delete-collection":
		return c.runDeleteCollection(args[1:])
	case "create-search":
		return c.runCreateSearch(args[1:])
	case "update-search":
		return c.runUpdateSearch(args[1:])
	case "delete-search":
		return c.runDeleteSearch(args[1:])
	case "init":
		return c.runInit(args[1:])
	case "index":
		return c.runIndex(args[1:])
	default:
		fmt.Fprintf(c.stderr, "unknown command: %s\n\n", args[0])
		c.printUsage()
		return ExitUsage
	}
}

func (c *CLI) printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(c.stdout, `%s is a minimal Zotero CLI.

Usage:
  %s <command>

Commands:
  version        Show CLI version
  init           Initialize ~/.zot/.env (streamlined setup with mode selection)
  config path    Print config path
  config show    Show active config with masked secrets
	config validate  Validate library_id and api_key against Zotero
	find           Search items in the configured Zotero library
	show           Show item details
  extract-text   Extract text from local PDF attachments
  annotate       Annotate a PDF attachment with highlights/underlines
  open           Open a PDF attachment in the default viewer
  select         Select an item in the Zotero UI
  annotations    List PDF annotations (highlights, notes, underlines)
  index          Build or manage full-text search index
	relate         Show explicit item relations
	cite           Generate a citation or bibliography entry
  export         Export bibliography entries
  collections    List collections
  notes          List notes
  tags           List tags
  searches       List saved searches
  deleted        Show deleted object keys
  stats          Show library item, collection, and search counts
  versions       Show changed object versions since a library version
  schema         Introspect Zotero metadata schema (types, fields, templates)
  key-info       Show the owner and privileges for an API key
  groups        List groups accessible to a user
  trash         List items currently in the trash
  collections-top  List only top-level collections
  publications     List items in My Publications
  create-item   Create a new item from JSON data
  update-item   Update an existing item from JSON data
  delete-item   Delete an item using a version precondition
  add-tag       Add a tag to multiple items
  remove-tag    Remove a tag from multiple items
  create-collection  Create a collection from JSON data
  update-collection  Update a collection from JSON data
  delete-collection  Delete a collection using a version precondition
  create-search  Create a saved search from JSON data
  update-search  Update a saved search from JSON data
  delete-search  Delete a saved search using a version precondition

Modes (set via ZOT_MODE env):
  web      (default)  Cloud-only via Zotero Web API; no local Zotero needed
  local               Read from local Zotero SQLite (requires ZOT_DATA_DIR)
  hybrid              Local-first with Web API fallback for unsupported features

Environment (run 'zot config show' for full list):
  ZOT_MODE         Operating mode: web | local | hybrid   (default: web)
  ZOT_API_KEY      Zotero Web API key
  ZOT_LIBRARY_ID   Numeric user or group library ID
  ZOT_LIBRARY_TYPE Library type: user | group            (default: user)

Delete Warnings:
  Delete commands are destructive. Review the target key, library, and version carefully before running them.
  If you are an agent or automation tool, stop and think before deleting anything.
  Prefer asking the user to confirm the exact object to delete when there is any ambiguity.
`, exe, exe)
}

func (c *CLI) printVersion() {
	fmt.Fprintf(c.stdout, "zot %s\n", version)
	fmt.Fprintf(c.stdout, "commit: %s\n", commit)
	fmt.Fprintf(c.stdout, "built: %s\n", buildDate)
}

func (c *CLI) printConfigUsage() {
	fmt.Fprint(c.stdout, `Usage:
  zot config path
  zot config show
  zot config validate
`)
}

func (c *CLI) printErr(err error) int {
	fmt.Fprintln(c.stderr, "error:", err)
	return 1
}

func isHelpOnly(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func (c *CLI) printCommandUsage(usage string) int {
	fmt.Fprintln(c.stdout, usage)
	return 0
}
