package cli

const usageSupplements = `usage: zot supplements <item-key> [--json] [--online]
       zot supplements --all [--json] [--limit N]

What: Find supplementary/data attachments. By default this inspects local Zotero
attachments and resolved local paths. With --online, it also checks public
publisher/repository metadata without logging in or downloading files.

Output:
  --json       Structured output for agents.
  --all        Scan all local library items.
  --limit N    Limit emitted supplement records after scanning.
  --online     For one item, query public Zenodo, Figshare, and
               Nature/Springer pages for downloadable supplement metadata.

Examples:
  zot supplements ABCD --json
  zot supplements ABCD --online --json
  zot supplements --all --json --limit 50

Notes:
  - Local discovery requires local/hybrid mode with local Zotero data.
  - Online discovery supports public pages/records only; private/login-gated
    content is reported as blocked or partial.
  - --online is intentionally not supported with --all in the first version.
  - Classification uses attachment title, filename, path, content type, and
    extension. Parent item titles are not used as supplement evidence.
  - See also: show KEY --full --json, extract-text.`
