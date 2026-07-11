package cli

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
