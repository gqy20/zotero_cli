package cli

const usageIndex = `usage: zot index build [--force] [--workers N] [--json]

What: Build the FTS5 full-text index over local PDF text. Required for
'zot find --fulltext' and 'zot find --snippet' to expand search to PDF
content (local/hybrid mode only).

Options:
  --force            Rebuild from scratch (default: incremental).
  --workers N        Parallel workers. Default 4. Capped at 20.
  --json             Structured progress + result.

Notes:
  - Requires local/hybrid mode and PyMuPDF. FTS5 is also used by 'find
    --fulltext' and 'show --snippet' once available.
  - First build on a large library can take minutes; rerun with --force
    to refresh after big PDF additions.`
