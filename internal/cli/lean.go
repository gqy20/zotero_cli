package cli

import "zotero_cli/internal/domain"

// LeanItem is a compact JSON-serializable projection of domain.Item.
// Verbose fields (abstract, attachments, notes, annotations, journal_rank, etc.)
// are removed to reduce payload size by ~80%.
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
	Collections     []string `json:"collections,omitempty"` // names only
	DateAdded       string   `json:"date_added,omitempty"`
	MatchedOn       []string `json:"matched_on,omitempty"`
	RelevanceScore  int      `json:"relevance_score,omitempty"`
	Abstract        string   `json:"abstract,omitempty"` // only when includeAbstract=true
}

// toLeanItem projects a domain.Item into a LeanItem.
// When includeAbstract is true, the abstract field is included (used by the abstract command).
func toLeanItem(item domain.Item, includeAbstract bool) LeanItem {
	collections := make([]string, 0, len(item.Collections))
	for _, c := range item.Collections {
		collections = append(collections, c.Name)
	}

	lean := LeanItem{
		Key:             item.Key,
		ItemType:        item.ItemType,
		Title:           item.Title,
		Date:            item.Date,
		CreatorsSummary: shortCreators(item.Creators),
		Container:       item.Container,
		Volume:          item.Volume,
		Issue:           item.Issue,
		Pages:           item.Pages,
		DOI:             item.DOI,
		URL:             item.URL,
		Tags:            item.Tags,
		Collections:     collections,
		DateAdded:       item.DateAdded,
		MatchedOn:       item.MatchedOn,
		RelevanceScore:  item.SearchScore,
	}

	if includeAbstract {
		lean.Abstract = item.Abstract
	}

	return lean
}

// toLeanItems batch-projects a slice of domain.Item into []LeanItem.
func toLeanItems(items []domain.Item, includeAbstract bool) []LeanItem {
	out := make([]LeanItem, 0, len(items))
	for i := range items {
		out = append(out, toLeanItem(items[i], includeAbstract))
	}
	return out
}

func appendLeanMeta(meta map[string]any) {
	meta["lean"] = true
	meta["omitted_fields"] = []string{"abstract", "attachments", "notes", "annotations", "journal_rank"}
	meta["full_hint"] = "Use --full --json to include omitted fields."
}
