package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func runConfig(args []string) int {
	if len(args) == 0 {
		printConfigUsage()
		return 0
	}

	switch args[0] {
	case "path":
		path, err := config.DefaultPath()
		if err != nil {
			return printErr(err)
		}
		fmt.Fprintln(stdout, path)
		return 0
	case "show":
		cfg, path, err := config.Load()
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				fmt.Fprintf(stderr, "config not found; run `zot config init` first\n")
				return 3
			}
			return printErr(err)
		}

		return writeJSON(map[string]any{
			"path":   path,
			"config": maskConfig(cfg),
		})
	case "validate":
		return runConfigValidate()
	case "init":
		return runConfigInit(args[1:])
	default:
		fmt.Fprintf(stderr, "unknown config command: %s\n\n", args[0])
		printConfigUsage()
		return 2
	}
}

func runConfigInit(args []string) int {
	path, err := config.DefaultPath()
	if err != nil {
		return printErr(err)
	}

	if len(args) > 0 && args[0] == "--example" {
		cfg := config.Default()
		cfg.LibraryType = "user"
		cfg.LibraryID = "123456"
		cfg.APIKey = "replace-me"
		fmt.Fprint(stdout, strings.Join([]string{
			"ZOT_MODE=web",
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
		fmt.Fprintf(stderr, "config already exists at %s\n", path)
		fmt.Fprintf(stderr, "edit it manually or remove it before re-running init\n")
		return 3
	} else if !errors.Is(err, os.ErrNotExist) {
		return printErr(err)
	}

	cfg, err := promptConfigSetup()
	if err != nil {
		return printErr(err)
	}
	if err := config.Save(cfg); err != nil {
		return printErr(err)
	}

	fmt.Fprintf(stdout, "created config at %s\n", path)
	fmt.Fprintln(stdout, "you can edit ~/.zot/.env later if you want to change keys or permissions")
	return 0
}

func runConfigValidate() int {
	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found.")
			fmt.Fprintln(stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(stderr, "run `zot config init` to set them up interactively in ~/.zot/.env")
			return 3
		}
		return printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	client := zoteroapi.New(cfg, baseURL, nil)

	result, err := client.ValidateLibraryAccess(context.Background())
	if err != nil {
		return printErr(err)
	}

	return writeJSON(jsonResponse{
		OK:      true,
		Command: "config-validate",
		Data:    result,
	})
}
