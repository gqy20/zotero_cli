package cli

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/zoteroapi"
)

func runFind(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageFind)
		return 2
	}

	opts, jsonOutput, queryProvided, err := parseFindArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageFind)
		return 2
	}

	if strings.TrimSpace(opts.Query) == "" && !opts.All && !queryProvided {
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

	if opts.Full || len(opts.IncludeFields) > 0 {
		for index, item := range items {
			renderFindItemDetailed(item, opts)
			if index < len(items)-1 {
				fmt.Fprintln(stdout)
			}
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

func runStats(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageStats)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	stats, err := client.GetLibraryStats(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "stats", Data: stats})
	}
	fmt.Fprintf(stdout, "library=%s:%s\n", stats.LibraryType, stats.LibraryID)
	fmt.Fprintf(stdout, "items=%d\n", stats.TotalItems)
	fmt.Fprintf(stdout, "collections=%d\n", stats.TotalCollections)
	fmt.Fprintf(stdout, "searches=%d\n", stats.TotalSearches)
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

	itemKey, collectionKey, findOpts, format, jsonOutput, err := parseExportArgs(args)
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
	} else if collectionKey != "" {
		items, err := client.ListCollectionItems(context.Background(), collectionKey, findOpts)
		if err != nil {
			return printErr(err)
		}
		items = filterDefaultFindItems(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
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

	result, err := client.ExportItems(context.Background(), keys, zoteroapi.ExportOptions{
		Format: format,
		Style:  cfg.Style,
		Locale: cfg.Locale,
	})
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "export",
			Data:    result,
			Meta: map[string]any{
				"total": len(keys),
			},
		})
	}

	if result.Text != "" {
		fmt.Fprintln(stdout, result.Text)
		return 0
	}
	return writeJSON(result.Data)
}
