package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"zotero_cli/internal/config"
)

func (c *CLI) promptInitSetup(cfg config.Config, provided map[string]bool, reader *bufio.Reader) (config.Config, error) {

	fmt.Fprintln(c.stdout, "Initialize ~/.zot/.env")
	fmt.Fprintln(c.stdout, "help:")
	fmt.Fprintln(c.stdout, "  API keys: https://www.zotero.org/settings/keys")
	fmt.Fprintln(c.stdout, "  User library ID: check your userID on https://www.zotero.org/settings/keys")
	fmt.Fprintln(c.stdout, "  Group library IDs: https://www.zotero.org/groups")
	fmt.Fprintln(c.stdout, "")
	fmt.Fprintln(c.stdout, "  Tip: if you are using an AI assistant with browser access,")
	fmt.Fprintln(c.stdout, "  it can navigate to the pages above and fill these values for you.")

	if !provided["mode"] {
		mode, err := c.promptWithDefault(reader, "Mode [web]: ")
		if err != nil {
			return config.Config{}, err
		}
		if mode != "" {
			cfg.Mode = mode
		}
	}

	if !provided["library_type"] {
		libraryType, err := c.promptRequired(reader, "Library type (user/group): ", func(value string) error {
			if value != "user" && value != "group" {
				return fmt.Errorf("must be user or group")
			}
			return nil
		})
		if err != nil {
			return config.Config{}, err
		}
		cfg.LibraryType = libraryType
	}

	if !provided["library_id"] {
		libraryID, err := c.promptRequired(reader, "Library ID: ", func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("cannot be empty")
			}
			return nil
		})
		if err != nil {
			return config.Config{}, err
		}
		cfg.LibraryID = libraryID
	}

	if !provided["api_key"] {
		apiKey, err := c.promptRequired(reader, "API key: ", func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("cannot be empty")
			}
			return nil
		})
		if err != nil {
			return config.Config{}, err
		}
		cfg.APIKey = apiKey
	}

	if cfg.Mode == "local" || cfg.Mode == "hybrid" {
		if !provided["data_dir"] {
			autoDir := discoverDataDir()
			var dataDir string
			var err error
			if autoDir != "" {
				dataDir, err = c.promptWithDefault(reader, fmt.Sprintf("Zotero data directory [%s]: ", autoDir))
				if err != nil {
					return config.Config{}, err
				}
				if dataDir == "" {
					dataDir = autoDir
				}
			} else {
				dataDir, err = c.promptRequired(reader, "Zotero data directory: ", func(value string) error {
					if strings.TrimSpace(value) == "" {
						return fmt.Errorf("cannot be empty in local/hybrid mode")
					}
					return nil
				})
				if err != nil {
					return config.Config{}, err
				}
			}
			cfg.DataDir = dataDir
		}
	}

	return cfg, nil
}

func (c *CLI) promptRequired(reader *bufio.Reader, label string, validate func(string) error) (string, error) {
	for {
		fmt.Fprint(c.stdout, label)
		value, err := readPromptLine(reader)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if validate != nil {
			if err := validate(value); err != nil {
				fmt.Fprintln(c.stderr, "error:", err)
				continue
			}
		}
		return value, nil
	}
}

func (c *CLI) promptWithDefault(reader *bufio.Reader, label string) (string, error) {
	fmt.Fprint(c.stdout, label)
	value, err := readPromptLine(reader)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (c *CLI) promptBool(reader *bufio.Reader, label string, defaultValue bool) (bool, error) {
	for {
		fmt.Fprint(c.stdout, label)
		value, err := readPromptLine(reader)
		if err != nil {
			return false, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return defaultValue, nil
		}
		parsed, err := parsePromptBool(value)
		if err != nil {
			fmt.Fprintln(c.stderr, "error:", err)
			continue
		}
		return parsed, err
	}
}

func readPromptLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) && line != "" {
		return line, nil
	}
	return line, err
}

func parsePromptBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes", "1", "true":
		return true, nil
	case "n", "no", "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("please answer yes or no")
	}
}
