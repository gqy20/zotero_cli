package cli

import (
	"context"
	"fmt"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type localTextReader interface {
	GetItem(ctx context.Context, key string) (domain.Item, error)
	ExtractItemFullText(ctx context.Context, item domain.Item) (string, error)
	ConsumeReadMetadata() backend.ReadMetadata
}

var newLocalTextReader = func(cfg config.Config) (localTextReader, error) {
	return backend.NewLocalReader(cfg)
}

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

	localReader, err := newLocalTextReader(cfg)
	if err != nil {
		return printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return printErr(err)
	}
	text, err := localReader.ExtractItemFullText(context.Background(), item)
	if err != nil {
		return printErr(err)
	}

	readMeta := localReader.ConsumeReadMetadata()
	if jsonOutput {
		meta := map[string]any{
			"total": len([]rune(text)),
		}
		appendExplicitReadMetadata(meta, readMeta)
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "extract-text",
			Data: map[string]any{
				"item_key": item.Key,
				"text":     text,
			},
			Meta: meta,
		})
	}

	warnIfSnapshotRead(readMeta)
	fmt.Fprintln(stdout, text)
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
				fmt.Fprintln(stderr, usageExtractText)
				return "", false, false
			}
			itemKey = arg
		}
	}

	if strings.TrimSpace(itemKey) == "" {
		fmt.Fprintln(stderr, usageExtractText)
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
