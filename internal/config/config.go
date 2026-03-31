package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("config not found")

type Config struct {
	Mode                       string `json:"mode"`
	DataDir                    string `json:"data_dir,omitempty"`
	LibraryType                string `json:"library_type"`
	LibraryID                  string `json:"library_id"`
	APIKey                     string `json:"api_key"`
	Style                      string `json:"style"`
	Locale                     string `json:"locale"`
	TimeoutSeconds             int    `json:"timeout_seconds"`
	RetryMaxAttempts           int    `json:"retry_max_attempts"`
	RetryBaseDelayMilliseconds int    `json:"retry_base_delay_ms"`
	AllowWrite                 bool   `json:"allow_write"`
	AllowDelete                bool   `json:"allow_delete"`
}

func Default() Config {
	return Config{
		Mode:                       "web",
		LibraryType:                "",
		LibraryID:                  "",
		APIKey:                     "",
		Style:                      "apa",
		Locale:                     "en-US",
		TimeoutSeconds:             20,
		RetryMaxAttempts:           3,
		RetryBaseDelayMilliseconds: 250,
		AllowWrite:                 true,
		AllowDelete:                false,
	}
}

func DefaultPath() (string, error) {
	return DefaultEnvPath()
}

func DefaultEnvPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zot", ".env"), nil
}

func Load() (Config, string, error) {
	path, err := DefaultEnvPath()
	if err != nil {
		return Config{}, "", err
	}

	cfg, found, err := loadEnvConfig()
	if err != nil {
		return Config{}, path, err
	}
	if !found {
		return Config{}, path, ErrNotFound
	}
	return cfg, path, nil
}

func Save(cfg Config) error {
	path, err := DefaultEnvPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	lines := []string{
		fmt.Sprintf("ZOT_MODE=%s", cfg.Mode),
		fmt.Sprintf("ZOT_DATA_DIR=%s", cfg.DataDir),
		fmt.Sprintf("ZOT_LIBRARY_TYPE=%s", cfg.LibraryType),
		fmt.Sprintf("ZOT_LIBRARY_ID=%s", cfg.LibraryID),
		fmt.Sprintf("ZOT_API_KEY=%s", cfg.APIKey),
		fmt.Sprintf("ZOT_STYLE=%s", cfg.Style),
		fmt.Sprintf("ZOT_LOCALE=%s", cfg.Locale),
		fmt.Sprintf("ZOT_TIMEOUT_SECONDS=%d", cfg.TimeoutSeconds),
		fmt.Sprintf("ZOT_RETRY_MAX_ATTEMPTS=%d", cfg.RetryMaxAttempts),
		fmt.Sprintf("ZOT_RETRY_BASE_DELAY_MS=%d", cfg.RetryBaseDelayMilliseconds),
		fmt.Sprintf("ZOT_ALLOW_WRITE=%s", formatBool(cfg.AllowWrite)),
		fmt.Sprintf("ZOT_ALLOW_DELETE=%s", formatBool(cfg.AllowDelete)),
		"",
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

func loadEnvConfig() (Config, bool, error) {
	envFile, err := readDotEnv()
	if err != nil {
		return Config{}, false, err
	}

	cfg := Default()
	found := false

	if value := firstNonEmpty(os.Getenv("ZOT_MODE"), envFile["ZOT_MODE"]); value != "" {
		cfg.Mode = value
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_DATA_DIR"), envFile["ZOT_DATA_DIR"]); value != "" {
		cfg.DataDir = value
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
	if value := firstNonEmpty(os.Getenv("ZOT_RETRY_MAX_ATTEMPTS"), envFile["ZOT_RETRY_MAX_ATTEMPTS"]); value != "" {
		attempts, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, false, err
		}
		cfg.RetryMaxAttempts = attempts
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_RETRY_BASE_DELAY_MS"), envFile["ZOT_RETRY_BASE_DELAY_MS"]); value != "" {
		delay, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, false, err
		}
		cfg.RetryBaseDelayMilliseconds = delay
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_ALLOW_WRITE"), envFile["ZOT_ALLOW_WRITE"]); value != "" {
		parsed, err := parseBool(value)
		if err != nil {
			return Config{}, false, err
		}
		cfg.AllowWrite = parsed
		found = true
	}
	if value := firstNonEmpty(os.Getenv("ZOT_ALLOW_DELETE"), envFile["ZOT_ALLOW_DELETE"]); value != "" {
		parsed, err := parseBool(value)
		if err != nil {
			return Config{}, false, err
		}
		cfg.AllowDelete = parsed
		found = true
	}
	return cfg, found, nil
}

func readDotEnv() (map[string]string, error) {
	path, err := DefaultEnvPath()
	if err != nil {
		return nil, err
	}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func formatBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
