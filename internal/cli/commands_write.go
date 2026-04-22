package cli

import (
	"context"
	"fmt"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

func (c *CLI) runCreateItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateItem)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	// Hybrid write: if Zotero is not running, create note via local SQLite
	if (cfg.Mode == "local" || cfg.Mode == "hybrid") && !isZoteroRunning() {
		itemType, _ := data["itemType"].(string)
		if itemType == "note" {
			localResult, localErr := createNoteLocally(cfg, data)
			if localErr != nil {
				fmt.Fprintf(c.stderr, "local write failed, falling back to web API: %v\n", localErr)
			} else {
				if jsonOutput {
					return c.writeJSON(jsonResponse{OK: true, Command: "create-item", Data: map[string]any{
						"key":                   localResult.Key,
						"last_modified_version": localResult.ItemID,
						"write_source":          "local",
					}})
				}
				fmt.Fprintf(c.stdout, "created item %s locally (SQLite)\n", localResult.Key)
				return 0
			}
		}
	}

	result, err := client.CreateItem(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "created item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateItem)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, true)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateItem(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteItem(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteItem)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteItem)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE item %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteItem(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-item", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runAddTag(args []string) int {
	return c.runUpdateTags(args, usageAddTag, "add-tag", true)
}

func (c *CLI) runRemoveTag(args []string) int {
	return c.runUpdateTags(args, usageRemoveTag, "remove-tag", false)
}

func (c *CLI) runUpdateTags(args []string, usage string, command string, add bool) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usage)
	}
	keys, version, tag, jsonOutput, err := parseKeysListArgs(args, false, true)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usage)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	items, err := client.GetItemsByKeys(context.Background(), keys)
	if err != nil {
		return c.printErr(err)
	}

	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		updatedTags := mutateTags(item.Tags, tag, add)
		payload = append(payload, map[string]any{
			"key":     item.Key,
			"version": item.Version,
			"tags":    toAPITags(updatedTags),
		})
	}

	result, err := client.UpdateItems(context.Background(), payload, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: command, Data: result})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Fprintf(c.stdout, "%s tag %q on %d items at library version %d\n", action, tag, len(keys), result.LastModifiedVersion)
	return 0
}

func (c *CLI) runCreateCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateCollection)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateCollection(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "created collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateCollection)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateCollection(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteCollection(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteCollection)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteCollection)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE collection %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteCollection(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-collection", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runCreateSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageCreateSearch)
	}
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageCreateSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateSearch(context.Background(), data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "create-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "created search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runUpdateSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageUpdateSearch)
	}
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageUpdateSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateSearch(context.Background(), key, data, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "update-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "updated search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func (c *CLI) runDeleteSearch(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageDeleteSearch)
	}
	key, version, jsonOutput, yesFlag, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageDeleteSearch)
		return 2
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	if !jsonOutput && !yesFlag {
		fmt.Fprintf(c.stderr, "⚠  You are about to DELETE search %s. This action cannot be undone.\n", key)
		if !c.confirm("Proceed with deletion") {
			fmt.Fprintln(c.stderr, "deletion cancelled")
			return 130
		}
	}

	result, err := client.DeleteSearch(context.Background(), key, version)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "delete-search", Data: result})
	}
	fmt.Fprintf(c.stdout, "deleted search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func createNoteLocally(cfg config.Config, data map[string]any) (backend.LocalCreateNoteResult, error) {
	parentKey, _ := data["parentItem"].(string)
	noteHTML, _ := data["note"].(string)
	if parentKey == "" || noteHTML == "" {
		return backend.LocalCreateNoteResult{}, fmt.Errorf("parentItem and note fields are required for local note creation")
	}

	localReader, err := backend.NewLocalReader(cfg)
	if err != nil {
		return backend.LocalCreateNoteResult{}, err
	}

	return localReader.CreateLocalNote(context.Background(), parentKey, noteHTML)
}
