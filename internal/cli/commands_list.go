package cli

import (
	"context"
	"fmt"

	"zotero_cli/internal/zoteroapi"
)

func runCollections(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usageCollections)
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

	if limit > 0 && len(collections) > limit {
		collections = collections[:limit]
	}

	if jsonOutput {
		meta := map[string]any{
			"total":       len(collections),
			"read_source": "web",
		}
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "collections",
			Data:    collections,
			Meta:    meta,
		})
	}

	if len(collections) == 0 {
		fmt.Fprintln(defaultCLI.stdout, "no collections found")
		return 0
	}

	for _, collection := range collections {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %-20s  items=%d  children=%d\n",
			collection.Key,
			collection.Name,
			collection.NumItems,
			collection.NumCollections,
		)
	}
	return 0
}

func runNotes(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usageNotes)
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

	if limit > 0 && len(notes) > limit {
		notes = notes[:limit]
	}

	if jsonOutput {
		meta := map[string]any{
			"total":       len(notes),
			"read_source": "web",
		}
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "notes",
			Data:    notes,
			Meta:    meta,
		})
	}

	visible := filterVisibleNotes(notes)
	if len(visible) == 0 {
		fmt.Fprintln(defaultCLI.stdout, "no readable notes found in text mode; use --json to inspect all notes")
		return 0
	}

	for _, note := range visible {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %s\n", note.Key, notePreview(note.Content))
	}
	return 0
}

func runTags(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usageTags)
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

	if limit > 0 && len(tags) > limit {
		tags = tags[:limit]
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "tags",
			Data:    tags,
			Meta: map[string]any{
				"total":       len(tags),
				"read_source": "web",
			},
		})
	}

	if len(tags) == 0 {
		fmt.Fprintln(defaultCLI.stdout, "no tags found")
		return 0
	}

	for _, tag := range tags {
		fmt.Fprintf(defaultCLI.stdout, "%-20s  items=%d\n", tag.Name, tag.NumItems)
	}
	return 0
}

func runSearches(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usageSearches)
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

	if limit > 0 && len(searches) > limit {
		searches = searches[:limit]
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
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
		fmt.Fprintln(defaultCLI.stdout, "no saved searches found")
		return 0
	}

	for _, search := range searches {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %-24s  conditions=%d\n", search.Key, search.Name, search.NumConditions)
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
			Meta: map[string]any{
				"total":       len(deleted.Items) + len(deleted.Collections) + len(deleted.Searches) + len(deleted.Tags),
				"read_source": "web",
			},
		})
	}

	fmt.Fprintf(defaultCLI.stdout, "collections=%d\n", len(deleted.Collections))
	fmt.Fprintf(defaultCLI.stdout, "searches=%d\n", len(deleted.Searches))
	fmt.Fprintf(defaultCLI.stdout, "items=%d\n", len(deleted.Items))
	fmt.Fprintf(defaultCLI.stdout, "tags=%d\n", len(deleted.Tags))
	return 0
}

func runVersions(args []string) int {
	objectType, opts, jsonOutput, err := parseVersionsArgs(args)
	if err != nil {
		fmt.Fprintln(defaultCLI.stderr, "error:", err)
		fmt.Fprintln(defaultCLI.stderr, usageVersions)
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
		fmt.Fprintf(defaultCLI.stdout, "not modified since version %d\n", opts.IfModifiedSinceVersion)
		return 0
	}

	for key, version := range result.Versions {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %d\n", key, version)
	}
	return 0
}

func runItemTypes(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageItemTypes)
	if !ok {
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypes(context.Background(), cfg.Locale)
	if err != nil {
		return printErr(err)
	}

	return renderLocalizedValues("item-types", values, jsonOutput)
}

func runItemFields(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageItemFields)
	if !ok {
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemFields(context.Background(), cfg.Locale)
	if err != nil {
		return printErr(err)
	}

	return renderLocalizedValues("item-fields", values, jsonOutput)
}

func runCreatorFields(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageCreatorFields)
	if !ok {
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListCreatorFields(context.Background(), cfg.Locale)
	if err != nil {
		return printErr(err)
	}

	return renderLocalizedValues("creator-fields", values, jsonOutput)
}

func runItemTypeFields(args []string) int {
	itemType, jsonOutput, ok := parseSingleValueCommand(args, usageItemTypeFields)
	if !ok {
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeFields(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return printErr(err)
	}

	return renderLocalizedValues("item-type-fields", values, jsonOutput)
}

func runItemTypeCreatorTypes(args []string) int {
	itemType, jsonOutput, ok := parseSingleValueCommand(args, usageItemTypeCreatorTypes)
	if !ok {
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	values, err := client.ListItemTypeCreatorTypes(context.Background(), itemType, cfg.Locale)
	if err != nil {
		return printErr(err)
	}

	return renderLocalizedValues("item-type-creator-types", values, jsonOutput)
}

func runItemTemplate(args []string) int {
	itemType, jsonOutput, ok := parseSingleValueCommand(args, usageItemTemplate)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	template, err := client.GetItemTemplate(context.Background(), itemType)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "item-template",
			Data:    template,
		})
	}

	return writeJSON(template)
}

func runKeyInfo(args []string) int {
	key, jsonOutput, ok := parseSingleValueCommand(args, usageKeyInfo)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	info, err := client.GetKeyInfo(context.Background(), key)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "key-info",
			Data:    info,
		})
	}

	fmt.Fprintf(defaultCLI.stdout, "user_id=%d\n", info.UserID)
	if len(info.Access) > 0 {
		return writeJSON(info.Access)
	}
	return 0
}

func runGroups(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageGroups)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	keyInfo, err := client.GetCurrentKeyInfo(context.Background())
	if err != nil {
		return printErr(err)
	}

	groups, err := client.ListGroupsForUser(context.Background(), fmt.Sprintf("%d", keyInfo.UserID))
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "groups",
			Data:    groups,
			Meta: map[string]any{
				"total": len(groups),
			},
		})
	}

	if len(groups) == 0 {
		fmt.Fprintln(defaultCLI.stdout, "no groups found for the current api key")
		return 0
	}

	for _, group := range groups {
		fmt.Fprintf(defaultCLI.stdout, "%-8d  %s\n", group.ID, group.Name)
	}
	return 0
}

func runTrash(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usageTrash)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	items, err := client.ListTrashItems(context.Background(), zoteroapi.FindOptions{Limit: limit})
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
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
		fmt.Fprintln(defaultCLI.stdout, "trash is empty")
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreatorsAPI(item.Creators),
			item.Title,
		)
	}
	return 0
}

func runCollectionsTop(args []string) int {
	jsonOutput, ok := parseJSONOnlyArgs(args, usageCollectionsTop)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	collections, err := client.ListTopCollections(context.Background())
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
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
		fmt.Fprintln(defaultCLI.stdout, "no top-level collections found")
		return 0
	}

	for _, collection := range collections {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %-20s  items=%d  children=%d\n",
			collection.Key,
			collection.Name,
			collection.NumItems,
			collection.NumCollections,
		)
	}
	return 0
}

func runPublications(args []string) int {
	jsonOutput, limit, ok := parseJSONAndLimitArgs(args, usagePublications)
	if !ok {
		return 2
	}

	_, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}

	items, err := client.ListPublicationsItems(context.Background(), zoteroapi.FindOptions{Limit: limit})
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{
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
		fmt.Fprintln(defaultCLI.stdout, "no publications found")
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(defaultCLI.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreatorsAPI(item.Creators),
			item.Title,
		)
	}
	return 0
}

func renderLocalizedValues(command string, values []zoteroapi.LocalizedValue, jsonOutput bool) int {
	if jsonOutput {
		return writeJSON(jsonResponse{
			OK:      true,
			Command: command,
			Data:    values,
			Meta: map[string]any{
				"total": len(values),
			},
		})
	}

	for _, value := range values {
		fmt.Fprintf(defaultCLI.stdout, "%-18s  %s\n", value.ID, value.Localized)
	}
	return 0
}
