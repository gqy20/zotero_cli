package domain

type Item struct {
	Version     int          `json:"version,omitempty"`
	Key         string       `json:"key"`
	ItemType    string       `json:"item_type"`
	Title       string       `json:"title"`
	Date        string       `json:"date"`
	Creators    []Creator    `json:"creators"`
	Container   string       `json:"container,omitempty"`
	DOI         string       `json:"doi,omitempty"`
	URL         string       `json:"url,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Collections []Collection `json:"collections,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
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
