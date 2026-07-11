package references

type Source string

const (
	SourcePMC    Source = "pmc_jats"
	SourcePubMed Source = "pubmed"
)

type Author struct {
	Family string `json:"family,omitempty"`
	Given  string `json:"given,omitempty"`
	Name   string `json:"name,omitempty"`
}

type Reference struct {
	Index     int      `json:"index"`
	ID        string   `json:"id,omitempty"`
	Raw       string   `json:"raw,omitempty"`
	Title     string   `json:"title,omitempty"`
	Authors   []Author `json:"authors,omitempty"`
	Container string   `json:"container,omitempty"`
	Year      string   `json:"year,omitempty"`
	Volume    string   `json:"volume,omitempty"`
	Issue     string   `json:"issue,omitempty"`
	Pages     string   `json:"pages,omitempty"`
	DOI       string   `json:"doi,omitempty"`
	PMID      string   `json:"pmid,omitempty"`
	PMCID     string   `json:"pmcid,omitempty"`
	Source    Source   `json:"source"`
}

type Identifiers struct {
	DOI   string `json:"doi,omitempty"`
	PMID  string `json:"pmid,omitempty"`
	PMCID string `json:"pmcid,omitempty"`
}

type Result struct {
	ItemKey      string      `json:"item_key"`
	ItemTitle    string      `json:"item_title"`
	Identifiers  Identifiers `json:"identifiers"`
	Strategy     string      `json:"strategy"`
	References   []Reference `json:"references"`
	CacheHits    int         `json:"cache_hits,omitempty"`
	NetworkCalls int         `json:"network_calls,omitempty"`
	ElapsedMS    int64       `json:"elapsed_ms"`
}

type Options struct {
	Source  string
	Refresh bool
}
