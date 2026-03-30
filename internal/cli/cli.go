package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

const version = "0.1.0"

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)
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
		fmt.Fprintln(stdout, version)
		return 0
	case "config":
		return runConfig(args[1:])
	case "find":
		return runFind(args[1:])
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
		fmt.Fprintln(stderr, "usage: zot find <query> [--json]")
		return 2
	}

	jsonOutput := false
	queryParts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		queryParts = append(queryParts, arg)
	}

	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if query == "" {
		fmt.Fprintln(stderr, "usage: zot find <query> [--json]")
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
	items, err := client.FindItems(context.Background(), query)
	if err != nil {
		return printErr(err)
	}

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
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n",
			item.Key,
			item.Title,
			renderCreators(item.Creators),
			item.Date,
			item.ItemType,
		)
	}
	return 0
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
`, exe, exe)
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
