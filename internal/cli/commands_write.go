package cli

import (
	"context"
	"fmt"
)

func runCreateItem(args []string) int {
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageCreateItem)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateItem(context.Background(), data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "create-item", Data: result})
	}
	fmt.Fprintf(stdout, "created item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runUpdateItem(args []string) int {
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, true)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageUpdateItem)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateItem(context.Background(), key, data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "update-item", Data: result})
	}
	fmt.Fprintf(stdout, "updated item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runDeleteItem(args []string) int {
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageDeleteItem)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.DeleteItem(context.Background(), key, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "delete-item", Data: result})
	}
	fmt.Fprintf(stdout, "deleted item %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runCreateItems(args []string) int {
	data, version, jsonOutput, err := parseWriteBatchArgs(args, true)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageCreateItems)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateItems(context.Background(), data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "create-items", Data: result})
	}
	fmt.Fprintf(stdout, "created %d items at library version %d\n", len(result.Successful), result.LastModifiedVersion)
	return 0
}

func runUpdateItems(args []string) int {
	data, version, jsonOutput, err := parseWriteBatchArgs(args, false)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageUpdateItems)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateItems(context.Background(), data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "update-items", Data: result})
	}
	fmt.Fprintf(stdout, "updated %d items (%d unchanged) at library version %d\n", len(result.Successful), len(result.Unchanged), result.LastModifiedVersion)
	return 0
}

func runDeleteItems(args []string) int {
	keys, version, _, jsonOutput, err := parseKeysListArgs(args, false, false)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageDeleteItems)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.DeleteItems(context.Background(), keys, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "delete-items", Data: result})
	}
	fmt.Fprintf(stdout, "deleted %d items at library version %d\n", len(result.Successful), result.LastModifiedVersion)
	return 0
}

func runAddTag(args []string) int {
	return runUpdateTags(args, usageAddTag, "add-tag", true)
}

func runRemoveTag(args []string) int {
	return runUpdateTags(args, usageRemoveTag, "remove-tag", false)
}

func runUpdateTags(args []string, usage string, command string, add bool) int {
	keys, version, tag, jsonOutput, err := parseKeysListArgs(args, false, true)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usage)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	items, err := client.GetItemsByKeys(context.Background(), keys)
	if err != nil {
		return printErr(err)
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
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: command, Data: result})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Fprintf(stdout, "%s tag %q on %d items at library version %d\n", action, tag, len(keys), result.LastModifiedVersion)
	return 0
}

func runCreateCollection(args []string) int {
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageCreateCollection)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateCollection(context.Background(), data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "create-collection", Data: result})
	}
	fmt.Fprintf(stdout, "created collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runUpdateCollection(args []string) int {
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageUpdateCollection)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateCollection(context.Background(), key, data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "update-collection", Data: result})
	}
	fmt.Fprintf(stdout, "updated collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runDeleteCollection(args []string) int {
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageDeleteCollection)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.DeleteCollection(context.Background(), key, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "delete-collection", Data: result})
	}
	fmt.Fprintf(stdout, "deleted collection %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runCreateSearch(args []string) int {
	data, version, jsonOutput, err := parseWriteCreateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageCreateSearch)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.CreateSearch(context.Background(), data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "create-search", Data: result})
	}
	fmt.Fprintf(stdout, "created search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runUpdateSearch(args []string) int {
	key, data, version, jsonOutput, err := parseWriteUpdateArgs(args, false)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageUpdateSearch)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.UpdateSearch(context.Background(), key, data, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "update-search", Data: result})
	}
	fmt.Fprintf(stdout, "updated search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}

func runDeleteSearch(args []string) int {
	key, version, jsonOutput, err := parseWriteDeleteArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		fmt.Fprintln(stderr, usageDeleteSearch)
		return 2
	}

	cfg, client, exitCode := loadClient()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := ensureDeleteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	result, err := client.DeleteSearch(context.Background(), key, version)
	if err != nil {
		return printErr(err)
	}

	if jsonOutput {
		return writeJSON(jsonResponse{OK: true, Command: "delete-search", Data: result})
	}
	fmt.Fprintf(stdout, "deleted search %s at library version %d\n", result.Key, result.LastModifiedVersion)
	return 0
}
