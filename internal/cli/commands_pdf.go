package cli

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

func runExtractText(args []string) int {
	if isHelpOnly(args) {
		return printCommandUsage(usageExtractText)
	}

	itemKey, jsonOutput, ok := parseExtractTextArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := defaultCLI.newLocalReader(cfg)
	if err != nil {
		return printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return printErr(err)
	}
	if jsonOutput {
		var (
			result backend.ItemFullTextResult
			err    error
		)
		if attachmentReader, ok := localReader.(attachmentTextReader); ok {
			result, err = attachmentReader.ExtractItemAttachmentTexts(context.Background(), item)
		} else {
			textReader, ok := localReader.(fullTextReader)
			if !ok {
				return printErr(fmt.Errorf("extract-text requires local full-text extraction support"))
			}
			var text string
			text, err = textReader.ExtractItemFullText(context.Background(), item)
			result = backend.ItemFullTextResult{Text: text}
		}
		if err != nil {
			return printErr(err)
		}

		readMeta := consumeReaderReadMetadata(localReader)
		meta := map[string]any{
			"total": len([]rune(result.Text)),
		}
		appendExplicitReadMetadata(meta, readMeta)
		attachments := make([]map[string]any, 0, len(result.Attachments))
		for _, attachment := range result.Attachments {
			entry := map[string]any{
				"attachment_key": attachment.Attachment.Key,
				"text":           attachment.Text,
				"total":          len([]rune(attachment.Text)),
			}
			if attachment.Attachment.Title != "" {
				entry["title"] = attachment.Attachment.Title
			}
			if attachment.Attachment.Filename != "" {
				entry["filename"] = attachment.Attachment.Filename
			}
			if attachment.Attachment.ResolvedPath != "" {
				entry["resolved_path"] = attachment.Attachment.ResolvedPath
			}
			if attachment.Source != "" {
				entry["full_text_source"] = attachment.Source
			}
			if attachment.CacheHit {
				entry["full_text_cache_hit"] = true
			}
			attachments = append(attachments, entry)
		}
		data := map[string]any{
			"item_key": item.Key,
			"text":     result.Text,
		}
		if result.PrimaryAttachmentKey != "" {
			data["primary_attachment_key"] = result.PrimaryAttachmentKey
		}
		if len(attachments) > 0 {
			data["attachments"] = attachments
		}
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "extract-text",
			Data:    data,
			Meta:    meta,
		})
	}

	textReader, ok := localReader.(fullTextReader)
	if !ok {
		return printErr(fmt.Errorf("extract-text requires local full-text extraction support"))
	}
	text, err := textReader.ExtractItemFullText(context.Background(), item)
	if err != nil {
		return printErr(err)
	}
	readMeta := consumeReaderReadMetadata(localReader)
	warnIfSnapshotRead(readMeta)
	fmt.Fprintln(defaultCLI.stdout, text)
	return 0
}

func parseExtractTextArgs(args []string) (string, bool, bool) {
	itemKey := ""
	jsonOutput := false

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "--") || itemKey != "" {
				fmt.Fprintln(defaultCLI.stderr, usageExtractText)
				return "", false, false
			}
			itemKey = arg
		}
	}

	if strings.TrimSpace(itemKey) == "" {
		fmt.Fprintln(defaultCLI.stderr, usageExtractText)
		return "", false, false
	}
	return itemKey, jsonOutput, true
}

func filterPDFAttachments(attachments []domain.Attachment) []domain.Attachment {
	filtered := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		if strings.EqualFold(strings.TrimSpace(attachment.ContentType), "application/pdf") {
			filtered = append(filtered, attachment)
		}
	}
	return filtered
}
