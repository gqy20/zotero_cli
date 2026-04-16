package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

func writeJSON(value any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return printErr(err)
	}
	return 0
}

func renderFindItemDetailed(item domain.Item, opts backend.FindOptions) {
	fmt.Fprintf(stdout, "Key: %s\n", item.Key)
	fmt.Fprintf(stdout, "Title: %s\n", item.Title)
	fmt.Fprintf(stdout, "Type: %s\n", item.ItemType)
	printDate := item.Date != "" && !fieldIncluded(opts.IncludeFields, "date")
	printCreators := len(item.Creators) > 0 && !fieldIncluded(opts.IncludeFields, "creators")
	if printDate {
		fmt.Fprintf(stdout, "Date: %s\n", item.Date)
	}
	if printCreators {
		fmt.Fprintf(stdout, "Creators: %s\n", joinCreatorNames(item.Creators))
	}

	fields := opts.IncludeFields
	if opts.Full {
		fields = []string{"container", "volume", "issue", "pages", "version", "doi", "url", "tags", "matched_on"}
	}

	for _, field := range fields {
		switch field {
		case "container":
			if item.Container != "" {
				fmt.Fprintf(stdout, "Container: %s\n", item.Container)
			}
		case "version":
			if item.Version != 0 {
				fmt.Fprintf(stdout, "Version: %d\n", item.Version)
			}
		case "volume":
			if item.Volume != "" {
				fmt.Fprintf(stdout, "Volume: %s\n", item.Volume)
			}
		case "issue":
			if item.Issue != "" {
				fmt.Fprintf(stdout, "Issue: %s\n", item.Issue)
			}
		case "pages":
			if item.Pages != "" {
				fmt.Fprintf(stdout, "Pages: %s\n", item.Pages)
			}
		case "doi":
			if item.DOI != "" {
				fmt.Fprintf(stdout, "DOI: %s\n", item.DOI)
			}
		case "url":
			if item.URL != "" {
				fmt.Fprintf(stdout, "URL: %s\n", item.URL)
			}
		case "tags":
			if len(item.Tags) > 0 {
				fmt.Fprintf(stdout, "Tags: %s\n", strings.Join(item.Tags, ", "))
			}
		case "date":
			if item.Date != "" {
				fmt.Fprintf(stdout, "Date: %s\n", item.Date)
			}
		case "creators":
			if len(item.Creators) > 0 {
				fmt.Fprintf(stdout, "Creators: %s\n", joinCreatorNames(item.Creators))
			}
		case "key":
			fmt.Fprintf(stdout, "Key: %s\n", item.Key)
		case "item_type":
			fmt.Fprintf(stdout, "Type: %s\n", item.ItemType)
		case "title":
			fmt.Fprintf(stdout, "Title: %s\n", item.Title)
		case "collections":
			if len(item.Collections) > 0 {
				fmt.Fprintf(stdout, "Collections: %s\n", joinCollectionNames(item.Collections))
			}
		case "attachments":
			if len(item.Attachments) > 0 {
				fmt.Fprintf(stdout, "Attachments: %d\n", len(item.Attachments))
				for _, attachment := range item.Attachments {
					fmt.Fprintf(stdout, "  - [%s] %s\n", attachmentKind(attachment), attachmentSummary(attachment))
				}
			}
		case "notes":
			if len(item.Notes) > 0 {
				fmt.Fprintf(stdout, "Notes: %d\n", len(item.Notes))
				for _, note := range item.Notes {
					fmt.Fprintf(stdout, "  - %s\n", noteSummary(note))
				}
			}
		case "matched_on":
			if len(item.MatchedOn) > 0 {
				fmt.Fprintf(stdout, "Matched On: %s\n", strings.Join(item.MatchedOn, ", "))
			}
		case "full_text_preview":
			if item.FullTextPreview != "" {
				fmt.Fprintf(stdout, "Full Text Preview: %s\n", item.FullTextPreview)
			}
		}
	}
}

func fieldIncluded(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func renderCreators(creators []domain.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	if len(creators) == 1 {
		return creators[0].Name
	}
	return creators[0].Name + " et al."
}

func joinCreatorNames(creators []domain.Creator) string {
	names := make([]string, 0, len(creators))
	for _, creator := range creators {
		if creator.Name != "" {
			names = append(names, creator.Name)
		}
	}
	return strings.Join(names, ", ")
}

func joinCollectionNames(collections []domain.Collection) string {
	names := make([]string, 0, len(collections))
	for _, collection := range collections {
		if collection.Name != "" {
			names = append(names, collection.Name)
		}
	}
	return strings.Join(names, ", ")
}

func attachmentLabel(attachment domain.Attachment) string {
	if attachment.Filename != "" {
		return attachment.Filename
	}
	if attachment.Title != "" {
		return attachment.Title
	}
	return attachment.Key
}

func attachmentSummary(attachment domain.Attachment) string {
	label := attachmentLabel(attachment)
	if attachment.Key == "" {
		return label
	}
	return label + " (" + attachment.Key + ")"
}

func noteSummary(note domain.Note) string {
	if note.Preview == "" {
		return note.Key
	}
	if note.Key == "" {
		return note.Preview
	}
	return note.Key + ": " + note.Preview
}

func attachmentPathLine(attachment domain.Attachment) string {
	if attachment.Resolved && attachment.ResolvedPath != "" {
		return "path: " + attachment.ResolvedPath
	}
	if attachment.ZoteroPath != "" {
		return "path: unresolved (" + attachment.ZoteroPath + ")"
	}
	return ""
}

func attachmentKind(attachment domain.Attachment) string {
	if attachment.ContentType == "application/pdf" {
		return "pdf"
	}
	switch attachment.LinkMode {
	case "linked_url":
		return "link"
	case "linked_file", "imported_file":
		return "file"
	default:
		if attachment.ContentType != "" {
			return "file"
		}
		return "attachment"
	}
}

func maskConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"mode":                cfg.Mode,
		"data_dir":            cfg.DataDir,
		"library_type":        cfg.LibraryType,
		"library_id":          cfg.LibraryID,
		"api_key":             maskSecret(cfg.APIKey),
		"style":               cfg.Style,
		"locale":              cfg.Locale,
		"timeout_seconds":     cfg.TimeoutSeconds,
		"retry_max_attempts":  cfg.RetryMaxAttempts,
		"retry_base_delay_ms": cfg.RetryBaseDelayMilliseconds,
		"allow_write":         cfg.AllowWrite,
		"allow_delete":        cfg.AllowDelete,
	}
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}
