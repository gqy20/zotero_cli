package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

func (c *CLI) runCollections(args []string) int {
	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(args, usageCollections)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	collections, err := client.ListCollections(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if limit > 0 && len(collections) > limit {
		collections = collections[:limit]
	}

	if jsonOutput {
		meta := map[string]any{
			"total":       len(collections),
			"read_source": "web",
		}
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "collections",
			Data:    collections,
			Meta:    meta,
		})
	}

	if len(collections) == 0 {
		fmt.Fprintln(c.stdout, "no collections found")
		return 0
	}

	for _, collection := range collections {
		fmt.Fprintf(c.stdout, "%-10s  %-20s  items=%d  children=%d\n",
			collection.Key,
			collection.Name,
			collection.NumItems,
			collection.NumCollections,
		)
	}
	return 0
}

func (c *CLI) runNotes(args []string) int {
	var query string
	filteredArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--query" && i+1 < len(args) {
			query = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--query=") {
			query = strings.TrimPrefix(args[i], "--query=")
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(filteredArgs, usageNotes)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	var notes []domain.Note
	notes, err := reader.ListNotes(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if query != "" {
		notes = filterNotesByQuery(notes, query)
	}

	if limit > 0 && len(notes) > limit {
		notes = notes[:limit]
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(notes),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "notes",
			Data:    notes,
			Meta:    meta,
		})
	}

	readMeta := c.consumeReaderReadMetadata(reader)
	c.warnIfSnapshotRead(readMeta)

	visible := filterVisibleNotesLocal(notes)
	if len(visible) == 0 {
		fmt.Fprintln(c.stdout, "no readable notes found in text mode; use --json to inspect all notes")
		return 0
	}

	for _, note := range visible {
		parent := ""
		if note.ParentItemKey != "" {
			parent = fmt.Sprintf("  [%s]", note.ParentItemKey)
		}
		fmt.Fprintf(c.stdout, "%-10s%s  %s\n", note.Key, parent, note.Preview)
	}
	return 0
}

func (c *CLI) runTags(args []string) int {
	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(args, usageTags)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	tags, err := reader.ListTags(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if limit > 0 && len(tags) > limit {
		tags = tags[:limit]
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(tags),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "tags",
			Data:    tags,
			Meta:    meta,
		})
	}

	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	if len(tags) == 0 {
		fmt.Fprintln(c.stdout, "no tags found")
		return 0
	}

	for _, tag := range tags {
		fmt.Fprintf(c.stdout, "%-20s  items=%d\n", tag.Name, tag.NumItems)
	}
	return 0
}

func (c *CLI) runSearches(args []string) int {
	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(args, usageSearches)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	searches, err := client.ListSearches(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if limit > 0 && len(searches) > limit {
		searches = searches[:limit]
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "searches",
			Data:    searches,
			Meta: map[string]any{
				"total":       len(searches),
				"read_source": "web",
			},
		})
	}

	if len(searches) == 0 {
		fmt.Fprintln(c.stdout, "no saved searches found")
		return 0
	}

	for _, search := range searches {
		fmt.Fprintf(c.stdout, "%-10s  %-24s  conditions=%d\n", search.Key, search.Name, search.NumConditions)
	}
	return 0
}

func (c *CLI) runDeleted(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageDeleted)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	deleted, err := client.GetDeleted(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "deleted",
			Data:    deleted,
			Meta: map[string]any{
				"total":       len(deleted.Items) + len(deleted.Collections) + len(deleted.Searches) + len(deleted.Tags),
				"read_source": "web",
			},
		})
	}

	fmt.Fprintf(c.stdout, "collections=%d\n", len(deleted.Collections))
	fmt.Fprintf(c.stdout, "searches=%d\n", len(deleted.Searches))
	fmt.Fprintf(c.stdout, "items=%d\n", len(deleted.Items))
	fmt.Fprintf(c.stdout, "tags=%d\n", len(deleted.Tags))
	return 0
}

func (c *CLI) runVersions(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageChanges)
	}
	objectType, opts, jsonOutput, err := parseVersionsArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageChanges)
		return 2
	}

	_, client, exitCode := c.loadClient()
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
		return c.printErr(err)
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

		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "changes",
			Data:    result.Versions,
			Meta:    meta,
		})
	}

	if result.NotModified {
		fmt.Fprintf(c.stdout, "not modified since version %d\n", opts.IfModifiedSinceVersion)
		return 0
	}

	for key, version := range result.Versions {
		fmt.Fprintf(c.stdout, "%-10s  %d\n", key, version)
	}
	return 0
}
func (c *CLI) runOverview(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageOverview)
	}

	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageOverview)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	ctx := context.Background()

	type result struct {
		stats       backend.LibraryStats
		statsErr    error
		collections []backend.Collection
		tags        []backend.Tag
		items       []domain.Item
	}

	var r result
	var wg sync.WaitGroup

	wg.Add(4)

	go func() {
		defer wg.Done()
		r.stats, r.statsErr = reader.GetLibraryStats(ctx)
	}()

	go func() {
		defer wg.Done()
		if colls, err := reader.ListCollections(ctx); err == nil {
			r.collections = colls
		}
	}()

	go func() {
		defer wg.Done()
		if tagList, err := reader.ListTags(ctx); err == nil {
			r.tags = tagList
		}
	}()

	go func() {
		defer wg.Done()
		if itemList, err := reader.FindItems(ctx, backend.FindOptions{Limit: 5}); err == nil {
			r.items = itemList
		}
	}()

	wg.Wait()

	if r.statsErr != nil {
		return c.jsonError(r.statsErr, "overview")
	}

	stats := r.stats
	if len(r.collections) > 5 {
		r.collections = r.collections[:5]
	}
	if len(r.tags) > 10 {
		r.tags = r.tags[:10]
	}

	// Index status — simple mode-based check
	indexStatus := "unavailable"
	if cfg.Mode == "local" || cfg.Mode == "hybrid" {
		if cfg.DataDir != "" {
			if _, statErr := os.Stat(cfg.DataDir); statErr == nil {
				indexStatus = "available"
			} else {
				indexStatus = "data_dir_missing"
			}
		} else {
			indexStatus = "not_configured"
		}
	}

	if jsonOutput {
		meta := map[string]any{
			"mode":               stats.LibraryType,
			"library_id":         stats.LibraryID,
			"total_items":        stats.TotalItems,
			"collections_count":  len(r.collections),
			"tags_count":         len(r.tags),
			"recent_items_count": len(r.items),
			"index_status":       indexStatus,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK: true, Command: "overview",
			Data: map[string]any{
				"stats":        stats,
				"collections":  r.collections,
				"tags":         r.tags,
				"recent_items": r.items,
			},
			Meta: meta,
		})
	}

	// Text output
	fmt.Fprintf(c.stdout, "Library: %s:%s\n", stats.LibraryType, stats.LibraryID)
	fmt.Fprintf(c.stdout, "Items: %d | Collections: %d | Searches: %d\n", stats.TotalItems, stats.TotalCollections, stats.TotalSearches)
	if stats.LastLibraryVersion > 0 {
		fmt.Fprintf(c.stdout, "Version: %d\n", stats.LastLibraryVersion)
	}
	fmt.Fprintln(c.stdout)

	if len(r.collections) > 0 {
		fmt.Fprintln(c.stdout, "Top Collections:")
		for _, col := range r.collections {
			fmt.Fprintf(c.stdout, "  %s (%d items)\n", col.Name, col.NumItems)
		}
	}

	if len(r.tags) > 0 {
		fmt.Fprintln(c.stdout, "Top Tags:")
		for _, tag := range r.tags {
			fmt.Fprintf(c.stdout, "  %s (%d items)\n", tag.Name, tag.NumItems)
		}
	}

	if len(r.items) > 0 {
		fmt.Fprintln(c.stdout, "Recent Items:")
		for _, item := range r.items {
			creators := shortCreators(item.Creators)
			fmt.Fprintf(c.stdout, "  %-10s  %-40s  %s\n", item.Key, truncate(item.Title, 38), creators)
		}
	}

	fmt.Fprintf(c.stdout, "Index: %s\n", indexStatus)
	return 0
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (c *CLI) runSchema(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageSchema)
	}

	if len(args) == 0 {
		fmt.Fprintf(c.stderr, "usage: zot schema <subcommand> [--json]\n")
		fmt.Fprintf(c.stderr, "subcommands: types, fields, creator-types, fields-for, creator-types-for, template\n\n")
		fmt.Fprint(c.stdout, usageSchema)
		return ExitUsage
	}

	switch args[0] {
	case "types":
		return c.runItemTypes(args[1:])
	case "fields":
		return c.runItemFields(args[1:])
	case "creator-types":
		return c.runCreatorFields(args[1:])
	case "fields-for":
		if len(args) < 2 {
			fmt.Fprintf(c.stderr, "usage: zot schema fields-for <item-type> [--json]\n")
			return ExitUsage
		}
		return c.runItemTypeFields(args[1:])
	case "creator-types-for":
		if len(args) < 2 {
			fmt.Fprintf(c.stderr, "usage: zot schema creator-types-for <item-type> [--json]\n")
			return ExitUsage
		}
		return c.runItemTypeCreatorTypes(args[1:])
	case "template":
		if len(args) < 2 {
			fmt.Fprintf(c.stderr, "usage: zot schema template <item-type> [--json]\n")
			return ExitUsage
		}
		return c.runItemTemplate(args[1:])
	default:
		fmt.Fprintf(c.stderr, "unknown schema subcommand: %s\n", args[0])
		fmt.Fprintf(c.stderr, "subcommands: types, fields, creator-types, fields-for, creator-types-for, template\n")
		return ExitUsage
	}
}

func (c *CLI) runItemTypes(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageItemTypes)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypes(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-types", values, jsonOutput)
}

func (c *CLI) runItemFields(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageItemFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemFields(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-fields", values, jsonOutput)
}

func (c *CLI) runCreatorFields(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageCreatorFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListCreatorFields(context.Background(), cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("creator-fields", values, jsonOutput)
}

func (c *CLI) runItemTypeFields(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTypeFields)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeFields(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-type-fields", values, jsonOutput)
}

func (c *CLI) runItemTypeCreatorTypes(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTypeCreatorTypes)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeCreatorTypes(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return c.printErr(err)
	}

	return c.renderLocalizedValues("item-type-creator-types", values, jsonOutput)
}

func (c *CLI) runItemTemplate(args []string) int {
	itemType, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageItemTemplate)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	template, err := client.GetItemTemplate(context.Background(), itemType)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "item-template",
			Data:    template,
		})
	}

	return c.writeJSON(template)
}

func (c *CLI) runKeyInfo(args []string) int {
	key, jsonOutput, ok, helpPrinted := c.parseSingleValueCommand(args, usageKeyInfo)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	info, err := client.GetKeyInfo(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "key-info",
			Data:    info,
		})
	}

	fmt.Fprintf(c.stdout, "user_id=%d\n", info.UserID)
	if len(info.Access) > 0 {
		return c.writeJSON(info.Access)
	}
	return 0
}

func (c *CLI) runGroups(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageGroups)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	keyInfo, err := client.GetCurrentKeyInfo(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	groups, err := client.ListGroupsForUser(context.Background(), fmt.Sprintf("%d", keyInfo.UserID))
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "groups",
			Data:    groups,
			Meta: map[string]any{
				"total": len(groups),
			},
		})
	}

	if len(groups) == 0 {
		fmt.Fprintln(c.stdout, "no groups found for the current api key")
		return 0
	}

	for _, group := range groups {
		fmt.Fprintf(c.stdout, "%-8d  %s\n", group.ID, group.Name)
	}
	return 0
}

func (c *CLI) runTrash(args []string) int {
	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(args, usageTrash)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	items, err := client.ListTrashItems(context.Background(), zoteroapi.FindOptions{Limit: limit})
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "trash",
			Data:    items,
			Meta: map[string]any{
				"total":       len(items),
				"read_source": "web",
			},
		})
	}

	if len(items) == 0 {
		fmt.Fprintln(c.stdout, "trash is empty")
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(c.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreatorsAPI(item.Creators),
			item.Title,
		)
	}
	return 0
}

func (c *CLI) runCollectionsTop(args []string) int {
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageCollectionsTop)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	collections, err := client.ListTopCollections(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "collections-top",
			Data:    collections,
			Meta: map[string]any{
				"total":       len(collections),
				"read_source": "web",
			},
		})
	}

	if len(collections) == 0 {
		fmt.Fprintln(c.stdout, "no top-level collections found")
		return 0
	}

	for _, collection := range collections {
		fmt.Fprintf(c.stdout, "%-10s  %-20s  items=%d  children=%d\n",
			collection.Key,
			collection.Name,
			collection.NumItems,
			collection.NumCollections,
		)
	}
	return 0
}

func (c *CLI) runPublications(args []string) int {
	jsonOutput, limit, ok, helpPrinted := c.parseJSONAndLimitArgs(args, usagePublications)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	items, err := client.ListPublicationsItems(context.Background(), zoteroapi.FindOptions{Limit: limit})
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "publications",
			Data:    items,
			Meta: map[string]any{
				"total":       len(items),
				"read_source": "web",
			},
		})
	}

	if len(items) == 0 {
		fmt.Fprintln(c.stdout, "no publications found")
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(c.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreatorsAPI(item.Creators),
			item.Title,
		)
	}
	return 0
}

func (c *CLI) renderLocalizedValues(command string, values []zoteroapi.LocalizedValue, jsonOutput bool) int {
	if jsonOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: command,
			Data:    values,
			Meta: map[string]any{
				"total": len(values),
			},
		})
	}

	for _, value := range values {
		fmt.Fprintf(c.stdout, "%-18s  %s\n", value.ID, value.Localized)
	}
	return 0
}
