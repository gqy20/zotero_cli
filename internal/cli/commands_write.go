package cli

import (
	"context"
	"fmt"
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
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
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
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
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
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
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
