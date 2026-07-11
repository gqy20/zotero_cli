package references

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"zotero_cli/internal/domain"
)

func TestServiceAutoPrefersPMCAndCaches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch strings.TrimPrefix(r.URL.Path, "/") {
		case "esearch.fcgi":
			fmt.Fprint(w, `{"esearchresult":{"idlist":["123"]}}`)
		case "efetch.fcgi":
			if r.URL.Query().Get("db") == "pubmed" {
				fmt.Fprint(w, pubmedXML("123", "PMC456", "10.1000/source", "Source article"))
				return
			}
			fmt.Fprint(w, `<article><back><ref-list><ref id="R1"><element-citation><article-title>Cited work</article-title><year>2020</year><pub-id pub-id-type="doi">10.1000/cited</pub-id></element-citation></ref></ref-list></back></article>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, CacheDir: filepath.Join(t.TempDir(), "cache"), MinInterval: time.Nanosecond, HTTPClient: server.Client()})
	service := NewService(client)
	item := domain.Item{Key: "ITEM1", Title: "Source article", DOI: "10.1000/source"}
	result, err := service.References(context.Background(), item, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Strategy != string(SourcePMC) || result.Identifiers.PMID != "123" || result.Identifiers.PMCID != "PMC456" || len(result.References) != 1 {
		t.Fatalf("result = %#v", result)
	}
	firstCalls := calls.Load()
	result, err = service.References(context.Background(), item, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != firstCalls || result.CacheHits < 3 {
		t.Fatalf("cache not used: calls=%d first=%d result=%#v", calls.Load(), firstCalls, result)
	}
}

func TestFetchPubMedReferencesUsesELinkAndBatchEFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/") {
		case "elink.fcgi":
			fmt.Fprint(w, `{"linksets":[{"linksetdbs":[{"links":["11","22"]}]}]}`)
		case "efetch.fcgi":
			if got := r.URL.Query().Get("id"); got != "11,22" {
				t.Fatalf("batched id = %q", got)
			}
			fmt.Fprint(w, `<PubmedArticleSet>`+pubmedXMLArticle("11", "", "10.1/one", "One")+pubmedXMLArticle("22", "PMC22", "10.1/two", "Two")+`</PubmedArticleSet>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(ClientConfig{BaseURL: server.URL, MinInterval: time.Nanosecond, HTTPClient: server.Client()})
	refs, err := client.FetchPubMedReferences(context.Background(), "99", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].PMID != "11" || refs[1].PMCID != "PMC22" || refs[1].Title != "Two" {
		t.Fatalf("refs = %#v", refs)
	}
}

func TestConcurrentClientRequestsRespectScheduledRate(t *testing.T) {
	var mu sync.Mutex
	var seen []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, time.Now())
		mu.Unlock()
		fmt.Fprint(w, `{"esearchresult":{"idlist":[]}}`)
	}))
	defer server.Close()
	client := NewClient(ClientConfig{BaseURL: server.URL, MinInterval: 25 * time.Millisecond, HTTPClient: server.Client()})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.ResolveDOI(context.Background(), fmt.Sprintf("10.1/%d", time.Now().UnixNano()), true)
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Fatalf("requests=%d", len(seen))
	}
	for i := 1; i < len(seen); i++ {
		if gap := seen[i].Sub(seen[i-1]); gap < 18*time.Millisecond {
			t.Fatalf("request gap=%s, want rate-limited", gap)
		}
	}
}

func pubmedXML(pmid, pmcid, doi, title string) string {
	return `<PubmedArticleSet>` + pubmedXMLArticle(pmid, pmcid, doi, title) + `</PubmedArticleSet>`
}

func pubmedXMLArticle(pmid, pmcid, doi, title string) string {
	return fmt.Sprintf(`<PubmedArticle><MedlineCitation><PMID>%s</PMID><Article><ArticleTitle>%s</ArticleTitle><Journal><JournalIssue><Volume>1</Volume><Issue>2</Issue><PubDate><Year>2024</Year></PubDate></JournalIssue><Title>Journal</Title></Journal><Pagination><MedlinePgn>1-9</MedlinePgn></Pagination><AuthorList><Author><LastName>Smith</LastName><ForeName>Jane</ForeName></Author></AuthorList></Article></MedlineCitation><PubmedData><ArticleIdList><ArticleId IdType="pubmed">%s</ArticleId><ArticleId IdType="doi">%s</ArticleId><ArticleId IdType="pmc">%s</ArticleId></ArticleIdList></PubmedData></PubmedArticle>`, pmid, title, pmid, doi, pmcid)
}
