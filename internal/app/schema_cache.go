package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type schemaCacheEntry struct {
	CachedAt time.Time       `json:"cached_at"`
	Data     json.RawMessage `json:"data"`
}

type schemaCacheState struct {
	Source string
	Stale  bool
	Age    time.Duration
}

func (s SchemaService) cached(ctx context.Context, configPath, key string, refresh bool, target any, fetch func() (any, error)) (schemaCacheState, error) {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	cachePath := schemaCachePath(s.CacheDir(configPath), key)
	entry, cacheErr := loadSchemaCache(cachePath)
	if cacheErr == nil && !refresh && now().Sub(entry.CachedAt) <= schemaCacheTTL {
		if err := json.Unmarshal(entry.Data, target); err == nil {
			return schemaCacheState{Source: "cache", Age: now().Sub(entry.CachedAt)}, nil
		}
	}
	value, fetchErr := fetch()
	if fetchErr == nil {
		data, err := json.Marshal(value)
		if err != nil {
			return schemaCacheState{}, err
		}
		if err := json.Unmarshal(data, target); err != nil {
			return schemaCacheState{}, err
		}
		if err := saveSchemaCache(cachePath, schemaCacheEntry{CachedAt: now(), Data: data}); err != nil {
			return schemaCacheState{}, fmt.Errorf("save schema cache: %w", err)
		}
		return schemaCacheState{Source: "web"}, nil
	}
	if cacheErr == nil {
		if err := json.Unmarshal(entry.Data, target); err == nil {
			return schemaCacheState{Source: "cache", Stale: true, Age: now().Sub(entry.CachedAt)}, nil
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return schemaCacheState{}, ctx.Err()
	}
	return schemaCacheState{}, fetchErr
}

func schemaCachePath(dir, key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(hash[:])+".json")
}

func loadSchemaCache(path string) (schemaCacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return schemaCacheEntry{}, err
	}
	var entry schemaCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return schemaCacheEntry{}, err
	}
	if entry.CachedAt.IsZero() || len(entry.Data) == 0 {
		return schemaCacheEntry{}, fmt.Errorf("invalid schema cache entry")
	}
	return entry, nil
}

func saveSchemaCache(path string, entry schemaCacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".schema-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, path)
}

func schemaMeta(state schemaCacheState) map[string]any {
	meta := map[string]any{"read_source": state.Source, "cache_hit": state.Source == "cache"}
	if state.Age > 0 {
		meta["cache_age_seconds"] = int64(state.Age / time.Second)
	}
	if state.Stale {
		meta["stale"] = true
	}
	return meta
}

func schemaWarnings(state schemaCacheState) []Warning {
	if !state.Stale {
		return nil
	}
	return []Warning{{Code: "stale_schema_cache", Message: "schema refresh failed; using stale cached data"}}
}
