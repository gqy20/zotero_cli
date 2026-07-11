package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

const (
	usageFind = `usage: zot find <query> [filters...] [output...] | zot find --all [filters...] [output...]

What: Search items in the library. local/hybrid mode auto-enables FTS5 (PDF
full-text). Source must be one of: a query, --all, or at least one filter
(with no source at all the command errors out). Filters alone implicitly
turn on --all.

Filters (AND across categories, OR within --tag-any):
  --tag TAG              Repeat for AND. --tag-any switches to OR.
  --tag-contains W       Substring match against tag names.
  --exclude-tag TAG      Exclude tag.
  --collection KEY       Restrict to one collection (repeat for OR).
  --no-collection KEY    Exclude these collections.
  --item-type TYPE       book | journalArticle | ... (see 'zot schema types').
  --no-type TYPE         Exclude item types.
  --date-after / --date-before   YYYY[-MM[-DD]] bounds on dateAdded.
  --modified-within / --added-since   Duration: 24h | 7d | 30d.
  --has-pdf              Only items with PDF attachments.
  --attachment-name / --attachment-path / --attachment-type   Attachment match.
  --missing-attachment   Items whose attachment files are unresolved or missing.
  --bad-attachment-name  Items with generic/unsafe attachment filenames.
  --attachment-health LEVEL   Items with attachment issues at LEVEL or worse
                          (critical | error | warning | info).
  --include-trashed      Include items in trash.

Output:
  --json                 Structured for agents (strongly recommended).
  --full                 Include abstract/notes/tags.
  --include-fields F[,F] Restrict to specific fields.
  --snippet              Add FTS5 match-snippet preview (local/hybrid + index).
  --fulltext             Merge metadata and PDF full-text matches.
  --fulltext-only        Match only PDF text (local/hybrid + FTS5).
  --metadata-only        Match only title/creator/tag/other metadata.
  --fulltext-any         OR semantics for --fulltext terms.
  --qmode MODE           titleCreatorYear (default) | everything (adds abstract+tags).
  --limit N              Max results. No default unless --snippet is set (then 50).
  --sort FIELD           relevance | dateAdded | title | creator | year. --direction asc|desc.

Examples:
  zot find "CRISPR" --json                              # Basic search
  zot find --tag ai --tag ml --json                     # AND-tag filter
  zot find "attention" --fulltext --snippet --limit 20 --json  # merged search
  zot find "attention" --fulltext-only --json                  # PDF text only
  zot find --all --has-pdf --json                       # All PDFs
  zot find --missing-attachment --json                  # Broken local attachments

Notes:
  - Metadata + FTS5 merging is automatic in local/hybrid when the index exists.
    Use --metadata-only or --fulltext-only to select one source explicitly.
  - --snippet / --fulltext-only require a local FTS5 index.
  - See also: export (write results), show <key> (item detail), schema types.`

	usageShow = `usage: zot show <item-key> [--json] [--full] [--snippet]

What: Show one item's metadata, child notes, annotations, and (if present)
journal-tier info. Single positional argument; the rest are flags.

Output:
  (default)    Title, creators, year, tags, collections, child notes count.
  --full       Include abstract, full note bodies, all child annotations,
               journal-tier codes (sciUp/pku/sjtu/...).
  --json       Structured output for agents.
  --snippet    Use FTS5 snippet context for note bodies (local/hybrid + index).

Examples:
  zot show ABCD
  zot show ABCD --json
  zot show ABCD --full --json

Notes:
  - Multiple item keys are NOT accepted. Use 'zot abstract KEY1 KEY2 ...'
    for batch abstract lookup, or call 'zot show' in a loop.
  - See also: abstract, find, annotations.`

	usageRelate = `usage: zot relate <item-key> [--json] [--aggregate] [--dot] [--predicate PRED] [--add TARGET] [--remove TARGET] [--dry-run] [--from-file PATH]

What: Read or modify explicit item relations. By default returns the relations
declared via the Zotero itemRelations field. Use --aggregate to expand the
neighborhood across three layers:

  Layer 1    Explicit itemRelations  (author-declared, always present)
  Layer 2    Notes attached to the item that link to other items
  Layer 3    Citations inside the note HTML (Zotero citation links)

Read flags:
  --json             Structured output (recommended for agents).
  --aggregate        Expand across layers 1-3. local/hybrid only.
  --predicate PRED   Restrict to one relation predicate (e.g. 'dc:replaces').
  --dot              Emit Graphviz DOT instead of JSON (pipe to 'dot -Tpng').

Write flags (modify itemRelations on the source item):
  --add TARGET       Add a relation to TARGET.
  --remove TARGET    Remove the relation to TARGET.
  --dry-run          Preview changes without applying.
  --from-file PATH   Batch operations from a JSON file
                     {"add":[{...}],"remove":[{...}]}.

Examples:
  zot relate ABCD --json                              # All relations
  zot relate ABCD --predicate dc:replaces --json      # Filter by predicate
  zot relate ABCD --aggregate --json                  # Three-layer expansion
  zot relate ABCD --dot > net.dot                     # Graphviz export
  zot relate ABCD --add EFGH --dry-run                # Preview addition
  zot relate ABCD --from-file ops.json --dry-run      # Batch preview

Notes:
  - --aggregate / --add / --remove / --from-file require local/hybrid mode.
    In web/remote mode only the basic relation read is supported.
  - Write operations require ZOT_ALLOW_WRITE=1 in env.
  - See also: find, show <key> (inspect related items).`
	usageExport = `usage: zot export <query> [--limit N] [--format FMT] [--json]
       | zot export --item-key KEY [--format FMT] [--json]
       | zot export --collection KEY [--format FMT] [--json]
       | zot export --from-find <find-args...> [--format FMT] [--json]
       | zot export --all [--format FMT] [--json]

What: Export bibliographic entries in the chosen format. By default emits the
formatted text on stdout. With --json, the response is wrapped in the standard
JSON envelope ({"ok":true,"command":"export","data":...}).

Source (pick exactly one):
  <query>                 Reuse 'zot find' query syntax — same filters apply.
  --item-key KEY          Single item.
  --collection KEY        All items in a collection.
  --from-find ...         Resolve keys using the full zot find parser first.
  --all                   Entire library.

Format (--format / -f):
  csljson    (default)    Citation Style Language JSON — structured, agent-friendly.
  bibtex                  Classic BibTeX.
  biblatex                BibLaTeX.
  ris                     RIS (RefMan) — useful for EndNote/Zotero import.

Examples:
  zot export "CRISPR" --format bibtex                         # stdout text
  zot export --item-key ABCD --format csljson --json          # JSON envelope
  zot export --collection COLL1 --format ris > refs.ris
  zot export --from-find "CRISPR" --tag review --format bibtex
  zot export --from-find --has-pdf --limit 20 --format csljson --json
  zot export --all --format csljson --json | jq '.data[]'     # Pipeline to jq

Notes:
  - Exactly one source mode: query, --item-key, --collection, --from-find, or --all.
    Combining any two is rejected.
  - --json wraps the formatted text in the agent envelope; the text itself
    stays plain (decodable by the matching importer).
  - See also: find, show <key>.`
	usageStats = "usage: zot stats [--json]\n\nLibrary counters: items, collections, saved searches, attachments, and notes. Single-call snapshot for agents that need a quick size check."
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
		opts.MissingAttachment ||
		opts.BadAttachmentName ||
		opts.AttachmentName != "" ||
		opts.AttachmentPath != "" ||
		opts.AttachmentType != "" ||
		opts.AttachmentHealth != "" ||
		opts.DateModifiedAfter != "" ||
		opts.DateAddedAfter != ""
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
