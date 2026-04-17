package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)
	stdin  = io.Reader(os.Stdin)

	version   = "0.0.3"
	commit    = "unknown"
	buildDate = "unknown"
)

func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version":
		printVersion()
		return 0
	case "config":
		return runConfig(args[1:])
	case "find":
		return runFind(args[1:])
	case "show":
		return runShow(args[1:])
	case "extract-text":
		return runExtractText(args[1:])
	case "relate":
		return runRelate(args[1:])
	case "cite":
		return runCite(args[1:])
	case "export":
		return runExport(args[1:])
	case "collections":
		return runCollections(args[1:])
	case "notes":
		return runNotes(args[1:])
	case "tags":
		return runTags(args[1:])
	case "searches":
		return runSearches(args[1:])
	case "deleted":
		return runDeleted(args[1:])
	case "stats":
		return runStats(args[1:])
	case "versions":
		return runVersions(args[1:])
	case "item-types":
		return runItemTypes(args[1:])
	case "item-fields":
		return runItemFields(args[1:])
	case "creator-fields":
		return runCreatorFields(args[1:])
	case "item-type-fields":
		return runItemTypeFields(args[1:])
	case "item-type-creator-types":
		return runItemTypeCreatorTypes(args[1:])
	case "item-template":
		return runItemTemplate(args[1:])
	case "key-info":
		return runKeyInfo(args[1:])
	case "groups":
		return runGroups(args[1:])
	case "trash":
		return runTrash(args[1:])
	case "collections-top":
		return runCollectionsTop(args[1:])
	case "publications":
		return runPublications(args[1:])
	case "create-item":
		return runCreateItem(args[1:])
	case "update-item":
		return runUpdateItem(args[1:])
	case "delete-item":
		return runDeleteItem(args[1:])
	case "create-items":
		return runCreateItems(args[1:])
	case "update-items":
		return runUpdateItems(args[1:])
	case "delete-items":
		return runDeleteItems(args[1:])
	case "add-tag":
		return runAddTag(args[1:])
	case "remove-tag":
		return runRemoveTag(args[1:])
	case "create-collection":
		return runCreateCollection(args[1:])
	case "update-collection":
		return runUpdateCollection(args[1:])
	case "delete-collection":
		return runDeleteCollection(args[1:])
	case "create-search":
		return runCreateSearch(args[1:])
	case "update-search":
		return runUpdateSearch(args[1:])
	case "delete-search":
		return runDeleteSearch(args[1:])
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(stdout, `%s is a minimal Zotero CLI.

Usage:
  %s <command>

Commands:
  version        Show CLI version
  config path    Print config path
  config init    Interactively create ~/.zot/.env
  config show    Show active config with masked secrets
	config validate  Validate library_id and api_key against Zotero
	find           Search items in the configured Zotero library
	show           Show item details
  extract-text   Extract text from local PDF attachments
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
  item-types     List Zotero item types
  item-fields    List Zotero item fields
  creator-fields List Zotero creator fields
  item-type-fields          List valid fields for an item type
  item-type-creator-types   List valid creator types for an item type
  item-template             Show template for a new item type
  key-info      Show the owner and privileges for an API key
  groups        List groups accessible to a user
  trash         List items currently in the trash
  collections-top  List only top-level collections
  publications     List items in My Publications
  create-item   Create a new item from JSON data
  update-item   Update an existing item from JSON data
  delete-item   Delete an item using a version precondition
  create-items  Create multiple items from a JSON array
  update-items  Update multiple items from a JSON array
  delete-items  Delete multiple items by key
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

func printVersion() {
	fmt.Fprintf(stdout, "zot %s\n", version)
	fmt.Fprintf(stdout, "commit: %s\n", commit)
	fmt.Fprintf(stdout, "built: %s\n", buildDate)
}

func printConfigUsage() {
	fmt.Fprint(stdout, `Usage:
  zot config path
  zot config init
  zot config init --example
  zot config show
  zot config validate
`)
}

func printErr(err error) int {
	fmt.Fprintln(stderr, "error:", err)
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

func printCommandUsage(usage string) int {
	fmt.Fprintln(stdout, usage)
	return 0
}
