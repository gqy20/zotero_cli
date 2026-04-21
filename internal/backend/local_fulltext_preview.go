package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"zotero_cli/internal/domain"
)

const fullTextPreviewLimit = 280

var hyphenatedLineBreakPattern = regexp.MustCompile(`([\p{L}\p{N}])-\s*\n\s*([\p{L}\p{N}])`)
var pageMarkerPattern = regexp.MustCompile(`^\|?\s*\d+\s+of\s+\d+(?:\s*\|\s*.*)?$`)
var journalHeaderPattern = regexp.MustCompile(`^[A-Z][A-Za-z .&-]+\. \d{4};.*\|\s*\d+\s+of\s+\d+.*$`)
var doiLinePattern = regexp.MustCompile(`^DOI:\s*10\.`)
var sectionHeadingPattern = regexp.MustCompile(`^\d+\s+\|\s+[A-Z][A-Z\s-]+$`)
var allCapsLinePattern = regexp.MustCompile(`^[A-Z][A-Z\s-]{2,}$`)
var brokenWordNextPattern = regexp.MustCompile(`^([a-z]{2,})\b`)
var punctuationSpacingPattern = regexp.MustCompile(`([,;:])([A-Za-z])`)
var closeParenSpacingPattern = regexp.MustCompile(`(\))([A-Za-z])`)
var etAlSpacingPattern = regexp.MustCompile(`\b([A-Z][a-z]+)et al\.`)

var figureCaptionPattern = regexp.MustCompile(`(?i)^(FIG\.?|Figure\s+\d|Table\s+\d)`)
var runningHeaderPattern = regexp.MustCompile(`(?i)^.{5,80}\set\s?al\.\s*doi:|^.{5,80}\s\d{4}\s*;\s*\d+.*\|\s*\d+\s+of\s+\d+`)
var standalonePageNumberPattern = regexp.MustCompile(`^\d+$`)

var fullTextPhraseFixes = map[string]string{
	"Also,these":            "Also, these",
	"Also,in":               "Also, in",
	"estab lishment":        "establishment",
	"appli cation":          "application",
	"challeng ing":          "challenging",
	"isolat ing":            "isolating",
	"isola tion":            "isolation",
	"in creased":            "increased",
	"evolu tion":            "evolution",
	"hybridiza tion":        "hybridization",
	"hybrid specia tion":    "hybrid speciation",
	"inter media":           "intermedia",
	"homop loid":            "homoploid",
	"analy sis":             "analysis",
	"repro ductive":         "reproductive",
	"complementa tion":      "complementation",
	"specia tion":           "speciation",
	"diver gence":           "divergence",
	"selec tion":            "selection",
	"fertil ity":            "fertility",
	"pheno types":           "phenotypes",
	"quanti ta tive":        "quantitative",
	"func tion ally":        "functionally",
	"heterol ogous":         "heterologous",
	"demon strates":         "demonstrates",
	"there fore":            "therefore",
	"thusmay":               "thus may",
	"deepin time":           "deep in time",
	"timeof hybrid":         "time of hybrid",
	"thatthe":               "that the",
	"thatthusmay":           "that thus may",
	"thatthus may":          "that thus may",
	"thathybridization":     "that hybridization",
	"thathomoploid":         "that homoploid",
	"willcontribute":        "will contribute",
	"ourclassification":     "our classification",
	"does notseem":          "does not seem",
	"takeadvantage":         "take advantage",
	"highfalse-positive":    "high false-positive",
	"dataand":               "data and",
	"soiliron":              "soil iron",
	"newlineage":            "new lineage",
	"body sizeof":           "body size of",
	"BDM)incompatibilities": "BDM) incompatibilities",
	"by defini tion":        "by definition",
	"in dels":               "indels",
	"paren tal":             "parental",
	"can didate":            "candidate",
	"criteria maybe":        "criteria may be",
	"straight forward":      "straightforward",
	"Timewhich":             "Time which",
}

var shortWordJoinStoplist = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {},
	"by": {}, "can": {}, "for": {}, "from": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"most": {}, "of": {}, "on": {}, "or": {}, "some": {}, "such": {}, "than": {}, "the": {},
	"their": {}, "this": {}, "to": {}, "was": {}, "were": {}, "with": {},
}

func (r *LocalReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	doc, ok, err := r.loadFullTextDocument(ctx, item, "")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return normalizeFullTextPreview(doc.Text), nil
}

func (r *LocalReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	doc, ok, err := r.loadFullTextDocument(ctx, item, query)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return buildFullTextSnippet(doc.Text, query), nil
}

func (r *LocalReader) loadFullTextDocument(_ context.Context, item domain.Item, query string) (FullTextDocument, bool, error) {
	cache := newFullTextCache(r.FullTextCacheDir)

	if item.SnippetAttachmentKey != "" {
		for _, attachment := range item.Attachments {
			if attachment.Key == item.SnippetAttachmentKey {
				doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
				if err != nil {
					return FullTextDocument{}, false, err
				}
				if ok && doc.Text != "" {
					r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
						FullTextSource:        doc.Meta.Extractor,
						FullTextAttachmentKey: doc.Meta.AttachmentKey,
						FullTextCacheHit:      doc.CacheHit,
					})
					return doc, true, nil
				}
				break
			}
		}
	}

	bestDoc := FullTextDocument{}
	bestScore := -1
	for _, attachment := range item.Attachments {
		doc, ok, err := r.loadFullTextDocumentForAttachment(item, attachment, cache)
		if err != nil {
			return FullTextDocument{}, false, err
		}
		if !ok || doc.Text == "" {
			continue
		}
		score := fullTextAttachmentMatchScore(attachment, doc.Text, query)
		if query == "" {
			score = 0
		}
		if bestScore < score || bestScore < 0 {
			bestDoc = doc
			bestScore = score
			if query == "" {
				break
			}
		}
	}
	if bestScore < 0 {
		return FullTextDocument{}, false, nil
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, ReadMetadata{
		FullTextSource:        bestDoc.Meta.Extractor,
		FullTextAttachmentKey: bestDoc.Meta.AttachmentKey,
		FullTextCacheHit:      bestDoc.CacheHit,
	})
	return bestDoc, true, nil
}

func (r *HybridReader) FullTextPreview(ctx context.Context, item domain.Item) (string, error) {
	previewer, ok := r.local.(interface {
		FullTextPreview(context.Context, domain.Item) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("show --snippet requires local or hybrid mode with local data")
	}
	preview, err := previewer.FullTextPreview(ctx, item)
	if err != nil {
		return "", err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return preview, nil
}

func (r *HybridReader) FullTextSnippet(ctx context.Context, item domain.Item, query string) (string, error) {
	snippeter, ok := r.local.(interface {
		FullTextSnippet(context.Context, domain.Item, string) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("find --snippet requires local or hybrid mode with local data")
	}
	snippet, err := snippeter.FullTextSnippet(ctx, item, query)
	if err != nil {
		return "", err
	}
	r.lastReadMetadata = mergeReadMetadata(r.lastReadMetadata, consumeReadMetadata(r.local))
	return snippet, nil
}

func (r *LocalReader) buildFullTextDocument(item domain.Item, attachment domain.Attachment) (FullTextDocument, bool, error) {
	// Priority 1: PyMuPDF (best quality with structured blocks, geometry-based header/footer removal)
	doc, ok, err := r.extractFullTextWithPyMuPDF(context.Background(), attachment)
	if err != nil || !ok {
		// Priority 2: Zotero .zotero-ft-cache (fast but lower quality, especially for CJK/scan PDFs)
		text, readErr := r.readAttachmentFullTextText(attachment.Key)
		if readErr == nil && text != "" {
			sourcePath, info, srcOk := fullTextAttachmentSourceInfo(attachment)
			if srcOk {
				return FullTextDocument{
					Text: normalizeFullTextText(text),
					Meta: fullTextCacheMeta{
						AttachmentKey:   attachment.Key,
						ParentItemKey:   item.Key,
						ResolvedPath:    sourcePath,
						ContentType:     attachment.ContentType,
						Title:           item.Title,
						Creators:        joinFullTextCreators(item.Creators),
						Tags:            strings.Join(item.Tags, " "),
						AttachmentTitle: attachment.Title,
						AttachmentName:  firstNonEmptyString(attachment.Filename, attachment.Title),
						AttachmentPath:  firstNonEmptyString(attachment.ResolvedPath, attachment.ZoteroPath),
						Extractor:       "zotero_ft_cache",
						SourceMtimeUnix: info.ModTime().Unix(),
						SourceSize:      info.Size(),
						ExtractedAt:     time.Now().UTC().Format(time.RFC3339),
					},
				}, true, nil
			}
		}
		// Priority 3: pdfium fallback
		doc, ok, err = extractFullTextWithPDFiumFunc(context.Background(), r, attachment)
	}
	if err != nil {
		return FullTextDocument{}, false, err
	}
	if !ok {
		return FullTextDocument{}, false, nil
	}
	doc.Meta.ParentItemKey = item.Key
	doc.Meta.Title = item.Title
	doc.Meta.Creators = joinFullTextCreators(item.Creators)
	doc.Meta.Tags = strings.Join(item.Tags, " ")
	doc.Meta.AttachmentTitle = attachment.Title
	doc.Meta.AttachmentName = firstNonEmptyString(attachment.Filename, attachment.Title)
	doc.Meta.AttachmentPath = firstNonEmptyString(attachment.ResolvedPath, attachment.ZoteroPath)
	doc.Meta.ExtractedAt = time.Now().UTC().Format(time.RFC3339)
	return doc, true, nil
}

func (r *LocalReader) ExtractAttachmentFullText(ctx context.Context, item domain.Item, att domain.Attachment) (FullTextDocument, bool, error) {
	return r.loadFullTextDocumentForAttachment(item, att, newFullTextCache(r.FullTextCacheDir))
}

func (r *LocalReader) ExtractAttachmentFullTextOnly(ctx context.Context, item domain.Item, att domain.Attachment) (FullTextDocument, bool, error) {
	doc, ok, err := newFullTextCache(r.FullTextCacheDir).Load(att)
	if err != nil {
		return FullTextDocument{}, false, err
	}
	if ok && doc.Text != "" {
		doc.Text = normalizeFullTextText(doc.Text)
		doc.Meta.ParentItemKey = firstNonEmptyString(doc.Meta.ParentItemKey, item.Key)
		doc.CacheHit = true
		return doc, true, nil
	}
	doc, ok, err = r.buildFullTextDocument(item, att)
	if err != nil {
		return FullTextDocument{}, false, err
	}
	return doc, ok, nil
}

func (r *LocalReader) SaveFullText(doc FullTextDocument) error {
	return newFullTextCache(r.FullTextCacheDir).Save(doc)
}

func (r *LocalReader) SaveFullTextBatch(docs []FullTextDocument) error {
	return newFullTextCache(r.FullTextCacheDir).SaveBatch(docs)
}

func (r *LocalReader) loadFullTextDocumentForAttachment(item domain.Item, attachment domain.Attachment, cache fullTextCache) (FullTextDocument, bool, error) {
	doc, ok, err := cache.Load(attachment)
	if err != nil {
		return FullTextDocument{}, false, err
	}
	if ok && doc.Text != "" {
		doc.Text = normalizeFullTextText(doc.Text)
		doc.Meta.ParentItemKey = firstNonEmptyString(doc.Meta.ParentItemKey, item.Key)
		doc.CacheHit = true
		return doc, true, nil
	}
	doc, ok, err = r.buildFullTextDocument(item, attachment)
	if err != nil {
		return FullTextDocument{}, false, err
	}
	if !ok || doc.Text == "" {
		return FullTextDocument{}, false, nil
	}
	if err := cache.Save(doc); err != nil {
		return FullTextDocument{}, false, err
	}
	return doc, true, nil
}

func (r *LocalReader) readAttachmentFullTextText(attachmentKey string) (string, error) {
	if strings.TrimSpace(attachmentKey) == "" {
		return "", nil
	}
	cachePath := filepath.Join(r.StorageDir, attachmentKey, ".zotero-ft-cache")
	content, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}

func normalizeFullTextText(value string) string {
	replacer := strings.NewReplacer(
		"\u00ad", "",
		"\u00a0", " ",
		"\u202f", " ",
		"\t", " ",
		"\f", "\n",
		"\r\n", "\n",
		"\r", "\n",
	)
	cleaned := replacer.Replace(value)
	cleaned = normalizeHyphenatedLineBreaks(cleaned)

	lines := strings.Split(cleaned, "\n")
	normalizedLines := make([]string, 0, len(lines))
	blankPending := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if shouldDropFullTextLine(line) {
			blankPending = len(normalizedLines) > 0
			continue
		}
		if line == "" {
			if len(normalizedLines) > 0 && !blankPending {
				normalizedLines = append(normalizedLines, "")
				blankPending = true
			}
			continue
		}
		normalizedLines = append(normalizedLines, line)
		blankPending = false
	}

	mergedLines := make([]string, 0, len(normalizedLines))
	for _, line := range normalizedLines {
		if line == "" {
			if len(mergedLines) > 0 && mergedLines[len(mergedLines)-1] != "" {
				mergedLines = append(mergedLines, "")
			}
			continue
		}
		if len(mergedLines) > 0 && mergedLines[len(mergedLines)-1] != "" && shouldMergeFullTextLines(mergedLines[len(mergedLines)-1], line) {
			mergedLines[len(mergedLines)-1] = mergeFullTextLines(mergedLines[len(mergedLines)-1], line)
			continue
		}
		mergedLines = append(mergedLines, line)
	}

	return strings.TrimSpace(postprocessFullTextText(strings.Join(mergedLines, "\n")))
}

func normalizeHyphenatedLineBreaks(value string) string {
	for {
		normalized := hyphenatedLineBreakPattern.ReplaceAllString(value, "$1$2")
		if normalized == value {
			return normalized
		}
		value = normalized
	}
}

func detectPDFiumCorruption(text string) bool {
	if len(text) < 200 {
		return false
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return false
	}
	nfdCount := 0
	for _, r := range runes {
		if r >= '\u0300' && r <= '\u036f' {
			nfdCount++
		}
	}
	if float64(nfdCount)/float64(len(runes)) > 0.005 {
		return true
	}
	asciiCount := 0
	for _, r := range runes {
		if r < 127 {
			asciiCount++
		}
	}
	if float64(asciiCount)/float64(len(runes)) > 0.98 {
		return true
	}
	return false
}

func shouldDropFullTextLine(line string) bool {
	if line == "" {
		return false
	}
	lower := strings.ToLower(line)
	switch {
	case pageMarkerPattern.MatchString(line):
		return true
	case journalHeaderPattern.MatchString(line):
		return true
	case strings.HasPrefix(lower, "wileyonlinelibrary.com/"):
		return true
	case strings.Contains(lower, "downloaded from https://"):
		return true
	case strings.Contains(lower, "see the terms and conditions"):
		return true
	case strings.Contains(lower, "creative commons attribution-noncommercial license"):
		return true
	case strings.HasPrefix(lower, "how to cite this article:"):
		return true
	case strings.HasPrefix(lower, "received: "):
		return true
	case line == "LONG and RIESEBERG":
		return true
	case doiLinePattern.MatchString(line):
		return true
	case figureCaptionPattern.MatchString(line):
		return true
	case runningHeaderPattern.MatchString(line):
		return true
	case standalonePageNumberPattern.MatchString(line):
		return len(line) <= 3
	}
	return false
}

func shouldMergeFullTextLines(prev string, curr string) bool {
	if prev == "" || curr == "" {
		return false
	}
	if isStandaloneFullTextLine(prev) || isStandaloneFullTextLine(curr) {
		return false
	}
	return true
}

func isStandaloneFullTextLine(line string) bool {
	if line == "" {
		return false
	}
	switch {
	case sectionHeadingPattern.MatchString(line):
		return true
	case allCapsLinePattern.MatchString(line) && len([]rune(line)) <= 80:
		return true
	case strings.HasSuffix(line, ":"):
		return true
	}
	switch line {
	case "Abstract", "COMMENT", "KEYWORDS", "Correspondence", "Handling Editor":
		return true
	}
	return false
}

func mergeFullTextLines(prev string, curr string) string {
	if shouldConcatenateBrokenWord(prev, curr) {
		return prev + curr
	}
	return prev + " " + curr
}

func shouldConcatenateBrokenWord(prev string, curr string) bool {
	prevFields := strings.Fields(prev)
	if len(prevFields) == 0 {
		return false
	}
	prevWord := prevFields[len(prevFields)-1]
	prevWord = strings.TrimFunc(prevWord, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	prevWord = strings.ToLower(prevWord)
	currMatch := brokenWordNextPattern.FindStringSubmatch(curr)
	if len(currMatch) < 2 {
		return false
	}
	if len(prevWord) < 2 || len(prevWord) > 4 {
		return false
	}
	for _, r := range prevWord {
		if !unicode.IsLower(r) {
			return false
		}
	}
	if _, blocked := shortWordJoinStoplist[prevWord]; blocked {
		return false
	}
	return true
}

func postprocessFullTextText(value string) string {
	value = punctuationSpacingPattern.ReplaceAllString(value, "$1 $2")
	value = closeParenSpacingPattern.ReplaceAllString(value, "$1 $2")
	value = etAlSpacingPattern.ReplaceAllString(value, "$1 et al.")
	for old, newValue := range fullTextPhraseFixes {
		value = strings.ReplaceAll(value, old, newValue)
	}
	value = strings.ReplaceAll(value, "thatthusmay", "that thus may")
	value = strings.ReplaceAll(value, "thatthus may", "that thus may")
	return value
}

func normalizeFullTextPreview(value string) string {
	normalized := strings.Join(strings.Fields(value), " ")
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) <= fullTextPreviewLimit {
		return normalized
	}
	return string(runes[:fullTextPreviewLimit]) + "..."
}

func buildFullTextSnippet(text string, query string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		return ""
	}
	tokens := fullTextQueryTokens(query)
	if len(tokens) == 0 {
		return normalizeFullTextPreview(normalized)
	}

	lowerRunes := []rune(strings.ToLower(normalized))
	textRunes := []rune(normalized)
	bestIdx := -1
	bestLen := 0
	for _, token := range tokens {
		tokenRunes := []rune(strings.ToLower(token))
		if len(tokenRunes) == 0 {
			continue
		}
		idx := indexRuneSlice(lowerRunes, tokenRunes)
		if idx < 0 {
			continue
		}
		if bestIdx == -1 || idx < bestIdx || (idx == bestIdx && len(tokenRunes) > bestLen) {
			bestIdx = idx
			bestLen = len(tokenRunes)
		}
	}
	if bestIdx < 0 {
		return normalizeFullTextPreview(normalized)
	}
	const contextRadius = 60
	start := bestIdx - contextRadius
	if start < 0 {
		start = 0
	}
	end := bestIdx + bestLen + contextRadius
	if end > len(textRunes) {
		end = len(textRunes)
	}
	snippet := string(textRunes[start:end])
	snippet = strings.TrimSpace(snippet)
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(textRunes) {
		snippet += "..."
	}
	return snippet
}

func fullTextAttachmentMatchScore(attachment domain.Attachment, text string, query string) int {
	if strings.TrimSpace(query) == "" {
		return 0
	}
	score := 0
	for _, token := range fullTextQueryTokens(query) {
		lowerToken := strings.ToLower(token)
		if strings.Contains(strings.ToLower(text), lowerToken) {
			score += 10
		}
		if strings.Contains(strings.ToLower(attachment.Title), lowerToken) {
			score += 3
		}
		if strings.Contains(strings.ToLower(attachment.Filename), lowerToken) {
			score += 3
		}
		if strings.Contains(strings.ToLower(attachment.ZoteroPath), lowerToken) || strings.Contains(strings.ToLower(attachment.ResolvedPath), lowerToken) {
			score += 2
		}
		if strings.Contains(strings.ToLower(attachment.ContentType), lowerToken) {
			score++
		}
	}
	return score
}

func joinFullTextCreators(creators []domain.Creator) string {
	names := make([]string, 0, len(creators))
	for _, creator := range creators {
		if strings.TrimSpace(creator.Name) != "" {
			names = append(names, creator.Name)
		}
	}
	return strings.Join(names, " ")
}

func fullTextQueryTokens(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	tokens := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func indexRuneSlice(text []rune, token []rune) int {
	if len(token) == 0 || len(text) < len(token) {
		return -1
	}
outer:
	for i := 0; i <= len(text)-len(token); i++ {
		for j := range token {
			if text[i+j] != token[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

func blocksToChunks(blocks []extractBlock) []chunk {
	if len(blocks) == 0 {
		return nil
	}
	filtered := filterBlocksByGeometry(blocks)
	if len(filtered) == 0 {
		return nil
	}

	var chunks []chunk
	var cur chunk
	for i, b := range filtered {
		if i == 0 {
			cur = startChunk(b)
			continue
		}
		prev := filtered[i-1]
		if shouldSplitChunk(prev, b) {
			chunks = append(chunks, cur)
			cur = startChunk(b)
		} else {
			cur = mergeIntoChunk(cur, b)
		}
	}
	if cur.Text != "" {
		chunks = append(chunks, cur)
	}
	return chunks
}

func chunksToPlainText(chunks []chunk) string {
	if len(chunks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func filterBlocksByGeometry(blocks []extractBlock) []extractBlock {
	if len(blocks) <= 2 {
		return blocks
	}
	result := make([]extractBlock, 0, len(blocks))
	pageSizes := estimatePageSizes(blocks)
	for _, b := range blocks {
		if isHeaderFooterByGeometry(b, pageSizes) {
			continue
		}
		result = append(result, b)
	}
	return result
}

func isHeaderFooterByGeometry(b extractBlock, pageSizes map[int][2]float64) bool {
	size, ok := pageSizes[b.Page]
	if !ok || size[1] == 0 {
		return false
	}
	marginRatio := 0.06
	bottomY := size[1]
	topMargin := bottomY * marginRatio
	bottomMargin := bottomY * (1 - marginRatio)

	if b.BBox[1] < topMargin && b.Size < 9 {
		return true
	}
	if b.BBox[3] > bottomMargin && b.Size < 9 {
		return true
	}
	return false
}

func estimatePageSizes(blocks []extractBlock) map[int][2]float64 {
	sizes := make(map[int][2]float64)
	for _, b := range blocks {
		w := b.BBox[2]
		h := b.BBox[3]
		existing, ok := sizes[b.Page]
		if !ok || w > existing[0] || h > existing[1] {
			sizes[b.Page] = [2]float64{w, h}
		}
	}
	return sizes
}

func startChunk(b extractBlock) chunk {
	return chunk{
		Page:       b.Page,
		BBox:       b.BBox,
		Text:       b.Text,
		BlockCount: 1,
	}
}

func mergeIntoChunk(c chunk, b extractBlock) chunk {
	c.BBox[2] = max(c.BBox[2], b.BBox[2])
	c.BBox[3] = max(c.BBox[3], b.BBox[3])
	c.Text = c.Text + " " + b.Text
	c.BlockCount++
	return c
}

func shouldSplitChunk(prev, curr extractBlock) bool {
	if prev.Page != curr.Page {
		return true
	}
	lineHeight := estimateLineHeight(prev.Size)
	gap := curr.BBox[1] - prev.BBox[3]
	if lineHeight > 0 && gap > lineHeight*1.8 {
		return true
	}
	if prev.Size > 0 && curr.Size > 0 {
		ratio := curr.Size / prev.Size
		if ratio > 1.4 || ratio < 0.7 {
			runes := []rune(curr.Text)
			if len(runes) <= 80 {
				return true
			}
		}
	}
	return false
}

func estimateLineHeight(fontSize float64) float64 {
	if fontSize <= 0 {
		return 14
	}
	return fontSize * 1.4
}
