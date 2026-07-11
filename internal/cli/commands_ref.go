package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
       zot ref retry [--workers N] [--json]

What: Manage the local structured-reference index. A direct item key is short
for 'ref show'. NCBI routing prefers complete PMC JATS and otherwise uses
PubMed reference links plus batched metadata.

Subcommands:
  show ITEMKEY  Fetch one item and persist it in the local reference index.
  build         Incrementally index every eligible top-level library item.
  status        Show index coverage and reference counts.
  failed        List failed items and their last errors.
  retry         Retry all currently failed items.

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
	case "retry":
		return c.runRefBuild(args[1:], true)
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
	report, err := references.NewBuilder(references.NewService(newNCBIClient(cfg)), store).Build(context.Background(), items, references.BuildOptions{Workers: opts.workers, Force: opts.force, Source: opts.source, Limit: opts.limit})
	if err != nil {
		return c.printErr(err)
	}
	command := "ref-build"
	if retryOnly {
		command = "ref-retry"
	}
	if opts.json {
		return c.writeJSON(jsonResponse{OK: true, Command: command, Data: report, Meta: map[string]any{"processed": report.Processed, "succeeded": report.Succeeded, "failed": report.Failed, "elapsed_ms": report.ElapsedMS}})
	}
	fmt.Fprintf(c.stdout, "Reference build: %d succeeded, %d failed, %d skipped; %d references (%s)\n", report.Succeeded, report.Failed, report.Skipped, report.References, time.Duration(report.ElapsedMS)*time.Millisecond)
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
	fmt.Fprintf(c.stdout, "Reference index: %d items (%d successful, %d failed), %d references\nPMC: %d  PubMed: %d\nLast indexed: %s\nPath: %s\n", status.IndexedItems, status.SuccessfulItems, status.FailedItems, status.TotalReferences, status.PMCItems, status.PubMedItems, status.LastIndexedAt, store.Path())
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
		fmt.Fprintf(c.stdout, "%3d  %-18s  %s\n", ref.Index, id, ref.Title)
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
func (c *CLI) refUsageError(message string) int {
	fmt.Fprintln(c.stderr, "error:", message)
	fmt.Fprintln(c.stderr, usageRef)
	return ExitUsage
}
