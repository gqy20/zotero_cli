package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"zotero_cli/internal/references"
)

func (c *CLI) runRefSearch(args []string) int {
	opts := references.SearchOptions{In: "all", Limit: 20}
	jsonOutput := false
	contextsOnly, referencesOnly, metadataOnly := false, false, false
	queryParts := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--contexts":
			contextsOnly = true
		case "--references":
			referencesOnly = true
		case "--metadata":
			metadataOnly = true
		case "--field":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --field")
			}
			i++
			opts.Field = strings.ToLower(args[i])
		case "--section":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --section")
			}
			i++
			opts.Section = args[i]
		case "--source":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --source")
			}
			i++
			opts.Source = strings.ToLower(args[i])
			if opts.Source == "pmc" {
				opts.Source = string(references.SourcePMC)
			}
		case "--target":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --target")
			}
			i++
			opts.Target = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --limit")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v < 1 || v > 200 {
				return c.refUsageError("invalid value for --limit")
			}
			opts.Limit = v
		default:
			if strings.HasPrefix(args[i], "-") {
				return c.refUsageError("unknown flag: " + args[i])
			}
			queryParts = append(queryParts, args[i])
		}
	}
	opts.Query = strings.Join(queryParts, " ")
	if strings.TrimSpace(opts.Query) == "" {
		return c.refUsageError("ref search requires a query")
	}
	if boolCount(contextsOnly, referencesOnly, metadataOnly) > 1 {
		return c.refUsageError("--contexts, --references, and --metadata cannot be combined")
	}
	if referencesOnly && opts.Section != "" {
		return c.refUsageError("--section applies to contexts and cannot be used with --references")
	}
	if contextsOnly || opts.Section != "" {
		opts.In = "contexts"
	} else if referencesOnly {
		opts.In = "references"
	} else if metadataOnly || opts.Field != "" {
		opts.In = "metadata"
	}
	validFields := map[string]bool{"": true, "mesh": true, "publication_types": true, "keywords": true, "chemicals": true, "grants": true, "corrections": true}
	if !validFields[opts.Field] {
		return c.refUsageError("invalid value for --field")
	}
	if opts.Source != "" && opts.Source != string(references.SourcePMC) && opts.Source != string(references.SourcePubMed) && opts.Source != string(references.SourceGROBID) {
		return c.refUsageError("invalid value for --source")
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
	hits, err := store.Search(context.Background(), opts)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-search", Data: hits, Meta: map[string]any{"query": opts.Query, "scope": opts.In, "total": len(hits), "limit": opts.Limit}})
	}
	fmt.Fprintf(c.stdout, "Reference search: %d result(s)\n", len(hits))
	for _, h := range hits {
		fmt.Fprintf(c.stdout, "%-10s ref %-4d %-12s %s\n", h.SourceItemKey, h.Reference.Index, strings.Join(h.MatchedOn, ","), h.Reference.Title)
		for _, x := range h.Contexts {
			fmt.Fprintf(c.stdout, "  [%s] %s\n", x.Section, x.Paragraph)
		}
	}
	return ExitOK
}

func boolCount(values ...bool) int {
	n := 0
	for _, v := range values {
		if v {
			n++
		}
	}
	return n
}
