package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			envCfg, envFound, envErr := loadEnvConfig()
			if envErr != nil {
				return Config{}, path, envErr
			}
			if !envFound {
				return Config{}, path, ErrNotFound
			}
			mergeConfig(&cfg, envCfg)
			return cfg, path, nil
		}
		return Config{}, path, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, err
	}

	envCfg, _, err := loadEnvConfig()
	if err != nil {
		return Config{}, path, err
	}
	mergeConfig(&cfg, envCfg)
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

func loadEnvConfig() (Config, bool, error) {
	envFile, err := readDotEnv()
	if err != nil {
		return Config{}, false, err
	}

	cfg := Config{}
	found := false

	if value := firstNonEmpty(os.Getenv("ZOT_MODE"), envFile["ZOT_MODE"]); value != "" {
		cfg.Mode = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_LIBRARY_TYPE"), envFile["ZOT_LIBRARY_TYPE"]); value != "" {
		cfg.LibraryType = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_LIBRARY_ID"), envFile["ZOT_LIBRARY_ID"]); value != "" {
		cfg.LibraryID = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_API_KEY"), envFile["ZOT_API_KEY"]); value != "" {
		cfg.APIKey = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_STYLE"), envFile["ZOT_STYLE"]); value != "" {
		cfg.Style = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_LOCALE"), envFile["ZOT_LOCALE"]); value != "" {
		cfg.Locale = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_TIMEOUT_SECONDS"), envFile["ZOT_TIMEOUT_SECONDS"]); value != "" {
		timeout, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, false, err
		}
		cfg.TimeoutSeconds = timeout
		found = true
	}

	return cfg, found, nil
}

func readDotEnv() (map[string]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(wd, ".env")
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func mergeConfig(dst *Config, src Config) {
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.LibraryType != "" {
		dst.LibraryType = src.LibraryType
	}
	if src.LibraryID != "" {
		dst.LibraryID = src.LibraryID
	}
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.Style != "" {
		dst.Style = src.Style
	}
	if src.Locale != "" {
		dst.Locale = src.Locale
	}
	if src.TimeoutSeconds != 0 {
		dst.TimeoutSeconds = src.TimeoutSeconds
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
