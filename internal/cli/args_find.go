package cli

import (
	"fmt"
	"strconv"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/zoteroapi"
)

type findParsedArgs struct {
	Opts          backend.FindOptions
	JSONOutput    bool
	Snippet       bool
	QueryProvided bool
}

func parseFindArgs(args []string) (findParsedArgs, error) {
	opts := backend.FindOptions{}
	jsonOutput := false
	snippet := false
	queryProvided := false
	queryParts := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--snippet":
			snippet = true
		case "--all":
			opts.All = true
		case "--fulltext":
			opts.FullText = true
		case "--fulltext-only":
			opts.FullText = true
			opts.FullTextOnly = true
		case "--metadata-only":
			opts.MetadataOnly = true
		case "--fulltext-any":
			opts.FullTextAny = true
		case "--full":
			opts.Full = true
		case "--item-type":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --item-type")
			}
			i++
			opts.ItemType = args[i]
		case "--tag":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --tag")
			}
			i++
			opts.Tags = append(opts.Tags, args[i])
		case "--tag-any":
			opts.TagAny = true
		case "--include-fields":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --include-fields")
			}
			i++
			fields, err := parseFindIncludeFields(args[i])
			if err != nil {
				return findParsedArgs{}, err
			}
			opts.IncludeFields = append(opts.IncludeFields, fields...)
		case "--date-after":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --date-after")
			}
			i++
			opts.DateAfter = strings.TrimSpace(args[i])
		case "--date-before":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --date-before")
			}
			i++
			opts.DateBefore = strings.TrimSpace(args[i])
		case "--limit", "-l":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return findParsedArgs{}, fmt.Errorf("invalid value for --limit")
			}
			opts.Limit = limit
		case "--start":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --start")
			}
			i++
			start, err := strconv.Atoi(args[i])
			if err != nil || start < 0 {
				return findParsedArgs{}, fmt.Errorf("invalid value for --start")
			}
			opts.Start = start
		case "--sort":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --sort")
			}
			i++
			opts.Sort = args[i]
		case "--direction":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --direction")
			}
			i++
			if args[i] != "asc" && args[i] != "desc" {
				return findParsedArgs{}, fmt.Errorf("invalid value for --direction")
			}
			opts.Direction = args[i]
		case "--qmode":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --qmode")
			}
			i++
			if args[i] != "titleCreatorYear" && args[i] != "everything" {
				return findParsedArgs{}, fmt.Errorf("invalid value for --qmode")
			}
			opts.QMode = args[i]
		case "--has-pdf":
			opts.HasPDF = true
		case "--attachment-name":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --attachment-name")
			}
			i++
			opts.AttachmentName = strings.TrimSpace(args[i])
		case "--attachment-path":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --attachment-path")
			}
			i++
			opts.AttachmentPath = strings.TrimSpace(args[i])
		case "--attachment-type":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --attachment-type")
			}
			i++
			opts.AttachmentType = strings.TrimSpace(args[i])
		case "--missing-attachment":
			opts.MissingAttachment = true
		case "--bad-attachment-name":
			opts.BadAttachmentName = true
		case "--attachment-health":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --attachment-health")
			}
			i++
			value := strings.TrimSpace(strings.ToLower(args[i]))
			if value != "critical" && value != "error" && value != "warning" && value != "info" {
				return findParsedArgs{}, fmt.Errorf("invalid value for --attachment-health")
			}
			opts.AttachmentHealth = value
		case "--collection":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --collection")
			}
			i++
			opts.Collection = append(opts.Collection, strings.TrimSpace(args[i]))
		case "--no-collection":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --no-collection")
			}
			i++
			opts.NoCollection = append(opts.NoCollection, strings.TrimSpace(args[i]))
		case "--tag-contains":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --tag-contains")
			}
			i++
			opts.TagContains = append(opts.TagContains, strings.TrimSpace(strings.ToLower(args[i])))
		case "--exclude-tag":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --exclude-tag")
			}
			i++
			opts.ExcludeTags = append(opts.ExcludeTags, strings.TrimSpace(strings.ToLower(args[i])))
		case "--no-type":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --no-type")
			}
			i++
			opts.ExcludeItemType = strings.TrimSpace(args[i])
		case "--modified-within":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --modified-within")
			}
			i++
			opts.DateModifiedAfter = strings.TrimSpace(args[i])
		case "--added-since":
			if i+1 >= len(args) {
				return findParsedArgs{}, fmt.Errorf("missing value for --added-since")
			}
			i++
			opts.DateAddedAfter = strings.TrimSpace(args[i])
		case "--include-trashed":
			opts.IncludeTrashed = true
		default:
			queryProvided = true
			queryParts = append(queryParts, args[i])
		}
	}

	if len(opts.Tags) == 1 {
		opts.Tag = opts.Tags[0]
	}

	opts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	if opts.FullTextOnly && opts.MetadataOnly {
		return findParsedArgs{}, fmt.Errorf("cannot combine --fulltext-only and --metadata-only")
	}
	if opts.MetadataOnly && opts.FullTextAny {
		return findParsedArgs{}, fmt.Errorf("cannot combine --metadata-only and --fulltext-any")
	}
	return findParsedArgs{Opts: opts, JSONOutput: jsonOutput, Snippet: snippet, QueryProvided: queryProvided}, nil
}

func parseFindIncludeFields(value string) ([]string, error) {
	allowed := map[string]struct{}{
		"key":               {},
		"version":           {},
		"item_type":         {},
		"title":             {},
		"date":              {},
		"date_added":        {},
		"abstract":          {},
		"creators":          {},
		"container":         {},
		"volume":            {},
		"issue":             {},
		"pages":             {},
		"doi":               {},
		"url":               {},
		"tags":              {},
		"collections":       {},
		"attachments":       {},
		"notes":             {},
		"matched_on":        {},
		"full_text_preview": {},
	}

	parts := strings.Split(value, ",")
	fields := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		field := strings.TrimSpace(strings.ToLower(part))
		if field == "" {
			continue
		}
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("invalid value for --include-fields: %s", field)
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields, nil
}

type exportParsedArgs struct {
	ItemKey       string
	CollectionKey string
	FindOpts      zoteroapi.FindOptions
	FromFind      bool
	FindBackend   backend.FindOptions
	Format        string
	JSONOutput    bool
}

func parseExportArgs(args []string) (exportParsedArgs, error) {
	var itemKey string
	var collectionKey string
	var fromFind bool
	var format string
	var jsonOutput bool
	findOpts := zoteroapi.FindOptions{}
	queryParts := make([]string, 0, len(args))
	findArgs := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if fromFind {
			switch arg {
			case "--json":
				jsonOutput = true
			case "--format", "-f":
				if i+1 >= len(args) {
					return exportParsedArgs{}, fmt.Errorf("missing value for --format")
				}
				i++
				format = args[i]
			default:
				findArgs = append(findArgs, arg)
			}
			continue
		}

		switch arg {
		case "--json":
			jsonOutput = true
		case "--from-find":
			fromFind = true
		case "--item-key":
			if i+1 >= len(args) {
				return exportParsedArgs{}, fmt.Errorf("missing value for --item-key")
			}
			i++
			itemKey = args[i]
		case "--collection":
			if i+1 >= len(args) {
				return exportParsedArgs{}, fmt.Errorf("missing value for --collection")
			}
			i++
			collectionKey = args[i]
		case "--format", "-f":
			if i+1 >= len(args) {
				return exportParsedArgs{}, fmt.Errorf("missing value for --format")
			}
			i++
			format = args[i]
		case "--limit":
			if i+1 >= len(args) {
				return exportParsedArgs{}, fmt.Errorf("missing value for --limit")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return exportParsedArgs{}, fmt.Errorf("invalid value for --limit")
			}
			findOpts.Limit = limit
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	sourceCount := 0
	if itemKey != "" {
		sourceCount++
	}
	if collectionKey != "" {
		sourceCount++
	}
	if fromFind {
		sourceCount++
	}
	if len(queryParts) > 0 {
		sourceCount++
	}
	if sourceCount > 1 {
		return exportParsedArgs{}, fmt.Errorf("cannot combine query, --item-key, --collection, and --from-find")
	}

	var backendFindOpts backend.FindOptions
	if fromFind {
		parsedFind, err := parseFindArgs(findArgs)
		if err != nil {
			return exportParsedArgs{}, err
		}
		backendFindOpts = backend.NormalizeFindOptions(parsedFind.Opts)
		if !backendFindOpts.All && strings.TrimSpace(backendFindOpts.Query) == "" && !hasSubstantiveFilters(backendFindOpts) {
			return exportParsedArgs{}, fmt.Errorf("missing find query or filter after --from-find")
		}
	} else if itemKey == "" && collectionKey == "" {
		findOpts.Query = strings.TrimSpace(strings.Join(queryParts, " "))
		if findOpts.Query == "" {
			return exportParsedArgs{}, fmt.Errorf("missing query, --item-key, --collection, or --from-find")
		}
	}

	if format != "" && format != "bibtex" && format != "biblatex" && format != "csljson" && format != "ris" {
		return exportParsedArgs{}, fmt.Errorf("unsupported format")
	}

	return exportParsedArgs{
		ItemKey:       itemKey,
		CollectionKey: collectionKey,
		FindOpts:      findOpts,
		FromFind:      fromFind,
		FindBackend:   backendFindOpts,
		Format:        format,
		JSONOutput:    jsonOutput,
	}, nil
}

func (c *CLI) parseJSONOnlyArgs(args []string, usage string) (bool, bool, bool) {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--help" || arg == "-h" {
			fmt.Fprintln(c.stdout, usage)
			return false, true, true
		}
		fmt.Fprintln(c.stderr, usage)
		return false, false, false
	}
	return jsonOutput, true, false
}

func (c *CLI) parseJSONAndLimitArgs(args []string, usage string) (bool, int, bool, bool) {
	jsonOutput := false
	limit := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, "error: missing value for --limit")
				fmt.Fprintln(c.stderr, usage)
				return false, 0, false, false
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				fmt.Fprintln(c.stderr, "error: invalid value for --limit")
				fmt.Fprintln(c.stderr, usage)
				return false, 0, false, false
			}
			limit = n
			i++
		case "--help", "-h":
			fmt.Fprintln(c.stdout, usage)
			return false, 0, true, true
		default:
			fmt.Fprintln(c.stderr, usage)
			return false, 0, false, false
		}
	}
	return jsonOutput, limit, true, false
}

func (c *CLI) parseSingleValueCommand(args []string, usage string) (string, bool, bool, bool) {
	jsonOutput := false
	value := ""

	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--help" || arg == "-h" {
			fmt.Fprintln(c.stdout, usage)
			return "", false, true, true
		}
		if value == "" {
			value = arg
			continue
		}
		fmt.Fprintln(c.stderr, usage)
		return "", false, false, false
	}

	return value, jsonOutput, true, false
}
