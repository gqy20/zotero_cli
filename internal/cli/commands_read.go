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

func (c *CLI) runFind(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageFind)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}

	parsed, err := parseFindArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}
	opts := parsed.Opts
	jsonOutput := parsed.JSONOutput
	snippet := parsed.Snippet
	queryProvided := parsed.QueryProvided

	if opts.FullTextAny && !opts.FullText {
		fmt.Fprintln(c.stderr, "error: --fulltext-any requires --fulltext")
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}

	if strings.TrimSpace(opts.Query) == "" && !opts.All && !queryProvided {
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	requestedIncludeFields := append([]string(nil), opts.IncludeFields...)
	injectedAttachments := false
	if snippet && !fieldIncluded(opts.IncludeFields, "attachments") {
		opts.IncludeFields = append(opts.IncludeFields, "attachments")
		injectedAttachments = true
	}

	items, err := reader.FindItems(context.Background(), opts)
	if err != nil {
		return c.printErr(err)
	}
	items = filterDefaultFindItems(items, opts)

	if snippet {
		snippeter, ok := reader.(snippetReader)
		if !ok {
			return c.printErr(fmt.Errorf("find --snippet requires local or hybrid mode with local data"))
		}
		for i := range items {
			preview, err := snippeter.FullTextSnippet(context.Background(), items[i], opts.Query)
			if err != nil {
				return c.printErr(err)
			}
			items[i].FullTextPreview = preview
		}
		if injectedAttachments {
			for i := range items {
				items[i].Attachments = nil
			}
		}
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(items),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "find",
			Data:    items,
			Meta:    meta,
		})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	if opts.Full || len(opts.IncludeFields) > 0 || snippet {
		renderOpts := opts
		renderOpts.IncludeFields = append([]string(nil), requestedIncludeFields...)
		if snippet && !fieldIncluded(renderOpts.IncludeFields, "full_text_preview") {
			renderOpts.IncludeFields = append(renderOpts.IncludeFields, "full_text_preview")
		}
		for index, item := range items {
			c.renderFindItemDetailed(item, renderOpts)
			if index < len(items)-1 {
				fmt.Fprintln(c.stdout)
			}
		}
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(c.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreators(item.Creators),
			item.Title,
		)
	}
	return 0
}

func (c *CLI) runStats(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageStats)
	}
	jsonOutput, ok := c.parseJSONOnlyArgs(args, usageStats)
	if !ok {
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	stats, err := reader.GetLibraryStats(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{
			"total": stats.TotalItems,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{OK: true, Command: "stats", Data: stats, Meta: meta})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))
	fmt.Fprintf(c.stdout, "library=%s:%s\n", stats.LibraryType, stats.LibraryID)
	fmt.Fprintf(c.stdout, "items=%d\n", stats.TotalItems)
	fmt.Fprintf(c.stdout, "collections=%d\n", stats.TotalCollections)
	fmt.Fprintf(c.stdout, "searches=%d\n", stats.TotalSearches)
	if stats.LastLibraryVersion > 0 {
		fmt.Fprintf(c.stdout, "last_library_version=%d\n", stats.LastLibraryVersion)
	}
	return 0
}

func (c *CLI) runShow(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageShow)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	jsonOutput := false
	snippet := false
	key := ""
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--snippet" {
			snippet = true
			continue
		}
		if key == "" {
			key = arg
			continue
		}
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	if snippet {
		previewer, ok := reader.(previewReader)
		if !ok {
			return c.printErr(fmt.Errorf("show --snippet requires local or hybrid mode with local data"))
		}
		preview, err := previewer.FullTextPreview(context.Background(), item)
		if err != nil {
			return c.printErr(err)
		}
		item.FullTextPreview = preview
	}

	if jsonOutput {
		meta := map[string]any{
			"total": 1,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "show",
			Data:    item,
			Meta:    meta,
		})
	}
	readMeta := c.consumeReaderReadMetadata(reader)
	c.warnIfSnapshotRead(readMeta)

	fmt.Fprintf(c.stdout, "Key: %s\n", item.Key)
	fmt.Fprintf(c.stdout, "Title: %s\n", item.Title)
	fmt.Fprintf(c.stdout, "Type: %s\n", item.ItemType)
	if len(item.Creators) > 0 {
		fmt.Fprintf(c.stdout, "Creators: %s\n", joinCreatorNames(item.Creators))
	}
	if item.Date != "" {
		fmt.Fprintf(c.stdout, "Date: %s\n", item.Date)
	}
	if item.Container != "" {
		fmt.Fprintf(c.stdout, "Container: %s\n", item.Container)
	}
	if item.Volume != "" {
		fmt.Fprintf(c.stdout, "Volume: %s\n", item.Volume)
	}
	if item.Issue != "" {
		fmt.Fprintf(c.stdout, "Issue: %s\n", item.Issue)
	}
	if item.Pages != "" {
		fmt.Fprintf(c.stdout, "Pages: %s\n", item.Pages)
	}
	if item.DOI != "" {
		fmt.Fprintf(c.stdout, "DOI: %s\n", item.DOI)
	}
	if item.URL != "" {
		fmt.Fprintf(c.stdout, "URL: %s\n", item.URL)
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(c.stdout, "Tags: %s\n", strings.Join(item.Tags, ", "))
	}
	if len(item.Collections) > 0 {
		fmt.Fprintf(c.stdout, "Collections: %s\n", joinCollectionNames(item.Collections))
	}
	if len(item.Attachments) > 0 {
		fmt.Fprintf(c.stdout, "Attachments: %d\n", len(item.Attachments))
		for _, attachment := range item.Attachments {
			fmt.Fprintf(c.stdout, "  - [%s] %s\n", attachmentKind(attachment), attachmentSummary(attachment))
			if pathLine := attachmentPathLine(attachment); pathLine != "" {
				fmt.Fprintf(c.stdout, "    %s\n", pathLine)
			}
		}
	}
	if len(item.Notes) > 0 {
		fmt.Fprintf(c.stdout, "Notes: %d\n", len(item.Notes))
		for _, note := range item.Notes {
			fmt.Fprintf(c.stdout, "  - %s\n", noteSummary(note))
		}
	}
	if item.FullTextPreview != "" {
		fmt.Fprintf(c.stdout, "Full Text Preview: %s\n", item.FullTextPreview)
		if line := fullTextSourceLine(readMeta); line != "" {
			fmt.Fprintln(c.stdout, line)
		}
	}
	return 0
}

func (c *CLI) runRelate(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageRelate)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageRelate)
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
		fmt.Fprintln(c.stderr, usageRelate)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(c.stderr, usageRelate)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	relations, err := reader.GetRelated(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{OK: true, Command: "relate", Data: relations, Meta: meta})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	if len(relations) == 0 {
		fmt.Fprintf(c.stdout, "Item: %s\n", key)
		fmt.Fprintln(c.stdout, "Explicit Relations: 0")
		return 0
	}

	fmt.Fprintf(c.stdout, "Item: %s\n", key)
	fmt.Fprintf(c.stdout, "Explicit Relations: %d\n", len(relations))
	for _, relation := range relations {
		fmt.Fprintf(c.stdout, "  - [%s][%s] %s\n", relation.Predicate, relation.Direction, relateSummary(relation.Target))
	}
	return 0
}

func (c *CLI) appendReadMetadata(meta map[string]any, reader backend.Reader) {
	c.appendExplicitReadMetadata(meta, c.consumeReaderReadMetadata(reader))
}

func (c *CLI) appendExplicitReadMetadata(meta map[string]any, readMeta backend.ReadMetadata) {
	if readMeta.ReadSource != "" {
		meta["read_source"] = readMeta.ReadSource
	}
	if readMeta.SQLiteFallback {
		meta["sqlite_fallback"] = true
	}
	if readMeta.FullTextEngine != "" {
		meta["full_text_engine"] = readMeta.FullTextEngine
	}
	if readMeta.FullTextSource != "" {
		meta["full_text_source"] = readMeta.FullTextSource
	}
	if readMeta.FullTextAttachmentKey != "" {
		meta["full_text_attachment_key"] = readMeta.FullTextAttachmentKey
	}
	if readMeta.FullTextCacheHit {
		meta["full_text_cache_hit"] = true
	}
}

func (c *CLI) consumeReaderReadMetadata(reader backend.Reader) backend.ReadMetadata {
	reporter, ok := reader.(interface{ ConsumeReadMetadata() backend.ReadMetadata })
	if !ok {
		return backend.ReadMetadata{}
	}
	return reporter.ConsumeReadMetadata()
}

func (c *CLI) warnIfSnapshotRead(readMeta backend.ReadMetadata) {
	if readMeta.ReadSource != "snapshot" && !readMeta.SQLiteFallback {
		return
	}
	fmt.Fprintln(c.stderr, "note: using snapshot fallback for local Zotero data")
}

func fullTextSourceLine(readMeta backend.ReadMetadata) string {
	if readMeta.FullTextSource == "" {
		return ""
	}
	line := "Full Text Source: " + readMeta.FullTextSource
	if readMeta.FullTextCacheHit {
		line += " (cache hit)"
	}
	if readMeta.FullTextAttachmentKey != "" {
		line += " [" + readMeta.FullTextAttachmentKey + "]"
	}
	return line
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

func (c *CLI) runCite(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCite)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageCite)
		return 2
	}

	key, opts, jsonOutput, err := parseCiteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCite)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
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
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "cite",
			Data:    result,
			Meta: map[string]any{
				"total": 1,
			},
		})
	}

	fmt.Fprintln(c.stdout, result.Text)
	return 0
}

func (c *CLI) runExport(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageExport)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageExport)
		return 2
	}

	exportParsed, err := parseExportArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageExport)
		return 2
	}
	itemKey := exportParsed.ItemKey
	collectionKey := exportParsed.CollectionKey
	findOpts := exportParsed.FindOpts
	format := exportParsed.Format
	jsonOutput := exportParsed.JSONOutput

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	keys := make([]string, 0, 8)
	if format == "csljson" && cfg.Mode != "web" {
		result, readMeta, handled, err := c.tryLocalCSLJSONExport(context.Background(), cfg, itemKey, collectionKey, findOpts)
		if handled {
			if err != nil {
				return c.printErr(err)
			}
			if jsonOutput {
				meta := map[string]any{
					"total": len(result.Data.([]map[string]any)),
				}
				c.appendExplicitReadMetadata(meta, readMeta)
				return c.writeJSON(jsonResponse{
					OK:      true,
					Command: "export",
					Data:    result,
					Meta:    meta,
				})
			}
			c.warnIfSnapshotRead(readMeta)
			return c.writeJSON(result.Data)
		}
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		items, err := client.ListCollectionItems(context.Background(), collectionKey, findOpts)
		if err != nil {
			return c.printErr(err)
		}
		items = filterDefaultFindItemsAPI(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	} else {
		items, err := client.FindItems(context.Background(), findOpts)
		if err != nil {
			return c.printErr(err)
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
		return c.printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(keys),
		}
		c.appendExplicitReadMetadata(meta, backend.ReadMetadata{ReadSource: "web"})
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "export",
			Data:    result,
			Meta:    meta,
		})
	}

	if result.Text != "" {
		fmt.Fprintln(c.stdout, result.Text)
		return 0
	}
	return c.writeJSON(result.Data)
}

func (c *CLI) tryLocalCSLJSONExport(ctx context.Context, cfg config.Config, itemKey string, collectionKey string, findOpts zoteroapi.FindOptions) (zoteroapi.ExportResult, backend.ReadMetadata, bool, error) {
	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
	}

	keys := make([]string, 0, 8)
	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		collectionReader, ok := localReader.(collectionItemKeyReader)
		if !ok {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, fmt.Errorf("local export requires collection access support")
		}
		keys, err = collectionReader.CollectionItemKeys(ctx, collectionKey, findOpts.Limit)
		if err != nil {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
		}
	} else {
		items, err := localReader.FindItems(ctx, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		if err != nil {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
		}
		items = filterDefaultFindItems(items, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}

	exporter, ok := localReader.(cslJSONExporter)
	if !ok {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, fmt.Errorf("local export requires CSL JSON export support")
	}
	payload, err := exporter.ExportItemsCSLJSON(ctx, keys)
	if err != nil {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
	}
	return zoteroapi.ExportResult{
		Format: "csljson",
		Data:   payload,
	}, c.consumeReaderReadMetadata(localReader), true, nil
}
