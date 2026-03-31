package backend

import (
	"context"
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

type Reader interface {
	FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error)
	GetItem(ctx context.Context, key string) (domain.Item, error)
}

func NewReader(cfg config.Config, httpClient *http.Client) (Reader, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = "web"
	}

	switch mode {
	case "web":
		baseURL := os.Getenv("ZOT_BASE_URL")
		return NewWebReader(zoteroapi.New(cfg, baseURL, httpClient)), nil
	case "local":
		return NewLocalReader(cfg)
	case "hybrid":
		return nil, fmt.Errorf("hybrid mode is not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported mode %q", mode)
	}
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
