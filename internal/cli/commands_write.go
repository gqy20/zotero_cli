package cli

import (
	"context"
	"fmt"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

const (
	usageCreateItem = `usage: zot create-item (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]

What: Add a new item to the library. The most common case: attach a note to an existing item.

Payload (for itemType=note, the only local-write path):
  itemType     "note"        required
  parentItem   <item-key>    required; links to existing parent item
  note         <html>        required; supports h1/p/ul/blockquote/em/code

Payload (for other item types):
  itemType     <type>        required; see 'zot schema types' for valid values
  ...                          other fields depend on type; use 'zot schema template <type>' for shape

Notes:
  - Hybrid routing: in local/hybrid mode + Zotero not running + itemType=note → SQLite direct write (~50ms, "write_source":"local"); else Web API (~2s).
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - --if-unmodified-since-version N is the optimistic lock; fetch N from find --json.
  - See also: notes (list), update-item (edit), schema template <type> (generate JSON).`

	usageUpdateItem = `usage: zot update-item <item-key> (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]

What: Edit an existing item. Pass only the fields you want to change; others are preserved.

Payload (JSON, partial update):
  <field>      <value>        any writable field for the item type
  ...                          use 'zot schema fields-for <type>' to list valid fields

Notes:
  - Web API only (no local-write path for update).
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - --if-unmodified-since-version N is the optimistic lock.
  - See also: create-item, show <key> (inspect current values).`

	usageDeleteItem = `usage: zot delete-item <item-key> --if-unmodified-since-version N [--json] [-y|--yes]

What: Permanently delete one item. Cannot be undone.

Payload (none; --if-unmodified-since-version is the safety lock):
  --if-unmodified-since-version N   Required. Reject the deletion if the library version
                                     has advanced past N. Fetch current N from
                                     'find --json | jq '.version''.

Notes:
  - IRREVERSIBLE. Zotero trash recovery is best-effort and short-lived.
  - Default prompts y/N on stderr; -y skips (use with caution; never in agent automation).
  - Requires ZOT_ALLOW_DELETE=1 in env.
  - To edit instead of destroy, use update-item.`

	usageAddTag = `usage: zot add-tag --items KEY1,KEY2 --tag TAG [--if-unmodified-since-version N] [--json]

What: Add a tag to multiple items in one call (atomic at the library version level).

Payload (none; --tag is the operation):
  --tag TAG                Tag name (no # prefix; tags are stored without it)
  --items KEY1,KEY2,...    Comma-separated item keys

Notes:
  - Fetches current item versions internally; --if-unmodified-since-version is optional here
    but recommended for atomic batch updates.
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - See also: remove-tag, find --tag TAG.`

	usageRemoveTag = `usage: zot remove-tag --items KEY1,KEY2 --tag TAG [--if-unmodified-since-version N] [--json]

What: Remove a tag from multiple items in one call.

Payload (none; --tag is the operation):
  --tag TAG                Tag name to remove
  --items KEY1,KEY2,...    Comma-separated item keys

Notes:
  - Idempotent: removing a non-existent tag is a no-op.
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - See also: add-tag.`

	usageCreateCollection = `usage: zot create-collection (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]

What: Create a new collection. Returns the new collection key.

Payload:
  name        string       required; collection display name
  parentCollection   <key> optional; nest under existing collection
  ...                     other fields optional; see Zotero API docs

Notes:
  - Web API only.
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - See also: create-item, collections (list).`

	usageUpdateCollection = `usage: zot update-collection <collection-key> (--data JSON | --from-file PATH) [--if-unmodified-since-version N] [--json]

What: Edit a collection's name or parent. Pass only the fields you want to change.

Payload (JSON, partial update):
  name        string       optional; new display name
  parentCollection <key>  optional; new parent (set to false to move to top level)

Notes:
  - Web API only.
  - Requires ZOT_ALLOW_WRITE=1 in env.`

	usageDeleteCollection = `usage: zot delete-collection <collection-key> --if-unmodified-since-version N [--json] [-y|--yes]

What: Permanently delete a collection. Items inside are NOT deleted — they move to "Unfiled".

Notes:
  - IRREVERSIBLE. Items remain; only the collection container is destroyed.
  - Default prompts y/N; -y skips.
  - Requires ZOT_ALLOW_DELETE=1 in env.`

	usageCreateSearch = `usage: zot create-search (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]

What: Create a saved search. The Zotero API uses a Zotero Search Condition expression in 'conditions'.

Payload:
  name        string       required
  conditions  [...]        required; see Zotero Search Condition syntax
  ...                     optional fields

Notes:
  - Web API only.
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - See also: searches (list), find (interactive search).`

	usageUpdateSearch = `usage: zot update-search <search-key> (--data JSON | --from-file PATH) [--if-unmodified-since-version N] [--json]

What: Edit a saved search's name or conditions.

Notes:
  - Web API only.
  - Requires ZOT_ALLOW_WRITE=1 in env.`

	usageDeleteSearch = `usage: zot delete-search <search-key> --if-unmodified-since-version N [--json] [-y|--yes]

What: Permanently delete a saved search. Items matched by it are NOT affected.

Notes:
  - IRREVERSIBLE. Items matched by the search are NOT affected.
  - Default prompts y/N; -y skips.
  - Requires ZOT_ALLOW_DELETE=1 in env.`
)

func (c *CLI) runCreateItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateItem)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	// Hybrid write: if Zotero is not running, create note via local SQLite
	if (cfg.Mode == "local" || cfg.Mode == "hybrid") && !isZoteroRunning() {
		itemType, _ := data["itemType"].(string)
		if itemType == "note" {
			localResult, localErr := createNoteLocally(cfg, data)
			if localErr != nil {
				fmt.Fprintf(c.stderr, "local write failed, falling back to web API: %v\n", localErr)
			} else {
				if jsonOutput {
					return c.writeJSON(jsonResponse{OK: true, Command: "create-item", Data: map[string]any{
						"key":                   localResult.Key,
						"last_modified_version": localResult.ItemID,
						"write_source":          "local",
					}})
				}
				fmt.Fprintf(c.stdout, "created item %s locally (SQLite)\n", localResult.Key)
				return 0
			}
		}
	}

	result, err := client.CreateItem(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "created item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateItem)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, true)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateItem(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteItem)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE item %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteItem(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runAddTag(args []string) int {
	return c.runUpdateTags(args, usageAddTag, "add-tag", true)
}

func (c *CLI) runRemoveTag(args []string) int {
	return c.runUpdateTags(args, usageRemoveTag, "remove-tag", false)
}

func (c *CLI) runUpdateTags(args []string, usage string, command string, add bool) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usage)
	}
	keys, version, tag, jsonOutput, err := parseKeysListArgs(args, false, true)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usage)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	items, err := client.GetItemsByKeys(context.Background(), keys)
	if err != nil {
		return c.printErr(err)
	}

	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		updatedTags := mutateTags(item.Tags, tag, add)
		payload = append(payload, map[string]any{
			"key":     item.Key,
			"version": item.Version,
			"tags":    toAPITags(updatedTags),
		})
	}

	result, err := client.UpdateItems(context.Background(), payload, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: command, Data: result})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Fprintf(c.stdout, "%s tag %q on %d items at library version %d\n", action, tag, len(keys), result.LastModifiedVersion)
	return 0
}

func (c *CLI) runCreateCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateCollection)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateCollection(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "created collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateCollection)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateCollection(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteCollection)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE collection %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteCollection(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runCreateSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateSearch)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateSearch(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "created search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateSearch)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateSearch(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteSearch)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE search %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteSearch(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func createNoteLocally(cfg config.Config, data map[string]any) (backend.LocalCreateNoteResult, error) {
	parentKey, _ := data["parentItem"].(string)
	noteHTML, _ := data["note"].(string)
	if parentKey == "" || noteHTML == "" {
		return backend.LocalCreateNoteResult{}, fmt.Errorf("parentItem and note fields are required for local note creation")
	}

	localReader, err := backend.NewLocalReader(cfg)
	if err != nil {
		return backend.LocalCreateNoteResult{}, err
	}

	return localReader.CreateLocalNote(context.Background(), parentKey, noteHTML)
}
