package cli

const usageOpen = `usage: zot open <item-key> [--page N]

What: Open the item's PDF in the system default viewer. With --page N, jumps
to that page after opening.

Modes:
  local/hybrid    Opens the file directly from local Zotero storage.
  remote          Downloads the PDF to a temp file first (slower).
  web             Not supported (no local PDF).

Notes:
  - On Windows/macOS the OS-default PDF viewer is launched.
  - --page requires the viewer to support a #page= fragment; some viewers
    ignore it. local mode is more reliable for page jumps.
  - See also: select <key> (highlights in Zotero UI), extract-text.`
