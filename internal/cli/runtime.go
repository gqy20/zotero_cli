package cli

import (
	"errors"
	"fmt"
	"os"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func (c *CLI) loadConfig() (config.Config, int) {
	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(c.stderr, "config not found.")
			fmt.Fprintln(c.stderr, "required fields: library_type, library_id, api_key")
			fmt.Fprintln(c.stderr, "run `zot config init` to set them up interactively in ~/.zot/.env")
			return config.Config{}, ExitConfig
		}
		return config.Config{}, c.printErr(err)
	}
	return cfg, ExitOK
}

func (c *CLI) loadClient() (config.Config, *zoteroapi.Client, int) {
	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return config.Config{}, nil, exitCode
	}

	remoteCfg, err := c.remoteClientConfig(cfg)
	if err != nil {
		return config.Config{}, nil, c.printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	return cfg, zoteroapi.New(remoteCfg, baseURL, nil), ExitOK
}

func (c *CLI) loadReader() (config.Config, backend.Reader, int) {
	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return config.Config{}, nil, exitCode
	}

	reader, err := c.backendNewReader(cfg, nil)
	if err != nil {
		return config.Config{}, nil, c.printErr(err)
	}
	return cfg, reader, ExitOK
}

func boolToInt(value bool) int {
	if value {
		return ExitError
	}
	return ExitOK
}

func (c *CLI) ensureWriteAllowed(cfg config.Config) int {
	if cfg.AllowWrite {
		return ExitOK
	}
	fmt.Fprintln(c.stderr, "error: writes are disabled in ~/.zot/.env; set ZOT_ALLOW_WRITE=1 to enable create/update operations")
	return ExitError
}

func (c *CLI) ensureDeleteAllowed(cfg config.Config) int {
	if cfg.AllowDelete {
		return ExitOK
	}
	fmt.Fprintln(c.stderr, "error: delete operations are disabled in ~/.zot/.env; set ZOT_ALLOW_DELETE=1 to enable delete commands")
	return ExitError
}

func (c *CLI) remoteClientConfig(cfg config.Config) (config.Config, error) {
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
