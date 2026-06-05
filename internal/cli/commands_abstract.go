package cli

import (
	"context"
	"fmt"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

func (c *CLI) runAbstract(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageAbstract)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageAbstract)
		return 2
	}

	jsonOutput := false
	keys := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		keys = append(keys, arg)
	}

	if len(keys) == 0 {
		fmt.Fprintln(c.stderr, usageAbstract)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if jsonOutput {
		return c.runAbstractJSON(reader, keys)
	}
	return c.runAbstractText(reader, keys)
}

func (c *CLI) runAbstractText(reader backend.Reader, keys []string) int {
	for _, key := range keys {
		item, err := reader.GetItem(context.Background(), key)
		if err != nil {
			fmt.Fprintf(c.stderr, "%s: %v\n", key, err)
			continue
		}
		if item.Abstract != "" {
			fmt.Fprintf(c.stdout, "[%s] %s\n\n%s\n", key, shortTitle(item.Title), item.Abstract)
		} else {
			fmt.Fprintf(c.stdout, "[%s] %s\n(no abstract available)\n", key, shortTitle(item.Title))
		}
	}
	return 0
}

func (c *CLI) runAbstractJSON(reader backend.Reader, keys []string) int {
	items := make([]domain.Item, 0, len(keys))
	for _, key := range keys {
		item, err := reader.GetItem(context.Background(), key)
		if err != nil {
			return c.printErr(err)
		}
		items = append(items, item)
	}
	meta := map[string]any{
		"total": len(items),
	}
	c.appendReadMetadata(meta, reader)
	// Lean mode but include abstract (it's the whole point of this command)
	return c.writeJSON(jsonResponse{
		OK:      true,
		Command: "abstract",
		Data:    toLeanItems(items, true), // includeAbstract=true
		Meta:    meta,
	})
}

func shortTitle(title string) string {
	if len(title) <= 60 {
		return title
	}
	return title[:57] + "..."
}
