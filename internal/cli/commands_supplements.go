package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

const usageSupplements = `usage: zot supplements <item-key> [--json]
       zot supplements --all [--json] [--limit N]

What: Find local supplementary/data attachments already present in the Zotero
library. This command inspects attachment metadata and resolved local paths; it
does not fetch publisher pages or DOI records.

Output:
  --json       Structured output for agents.
  --all        Scan all local library items.
  --limit N    Limit emitted supplement records after scanning.

Examples:
  zot supplements ABCD --json
  zot supplements --all --json --limit 50

Notes:
  - Requires local/hybrid mode with local Zotero data.
  - Classification uses attachment title, filename, path, content type, and
    extension. Parent item titles are not used as supplement evidence.
  - See also: show KEY --full --json, extract-text.`

type supplementsArgs struct {
	key        string
	all        bool
	jsonOutput bool
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
		return c.printErr(fmt.Errorf("supplements requires local or hybrid mode with local Zotero data"))
	}

	ctx := context.Background()
	items, err := loadSupplementItems(ctx, reader, parsed)
	if err != nil {
		return c.printErr(err)
	}
	supplements := backend.LocalSupplements(items)
	totalBeforeLimit := len(supplements)
	if parsed.limit > 0 && len(supplements) > parsed.limit {
		supplements = supplements[:parsed.limit]
	}

	if parsed.jsonOutput {
		meta := map[string]any{
			"total":                 len(supplements),
			"total_before_limit":    totalBeforeLimit,
			"scanned_items":         len(items),
			"provider":              "local",
			"provider_status":       "complete",
			"online_lookup_enabled": false,
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
