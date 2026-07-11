package cli

const usageAnnotate = `usage: zot annotate <item-key> (--text TEXT | --page N (--rect x0,y0,x1,y2 | --point x,y)) [--color COLOR] [--comment TEXT] [--type TYPE] [--dry-run] [--clear] [--author AUTHOR] [--json]
       zot annotate [<default-item-key>] --from-file PATH [--dry-run] [--json]

What: Add highlights/underlines/notes to a PDF. Three modes:

  Mode 1     --text TEXT
             Plain text search. Zotero locates the first match on the
             page (auto-resolved). Works without geometry knowledge.
             zot annotate KEY --text "GATK" --color yellow

  Mode 1.5   --text TEXT --page N       (recommended)
             Text + page constraint. Faster + more precise than Mode 1.
             zot annotate KEY --page 4 --text "GATK" --color red

  Mode 2     --page N --rect/--point
             Pure geometry. No text match. For image-only or exact regions.
             zot annotate KEY --page 4 --rect 100,200,400,250 --color blue

Options:
  --color       yellow | red | blue | green | magenta | cyan | orange
  --type        highlight (default) | underline | note | image
  --comment     Sticky note text (when --type=note)
  --author      Annotation author (defaults to current Zotero user)
  --dry-run     Preview matched pages/rectangles without writing the PDF.
  --from-file   Batch annotations from a JSON array. Each entry accepts
                item_key, text, page, rect, point, color, comment, type,
                and dry_run. item_key may be omitted when <default-item-key>
                is provided on the command line.
  --clear       Remove existing annotations. By default clears both API and
                local DB; the DB layer is only removable when Zotero is closed.
                Combine with --type / --page to scope.

Examples:
  zot annotate ABCD --page 4 --text "GATK" --color yellow --json
  zot annotate ABCD --page 4 --text "GATK" --dry-run --json
  zot annotate ABCD --from-file annotations.json --dry-run --json
  zot annotate ABCD --page 1 --rect 50,100,300,150 --color red --json
  zot annotate ABCD --type note --page 5 --comment "TODO" --json
  zot annotate ABCD --clear --page 4 --type highlight

Notes:
  - Requires ZOT_ALLOW_WRITE=1 in env.
  - In local/hybrid mode annotations are written to the local SQLite when
    Zotero is not running (~50ms), else via the Web API (~2s).
  - See also: annotations (read back), extract-text (read PDF body).`
