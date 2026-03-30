package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)

	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version":
		printVersion()
		return 0
	case "config":
		return runConfig(args[1:])
	case "find":
		return runFind(args[1:])
	case "show":
		return runShow(args[1:])
	case "cite":
		return runCite(args[1:])
	case "export":
		return runExport(args[1:])
	case "collections":
		return runCollections(args[1:])
	case "notes":
		return runNotes(args[1:])
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 2
	}
}

func runConfig(args []string) int {
	if len(args) == 0 {
		printConfigUsage()
		return 0
	}

	switch args[0] {
	case "path":
		path, err := config.DefaultPath()
		if err != nil {
			return printErr(err)
		}
		fmt.Fprintln(stdout, path)
		return 0
	case "show":
		cfg, path, err := config.Load()
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				fmt.Fprintf(stderr, "config not found; run `zot config init` first\n")
				return 3
			}
			return printErr(err)
		}

		out := map[string]any{
			"path":   path,
			"config": maskConfig(cfg),
		}

		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	case "init":
		return runConfigInit(args[1:])
	default:
		fmt.Fprintf(stderr, "unknown config command: %s\n\n", args[0])
		printConfigUsage()
		return 2
	}
}

func runConfigInit(args []string) int {
	path, err := config.DefaultPath()
	if err != nil {
		return printErr(err)
	}

	if len(args) > 0 && args[0] == "--example" {
		cfg := config.Default()
		cfg.LibraryType = "user"
		cfg.LibraryID = "123456"
		cfg.APIKey = "replace-me"

		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			return printErr(err)
		}
		return 0
	}

	if _, _, err := config.Load(); err == nil {
		fmt.Fprintf(stderr, "config already exists at %s\n", path)
		fmt.Fprintf(stderr, "edit it manually or remove it before re-running init\n")
		return 3
	} else if !errors.Is(err, config.ErrNotFound) {
		return printErr(err)
	}

	cfg := config.Default()
	if err := config.Save(cfg); err != nil {
		return printErr(err)
	}

	fmt.Fprintf(stdout, "created config at %s\n", path)
	fmt.Fprintln(stdout, "edit the file and fill in your library_type, library_id, and api_key")
	return 0
}

func runFind(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: zot find <query> [--json] [--item-type TYPE] [--limit N]")
		return 2
	}

	opts, jsonOutput, err := parseFindArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, "usage: zot find <query> [--json] [--item-type TYPE] [--limit N]")
		return 2
	}

	if strings.TrimSpace(opts.Query) == "" {
		fmt.Fprintln(stderr, "usage: zot find <query> [--json] [--item-type TYPE] [--limit N]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)
	items, err := client.FindItems(context.Background(), opts)
	if err != nil {
		return printErr(err)
	}
	items = filterDefaultFindItems(items, opts)

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "find",
			"data":    items,
			"meta": map[string]any{
				"total": len(items),
			},
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreators(item.Creators),
			item.Title,
		)
	}
	return 0
}

func parseFindArgs(args []string) (zoteroapi.FindOptions, bool, error) {
	opts := zoteroapi.FindOptions{}
	jsonOutput := false
	queryParts := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--item-type":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --item-type")
			}
			i++
			opts.ItemType = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return zoteroapi.FindOptions{}, false, errors.New("invalid value for --limit")
			}
			opts.Limit = limit
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	opts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	return opts, jsonOutput, nil
}

func filterDefaultFindItems(items []zoteroapi.Item, opts zoteroapi.FindOptions) []zoteroapi.Item {
	if opts.ItemType != "" {
		return items
	}

	filtered := make([]zoteroapi.Item, 0, len(items))
	for _, item := range items {
		if item.ItemType == "attachment" || item.ItemType == "note" {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func shortDate(value string) string {
	if len(value) >= 4 {
		return value[:4]
	}
	return value
}

func shortCreators(creators []zoteroapi.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	name := creators[0].Name
	if len(creators) == 1 {
		return name
	}
	return name + " et al."
}

func runShow(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: zot show <item-key> [--json]")
		return 2
	}

	jsonOutput := false
	key := ""
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if key == "" {
			key = arg
		}
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(stderr, "usage: zot show <item-key> [--json]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)
	item, err := client.GetItem(context.Background(), key)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "show",
			"data":    item,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	fmt.Fprintf(stdout, "Key: %s\n", item.Key)
	fmt.Fprintf(stdout, "Title: %s\n", item.Title)
	fmt.Fprintf(stdout, "Type: %s\n", item.ItemType)
	if len(item.Creators) > 0 {
		fmt.Fprintf(stdout, "Creators: %s\n", joinCreatorNames(item.Creators))
	}
	if item.Date != "" {
		fmt.Fprintf(stdout, "Date: %s\n", item.Date)
	}
	if item.Container != "" {
		fmt.Fprintf(stdout, "Container: %s\n", item.Container)
	}
	if item.DOI != "" {
		fmt.Fprintf(stdout, "DOI: %s\n", item.DOI)
	}
	if item.URL != "" {
		fmt.Fprintf(stdout, "URL: %s\n", item.URL)
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(stdout, "Tags: %s\n", strings.Join(item.Tags, ", "))
	}
	if len(item.Attachments) > 0 {
		fmt.Fprintf(stdout, "Attachments: %d\n", len(item.Attachments))
		for _, attachment := range item.Attachments {
			label := attachment.Title
			if attachment.Filename != "" {
				label = attachment.Filename
			}
			if label == "" {
				label = attachment.Key
			}
			fmt.Fprintf(stdout, "  - [%s] %s\n", attachmentKind(attachment), label)
		}
	}
	return 0
}

func runCite(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: zot cite <item-key> [--format citation|bib] [--style STYLE] [--locale LOCALE] [--json]")
		return 2
	}

	key, opts, jsonOutput, err := parseCiteArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, "usage: zot cite <item-key> [--format citation|bib] [--style STYLE] [--locale LOCALE] [--json]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	if opts.Format == "" {
		opts.Format = "citation"
	}
	if opts.Style == "" {
		opts.Style = cfg.Style
	}
	if opts.Locale == "" {
		opts.Locale = cfg.Locale
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)
	result, err := client.GetCitation(context.Background(), key, opts)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "cite",
			"data":    result,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	fmt.Fprintln(stdout, result.Text)
	return 0
}

func runExport(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: zot export <query> [--limit N] [--json] | zot export --item-key KEY [--json]")
		return 2
	}

	itemKey, findOpts, jsonOutput, err := parseExportArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, "usage: zot export <query> [--limit N] [--json] | zot export --item-key KEY [--json]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)

	keys := make([]string, 0, 8)
	if itemKey != "" {
		keys = append(keys, itemKey)
	} else {
		items, err := client.FindItems(context.Background(), findOpts)
		if err != nil {
			return printErr(err)
		}
		items = filterDefaultFindItems(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}

	results := make([]zoteroapi.CitationResult, 0, len(keys))
	for _, key := range keys {
		result, err := client.GetCitation(context.Background(), key, zoteroapi.CitationOptions{
			Format: "bib",
			Style:  cfg.Style,
			Locale: cfg.Locale,
		})
		if err != nil {
			return printErr(err)
		}
		results = append(results, result)
	}

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "export",
			"data":    results,
			"meta": map[string]any{
				"total": len(results),
			},
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	for i, result := range results {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintln(stdout, result.Text)
	}
	return 0
}

func runCollections(args []string) int {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		fmt.Fprintln(stderr, "usage: zot collections [--json]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)
	collections, err := client.ListCollections(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "collections",
			"data":    collections,
			"meta": map[string]any{
				"total": len(collections),
			},
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	for _, collection := range collections {
		fmt.Fprintf(stdout, "%-10s  %-20s  items=%d  children=%d\n",
			collection.Key,
			collection.Name,
			collection.NumItems,
			collection.NumCollections,
		)
	}
	return 0
}

func runNotes(args []string) int {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		fmt.Fprintln(stderr, "usage: zot notes [--json]")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)
	notes, err := client.ListNotes(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		out := map[string]any{
			"ok":      true,
			"command": "notes",
			"data":    notes,
			"meta": map[string]any{
				"total": len(notes),
			},
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return printErr(err)
		}
		return 0
	}

	for _, note := range notes {
		fmt.Fprintf(stdout, "%-10s  %s\n", note.Key, note.Content)
	}
	return 0
}

func parseExportArgs(args []string) (string, zoteroapi.FindOptions, bool, error) {
	var itemKey string
	var jsonOutput bool
	findOpts := zoteroapi.FindOptions{}
	queryParts := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--item-key":
			if i+1 >= len(args) {
				return "", zoteroapi.FindOptions{}, false, errors.New("missing value for --item-key")
			}
			i++
			itemKey = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return "", zoteroapi.FindOptions{}, false, errors.New("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return "", zoteroapi.FindOptions{}, false, errors.New("invalid value for --limit")
			}
			findOpts.Limit = limit
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	if itemKey != "" && len(queryParts) > 0 {
		return "", zoteroapi.FindOptions{}, false, errors.New("cannot use query and --item-key together")
	}

	if itemKey == "" {
		findOpts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
		if findOpts.Query == "" {
			return "", zoteroapi.FindOptions{}, false, errors.New("missing query or --item-key")
		}
	}

	return itemKey, findOpts, jsonOutput, nil
}

func parseCiteArgs(args []string) (string, zoteroapi.CitationOptions, bool, error) {
	var key string
	opts := zoteroapi.CitationOptions{}
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--format":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --format")
			}
			i++
			opts.Format = args[i]
		case "--style":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --style")
			}
			i++
			opts.Style = args[i]
		case "--locale":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --locale")
			}
			i++
			opts.Locale = args[i]
		default:
			if key == "" {
				key = args[i]
				continue
			}
			return "", zoteroapi.CitationOptions{}, false, errors.New("too many positional arguments")
		}
	}

	if strings.TrimSpace(key) == "" {
		return "", zoteroapi.CitationOptions{}, false, errors.New("missing item key")
	}
	if opts.Format != "" && opts.Format != "citation" && opts.Format != "bib" {
		return "", zoteroapi.CitationOptions{}, false, errors.New("unsupported format")
	}

	return key, opts, jsonOutput, nil
}

func renderCreators(creators []zoteroapi.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	if len(creators) == 1 {
		return creators[0].Name
	}
	return creators[0].Name + " et al."
}

func joinCreatorNames(creators []zoteroapi.Creator) string {
	names := make([]string, 0, len(creators))
	for _, creator := range creators {
		if creator.Name != "" {
			names = append(names, creator.Name)
		}
	}
	return strings.Join(names, ", ")
}

func attachmentKind(attachment zoteroapi.Attachment) string {
	if attachment.ContentType == "application/pdf" {
		return "pdf"
	}
	switch attachment.LinkMode {
	case "linked_url":
		return "link"
	case "linked_file", "imported_file":
		return "file"
	default:
		if attachment.ContentType != "" {
			return "file"
		}
		return "attachment"
	}
}

func maskConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"mode":            cfg.Mode,
		"library_type":    cfg.LibraryType,
		"library_id":      cfg.LibraryID,
		"api_key":         maskSecret(cfg.APIKey),
		"style":           cfg.Style,
		"locale":          cfg.Locale,
		"timeout_seconds": cfg.TimeoutSeconds,
	}
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(stdout, `%s is a minimal Zotero CLI.

Usage:
  %s <command>

Commands:
  version        Show CLI version
  config path    Print config path
  config init    Create a starter config file
  config show    Show active config with masked secrets
  find           Search items in the configured Zotero library
  show           Show item details
  cite           Generate a citation or bibliography entry
  export         Export bibliography entries
  collections    List collections
  notes          List notes
`, exe, exe)
}

func printVersion() {
	fmt.Fprintf(stdout, "zot %s\n", version)
	fmt.Fprintf(stdout, "commit: %s\n", commit)
	fmt.Fprintf(stdout, "built: %s\n", buildDate)
}

func printConfigUsage() {
	fmt.Fprint(stdout, `Usage:
  zot config path
  zot config init
  zot config init --example
  zot config show
`)
}

func printErr(err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}
