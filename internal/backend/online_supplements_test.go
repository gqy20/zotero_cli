package backend

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"zotero_cli/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDiscoverOnlineSupplementsZenodo(t *testing.T) {
	client := fakeSupplementClient(t, func(req *http.Request) string {
		if req.URL.String() != "https://zenodo.org/api/records/12345" {
			t.Fatalf("unexpected URL: %s", req.URL)
		}
		return `{"metadata":{"resource_type":{"type":"dataset"}},"files":[{"key":"dataset.xlsx","size":42,"links":{"self":"https://zenodo.org/api/records/12345/files/dataset.xlsx/content"}}]}`
	})

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key:   "ITEM1234",
		Title: "Zenodo item",
		DOI:   "10.5281/zenodo.12345",
	})

	if len(got.Supplements) != 1 {
		t.Fatalf("expected 1 supplement, got %#v", got)
	}
	s := got.Supplements[0]
	if s.Provider != "zenodo" || s.Label != "dataset.xlsx" || s.DownloadURL == "" || s.LinkType != "api_content" || s.Size != 42 {
		t.Fatalf("unexpected supplement: %#v", s)
	}
	if len(got.Providers) != 1 || got.Providers[0].Status != "complete" {
		t.Fatalf("unexpected provider status: %#v", got.Providers)
	}
}

func TestDiscoverOnlineSupplementsZenodoSoftwareRanksDataFilesHigher(t *testing.T) {
	client := fakeSupplementClient(t, func(req *http.Request) string {
		return `{"metadata":{"resource_type":{"type":"software"}},"files":[
			{"key":"README.md","size":42,"links":{"self":"https://zenodo.org/api/records/12345/files/README.md/content"}},
			{"key":"results.csv","size":420,"links":{"self":"https://zenodo.org/api/records/12345/files/results.csv/content"}}
		]}`
	})

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key: "ITEM1234",
		DOI: "10.5281/zenodo.12345",
	})

	if len(got.Supplements) != 2 {
		t.Fatalf("expected 2 supplements, got %#v", got)
	}
	if got.Supplements[0].Label != "results.csv" || got.Supplements[0].Confidence <= got.Supplements[1].Confidence {
		t.Fatalf("expected data file first, got %#v", got.Supplements)
	}
	if got.Supplements[1].Kind != "repository_file" {
		t.Fatalf("Kind = %q, want repository_file", got.Supplements[1].Kind)
	}
}

func TestDiscoverOnlineSupplementsFigshare(t *testing.T) {
	client := fakeSupplementClient(t, func(req *http.Request) string {
		if req.URL.String() != "https://api.figshare.com/v2/articles/5616409/files" {
			t.Fatalf("unexpected URL: %s", req.URL)
		}
		return `[{"id":9778696,"name":"table.xlsx","size":123,"download_url":"https://ndownloader.figshare.com/files/9778696","mimetype":"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}]`
	})

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key: "ITEM1234",
		DOI: "10.6084/m9.figshare.5616409.v3",
	})

	if len(got.Supplements) != 1 {
		t.Fatalf("expected 1 supplement, got %#v", got)
	}
	s := got.Supplements[0]
	if s.Provider != "figshare" || s.ContentType == "" || s.DownloadURL != "https://ndownloader.figshare.com/files/9778696" {
		t.Fatalf("unexpected supplement: %#v", s)
	}
	if s.LinkType != "direct_download" {
		t.Fatalf("LinkType = %q, want direct_download", s.LinkType)
	}
}

func TestDiscoverOnlineSupplementsNature(t *testing.T) {
	client := fakeSupplementClient(t, func(req *http.Request) string {
		if req.URL.Host != "www.nature.com" || req.URL.Path != "/articles/s41586-024-07447-4" {
			t.Fatalf("unexpected URL: %s", req.URL)
		}
		return `<html><body>
			<a href="https://static-content.springer.com/esm/art%3A10.1038%2Fs41586-024-07447-4/MediaObjects/41586_2024_7447_MOESM1_ESM.xlsx">Supplementary Table 1</a>
			<a href="https://static-content.springer.com/esm/art%3A10.1038%2Fs41586-024-07447-4/MediaObjects/41586_2024_7447_MOESM2_ESM.pdf">Reporting Summary</a>
			<a href="//static-content.springer.com/esm/art%3A10.1038%2Fs41586-024-07447-4/MediaObjects/41586_2024_7447_MOESM9_ESM.xlsx">Source data to Fig. 1</a>
			<a href="#MOESM2">Nature Portfolio Reporting Summary</a>
			<a href="/articles/s41586-024-07447-4.pdf">Download PDF</a>
		</body></html>`
	})

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key:   "ITEM1234",
		Title: "Nature item",
		DOI:   "10.1038/s41586-024-07447-4",
	})

	if len(got.Supplements) != 3 {
		t.Fatalf("expected 3 supplements, got %#v", got)
	}
	if got.Supplements[0].Provider != "nature" || got.Supplements[0].DownloadURL == "" {
		t.Fatalf("unexpected supplement: %#v", got.Supplements[0])
	}
	if len(got.Providers) != 1 || got.Providers[0].Status != "complete" {
		t.Fatalf("unexpected provider status: %#v", got.Providers)
	}
	foundLanding := false
	foundReportingSummary := false
	for _, s := range got.Supplements {
		if s.LinkType == "landing_page" {
			foundLanding = true
		}
		if s.Kind == "reporting_summary" && strings.Contains(s.DownloadURL, "static-content.springer.com") {
			foundReportingSummary = true
		}
	}
	if foundLanding {
		t.Fatalf("expected landing page to resolve to static link: %#v", got.Supplements)
	}
	if !foundReportingSummary {
		t.Fatalf("expected reporting summary static supplement: %#v", got.Supplements)
	}
}

func TestDiscoverOnlineSupplementsNatureOnlyLandingIsPartial(t *testing.T) {
	client := fakeSupplementClient(t, func(req *http.Request) string {
		return `<html><body><a href="#MOESM2">Nature Portfolio Reporting Summary</a></body></html>`
	})

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key: "ITEM1234",
		DOI: "10.1038/s41586-024-07447-4",
	})

	if len(got.Supplements) != 1 {
		t.Fatalf("expected 1 supplement, got %#v", got)
	}
	if got.Supplements[0].LinkType != "landing_page" {
		t.Fatalf("LinkType = %q, want landing_page", got.Supplements[0].LinkType)
	}
	if len(got.Providers) != 1 || got.Providers[0].Status != "partial" {
		t.Fatalf("unexpected provider status: %#v", got.Providers)
	}
}

func TestDiscoverOnlineSupplementsNatureLandingStaticProbe(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			if req.URL.Host != "www.nature.com" || req.URL.Path != "/articles/s41586-024-07447-4" {
				t.Fatalf("unexpected GET URL: %s", req.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`<html><body><a href="#MOESM2">Nature Portfolio Reporting Summary</a></body></html>`)),
				Request:    req,
			}, nil
		case http.MethodHead:
			want := "https://static-content.springer.com/esm/art%3A10.1038%2Fs41586-024-07447-4/MediaObjects/41586_2024_7447_MOESM2_ESM.pdf"
			if req.URL.String() != want {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			}
			header := make(http.Header)
			header.Set("Content-Type", "application/pdf")
			return &http.Response{
				StatusCode:    http.StatusOK,
				Header:        header,
				Body:          io.NopCloser(strings.NewReader("")),
				ContentLength: 2048,
				Request:       req,
			}, nil
		default:
			t.Fatalf("unexpected method: %s", req.Method)
			return nil, nil
		}
	})}

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key: "ITEM1234",
		DOI: "10.1038/s41586-024-07447-4",
	})

	if len(got.Supplements) != 1 {
		t.Fatalf("expected 1 supplement, got %#v", got)
	}
	s := got.Supplements[0]
	if s.LinkType != "direct_download" || !strings.Contains(s.DownloadURL, "static-content.springer.com") {
		t.Fatalf("expected probed static download, got %#v", s)
	}
	if s.Size != 2048 || s.ContentType != "application/pdf" {
		t.Fatalf("expected probed metadata, got %#v", s)
	}
	if len(got.Providers) != 1 || got.Providers[0].Status != "complete" {
		t.Fatalf("unexpected provider status: %#v", got.Providers)
	}
}

func TestDiscoverOnlineSupplementsNatureLandingStaticProbeFromURL(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`<html><body><a href="#MOESM2">Nature Portfolio Reporting Summary</a></body></html>`)),
				Request:    req,
			}, nil
		}
		if req.Method == http.MethodHead && strings.Contains(req.URL.String(), "41586_2024_7447_MOESM2_ESM.pdf") {
			header := make(http.Header)
			header.Set("Content-Type", "application/pdf")
			return &http.Response{
				StatusCode:    http.StatusOK,
				Header:        header,
				Body:          io.NopCloser(strings.NewReader("")),
				ContentLength: 2048,
				Request:       req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})}

	got := DiscoverOnlineSupplements(context.Background(), client, domain.Item{
		Key: "ITEM1234",
		URL: "https://www.nature.com/articles/s41586-024-07447-4",
	})

	if len(got.Supplements) != 1 || got.Supplements[0].LinkType != "direct_download" {
		t.Fatalf("expected URL-only item to resolve static supplement, got %#v", got)
	}
}

func fakeSupplementClient(t *testing.T, body func(*http.Request) string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body(req))),
			Request:    req,
		}, nil
	})}
}
