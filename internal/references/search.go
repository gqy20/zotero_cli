package references

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type SearchOptions struct {
	Query, In, Field, Section, Source, Target string
	Limit                                     int
}
type SearchHit struct {
	SourceItemKey string         `json:"source_item_key"`
	SourceTitle   string         `json:"source_title"`
	Reference     Reference      `json:"reference"`
	Contexts      []Context      `json:"contexts,omitempty"`
	MatchedOn     []string       `json:"matched_on"`
	Score         float64        `json:"score"`
	Metadata      PubMedMetadata `json:"metadata,omitempty"`
	Annotation    *Annotation    `json:"annotation,omitempty"`
}

func referenceFTSQuery(q string) string {
	return strings.TrimSpace(q)
}

func (s *Store) Search(ctx context.Context, opts SearchOptions) ([]SearchHit, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.In == "" {
		opts.In = "all"
	}
	q := referenceFTSQuery(opts.Query)
	if q == "" {
		return nil, fmt.Errorf("empty reference search query")
	}
	hits := map[string]*SearchHit{}
	order := []string{}
	add := func(hit SearchHit, matched string) {
		key := fmt.Sprintf("%s:%d", hit.SourceItemKey, hit.Reference.Index)
		if existing, ok := hits[key]; ok {
			if matched == "context" {
				existing.Contexts = append(existing.Contexts, hit.Contexts...)
			}
			if !containsString(existing.MatchedOn, matched) {
				existing.MatchedOn = append(existing.MatchedOn, matched)
			}
			if hit.Score < existing.Score {
				existing.Score = hit.Score
			}
			return
		}
		hit.MatchedOn = []string{matched}
		hits[key] = &hit
		order = append(order, key)
	}
	if opts.In == "all" || opts.In == "references" {
		rows, err := s.searchReferenceRows(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, h := range rows {
			add(h, "reference")
		}
	}
	if opts.In == "all" || opts.In == "contexts" {
		rows, err := s.searchContextRows(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, h := range rows {
			add(h, "context")
		}
	}
	if (opts.In == "all" || opts.In == "metadata") && opts.Field != "annotations" {
		rows, err := s.searchMetadataRows(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, h := range rows {
			add(h, "metadata")
		}
	}
	if opts.Field == "annotations" {
		rows, err := s.searchAnnotationRows(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, h := range rows {
			add(h, "annotation")
		}
	}
	out := make([]SearchHit, 0, len(order))
	for _, key := range order {
		out = append(out, *hits[key])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score < out[j].Score })
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (s *Store) searchAnnotationRows(ctx context.Context, q string, o SearchOptions) ([]SearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.item_key,COALESCE(i.title,''),a.provider,a.annotation_type,a.entity,a.label,a.section,a.exact_text,bm25(ref_annotations_fts) FROM ref_annotations_fts JOIN ref_annotations a ON a.item_key=ref_annotations_fts.item_key AND a.ordinal=ref_annotations_fts.ordinal LEFT JOIN ref_items i ON i.item_key=a.item_key WHERE ref_annotations_fts MATCH ? ORDER BY bm25(ref_annotations_fts) LIMIT ?`, q, o.Limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var a Annotation
		if err := rows.Scan(&h.SourceItemKey, &h.SourceTitle, &a.Provider, &a.Type, &a.Entity, &a.Label, &a.Section, &a.Exact, &h.Score); err != nil {
			return nil, err
		}
		h.Reference = Reference{Title: h.SourceTitle}
		h.Annotation = &a
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) searchMetadataRows(ctx context.Context, q string, o SearchOptions) ([]SearchHit, error) {
	match := q
	if o.Field != "" {
		match = o.Field + ":(" + q + ")"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT i.item_key,i.title,i.pubmed_metadata_json,bm25(ref_metadata_fts) FROM ref_metadata_fts JOIN ref_items i ON i.item_key=ref_metadata_fts.item_key WHERE ref_metadata_fts MATCH ? ORDER BY bm25(ref_metadata_fts) LIMIT ?`, match, o.Limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var metadata string
		if err := rows.Scan(&h.SourceItemKey, &h.SourceTitle, &metadata, &h.Score); err != nil {
			return nil, err
		}
		h.Reference = Reference{Title: h.SourceTitle}
		_ = json.Unmarshal([]byte(metadata), &h.Metadata)
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Store) searchReferenceRows(ctx context.Context, q string, o SearchOptions) ([]SearchHit, error) {
	query := `SELECT e.source_item_key,i.title,e.ref_index,e.ref_id,e.raw,e.title,e.authors_json,e.container,e.year,e.volume,e.issue,e.pages,e.doi,e.pmid,e.pmcid,e.source,COALESCE(e.target_item_key,''),COALESCE(e.match_method,''),e.match_score,COALESCE(e.match_status,''),e.context_status,e.context_count,bm25(ref_entries_fts) FROM ref_entries_fts JOIN ref_entries e ON e.source_item_key=ref_entries_fts.source_item_key AND e.ref_index=ref_entries_fts.ref_index JOIN ref_items i ON i.item_key=e.source_item_key WHERE ref_entries_fts MATCH ?`
	args := []any{q}
	query, args = applySearchFilters(query, args, o, "e")
	query += ` ORDER BY bm25(ref_entries_fts) LIMIT ?`
	args = append(args, o.Limit*3)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		h, err := scanSearchHit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Store) searchContextRows(ctx context.Context, q string, o SearchOptions) ([]SearchHit, error) {
	query := `SELECT e.source_item_key,i.title,e.ref_index,e.ref_id,e.raw,e.title,e.authors_json,e.container,e.year,e.volume,e.issue,e.pages,e.doi,e.pmid,e.pmcid,e.source,COALESCE(e.target_item_key,''),COALESCE(e.match_method,''),e.match_score,COALESCE(e.match_status,''),e.context_status,e.context_count,bm25(ref_contexts_fts),c.reference_id,c.reference_index,c.marker,c.section,c.paragraph,COALESCE(c.target_item_key,''),c.source FROM ref_contexts_fts JOIN ref_contexts c ON c.source_item_key=ref_contexts_fts.source_item_key AND c.ordinal=ref_contexts_fts.ordinal JOIN ref_entries e ON e.source_item_key=c.source_item_key AND e.ref_index=c.reference_index JOIN ref_items i ON i.item_key=e.source_item_key WHERE ref_contexts_fts MATCH ?`
	args := []any{q}
	query, args = applySearchFilters(query, args, o, "e")
	if o.Section != "" {
		query += ` AND c.section LIKE ?`
		args = append(args, "%"+o.Section+"%")
	}
	query += ` ORDER BY bm25(ref_contexts_fts) LIMIT ?`
	args = append(args, o.Limit*5)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		h, err := scanSearchHitWithContext(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func applySearchFilters(query string, args []any, o SearchOptions, alias string) (string, []any) {
	if o.Source != "" {
		query += ` AND ` + alias + `.source=?`
		args = append(args, o.Source)
	}
	if o.Target != "" {
		query += ` AND ` + alias + `.target_item_key=?`
		args = append(args, o.Target)
	}
	return query, args
}

type rowScanner interface{ Scan(...any) error }

func scanSearchHit(row rowScanner) (SearchHit, error) {
	var h SearchHit
	var authors, source string
	err := row.Scan(&h.SourceItemKey, &h.SourceTitle, &h.Reference.Index, &h.Reference.ID, &h.Reference.Raw, &h.Reference.Title, &authors, &h.Reference.Container, &h.Reference.Year, &h.Reference.Volume, &h.Reference.Issue, &h.Reference.Pages, &h.Reference.DOI, &h.Reference.PMID, &h.Reference.PMCID, &source, &h.Reference.TargetItemKey, &h.Reference.MatchMethod, &h.Reference.MatchScore, &h.Reference.MatchStatus, &h.Reference.ContextStatus, &h.Reference.ContextCount, &h.Score)
	if err != nil {
		return h, err
	}
	_ = json.Unmarshal([]byte(authors), &h.Reference.Authors)
	h.Reference.Source = Source(source)
	return h, nil
}
func scanSearchHitWithContext(row *sql.Rows) (SearchHit, error) {
	var h SearchHit
	var authors, source, contextSource string
	var c Context
	err := row.Scan(&h.SourceItemKey, &h.SourceTitle, &h.Reference.Index, &h.Reference.ID, &h.Reference.Raw, &h.Reference.Title, &authors, &h.Reference.Container, &h.Reference.Year, &h.Reference.Volume, &h.Reference.Issue, &h.Reference.Pages, &h.Reference.DOI, &h.Reference.PMID, &h.Reference.PMCID, &source, &h.Reference.TargetItemKey, &h.Reference.MatchMethod, &h.Reference.MatchScore, &h.Reference.MatchStatus, &h.Reference.ContextStatus, &h.Reference.ContextCount, &h.Score, &c.ReferenceID, &c.ReferenceIndex, &c.Marker, &c.Section, &c.Paragraph, &c.TargetItemKey, &contextSource)
	if err != nil {
		return h, err
	}
	_ = json.Unmarshal([]byte(authors), &h.Reference.Authors)
	h.Reference.Source = Source(source)
	c.Source = Source(contextSource)
	h.Contexts = []Context{c}
	return h, nil
}
func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
