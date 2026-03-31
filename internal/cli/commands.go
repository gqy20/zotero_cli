package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

const (
	usageFind        = "usage: zot find <query> [--json] [--item-type TYPE] [--limit N]"
	usageShow        = "usage: zot show <item-key> [--json]"
	usageCite        = "usage: zot cite <item-key> [--format citation|bib] [--style STYLE] [--locale LOCALE] [--json]"
	usageExport      = "usage: zot export <query> [--limit N] [--json] | zot export --item-key KEY [--json]"
	usageCollections = "usage: zot collections [--json]"
	usageNotes       = "usage: zot notes [--json]"
	usageTags        = "usage: zot tags [--json]"
	usageSearches    = "usage: zot searches [--json]"
	usageDeleted     = "usage: zot deleted [--json]"
	usageVersions    = "usage: zot versions <collections|searches|items|items-top> --since N [--include-trashed] [--if-modified-since-version N] [--json]"
)

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

		return writeJSON(map[string]any{
			"path":   path,
			"config": maskConfig(cfg),
		})
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
		return writeJSON(cfg)
	}

	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "config already exists at %s\n", path)
		fmt.Fprintf(stderr, "edit it manually or remove it before re-running init\n")
		return 3
	} else if !errors.Is(err, os.ErrNotExist) {
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
		fmt.Fprintln(stderr, usageFind)
		return 2
	}

	opts, jsonOutput, err := parseFindArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageFind)
		return 2
	}

	if strings.TrimSpace(opts.Query) == "" {
		fmt.Fprintln(stderr, usageFind)
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	items, err := client.FindItems(context.Background(), opts)
	if err != nil {
		return printErr(err)
	}
	items = filterDefaultFindItems(items, opts)

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "find",
			Data:    items,
			Meta: map[string]any{
				"total": len(items),
			},
		})
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

func runShow(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageShow)
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
			continue
		}
		fmt.Fprintln(stderr, usageShow)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(stderr, usageShow)
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	item, err := client.GetItem(context.Background(), key)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "show",
			Data:    item,
		})
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
		fmt.Fprintln(stderr, usageCite)
		return 2
	}

	key, opts, jsonOutput, err := parseCiteArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageCite)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
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

	result, err := client.GetCitation(context.Background(), key, opts)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "cite",
			Data:    result,
		})
	}

	fmt.Fprintln(stdout, result.Text)
	return 0
}

func runExport(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageExport)
		return 2
	}

	itemKey, findOpts, jsonOutput, err := parseExportArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageExport)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

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
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "export",
			Data:    results,
			Meta: map[string]any{
				"total": len(results),
			},
		})
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
	jsonOutput, ok := parseJSONOnlyArgs(args, usageCollections)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	collections, err := client.ListCollections(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "collections",
			Data:    collections,
			Meta: map[string]any{
				"total": len(collections),
			},
		})
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
	jsonOutput, ok := parseJSONOnlyArgs(args, usageNotes)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	notes, err := client.ListNotes(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "notes",
			Data:    notes,
			Meta: map[string]any{
				"total": len(notes),
			},
		})
	}

	visible := filterVisibleNotes(notes)
	if len(visible) == 0 {
		fmt.Fprintln(stdout, "no readable notes found in text mode; use --json to inspect all notes")
		return 0
	}

	for _, note := range visible {
		fmt.Fprintf(stdout, "%-10s  %s\n", note.Key, notePreview(note.Content))
	}
	return 0
}

func runTags(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageTags)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	tags, err := client.ListTags(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "tags",
			Data:    tags,
			Meta: map[string]any{
				"total": len(tags),
			},
		})
	}

	for _, tag := range tags {
		fmt.Fprintf(stdout, "%-20s  items=%d\n", tag.Name, tag.NumItems)
	}
	return 0
}

func runSearches(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageSearches)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	searches, err := client.ListSearches(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "searches",
			Data:    searches,
			Meta: map[string]any{
				"total": len(searches),
			},
		})
	}

	for _, search := range searches {
		fmt.Fprintf(stdout, "%-10s  %-24s  conditions=%d\n", search.Key, search.Name, search.NumConditions)
	}
	return 0
}

func runDeleted(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageDeleted)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	deleted, err := client.GetDeleted(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "deleted",
			Data:    deleted,
		})
	}

	fmt.Fprintf(stdout, "collections=%d\n", len(deleted.Collections))
	fmt.Fprintf(stdout, "searches=%d\n", len(deleted.Searches))
	fmt.Fprintf(stdout, "items=%d\n", len(deleted.Items))
	fmt.Fprintf(stdout, "tags=%d\n", len(deleted.Tags))
	return 0
}

func runVersions(args []string) int {
	objectType, opts, jsonOutput, err := parseVersionsArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageVersions)
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	result, err := client.GetVersionsResult(context.Background(), zoteroapi.VersionsOptions{
		ObjectType:             objectType,
		Since:                  opts.Since,
		IncludeTrashed:         opts.IncludeTrashed,
		IfModifiedSinceVersion: opts.IfModifiedSinceVersion,
	})
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{
			"object_type": objectType,
			"total":       len(result.Versions),
		}
		if result.LastModifiedVersion > 0 {
			meta["last_modified_version"] = result.LastModifiedVersion
		}
		if result.NotModified {
			meta["not_modified"] = true
		}

		return writeJSON(jsonResponse{
			OK:      true,
			Command: "versions",
			Data:    result.Versions,
			Meta:    meta,
		})
	}

	if result.NotModified {
		fmt.Fprintf(stdout, "not modified since version %d\n", opts.IfModifiedSinceVersion)
		return 0
	}

	for key, version := range result.Versions {
		fmt.Fprintf(stdout, "%-10s  %d\n", key, version)
	}
	return 0
}
