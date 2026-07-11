package references

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"zotero_cli/internal/domain"
)

var (
	pmidPattern  = regexp.MustCompile(`(?i)(?:PMID[:/\s]*|pubmed\.ncbi\.nlm\.nih\.gov/)(\d{5,10})`)
	pmcidPattern = regexp.MustCompile(`(?i)(PMC\d{4,12})`)
)

type Service struct{ client *Client }

func NewService(client *Client) *Service { return &Service{client: client} }

func (s *Service) ResolveItem(ctx context.Context, item domain.Item, refresh bool) (Identifiers, error) {
	ids := identifiersFromItem(item)
	if ids.PMID == "" && ids.DOI != "" {
		pmid, err := s.client.ResolveDOI(ctx, ids.DOI, refresh)
		if err != nil {
			return ids, err
		}
		ids.PMID = pmid
	}
	if ids.PMID == "" {
		return ids, &UnsupportedError{ItemKey: item.Key, Reason: "could not be resolved to PubMed"}
	}
	return ids, nil
}

func (s *Service) Related(ctx context.Context, item domain.Item, limit int, alsoViewed, refresh bool) ([]RelatedArticle, Identifiers, error) {
	ids, err := s.ResolveItem(ctx, item, refresh)
	if err != nil {
		return nil, ids, err
	}
	var rows []RelatedArticle
	if alsoViewed {
		rows, err = s.client.FetchAlsoViewedArticles(ctx, ids.PMID, limit, refresh)
	} else {
		rows, err = s.client.FetchRelatedArticles(ctx, ids.PMID, limit, refresh)
	}
	return rows, ids, err
}

func (s *Service) Links(ctx context.Context, item domain.Item, refresh bool) ([]ResourceLink, Identifiers, error) {
	ids, err := s.ResolveItem(ctx, item, refresh)
	if err != nil {
		return nil, ids, err
	}
	rows, err := s.client.FetchResourceLinks(ctx, ids.PMID, refresh)
	return rows, ids, err
}

func (s *Service) References(ctx context.Context, item domain.Item, opts Options) (Result, error) {
	started := time.Now()
	result := Result{ItemKey: item.Key, ItemTitle: item.Title, Identifiers: identifiersFromItem(item)}
	if result.Identifiers.PMID == "" && result.Identifiers.DOI != "" {
		pmid, err := s.client.ResolveDOI(ctx, result.Identifiers.DOI, opts.Refresh)
		if err != nil {
			return result, err
		}
		result.Identifiers.PMID = pmid
	}
	if result.Identifiers.PMID != "" && result.Identifiers.PMCID == "" {
		record, err := s.client.FetchPubMedArticle(ctx, result.Identifiers.PMID, opts.Refresh)
		if err != nil {
			return result, err
		}
		result.Identifiers.PMCID = record.PMCID
		if result.Identifiers.DOI == "" {
			result.Identifiers.DOI = record.DOI
		}
		result.Metadata = record.Metadata
	} else if result.Identifiers.PMID != "" {
		record, err := s.client.FetchPubMedArticle(ctx, result.Identifiers.PMID, opts.Refresh)
		if err != nil {
			return result, err
		}
		result.Metadata = record.Metadata
	}

	source := strings.ToLower(strings.TrimSpace(opts.Source))
	if source == "" {
		source = "auto"
	}
	switch {
	case source == "pmc" && result.Identifiers.PMCID == "":
		return result, &UnsupportedError{ItemKey: item.Key, Reason: "no PMC identifier"}
	case source == "pubmed" && result.Identifiers.PMID == "":
		return result, &UnsupportedError{ItemKey: item.Key, Reason: "could not be resolved to PubMed"}
	case source != "auto" && source != "pmc" && source != "pubmed":
		return result, fmt.Errorf("invalid reference source %q", source)
	}

	var err error
	if result.Identifiers.PMCID != "" && source != "pubmed" {
		result.Strategy = string(SourcePMC)
		result.References, result.Contexts, result.ContextError, err = s.client.FetchPMCDocument(ctx, result.Identifiers.PMCID, opts.Refresh)
	} else if result.Identifiers.PMID != "" {
		result.Strategy = string(SourcePubMed)
		result.References, err = s.client.FetchPubMedReferences(ctx, result.Identifiers.PMID, opts.Refresh)
	} else {
		err = &UnsupportedError{ItemKey: item.Key, Reason: "no DOI, PMID, or PMCID usable by NCBI"}
	}
	result.CacheHits, result.NetworkCalls = s.client.Stats()
	result.ContextSummary = SummarizeContexts(result.Strategy, result.References, result.Contexts)
	if result.ContextError != "" {
		result.ContextSummary.Status = ContextParseFailed
	}
	AnnotateReferenceContexts(result.References, result.Contexts, result.ContextSummary.Status)
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, err
}

func identifiersFromItem(item domain.Item) Identifiers {
	text := strings.Join([]string{item.URL, item.Abstract}, " ")
	ids := Identifiers{DOI: normalizeDOI(item.DOI)}
	if match := pmidPattern.FindStringSubmatch(text); len(match) > 1 {
		ids.PMID = match[1]
	}
	if match := pmcidPattern.FindStringSubmatch(text); len(match) > 1 {
		ids.PMCID = normalizePMCID(match[1])
	}
	return ids
}

func normalizeDOI(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, prefix := range []string{"https://doi.org/", "http://doi.org/", "doi:"} {
		value = strings.TrimPrefix(value, prefix)
	}
	return strings.TrimSpace(value)
}

func normalizePMCID(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value != "" && !strings.HasPrefix(value, "PMC") {
		value = "PMC" + value
	}
	return value
}

func cleanSpace(value string) string { return strings.Join(strings.Fields(value), " ") }
