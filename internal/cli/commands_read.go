package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/zoteroapi"
)

func hasSubstantiveFilters(opts backend.FindOptions) bool {
	return len(opts.Tags) > 0 ||
		len(opts.TagContains) > 0 ||
		len(opts.ExcludeTags) > 0 ||
		len(opts.Collection) > 0 ||
		len(opts.NoCollection) > 0 ||
		opts.DateAfter != "" ||
		opts.DateBefore != "" ||
		opts.ItemType != "" ||
		opts.ExcludeItemType != "" ||
		opts.HasPDF ||
		opts.AttachmentName != "" ||
		opts.AttachmentPath != "" ||
		opts.AttachmentType != "" ||
		opts.DateModifiedAfter != "" ||
		opts.DateAddedAfter != ""
}

func (c *CLI) runFind(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageFind)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}

	parsed, err := parseFindArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}
	opts := parsed.Opts
	opts = backend.NormalizeFindOptions(opts)
	jsonOutput := parsed.JSONOutput
	snippet := parsed.Snippet
	queryProvided := parsed.QueryProvided

	if opts.FullTextAny && !opts.FullText {
		fmt.Fprintln(c.stderr, "error: --fulltext-any requires --fulltext")
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}

	if strings.TrimSpace(opts.Query) == "" && !opts.All && !queryProvided && !hasSubstantiveFilters(opts) {
		fmt.Fprintln(c.stderr, usageFind)
		return 2
	}
	if hasSubstantiveFilters(opts) && strings.TrimSpace(opts.Query) == "" {
		opts.All = true
	}

	cfg, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if !opts.FullText && cfg.Mode != "web" {
		if hasFullTextData(reader) {
			opts.FullText = true
		}
	}

	requestedIncludeFields := append([]string(nil), opts.IncludeFields...)
	injectedAttachments := false
	if snippet && !fieldIncluded(opts.IncludeFields, "attachments") {
		opts.IncludeFields = append(opts.IncludeFields, "attachments")
		injectedAttachments = true
	}

	items, err := reader.FindItems(context.Background(), opts)
	if err != nil {
		return c.printErr(err)
	}
	items = filterDefaultFindItems(items, opts)
	for i := range items {
		enrichWithJournalRank(&items[i])
	}

	if snippet {
		if opts.Limit <= 0 {
			opts.Limit = 50
		}
		snippeter, ok := reader.(snippetReader)
		if !ok {
			return c.printErr(fmt.Errorf("find --snippet requires local or hybrid mode with local data"))
		}
		for i := range items {
			preview, err := snippeter.FullTextSnippet(context.Background(), items[i], opts.Query)
			if err != nil {
				return c.printErr(err)
			}
			items[i].FullTextPreview = preview
		}
		if injectedAttachments {
			for i := range items {
				items[i].Attachments = nil
			}
		}
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(items),
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "find",
			Data:    items,
			Meta:    meta,
		})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	if opts.Full || len(opts.IncludeFields) > 0 || snippet {
		renderOpts := opts
		renderOpts.IncludeFields = append([]string(nil), requestedIncludeFields...)
		if snippet && !fieldIncluded(renderOpts.IncludeFields, "full_text_preview") {
			renderOpts.IncludeFields = append(renderOpts.IncludeFields, "full_text_preview")
		}
		for index, item := range items {
			c.renderFindItemDetailed(item, renderOpts)
			if index < len(items)-1 {
				fmt.Fprintln(c.stdout)
			}
		}
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(c.stdout, "%-10s  %-16s  %-6s  %-18s  %s\n",
			item.Key,
			item.ItemType,
			shortDate(item.Date),
			shortCreators(item.Creators),
			item.Title,
		)
	}
	return 0
}

func (c *CLI) runStats(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageStats)
	}
	jsonOutput, ok, helpPrinted := c.parseJSONOnlyArgs(args, usageStats)
	if helpPrinted {
		return 0
	}
	if !ok {
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	stats, err := reader.GetLibraryStats(context.Background())
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{
			"total": stats.TotalItems,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{OK: true, Command: "stats", Data: stats, Meta: meta})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))
	fmt.Fprintf(c.stdout, "library=%s:%s\n", stats.LibraryType, stats.LibraryID)
	fmt.Fprintf(c.stdout, "items=%d\n", stats.TotalItems)
	fmt.Fprintf(c.stdout, "collections=%d\n", stats.TotalCollections)
	fmt.Fprintf(c.stdout, "searches=%d\n", stats.TotalSearches)
	if stats.LastLibraryVersion > 0 {
		fmt.Fprintf(c.stdout, "last_library_version=%d\n", stats.LastLibraryVersion)
	}
	return 0
}

func (c *CLI) runShow(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageShow)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	jsonOutput := false
	snippet := false
	key := ""
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--snippet" {
			snippet = true
			continue
		}
		if key == "" {
			key = arg
			continue
		}
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(c.stderr, usageShow)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	item, err := reader.GetItem(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	enrichWithJournalRank(&item)
	if snippet {
		previewer, ok := reader.(previewReader)
		if !ok {
			return c.printErr(fmt.Errorf("show --snippet requires local or hybrid mode with local data"))
		}
		preview, err := previewer.FullTextPreview(context.Background(), item)
		if err != nil {
			return c.printErr(err)
		}
		item.FullTextPreview = preview
	}

	if jsonOutput {
		meta := map[string]any{
			"total": 1,
		}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "show",
			Data:    item,
			Meta:    meta,
		})
	}
	readMeta := c.consumeReaderReadMetadata(reader)
	c.warnIfSnapshotRead(readMeta)

	fmt.Fprintf(c.stdout, "Key: %s\n", item.Key)
	fmt.Fprintf(c.stdout, "Title: %s\n", item.Title)
	fmt.Fprintf(c.stdout, "Type: %s\n", item.ItemType)
	if len(item.Creators) > 0 {
		fmt.Fprintf(c.stdout, "Creators: %s\n", joinCreatorNames(item.Creators))
	}
	if item.Date != "" {
		fmt.Fprintf(c.stdout, "Date: %s\n", item.Date)
	}
	if item.Container != "" {
		fmt.Fprintf(c.stdout, "Container: %s\n", item.Container)
	}
	if item.JournalRank != nil && len(item.JournalRank.Ranks) > 0 {
		c.renderJournalRank(item.JournalRank)
	}
	if item.Volume != "" {
		fmt.Fprintf(c.stdout, "Volume: %s\n", item.Volume)
	}
	if item.Issue != "" {
		fmt.Fprintf(c.stdout, "Issue: %s\n", item.Issue)
	}
	if item.Pages != "" {
		fmt.Fprintf(c.stdout, "Pages: %s\n", item.Pages)
	}
	if item.DOI != "" {
		fmt.Fprintf(c.stdout, "DOI: %s\n", item.DOI)
	}
	if item.URL != "" {
		fmt.Fprintf(c.stdout, "URL: %s\n", item.URL)
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(c.stdout, "Tags: %s\n", strings.Join(item.Tags, ", "))
	}
	if len(item.Collections) > 0 {
		fmt.Fprintf(c.stdout, "Collections: %s\n", joinCollectionNames(item.Collections))
	}
	if len(item.Attachments) > 0 {
		fmt.Fprintf(c.stdout, "Attachments: %d\n", len(item.Attachments))
		for _, attachment := range item.Attachments {
			fmt.Fprintf(c.stdout, "  - [%s] %s\n", attachmentKind(attachment), attachmentSummary(attachment))
			if pathLine := attachmentPathLine(attachment); pathLine != "" {
				fmt.Fprintf(c.stdout, "    %s\n", pathLine)
			}
		}
	}
	if len(item.Notes) > 0 {
		fmt.Fprintf(c.stdout, "Notes: %d\n", len(item.Notes))
		for _, note := range item.Notes {
			fmt.Fprintf(c.stdout, "  - %s\n", noteSummary(note))
		}
	}
	if len(item.Annotations) > 0 {
		fmt.Fprintf(c.stdout, "Annotations: %d\n", len(item.Annotations))
		for _, a := range item.Annotations {
			text := a.Text
			if len(text) > 60 {
				text = text[:60] + "..."
			}
			fmt.Fprintf(c.stdout, "  - [%s] color=%s page=%s: %s", a.Type, a.Color, a.PageLabel, text)
			if a.Comment != "" {
				fmt.Fprintf(c.stdout, " | %s", a.Comment)
			}
			fmt.Fprintln(c.stdout)
		}
	}
	if item.FullTextPreview != "" {
		fmt.Fprintf(c.stdout, "Full Text Preview: %s\n", item.FullTextPreview)
		if line := fullTextSourceLine(readMeta); line != "" {
			fmt.Fprintln(c.stdout, line)
		}
	}
	return 0
}

func (c *CLI) runRelate(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageRelate)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageRelate)
		return 2
	}

	jsonOutput := false
	aggregate := false
	predicate := ""
	addTarget := ""
	removeTarget := ""
	dryRun := false
	dotOutput := false
	fromFile := ""
	key := ""
	nextFlag := ""
	for _, arg := range args {
		if nextFlag != "" {
			switch nextFlag {
			case "predicate":
				predicate = arg
			case "add":
				addTarget = arg
			case "remove":
				removeTarget = arg
			case "from-file":
				fromFile = arg
			}
			nextFlag = ""
			continue
		}
		switch arg {
		case "--json":
			jsonOutput = true
		case "--aggregate":
			aggregate = true
		case "--predicate":
			nextFlag = "predicate"
		case "--add":
			nextFlag = "add"
		case "--remove":
			nextFlag = "remove"
		case "--dry-run", "-n":
			dryRun = true
		case "--dot":
			dotOutput = true
		case "--from-file":
			nextFlag = "from-file"
		default:
			if key == "" {
				key = arg
			} else {
				fmt.Fprintln(c.stderr, usageRelate)
				return 2
			}
		}
	}

	if fromFile != "" {
		return c.runRelateBatch(fromFile, dryRun, jsonOutput)
	}
	if addTarget != "" || removeTarget != "" {
		return c.runRelateWrite(key, addTarget, removeTarget, predicate, dryRun, jsonOutput)
	}
	if nextFlag != "" {
		fmt.Fprintf(c.stderr, "missing value for --%s\n", nextFlag)
		return 2
	}

	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(c.stderr, usageRelate)
		return 2
	}

	_, reader, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}

	if aggregate {
		if dotOutput {
			return c.runRelateDotAggregate(reader, key, predicate)
		}
		return c.runRelateAggregate(reader, key, predicate, jsonOutput)
	}

	relations, err := reader.GetRelated(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}

	if predicate != "" {
		relations = filterRelationsByPredicate(relations, predicate)
	}

	if dotOutput {
		return c.writeRelateDot(key, relations, nil)
	}

	if jsonOutput {
		meta := map[string]any{}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{OK: true, Command: "relate", Data: relations, Meta: meta})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	if len(relations) == 0 {
		fmt.Fprintf(c.stdout, "Item: %s\n", key)
		fmt.Fprintln(c.stdout, "Explicit Relations: 0")
		return 0
	}

	fmt.Fprintf(c.stdout, "Item: %s\n", key)
	fmt.Fprintf(c.stdout, "Explicit Relations: %d\n", len(relations))
	for _, relation := range relations {
		fmt.Fprintf(c.stdout, "  - [%s][%s] %s\n", relation.Predicate, relation.Direction, relateSummary(relation.Target))
	}
	return 0
}

func extractLocalReader(reader backend.Reader) (*backend.LocalReader, bool) {
	if lr, ok := reader.(*backend.LocalReader); ok {
		return lr, true
	}
	if hr, ok := reader.(*backend.HybridReader); ok {
		if lr := hr.LocalReader(); lr != nil {
			return lr, true
		}
	}
	return nil, false
}

func (c *CLI) runRelateAggregate(reader backend.Reader, key, predicate string, jsonOutput bool) int {
	localReader, ok := extractLocalReader(reader)
	if !ok {
		if jsonOutput {
			return c.writeJSON(jsonResponse{
				OK:      false,
				Command: "relate",
				Data:    nil,
				Meta:    map[string]any{"error": "--aggregate requires local or hybrid mode (ZOT_MODE=local or ZOT_MODE=hybrid)"},
			})
		}
		fmt.Fprintln(c.stderr, "--aggregate requires local or hybrid mode (set ZOT_MODE=local or ZOT_MODE=hybrid)")
		return 1
	}

	agg, err := localReader.GetRelatedAggregate(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}

	if predicate != "" {
		agg.Self = filterRelationsByPredicate(agg.Self, predicate)
		for i := range agg.Notes {
			agg.Notes[i].Relations = filterRelationsByPredicate(agg.Notes[i].Relations, predicate)
		}
	}

	if jsonOutput {
		meta := map[string]any{}
		c.appendReadMetadata(meta, reader)
		return c.writeJSON(jsonResponse{OK: true, Command: "relate", Data: agg, Meta: meta})
	}
	c.warnIfSnapshotRead(c.consumeReaderReadMetadata(reader))

	fmt.Fprintf(c.stdout, "Item: %s (aggregated)\n", key)

	fmt.Fprintf(c.stdout, "\nSelf Relations: %d\n", len(agg.Self))
	for _, rel := range agg.Self {
		fmt.Fprintf(c.stdout, "  - [%s][%s] %s\n", rel.Predicate, rel.Direction, relateSummary(rel.Target))
	}

	fmt.Fprintf(c.stdout, "\nNote Relations: %d\n", len(agg.Notes))
	for _, nr := range agg.Notes {
		fmt.Fprintf(c.stdout, "  Note: %s", nr.Source.Key)
		if nr.Preview != "" {
			preview := nr.Preview
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			fmt.Fprintf(c.stdout, " (%s)", preview)
		}
		fmt.Fprintln(c.stdout)
		for _, rel := range nr.Relations {
			fmt.Fprintf(c.stdout, "    - [%s][%s] %s\n", rel.Predicate, rel.Direction, relateSummary(rel.Target))
		}
	}

	if len(agg.Citations) > 0 {
		fmt.Fprintf(c.stdout, "\nEmbedded Citations: %d\n", len(agg.Citations))
		for _, cit := range agg.Citations {
			fmt.Fprintf(c.stdout, "  From %s:\n", cit.SourceKey)
			for _, t := range cit.Targets {
				fmt.Fprintf(c.stdout, "    - %s\n", relateSummary(t))
			}
		}
	}
	return 0
}

func (c *CLI) runRelateWrite(key, addTarget, removeTarget, predicate string, dryRun, jsonOutput bool) int {
	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(c.stderr, usageRelate)
		return 2
	}
	if addTarget != "" && removeTarget != "" {
		fmt.Fprintln(c.stderr, "cannot use --add and --remove together")
		return 2
	}
	if predicate == "" {
		predicate = "dc:relation"
	}

	cfg, _, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
		return exitCode
	}

	localReader, err := backend.NewLocalReader(cfg)
	if err != nil {
		return c.printErr(fmt.Errorf("local reader: %w", err))
	}

	ctx := context.Background()

	if dryRun || jsonOutput {
		action := "add"
		target := addTarget
		if removeTarget != "" {
			action = "remove"
			target = removeTarget
		}
		if jsonOutput {
			return c.writeJSON(jsonResponse{
				OK:      true,
				Command: "relate",
				Data: map[string]any{
					"dry_run":   dryRun,
					"action":    action,
					"source":    key,
					"target":    target,
					"predicate": predicate,
				},
			})
		}
		fmt.Fprintf(c.stdout, "[dry-run] would %s relation: %s --[%s]--> %s\n", action, key, predicate, target)
		return 0
	}

	if addTarget != "" {
		err = localReader.AddRelation(ctx, key, addTarget, predicate)
	} else {
		err = localReader.RemoveRelation(ctx, key, removeTarget, predicate)
	}
	if err != nil {
		return c.printErr(err)
	}

	action := "added"
	target := addTarget
	if removeTarget != "" {
		action = "removed"
		target = removeTarget
	}
	fmt.Fprintf(c.stdout, "%s relation: %s --[%s]--> %s\n", action, key, predicate, target)
	return 0
}

func (c *CLI) runRelateDotAggregate(reader backend.Reader, key, predicate string) int {
	localReader, ok := extractLocalReader(reader)
	if !ok {
		fmt.Fprintln(c.stderr, "--dot with --aggregate requires local or hybrid mode")
		return 1
	}
	agg, err := localReader.GetRelatedAggregate(context.Background(), key)
	if err != nil {
		return c.printErr(err)
	}
	if predicate != "" {
		agg.Self = filterRelationsByPredicate(agg.Self, predicate)
		for i := range agg.Notes {
			agg.Notes[i].Relations = filterRelationsByPredicate(agg.Notes[i].Relations, predicate)
		}
	}
	return c.writeRelateDot(key, agg.Self, agg)
}

func (c *CLI) writeRelateDot(key string, relations []domain.Relation, agg *domain.AggregatedRelations) int {
	w := c.stdout
	fmt.Fprintln(w, "digraph {")
	fmt.Fprintln(w, "\trankdir=LR;")
	fmt.Fprintln(w, "\tnode [fontname=\"Helvetica\"];")
	fmt.Fprintln(w, "\tedge [fontname=\"Helvetica\", fontsize=10];")

	dotLabel := func(ref domain.ItemRef) string {
		title := ref.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		title = strings.ReplaceAll(title, `"`, `\"`)
		if title == "" {
			title = ref.Key
		}
		return fmt.Sprintf("%s\\n[%s]", title, ref.Key)
	}

	emitNode := func(nodeKey string, label string, shape, fillcolor string) {
		fmt.Fprintf(w, "\t\"%s\" [label=%q, shape=%s, style=filled, fillcolor=%q];\n",
			nodeKey, label, shape, fillcolor)
	}

	emitEdge := func(from, to, label, color, style, dir string) {
		fmt.Fprintf(w, "\t\"%s\" -> \"%s\" [label=%q, color=%q, style=%s, dir=%s];\n",
			from, to, label, color, style, dir)
	}

	emitNode(key, dotLabel(domain.ItemRef{Key: key}), "box", "#4a90d9")

	if agg == nil {
		for _, r := range relations {
			dir := "both"
			if r.Direction == "outgoing" {
				dir = "forward"
			} else if r.Direction == "incoming" {
				dir = "back"
			}
			color := "#333333"
			switch r.Predicate {
			case "dc:relation":
				color = "#4a90d9"
			case "owl:sameAs":
				color = "#7bc96f"
			default:
				color = "#e8913a"
			}
			emitEdge(key, r.Target.Key, r.Predicate, color, "solid", dir)
			shape, fill := "box", "#f0f0f0"
			if r.Target.ItemType == "note" {
				shape, fill = "note", "#fff3e0"
			}
			emitNode(r.Target.Key, dotLabel(r.Target), shape, fill)
		}
	} else {
		for _, r := range agg.Self {
			dir := "both"
			if r.Direction == "outgoing" {
				dir = "forward"
			} else if r.Direction == "incoming" {
				dir = "back"
			}
			emitEdge(key, r.Target.Key, r.Predicate, "#4a90d9", "solid", dir)
			shape, fill := "box", "#f0f0f0"
			if r.Target.ItemType == "note" {
				shape, fill = "note", "#fff3e0"
			}
			emitNode(r.Target.Key, dotLabel(r.Target), shape, fill)
		}
		for _, n := range agg.Notes {
			emitNode(n.Source.Key, dotLabel(n.Source), "note", "#e8913a")
			emitEdge(key, n.Source.Key, "parent", "#999999", "dotted", "forward")
			for _, r := range n.Relations {
				dir := "both"
				if r.Direction == "outgoing" {
					dir = "forward"
				} else if r.Direction == "incoming" {
					dir = "back"
				}
				emitEdge(n.Source.Key, r.Target.Key, r.Predicate, "#e8913a", "solid", dir)
				shape, fill := "box", "#f0f0f0"
				if r.Target.ItemType == "note" {
					shape, fill = "note", "#fff3e0"
				}
				emitNode(r.Target.Key, dotLabel(r.Target), shape, fill)
			}
		}
		for _, cit := range agg.Citations {
			for _, t := range cit.Targets {
				emitEdge(cit.SourceKey, t.Key, "citation", "#7bc96f", "dashed", "forward")
				shape, fill := "box", "#f0f0f0"
				if t.ItemType == "note" {
					shape, fill = "note", "#fff3e0"
				}
				emitNode(t.Key, dotLabel(t), shape, fill)
			}
		}
	}

	fmt.Fprintln(w, "}")
	return 0
}

func (c *CLI) runRelateBatch(fromFile string, dryRun bool, jsonOutput bool) int {
	data, err := os.ReadFile(fromFile)
	if err != nil {
		return c.printErr(fmt.Errorf("read %s: %w", fromFile, err))
	}
	var ops []struct {
		Action    string `json:"action"`
		Source    string `json:"source"`
		Target    string `json:"target"`
		Predicate string `json:"predicate"`
	}
	if err := json.Unmarshal(data, &ops); err != nil {
		return c.printErr(fmt.Errorf("parse %s: %w (expected [{action,source,target,predicate}])", fromFile, err))
	}
	if len(ops) == 0 {
		fmt.Fprintln(c.stderr, "no operations in batch file")
		return 1
	}

	cfg, _, exitCode := c.loadReader()
	if exitCode != 0 {
		return exitCode
	}
	if !dryRun {
		if exitCode := c.ensureWriteAllowed(cfg); exitCode != 0 {
			return exitCode
		}
	}

	localReader, err := backend.NewLocalReader(cfg)
	if err != nil {
		return c.printErr(fmt.Errorf("local reader: %w", err))
	}

	ctx := context.Background()
	results := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		if op.Predicate == "" {
			op.Predicate = "dc:relation"
		}
		if op.Action == "" {
			op.Action = "add"
		}
		result := map[string]any{"source": op.Source, "target": op.Target, "predicate": op.Predicate, "action": op.Action}
		if dryRun || jsonOutput {
			result["dry_run"] = true
			results = append(results, result)
			continue
		}
		var opErr error
		switch op.Action {
		case "add":
			opErr = localReader.AddRelation(ctx, op.Source, op.Target, op.Predicate)
		case "remove":
			opErr = localReader.RemoveRelation(ctx, op.Source, op.Target, op.Predicate)
		default:
			opErr = fmt.Errorf("unknown action %q (use add or remove)", op.Action)
		}
		if opErr != nil {
			result["error"] = opErr.Error()
			results = append(results, result)
			continue
		}
		result["ok"] = true
		results = append(results, result)
	}

	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: true, Command: "relate", Data: results})
	}
	for _, r := range results {
		if errMsg, ok := r["error"]; ok {
			fmt.Fprintf(c.stdout, "FAIL [%s] %s --[%s]--> %s: %v\n", r["action"], r["source"], r["predicate"], r["target"], errMsg)
		} else if dr, ok := r["dry_run"]; ok && dr.(bool) {
			fmt.Fprintf(c.stdout, "[dry-run] would %s: %s --[%s]--> %s\n", r["action"], r["source"], r["predicate"], r["target"])
		} else {
			fmt.Fprintf(c.stdout, "%s: %s --[%s]--> %s\n", r["action"], r["source"], r["predicate"], r["target"])
		}
	}
	errCount := 0
	for _, r := range results {
		if _, ok := r["error"]; ok {
			errCount++
		}
	}
	fmt.Fprintf(c.stdout, "\n%d operations completed (%d ok, %d failed)\n", len(results), len(results)-errCount, errCount)
	return 0
}

func filterRelationsByPredicate(relations []domain.Relation, predicate string) []domain.Relation {
	if predicate == "" {
		return relations
	}
	filtered := make([]domain.Relation, 0, len(relations))
	for _, r := range relations {
		if r.Predicate == predicate {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (c *CLI) appendReadMetadata(meta map[string]any, reader backend.Reader) {
	c.appendExplicitReadMetadata(meta, c.consumeReaderReadMetadata(reader))
}

func (c *CLI) appendExplicitReadMetadata(meta map[string]any, readMeta backend.ReadMetadata) {
	if readMeta.ReadSource != "" {
		meta["read_source"] = readMeta.ReadSource
	}
	if readMeta.SQLiteFallback {
		meta["sqlite_fallback"] = true
	}
	if readMeta.FullTextEngine != "" {
		meta["full_text_engine"] = readMeta.FullTextEngine
	}
	if readMeta.FullTextSource != "" {
		meta["full_text_source"] = readMeta.FullTextSource
	}
	if readMeta.FullTextAttachmentKey != "" {
		meta["full_text_attachment_key"] = readMeta.FullTextAttachmentKey
	}
	if readMeta.FullTextCacheHit {
		meta["full_text_cache_hit"] = true
	}
}

func (c *CLI) consumeReaderReadMetadata(reader backend.Reader) backend.ReadMetadata {
	reporter, ok := reader.(interface{ ConsumeReadMetadata() backend.ReadMetadata })
	if !ok {
		return backend.ReadMetadata{}
	}
	return reporter.ConsumeReadMetadata()
}

func hasFullTextData(reader backend.Reader) bool {
	lr, ok := reader.(*backend.LocalReader)
	if !ok {
		return false
	}
	indexPath := filepath.Join(lr.FullTextCacheDir, "index.sqlite")
	info, err := os.Stat(indexPath)
	if err != nil {
		return false
	}
	return info.Size() > 4096
}

func (c *CLI) warnIfSnapshotRead(readMeta backend.ReadMetadata) {
	if readMeta.ReadSource != "snapshot" && !readMeta.SQLiteFallback {
		return
	}
	fmt.Fprintln(c.stderr, "note: using snapshot fallback for local Zotero data")
	if readMeta.SnapshotStale {
		fmt.Fprintln(c.stderr, "warning: snapshot may be stale (Zotero may have newer data)")
	}
}

func fullTextSourceLine(readMeta backend.ReadMetadata) string {
	if readMeta.FullTextSource == "" {
		return ""
	}
	line := "Full Text Source: " + readMeta.FullTextSource
	if readMeta.FullTextCacheHit {
		line += " (cache hit)"
	}
	if readMeta.FullTextAttachmentKey != "" {
		line += " [" + readMeta.FullTextAttachmentKey + "]"
	}
	return line
}

func enrichWithJournalRank(item *domain.Item) {
	if item.Container == "" {
		return
	}
	rank := backend.LookupJournalRank(item.Container)
	item.JournalRank = rank
}

func (c *CLI) renderJournalRank(rank *domain.JournalRank) {
	fmt.Fprintln(c.stdout, "Journal Rank:")
	priorityFields := []string{"sciif", "sci", "sciUp", "jci", "esi"}
	var extra []string
	for _, key := range priorityFields {
		if val, ok := rank.Ranks[key]; ok {
			label := fieldLabel(key)
			extra = append(extra, fmt.Sprintf("  %s: %s", label, val))
		}
	}
	for key, val := range rank.Ranks {
		skip := false
		for _, p := range priorityFields {
			if key == p {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		label := fieldLabel(key)
		extra = append(extra, fmt.Sprintf("  %s: %s", label, val))
	}
	for _, line := range extra {
		fmt.Fprintln(c.stdout, line)
	}
}

func fieldLabel(key string) string {
	labels := map[string]string{
		"sciif":             "SCI-IF",
		"sciif5":            "SCI-IF5",
		"sci":               "SCI-JCR",
		"ssci":              "SSCI",
		"jci":               "JCI",
		"esi":               "ESI",
		"eii":               "EI",
		"sciBase":           "中科院基础版",
		"sciUp":             "中科院升级版",
		"sciUpSmall":        "中科院小类",
		"sciUpTop":          "中科院TOP",
		"cscd":              "CSCD",
		"cssci":             "CSSCI",
		"pku":               "北大核心",
		"swjtu":             "西南交大",
		"sdufe":             "山东财经",
		"swufe":             "西南财经",
		"cufe":              "中央财经",
		"uibe":              "对外经贸",
		"nju":               "南京大学",
		"sjtu":              "上海交大",
		"fdu":               "复旦",
		"hhu":               "河海大学",
		"cug":               "中国地质",
		"zju":               "浙大",
		"xju":               "新疆大学",
		"xdu":               "西电",
		"ruc":               "人大",
		"xmu":               "厦大",
		"scu":               "川大",
		"cpu":               "中国药科",
		"cju":               "长江大学",
		"cqu":               "重庆大学",
		"ccf":               "CCF",
		"fms":               "FMS",
		"ajg":               "ABS",
		"zhongguokejihexin": "科技核心",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func relateSummary(ref domain.ItemRef) string {
	if ref.Title == "" {
		return ref.Key
	}
	if ref.ItemType == "" {
		return ref.Key + "  " + ref.Title
	}
	return ref.Key + "  " + ref.ItemType + "  " + ref.Title
}

func (c *CLI) runExport(args []string) int {
	if isHelpOnly(args) || containsHelp(args) {
		return c.printCommandUsage(usageExport)
	}
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, usageExport)
		return 2
	}

	exportParsed, err := parseExportArgs(args)
	if err != nil {
		fmt.Fprintln(c.stderr, "error:", err)
		fmt.Fprintln(c.stderr, usageExport)
		return 2
	}
	itemKey := exportParsed.ItemKey
	collectionKey := exportParsed.CollectionKey
	findOpts := exportParsed.FindOpts
	format := exportParsed.Format
	jsonOutput := exportParsed.JSONOutput

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	keys := make([]string, 0, 8)
	if format == "csljson" && cfg.Mode != "web" {
		result, readMeta, handled, err := c.tryLocalCSLJSONExport(context.Background(), cfg, itemKey, collectionKey, findOpts)
		if handled {
			if err != nil {
				return c.printErr(err)
			}
			if jsonOutput {
				meta := map[string]any{
					"total": len(result.Data.([]map[string]any)),
				}
				c.appendExplicitReadMetadata(meta, readMeta)
				return c.writeJSON(jsonResponse{
					OK:      true,
					Command: "export",
					Data:    result,
					Meta:    meta,
				})
			}
			c.warnIfSnapshotRead(readMeta)
			return c.writeJSON(jsonResponse{OK: true, Command: "export", Data: result.Data})
		}
	}

	cfg, client, exitCode := c.loadClient()
	if exitCode != 0 {
		return exitCode
	}

	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		items, err := client.ListCollectionItems(context.Background(), collectionKey, findOpts)
		if err != nil {
			return c.printErr(err)
		}
		items = filterDefaultFindItemsAPI(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	} else {
		items, err := client.FindItems(context.Background(), findOpts)
		if err != nil {
			return c.printErr(err)
		}
		items = filterDefaultFindItemsAPI(items, findOpts)
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}

	result, err := client.ExportItems(context.Background(), keys, zoteroapi.ExportOptions{
		Format: format,
		Style:  cfg.Style,
		Locale: cfg.Locale,
	})
	if err != nil {
		return c.printErr(err)
	}

	if jsonOutput {
		meta := map[string]any{
			"total": len(keys),
		}
		c.appendExplicitReadMetadata(meta, backend.ReadMetadata{ReadSource: "web"})
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "export",
			Data:    result,
			Meta:    meta,
		})
	}

	if result.Text != "" {
		fmt.Fprintln(c.stdout, result.Text)
		return 0
	}
	return c.writeJSON(jsonResponse{OK: true, Command: "export", Data: result.Data})
}

func (c *CLI) tryLocalCSLJSONExport(ctx context.Context, cfg config.Config, itemKey string, collectionKey string, findOpts zoteroapi.FindOptions) (zoteroapi.ExportResult, backend.ReadMetadata, bool, error) {
	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
	}

	keys := make([]string, 0, 8)
	if itemKey != "" {
		keys = append(keys, itemKey)
	} else if collectionKey != "" {
		collectionReader, ok := localReader.(collectionItemKeyReader)
		if !ok {
			if cfg.Mode == "hybrid" {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, fmt.Errorf("local export requires collection access support")
		}
		keys, err = collectionReader.CollectionItemKeys(ctx, collectionKey, findOpts.Limit)
		if err != nil {
			if cfg.Mode == "hybrid" && shouldFallbackLocalCSLJSONExport(err) {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
		}
	} else {
		items, err := localReader.FindItems(ctx, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		if err != nil {
			if cfg.Mode == "hybrid" && shouldFallbackLocalCSLJSONExport(err) {
				return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
			}
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
		}
		items = filterDefaultFindItems(items, backend.FindOptions{
			Query: findOpts.Query,
			Limit: findOpts.Limit,
		})
		for _, item := range items {
			keys = append(keys, item.Key)
		}
	}

	exporter, ok := localReader.(cslJSONExporter)
	if !ok {
		if cfg.Mode == "hybrid" {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, fmt.Errorf("local export requires CSL JSON export support")
	}
	payload, err := exporter.ExportItemsCSLJSON(ctx, keys)
	if err != nil {
		if cfg.Mode == "hybrid" && shouldFallbackLocalCSLJSONExport(err) {
			return zoteroapi.ExportResult{}, backend.ReadMetadata{}, false, nil
		}
		return zoteroapi.ExportResult{}, backend.ReadMetadata{}, true, err
	}
	return zoteroapi.ExportResult{
		Format: "csljson",
		Data:   payload,
	}, c.consumeReaderReadMetadata(localReader), true, nil
}

func shouldFallbackLocalCSLJSONExport(err error) bool {
	return errors.Is(err, backend.ErrItemNotFound) ||
		errors.Is(err, backend.ErrUnsupportedFeature) ||
		errors.Is(err, backend.ErrLocalTemporarilyUnavailable)
}
