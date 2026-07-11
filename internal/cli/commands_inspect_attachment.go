package cli

const usageInspectAttachment = `usage: zot inspect-attachment <attachment-key> [--health] [--sheet NAME] [--head N] [--max-sheets N] [--max-columns N] [--json]
       zot inspect-attachment --item <item-key> [--health] [--head N] [--max-sheets N] [--max-columns N] [--json]

What: Preview local spreadsheet attachments. The command reads local .xlsx files
in streaming mode and returns sheet names, dimensions, likely header rows, and
small row previews. With --health, it also reports local attachment path/name
issues and can inspect non-spreadsheet attachments. It does not modify files.

Output:
  --json          Structured output for agents.
  --sheet NAME    Inspect one sheet in one attachment.
  --head N        Preview N non-empty rows per inspected sheet (default 5).
  --max-sheets N  Inspect at most N sheets per workbook unless --sheet is set
                  (default 5).
  --max-columns N Preview at most N cells from each row while keeping the real
                  column count in metadata (default 12).
  --item KEY      Inspect all local .xlsx attachments under one Zotero item.
  --health        Include attachment health diagnostics. With --item, scans all
                  attachments under the item instead of only spreadsheets.

Examples:
  zot inspect-attachment ATT123 --json
  zot inspect-attachment ATT123 --sheet "Table S1" --head 20 --json
  zot inspect-attachment --item ITEM123 --json
  zot inspect-attachment --item ITEM123 --health --json

Notes:
  - Requires local/hybrid mode with resolved local attachment files.
  - Currently supports .xlsx/.xlsm workbooks. Use supplements first to discover
    candidate attachment keys.`
