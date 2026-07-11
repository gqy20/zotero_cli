package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type SupplementsRequest struct {
	Key    string
	All    bool
	Online bool
	Limit  int
}

func (s ReadService) Supplements(ctx context.Context, req SupplementsRequest) (Result, error) {
	cfg, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	if req.Online && req.All {
		return Result{}, fmt.Errorf("item supp --online does not support --all; query one item key at a time")
	}
	includeLocal := cfg.Mode != "web" && cfg.Mode != "remote"
	if !includeLocal && !req.Online {
		return Result{}, fmt.Errorf("item supp requires local or hybrid mode; add --online to query public provider metadata")
	}
	var items []domain.Item
	if req.All {
		items, err = reader.FindItems(ctx, backend.FindOptions{All: true, Full: true})
	} else {
		var item domain.Item
		item, err = reader.GetItem(ctx, req.Key)
		items = []domain.Item{item}
	}
	if err != nil {
		return Result{}, err
	}
	supplements := []backend.Supplement{}
	if includeLocal {
		supplements = append(supplements, backend.LocalSupplements(items)...)
	}
	var discovery backend.OnlineSupplementDiscovery
	if req.Online {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
		for _, item := range items {
			found := backend.DiscoverOnlineSupplements(ctx, client, item)
			discovery.Providers = append(discovery.Providers, found.Providers...)
			supplements = append(supplements, found.Supplements...)
		}
	}
	totalBeforeLimit := len(supplements)
	if req.Limit > 0 && len(supplements) > req.Limit {
		supplements = supplements[:req.Limit]
	}
	meta := readMeta(reader)
	meta["total"] = len(supplements)
	meta["total_before_limit"] = totalBeforeLimit
	meta["scanned_items"] = len(items)
	meta["online_lookup_enabled"] = req.Online
	if req.Online {
		meta["online_providers"] = discovery.Providers
	}
	return Result{Data: supplements, Meta: meta, Text: supplementsText(supplements), Warnings: readWarnings(meta)}, nil
}

func supplementsText(values []backend.Supplement) string {
	if len(values) == 0 {
		return "No supplement candidates found."
	}
	var b strings.Builder
	for i, supplement := range values {
		fmt.Fprintf(&b, "%s  %-22s  %-20s  %.2f  %s", supplement.ItemKey, supplement.Kind, supplement.ResolutionStatus, supplement.Confidence, supplement.Label)
		if supplement.LocalPath != "" {
			fmt.Fprintf(&b, "\n  path: %s", supplement.LocalPath)
		} else if supplement.ZoteroPath != "" {
			fmt.Fprintf(&b, "\n  path: unresolved (%s)", supplement.ZoteroPath)
		}
		if supplement.DownloadURL != "" {
			fmt.Fprintf(&b, "\n  download: %s", supplement.DownloadURL)
		}
		if len(supplement.Evidence) > 0 {
			fmt.Fprintf(&b, "\n  evidence: %s", strings.Join(supplement.Evidence, ", "))
		}
		if i < len(values)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
