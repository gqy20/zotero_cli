package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type FindOptions struct {
	Query          string
	All            bool
	Full           bool
	ItemType       string
	Limit          int
	Start          int
	Tag            string
	Tags           []string
	TagAny         bool
	IncludeFields  []string
	Sort           string
	Direction      string
	QMode          string
	IncludeTrashed bool
	DateAfter      string
	DateBefore     string
}

type LibraryStats struct {
	LibraryType      string `json:"library_type"`
	LibraryID        string `json:"library_id"`
	TotalItems       int    `json:"total_items"`
	TotalCollections int    `json:"total_collections"`
	TotalSearches    int    `json:"total_searches"`
}

type Reader interface {
	FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error)
	GetItem(ctx context.Context, key string) (domain.Item, error)
	GetRelated(ctx context.Context, key string) ([]domain.Relation, error)
	GetLibraryStats(ctx context.Context) (LibraryStats, error)
}

type HybridReader struct {
	local Reader
	web   Reader
}

func NewReader(cfg config.Config, httpClient *http.Client) (Reader, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = "web"
	}

	remoteCfg := cfg
	remoteCfg.Mode = "web"

	switch mode {
	case "web":
		baseURL := os.Getenv("ZOT_BASE_URL")
		return NewWebReader(zoteroapi.New(remoteCfg, baseURL, httpClient)), nil
	case "local":
		return NewLocalReader(cfg)
	case "hybrid":
		baseURL := os.Getenv("ZOT_BASE_URL")
		webReader := NewWebReader(zoteroapi.New(remoteCfg, baseURL, httpClient))
		localReader, err := NewLocalReader(cfg)
		if err != nil {
			return &HybridReader{web: webReader}, nil
		}
		return &HybridReader{local: localReader, web: webReader}, nil
	default:
		return nil, fmt.Errorf("unsupported mode %q", mode)
	}
}

func (r *HybridReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	if r.local != nil {
		items, err := r.local.FindItems(ctx, opts)
		if err == nil {
			return items, nil
		}
		if !shouldFallbackToWeb(err) {
			return nil, err
		}
	}
	return r.web.FindItems(ctx, opts)
}

func (r *HybridReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	if r.local != nil {
		item, err := r.local.GetItem(ctx, key)
		if err == nil {
			return item, nil
		}
		if !shouldFallbackToWeb(err) {
			return domain.Item{}, err
		}
	}
	return r.web.GetItem(ctx, key)
}

func (r *HybridReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	if r.local != nil {
		relations, err := r.local.GetRelated(ctx, key)
		if err == nil {
			return relations, nil
		}
		if !shouldFallbackToWeb(err) {
			return nil, err
		}
	}
	return r.web.GetRelated(ctx, key)
}

func (r *HybridReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	if r.local != nil {
		stats, err := r.local.GetLibraryStats(ctx)
		if err == nil {
			return stats, nil
		}
		if !shouldFallbackToWeb(err) {
			return LibraryStats{}, err
		}
	}
	return r.web.GetLibraryStats(ctx)
}

func shouldFallbackToWeb(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrItemNotFound) || errors.Is(err, ErrUnsupportedFeature)
}

func toAPIFindOptions(opts FindOptions) zoteroapi.FindOptions {
	return zoteroapi.FindOptions{
		Query:          opts.Query,
		All:            opts.All,
		Full:           opts.Full,
		ItemType:       opts.ItemType,
		Limit:          opts.Limit,
		Start:          opts.Start,
		Tag:            opts.Tag,
		Tags:           append([]string(nil), opts.Tags...),
		TagAny:         opts.TagAny,
		IncludeFields:  append([]string(nil), opts.IncludeFields...),
		Sort:           opts.Sort,
		Direction:      opts.Direction,
		QMode:          opts.QMode,
		IncludeTrashed: opts.IncludeTrashed,
		DateAfter:      opts.DateAfter,
		DateBefore:     opts.DateBefore,
	}
}
