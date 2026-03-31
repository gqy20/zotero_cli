package cli

import (
	"errors"
	"fmt"
	"os"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func loadClient() (config.Config, *zoteroapi.Client, int) {
	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found.")
			fmt.Fprintln(stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(stderr, "run `zot config init` to set them up interactively in ~/.zot/.env")
			return config.Config{}, nil, 3
		}
		return config.Config{}, nil, printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	return cfg, zoteroapi.New(cfg, baseURL, nil), 0
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureWriteAllowed(cfg config.Config) int {
	if cfg.AllowWrite {
		return 0
	}
	fmt.Fprintln(stderr, "error: writes are disabled in ~/.zot/.env; set ZOT_ALLOW_WRITE=1 to enable create/update operations")
	return 1
}

func ensureDeleteAllowed(cfg config.Config) int {
	if cfg.AllowDelete {
		return 0
	}
	fmt.Fprintln(stderr, "error: delete operations are disabled in ~/.zot/.env; set ZOT_ALLOW_DELETE=1 to enable delete commands")
	return 1
}
