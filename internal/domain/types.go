package domain

type Item struct {
	Version         int          `json:"version,omitempty"`
	Key             string       `json:"key"`
	ItemType        string       `json:"item_type"`
	Title           string       `json:"title"`
	Date            string       `json:"date"`
	SearchScore     int          `json:"-"`
	SnippetAttachmentKey string  `json:"-"`
	Creators        []Creator    `json:"creators"`
	MatchedOn       []string     `json:"matched_on,omitempty"`
	FullTextPreview string       `json:"full_text_preview,omitempty"`
	Container       string       `json:"container,omitempty"`
	Volume          string       `json:"volume,omitempty"`
	Issue           string       `json:"issue,omitempty"`
	Pages           string       `json:"pages,omitempty"`
	DOI             string       `json:"doi,omitempty"`
	URL             string       `json:"url,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	Collections     []Collection `json:"collections,omitempty"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	Notes           []Note       `json:"notes,omitempty"`
	Annotations     []Annotation `json:"annotations,omitempty"`
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
	Key     string `json:"key"`
	Preview string `json:"preview,omitempty"`
}

// Annotation represents a Zotero reader highlight or note on a PDF attachment.
type Annotation struct {
	Key        string `json:"key"`
	Type       string  `json:"type"`                 // "highlight" | "note" | "image" | "ink"
	Text       string  `json:"text,omitempty"`        // highlighted text (original passage)
	Comment    string  `json:"comment,omitempty"`     // user-written note
	Color      string  `json:"color,omitempty"`       // "#ffd400"
	PageLabel  string  `json:"page_label,omitempty"`  // "2"
	PageIndex  int     `json:"page_index,omitempty"`   // parsed from position JSON
	Position   string  `json:"position,omitempty"`    // raw position JSON
	SortIndex  string  `json:"sort_index,omitempty"`
	IsExternal bool    `json:"is_external"`
}

type ItemRef struct {
	Key      string `json:"key"`
	ItemType string `json:"item_type,omitempty"`
	Title    string `json:"title,omitempty"`
}

type Relation struct {
	Predicate string  `json:"predicate"`
	Direction string  `json:"direction"`
	Target    ItemRef `json:"target"`
}
