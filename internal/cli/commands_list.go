package cli

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

const (
	usageCollections = "usage: zot collections [--limit N] [--json]\n\nList all collections (with parent nesting and item counts). Pairs with collections-top (root only)."
	usageNotes       = "usage: zot notes [--query QUERY] [--limit N] [--json]\n\nList standalone notes (not attached as item children). Use 'zot show <key>' to read the notes attached to one item. See also: create-item (add a note)."
	usageTags        = "usage: zot tags [--limit N] [--json]\n\nList all tags with item counts. Pairs with find --tag TAG / find --tag-contains W."
	usageSearches    = "usage: zot searches [--limit N] [--json]\n\nList saved searches owned by the library. Pairs with create-search / update-search / delete-search."
	usageDeleted     = "usage: zot deleted [--json]\n\nList keys of items/collections/searches deleted since a recent sync. Use the keys to inspect history, not to undelete."
	usageChanges     = "usage: zot changes <collections|searches|items|items-top> --since N [--include-trashed] [--if-modified-since-version N] [--json]\n\nDelta sync: return objects changed since library version N. Pass 'items-top' to skip child notes/attachments. Use with versions polling to detect remote edits."
	usageSchema      = `usage: zot schema <subcommand> [args] [--json]

Introspect Zotero metadata schema.

Subcommands:
  types                     List all Zotero item types
  fields                    List all Zotero item fields
  creator-types             List all Zotero creator fields (roles)
  fields-for <type>         List valid fields for a specific item type
  creator-types-for <type>  List valid creator roles for a specific item type
  template <type>           Show JSON template for creating a new item

Examples:
  zot schema types                          # List all item types
  zot schema types --json                   # JSON output
  zot schema fields-for journalArticle      # Fields for journal articles
  zot schema template book --json           # Template for a new book
  zot schema creator-types-for artwork      # Creator roles for artwork
`
	usageItemTypes            = "usage: zot schema types [--json]"
	usageItemFields           = "usage: zot schema fields [--json]"
	usageCreatorFields        = "usage: zot schema creator-types [--json]"
	usageItemTypeFields       = "usage: zot schema fields-for <item-type> [--json]"
	usageItemTypeCreatorTypes = "usage: zot schema creator-types-for <item-type> [--json]"
	usageItemTemplate         = "usage: zot schema template <item-type> [--json]"
	usageKeyInfo              = "usage: zot key-info <api-key> [--json]\n\nLook up the owner username and privilege flags of an API key. Use to debug 403 / 404 errors or to verify write/delete permission before destructive operations."
	usageGroups               = "usage: zot groups [--json]\n\nList Zotero groups the current API key has access to. Use a group's numeric ID with ZOT_LIBRARY_TYPE=group to query its items."
	usageTrash                = "usage: zot trash [--limit N] [--json]\n\nList items currently in the library trash. trash is best-effort recovery; 'delete-item' is permanent."
	usageCollectionsTop       = "usage: zot collections-top [--json]\n\nList only top-level (root) collections. Use 'collections' for the full nested tree."
	usagePublications         = "usage: zot publications [--limit N] [--json]\n\nList items in 'My Publications' (Zotero's user-facing publication subset of your library)."
	usageOverview             = `usage: zot overview [--json]

One-shot library overview: stats, top collections, top tags, recent items,
and FTS index status in a single parallel call (~6s).

Examples:
  zot overview                          # Text summary
  zot overview --json                     # Structured for agents

Notes:
  - Prefer this over calling find + stats + tags separately — it runs
    four sub-queries in parallel and returns one envelope.`
)

func (c *CLI) runSchema(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(lookupCommand("schema").Long)
	}

	if len(args) == 0 {
		fmt.Fprintln(c.stderr, lookupCommand("schema").Long)
		return ExitUsage
	}

	sub, ok := schemaSubUsages[args[0]]
	if !ok {
		fmt.Fprintf(c.stderr, "unknown schema subcommand: %s\n", args[0])
		fmt.Fprintln(c.stderr, lookupCommand("schema").Long)
		return ExitUsage
	}

	switch args[0] {
	case "types":
		return c.runItemTypes(args[1:])
	case "fields":
		return c.runItemFields(args[1:])
	case "creator-types":
		return c.runCreatorFields(args[1:])
	case "fields-for":
		if len(args) < 2 {
			fmt.Fprintln(c.stderr, sub)
			return ExitUsage
		}
		return c.runItemTypeFields(args[1:])
	case "creator-types-for":
		if len(args) < 2 {
			fmt.Fprintln(c.stderr, sub)
			return ExitUsage
		}
		return c.runItemTypeCreatorTypes(args[1:])
	case "template":
		if len(args) < 2 {
			fmt.Fprintln(c.stderr, sub)
			return ExitUsage
		}
		return c.runItemTemplate(args[1:])
	}
	return 0
}

func (c *CLI) runItemTypes(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageItemTypes)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypes(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-types", values, jsonOutput)
}

func (c *CLI) runItemFields(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageItemFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemFields(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-fields", values, jsonOutput)
}

func (c *CLI) runCreatorFields(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageCreatorFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListCreatorFields(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("creator-fields", values, jsonOutput)
}

func (c *CLI) runItemTypeFields(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTypeFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeFields(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-type-fields", values, jsonOutput)
}

func (c *CLI) runItemTypeCreatorTypes(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTypeCreatorTypes)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeCreatorTypes(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-type-creator-types", values, jsonOutput)
}

func (c *CLI) runItemTemplate(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTemplate)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	template, err := client.GetItemTemplate(context.Background(), itemType)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "item-template",
			Data:    template,
		})
	}

	return c.writeJSON(jsonResponse{OK: true, Command: "item-template", Data: template})
}

func (c *CLI) runKeyInfo(args []string) int {
	key, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageKeyInfo)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	if strings.TrimSpace(key) == "" {
		key = cfg.APIKey
	}

	info, err := client.GetKeyInfo(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "key-info",
			Data:    info,
		})
	}

	fmt.Fprintf(c.stdout, "user_id=%d\n", info.UserID)
	if len(info.Access) > 0 {
		return c.writeJSON(jsonResponse{OK: true, Command: "key-info", Data: info.Access})
	}
	return 0
}

func (c *CLI) renderLocalizedValues(command string, values []zoteroapi.LocalizedValue, jsonOutput bool) int {
	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: command,
			Data:    values,
			Meta: map[string]any{
				"total": len(values),
			},
		})
	}

	for _, value := range values {
		fmt.Fprintf(c.stdout, "%-18s  %s\n", value.ID, value.Localized)
	}
	return 0
}
