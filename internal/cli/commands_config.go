package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func (c *CLI) runConfig(args []string) int {
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
				fmt.Fprintf(c.stderr, "config not found; run `zot config init` first\n")
				return 3
			}
			return c.printErr(err)
		}

		return c.writeJSON(map[string]any{
			"path":   path,
			"config": maskConfig(cfg),
		})
	case "validate":
		return c.runConfigValidate()
	case "init":
		return c.runConfigInit(args[1:])
	default:
		fmt.Fprintf(c.stderr, "unknown config command: %s\n\n", args[0])
		c.printConfigUsage()
		return 2
	}
}

func (c *CLI) runConfigInit(args []string) int {
	path, err := config.DefaultPath()
	if err != nil {
		return c.printErr(err)
	}

	if len(args) > 0 && args[0] == "--example" {
		cfg := config.Default()
		cfg.LibraryType = "user"
		cfg.LibraryID = "123456"
		cfg.APIKey = "replace-me"
		fmt.Fprint(c.stdout, strings.Join([]string{
			"ZOT_MODE=web",
			"ZOT_DATA_DIR=",
			"ZOT_LIBRARY_TYPE=user",
			"ZOT_LIBRARY_ID=123456",
			"ZOT_API_KEY=replace-me",
			"ZOT_STYLE=apa",
			"ZOT_LOCALE=en-US",
			fmt.Sprintf("ZOT_TIMEOUT_SECONDS=%d", cfg.TimeoutSeconds),
			fmt.Sprintf("ZOT_RETRY_MAX_ATTEMPTS=%d", cfg.RetryMaxAttempts),
			fmt.Sprintf("ZOT_RETRY_BASE_DELAY_MS=%d", cfg.RetryBaseDelayMilliseconds),
			fmt.Sprintf("ZOT_ALLOW_WRITE=%d", boolToInt(cfg.AllowWrite)),
			fmt.Sprintf("ZOT_ALLOW_DELETE=%d", boolToInt(cfg.AllowDelete)),
			"",
		}, "\n"))
		return 0
	}

	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(c.stderr, "config already exists at %s\n", path)
		fmt.Fprintf(c.stderr, "edit it manually or remove it before re-running init\n")
		return 3
	} else if !errors.Is(err, os.ErrNotExist) {
		return c.printErr(err)
	}

	cfg, err := c.promptConfigSetup()
	if err != nil {
		return c.printErr(err)
	}
	if err := config.Save(cfg); err != nil {
		return c.printErr(err)
	}

	fmt.Fprintf(c.stdout, "created config at %s\n", path)
	fmt.Fprintln(c.stdout, "you can edit ~/.zot/.env later if you want to change keys or permissions")
	return 0
}

func (c *CLI) runConfigValidate() int {
	cfg, path, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(c.stderr, "config not found.")
			fmt.Fprintln(c.stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(c.stderr, "run `zot config init` to set them up interactively in ~/.zot/.env")
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
		"data_dir_configured": strings.TrimSpace(cfg.DataDir) != "",
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
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
