package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"zotero_cli/internal/references"
)

func (c *CLI) runRefRelated(args []string) int {
	key, limit, refresh, alsoViewed, jsonOutput := "", 20, false, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--refresh":
			refresh = true
		case "--also-viewed":
			alsoViewed = true
		case "--limit":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --limit")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 || n > 100 {
				return c.refUsageError("invalid value for --limit")
			}
			limit = n
		default:
			if strings.HasPrefix(args[i], "-") {
				return c.refUsageError("unknown flag: " + args[i])
			}
			if key != "" {
				return c.refUsageError("related accepts exactly one item key")
			}
			key = args[i]
		}
	}
	if key == "" {
		return c.refUsageError("related requires one item key")
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	rows, ids, err := references.NewService(newNCBIClient(cfg)).Related(context.Background(), item, limit, alsoViewed, refresh)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		mode := "similar"
		if alsoViewed {
			mode = "also_viewed"
		}
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-related", Data: rows, Meta: map[string]any{"item_key": key, "pmid": ids.PMID, "total": len(rows), "limit": limit, "mode": mode}})
	}
	fmt.Fprintf(c.stdout, "PubMed related articles for %s: %d\n", key, len(rows))
	for _, row := range rows {
		fmt.Fprintf(c.stdout, "%3d  PMID %-10s %s\n", row.Rank, row.PMID, row.Title)
	}
	return ExitOK
}

func (c *CLI) runRefLinks(args []string) int {
	key, refresh, jsonOutput := "", false, false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--refresh":
			refresh = true
		default:
			if strings.HasPrefix(arg, "-") {
				return c.refUsageError("unknown flag: " + arg)
			}
			if key != "" {
				return c.refUsageError("links accepts exactly one item key")
			}
			key = arg
		}
	}
	if key == "" {
		return c.refUsageError("links requires one item key")
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	rows, ids, err := references.NewService(newNCBIClient(cfg)).Links(context.Background(), item, refresh)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-links", Data: rows, Meta: map[string]any{"item_key": key, "pmid": ids.PMID, "total": len(rows)}})
	}
	fmt.Fprintf(c.stdout, "NCBI resource links for %s: %d type(s)\n", key, len(rows))
	for _, row := range rows {
		shown := row.IDs
		if len(shown) > 10 {
			shown = shown[:10]
		}
		suffix := ""
		if len(row.IDs) > len(shown) {
			suffix = fmt.Sprintf(" ... (%d total)", len(row.IDs))
		}
		fmt.Fprintf(c.stdout, "%-12s %s%s\n", row.Database, strings.Join(shown, ", "), suffix)
	}
	return ExitOK
}

func (c *CLI) runRefAnnotations(args []string) int {
	key, refresh, jsonOutput := "", false, false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--refresh":
			refresh = true
		default:
			if strings.HasPrefix(arg, "-") {
				return c.refUsageError("unknown flag: " + arg)
			}
			if key != "" {
				return c.refUsageError("annotations accepts exactly one item key")
			}
			key = arg
		}
	}
	if key == "" {
		return c.refUsageError("annotations requires one item key")
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	rows, ids, err := references.NewService(newNCBIClient(cfg)).Annotations(context.Background(), item, refresh)
	if err != nil {
		return c.printErr(err)
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	if err := store.SaveAnnotations(context.Background(), key, rows); err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-annotations", Data: rows, Meta: map[string]any{"item_key": key, "pmid": ids.PMID, "total": len(rows), "source": "europe_pmc"}})
	}
	fmt.Fprintf(c.stdout, "Europe PMC annotations for %s: %d\n", key, len(rows))
	for i, row := range rows {
		if i >= 50 {
			fmt.Fprintf(c.stdout, "... (%d total)\n", len(rows))
			break
		}
		fmt.Fprintf(c.stdout, "%-24s %-20s %s\n", row.Type, row.Label, row.Exact)
	}
	return ExitOK
}

func (c *CLI) runRefProfile(args []string) int {
	key, refresh, jsonOutput := "", false, false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--refresh":
			refresh = true
		default:
			if strings.HasPrefix(arg, "-") {
				return c.refUsageError("unknown flag: " + arg)
			}
			if key != "" {
				return c.refUsageError("profile accepts exactly one item key")
			}
			key = arg
		}
	}
	if key == "" {
		return c.refUsageError("profile requires one item key")
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	profile, err := references.NewService(newNCBIClient(cfg)).Profile(context.Background(), item, refresh)
	if err != nil {
		return c.printErr(err)
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "ref-profile", Data: profile, Meta: map[string]any{"item_key": key, "source": "europe_pmc"}})
	}
	fmt.Fprintf(c.stdout, "Europe PMC profile for %s: %s/%s, cited by %d, OA %v (%s)\n", key, profile.Source, profile.ID, profile.CitedByCount, profile.OpenAccess, profile.License)
	for _, v := range profile.Versions {
		fmt.Fprintf(c.stdout, "%-12s %-12s %s\n", v.Type, v.Source+"/"+v.ID, v.Reference)
	}
	fmt.Fprintf(c.stdout, "Funding: %d grant(s); evaluations: %d\n", len(profile.Grants), len(profile.Evaluations))
	return ExitOK
}
