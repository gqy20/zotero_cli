package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("config not found")

type Config struct {
	Mode           string `json:"mode"`
	LibraryType    string `json:"library_type"`
	LibraryID      string `json:"library_id"`
	APIKey         string `json:"api_key"`
	Style          string `json:"style"`
	Locale         string `json:"locale"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func Default() Config {
	return Config{
		Mode:           "web",
		LibraryType:    "",
		LibraryID:      "",
		APIKey:         "",
		Style:          "apa",
		Locale:         "en-US",
		TimeoutSeconds: 20,
	}
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "zotcli", "config.json"), nil
}

func Load() (Config, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, path, ErrNotFound
		}
		return Config{}, path, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func Save(cfg Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
