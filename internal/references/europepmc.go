package references

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const europePMCBaseURL = "https://www.ebi.ac.uk/europepmc/webservices/rest"
const europePMCAnnotationsBaseURL = "https://www.ebi.ac.uk/europepmc/annotations_api"

type EuropeCitation struct {
	ID      string `json:"id,omitempty"`
	Source  string `json:"source,omitempty"`
	PMID    string `json:"pmid,omitempty"`
	PMCID   string `json:"pmcid,omitempty"`
	DOI     string `json:"doi,omitempty"`
	Title   string `json:"title,omitempty"`
	Authors string `json:"authorString,omitempty"`
	Journal string `json:"journalAbbreviation,omitempty"`
	Year    int    `json:"pubYear,omitempty"`
}

type Annotation struct {
	Provider string `json:"provider,omitempty"`
	Type     string `json:"type,omitempty"`
	Entity   string `json:"entity,omitempty"`
	Label    string `json:"label,omitempty"`
	Section  string `json:"section,omitempty"`
	Exact    string `json:"exact,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
}

type EuropeVersion struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	Reference string `json:"reference,omitempty"`
}
type EuropeEvaluation struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source,omitempty"`
	Type   string `json:"type,omitempty"`
	URL    string `json:"url,omitempty"`
	Text   string `json:"text,omitempty"`
}
type EuropeProfile struct {
	ID             string             `json:"id"`
	Source         string             `json:"source"`
	PMID           string             `json:"pmid,omitempty"`
	PMCID          string             `json:"pmcid,omitempty"`
	DOI            string             `json:"doi,omitempty"`
	Title          string             `json:"title,omitempty"`
	OpenAccess     bool               `json:"open_access"`
	License        string             `json:"license,omitempty"`
	CitedByCount   int                `json:"cited_by_count"`
	HasReferences  bool               `json:"has_references"`
	HasAnnotations bool               `json:"has_annotations"`
	HasData        bool               `json:"has_data"`
	Grants         []Grant            `json:"grants,omitempty"`
	Versions       []EuropeVersion    `json:"versions,omitempty"`
	Evaluations    []EuropeEvaluation `json:"evaluations,omitempty"`
}

func (c *Client) FetchEuropeProfile(ctx context.Context, ids Identifiers, refresh bool) (EuropeProfile, error) {
	query := ""
	if ids.PMID != "" {
		query = "EXT_ID:" + ids.PMID + " AND SRC:MED"
	} else if ids.DOI != "" {
		query = `DOI:"` + normalizeDOI(ids.DOI) + `"`
	} else {
		return EuropeProfile{}, fmt.Errorf("Europe PMC profile requires PMID or DOI")
	}
	data, err := c.getFrom(ctx, c.cfg.EuropePMCBaseURL, "search", url.Values{"query": {query}, "resultType": {"core"}, "format": {"json"}, "pageSize": {"1"}}, refresh)
	if err != nil {
		return EuropeProfile{}, err
	}
	type coreGrant struct {
		ID      string `json:"grantId"`
		Agency  string `json:"agency"`
		Acronym string `json:"acronym"`
	}
	type correction struct {
		ID        string `json:"id"`
		Source    string `json:"source"`
		Type      string `json:"type"`
		Reference string `json:"reference"`
	}
	var response struct {
		Results struct {
			Rows []struct {
				ID             string `json:"id"`
				Source         string `json:"source"`
				PMID           string `json:"pmid"`
				PMCID          string `json:"pmcid"`
				DOI            string `json:"doi"`
				Title          string `json:"title"`
				OA             string `json:"isOpenAccess"`
				License        string `json:"license"`
				Cited          int    `json:"citedByCount"`
				HasRefs        string `json:"hasReferences"`
				HasTerms       string `json:"hasTextMinedTerms"`
				HasData        string `json:"hasData"`
				HasEvaluations string `json:"hasEvaluations"`
				Grants         struct {
					Rows []coreGrant `json:"grant"`
				} `json:"grantsList"`
				Corrections struct {
					Rows []correction `json:"commentCorrection"`
				} `json:"commentCorrectionList"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return EuropeProfile{}, fmt.Errorf("decode Europe PMC profile: %w", err)
	}
	if len(response.Results.Rows) == 0 {
		return EuropeProfile{}, fmt.Errorf("article not found in Europe PMC")
	}
	row := response.Results.Rows[0]
	p := EuropeProfile{ID: row.ID, Source: row.Source, PMID: row.PMID, PMCID: row.PMCID, DOI: row.DOI, Title: row.Title, OpenAccess: row.OA == "Y", License: row.License, CitedByCount: row.Cited, HasReferences: row.HasRefs == "Y", HasAnnotations: row.HasTerms == "Y", HasData: row.HasData == "Y"}
	for _, g := range row.Grants.Rows {
		p.Grants = append(p.Grants, Grant{ID: g.ID, Agency: g.Agency, Acronym: g.Acronym})
	}
	for _, v := range row.Corrections.Rows {
		if strings.Contains(strings.ToLower(v.Type), "preprint") {
			p.Versions = append(p.Versions, EuropeVersion{ID: v.ID, Source: v.Source, Type: v.Type, Reference: v.Reference})
		}
	}
	if row.HasEvaluations == "Y" {
		p.Evaluations, _ = c.FetchEuropeEvaluations(ctx, row.Source, row.ID, refresh)
	}
	return p, nil
}

func (c *Client) FetchEuropeEvaluations(ctx context.Context, source, id string, refresh bool) ([]EuropeEvaluation, error) {
	data, err := c.getFrom(ctx, c.cfg.EuropePMCBaseURL, "evaluations/"+source+"/"+id, url.Values{"format": {"json"}}, refresh)
	if err != nil {
		return nil, err
	}
	var response struct {
		List struct {
			Rows []EuropeEvaluation `json:"evaluation"`
		} `json:"evaluationList"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Europe PMC evaluations: %w", err)
	}
	return response.List.Rows, nil
}

func (c *Client) FetchEuropeAnnotations(ctx context.Context, pmid string, refresh bool) ([]Annotation, error) {
	data, err := c.getFrom(ctx, c.cfg.EuropePMCAnnotationsBaseURL, "annotationsByArticleIds", url.Values{"articleIds": {"MED:" + pmid}, "format": {"JSON"}}, refresh)
	if err != nil {
		return nil, err
	}
	var articles []struct {
		Annotations []struct {
			Provider string `json:"provider"`
			Type     string `json:"type"`
			Section  string `json:"section"`
			Exact    string `json:"exact"`
			Prefix   string `json:"prefix"`
			Postfix  string `json:"postfix"`
			Tags     []struct {
				Name string `json:"name"`
				URI  string `json:"uri"`
			} `json:"tags"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(data, &articles); err != nil {
		return nil, fmt.Errorf("decode Europe PMC annotations: %w", err)
	}
	out := []Annotation{}
	for _, article := range articles {
		for _, a := range article.Annotations {
			if len(a.Tags) == 0 {
				out = append(out, Annotation{Provider: a.Provider, Type: a.Type, Section: a.Section, Exact: a.Exact, Prefix: a.Prefix, Suffix: a.Postfix})
				continue
			}
			for _, tag := range a.Tags {
				out = append(out, Annotation{Provider: a.Provider, Type: a.Type, Entity: tag.URI, Label: tag.Name, Section: a.Section, Exact: a.Exact, Prefix: a.Prefix, Suffix: a.Postfix})
			}
		}
	}
	return out, nil
}

func (c *Client) FetchEuropeReferences(ctx context.Context, pmid string, refresh bool) ([]Reference, error) {
	rows, err := c.fetchEuropeCitationList(ctx, pmid, "references", refresh)
	if err != nil {
		return nil, err
	}
	out := make([]Reference, 0, len(rows))
	for i, row := range rows {
		pmid := row.PMID
		if pmid == "" && row.Source == "MED" {
			pmid = row.ID
		}
		year := ""
		if row.Year > 0 {
			year = fmt.Sprint(row.Year)
		}
		out = append(out, Reference{Index: i + 1, Title: row.Title, Container: row.Journal, Year: year, DOI: normalizeDOI(row.DOI), PMID: pmid, PMCID: normalizePMCID(row.PMCID), Source: SourceEuropePMC})
	}
	return out, nil
}

func (c *Client) FetchEuropeCitations(ctx context.Context, pmid string, limit int, refresh bool) ([]EuropeCitation, int, error) {
	values := url.Values{"format": {"json"}, "pageSize": {fmt.Sprint(limit)}}
	data, err := c.getFrom(ctx, c.cfg.EuropePMCBaseURL, "MED/"+pmid+"/citations", values, refresh)
	if err != nil {
		return nil, 0, err
	}
	var response struct {
		HitCount int `json:"hitCount"`
		List     struct {
			Rows []EuropeCitation `json:"citation"`
		} `json:"citationList"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, 0, fmt.Errorf("decode Europe PMC citations: %w", err)
	}
	return response.List.Rows, response.HitCount, nil
}

func (c *Client) fetchEuropeCitationList(ctx context.Context, pmid, kind string, refresh bool) ([]EuropeCitation, error) {
	values := url.Values{"format": {"json"}, "pageSize": {"1000"}}
	data, err := c.getFrom(ctx, c.cfg.EuropePMCBaseURL, "MED/"+pmid+"/"+kind, values, refresh)
	if err != nil {
		return nil, err
	}
	var response struct {
		List struct {
			Rows []EuropeCitation `json:"reference"`
		} `json:"referenceList"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Europe PMC %s: %w", kind, err)
	}
	return response.List.Rows, nil
}

func (c *Client) FetchEuropeDataLinks(ctx context.Context, pmid string, refresh bool) ([]ResourceLink, error) {
	data, err := c.getFrom(ctx, c.cfg.EuropePMCBaseURL, "MED/"+pmid+"/databaseLinks", url.Values{"format": {"json"}}, refresh)
	if err != nil {
		return nil, err
	}
	var response struct {
		List struct {
			Rows []struct {
				Database string `json:"dbName"`
				ID       string `json:"id"`
			} `json:"dbCrossReference"`
		} `json:"dbCrossReferenceList"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Europe PMC database links: %w", err)
	}
	grouped := map[string][]string{}
	order := []string{}
	for _, row := range response.List.Rows {
		name := strings.ToLower(strings.TrimSpace(row.Database))
		if name == "" || row.ID == "" {
			continue
		}
		if _, ok := grouped[name]; !ok {
			order = append(order, name)
		}
		grouped[name] = append(grouped[name], row.ID)
	}
	out := []ResourceLink{}
	for _, name := range order {
		out = append(out, ResourceLink{Database: name, LinkName: "europepmc_database_links", IDs: grouped[name]})
	}
	return out, nil
}

func MergeReferences(primary, supplement []Reference) []Reference {
	seen := map[string]bool{}
	out := append([]Reference(nil), primary...)
	for _, r := range primary {
		for _, k := range referenceMatchKeys(r) {
			seen[k] = true
		}
	}
	for _, r := range supplement {
		duplicate := false
		for _, k := range referenceMatchKeys(r) {
			if seen[k] {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		r.Index = len(out) + 1
		out = append(out, r)
		for _, k := range referenceMatchKeys(r) {
			seen[k] = true
		}
	}
	return out
}
