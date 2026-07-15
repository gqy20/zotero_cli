package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/references"
)

var (
	importPMIDPattern = regexp.MustCompile(`(?i)^(?:PMID\s*:\s*)?(\d{1,12})$`)
	importDOIPattern  = regexp.MustCompile(`(?i)^(?:DOI\s*:\s*)?(10\.\d{4,9}/\S+)$`)
)

func isMetadataImportSource(source string) bool {
	source = strings.TrimSpace(source)
	if importPMIDPattern.MatchString(source) || importDOIPattern.MatchString(source) {
		return true
	}
	u, err := url.Parse(source)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func (s ItemImportService) importMetadata(ctx context.Context, cfg config.Config, req ItemImportRequest, source string) (Result, error) {
	ids, err := metadataImportIdentifiers(source, req.FromData)
	if err != nil {
		return Result{}, err
	}
	article, err := s.ResolveArticle(ctx, cfg, ids)
	if err != nil {
		return Result{}, fmt.Errorf("resolve import metadata: %w", err)
	}

	data := ItemImportResult{
		Status: "ready", SourceType: "metadata", Identifier: importIdentifier(article), Mode: cfg.Mode,
		DryRun: req.DryRun, Stages: map[string]string{"metadata": "success"},
	}
	payload := articleItemPayload(article)
	if strings.TrimSpace(req.Collection) != "" {
		collection, resolveErr := s.resolveMetadataCollection(ctx, cfg, req.Collection)
		if resolveErr != nil {
			return Result{}, resolveErr
		}
		data.CollectionKey, data.CollectionName, data.CollectionPath = collection.Key, collection.Name, collection.Path
		payload["collections"] = []string{collection.Key}
	}

	candidates, err := s.findMetadataDuplicates(ctx, cfg, article)
	if err != nil {
		return Result{}, fmt.Errorf("check import duplicates: %w", err)
	}
	data.DuplicateCandidates = candidates
	data.Stages["duplicate_check"] = "success"
	if len(candidates) > 1 {
		return Result{Data: data}, fmt.Errorf("import is ambiguous: %d existing items match; no item was created", len(candidates))
	}
	if len(candidates) == 1 {
		data.Status = "existing"
		data.ItemKey = candidates[0].Key
		return Result{Data: data, Meta: map[string]any{"write_source": "none"}, Text: fmt.Sprintf("already in library: %s (%s)", candidates[0].Title, candidates[0].Key)}, nil
	}

	data.PlannedActions = []string{"create Zotero item from PubMed metadata"}
	if data.CollectionKey != "" {
		data.PlannedActions = append(data.PlannedActions, "assign item to collection "+data.CollectionKey)
	}
	if req.DryRun {
		return Result{Data: data, Meta: map[string]any{"dry_run": true, "payload": payload}, Text: fmt.Sprintf("dry run: ready to create %s", article.Title)}, nil
	}
	if !cfg.AllowWrite {
		return Result{}, fmt.Errorf("writes are disabled; set ZOT_ALLOW_WRITE=1")
	}
	writer, err := s.NewWriteClient(cfg)
	if err != nil {
		return Result{}, fmt.Errorf("metadata import requires web API write access: %w", err)
	}
	version, err := writer.GetLibraryVersion(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("resolve library version: %w", err)
	}
	created, err := writer.CreateItem(ctx, payload, version)
	if err != nil {
		return Result{}, err
	}
	data.Accepted = true
	data.Status = "success"
	data.ItemKey = created.Key
	data.CollectionAssigned = data.CollectionKey != ""
	data.Stages["create"] = "success"
	return Result{Data: data, Meta: map[string]any{"write_source": "zotero_web_api", "last_modified_version": created.LastModifiedVersion}, Text: fmt.Sprintf("created %s (%s)", article.Title, created.Key)}, nil
}

func metadataImportIdentifiers(source string, fromData []byte) (references.Identifiers, error) {
	if len(fromData) > 0 {
		return identifiersFromImportJSON(fromData)
	}
	source = strings.TrimSpace(source)
	if match := importPMIDPattern.FindStringSubmatch(source); len(match) == 2 {
		return references.Identifiers{PMID: match[1]}, nil
	}
	if match := importDOIPattern.FindStringSubmatch(source); len(match) == 2 {
		return references.Identifiers{DOI: trimDOIPunctuation(match[1])}, nil
	}
	u, err := url.Parse(source)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return references.Identifiers{}, fmt.Errorf("import source must be a PDF path, DOI, PMID, supported URL, or --from JSON")
	}
	host := strings.ToLower(u.Hostname())
	if host == "doi.org" || host == "dx.doi.org" {
		doi := strings.TrimPrefix(u.EscapedPath(), "/")
		if decoded, decodeErr := url.PathUnescape(doi); decodeErr == nil {
			doi = decoded
		}
		if importDOIPattern.MatchString(doi) {
			return references.Identifiers{DOI: trimDOIPunctuation(doi)}, nil
		}
	}
	if strings.HasSuffix(host, "pubmed.ncbi.nlm.nih.gov") {
		pmid := strings.Trim(strings.TrimSpace(u.Path), "/")
		if importPMIDPattern.MatchString(pmid) {
			return references.Identifiers{PMID: pmid}, nil
		}
	}
	return references.Identifiers{}, fmt.Errorf("unsupported import URL %q; use a doi.org or PubMed URL", source)
}

func identifiersFromImportJSON(data []byte) (references.Identifiers, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return references.Identifiers{}, fmt.Errorf("decode import JSON: %w", err)
	}
	var found []references.Identifiers
	collectImportIdentifiers(value, &found)
	unique := make([]references.Identifiers, 0, len(found))
	seen := map[string]bool{}
	for _, ids := range found {
		key := strings.ToLower(ids.PMID + "\x00" + ids.DOI)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ids)
		}
	}
	if len(unique) == 0 {
		return references.Identifiers{}, fmt.Errorf("import JSON contains no DOI or PMID")
	}
	if len(unique) > 1 {
		return references.Identifiers{}, fmt.Errorf("import JSON contains %d candidates; import one DOI or PMID at a time", len(unique))
	}
	return unique[0], nil
}

func collectImportIdentifiers(value any, found *[]references.Identifiers) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			collectImportIdentifiers(child, found)
		}
	case map[string]any:
		var ids references.Identifiers
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "doi":
				ids.DOI, _ = child.(string)
			case "pmid":
				ids.PMID, _ = child.(string)
			}
		}
		ids.DOI = trimDOIPunctuation(ids.DOI)
		ids.PMID = strings.TrimSpace(ids.PMID)
		if ids.DOI != "" || ids.PMID != "" {
			*found = append(*found, ids)
			return
		}
		for _, child := range typed {
			collectImportIdentifiers(child, found)
		}
	}
}

func (s ItemImportService) findMetadataDuplicates(ctx context.Context, cfg config.Config, article references.Article) ([]DuplicateCandidate, error) {
	reader, err := s.NewReader(cfg)
	if err != nil {
		return nil, err
	}
	queries := []string{article.DOI, article.PMID, article.Title}
	itemsByKey := map[string]domain.Item{}
	seenQueries := map[string]bool{}
	for _, query := range queries {
		query = strings.TrimSpace(query)
		normalized := strings.ToLower(query)
		if query == "" || seenQueries[normalized] {
			continue
		}
		seenQueries[normalized] = true
		items, findErr := reader.FindItems(ctx, backend.FindOptions{Query: query, In: "metadata", All: true, Full: true, Limit: 100})
		if findErr != nil {
			return nil, findErr
		}
		for _, item := range items {
			itemsByKey[item.Key] = item
		}
	}
	var matches []DuplicateCandidate
	for _, item := range itemsByKey {
		match := metadataDuplicateMatch(item, article)
		if match != "" {
			matches = append(matches, DuplicateCandidate{Key: item.Key, Title: item.Title, Match: match})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Key < matches[j].Key })
	return matches, nil
}

func metadataDuplicateMatch(item domain.Item, article references.Article) string {
	if article.DOI != "" && normalizeImportDOI(item.DOI) == normalizeImportDOI(article.DOI) {
		return "doi"
	}
	if article.PMID != "" {
		pmidURL := strings.Contains(strings.ToLower(item.URL), "pubmed.ncbi.nlm.nih.gov/"+article.PMID)
		pmidExtra := regexp.MustCompile(`(?im)^\s*PMID\s*:\s*` + regexp.QuoteMeta(article.PMID) + `\s*$`).MatchString(item.Extra)
		if pmidURL || pmidExtra {
			return "pmid"
		}
	}
	if strings.TrimSpace(article.Title) == "" || importYear(article.Year) == "" || normalizeImportText(item.Title) != normalizeImportText(article.Title) || importYear(item.Date) != importYear(article.Year) {
		return ""
	}
	if len(article.Authors) > 0 && len(item.Creators) > 0 {
		family := article.Authors[0].Family
		if family == "" {
			family = article.Authors[0].Name
		}
		if family != "" && !strings.Contains(normalizeImportText(item.Creators[0].Name), normalizeImportText(family)) {
			return ""
		}
	}
	return "title_year_author"
}

func articleItemPayload(article references.Article) map[string]any {
	creators := make([]map[string]any, 0, len(article.Authors))
	for _, author := range article.Authors {
		creator := map[string]any{"creatorType": "author"}
		if author.Name != "" {
			creator["name"] = author.Name
		} else {
			creator["firstName"], creator["lastName"] = author.Given, author.Family
		}
		creators = append(creators, creator)
	}
	extra := []string{}
	if article.PMID != "" {
		extra = append(extra, "PMID: "+article.PMID)
	}
	if article.PMCID != "" {
		extra = append(extra, "PMCID: "+article.PMCID)
	}
	payload := map[string]any{
		"itemType": "journalArticle", "title": article.Title, "creators": creators,
		"publicationTitle": article.Container, "date": article.Year, "volume": article.Volume,
		"issue": article.Issue, "pages": article.Pages, "DOI": article.DOI,
		"extra": strings.Join(extra, "\n"),
	}
	if article.PMID != "" {
		payload["url"] = "https://pubmed.ncbi.nlm.nih.gov/" + article.PMID + "/"
	}
	return payload
}

func (s ItemImportService) resolveMetadataCollection(ctx context.Context, cfg config.Config, selector string) (backend.CollectionTarget, error) {
	if s.NewResolver != nil {
		if resolver, err := s.NewResolver(cfg); err == nil {
			if target, targetErr := resolver.CollectionTarget(ctx, selector); targetErr == nil {
				return target, nil
			}
		}
	}
	selector = strings.TrimSpace(selector)
	if matched, _ := regexp.MatchString(`^[A-Z0-9]{8}$`, selector); matched {
		return backend.CollectionTarget{Key: selector, Name: selector}, nil
	}
	return backend.CollectionTarget{}, fmt.Errorf("collection %q could not be resolved; in web/remote mode pass an 8-character collection key", selector)
}

func importIdentifier(article references.Article) string {
	if article.DOI != "" {
		return "DOI:" + article.DOI
	}
	return "PMID:" + article.PMID
}

func normalizeImportDOI(value string) string {
	return strings.ToLower(trimDOIPunctuation(strings.TrimPrefix(strings.TrimSpace(value), "https://doi.org/")))
}

func trimDOIPunctuation(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), ".,;)]}")
}

func normalizeImportText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func importYear(value string) string {
	match := regexp.MustCompile(`\d{4}`).FindString(value)
	return match
}
