package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

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

type supplementsArgs struct {
	key        string
	all        bool
	jsonOutput bool
	online     bool
	limit      int
}

func (c *CLI) runSupplements(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageSupplements)
	}
	parsed, ok := parseSupplementsArgs(args)
	if !ok {
		fmt.Fprintln(c.stderr, usageSupplements)
		return ExitUsage
	}

	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	if cfg.Mode == "web" || cfg.Mode == "remote" {
		if !parsed.online {
			return c.printErr(fmt.Errorf("supplements requires local or hybrid mode with local Zotero data; add --online to query public provider metadata"))
		}
	}
	if parsed.online && parsed.all {
		return c.printErr(fmt.Errorf("supplements --online does not support --all; query one item key at a time"))
	}

	ctx := context.Background()
	items, err := loadSupplementItems(ctx, reader, parsed)
	if err != nil {
		return c.printErr(err)
	}
	supplements := []backend.Supplement{}
	includeLocal := cfg.Mode != "web" && cfg.Mode != "remote"
	if includeLocal {
		supplements = append(supplements, backend.LocalSupplements(items)...)
	}
	var onlineDiscovery backend.OnlineSupplementDiscovery
	if parsed.online {
		jar, _ := cookiejar.New(nil)
		onlineClient := &http.Client{Timeout: 30 * time.Second, Jar: jar}
		for _, item := range items {
			discovery := backend.DiscoverOnlineSupplements(ctx, onlineClient, item)
			onlineDiscovery.Providers = append(onlineDiscovery.Providers, discovery.Providers...)
			supplements = append(supplements, discovery.Supplements...)
		}
	}
	totalBeforeLimit := len(supplements)
	if parsed.limit > 0 && len(supplements) > parsed.limit {
		supplements = supplements[:parsed.limit]
	}

	if parsed.jsonOutput {
		meta := map[string]any{
			"total":                 len(supplements),
			"total_before_limit":    totalBeforeLimit,
			"scanned_items":         len(items),
			"provider":              supplementsProviderSummary(includeLocal, parsed.online),
			"provider_status":       supplementsProviderStatus(includeLocal, parsed.online, onlineDiscovery.Providers),
			"online_lookup_enabled": parsed.online,
		}
		if parsed.online {
			meta["online_providers"] = onlineDiscovery.Providers
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "supplements",
			Data:    supplements,
			Meta:    meta,
		})
	}

	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))
	if len(supplements) == 0 {
		fmt.Fprintln(c.stdout, "No local supplement candidates found.")
		return ExitOK
	}
	for _, supplement := range supplements {
		fmt.Fprintf(c.stdout, "%s  %-22s  %-20s  %.2f  %s\n",
			supplement.ItemKey,
			supplement.Kind,
			supplement.ResolutionStatus,
			supplement.Confidence,
			supplement.Label,
		)
		if supplement.LocalPath != "" {
			fmt.Fprintf(c.stdout, "  path: %s\n", supplement.LocalPath)
		} else if supplement.ZoteroPath != "" {
			fmt.Fprintf(c.stdout, "  path: unresolved (%s)\n", supplement.ZoteroPath)
		}
		if supplement.DownloadURL != "" {
			fmt.Fprintf(c.stdout, "  download: %s\n", supplement.DownloadURL)
		}
		if len(supplement.Evidence) > 0 {
			fmt.Fprintf(c.stdout, "  evidence: %s\n", strings.Join(supplement.Evidence, ", "))
		}
	}
	return ExitOK
}

func parseSupplementsArgs(args []string) (supplementsArgs, bool) {
	var parsed supplementsArgs
	nextFlag := ""
	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "limit":
				limit, err := strconv.Atoi(arg)
				if err != nil || limit <= 0 {
					return supplementsArgs{}, false
				}
				parsed.limit = limit
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--json":
			parsed.jsonOutput = true
		case "--all":
			parsed.all = true
		case "--online":
			parsed.online = true
		case "--limit":
			nextFlag = "limit"
		default:
			if strings.HasPrefix(arg, "-") {
				return supplementsArgs{}, false
			}
			if parsed.key != "" {
				return supplementsArgs{}, false
			}
			parsed.key = arg
		}
	}
	if nextFlag != "" {
		return supplementsArgs{}, false
	}
	if parsed.all == (strings.TrimSpace(parsed.key) != "") {
		return supplementsArgs{}, false
	}
	return parsed, true
}

func loadSupplementItems(ctx context.Context, reader backend.Reader, parsed supplementsArgs) ([]domain.Item, error) {
	if parsed.all {
		return reader.FindItems(ctx, backend.FindOptions{All: true, Full: true})
	}
	item, err := reader.GetItem(ctx, parsed.key)
	if err != nil {
		return nil, err
	}
	return []domain.Item{item}, nil
}

func supplementsProviderSummary(includeLocal bool, online bool) string {
	parts := []string{}
	if includeLocal {
		parts = append(parts, "local")
	}
	if online {
		parts = append(parts, "online")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "+")
}

func supplementsProviderStatus(includeLocal bool, online bool, statuses []backend.SupplementProviderStatus) string {
	if !online {
		if includeLocal {
			return "complete"
		}
		return ""
	}
	if len(statuses) == 0 {
		return "not_applicable"
	}
	complete := false
	partialOrBlocked := false
	for _, status := range statuses {
		switch status.Status {
		case "complete":
			complete = true
		case "partial", "blocked":
			partialOrBlocked = true
		}
	}
	if complete && !partialOrBlocked {
		return "complete"
	}
	if complete {
		return "partial"
	}
	if partialOrBlocked {
		return "partial"
	}
	return "blocked"
}
