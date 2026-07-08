package backend

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"zotero_cli/internal/domain"
)

type Supplement struct {
	Provider         string            `json:"provider"`
	ProviderStatus   string            `json:"provider_status,omitempty"`
	Kind             string            `json:"kind"`
	Label            string            `json:"label"`
	ItemKey          string            `json:"item_key"`
	ItemTitle        string            `json:"item_title,omitempty"`
	AttachmentKey    string            `json:"attachment_key"`
	ContentType      string            `json:"content_type,omitempty"`
	Filename         string            `json:"filename,omitempty"`
	ZoteroPath       string            `json:"zotero_path,omitempty"`
	LocalPath        string            `json:"local_path,omitempty"`
	Resolved         bool              `json:"resolved"`
	ResolutionStatus string            `json:"resolution_status"`
	Size             int64             `json:"size,omitempty"`
	Confidence       float64           `json:"confidence"`
	Evidence         []string          `json:"evidence,omitempty"`
	Attachment       domain.Attachment `json:"attachment,omitempty"`
}

var (
	supplementMOESMPattern     = regexp.MustCompile(`(?i)(^|[^a-z0-9])moesm\d+([^a-z0-9]|$)`)
	supplementMMCPattern       = regexp.MustCompile(`(?i)(^|[^a-z0-9])mmc\d+([^a-z0-9]|$)`)
	supplementPGenPattern      = regexp.MustCompile(`(?i)\bpgen\.[a-z0-9.]*\.s\d+\b`)
	supplementScienceTable     = regexp.MustCompile(`(?i)\bscience\.[a-z0-9._-]*tables?`)
	supplementTableRange       = regexp.MustCompile(`(?i)tables?_s\d+(_to_s\d+)?`)
	supplementDatasetText      = regexp.MustCompile(`(?i)(^|[^a-z0-9])supplement(ar)?y[-_ ]+(table|tables|dataset|data|file|files|material|materials|information)([^a-z0-9]|$)`)
	supplementSourceDataText   = regexp.MustCompile(`(?i)(^|[^a-z0-9])source[-_ ]+data([^a-z0-9]|$)`)
	supplementReportingSummary = regexp.MustCompile(`(?i)(^|[^a-z0-9])(reporting[-_ ]+summary|life[-_ ]+sciences[-_ ]+reporting[-_ ]+summary)([^a-z0-9]|$)`)
)

var supplementDataExtensions = map[string]bool{
	".bam":   true,
	".bed":   true,
	".csv":   true,
	".doc":   true,
	".docx":  true,
	".fa":    true,
	".fasta": true,
	".fastq": true,
	".gz":    true,
	".h5":    true,
	".hdf5":  true,
	".json":  true,
	".jsonl": true,
	".rdata": true,
	".rds":   true,
	".sam":   true,
	".tar":   true,
	".tgz":   true,
	".tsv":   true,
	".txt":   true,
	".vcf":   true,
	".xls":   true,
	".xlsx":  true,
	".xml":   true,
	".zip":   true,
}

func LocalSupplements(items []domain.Item) []Supplement {
	out := make([]Supplement, 0)
	for _, item := range items {
		for _, attachment := range item.Attachments {
			supplement, ok := ClassifyLocalSupplement(item, attachment)
			if ok {
				out = append(out, supplement)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].ItemKey != out[j].ItemKey {
			return out[i].ItemKey < out[j].ItemKey
		}
		return out[i].AttachmentKey < out[j].AttachmentKey
	})
	return out
}

func ClassifyLocalSupplement(item domain.Item, attachment domain.Attachment) (Supplement, bool) {
	label := attachmentLabel(attachment)
	haystack := strings.ToLower(strings.Join([]string{
		attachment.Title,
		attachment.Filename,
		attachment.ZoteroPath,
		attachment.ContentType,
	}, " "))
	ext := attachmentExtension(attachment)
	contentType := strings.ToLower(attachment.ContentType)

	kind := ""
	confidence := 0.0
	evidence := []string{}

	if supplementSourceDataText.MatchString(haystack) {
		kind = "source_data"
		confidence = 0.96
		evidence = append(evidence, "text:source_data")
	}
	if supplementReportingSummary.MatchString(haystack) {
		kind = maxKind(kind, "reporting_summary")
		confidence = maxFloat(confidence, 0.9)
		evidence = append(evidence, "text:reporting_summary")
	}
	if supplementDatasetText.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		confidence = maxFloat(confidence, 0.94)
		evidence = append(evidence, "text:supplementary")
	}
	if supplementMOESMPattern.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		confidence = maxFloat(confidence, 0.9)
		evidence = append(evidence, "pattern:moesm")
	}
	if supplementMMCPattern.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		confidence = maxFloat(confidence, 0.88)
		evidence = append(evidence, "pattern:mmc")
	}
	if supplementPGenPattern.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		confidence = maxFloat(confidence, 0.86)
		evidence = append(evidence, "pattern:pgen_supplement")
	}
	if supplementScienceTable.MatchString(haystack) || supplementTableRange.MatchString(haystack) {
		kind = maxKind(kind, "supplementary_dataset")
		confidence = maxFloat(confidence, 0.86)
		evidence = append(evidence, "pattern:table_file")
	}
	if supplementDataExtensions[ext] {
		if kind == "" {
			kind = "data_file"
		}
		confidence = maxFloat(confidence, extensionConfidence(ext, contentType))
		evidence = append(evidence, "extension:"+ext)
	}

	if kind == "" || confidence <= 0 {
		return Supplement{}, false
	}
	if ext == ".pdf" && !hasPDFSupplementEvidence(evidence) {
		return Supplement{}, false
	}

	status := localResolutionStatus(attachment)
	supplement := Supplement{
		Provider:         "local",
		ProviderStatus:   "complete",
		Kind:             kind,
		Label:            label,
		ItemKey:          item.Key,
		ItemTitle:        item.Title,
		AttachmentKey:    attachment.Key,
		ContentType:      attachment.ContentType,
		Filename:         attachment.Filename,
		ZoteroPath:       attachment.ZoteroPath,
		LocalPath:        attachment.ResolvedPath,
		Resolved:         attachment.Resolved,
		ResolutionStatus: status,
		Confidence:       confidence,
		Evidence:         evidence,
		Attachment:       attachment,
	}
	if attachment.Resolved && attachment.ResolvedPath != "" {
		if info, err := os.Stat(attachment.ResolvedPath); err == nil {
			supplement.Size = info.Size()
		}
	}
	return supplement, true
}

func attachmentLabel(attachment domain.Attachment) string {
	for _, value := range []string{attachment.Title, attachment.Filename, pathName(attachment.ZoteroPath), attachment.Key} {
		value = strings.TrimSpace(value)
		if value != "" && value != "." {
			return value
		}
	}
	return attachment.Key
}

func attachmentExtension(attachment domain.Attachment) string {
	for _, value := range []string{attachment.Filename, attachment.ZoteroPath, attachment.Title} {
		if value == "" {
			continue
		}
		if after, ok := strings.CutPrefix(value, "storage:"); ok {
			value = after
		}
		if after, ok := strings.CutPrefix(value, "attachments:"); ok {
			value = after
		}
		if ext := strings.ToLower(filepath.Ext(value)); ext != "" {
			return ext
		}
	}
	return ""
}

func pathName(value string) string {
	if value == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(value, "storage:"); ok {
		value = after
	}
	if after, ok := strings.CutPrefix(value, "attachments:"); ok {
		value = after
	}
	return filepath.Base(filepath.FromSlash(value))
}

func localResolutionStatus(attachment domain.Attachment) string {
	if attachment.Resolved {
		switch {
		case strings.HasPrefix(attachment.ZoteroPath, "storage:"):
			return "stored_file_found"
		case strings.HasPrefix(attachment.ZoteroPath, "attachments:"):
			return "linked_file_found"
		case filepath.IsAbs(attachment.ZoteroPath):
			return "absolute_file_found"
		default:
			return "file_found"
		}
	}
	switch {
	case attachment.ZoteroPath == "":
		return "metadata_only"
	case strings.HasPrefix(attachment.ZoteroPath, "attachments:"):
		return "linked_file_unresolved"
	case strings.HasPrefix(attachment.ZoteroPath, "storage:"):
		return "storage_missing"
	case filepath.IsAbs(attachment.ZoteroPath):
		return "absolute_file_missing"
	default:
		return "unresolved"
	}
}

func hasPDFSupplementEvidence(evidence []string) bool {
	for _, value := range evidence {
		if strings.HasPrefix(value, "text:") || strings.HasPrefix(value, "pattern:") {
			return true
		}
	}
	return false
}

func extensionConfidence(ext string, contentType string) float64 {
	switch ext {
	case ".xlsx", ".xls", ".csv", ".tsv", ".zip", ".gz", ".json", ".jsonl":
		return 0.78
	case ".doc", ".docx":
		return 0.68
	case ".txt", ".xml":
		if strings.Contains(contentType, "text") || strings.Contains(contentType, "xml") {
			return 0.62
		}
		return 0.55
	default:
		return 0.72
	}
}

func maxKind(current string, candidate string) string {
	if current == "" || current == "data_file" {
		return candidate
	}
	return current
}

func maxFloat(a float64, b float64) float64 {
	if b > a {
		return b
	}
	return a
}
