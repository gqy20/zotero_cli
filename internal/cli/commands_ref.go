package cli

const usageRef = `usage: zot ref <item-key> [--source auto|pmc|pubmed] [--refresh] [--json]
       zot ref show <item-key> [options]
       zot ref build [--workers N] [--force] [--limit N] [--source SOURCE] [--json]
       zot ref status [--json]
       zot ref failed [--json]
       zot ref unsupported [--json]
       zot ref retry [--workers N] [--json]
       zot ref resolve [--workers N] [--json]
	   zot ref cited-by <item-key> [--external] [--limit N] [--json]
       zot ref contexts <item-key> [--json]
       zot ref contexts build [--workers N] [--limit N] [--refresh] [--json]
       zot ref grobid <status|build> [options]
	   zot ref search <query> [--contexts|--references|--metadata] [--field FIELD] [filters]
	   zot ref related <item-key> [--limit N] [--also-viewed] [--refresh] [--json]
	   zot ref links <item-key> [--refresh] [--json]
	   zot ref annotations <item-key> [--refresh] [--json]
	   zot ref profile <item-key> [--refresh] [--json]

What: Manage the local structured-reference index. The officially supported
core is NCBI: prefer complete PMC JATS, otherwise use PubMed reference links
plus batched metadata and Europe PMC reference supplementation. Europe PMC
also provides opt-in external citations, annotations, links, and profiles.
GROBID is an experimental PDF fallback, not part of the default build route.

Subcommands:
  show ITEMKEY  Fetch one item and persist it in the local reference index.
  build         Incrementally index every eligible top-level library item.
  status        Show index coverage and reference counts.
  failed        List failed items and their last errors.
  unsupported   List items outside the supported NCBI coverage.
  retry         Retry all currently failed items.
  resolve       Match indexed references back to local Zotero items.
  cited-by      List indexed library items that cite one local item.
  contexts      Show or backfill PMC JATS citation contexts.
  grobid        EXPERIMENTAL: check or run the optional PDF fallback.
  search        Search references, contexts, PubMed metadata, and annotations.
  related       List PubMed Similar Articles in official rank order.
  links         Merge linked NCBI and Europe PMC biological resources.
  annotations   Show Europe PMC text-mined entities and relationships.
  profile       Show Europe PMC versions, evaluations, funding, and OA status.

Options:
  --source auto|pmc|pubmed  Select NCBI routing (default auto).
  --refresh                 Bypass response and index caches for one item.
  --workers N               Build workers (default 3, maximum 16).
  --force                   Reprocess items even when their fingerprint is fresh.
  --limit N                 Process at most N pending items (testing/staged runs).
  --json                    Structured output for agents.

Examples:
  zot ref ABCD1234 --json
  zot ref build --workers 3 --json
  zot ref status --json
  zot ref retry --workers 2 --json`
