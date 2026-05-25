package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
)

func (c *CLI) runAnnotate(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageAnnotate)
	}

	itemKey, req, clearMode, authorFilter, jsonOutput, ok := c.parseAnnotateArgs(args)
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if cfg.Mode != "remote" {
		if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
			return exitCode
		}
	}

	item, err := reader.GetItem(context.Background(), itemKey)
	if err != nil {
		return c.printErr(err)
	}

	// Handle --clear mode: delete annotations instead of creating
	if clearMode {
		delReq := backend.DeleteAnnotationsRequest{
			Page:   req.Page,
			Type:   req.Type,
			Author: authorFilter,
		}
		return c.runAnnotationClear(reader, itemKey, delReq, jsonOutput, "annotate")
	}

	lr, ok := reader.(itemAnnotator)
	if !ok {
		return c.printErr(fmt.Errorf("annotation writing is not available for the current reader"))
	}

	result, err := lr.AnnotateItem(context.Background(), item, req)
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		data := map[string]any{
			"item_key":       itemKey,
			"attachment_key": result.AttachmentKey,
			"pdf_path":       result.PDFPath,
			"matches":        result.Matches,
			"total_matches":  len(result.Matches),
		}
		meta := map[string]any{
			"total_matches": len(result.Matches),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "annotate",
			Data:    data,
			Meta:    meta,
		})
	}

	fmt.Fprintf(c.stdout, "Annotated %s (%s)\n", itemKey, result.AttachmentKey)
	if strings.TrimSpace(result.PDFPath) != "" {
		fmt.Fprintf(c.stdout, "PDF: %s\n", result.PDFPath)
	}
	fmt.Fprintf(c.stdout, "Matches: %d\n\n", len(result.Matches))
	for _, m := range result.Matches {
		fmt.Fprintf(c.stdout, "  Page %d [%s %s]: \"%s\"\n", m.Page, m.Type, m.Color, m.Text)
	}
	return 0
}

func (c *CLI) parseAnnotateArgs(args []string) (string, backend.AnnotateRequest, bool, string, bool, bool) {
	var itemKey string
	req := backend.AnnotateRequest{
		Type:  "highlight",
		Color: "yellow",
	}
	clearMode := false
	authorFilter := ""
	jsonOutput := false
	nextFlag := ""

	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "text":
				req.Text = arg
			case "color":
				req.Color = arg
			case "comment":
				req.Comment = arg
			case "type":
				req.Type = arg
			case "page":
				n, err := strconv.Atoi(arg)
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				req.Page = n
			case "rect":
				parts := strings.Split(arg, ",")
				if len(parts) != 4 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				var rc [4]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return "", backend.AnnotateRequest{}, false, "", false, false
					}
					rc[i] = v
				}
				req.Rect = &rc
			case "point":
				parts := strings.Split(arg, ",")
				if len(parts) != 2 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				var pt [2]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return "", backend.AnnotateRequest{}, false, "", false, false
					}
					pt[i] = v
				}
				req.Point = &pt
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
		case "--text":
			nextFlag = "text"
		case "--color":
			nextFlag = "color"
		case "--comment":
			nextFlag = "comment"
		case "--type":
			nextFlag = "type"
		case "--page":
			nextFlag = "page"
		case "--rect":
			nextFlag = "rect"
		case "--point":
			nextFlag = "point"
		case "--author":
			nextFlag = "author"
		default:
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				fmt.Fprintln(c.stderr, usageAnnotate)
				return "", backend.AnnotateRequest{}, false, "", false, false
			}
			if strings.HasPrefix(arg, "--text=") {
				req.Text = strings.TrimPrefix(arg, "--text=")
			} else if strings.HasPrefix(arg, "--color=") {
				req.Color = strings.TrimPrefix(arg, "--color=")
			} else if strings.HasPrefix(arg, "--comment=") {
				req.Comment = strings.TrimPrefix(arg, "--comment=")
			} else if strings.HasPrefix(arg, "--type=") {
				req.Type = strings.TrimPrefix(arg, "--type=")
			} else if strings.HasPrefix(arg, "--page=") {
				n, err := strconv.Atoi(strings.TrimPrefix(arg, "--page="))
				if err != nil || n < 1 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				req.Page = n
			} else if strings.HasPrefix(arg, "--rect=") {
				parts := strings.Split(strings.TrimPrefix(arg, "--rect="), ",")
				if len(parts) != 4 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				var rc [4]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return "", backend.AnnotateRequest{}, false, "", false, false
					}
					rc[i] = v
				}
				req.Rect = &rc
			} else if strings.HasPrefix(arg, "--point=") {
				parts := strings.Split(strings.TrimPrefix(arg, "--point="), ",")
				if len(parts) != 2 {
					fmt.Fprintln(c.stderr, usageAnnotate)
					return "", backend.AnnotateRequest{}, false, "", false, false
				}
				var pt [2]float64
				for i, p := range parts {
					v, err := strconv.ParseFloat(p, 64)
					if err != nil {
						fmt.Fprintln(c.stderr, usageAnnotate)
						return "", backend.AnnotateRequest{}, false, "", false, false
					}
					pt[i] = v
				}
				req.Point = &pt
			} else if strings.HasPrefix(arg, "--author=") {
				authorFilter = strings.TrimPrefix(arg, "--author=")
			} else if itemKey != "" {
				fmt.Fprintln(c.stderr, usageAnnotate)
				return "", backend.AnnotateRequest{}, false, "", false, false
			} else {
				itemKey = arg
			}
		}
	}

	// In clear mode, only itemKey is required (page/type/author are optional filters)
	if clearMode {
		if itemKey == "" {
			fmt.Fprintln(c.stderr, usageAnnotate)
			return "", backend.AnnotateRequest{}, false, "", false, false
		}
		req.Type = "" // reset default "highlight" so clear deletes all types
		return itemKey, req, true, authorFilter, jsonOutput, true
	}

	hasText := req.Text != ""
	hasRect := req.Page > 0 && req.Rect != nil
	hasPoint := req.Page > 0 && req.Point != nil

	if itemKey == "" || (!hasText && !hasRect && !hasPoint) {
		fmt.Fprintln(c.stderr, usageAnnotate)
		return "", backend.AnnotateRequest{}, false, "", false, false
	}

	if hasPoint && req.Comment == "" {
		req.Comment = "Note"
	}

	return itemKey, req, false, authorFilter, jsonOutput, true
}
