package app

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type ItemFindRequest struct {
	Options backend.FindOptions
	Snippet bool
}

const (
	defaultFindLimit         = 100
	defaultDetailedFindLimit = 20
)

type ItemShowRequest struct {
	Key     string
	Full    bool
	Snippet bool
}

type LeanItem struct {
	Key             string   `json:"key"`
	ItemType        string   `json:"item_type"`
	Title           string   `json:"title"`
	Date            string   `json:"date,omitempty"`
	CreatorsSummary string   `json:"creators"`
	Container       string   `json:"container,omitempty"`
	Volume          string   `json:"volume,omitempty"`
	Issue           string   `json:"issue,omitempty"`
	Pages           string   `json:"pages,omitempty"`
	DOI             string   `json:"doi,omitempty"`
	URL             string   `json:"url,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Collections     []string `json:"collections,omitempty"`
	DateAdded       string   `json:"date_added,omitempty"`
	MatchedOn       []string `json:"matched_on,omitempty"`
	RelevanceScore  int      `json:"relevance_score,omitempty"`
}

type itemSnippetReader interface {
	FullTextSnippet(context.Context, domain.Item, string) (string, error)
}

type itemPreviewReader interface {
	FullTextPreview(context.Context, domain.Item) (string, error)
}

func (s ReadService) FindItems(ctx context.Context, req ItemFindRequest) (Result, error) {
	opts := backend.NormalizeFindOptions(req.Options)
	if opts.All && opts.Limit > 0 {
		return Result{}, fmt.Errorf("--all and --limit are mutually exclusive")
	}
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	pageLimit := findResultLimit(req, opts)
	if pageLimit > 0 {
		opts.Limit = pageLimit + 1
	} else {
		opts.Limit = 0
	}
	if req.Snippet && opts.In == "metadata" {
		return Result{}, fmt.Errorf("find --snippet requires --in fulltext")
	}
	injectedAttachments := false
	if req.Snippet && !containsString(opts.IncludeFields, "attachments") {
		opts.IncludeFields = append(opts.IncludeFields, "attachments")
		injectedAttachments = true
	}
	items, err := reader.FindItems(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	items = filterItems(items, opts)
	hasMore := pageLimit > 0 && len(items) > pageLimit
	if hasMore {
		items = items[:pageLimit]
	}
	for i := range items {
		enrichJournalRank(&items[i])
	}
	if req.Snippet {
		snippeter, ok := reader.(itemSnippetReader)
		if !ok {
			return Result{}, fmt.Errorf("find --snippet requires local or hybrid mode with local data")
		}
		for i := range items {
			items[i].FullTextPreview, err = snippeter.FullTextSnippet(ctx, items[i], opts.Query)
			if err != nil {
				return Result{}, err
			}
			if injectedAttachments {
				items[i].Attachments = nil
			}
		}
	}
	meta := readMeta(reader)
	meta["total"] = len(items)
	meta["returned"] = len(items)
	meta["limit"] = pageLimit
	meta["offset"] = opts.Start
	meta["has_more"] = hasMore
	if hasMore {
		meta["next_offset"] = opts.Start + len(items)
	}
	var data any = items
	if !opts.Full && !req.Snippet {
		data = leanItems(items)
		appendLeanMetadata(meta)
	} else if req.Snippet {
		data = findSnippetJSONItems(items)
	}
	text := itemFindText(items, opts.Full || req.Snippet || len(opts.IncludeFields) > 0)
	if hasMore {
		text += fmt.Sprintf("\n\nMore results available; use --offset %d", opts.Start+len(items))
	}
	return Result{Data: data, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func findResultLimit(req ItemFindRequest, opts backend.FindOptions) int {
	if opts.All {
		return 0
	}
	if opts.Limit > 0 {
		return opts.Limit
	}
	if req.Snippet || opts.Full {
		return defaultDetailedFindLimit
	}
	return defaultFindLimit
}

func findSnippetJSONItems(items []domain.Item) []domain.Item {
	result := append([]domain.Item(nil), items...)
	for i := range result {
		if result[i].MatchedChunk != nil && strings.TrimSpace(result[i].MatchedChunk.Context) != "" {
			result[i].FullTextPreview = ""
		}
	}
	return result
}

func (s ReadService) ShowItem(ctx context.Context, req ItemShowRequest) (Result, error) {
	_, reader, err := s.reader()
	if err != nil {
		return Result{}, err
	}
	item, err := reader.GetItem(ctx, req.Key)
	if err != nil {
		return Result{}, err
	}
	enrichJournalRank(&item)
	if req.Snippet {
		previewer, ok := reader.(itemPreviewReader)
		if !ok {
			return Result{}, fmt.Errorf("show --snippet requires local or hybrid mode with local data")
		}
		item.FullTextPreview, err = previewer.FullTextPreview(ctx, item)
		if err != nil {
			return Result{}, err
		}
	}
	meta := readMeta(reader)
	meta["total"] = 1
	var data any = item
	if !req.Full && !req.Snippet {
		data = leanItem(item)
		appendLeanMetadata(meta)
	}
	text := itemShowText(item)
	if source, _ := meta["full_text_source"].(string); source != "" {
		text += "\nFull Text Source: " + source
		if hit, _ := meta["full_text_cache_hit"].(bool); hit {
			text += " (cache hit)"
		}
		if key, _ := meta["full_text_attachment_key"].(string); key != "" {
			text += " [" + key + "]"
		}
	}
	return Result{Data: data, Meta: meta, Text: text, Warnings: readWarnings(meta)}, nil
}

func filterItems(items []domain.Item, opts backend.FindOptions) []domain.Item {
	filtered := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if backend.ShouldIncludeFindItem(item.ItemType, item.Tags, item.Date, opts.ItemType, opts.Tags, opts.TagAny, opts.DateAfter, opts.DateBefore) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func enrichJournalRank(item *domain.Item) {
	if item.Container != "" {
		item.JournalRank = backend.LookupJournalRank(item.Container)
	}
}

func leanItem(item domain.Item) LeanItem {
	collections := make([]string, 0, len(item.Collections))
	for _, collection := range item.Collections {
		collections = append(collections, collection.Name)
	}
	return LeanItem{Key: item.Key, ItemType: item.ItemType, Title: item.Title, Date: item.Date, CreatorsSummary: creatorSummary(item.Creators), Container: item.Container, Volume: item.Volume, Issue: item.Issue, Pages: item.Pages, DOI: item.DOI, URL: item.URL, Tags: item.Tags, Collections: collections, DateAdded: item.DateAdded, MatchedOn: item.MatchedOn, RelevanceScore: item.SearchScore}
}

func leanItems(items []domain.Item) []LeanItem {
	result := make([]LeanItem, 0, len(items))
	for _, item := range items {
		result = append(result, leanItem(item))
	}
	return result
}

func appendLeanMetadata(meta map[string]any) {
	meta["lean"] = true
	meta["omitted_fields"] = []string{"abstract", "attachments", "notes", "annotations", "journal_rank"}
	meta["full_hint"] = "Use --full --json to include omitted fields."
}

func creatorSummary(creators []domain.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	if len(creators) == 1 {
		return creators[0].Name
	}
	return creators[0].Name + " et al."
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func itemFindText(items []domain.Item, detailed bool) string {
	if len(items) == 0 {
		return "no items found"
	}
	var b strings.Builder
	for i, item := range items {
		if detailed {
			b.WriteString(itemShowText(item))
			if i < len(items)-1 {
				b.WriteString("\n\n")
			}
			continue
		}
		fmt.Fprintf(&b, "%-10s  %-16s  %-10s  %-10s  %-18s  %s", item.Key, item.ItemType, shortYear(item.DateAdded), shortYear(item.Date), creatorSummary(item.Creators), item.Title)
		if i < len(items)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func itemShowText(item domain.Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Key: %s\nTitle: %s\nType: %s", item.Key, item.Title, item.ItemType)
	if creators := creatorSummary(item.Creators); creators != "" {
		fmt.Fprintf(&b, "\nCreators: %s", creators)
	}
	for _, field := range []struct{ label, value string }{{"Date", item.Date}, {"Date Added", item.DateAdded}, {"Abstract", item.Abstract}, {"Container", item.Container}, {"Volume", item.Volume}, {"Issue", item.Issue}, {"Pages", item.Pages}, {"DOI", item.DOI}, {"URL", item.URL}} {
		if field.value != "" {
			fmt.Fprintf(&b, "\n%s: %s", field.label, field.value)
		}
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(&b, "\nTags: %s", strings.Join(item.Tags, ", "))
	}
	if item.Version > 0 {
		fmt.Fprintf(&b, "\nVersion: %d", item.Version)
	}
	if len(item.MatchedOn) > 0 {
		fmt.Fprintf(&b, "\nMatched On: %s", strings.Join(item.MatchedOn, ", "))
	}
	if len(item.Collections) > 0 {
		names := make([]string, 0, len(item.Collections))
		for _, collection := range item.Collections {
			names = append(names, collection.Name)
		}
		fmt.Fprintf(&b, "\nCollections: %s", strings.Join(names, ", "))
	}
	if len(item.Attachments) > 0 {
		fmt.Fprintf(&b, "\nAttachments: %d", len(item.Attachments))
		for _, attachment := range item.Attachments {
			fmt.Fprintf(&b, "\n  - [%s] %s", itemAttachmentKind(attachment), itemAttachmentSummary(attachment))
			if attachment.Resolved && attachment.ResolvedPath != "" {
				fmt.Fprintf(&b, "\n    path: %s", attachment.ResolvedPath)
			} else if attachment.ZoteroPath != "" {
				fmt.Fprintf(&b, "\n    path: unresolved (%s)", attachment.ZoteroPath)
			}
		}
	}
	if len(item.Notes) > 0 {
		fmt.Fprintf(&b, "\nNotes: %d", len(item.Notes))
		for _, note := range item.Notes {
			fmt.Fprintf(&b, "\n  - %s: %s", note.Key, note.Preview)
		}
	}
	if len(item.Annotations) > 0 {
		fmt.Fprintf(&b, "\nAnnotations: %d", len(item.Annotations))
		for _, annotation := range item.Annotations {
			fmt.Fprintf(&b, "\n  - [%s] color=%s page=%s: %s", annotation.Type, annotation.Color, annotation.PageLabel, annotation.Text)
			if annotation.Comment != "" {
				fmt.Fprintf(&b, " | %s", annotation.Comment)
			}
		}
	}
	if item.FullTextPreview != "" {
		fmt.Fprintf(&b, "\nFull Text Preview: %s", item.FullTextPreview)
	}
	return b.String()
}

func itemAttachmentSummary(attachment domain.Attachment) string {
	label := attachment.Filename
	if label == "" {
		label = attachment.Title
	}
	if label == "" {
		label = attachment.Key
	}
	if attachment.Key != "" {
		label += " (" + attachment.Key + ")"
	}
	return label
}

func itemAttachmentKind(attachment domain.Attachment) string {
	if attachment.ContentType == "application/pdf" {
		return "pdf"
	}
	if attachment.LinkMode == "linked_url" {
		return "link"
	}
	if attachment.ContentType != "" || attachment.LinkMode == "linked_file" || attachment.LinkMode == "imported_file" {
		return "file"
	}
	return "attachment"
}

func shortYear(value string) string {
	if len(value) >= 4 {
		return value[:4]
	}
	return value
}
