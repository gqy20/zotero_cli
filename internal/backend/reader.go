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
	FullText       bool
	FullTextAny    bool
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
	HasPDF         bool
	AttachmentName string
	AttachmentPath string
	AttachmentType string
}

type LibraryStats struct {
	LibraryType        string `json:"library_type"`
	LibraryID          string `json:"library_id"`
	TotalItems         int    `json:"total_items"`
	TotalCollections   int    `json:"total_collections"`
	TotalSearches      int    `json:"total_searches"`
	LastLibraryVersion int    `json:"last_library_version,omitempty"`
}

type ReadMetadata struct {
	ReadSource            string `json:"read_source,omitempty"`
	SQLiteFallback        bool   `json:"sqlite_fallback,omitempty"`
	FullTextEngine        string `json:"full_text_engine,omitempty"`
	FullTextSource        string `json:"full_text_source,omitempty"`
	FullTextAttachmentKey string `json:"full_text_attachment_key,omitempty"`
	FullTextCacheHit      bool   `json:"full_text_cache_hit,omitempty"`
}

type Reader interface {
	FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error)
	GetItem(ctx context.Context, key string) (domain.Item, error)
	GetRelated(ctx context.Context, key string) ([]domain.Relation, error)
	GetLibraryStats(ctx context.Context) (LibraryStats, error)
}

type readMetadataReporter interface {
	ConsumeReadMetadata() ReadMetadata
}

type HybridReader struct {
	local            Reader
	web              Reader
	lastReadMetadata ReadMetadata
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
	return readWithFallback(r,
		func(reader Reader) ([]domain.Item, error) {
			return reader.FindItems(ctx, opts)
		},
	)
}

func (r *HybridReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return readWithFallback(r,
		func(reader Reader) (domain.Item, error) {
			return reader.GetItem(ctx, key)
		},
	)
}

func (r *HybridReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return readWithFallback(r,
		func(reader Reader) ([]domain.Relation, error) {
			return reader.GetRelated(ctx, key)
		},
	)
}

func (r *HybridReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	return readWithFallback(r,
		func(reader Reader) (LibraryStats, error) {
			return reader.GetLibraryStats(ctx)
		},
	)
}

func readWithFallback[T any](r *HybridReader, read func(Reader) (T, error)) (T, error) {
	var zero T
	if r.local != nil {
		value, err := read(r.local)
		if err == nil {
			r.lastReadMetadata = consumeReadMetadata(r.local)
			return value, nil
		}
		if !shouldFallbackToWeb(err) {
			return zero, err
		}
	}
	value, err := read(r.web)
	if err == nil {
		r.lastReadMetadata = consumeReadMetadata(r.web)
	}
	return value, err
}

func (r *HybridReader) ConsumeReadMetadata() ReadMetadata {
	meta := r.lastReadMetadata
	r.lastReadMetadata = ReadMetadata{}
	return meta
}

func consumeReadMetadata(reader Reader) ReadMetadata {
	reporter, ok := reader.(readMetadataReporter)
	if !ok {
		return ReadMetadata{}
	}
	return reporter.ConsumeReadMetadata()
}

func mergeReadMetadata(base ReadMetadata, extra ReadMetadata) ReadMetadata {
	if extra.ReadSource != "" {
		base.ReadSource = extra.ReadSource
	}
	if extra.SQLiteFallback {
		base.SQLiteFallback = true
	}
	if extra.FullTextSource != "" {
		base.FullTextSource = extra.FullTextSource
	}
	if extra.FullTextEngine != "" {
		base.FullTextEngine = extra.FullTextEngine
	}
	if extra.FullTextAttachmentKey != "" {
		base.FullTextAttachmentKey = extra.FullTextAttachmentKey
	}
	if extra.FullTextCacheHit {
		base.FullTextCacheHit = true
	}
	return base
}

func shouldFallbackToWeb(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrItemNotFound) || errors.Is(err, ErrUnsupportedFeature) || errors.Is(err, ErrLocalTemporarilyUnavailable)
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
