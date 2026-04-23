package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func (c *CLI) runConfig(args []string) int {
	if isHelpOnly(args) {
		c.printConfigUsage()
		return 0
	}
	if len(args) == 0 {
		c.printConfigUsage()
		return 0
	}

	switch args[0] {
	case "path":
		path, err := config.DefaultPath()
		if err != nil {
			return c.printErr(err)
		}
		fmt.Fprintln(c.stdout, path)
		return 0
	case "show":
		cfg, path, err := config.Load()
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				fmt.Fprintf(c.stderr, "config not found; run `zot init` first\n")
				return 3
			}
			return c.printErr(err)
		}

		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "config-show",
			Data: map[string]any{
				"path":   path,
				"config": maskConfig(cfg),
			},
		})
	case "validate":
		return c.runConfigValidate()
	case "init":
		fmt.Fprintln(c.stderr, "`zot config init` has been replaced by `zot init`")
		fmt.Fprintln(c.stderr, "run `zot init` for streamlined setup with mode selection and optional PyMuPDF installation")
		return 2
	default:
		fmt.Fprintf(c.stderr, "unknown config command: %s\n\n", args[0])
		c.printConfigUsage()
		return 2
	}
}

func (c *CLI) runConfigValidate() int {
	cfg, path, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(c.stderr, "config not found.")
			fmt.Fprintln(c.stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(c.stderr, "run `zot init` to set them up interactively in ~/.zot/.env")
			return 3
		}
		return c.printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)

	result, err := client.ValidateLibraryAccess(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	return c.writeJSON(jsonResponse{
		OK:      true,
		Command: "config-validate",
		Data:    result,
		Meta:    configValidateMeta(cfg, path),
	})
}

func configValidateMeta(cfg config.Config, path string) map[string]any {
	meta := map[string]any{
		"config_path":         path,
		"mode":                cfg.Mode,
		"data_dir_configured": cfg.DataDir != "",
	}
	if cfg.DataDir == "" {
		return meta
	}

	if _, err := backend.NewLocalReader(cfg); err != nil {
		meta["local_reader_available"] = false
		meta["local_reader_error"] = err.Error()
		return meta
	}
	meta["local_reader_available"] = true
	return meta
}
