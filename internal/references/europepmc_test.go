package references

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEuropePMCReferencesCitationsLinksAndAnnotations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/references"):
			fmt.Fprint(w, `{"referenceList":{"reference":[{"id":"11","source":"MED","title":"PubMed work","authorString":"A B","journalAbbreviation":"J","pubYear":2020},{"id":"PPR1","source":"PPR","title":"Preprint work","pubYear":2021}]}}`)
		case strings.HasSuffix(r.URL.Path, "/citations"):
			fmt.Fprint(w, `{"hitCount":9,"citationList":{"citation":[{"id":"22","source":"MED","title":"Citing work","pubYear":2024}]}}`)
		case strings.HasSuffix(r.URL.Path, "/databaseLinks"):
			fmt.Fprint(w, `{"dbCrossReferenceList":{"dbCrossReference":[{"dbName":"UniProt","id":"P12345"}]}}`)
		case strings.HasSuffix(r.URL.Path, "/annotationsByArticleIds"):
			fmt.Fprint(w, `[{"annotations":[{"provider":"Europe PMC","type":"Genes/Proteins","section":"Abstract","exact":"BRCA1","prefix":"the ","postfix":" gene","tags":[{"name":"BRCA1","uri":"urn:gene:672"}]}]}]`)
		case strings.HasSuffix(r.URL.Path, "/search"):
			fmt.Fprint(w, `{"resultList":{"result":[{"id":"1","source":"MED","pmid":"1","doi":"10.1/a","title":"Article","isOpenAccess":"Y","license":"cc by","citedByCount":12,"hasReferences":"Y","hasTextMinedTerms":"Y","hasData":"Y","hasEvaluations":"Y","grantsList":{"grant":[{"grantId":"G1","agency":"Wellcome","acronym":"WT"}]},"commentCorrectionList":{"commentCorrection":[{"id":"PPR1","source":"PPR","type":"Preprint in"}]}}]}}`)
		case strings.Contains(r.URL.Path, "/evaluations/"):
			fmt.Fprint(w, `{"evaluationList":{"evaluation":[{"id":"E1","source":"Sciety","type":"review","url":"https://example.test/e1"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := NewClient(ClientConfig{BaseURL: server.URL, EuropePMCBaseURL: server.URL, EuropePMCAnnotationsBaseURL: server.URL, MinInterval: time.Nanosecond, HTTPClient: server.Client()})
	refs, err := c.FetchEuropeReferences(context.Background(), "1", false)
	if err != nil || len(refs) != 2 || refs[0].PMID != "11" || refs[1].Source != SourceEuropePMC {
		t.Fatalf("refs=%#v err=%v", refs, err)
	}
	cites, total, err := c.FetchEuropeCitations(context.Background(), "1", 5, false)
	if err != nil || total != 9 || cites[0].ID != "22" {
		t.Fatalf("cites=%#v total=%d err=%v", cites, total, err)
	}
	links, err := c.FetchEuropeDataLinks(context.Background(), "1", false)
	if err != nil || len(links) != 1 || links[0].IDs[0] != "P12345" {
		t.Fatalf("links=%#v err=%v", links, err)
	}
	annotations, err := c.FetchEuropeAnnotations(context.Background(), "1", false)
	if err != nil || len(annotations) != 1 || annotations[0].Entity != "urn:gene:672" || annotations[0].Suffix != " gene" {
		t.Fatalf("annotations=%#v err=%v", annotations, err)
	}
	profile, err := c.FetchEuropeProfile(context.Background(), Identifiers{PMID: "1"}, false)
	if err != nil || profile.CitedByCount != 12 || len(profile.Grants) != 1 || len(profile.Versions) != 1 || len(profile.Evaluations) != 1 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
}

func TestMergeEuropeReferencesDeduplicatesAndAppends(t *testing.T) {
	primary := []Reference{{Index: 1, PMID: "11", Title: "Same"}}
	extra := []Reference{{PMID: "11", Title: "Duplicate", Source: SourceEuropePMC}, {Title: "New preprint", DOI: "10.1/new", Source: SourceEuropePMC}}
	got := MergeReferences(primary, extra)
	if len(got) != 2 || got[1].Index != 2 || got[1].DOI != "10.1/new" {
		t.Fatalf("merged=%#v", got)
	}
}
