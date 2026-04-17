package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"zotero_cli/internal/config"
)

func (c *CLI) promptConfigSetup() (config.Config, error) {
	reader := bufio.NewReader(c.stdin)
	cfg := config.Default()

	fmt.Fprintln(c.stdout, "first-time setup for ~/.zot/.env")
	fmt.Fprintln(c.stdout, "required: library_type, library_id, api_key")
	fmt.Fprintln(c.stdout, "help:")
	fmt.Fprintln(c.stdout, "  API keys: https://www.zotero.org/settings/keys")
	fmt.Fprintln(c.stdout, "  User library ID: check your userID on https://www.zotero.org/settings/keys")
	fmt.Fprintln(c.stdout, "  Group library IDs: https://www.zotero.org/groups")
	fmt.Fprintln(c.stdout, "  Web API basics: https://www.zotero.org/support/dev/web_api/v3/basics")

	libraryType, err := c.promptRequired(reader, "Library type (user/group): ", func(value string) error {
		if value != "user" && value != "group" {
			return fmt.Errorf("library_type must be user or group")
		}
		return nil
	})
	if err != nil {
		return config.Config{}, err
	}
	cfg.LibraryType = libraryType

	libraryID, err := c.promptRequired(reader, "Library ID: ", func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("library_id cannot be empty")
		}
		return nil
	})
	if err != nil {
		return config.Config{}, err
	}
	cfg.LibraryID = libraryID

	apiKey, err := c.promptRequired(reader, "API key: ", func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("api_key cannot be empty")
		}
		return nil
	})
	if err != nil {
		return config.Config{}, err
	}
	cfg.APIKey = apiKey

	style, err := c.promptWithDefault(reader, fmt.Sprintf("Citation style [%s]: ", cfg.Style))
	if err != nil {
		return config.Config{}, err
	}
	if style != "" {
		cfg.Style = style
	}

	locale, err := c.promptWithDefault(reader, fmt.Sprintf("Locale [%s]: ", cfg.Locale))
	if err != nil {
		return config.Config{}, err
	}
	if locale != "" {
		cfg.Locale = locale
	}

	allowWrite, err := c.promptBool(reader, "Allow create/update operations? [Y/n]: ", cfg.AllowWrite)
	if err != nil {
		return config.Config{}, err
	}
	cfg.AllowWrite = allowWrite

	allowDelete, err := c.promptBool(reader, "Allow delete operations? [y/N]: ", cfg.AllowDelete)
	if err != nil {
		return config.Config{}, err
	}
	cfg.AllowDelete = allowDelete

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
		return parsed, nil
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
