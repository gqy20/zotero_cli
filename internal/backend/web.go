package backend

import (
	"context"

	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

type WebReader struct {
	client *zoteroapi.Client
}

func NewWebReader(client *zoteroapi.Client) *WebReader {
	return &WebReader{client: client}
}

func (r *WebReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	items, err := r.client.FindItems(ctx, toAPIFindOptions(opts))
	if err != nil {
		return nil, err
	}
	return mapItems(items), nil
}

func (r *WebReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	item, err := r.client.GetItem(ctx, key)
	if err != nil {
		return domain.Item{}, err
	}
	return mapItem(item), nil
}

func (r *WebReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return nil, newUnsupportedFeatureError("web", "relate")
}

func (r *WebReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	stats, err := r.client.GetLibraryStats(ctx)
	if err != nil {
		return LibraryStats{}, err
	}
	return LibraryStats{
		LibraryType:      stats.LibraryType,
		LibraryID:        stats.LibraryID,
		TotalItems:       stats.TotalItems,
		TotalCollections: stats.TotalCollections,
		TotalSearches:    stats.TotalSearches,
	}, nil
}

func mapItems(items []zoteroapi.Item) []domain.Item {
	out := make([]domain.Item, 0, len(items))
	for _, item := range items {
		out = append(out, mapItem(item))
	}
	return out
}

func mapItem(item zoteroapi.Item) domain.Item {
	return domain.Item{
		Version:     item.Version,
		Key:         item.Key,
		ItemType:    item.ItemType,
		Title:       item.Title,
		Date:        item.Date,
		Creators:    mapCreators(item.Creators),
		Container:   item.Container,
		Volume:      item.Volume,
		Issue:       item.Issue,
		Pages:       item.Pages,
		DOI:         item.DOI,
		URL:         item.URL,
		Tags:        append([]string(nil), item.Tags...),
		Attachments: mapAttachments(item.Attachments),
	}
}

func mapCreators(creators []zoteroapi.Creator) []domain.Creator {
	out := make([]domain.Creator, 0, len(creators))
	for _, creator := range creators {
		out = append(out, domain.Creator{
			Name:        creator.Name,
			CreatorType: creator.CreatorType,
		})
	}
	return out
}

func mapAttachments(attachments []zoteroapi.Attachment) []domain.Attachment {
	out := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		out = append(out, domain.Attachment{
			Key:          attachment.Key,
			ItemType:     attachment.ItemType,
			Title:        attachment.Title,
			ContentType:  attachment.ContentType,
			LinkMode:     attachment.LinkMode,
			Filename:     attachment.Filename,
			ZoteroPath:   "",
			ResolvedPath: "",
			Resolved:     false,
		})
	}
	return out
}
