package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

func (c *CLI) runAnnotations(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageAnnotations)
	}

	itemKey, pageFilter, typeFilter, authorFilter, jsonOutput, clearMode, ok := c.parseAnnotationsArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	if !clearMode {
		return c.runAnnotationsReadOnly(itemKey, pageFilter, typeFilter, jsonOutput)
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if cfg.Mode != "remote" {
		if exitCode := c.ensureDeleteAllowed(cfg); exitCode != 0 {
			return exitCode
		}
	}

	req := backend.DeleteAnnotationsRequest{
		Page:   pageFilter,
		Type:   typeFilter,
		Author: authorFilter,
	}
	return c.runAnnotationClear(reader, itemKey, req, jsonOutput, "annotations")
}

func (c *CLI) runAnnotationsReadOnly(itemKey string, pageFilter int, typeFilter string, jsonOutput bool) int {
	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	item, err := reader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}

	annotationsReader, ok := reader.(itemAnnotationsReader)
	if !ok {
		return c.printErr(fmt.Errorf("annotations are not available for the current reader"))
	}
	result, err := annotationsReader.ReadItemAnnotations(context.Background(), item)
	if err != nil {
		return c.printErr(err)
	}

	filteredPDF := filterPDFAnns(result.PDFAnnotations, pageFilter, typeFilter)
	filteredDB := filterDBAnns(result.DBAnnotations, pageFilter, typeFilter)

	if jsonOutput {
		data := map[string]any{
			"item_key":        itemKey,
			"attachment_key":  result.AttachmentKey,
			"pdf_path":        result.PDFPath,
			"pdf_annotations": filteredPDF,
			"db_annotations":  filteredDB,
			"total_pdf":       len(filteredPDF),
			"total_db":        len(filteredDB),
		}
		meta := map[string]any{
			"total_pdf": len(filteredPDF),
			"total_db":  len(filteredDB),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "annotations",
			Data:    data,
			Meta:    meta,
		})
	}

	fmt.Fprintf(c.stdout, "Annotations for %s (%s)\n", itemKey, item.Title)
	if strings.TrimSpace(result.PDFPath) != "" {
		fmt.Fprintf(c.stdout, "PDF: %s\n", result.PDFPath)
	}

	if len(filteredDB) > 0 {
		fmt.Fprintf(c.stdout, "\nZotero Reader Annotations (%d):\n", len(filteredDB))
		for _, a := range filteredDB {
			colorStr := ""
			if a.Color != "" {
				colorStr = " " + a.Color
			}
			dateStr := ""
			if a.DateAdded != "" {
				dateStr = " " + a.DateAdded
			}
			pageStr := ""
			if a.PageIndex >= 0 {
				pageStr = fmt.Sprintf(" page=%d", a.PageIndex+1)
			}
			switch a.Type {
			case "highlight":
				fmt.Fprintf(c.stdout, "  [%s%s%s%s]: \"%s\"\n", a.Type, colorStr, dateStr, pageStr, a.Text)
			case "note":
				fmt.Fprintf(c.stdout, "  [note%s%s%s]: \"%s\"\n", colorStr, dateStr, pageStr, a.Comment)
			default:
				fmt.Fprintf(c.stdout, "  [%s%s%s%s]\n", a.Type, colorStr, dateStr, pageStr)
			}
		}
	}

	if len(filteredPDF) > 0 {
		fmt.Fprintf(c.stdout, "\nPDF File Annotations (%d):\n", len(filteredPDF))
		for _, a := range filteredPDF {
			colorStr := ""
			if a.Color != "" {
				colorStr = " " + a.Color
			}
			dateStr := ""
			if a.Date != "" {
				dateStr = " " + a.Date
			}
			switch a.Type {
			case "highlight", "underline":
				fmt.Fprintf(c.stdout, "  Page %d [%s%s%s]: \"%s\"\n", a.Page, a.Type, colorStr, dateStr, a.Text)
			case "text":
				fmt.Fprintf(c.stdout, "  Page %d [note%s%s]: \"%s\"\n", a.Page, colorStr, dateStr, a.Comment)
			default:
				fmt.Fprintf(c.stdout, "  Page %d [%s%s%s]\n", a.Page, a.Type, colorStr, dateStr)
			}
		}
	}

	total := len(filteredDB) + len(filteredPDF)
	if total == 0 {
		fmt.Fprintf(c.stdout, "\nNo annotations found.\n")
	} else {
		fmt.Fprintf(c.stdout, "\nTotal: %d (db:%d + pdf:%d)\n", total, len(filteredDB), len(filteredPDF))
	}
	return 0
}

func (c *CLI) runAnnotationClear(reader backend.Reader, itemKey string, req backend.DeleteAnnotationsRequest, jsonOutput bool, command string) int {
	item, err := reader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}

	clearer, ok := reader.(itemAnnotationClearer)
	if !ok {
		return c.printErr(fmt.Errorf("annotation deletion is not available for the current reader"))
	}

	result, err := clearer.ClearItemAnnotations(context.Background(), item, req)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		data := map[string]any{
			"item_key":       itemKey,
			"attachment_key": result.AttachmentKey,
			"pdf_path":       result.PDFPath,
			"pdf_deleted":    result.PDFDeleted,
			"db_deleted":     result.DBDeleted,
			"deleted":        result.Deleted,
		}
		if result.DBError != "" {
			data["db_error"] = result.DBError
		}
		meta := map[string]any{
			"deleted": result.Deleted,
		}
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: command,
			Data:    data,
			Meta:    meta,
		})
	}

	fmt.Fprintf(c.stdout, "Deleted %d annotation(s) from %s\n", result.Deleted, itemKey)
	if strings.TrimSpace(result.PDFPath) != "" {
		fmt.Fprintf(c.stdout, "PDF: %s\n", result.PDFPath)
	}
	if result.DBError != "" {
		fmt.Fprintf(c.stderr, "warning: could not delete DB annotations (Zotero may be running): %s\n", result.DBError)
	}
	return 0
}

func filterPDFAnns(anns []backend.PDFAnnotation, page int, typ string) []backend.PDFAnnotation {
	if page == 0 && typ == "" {
		return anns
	}
	var out []backend.PDFAnnotation
	for _, a := range anns {
		if page > 0 && a.Page != page {
			continue
		}
		if typ != "" && !strings.EqualFold(a.Type, typ) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func filterDBAnns(anns []domain.Annotation, page int, typ string) []domain.Annotation {
	if page == 0 && typ == "" {
		return anns
	}
	var out []domain.Annotation
	for _, a := range anns {
		if page > 0 && a.PageIndex+1 != page {
			continue
		}
		if typ != "" && !strings.EqualFold(a.Type, typ) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DateAdded > out[j].DateAdded })
	return out
}

func (c *CLI) parseAnnotationsArgs(args []string) (string, int, string, string, bool, bool, bool) {
	var itemKey string
	pageFilter := 0
	typeFilter := ""
	authorFilter := ""
	jsonOutput := false
	clearMode := false
	nextFlag := ""

	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "page":
				n, err := strconv.Atoi(arg)
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotations)
					return "", 0, "", "", false, false, false
				}
				pageFilter = n
			case "type":
				typeFilter = arg
			case "author":
				authorFilter = arg
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--json":
			jsonOutput = true
		case "--clear":
			clearMode = true
		case "--page":
			nextFlag = "page"
		case "--type":
			nextFlag = "type"
		case "--author":
			nextFlag = "author"
		default:
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				fmt.Fprintln(c.stderr, usageAnnotations)
				return "", 0, "", "", false, false, false
			}
			if strings.HasPrefix(arg, "--page=") {
				n, err := strconv.Atoi(strings.TrimPrefix(arg, "--page="))
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotations)
					return "", 0, "", "", false, false, false
				}
				pageFilter = n
			} else if strings.HasPrefix(arg, "--type=") {
				typeFilter = strings.TrimPrefix(arg, "--type=")
			} else if strings.HasPrefix(arg, "--author=") {
				authorFilter = strings.TrimPrefix(arg, "--author=")
			} else if itemKey != "" {
				fmt.Fprintln(c.stderr, usageAnnotations)
				return "", 0, "", "", false, false, false
			} else {
				itemKey = arg
			}
		}
	}

	if itemKey == "" {
		fmt.Fprintln(c.stderr, usageAnnotations)
		return "", 0, "", "", false, false, false
	}
	return itemKey, pageFilter, typeFilter, authorFilter, jsonOutput, clearMode, true
}
