package cli

const usageAnnotations = `usage: zot annotations <item-key> [--json] [--clear] [--page N] [--type TYPE] [--author AUTHOR]

What: List or clear PDF annotations on an item. Reads from local SQLite
(local/hybrid) or PDF directly (web/remote). Returns highlights, underlines,
and sticky notes with page, color, author, and text.

Options:
  --page N           Restrict to one page.
  --type TYPE        highlight | underline | note | image.
  --author NAME      Filter by author.
  --json             Structured output for agents (recommended).
  --clear            Remove annotations matching the filter (--page, --type).
                     IMPORTANT: --clear operates in two layers:
                       1. Remote API layer (always removable).
                       2. Local DB layer — only removable when Zotero is closed
                          (DB is locked while Zotero holds the write lock).
                     Without --page/--type, --clear removes ALL annotations on
                     the item. Combine filters to scope.

Examples:
  zot annotations ABCD --json
  zot annotations ABCD --page 4 --type highlight --json
  zot annotations ABCD --clear --page 4 --type highlight
  zot annotations ABCD --clear --type note --author "Alice"

Notes:
  - The --clear operation is permanent; there is no undo.
  - Requires ZOT_ALLOW_WRITE=1 in env when using --clear.
  - See also: annotate (add new annotations), extract-text (read PDF body).`
