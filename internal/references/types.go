package references

import "fmt"

type Source string

const (
	SourcePMC    Source = "pmc_jats"
	SourcePubMed Source = "pubmed"
	SourceGROBID Source = "grobid"
)

type Author struct {
	Family string `json:"family,omitempty"`
	Given  string `json:"given,omitempty"`
	Name   string `json:"name,omitempty"`
}

type Reference struct {
	Index         int      `json:"index"`
	ID            string   `json:"id,omitempty"`
	Raw           string   `json:"raw,omitempty"`
	Title         string   `json:"title,omitempty"`
	Authors       []Author `json:"authors,omitempty"`
	Container     string   `json:"container,omitempty"`
	Year          string   `json:"year,omitempty"`
	Volume        string   `json:"volume,omitempty"`
	Issue         string   `json:"issue,omitempty"`
	Pages         string   `json:"pages,omitempty"`
	DOI           string   `json:"doi,omitempty"`
	PMID          string   `json:"pmid,omitempty"`
	PMCID         string   `json:"pmcid,omitempty"`
	Source        Source   `json:"source"`
	TargetItemKey string   `json:"target_item_key,omitempty"`
	MatchMethod   string   `json:"match_method,omitempty"`
	MatchScore    float64  `json:"match_score,omitempty"`
	MatchStatus   string   `json:"match_status,omitempty"`
	ContextStatus string   `json:"context_status"`
	ContextCount  int      `json:"context_count"`
}

type Context struct {
	ReferenceID    string `json:"reference_id"`
	ReferenceIndex int    `json:"reference_index,omitempty"`
	Marker         string `json:"marker,omitempty"`
	Section        string `json:"section,omitempty"`
	Paragraph      string `json:"paragraph"`
	TargetItemKey  string `json:"target_item_key,omitempty"`
	Source         Source `json:"source"`
}

type Identifiers struct {
	DOI   string `json:"doi,omitempty"`
	PMID  string `json:"pmid,omitempty"`
	PMCID string `json:"pmcid,omitempty"`
}

const (
	ContextAvailable    = "available"
	ContextNotSupported = "not_supported"
	ContextNotFound     = "not_found"
	ContextParseFailed  = "parse_failed"
	ContextNotIndexed   = "not_indexed"
)

type ContextSummary struct {
	Status                   string  `json:"status"`
	ContextCount             int     `json:"context_count"`
	ReferencesWithContext    int     `json:"references_with_context"`
	ReferencesWithoutContext int     `json:"references_without_context"`
	Coverage                 float64 `json:"coverage"`
}

type Result struct {
	ItemKey        string         `json:"item_key"`
	ItemTitle      string         `json:"item_title"`
	Identifiers    Identifiers    `json:"identifiers"`
	Strategy       string         `json:"strategy"`
	References     []Reference    `json:"references"`
	Contexts       []Context      `json:"contexts,omitempty"`
	ContextSummary ContextSummary `json:"context_summary"`
	ContextError   string         `json:"context_error,omitempty"`
	CacheHits      int            `json:"cache_hits,omitempty"`
	NetworkCalls   int            `json:"network_calls,omitempty"`
	ElapsedMS      int64          `json:"elapsed_ms"`
}

func SummarizeContexts(strategy string, references []Reference, contexts []Context) ContextSummary {
	summary := ContextSummary{Status: ContextNotIndexed, ContextCount: len(contexts)}
	if strategy == string(SourcePubMed) {
		summary.Status = ContextNotSupported
	} else if strategy == string(SourcePMC) || strategy == string(SourceGROBID) {
		if len(contexts) > 0 {
			summary.Status = ContextAvailable
		} else {
			summary.Status = ContextNotFound
		}
	}
	seen := map[int]bool{}
	for _, c := range contexts {
		if c.ReferenceIndex > 0 {
			seen[c.ReferenceIndex] = true
		}
	}
	summary.ReferencesWithContext = len(seen)
	summary.ReferencesWithoutContext = len(references) - len(seen)
	if summary.ReferencesWithoutContext < 0 {
		summary.ReferencesWithoutContext = 0
	}
	if len(references) > 0 {
		summary.Coverage = float64(summary.ReferencesWithContext) / float64(len(references))
	}
	return summary
}

func AnnotateReferenceContexts(references []Reference, contexts []Context, overallStatus string) {
	counts := map[int]int{}
	for _, c := range contexts {
		if c.ReferenceIndex > 0 {
			counts[c.ReferenceIndex]++
		}
	}
	for i := range references {
		references[i].ContextCount = counts[references[i].Index]
		switch overallStatus {
		case ContextAvailable:
			if references[i].ContextCount > 0 {
				references[i].ContextStatus = ContextAvailable
			} else {
				references[i].ContextStatus = ContextNotFound
			}
		default:
			references[i].ContextStatus = overallStatus
		}
	}
}

type Options struct {
	Source  string
	Refresh bool
}

// UnsupportedError reports a deterministic NCBI coverage limitation. Retrying
// the same unchanged item cannot help; another backend such as GROBID may.
type UnsupportedError struct {
	ItemKey string
	Reason  string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("item %s is unsupported by NCBI: %s", e.ItemKey, e.Reason)
}
