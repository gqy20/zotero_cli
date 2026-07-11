package cli

import (
	"strings"

	"zotero_cli/internal/domain"
)

const (
	usageExtractText = `usage: zot extract-text <item-key>|--all [--json] [--output-dir DIR] [--pages RANGE] [--max-chars N] [--grep TEXT] [--attachment KEY]

What: Extract plain text from a PDF attachment. Returns the concatenated text
content of all pages. Results are cached to disk (keyed by item key + file
mtime), so repeat calls are near-instant.

Output controls:
  --output-dir DIR Write Markdown files to DIR instead of printing text.
  --all            Extract all local items with PDF attachments to Markdown files.
  --pages RANGE      Return only selected PDF pages, e.g. 3 or 2,5-7.
  --max-chars N      Return at most N characters of text (applies per field).
  --grep TEXT        Return only lines containing TEXT (case-insensitive).
  --attachment KEY   Return text for one attachment key.

Modes:
  local/hybrid    Read PDF from local Zotero storage. Requires PyMuPDF.
  remote          Server reads PDF and returns text.
  web             Not supported (no local PDF to read).

Examples:
  zot extract-text ABCD --json
  zot extract-text ABCD --json --pages 3-8
  zot extract-text ABCD --json --max-chars 12000
  zot extract-text ABCD --json --grep methods --attachment ATT123
  zot extract-text ABCD -o ./markdown
  zot extract-text --all -o ./markdown --json

Notes:
  - Requires PyMuPDF; install via 'zot init --pdf' or 'pip install pymupdf'.
  - For snippet-level search use 'zot find ... --fulltext --snippet N'.
  - See also: extract-figures, find --has-pdf, annotations.`

	usageExtractFigures = `usage: zot extract-figures <item-key> [...]|--all [--output-dir DIR] [--json] [--workers N]

What: Extract scientific figures (charts, plots, diagrams) from PDF attachments
as PNG files. Filters cover pages, logos, and author headshots by default.

Options:
  --all                Extract all local items with PDF attachments.
  --output-dir DIR      Where to write PNGs. Default: {ZOT_DATA_DIR}/.zotero_cli/figures
                        (auto-created). Override with this flag.
  --workers N           Parallel workers. Default: CPU count (min 2, max 8).
  --json                Return JSON {key, page, file, ...} instead of writing.

Advanced:
  --max-per-page N      Stop after N figures per page to bound output (default 25).

Modes:
  local/hybrid    Reads from local Zotero storage. Requires PyMuPDF.
  remote          Server extracts via PyMuPDF (same backend).
  web             Not supported.

Examples:
  zot extract-figures ABCD --json
  zot extract-figures ABC1 ABC2 -o ./figs --workers 8 --json
  zot extract-figures --all -o ./figs --workers 8 --json

Notes:
  - Results are cached on disk; rerun skips already-extracted pages.
  - Multi-item runs sort by page count (longest first) for better parallelism.
  - Requires PyMuPDF. See 'zot init --pdf'.
  - See also: extract-text, annotations, open <key> (view in Zotero).`
)

func filterPDFAttachments(attachments []domain.Attachment) []domain.Attachment {
	pdfs := make([]domain.Attachment, 0)
	for _, attachment := range attachments {
		if strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			pdfs = append(pdfs, attachment)
		}
	}
	return pdfs
}
