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

	itemKey, pageFilter, typeFilter, jsonOutput, clearMode, ok := c.parseAnnotationsArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		return c.printErr(err)
	}

	item, err := localReader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}

	pdfs := filterPDFAttachments(item.Attachments)
	if len(pdfs) == 0 {
		return c.printErr(fmt.Errorf("item %s has no PDF attachment", itemKey))
	}
	att := pdfs[0]

	if clearMode {
		lr, ok := localReader.(interface {
			DeletePDFAnnotations(context.Context, domain.Attachment, backend.DeleteAnnotationsRequest) (backend.DeleteAnnotationsResult, error)
		})
		if !ok {
			return c.printErr(fmt.Errorf("delete annotations requires local reader support"))
		}
		req := backend.DeleteAnnotationsRequest{
			Page:  pageFilter,
			Type:  typeFilter,
		}
		result, err := lr.DeletePDFAnnotations(context.Background(), att, req)
		if err != nil {
			return c.printErr(err)
		}
		fmt.Fprintf(c.stdout, "Deleted %d annotation(s) from %s\n", result.Deleted, itemKey)
		fmt.Fprintf(c.stdout, "PDF: %s\n", result.PDFPath)
		return 0
	}

	// Source 1: PDF file annotations (PyMuPDF)
	var pdfAnns []backend.PDFAnnotation
	lr, ok := localReader.(interface {
		ReadPDFAnnotations(context.Context, domain.Attachment) (backend.ReadAnnotationsResult, error)
	})
	if ok {
		pdfResult, err := lr.ReadPDFAnnotations(context.Background(), att)
		if err == nil {
			pdfAnns = pdfResult.Annotations
		}
	}

	// Source 2: Zotero reader annotations (SQLite itemAnnotations table)
	dbAnns := item.Annotations

	// Apply filters to both sources
	filteredPDF := filterPDFAnns(pdfAnns, pageFilter, typeFilter)
	filteredDB := filterDBAnns(dbAnns, pageFilter, typeFilter)

	if jsonOutput {
		data := map[string]any{
			"item_key":       itemKey,
			"attachment_key": att.Key,
			"pdf_path":       att.ResolvedPath,
			"pdf_annotations":   filteredPDF,
			"db_annotations":    filteredDB,
			"total_pdf":         len(filteredPDF),
			"total_db":          len(filteredDB),
		}
		meta := map[string]any{
			"total_pdf": len(filteredPDF),
			"total_db":  len(filteredDB),
		}
		c.appendReadMetadata(meta, localReader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "annotations",
			Data:    data,
			Meta:    meta,
		})
	}

	fmt.Fprintf(c.stdout, "Annotations for %s (%s)\n", itemKey, item.Title)
	fmt.Fprintf(c.stdout, "PDF: %s\n", att.ResolvedPath)

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

func (c *CLI) parseAnnotationsArgs(args []string) (string, int, string, bool, bool, bool) {
	var itemKey string
	pageFilter := 0
	typeFilter := ""
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
					return "", 0, "", false, false, false
				}
				pageFilter = n
			case "type":
				typeFilter = arg
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
		default:
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				fmt.Fprintln(c.stderr, usageAnnotations)
				return "", 0, "", false, false, false
			}
			if strings.HasPrefix(arg, "--page=") {
				n, err := strconv.Atoi(strings.TrimPrefix(arg, "--page="))
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotations)
					return "", 0, "", false, false, false
				}
				pageFilter = n
			} else if strings.HasPrefix(arg, "--type=") {
				typeFilter = strings.TrimPrefix(arg, "--type=")
			} else if itemKey != "" {
				fmt.Fprintln(c.stderr, usageAnnotations)
				return "", 0, "", false, false, false
			} else {
				itemKey = arg
			}
		}
	}

	if itemKey == "" {
		fmt.Fprintln(c.stderr, usageAnnotations)
		return "", 0, "", false, false, false
	}
	return itemKey, pageFilter, typeFilter, jsonOutput, clearMode, true
}
