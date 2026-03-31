package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type versionsArgs struct {
	Since                  int
	IncludeTrashed         bool
	IfModifiedSinceVersion int
}

type jsonResponse struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Data    any            `json:"data"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func parseFindArgs(args []string) (zoteroapi.FindOptions, bool, error) {
	opts := zoteroapi.FindOptions{}
	jsonOutput := false
	queryParts := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--item-type":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --item-type")
			}
			i++
			opts.ItemType = args[i]
		case "--tag":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --tag")
			}
			i++
			opts.Tag = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return zoteroapi.FindOptions{}, false, errors.New("invalid value for --limit")
			}
			opts.Limit = limit
		case "--start":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --start")
			}
			i++
			start, err := strconv.Atoi(args[i])
			if err != nil || start < 0 {
				return zoteroapi.FindOptions{}, false, errors.New("invalid value for --start")
			}
			opts.Start = start
		case "--sort":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --sort")
			}
			i++
			opts.Sort = args[i]
		case "--direction":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --direction")
			}
			i++
			if args[i] != "asc" && args[i] != "desc" {
				return zoteroapi.FindOptions{}, false, errors.New("invalid value for --direction")
			}
			opts.Direction = args[i]
		case "--qmode":
			if i+1 >= len(args) {
				return zoteroapi.FindOptions{}, false, errors.New("missing value for --qmode")
			}
			i++
			if args[i] != "titleCreatorYear" && args[i] != "everything" {
				return zoteroapi.FindOptions{}, false, errors.New("invalid value for --qmode")
			}
			opts.QMode = args[i]
		case "--include-trashed":
			opts.IncludeTrashed = true
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	opts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	return opts, jsonOutput, nil
}

func parseExportArgs(args []string) (string, zoteroapi.FindOptions, string, bool, error) {
	var itemKey string
	var format string
	var jsonOutput bool
	findOpts := zoteroapi.FindOptions{}
	queryParts := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--item-key":
			if i+1 >= len(args) {
				return "", zoteroapi.FindOptions{}, "", false, errors.New("missing value for --item-key")
			}
			i++
			itemKey = args[i]
		case "--format":
			if i+1 >= len(args) {
				return "", zoteroapi.FindOptions{}, "", false, errors.New("missing value for --format")
			}
			i++
			format = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return "", zoteroapi.FindOptions{}, "", false, errors.New("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return "", zoteroapi.FindOptions{}, "", false, errors.New("invalid value for --limit")
			}
			findOpts.Limit = limit
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	if itemKey != "" && len(queryParts) > 0 {
		return "", zoteroapi.FindOptions{}, "", false, errors.New("cannot use query and --item-key together")
	}

	if itemKey == "" {
		findOpts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
		if findOpts.Query == "" {
			return "", zoteroapi.FindOptions{}, "", false, errors.New("missing query or --item-key")
		}
	}

	if format != "" && format != "bib" && format != "bibtex" && format != "biblatex" && format != "csljson" && format != "ris" {
		return "", zoteroapi.FindOptions{}, "", false, errors.New("unsupported format")
	}

	return itemKey, findOpts, format, jsonOutput, nil
}

func parseCiteArgs(args []string) (string, zoteroapi.CitationOptions, bool, error) {
	var key string
	opts := zoteroapi.CitationOptions{}
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--format":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --format")
			}
			i++
			opts.Format = args[i]
		case "--style":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --style")
			}
			i++
			opts.Style = args[i]
		case "--locale":
			if i+1 >= len(args) {
				return "", zoteroapi.CitationOptions{}, false, errors.New("missing value for --locale")
			}
			i++
			opts.Locale = args[i]
		default:
			if key == "" {
				key = args[i]
				continue
			}
			return "", zoteroapi.CitationOptions{}, false, errors.New("too many positional arguments")
		}
	}

	if strings.TrimSpace(key) == "" {
		return "", zoteroapi.CitationOptions{}, false, errors.New("missing item key")
	}
	if opts.Format != "" && opts.Format != "citation" && opts.Format != "bib" {
		return "", zoteroapi.CitationOptions{}, false, errors.New("unsupported format")
	}

	return key, opts, jsonOutput, nil
}

func parseJSONOnlyArgs(args []string, usage string) (bool, bool) {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		fmt.Fprintln(stderr, usage)
		return false, false
	}
	return jsonOutput, true
}

func parseVersionsArgs(args []string) (string, versionsArgs, bool, error) {
	if len(args) == 0 {
		return "", versionsArgs{}, false, errors.New("missing object type")
	}

	objectType := args[0]
	switch objectType {
	case "collections", "searches", "items", "items-top":
	default:
		return "", versionsArgs{}, false, errors.New("unsupported object type")
	}

	var opts versionsArgs
	var jsonOutput bool
	var sinceSet bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--include-trashed":
			opts.IncludeTrashed = true
		case "--since":
			if i+1 >= len(args) {
				return "", versionsArgs{}, false, errors.New("missing value for --since")
			}
			i++
			since, err := strconv.Atoi(args[i])
			if err != nil || since < 0 {
				return "", versionsArgs{}, false, errors.New("invalid value for --since")
			}
			opts.Since = since
			sinceSet = true
		case "--if-modified-since-version":
			if i+1 >= len(args) {
				return "", versionsArgs{}, false, errors.New("missing value for --if-modified-since-version")
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 0 {
				return "", versionsArgs{}, false, errors.New("invalid value for --if-modified-since-version")
			}
			opts.IfModifiedSinceVersion = value
		default:
			return "", versionsArgs{}, false, errors.New("too many positional arguments")
		}
	}

	if !sinceSet {
		return "", versionsArgs{}, false, errors.New("missing value for --since")
	}

	return objectType, opts, jsonOutput, nil
}

func parseSingleValueCommand(args []string, usage string) (string, bool, bool) {
	jsonOutput := false
	value := ""

	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if value == "" {
			value = arg
			continue
		}
		fmt.Fprintln(stderr, usage)
		return "", false, false
	}

	if strings.TrimSpace(value) == "" {
		fmt.Fprintln(stderr, usage)
		return "", false, false
	}

	return value, jsonOutput, true
}

func parseWriteCreateArgs(args []string, usage string) (map[string]any, int, bool, bool) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || !versionSet {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func parseWriteUpdateArgs(args []string, usage string, requireVersion bool) (string, map[string]any, int, bool, bool) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return "", nil, 0, false, false
	}
	key := args[0]
	data, version, jsonOutput, ok := parseWriteCreateArgs(args[1:], usage)
	if ok {
		return key, data, version, jsonOutput, true
	}

	data, version, jsonOutput, ok = parseWriteCreateLikeArgs(args[1:], usage, requireVersion)
	if !ok {
		return "", nil, 0, false, false
	}
	return key, data, version, jsonOutput, true
}

func parseWriteDeleteArgs(args []string, usage string) (string, int, bool, bool) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return "", 0, false, false
	}
	key := args[0]
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return "", 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return "", 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return "", 0, false, false
		}
	}

	if !versionSet {
		fmt.Fprintln(stderr, usage)
		return "", 0, false, false
	}

	return key, version, jsonOutput, true
}

func parseWriteCreateLikeArgs(args []string, usage string, requireVersion bool) (map[string]any, int, bool, bool) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || (requireVersion && !versionSet) {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func parseWriteBatchArgs(args []string, usage string, requireVersion bool) ([]map[string]any, int, bool, bool) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || (requireVersion && !versionSet) {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data []map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil || len(data) == 0 {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func loadClient() (config.Config, *zoteroapi.Client, int) {
	cfg, _, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "config not found; run `zot config init` first")
			return config.Config{}, nil, 3
		}
		return config.Config{}, nil, printErr(err)
	}

	baseURL := os.Getenv("ZOT_BASE_URL")
	return cfg, zoteroapi.New(cfg, baseURL, nil), 0
}

func writeJSON(value any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return printErr(err)
	}
	return 0
}

func filterDefaultFindItems(items []zoteroapi.Item, opts zoteroapi.FindOptions) []zoteroapi.Item {
	if opts.ItemType != "" {
		return items
	}

	filtered := make([]zoteroapi.Item, 0, len(items))
	for _, item := range items {
		if item.ItemType == "attachment" || item.ItemType == "note" {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func shortDate(value string) string {
	if len(value) >= 4 {
		return value[:4]
	}
	return value
}

func shortCreators(creators []zoteroapi.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	name := creators[0].Name
	if len(creators) == 1 {
		return name
	}
	return name + " et al."
}

func renderCreators(creators []zoteroapi.Creator) string {
	if len(creators) == 0 {
		return ""
	}
	if len(creators) == 1 {
		return creators[0].Name
	}
	return creators[0].Name + " et al."
}

func joinCreatorNames(creators []zoteroapi.Creator) string {
	names := make([]string, 0, len(creators))
	for _, creator := range creators {
		if creator.Name != "" {
			names = append(names, creator.Name)
		}
	}
	return strings.Join(names, ", ")
}

func attachmentKind(attachment zoteroapi.Attachment) string {
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

func filterVisibleNotes(notes []zoteroapi.Note) []zoteroapi.Note {
	filtered := make([]zoteroapi.Note, 0, len(notes))
	for _, note := range notes {
		if isMachineNote(note.Content) {
			continue
		}
		filtered = append(filtered, note)
	}
	return filtered
}

func isMachineNote(content string) bool {
	content = strings.TrimSpace(content)
	return strings.Contains(content, "{\"readingTime\":")
}

func notePreview(content string) string {
	content = strings.TrimSpace(content)
	const limit = 96
	if len(content) <= limit {
		return content
	}
	return strings.TrimSpace(content[:limit-3]) + "..."
}

func maskConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"mode":            cfg.Mode,
		"library_type":    cfg.LibraryType,
		"library_id":      cfg.LibraryID,
		"api_key":         maskSecret(cfg.APIKey),
		"style":           cfg.Style,
		"locale":          cfg.Locale,
		"timeout_seconds": cfg.TimeoutSeconds,
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
