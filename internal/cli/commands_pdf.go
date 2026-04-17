package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type localPDFReader interface {
	GetItem(ctx context.Context, key string) (domain.Item, error)
	ExtractAttachmentImages(ctx context.Context, attachment domain.Attachment, outputDir string) ([]backend.ExtractedImage, error)
	ConsumeReadMetadata() backend.ReadMetadata
}

var newLocalPDFReader = func(cfg config.Config) (localPDFReader, error) {
	return backend.NewLocalReader(cfg)
}

func runExtractImages(args []string) int {
	if isHelpOnly(args) {
		return printCommandUsage(usageExtractImages)
	}

	itemKey, outputDir, jsonOutput, ok := parseExtractImagesArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := newLocalPDFReader(cfg)
	if err != nil {
		return printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return printErr(err)
	}

	pdfAttachments := filterPDFAttachments(item.Attachments)
	allImages := make([]backend.ExtractedImage, 0)
	for _, attachment := range pdfAttachments {
		images, err := localReader.ExtractAttachmentImages(context.Background(), attachment, outputDir)
		if err != nil {
			return printErr(err)
		}
		allImages = append(allImages, images...)
	}

	readMeta := localReader.ConsumeReadMetadata()
	if jsonOutput {
		meta := map[string]any{
			"total": len(allImages),
		}
		appendExplicitReadMetadata(meta, readMeta)
		return writeJSON(jsonResponse{
			OK:      true,
			Command: "extract-images",
			Data: map[string]any{
				"item_key": item.Key,
				"images":   allImages,
			},
			Meta: meta,
		})
	}

	warnIfSnapshotRead(readMeta)
	fmt.Fprintf(stdout, "Item: %s\n", item.Key)
	fmt.Fprintf(stdout, "PDF Attachments: %d\n", len(pdfAttachments))
	fmt.Fprintf(stdout, "Extracted Images: %d\n", len(allImages))
	for _, image := range allImages {
		fmt.Fprintf(stdout, "  - [%s] page=%d %dx%d %s\n", image.AttachmentKey, image.Page, image.Width, image.Height, image.Path)
	}
	return 0
}

func parseExtractImagesArgs(args []string) (string, string, bool, bool) {
	itemKey := ""
	outputDir := ""
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "error: missing value for --output")
				fmt.Fprintln(stderr, usageExtractImages)
				return "", "", false, false
			}
			outputDir = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "--") || itemKey != "" {
				fmt.Fprintln(stderr, usageExtractImages)
				return "", "", false, false
			}
			itemKey = args[i]
		}
	}

	if strings.TrimSpace(itemKey) == "" {
		fmt.Fprintln(stderr, usageExtractImages)
		return "", "", false, false
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Join(".", sanitizeOutputDirName(itemKey)+"_images")
	}
	return itemKey, outputDir, jsonOutput, true
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

func sanitizeOutputDirName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "item"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	value = replacer.Replace(value)
	value = strings.Trim(value, "._")
	if value == "" {
		return "item"
	}
	return value
}
