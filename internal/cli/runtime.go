package cli

import (
	"errors"
	"fmt"
	"os"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func loadConfig() (config.Config, int) {
	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(defaultCLI.stderr, "config not found.")
			fmt.Fprintln(defaultCLI.stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(defaultCLI.stderr, "run `zot config init` to set them up interactively in ~/.zot/.env")
			return config.Config{}, 3
		}
		return config.Config{}, printErr(err)
	}
	return cfg, 0
}

func loadClient() (config.Config, *zoteroapi.Client, int) {
	cfg, exitCode := loadConfig()
	if exitCode != 0 {
		return config.Config{}, nil, exitCode
	}

	remoteCfg, err := remoteClientConfig(cfg)
	if err != nil {
		return config.Config{}, nil, printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	return cfg, zoteroapi.New(remoteCfg, baseURL, nil), 0
}

func loadReader() (config.Config, backend.Reader, int) {
	cfg, exitCode := loadConfig()
	if exitCode != 0 {
		return config.Config{}, nil, exitCode
	}

	reader, err := defaultCLI.backendNewReader(cfg, nil)
	if err != nil {
		return config.Config{}, nil, printErr(err)
	}
	return cfg, reader, 0
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
	fmt.Fprintln(defaultCLI.stderr, "error: writes are disabled in ~/.zot/.env; set ZOT_ALLOW_WRITE=1 to enable create/update operations")
	return 1
}

func ensureDeleteAllowed(cfg config.Config) int {
	if cfg.AllowDelete {
		return 0
	}
	fmt.Fprintln(defaultCLI.stderr, "error: delete operations are disabled in ~/.zot/.env; set ZOT_ALLOW_DELETE=1 to enable delete commands")
	return 1
}

func remoteClientConfig(cfg config.Config) (config.Config, error) {
	normalized := cfg
	switch normalized.Mode {
	case "", "web":
		normalized.Mode = "web"
		return normalized, nil
	case "hybrid":
		normalized.Mode = "web"
		return normalized, nil
	case "local":
		return config.Config{}, fmt.Errorf("web API commands are not available in local mode; use web or hybrid mode")
	default:
		return config.Config{}, fmt.Errorf("unsupported mode %q", normalized.Mode)
	}
}
