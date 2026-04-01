package cli

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

func runFind(args []string) int {
	if isHelpOnly(args) {
		return printCommandUsage(usageFind)
	}
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

	_, reader, exitCode := loadReader()
	if exitCode != 0 {
		return exitCode
	}

	items, err := reader.FindItems(context.Background(), opts)
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
	if isHelpOnly(args) {
		return printCommandUsage(usageStats)
	}
	jsonOutput, ok := parseJSONOnlyArgs(args, usageStats)
	if !ok {
		return 2
	}

	_, reader, exitCode := loadReader()
	if exitCode != 0 {
		return exitCode
	}

	stats, err := reader.GetLibraryStats(context.Background())
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
	if isHelpOnly(args) {
		return printCommandUsage(usageShow)
	}
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

	_, reader, exitCode := loadReader()
	if exitCode != 0 {
		return exitCode
	}

	item, err := reader.GetItem(context.Background(), key)
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
	if item.Volume != "" {
		fmt.Fprintf(stdout, "Volume: %s\n", item.Volume)
	}
	if item.Issue != "" {
		fmt.Fprintf(stdout, "Issue: %s\n", item.Issue)
	}
	if item.Pages != "" {
		fmt.Fprintf(stdout, "Pages: %s\n", item.Pages)
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
	if len(item.Collections) > 0 {
		fmt.Fprintf(stdout, "Collections: %s\n", joinCollectionNames(item.Collections))
	}
	if len(item.Attachments) > 0 {
		fmt.Fprintf(stdout, "Attachments: %d\n", len(item.Attachments))
		for _, attachment := range item.Attachments {
			fmt.Fprintf(stdout, "  - [%s] %s\n", attachmentKind(attachment), attachmentSummary(attachment))
			if pathLine := attachmentPathLine(attachment); pathLine != "" {
				fmt.Fprintf(stdout, "    %s\n", pathLine)
			}
		}
	}
	if len(item.Notes) > 0 {
		fmt.Fprintf(stdout, "Notes: %d\n", len(item.Notes))
		for _, note := range item.Notes {
			fmt.Fprintf(stdout, "  - %s\n", noteSummary(note))
		}
	}
	return 0
}

func runRelate(args []string) int {
	if isHelpOnly(args) {
		return printCommandUsage(usageRelate)
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageRelate)
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
		fmt.Fprintln(stderr, usageRelate)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(stderr, usageRelate)
		return 2
	}

	_, reader, exitCode := loadReader()
	if exitCode != 0 {
		return exitCode
	}

	relations, err := reader.GetRelated(context.Background(), key)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "relate", Data: relations})
	}

	if len(relations) == 0 {
		fmt.Fprintf(stdout, "Item: %s\n", key)
		fmt.Fprintln(stdout, "Explicit Relations: 0")
		return 0
	}

	fmt.Fprintf(stdout, "Item: %s\n", key)
	fmt.Fprintf(stdout, "Explicit Relations: %d\n", len(relations))
	for _, relation := range relations {
		fmt.Fprintf(stdout, "  - [%s][%s] %s\n", relation.Predicate, relation.Direction, relateSummary(relation.Target))
	}
	return 0
}

func relateSummary(ref domain.ItemRef) string {
	if ref.Title == "" {
		return ref.Key
	}
	if ref.ItemType == "" {
		return ref.Key + "  " + ref.Title
	}
	return ref.Key + "  " + ref.ItemType + "  " + ref.Title
}

func runCite(args []string) int {
	if isHelpOnly(args) {
		return printCommandUsage(usageCite)
	}
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
	if isHelpOnly(args) {
		return printCommandUsage(usageExport)
	}
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

	cfg, exitCode := loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	keys := make([]string, 0, 8)
	if format == "csljson" && cfg.Mode != "web" {
		result, handled, err := tryLocalCSLJSONExport(context.Background(), cfg, itemKey, collectionKey, findOpts)
		if handled {
			if err != nil {
				return printErr(err)
			}
			if jsonOutput {
				return writeJSON(jsonResponse{
					OK:      true,
					Command: "export",
					Data:    result,
					Meta: map[string]any{
						"total": len(result.Data.([]map[string]any)),
					},
				})
			}
			return writeJSON(result.Data)
		}
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		items, err := client.ListCollectionItems(context.Background(), collectionKey, findOpts)
		if err != nil {
			return printErr(err)
		}
		items = filterDefaultFindItemsAPI(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	} else {
		items, err := client.FindItems(context.Background(), findOpts)
		if err != nil {
			return printErr(err)
		}
		items = filterDefaultFindItemsAPI(items, findOpts)
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

func tryLocalCSLJSONExport(ctx context.Context, cfg config.Config, itemKey string, collectionKey string, findOpts zoteroapi.FindOptions) (zoteroapi.ExportResult, bool, error) {
	localReader, err := backend.NewLocalReader(cfg)
	if err != nil {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, false, nil
		}
		return zoteroapi.ExportResult{}, true, err
	}

	keys := make([]string, 0, 8)
	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		keys, err = localReader.CollectionItemKeys(ctx, collectionKey, findOpts.Limit)
		if err != nil {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, false, nil
			}
			return zoteroapi.ExportResult{}, true, err
		}
	} else {
		items, err := localReader.FindItems(ctx, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		if err != nil {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, false, nil
			}
			return zoteroapi.ExportResult{}, true, err
		}
		items = filterDefaultFindItems(items, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}

	payload, err := localReader.ExportItemsCSLJSON(ctx, keys)
	if err != nil {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, false, nil
		}
		return zoteroapi.ExportResult{}, true, err
	}
	return zoteroapi.ExportResult{
		Format: "csljson",
		Data:   payload,
	}, true, nil
}
