package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

const usageInspectAttachment = `usage: zot inspect-attachment <attachment-key> [--health] [--sheet NAME] [--head N] [--max-sheets N] [--max-columns N] [--json]
       zot inspect-attachment --item <item-key> [--health] [--head N] [--max-sheets N] [--max-columns N] [--json]

What: Preview local spreadsheet attachments. The command reads local .xlsx files
in streaming mode and returns sheet names, dimensions, likely header rows, and
small row previews. With --health, it also reports local attachment path/name
issues and can inspect non-spreadsheet attachments. It does not modify files.

Output:
  --json          Structured output for agents.
  --sheet NAME    Inspect one sheet in one attachment.
  --head N        Preview N non-empty rows per inspected sheet (default 5).
  --max-sheets N  Inspect at most N sheets per workbook unless --sheet is set
                  (default 5).
  --max-columns N Preview at most N cells from each row while keeping the real
                  column count in metadata (default 12).
  --item KEY      Inspect all local .xlsx attachments under one Zotero item.
  --health        Include attachment health diagnostics. With --item, scans all
                  attachments under the item instead of only spreadsheets.

Examples:
  zot inspect-attachment ATT123 --json
  zot inspect-attachment ATT123 --sheet "Table S1" --head 20 --json
  zot inspect-attachment --item ITEM123 --json
  zot inspect-attachment --item ITEM123 --health --json

Notes:
  - Requires local/hybrid mode with resolved local attachment files.
  - Currently supports .xlsx/.xlsm workbooks. Use supplements first to discover
    candidate attachment keys.`

type inspectAttachmentArgs struct {
	attachmentKey string
	itemKey       string
	sheet         string
	head          int
	maxSheets     int
	maxColumns    int
	jsonOutput    bool
	health        bool
}

type inspectAttachmentResult struct {
	AttachmentKey string                    `json:"attachment_key"`
	ItemKey       string                    `json:"item_key,omitempty"`
	Label         string                    `json:"label,omitempty"`
	ContentType   string                    `json:"content_type,omitempty"`
	LocalPath     string                    `json:"local_path"`
	Health        *backend.AttachmentHealth `json:"health,omitempty"`
	Workbook      *backend.TableWorkbook    `json:"workbook,omitempty"`
}

func (c *CLI) runInspectAttachment(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageInspectAttachment)
	}
	parsed, ok := parseInspectAttachmentArgs(args)
	if !ok {
		fmt.Fprintln(c.stderr, usageInspectAttachment)
		return ExitUsage
	}

	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	if cfg.Mode == "web" {
		return c.printErr(fmt.Errorf("inspect-attachment requires local, hybrid, or remote mode with local attachment access"))
	}

	ctx := context.Background()
	results, err := inspectAttachments(ctx, reader, parsed)
	if err != nil {
		return c.printErr(err)
	}

	if parsed.jsonOutput {
		meta := map[string]any{
			"total":       len(results),
			"head":        normalizedHead(parsed.head),
			"max_sheets":  normalizedMaxSheets(parsed.maxSheets),
			"max_columns": normalizedMaxColumns(parsed.maxColumns),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "inspect-attachment",
			Data:    results,
			Meta:    meta,
		})
	}

	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))
	for index, result := range results {
		if index > 0 {
			fmt.Fprintln(c.stdout)
		}
		c.renderInspectAttachmentResult(result)
	}
	return ExitOK
}

func parseInspectAttachmentArgs(args []string) (inspectAttachmentArgs, bool) {
	var parsed inspectAttachmentArgs
	nextFlag := ""
	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "sheet":
				parsed.sheet = arg
			case "head":
				value, err := strconv.Atoi(arg)
				if err != nil || value <= 0 {
					return inspectAttachmentArgs{}, false
				}
				parsed.head = value
			case "max-sheets":
				value, err := strconv.Atoi(arg)
				if err != nil || value <= 0 {
					return inspectAttachmentArgs{}, false
				}
				parsed.maxSheets = value
			case "max-columns":
				value, err := strconv.Atoi(arg)
				if err != nil || value <= 0 {
					return inspectAttachmentArgs{}, false
				}
				parsed.maxColumns = value
			case "item":
				parsed.itemKey = arg
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--json":
			parsed.jsonOutput = true
		case "--health":
			parsed.health = true
		case "--sheet":
			nextFlag = "sheet"
		case "--head":
			nextFlag = "head"
		case "--max-sheets":
			nextFlag = "max-sheets"
		case "--max-columns":
			nextFlag = "max-columns"
		case "--item":
			nextFlag = "item"
		default:
			if strings.HasPrefix(arg, "-") {
				return inspectAttachmentArgs{}, false
			}
			if parsed.attachmentKey != "" {
				return inspectAttachmentArgs{}, false
			}
			parsed.attachmentKey = arg
		}
	}
	if nextFlag != "" {
		return inspectAttachmentArgs{}, false
	}
	if parsed.itemKey != "" && parsed.attachmentKey != "" {
		return inspectAttachmentArgs{}, false
	}
	if parsed.itemKey == "" && parsed.attachmentKey == "" {
		return inspectAttachmentArgs{}, false
	}
	if parsed.itemKey != "" && parsed.sheet != "" {
		return inspectAttachmentArgs{}, false
	}
	return parsed, true
}

func inspectAttachments(ctx context.Context, reader backend.Reader, parsed inspectAttachmentArgs) ([]inspectAttachmentResult, error) {
	opts := backend.TableInspectOptions{
		Sheet:      parsed.sheet,
		Head:       normalizedHead(parsed.head),
		MaxSheets:  normalizedMaxSheets(parsed.maxSheets),
		MaxColumns: normalizedMaxColumns(parsed.maxColumns),
	}
	if parsed.attachmentKey != "" {
		path, contentType, err := reader.GetAttachmentFile(ctx, parsed.attachmentKey)
		if err != nil {
			return nil, err
		}
		attachment := domain.Attachment{
			Key:          parsed.attachmentKey,
			ContentType:  contentType,
			ResolvedPath: path,
			Resolved:     true,
		}
		result := inspectAttachmentResult{
			AttachmentKey: parsed.attachmentKey,
			ContentType:   contentType,
			LocalPath:     path,
		}
		if parsed.health {
			health := backend.InspectAttachmentHealth(attachment)
			result.Health = &health
		}
		if !parsed.health || isTableAttachmentPath(path) {
			workbook, err := backend.InspectTableFile(path, opts)
			if err != nil {
				return nil, err
			}
			result.Workbook = &workbook
		}
		return []inspectAttachmentResult{result}, nil
	}

	item, err := reader.GetItem(ctx, parsed.itemKey)
	if err != nil {
		return nil, err
	}
	results := make([]inspectAttachmentResult, 0)
	attachments := tableAttachments(item.Attachments)
	if parsed.health {
		attachments = item.Attachments
	}
	for _, attachment := range attachments {
		if !attachment.Resolved {
			if !parsed.health {
				continue
			}
		}
		result := inspectAttachmentResult{
			AttachmentKey: attachment.Key,
			ItemKey:       item.Key,
			Label:         attachmentLabelForInspect(attachment),
			ContentType:   attachment.ContentType,
			LocalPath:     attachment.ResolvedPath,
		}
		if parsed.health {
			health := backend.InspectAttachmentHealth(attachment)
			result.Health = &health
		}
		if attachment.Resolved && isTableAttachmentPath(firstNonEmptyInspectString(attachment.Filename, attachment.ZoteroPath, attachment.ResolvedPath, attachment.Title)) {
			workbook, err := backend.InspectTableFile(attachment.ResolvedPath, opts)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", attachment.Key, err)
			}
			result.Workbook = &workbook
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		if parsed.health {
			return nil, fmt.Errorf("no attachments found for item: %s", parsed.itemKey)
		}
		return nil, fmt.Errorf("no resolved local .xlsx attachments found for item: %s", parsed.itemKey)
	}
	return results, nil
}

func tableAttachments(attachments []domain.Attachment) []domain.Attachment {
	out := make([]domain.Attachment, 0)
	for _, attachment := range attachments {
		ext := strings.ToLower(filepath.Ext(firstNonEmptyInspectString(attachment.Filename, attachment.ZoteroPath, attachment.ResolvedPath, attachment.Title)))
		if ext == ".xlsx" || ext == ".xlsm" || ext == ".xltx" || ext == ".xltm" {
			out = append(out, attachment)
		}
	}
	return out
}

func isTableAttachmentPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".xlsx" || ext == ".xlsm" || ext == ".xltx" || ext == ".xltm"
}

func attachmentLabelForInspect(attachment domain.Attachment) string {
	return firstNonEmptyInspectString(attachment.Title, attachment.Filename, filepath.Base(attachment.ResolvedPath), attachment.Key)
}

func normalizedHead(value int) int {
	if value <= 0 {
		return 5
	}
	return value
}

func normalizedMaxSheets(value int) int {
	if value <= 0 {
		return 5
	}
	return value
}

func normalizedMaxColumns(value int) int {
	if value <= 0 {
		return 12
	}
	return value
}

func firstNonEmptyInspectString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *CLI) renderInspectAttachmentResult(result inspectAttachmentResult) {
	title := result.AttachmentKey
	if result.Label != "" {
		title = fmt.Sprintf("%s (%s)", result.Label, result.AttachmentKey)
	}
	fmt.Fprintf(c.stdout, "Attachment: %s\n", title)
	fmt.Fprintf(c.stdout, "Path: %s\n", result.LocalPath)
	if result.Health != nil {
		fmt.Fprintf(c.stdout, "Health: %s\n", result.Health.Status)
		for _, issue := range result.Health.Issues {
			fmt.Fprintf(c.stdout, "  - %s %s: %s\n", issue.Severity, issue.Code, issue.Message)
			if issue.SuggestedName != "" {
				fmt.Fprintf(c.stdout, "    suggested_name: %s\n", issue.SuggestedName)
			}
		}
	}
	if result.Workbook == nil {
		return
	}
	fmt.Fprintf(c.stdout, "Workbook: %s, sheets=%d\n", result.Workbook.FileType, result.Workbook.SheetCount)
	for _, sheet := range result.Workbook.Sheets {
		fmt.Fprintf(c.stdout, "  - %s rows=%d cols=%d", sheet.Name, sheet.Rows, sheet.Columns)
		if sheet.Kind != "" {
			fmt.Fprintf(c.stdout, " kind=%s", sheet.Kind)
		}
		if sheet.HeaderRow > 0 {
			fmt.Fprintf(c.stdout, " header_row=%d", sheet.HeaderRow)
		}
		fmt.Fprintln(c.stdout)
		for _, row := range sheet.Preview {
			fmt.Fprintf(c.stdout, "      %s\n", strings.Join(row, " | "))
		}
		if sheet.ColumnsTruncated {
			fmt.Fprintln(c.stdout, "      ... columns truncated")
		}
		if sheet.PreviewTruncated {
			fmt.Fprintln(c.stdout, "      ...")
		}
	}
	if result.Workbook.SheetsTruncated {
		fmt.Fprintln(c.stdout, "  ...")
	}
}
