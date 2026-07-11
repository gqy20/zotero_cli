package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/references"
)

const usageRef = `usage: zot ref <item-key> [--source auto|pmc|pubmed] [--refresh] [--json]
       zot ref show <item-key> [options]
       zot ref build [--workers N] [--force] [--limit N] [--source SOURCE] [--json]
       zot ref status [--json]
       zot ref failed [--json]
       zot ref unsupported [--json]
       zot ref retry [--workers N] [--json]
       zot ref resolve [--workers N] [--json]
       zot ref cited-by <item-key> [--json]
       zot ref contexts <item-key> [--json]
       zot ref contexts build [--workers N] [--limit N] [--refresh] [--json]
       zot ref grobid <status|build> [options]
       zot ref search <query> [--contexts|--references] [filters]

What: Manage the local structured-reference index. The officially supported
core is NCBI: prefer complete PMC JATS, otherwise use PubMed reference links
plus batched metadata. GROBID is an experimental, opt-in PDF fallback only;
it is not part of the default build route.

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
  search        Search structured references and citation contexts.

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

type refCommonOptions struct {
	source  string
	json    bool
	force   bool
	limit   int
	workers int
}

func (c *CLI) runRef(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageRef)
	}
	if len(args) == 0 {
		return c.refUsageError("missing item key or subcommand")
	}
	switch args[0] {
	case "show":
		return c.runRefShow(args[1:])
	case "build":
		return c.runRefBuild(args[1:], false)
	case "status":
		return c.runRefStatus(args[1:])
	case "failed":
		return c.runRefFailed(args[1:])
	case "unsupported":
		return c.runRefUnsupported(args[1:])
	case "retry":
		return c.runRefBuild(args[1:], true)
	case "resolve":
		return c.runRefResolve(args[1:])
	case "cited-by":
		return c.runRefCitedBy(args[1:])
	case "contexts":
		return c.runRefContexts(args[1:])
	case "grobid":
		return c.runRefGrobid(args[1:])
	case "search":
		return c.runRefSearch(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return c.refUsageError("missing item key or subcommand")
		}
		return c.runRefShow(args)
	}
}

func (c *CLI) runRefShow(args []string) int {
	itemKey := ""
	opts := refCommonOptions{source: "auto"}
	refresh := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.json = true
		case "--refresh":
			refresh = true
		case "--source":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --source")
			}
			i++
			opts.source = strings.ToLower(args[i])
		default:
			if strings.HasPrefix(args[i], "-") {
				return c.refUsageError("unknown flag: " + args[i])
			}
			if itemKey != "" {
				return c.refUsageError("ref show accepts exactly one item key")
			}
			itemKey = args[i]
		}
	}
	if itemKey == "" {
		return c.refUsageError("missing item key")
	}
	if !validRefSource(opts.source) {
		return c.refUsageError("invalid value for --source")
	}
	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	item, err := reader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	if !refresh && opts.source == "auto" {
		if stored, ok, loadErr := store.LoadResult(context.Background(), itemKey); loadErr != nil {
			return c.printErr(loadErr)
		} else if ok {
			stored.CacheHits = 1
			return c.renderRefResult(stored, opts.json, true)
		}
	}
	service := references.NewService(newNCBIClient(cfg))
	result, err := service.References(context.Background(), item, references.Options{Source: opts.source, Refresh: refresh})
	if err != nil {
		return c.printErr(err)
	}
	if err := store.SaveResult(context.Background(), result, references.Fingerprint(item)); err != nil {
		return c.printErr(err)
	}
	return c.renderRefResult(result, opts.json, false)
}

func (c *CLI) runRefBuild(args []string, retryOnly bool) int {
	opts := refCommonOptions{source: "auto", workers: 3}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.json = true
		case "--force":
			opts.force = true
		case "--source":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --source")
			}
			i++
			opts.source = strings.ToLower(args[i])
		case "--workers":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --workers")
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 1 || value > 16 {
				return c.refUsageError("invalid value for --workers")
			}
			opts.workers = value
		case "--limit":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --limit")
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 1 {
				return c.refUsageError("invalid value for --limit")
			}
			opts.limit = value
		default:
			return c.refUsageError("unknown argument: " + args[i])
		}
	}
	if !validRefSource(opts.source) {
		return c.refUsageError("invalid value for --source")
	}
	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	var items []domain.Item
	if retryOnly {
		failed, err := store.Failed(context.Background())
		if err != nil {
			return c.printErr(err)
		}
		for _, failedItem := range failed {
			item, getErr := reader.GetItem(context.Background(), failedItem.ItemKey)
			if getErr == nil {
				items = append(items, item)
			}
		}
		opts.force = true
	} else {
		items, err = reader.FindItems(context.Background(), backend.FindOptions{All: true, Full: true})
		if err != nil {
			return c.printErr(err)
		}
	}
	report, err := references.NewBuilder(references.NewService(newNCBIClient(cfg)), store).Build(context.Background(), items, references.BuildOptions{Workers: opts.workers, Force: opts.force, Refresh: opts.force, Source: opts.source, Limit: opts.limit})
	if err != nil {
		return c.printErr(err)
	}
	command := "ref-build"
	if retryOnly {
		command = "ref-retry"
	}
	if opts.json {
		return c.writeJSON(jsonResponse{OK: true, Command: command, Data: report, Meta: map[string]any{"processed": report.Processed, "succeeded": report.Succeeded, "unsupported": report.Unsupported, "failed": report.Failed, "elapsed_ms": report.ElapsedMS}})
	}
	fmt.Fprintf(c.stdout, "Reference build: %d succeeded, %d unsupported, %d failed, %d skipped; %d references (%s)\n", report.Succeeded, report.Unsupported, report.Failed, report.Skipped, report.References, time.Duration(report.ElapsedMS)*time.Millisecond)
	return ExitOK
}

func (c *CLI) runRefStatus(args []string) int {
	jsonOutput, ok := parseRefJSONOnly(args)
	if !ok {
		return c.refUsageError("status accepts only --json")
	}
	cfg, code := c.loadConfig()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	status, err := store.Status(context.Background())
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-status", Data: status, Meta: map[string]any{"index_path": store.Path()}})
	}
	fmt.Fprintf(c.stdout, "Reference index: %d items (%d successful, %d unsupported, %d failed), %d references\nResolved: %d  Unresolved: %d  Contexts: %d\nPMC: %d  PubMed: %d  GROBID: %d\nLast indexed: %s\nPath: %s\n", status.IndexedItems, status.SuccessfulItems, status.UnsupportedItems, status.FailedItems, status.TotalReferences, status.ResolvedReferences, status.UnresolvedReferences, status.CitationContexts, status.PMCItems, status.PubMedItems, status.GrobidItems, status.LastIndexedAt, store.Path())
	fmt.Fprintf(c.stdout, "Context status: %d available, %d not supported, %d not found, %d parse failed, %d not indexed\nReferences with context: %d  without context: %d\n", status.ContextAvailableItems, status.ContextNotSupportedItems, status.ContextNotFoundItems, status.ContextParseFailedItems, status.ContextNotIndexedItems, status.ReferencesWithContext, status.ReferencesWithoutContext)
	return ExitOK
}

func (c *CLI) runRefFailed(args []string) int {
	jsonOutput, ok := parseRefJSONOnly(args)
	if !ok {
		return c.refUsageError("failed accepts only --json")
	}
	cfg, code := c.loadConfig()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	failed, err := store.Failed(context.Background())
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-failed", Data: failed, Meta: map[string]any{"total": len(failed)}})
	}
	for _, item := range failed {
		fmt.Fprintf(c.stdout, "%-10s  attempts=%d  %s  %s\n", item.ItemKey, item.Attempts, item.Title, item.Error)
	}
	return ExitOK
}

func (c *CLI) runRefUnsupported(args []string) int {
	jsonOutput, ok := parseRefJSONOnly(args)
	if !ok {
		return c.refUsageError("unsupported accepts only --json")
	}
	cfg, code := c.loadConfig()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	items, err := store.Unsupported(context.Background())
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-unsupported", Data: items, Meta: map[string]any{"total": len(items)}})
	}
	for _, item := range items {
		fmt.Fprintf(c.stdout, "%-10s  attempts=%d  %s  %s\n", item.ItemKey, item.Attempts, item.Title, item.Error)
	}
	return ExitOK
}

func (c *CLI) runRefResolve(args []string) int {
	jsonOutput := false
	workers := min(runtime.NumCPU(), 16)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--workers":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --workers")
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 1 || value > 32 {
				return c.refUsageError("invalid value for --workers")
			}
			workers = value
		default:
			return c.refUsageError("unknown argument: " + args[i])
		}
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	items, err := reader.FindItems(context.Background(), backend.FindOptions{All: true, Full: true})
	if err != nil {
		return c.printErr(err)
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	report, err := store.Resolve(context.Background(), references.NewResolver(items), workers)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-resolve", Data: report, Meta: map[string]any{"library_items": len(items), "workers": workers, "elapsed_ms": report.ElapsedMS}})
	}
	fmt.Fprintf(c.stdout, "Resolved %d/%d references in %s (DOI %d, PMID %d, exact title %d, fuzzy title %d)\n", report.Resolved, report.Total, time.Duration(report.ElapsedMS)*time.Millisecond, report.DOI, report.PMID, report.ExactTitle, report.FuzzyTitle)
	return ExitOK
}

func (c *CLI) runRefCitedBy(args []string) int {
	key, jsonOutput, ok := parseRefKeyJSON(args)
	if !ok {
		return c.refUsageError("cited-by requires one item key and optional --json")
	}
	cfg, code := c.loadConfig()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	rows, err := store.CitedBy(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-cited-by", Data: rows, Meta: map[string]any{"target_item_key": key, "total": len(rows)}})
	}
	fmt.Fprintf(c.stdout, "Cited by %d indexed item(s):\n", len(rows))
	for _, row := range rows {
		fmt.Fprintf(c.stdout, "%-10s  ref %d  %s (%d contexts)\n", row.SourceItemKey, row.Reference.Index, row.SourceTitle, len(row.Contexts))
	}
	return ExitOK
}

func (c *CLI) runRefContexts(args []string) int {
	if len(args) > 0 && args[0] == "build" {
		return c.runRefContextsBuild(args[1:])
	}
	key, jsonOutput, ok := parseRefKeyJSON(args)
	if !ok {
		return c.refUsageError("contexts requires one item key and optional --json")
	}
	cfg, code := c.loadConfig()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	rows, err := store.Contexts(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	summary, found, err := store.ContextSummary(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		meta := map[string]any{"item_key": key, "total": len(rows), "context_summary": summary, "summary_found": found}
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-contexts", Data: rows, Meta: meta})
	}
	fmt.Fprintf(c.stdout, "Citation contexts for %s: %d (status %s, coverage %.1f%%, %d references without context)\n", key, len(rows), summary.Status, summary.Coverage*100, summary.ReferencesWithoutContext)
	for _, row := range rows {
		fmt.Fprintf(c.stdout, "ref %d %-12s  [%s] %s\n", row.ReferenceIndex, row.TargetItemKey, row.Section, row.Paragraph)
	}
	return ExitOK
}

func (c *CLI) runRefContextsBuild(args []string) int {
	opts := refCommonOptions{workers: 3}
	refresh := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.json = true
		case "--refresh":
			refresh = true
		case "--workers":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --workers")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v < 1 || v > 16 {
				return c.refUsageError("invalid value for --workers")
			}
			opts.workers = v
		case "--limit":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --limit")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v < 1 {
				return c.refUsageError("invalid value for --limit")
			}
			opts.limit = v
		default:
			return c.refUsageError("unknown argument: " + args[i])
		}
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	pending, err := store.ContextPending(context.Background(), opts.limit)
	if err != nil {
		return c.printErr(err)
	}
	items := make([]domain.Item, 0, len(pending))
	for _, p := range pending {
		item, e := reader.GetItem(context.Background(), p.ItemKey)
		if e != nil {
			return c.printErr(e)
		}
		items = append(items, item)
	}
	report, err := references.NewBuilder(references.NewService(newNCBIClient(cfg)), store).Build(context.Background(), items, references.BuildOptions{Workers: opts.workers, Force: true, Refresh: refresh, Source: "pmc"})
	if err != nil {
		return c.printErr(err)
	}
	if opts.json {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-contexts-build", Data: report, Meta: map[string]any{"pending": len(pending), "workers": opts.workers, "refresh": refresh, "elapsed_ms": report.ElapsedMS}})
	}
	fmt.Fprintf(c.stdout, "Context backfill: %d succeeded, %d failed, %d unsupported; %d pending (%s)\n", report.Succeeded, report.Failed, report.Unsupported, len(pending), time.Duration(report.ElapsedMS)*time.Millisecond)
	return ExitOK
}

func (c *CLI) renderRefResult(result references.Result, jsonOutput, indexHit bool) int {
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref", Data: result, Meta: map[string]any{"total": len(result.References), "strategy": result.Strategy, "index_hit": indexHit, "cache_hits": result.CacheHits, "network_calls": result.NetworkCalls, "elapsed_ms": result.ElapsedMS}})
	}
	fmt.Fprintf(c.stdout, "References for %s (%s) via %s: %d\n", result.ItemTitle, result.ItemKey, result.Strategy, len(result.References))
	for _, ref := range result.References {
		id := ref.DOI
		if id == "" {
			id = ref.PMID
		}
		target := ""
		if ref.TargetItemKey != "" {
			target = " -> " + ref.TargetItemKey
		}
		fmt.Fprintf(c.stdout, "%3d  %-18s  %s%s\n", ref.Index, id, ref.Title, target)
	}
	if len(result.Contexts) > 0 {
		fmt.Fprintf(c.stdout, "Citation contexts: %d\n", len(result.Contexts))
	}
	return ExitOK
}

func newNCBIClient(cfg config.Config) *references.Client {
	root := referenceRoot(cfg)
	apiKey := os.Getenv("ZOT_NCBI_API_KEY")
	interval := 400 * time.Millisecond
	if apiKey != "" {
		interval = 125 * time.Millisecond
	}
	if raw := strings.TrimSpace(os.Getenv("ZOT_NCBI_RATE_MS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			interval = time.Duration(value) * time.Millisecond
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return references.NewClient(references.ClientConfig{BaseURL: os.Getenv("ZOT_NCBI_BASE_URL"), APIKey: apiKey, Email: os.Getenv("ZOT_NCBI_EMAIL"), CacheDir: filepath.Join(root, "ncbi"), MinInterval: interval, MaxAttempts: cfg.RetryMaxAttempts, HTTPClient: &http.Client{Timeout: timeout}})
}

func openReferenceStore(cfg config.Config) (*references.Store, error) {
	return references.OpenStore(filepath.Join(referenceRoot(cfg), "index.sqlite"))
}
func referenceRoot(cfg config.Config) string {
	if cfg.DataDir != "" {
		return filepath.Join(cfg.DataDir, ".zotero_cli", "ref")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zot", ".zotero_cli", "ref")
}
func validRefSource(source string) bool {
	return source == "auto" || source == "pmc" || source == "pubmed"
}
func parseRefJSONOnly(args []string) (bool, bool) {
	jsonOutput := false
	for _, arg := range args {
		if arg != "--json" {
			return false, false
		}
		jsonOutput = true
	}
	return jsonOutput, true
}
func parseRefKeyJSON(args []string) (string, bool, bool) {
	key := ""
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else if strings.HasPrefix(arg, "-") || key != "" {
			return "", false, false
		} else {
			key = arg
		}
	}
	return key, jsonOutput, key != ""
}
func (c *CLI) refUsageError(message string) int {
	fmt.Fprintln(c.stderr, "error:", message)
	fmt.Fprintln(c.stderr, usageRef)
	return ExitUsage
}
