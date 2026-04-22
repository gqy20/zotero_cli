package domain

type Item struct {
	Version              int               `json:"version,omitempty"`
	Key                  string            `json:"key"`
	ItemType             string            `json:"item_type"`
	Title                string            `json:"title"`
	Date                 string            `json:"date"`
	SearchScore          int               `json:"-"`
	SnippetAttachmentKey string            `json:"-"`
	Creators             []Creator         `json:"creators"`
	MatchedOn            []string          `json:"matched_on,omitempty"`
	FullTextPreview      string            `json:"full_text_preview,omitempty"`
	Container            string            `json:"container,omitempty"`
	Volume               string            `json:"volume,omitempty"`
	Issue                string            `json:"issue,omitempty"`
	Pages                string            `json:"pages,omitempty"`
	DOI                  string            `json:"doi,omitempty"`
	URL                  string            `json:"url,omitempty"`
	Tags                 []string          `json:"tags,omitempty"`
	Collections          []Collection      `json:"collections,omitempty"`
	Attachments          []Attachment      `json:"attachments,omitempty"`
	Notes                []Note            `json:"notes,omitempty"`
	Annotations          []Annotation      `json:"annotations,omitempty"`
	MatchedChunk         *MatchedChunkInfo `json:"matched_chunk,omitempty"`
	JournalRank          *JournalRank      `json:"journal_rank,omitempty"`
}

type MatchedChunkInfo struct {
	Text          string     `json:"text"`
	Page          int        `json:"page"`
	BBox          [4]float64 `json:"bbox"`
	AttachmentKey string     `json:"attachment_key"`
}

type Creator struct {
	Name        string `json:"name"`
	CreatorType string `json:"creator_type"`
}

type Collection struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Attachment struct {
	Key          string `json:"key"`
	ItemType     string `json:"item_type"`
	Title        string `json:"title,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	LinkMode     string `json:"link_mode,omitempty"`
	Filename     string `json:"filename,omitempty"`
	ZoteroPath   string `json:"zotero_path,omitempty"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	Resolved     bool   `json:"resolved,omitempty"`
}

type Note struct {
	Key           string `json:"key"`
	ParentItemKey string `json:"parent_item_key,omitempty"`
	Content       string `json:"content,omitempty"`
	Preview       string `json:"preview,omitempty"`
}

// Annotation represents a Zotero reader highlight or note on a PDF attachment.
type Annotation struct {
	Key        string `json:"key"`
	Type       string `json:"type"`                 // "highlight" | "note" | "image" | "ink"
	Text       string `json:"text,omitempty"`       // highlighted text (original passage)
	Comment    string `json:"comment,omitempty"`    // user-written note
	Color      string `json:"color,omitempty"`      // "#ffd400"
	PageLabel  string `json:"page_label,omitempty"` // "2"
	PageIndex  int    `json:"page_index,omitempty"` // parsed from position JSON
	Position   string `json:"position,omitempty"`   // raw position JSON
	SortIndex  string `json:"sort_index,omitempty"`
	IsExternal bool   `json:"is_external"`
	DateAdded  string `json:"date_added,omitempty"`
}

type ItemRef struct {
	Key      string   `json:"key"`
	ItemType string   `json:"item_type,omitempty"`
	Title    string   `json:"title,omitempty"`
	Date     string   `json:"date,omitempty"`
	Creators []string `json:"creators,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type Relation struct {
	Predicate string  `json:"predicate"`
	Direction string  `json:"direction"`
	Target    ItemRef `json:"target"`
}

type AggregatedRelations struct {
	Self      []Relation       `json:"self"`
	Notes     []NoteRelations  `json:"notes"`
	Citations []CitationSource `json:"citations"`
}

type NoteRelations struct {
	Source    ItemRef    `json:"source"`
	Preview   string     `json:"preview,omitempty"`
	Relations []Relation `json:"relations"`
}

type CitationSource struct {
	SourceKey string    `json:"source_key"`
	Targets   []ItemRef `json:"targets"`
}

type CitationOptions struct {
	Format string // "citation" | "bib"
	Style  string
	Locale string
}

type CitationResult struct {
	Key    string `json:"key"`
	Format string `json:"format"`
	Style  string `json:"style,omitempty"`
	Text   string `json:"text"`
}

type JournalRank struct {
	MatchedName string            `json:"matched_name,omitempty"`
	Ranks       map[string]string `json:"ranks,omitempty"`
}
