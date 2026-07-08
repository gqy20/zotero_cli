package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"zotero_cli/internal/domain"
)

const supplementUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36 zotero-cli/0.1"

type OnlineSupplementDiscovery struct {
	Supplements []Supplement               `json:"supplements"`
	Providers   []SupplementProviderStatus `json:"providers"`
}

type SupplementProviderStatus struct {
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	SourceURL string `json:"source_url,omitempty"`
	Message   string `json:"message,omitempty"`
}

func DiscoverOnlineSupplements(ctx context.Context, client *http.Client, item domain.Item) OnlineSupplementDiscovery {
	if client == nil {
		client = http.DefaultClient
	}
	discovery := OnlineSupplementDiscovery{}
	providers := []onlineSupplementProvider{
		zenodoSupplementProvider{},
		figshareSupplementProvider{},
		natureSupplementProvider{},
	}
	for _, provider := range providers {
		result := provider.discover(ctx, client, item)
		if result.status.Provider == "" {
			continue
		}
		discovery.Providers = append(discovery.Providers, result.status)
		discovery.Supplements = append(discovery.Supplements, result.supplements...)
	}
	sort.SliceStable(discovery.Supplements, func(i, j int) bool {
		if discovery.Supplements[i].Confidence != discovery.Supplements[j].Confidence {
			return discovery.Supplements[i].Confidence > discovery.Supplements[j].Confidence
		}
		if discovery.Supplements[i].Provider != discovery.Supplements[j].Provider {
			return discovery.Supplements[i].Provider < discovery.Supplements[j].Provider
		}
		return discovery.Supplements[i].Label < discovery.Supplements[j].Label
	})
	return discovery
}

type onlineSupplementProvider interface {
	discover(context.Context, *http.Client, domain.Item) onlineSupplementProviderResult
}

type onlineSupplementProviderResult struct {
	status      SupplementProviderStatus
	supplements []Supplement
}

var (
	zenodoDOIPattern     = regexp.MustCompile(`(?i)10\.5281/zenodo\.(\d+)`)
	zenodoURLPattern     = regexp.MustCompile(`(?i)zenodo\.org/(?:records|record|doi/10\.5281/zenodo)\.?/(\d+)`)
	figshareDOIPattern   = regexp.MustCompile(`(?i)10\.6084/m9\.figshare\.(\d+)`)
	figshareURLPattern   = regexp.MustCompile(`(?i)figshare\.com/.*/(\d+)(?:\.v\d+)?(?:[/?#]|$)`)
	natureDOIPattern     = regexp.MustCompile(`(?i)^10\.1038/(.+)$`)
	bmcSpringerDOIPrefix = regexp.MustCompile(`(?i)^10\.(1186|1007)/`)
)

type zenodoSupplementProvider struct{}

func (zenodoSupplementProvider) discover(ctx context.Context, client *http.Client, item domain.Item) onlineSupplementProviderResult {
	recordID := firstRegexGroup(zenodoDOIPattern, item.DOI, item.URL)
	if recordID == "" {
		recordID = firstRegexGroup(zenodoURLPattern, item.URL)
	}
	if recordID == "" {
		return onlineSupplementProviderResult{}
	}
	sourceURL := "https://zenodo.org/api/records/" + recordID
	status := SupplementProviderStatus{Provider: "zenodo", SourceURL: sourceURL}
	var payload struct {
		Metadata struct {
			ResourceType struct {
				Type string `json:"type"`
			} `json:"resource_type"`
		} `json:"metadata"`
		Files []struct {
			Key   string `json:"key"`
			Size  int64  `json:"size"`
			Links struct {
				Self string `json:"self"`
			} `json:"links"`
		} `json:"files"`
	}
	if err := getJSON(ctx, client, sourceURL, &payload); err != nil {
		status.Status = "blocked"
		status.Message = err.Error()
		return onlineSupplementProviderResult{status: status}
	}
	status.Status = "complete"
	out := make([]Supplement, 0, len(payload.Files))
	for _, file := range payload.Files {
		s := onlineSupplementFromFile(item, "zenodo", sourceURL, file.Key, file.Links.Self, file.Size, 0.98)
		s.LinkType = "api_content"
		if strings.EqualFold(payload.Metadata.ResourceType.Type, "software") {
			s.Kind = "repository_file"
			s.Confidence = zenodoSoftwareFileConfidence(file.Key)
			s.Evidence = append(s.Evidence, "resource_type:software")
		}
		out = append(out, s)
	}
	return onlineSupplementProviderResult{status: status, supplements: out}
}

func zenodoSoftwareFileConfidence(label string) float64 {
	lower := strings.ToLower(label)
	base := path.Base(lower)
	ext := strings.ToLower(path.Ext(base))
	switch {
	case base == "readme.md" || base == "readme" || base == "license" || base == ".gitignore":
		return 0.35
	case strings.HasSuffix(base, ".sample") || base == "cargo.toml" || base == "cargo.lock":
		return 0.35
	case ext == ".rs" || ext == ".py" || ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".toml":
		return 0.45
	case ext == ".zip" || ext == ".gz" || ext == ".tgz" || ext == ".tar":
		return 0.82
	case ext == ".csv" || ext == ".tsv" || ext == ".xlsx" || ext == ".json" || ext == ".jsonl":
		return 0.8
	case ext == ".fa" || ext == ".fasta" || ext == ".fastq" || ext == ".vcf" || ext == ".bed":
		return 0.8
	default:
		return 0.55
	}
}

type figshareSupplementProvider struct{}

func (figshareSupplementProvider) discover(ctx context.Context, client *http.Client, item domain.Item) onlineSupplementProviderResult {
	articleID := firstRegexGroup(figshareDOIPattern, item.DOI, item.URL)
	if articleID == "" {
		articleID = firstRegexGroup(figshareURLPattern, item.URL)
	}
	if articleID == "" {
		return onlineSupplementProviderResult{}
	}
	sourceURL := "https://api.figshare.com/v2/articles/" + articleID + "/files"
	status := SupplementProviderStatus{Provider: "figshare", SourceURL: sourceURL}
	var files []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		DownloadURL string `json:"download_url"`
		Mimetype    string `json:"mimetype"`
	}
	if err := getJSON(ctx, client, sourceURL, &files); err != nil {
		status.Status = "blocked"
		status.Message = err.Error()
		return onlineSupplementProviderResult{status: status}
	}
	status.Status = "complete"
	out := make([]Supplement, 0, len(files))
	for _, file := range files {
		s := onlineSupplementFromFile(item, "figshare", sourceURL, file.Name, file.DownloadURL, file.Size, 0.98)
		s.ContentType = file.Mimetype
		s.LinkType = "direct_download"
		out = append(out, s)
	}
	return onlineSupplementProviderResult{status: status, supplements: out}
}

type natureSupplementProvider struct{}

func (natureSupplementProvider) discover(ctx context.Context, client *http.Client, item domain.Item) onlineSupplementProviderResult {
	sourceURL := natureSourceURL(item)
	if sourceURL == "" {
		return onlineSupplementProviderResult{}
	}
	status := SupplementProviderStatus{Provider: "nature", SourceURL: sourceURL}
	body, finalURL, err := getText(ctx, client, sourceURL)
	if err != nil {
		status.Status = "blocked"
		status.Message = err.Error()
		return onlineSupplementProviderResult{status: status}
	}
	links := resolveNatureLandingLinks(finalURL, extractSupplementLinks(finalURL, body))
	if len(links) == 0 {
		status.Status = "partial"
		status.Message = "no supplement links found in fetched HTML"
		return onlineSupplementProviderResult{status: status}
	}
	out := make([]Supplement, 0, len(links))
	seen := map[string]bool{}
	for _, link := range links {
		key := link.href
		if seen[key] {
			continue
		}
		seen[key] = true
		supplement := natureSupplementFromLink(item, finalURL, link.text, link.href)
		if supplement.LinkType == "landing_page" {
			supplement = resolveNatureStaticCandidate(ctx, client, item, supplement)
		}
		out = append(out, supplement)
	}
	status.Status = "complete"
	if len(out) > 0 && !hasDirectSupplementLink(out) {
		status.Status = "partial"
		status.Message = "only landing-page supplement anchors found"
	}
	return onlineSupplementProviderResult{status: status, supplements: out}
}

func onlineSupplementFromFile(item domain.Item, provider string, sourceURL string, label string, downloadURL string, size int64, confidence float64) Supplement {
	kind, evidence := onlineSupplementKind(label, downloadURL)
	return Supplement{
		Provider:       provider,
		ProviderStatus: "complete",
		Kind:           kind,
		Label:          label,
		ItemKey:        item.Key,
		ItemTitle:      item.Title,
		SourceURL:      sourceURL,
		DownloadURL:    downloadURL,
		LinkType:       "direct_download",
		Size:           size,
		Confidence:     confidence,
		Evidence:       evidence,
	}
}

func natureSupplementFromLink(item domain.Item, sourceURL string, label string, downloadURL string) Supplement {
	label = cleanSupplementLabel(label)
	if strings.TrimSpace(label) == "" {
		label = path.Base(downloadURLPath(downloadURL))
	}
	kind, evidence := onlineSupplementKind(label, downloadURL)
	linkType := "direct_download"
	if isSamePageFragment(sourceURL, downloadURL) {
		linkType = "landing_page"
	}
	return Supplement{
		Provider:       "nature",
		ProviderStatus: "complete",
		Kind:           kind,
		Label:          label,
		ItemKey:        item.Key,
		ItemTitle:      item.Title,
		SourceURL:      sourceURL,
		DownloadURL:    downloadURL,
		LinkType:       linkType,
		Confidence:     0.9,
		Evidence:       evidence,
	}
}

func resolveNatureStaticCandidate(ctx context.Context, client *http.Client, item domain.Item, supplement Supplement) Supplement {
	moesm := moesmID(supplement.DownloadURL)
	prefix, doiSuffix := natureStaticPrefixFromItem(item, supplement.SourceURL)
	if moesm == "" || prefix == "" || doiSuffix == "" {
		return supplement
	}
	for _, ext := range natureCandidateExtensions(supplement) {
		candidate := fmt.Sprintf(
			"https://static-content.springer.com/esm/art%%3A10.1038%%2F%s/MediaObjects/%s_%s_ESM.%s",
			doiSuffix,
			prefix,
			moesm,
			ext,
		)
		size, contentType, ok := probeSupplementURL(ctx, client, candidate)
		if !ok {
			continue
		}
		supplement.DownloadURL = candidate
		supplement.LinkType = "direct_download"
		supplement.Size = size
		supplement.ContentType = contentType
		supplement.Kind, supplement.Evidence = onlineSupplementKind(supplement.Label, supplement.DownloadURL)
		supplement.Evidence = appendMissing(supplement.Evidence, "heuristic:nature_static_url")
		return supplement
	}
	return supplement
}

func natureStaticPrefixFromItem(item domain.Item, sourceURL string) (string, string) {
	if prefix, doiSuffix := natureStaticPrefix(item.DOI); prefix != "" {
		return prefix, doiSuffix
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "", ""
	}
	return natureStaticPrefix(path.Base(parsed.Path))
}

func natureStaticPrefix(doiOrSuffix string) (string, string) {
	value := strings.TrimSpace(doiOrSuffix)
	value = strings.TrimPrefix(strings.ToLower(value), "10.1038/")
	m := regexp.MustCompile(`(?i)^(s(\d{5})-(\d{3})-(\d+)-[a-z0-9]+)$`).FindStringSubmatch(value)
	if len(m) != 5 {
		return "", ""
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return "", ""
	}
	articleNumber := strings.TrimLeft(m[4], "0")
	if articleNumber == "" {
		articleNumber = "0"
	}
	return fmt.Sprintf("%s_%d_%s", m[2], 2000+year, articleNumber), m[1]
}

func natureCandidateExtensions(supplement Supplement) []string {
	haystack := strings.ToLower(supplement.Label + " " + strings.Join(supplement.Evidence, " "))
	switch {
	case strings.Contains(haystack, "reporting") || strings.Contains(haystack, "summary"):
		return []string{"pdf", "docx", "xlsx", "zip"}
	case strings.Contains(haystack, "source data") || strings.Contains(haystack, "table"):
		return []string{"xlsx", "zip", "pdf", "docx"}
	default:
		return []string{"pdf", "xlsx", "docx", "zip"}
	}
}

func probeSupplementURL(ctx context.Context, client *http.Client, rawURL string) (int64, string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, "", false
	}
	setSupplementHeaders(req, "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", false
	}
	if resp.ContentLength <= 0 && strings.TrimSpace(resp.Header.Get("Content-Type")) == "" {
		return 0, "", false
	}
	return resp.ContentLength, resp.Header.Get("Content-Type"), true
}

func appendMissing(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, addition := range additions {
		if addition == "" || seen[addition] {
			continue
		}
		values = append(values, addition)
		seen[addition] = true
	}
	return values
}

func cleanSupplementLabel(label string) string {
	label = strings.Join(strings.Fields(label), " ")
	label = regexp.MustCompile(`(?i)\s*\(download\s+[A-Z0-9]+\s*\)\s*$`).ReplaceAllString(label, "")
	return strings.TrimSpace(label)
}

func hasDirectSupplementLink(supplements []Supplement) bool {
	for _, supplement := range supplements {
		if supplement.LinkType == "direct_download" || supplement.LinkType == "api_content" {
			return true
		}
	}
	return false
}

func onlineSupplementKind(label string, downloadURL string) (string, []string) {
	haystack := strings.ToLower(label + " " + downloadURL)
	evidence := []string{}
	kind := ""
	if supplementSourceDataText.MatchString(haystack) {
		kind = "source_data"
		evidence = append(evidence, "text:source_data")
	}
	if supplementReportingSummary.MatchString(haystack) {
		kind = maxKind(kind, "reporting_summary")
		evidence = append(evidence, "text:reporting_summary")
	}
	if supplementDatasetText.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		evidence = append(evidence, "text:supplementary")
	}
	if ext := strings.ToLower(path.Ext(downloadURLPath(downloadURL))); ext != "" {
		evidence = append(evidence, "extension:"+ext)
		if supplementDataExtensions[ext] && kind == "" {
			kind = "data_file"
		}
	}
	if kind == "" {
		kind = "supplementary_dataset"
	}
	if len(evidence) == 0 {
		evidence = append(evidence, "provider:file")
	}
	return kind, evidence
}

func natureSourceURL(item domain.Item) string {
	rawURL := strings.TrimSpace(item.URL)
	if rawURL != "" {
		lower := strings.ToLower(rawURL)
		if strings.Contains(lower, "nature.com/articles/") ||
			strings.Contains(lower, "biomedcentral.com/articles/") ||
			strings.Contains(lower, "springer.com/article/") {
			return rawURL
		}
	}
	doi := strings.TrimSpace(item.DOI)
	if m := natureDOIPattern.FindStringSubmatch(doi); len(m) == 2 {
		return "https://www.nature.com/articles/" + m[1]
	}
	if bmcSpringerDOIPrefix.MatchString(doi) {
		return "https://doi.org/" + doi
	}
	return ""
}

func extractSupplementLinks(baseURL string, body string) []supplementHTMLLink {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	var out []supplementHTMLLink
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attrValue(n, "href")
			text := strings.Join(strings.Fields(nodeText(n)), " ")
			absolute := absolutizeURL(baseURL, href)
			if isOnlineSupplementLink(text, absolute) {
				out = append(out, supplementHTMLLink{text: text, href: absolute})
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

type supplementHTMLLink struct {
	text string
	href string
}

func resolveNatureLandingLinks(baseURL string, links []supplementHTMLLink) []supplementHTMLLink {
	byFragment := map[string]supplementHTMLLink{}
	for _, link := range links {
		if !strings.Contains(strings.ToLower(link.href), "static-content.springer.com") {
			continue
		}
		if id := moesmID(link.href); id != "" {
			byFragment[id] = link
		}
	}
	if len(byFragment) == 0 {
		return links
	}
	out := make([]supplementHTMLLink, 0, len(links))
	for _, link := range links {
		if isSamePageFragment(baseURL, link.href) {
			parsed, err := url.Parse(link.href)
			if err == nil {
				if resolved, ok := byFragment[parsed.Fragment]; ok {
					if strings.TrimSpace(link.text) != "" {
						resolved.text = link.text
					}
					out = append(out, resolved)
					continue
				}
			}
		}
		out = append(out, link)
	}
	return out
}

func moesmID(rawURL string) string {
	upper := strings.ToUpper(rawURL)
	idx := strings.LastIndex(upper, "MOESM")
	if idx < 0 {
		return ""
	}
	rest := upper[idx:]
	var builder strings.Builder
	for _, r := range rest {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		break
	}
	value := builder.String()
	if value == "MOESM" {
		return ""
	}
	return value
}

func isOnlineSupplementLink(text string, href string) bool {
	haystack := strings.ToLower(text + " " + href)
	if strings.TrimSpace(href) == "" || strings.HasPrefix(href, "#") {
		return false
	}
	if isNatureArticlePDFLink(text, href) {
		return false
	}
	return strings.Contains(haystack, "supplementary") ||
		strings.Contains(haystack, "source data") ||
		strings.Contains(haystack, "reporting summary") ||
		strings.Contains(haystack, "static-content.springer.com") ||
		supplementDataExtensions[strings.ToLower(path.Ext(downloadURLPath(href)))]
}

func isNatureArticlePDFLink(text string, href string) bool {
	parsed, err := url.Parse(href)
	if err != nil {
		return false
	}
	lowerPath := strings.ToLower(parsed.Path)
	lowerText := strings.ToLower(strings.Join(strings.Fields(text), " "))
	return strings.Contains(lowerPath, "/articles/") &&
		strings.HasSuffix(lowerPath, ".pdf") &&
		(lowerText == "download pdf" || lowerText == "pdf" || strings.Contains(lowerText, "download pdf"))
}

func isSamePageFragment(baseURL string, candidateURL string) bool {
	base, err1 := url.Parse(baseURL)
	candidate, err2 := url.Parse(candidateURL)
	if err1 != nil || err2 != nil {
		return false
	}
	base.Fragment = ""
	base.RawQuery = ""
	fragment := candidate.Fragment
	candidate.Fragment = ""
	candidate.RawQuery = ""
	return fragment != "" && base.String() == candidate.String()
}

func getJSON(ctx context.Context, client *http.Client, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	setSupplementHeaders(req, "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 10<<20))
	return decoder.Decode(out)
}

func getText(ctx context.Context, client *http.Client, rawURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	setSupplementHeaders(req, "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", "", err
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return string(body), finalURL, nil
}

func setSupplementHeaders(req *http.Request, accept string) {
	req.Header.Set("User-Agent", supplementUserAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func firstRegexGroup(pattern *regexp.Regexp, values ...string) string {
	for _, value := range values {
		if m := pattern.FindStringSubmatch(strings.TrimSpace(value)); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

func attrValue(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteString(" ")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return builder.String()
}

func absolutizeURL(baseURL string, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

func downloadURLPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Path
}

func parseInt64(value string) int64 {
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}
