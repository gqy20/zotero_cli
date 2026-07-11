package references

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"zotero_cli/internal/domain"
)

type ResolveReport struct {
	Total      int   `json:"total"`
	Resolved   int   `json:"resolved"`
	Unresolved int   `json:"unresolved"`
	DOI        int   `json:"doi"`
	PMID       int   `json:"pmid"`
	ExactTitle int   `json:"exact_title"`
	FuzzyTitle int   `json:"fuzzy_title"`
	ElapsedMS  int64 `json:"elapsed_ms"`
}

type Resolver struct {
	items            map[string]domain.Item
	doi, pmid, title map[string]string
	tokens           map[string][]string
}

var resolverWords = regexp.MustCompile(`[\p{L}\p{N}]+`)

func NewResolver(items []domain.Item) *Resolver {
	r := &Resolver{items: map[string]domain.Item{}, doi: map[string]string{}, pmid: map[string]string{}, title: map[string]string{}, tokens: map[string][]string{}}
	for _, item := range items {
		if item.Key == "" || item.Title == "" {
			continue
		}
		r.items[item.Key] = item
		ids := identifiersFromItem(item)
		if ids.DOI != "" {
			r.doi[strings.ToLower(ids.DOI)] = item.Key
		}
		if ids.PMID != "" {
			r.pmid[ids.PMID] = item.Key
		}
		n := normalizeTitle(item.Title)
		if n != "" {
			r.title[n] = item.Key
		}
		for _, token := range titleTokens(n) {
			r.tokens[token] = append(r.tokens[token], item.Key)
		}
	}
	return r
}

func (r *Resolver) Resolve(ref Reference, sourceKey string) Reference {
	ref.TargetItemKey, ref.MatchMethod, ref.MatchScore, ref.MatchStatus = "", "", 0, "unresolved"
	if key := r.doi[strings.ToLower(strings.TrimSpace(ref.DOI))]; key != "" && key != sourceKey {
		return resolved(ref, key, "doi", 1)
	}
	if key := r.pmid[strings.TrimSpace(ref.PMID)]; key != "" && key != sourceKey {
		return resolved(ref, key, "pmid", 1)
	}
	n := normalizeTitle(ref.Title)
	if key := r.title[n]; n != "" && key != "" && key != sourceKey {
		return resolved(ref, key, "title_exact", 1)
	}
	query := titleTokens(n)
	if len(query) < 3 {
		return ref
	}
	counts := map[string]int{}
	for _, token := range query {
		for _, key := range r.tokens[token] {
			if key != sourceKey {
				counts[key]++
			}
		}
	}
	type candidate struct {
		key   string
		score float64
	}
	cs := make([]candidate, 0, len(counts))
	for key := range counts {
		other := titleTokens(normalizeTitle(r.items[key].Title))
		score := dice(query, other)
		if score >= .88 {
			cs = append(cs, candidate{key, score})
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
	if len(cs) > 0 && (len(cs) == 1 || cs[0].score-cs[1].score >= .03) {
		return resolved(ref, cs[0].key, "title_fuzzy", cs[0].score)
	}
	return ref
}

func resolved(ref Reference, key, method string, score float64) Reference {
	ref.TargetItemKey, ref.MatchMethod, ref.MatchScore, ref.MatchStatus = key, method, score, "resolved"
	return ref
}
func normalizeTitle(s string) string {
	return strings.Join(resolverWords.FindAllString(strings.ToLower(norm.NFKC.String(s)), -1), " ")
}
func titleTokens(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, w := range strings.Fields(s) {
		w = strings.TrimFunc(w, unicode.IsSpace)
		if len([]rune(w)) < 2 || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}
func dice(a, b []string) float64 {
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	n := 0
	for _, x := range b {
		if m[x] {
			n++
		}
	}
	if len(a)+len(b) == 0 {
		return 0
	}
	return 2 * float64(n) / float64(len(a)+len(b))
}
