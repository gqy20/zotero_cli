package references

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	path string
	db   *sql.DB
}

type Status struct {
	IndexedItems         int    `json:"indexed_items"`
	SuccessfulItems      int    `json:"successful_items"`
	FailedItems          int    `json:"failed_items"`
	TotalReferences      int    `json:"total_references"`
	PMCItems             int    `json:"pmc_items"`
	PubMedItems          int    `json:"pubmed_items"`
	ResolvedReferences   int    `json:"resolved_references"`
	UnresolvedReferences int    `json:"unresolved_references"`
	CitationContexts     int    `json:"citation_contexts"`
	LastIndexedAt        string `json:"last_indexed_at,omitempty"`
}

type CitedBy struct {
	SourceItemKey string    `json:"source_item_key"`
	SourceTitle   string    `json:"source_title"`
	Reference     Reference `json:"reference"`
	Contexts      []Context `json:"contexts,omitempty"`
}

type FailedItem struct {
	ItemKey   string `json:"item_key"`
	Title     string `json:"title"`
	Error     string `json:"error"`
	Attempts  int    `json:"attempts"`
	UpdatedAt string `json:"updated_at"`
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{path: path, db: db}
	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) init() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS ref_items (
  item_key TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  doi TEXT, pmid TEXT, pmcid TEXT,
  strategy TEXT,
  status TEXT NOT NULL,
  reference_count INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  attempts INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS ref_entries (
  source_item_key TEXT NOT NULL,
  ref_index INTEGER NOT NULL,
  ref_id TEXT, raw TEXT, title TEXT,
  authors_json TEXT, container TEXT, year TEXT,
  volume TEXT, issue TEXT, pages TEXT,
  doi TEXT, pmid TEXT, pmcid TEXT, source TEXT NOT NULL,
	  target_item_key TEXT, match_method TEXT, match_score REAL NOT NULL DEFAULT 0, match_status TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(source_item_key, ref_index),
  FOREIGN KEY(source_item_key) REFERENCES ref_items(item_key) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS ref_contexts (
  source_item_key TEXT NOT NULL, ordinal INTEGER NOT NULL,
  reference_id TEXT, reference_index INTEGER, marker TEXT, section TEXT, paragraph TEXT NOT NULL,
  target_item_key TEXT, source TEXT NOT NULL,
  PRIMARY KEY(source_item_key, ordinal),
  FOREIGN KEY(source_item_key) REFERENCES ref_items(item_key) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ref_entries_doi ON ref_entries(doi);
CREATE INDEX IF NOT EXISTS idx_ref_entries_pmid ON ref_entries(pmid);
CREATE INDEX IF NOT EXISTS idx_ref_items_status ON ref_items(status);
CREATE INDEX IF NOT EXISTS idx_ref_contexts_target ON ref_contexts(target_item_key);
`)
	if err != nil {
		return err
	}
	// Upgrade indexes created by versions predating local citation resolution.
	for _, alter := range []string{
		`ALTER TABLE ref_entries ADD COLUMN target_item_key TEXT`, `ALTER TABLE ref_entries ADD COLUMN match_method TEXT`,
		`ALTER TABLE ref_entries ADD COLUMN match_score REAL NOT NULL DEFAULT 0`, `ALTER TABLE ref_entries ADD COLUMN match_status TEXT NOT NULL DEFAULT ''`,
	} {
		if _, e := s.db.Exec(alter); e != nil && !strings.Contains(strings.ToLower(e.Error()), "duplicate column") {
			return e
		}
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ref_entries_target ON ref_entries(target_item_key)`)
	return err
}

func (s *Store) IsFresh(ctx context.Context, itemKey, fingerprint string) (bool, error) {
	var stored, status string
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint, status FROM ref_items WHERE item_key=?`, itemKey).Scan(&stored, &status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return stored == fingerprint && status == "success", nil
}

func (s *Store) SaveResult(ctx context.Context, result Result, fingerprint string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, `INSERT INTO ref_items(item_key,title,fingerprint,doi,pmid,pmcid,strategy,status,reference_count,error,attempts,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,'',1,?) ON CONFLICT(item_key) DO UPDATE SET title=excluded.title,fingerprint=excluded.fingerprint,doi=excluded.doi,pmid=excluded.pmid,pmcid=excluded.pmcid,strategy=excluded.strategy,status='success',reference_count=excluded.reference_count,error='',attempts=ref_items.attempts+1,updated_at=excluded.updated_at`,
		result.ItemKey, result.ItemTitle, fingerprint, result.Identifiers.DOI, result.Identifiers.PMID, result.Identifiers.PMCID, result.Strategy, "success", len(result.References), now)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_entries WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_contexts WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ref_entries(source_item_key,ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source,target_item_key,match_method,match_score,match_status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ref := range result.References {
		authors, _ := json.Marshal(ref.Authors)
		if _, err = stmt.ExecContext(ctx, result.ItemKey, ref.Index, ref.ID, ref.Raw, ref.Title, string(authors), ref.Container, ref.Year, ref.Volume, ref.Issue, ref.Pages, ref.DOI, ref.PMID, ref.PMCID, ref.Source, ref.TargetItemKey, ref.MatchMethod, ref.MatchScore, ref.MatchStatus); err != nil {
			return err
		}
	}
	contextStmt, err := tx.PrepareContext(ctx, `INSERT INTO ref_contexts(source_item_key,ordinal,reference_id,reference_index,marker,section,paragraph,target_item_key,source) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer contextStmt.Close()
	for i, c := range result.Contexts {
		if _, err = contextStmt.ExecContext(ctx, result.ItemKey, i+1, c.ReferenceID, c.ReferenceIndex, c.Marker, c.Section, c.Paragraph, c.TargetItemKey, c.Source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SaveFailure(ctx context.Context, itemKey, title, fingerprint string, cause error) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO ref_items(item_key,title,fingerprint,status,error,attempts,updated_at) VALUES(?,?,?,'failed',?,1,?)
ON CONFLICT(item_key) DO UPDATE SET title=excluded.title,fingerprint=excluded.fingerprint,status='failed',error=excluded.error,attempts=ref_items.attempts+1,updated_at=excluded.updated_at`, itemKey, title, fingerprint, cause.Error(), now)
	return err
}

func (s *Store) LoadResult(ctx context.Context, itemKey string) (Result, bool, error) {
	var result Result
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT item_key,title,doi,pmid,pmcid,strategy,status FROM ref_items WHERE item_key=?`, itemKey).Scan(&result.ItemKey, &result.ItemTitle, &result.Identifiers.DOI, &result.Identifiers.PMID, &result.Identifiers.PMCID, &result.Strategy, &status)
	if err == sql.ErrNoRows {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if status != "success" {
		return Result{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source,target_item_key,match_method,match_score,match_status FROM ref_entries WHERE source_item_key=? ORDER BY ref_index`, itemKey)
	if err != nil {
		return Result{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref Reference
		var authors, source string
		if err := rows.Scan(&ref.Index, &ref.ID, &ref.Raw, &ref.Title, &authors, &ref.Container, &ref.Year, &ref.Volume, &ref.Issue, &ref.Pages, &ref.DOI, &ref.PMID, &ref.PMCID, &source, &ref.TargetItemKey, &ref.MatchMethod, &ref.MatchScore, &ref.MatchStatus); err != nil {
			return Result{}, false, err
		}
		_ = json.Unmarshal([]byte(authors), &ref.Authors)
		ref.Source = Source(source)
		result.References = append(result.References, ref)
	}
	if err := rows.Err(); err != nil {
		return Result{}, false, err
	}
	rows.Close()
	result.Contexts, err = s.Contexts(ctx, itemKey)
	return result, true, err
}

func (s *Store) Status(ctx context.Context) (Status, error) {
	var status Status
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(status='success'),0),COALESCE(SUM(status='failed'),0),COALESCE(SUM(reference_count),0),COALESCE(SUM(strategy='pmc_jats' AND status='success'),0),COALESCE(SUM(strategy='pubmed' AND status='success'),0),COALESCE(MAX(updated_at),'') FROM ref_items`).Scan(&status.IndexedItems, &status.SuccessfulItems, &status.FailedItems, &status.TotalReferences, &status.PMCItems, &status.PubMedItems, &status.LastIndexedAt)
	if err == nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(match_status='resolved'),0),COALESCE(SUM(match_status!='resolved'),0) FROM ref_entries`).Scan(&status.ResolvedReferences, &status.UnresolvedReferences)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ref_contexts`).Scan(&status.CitationContexts)
	}
	return status, err
}

func (s *Store) Resolve(ctx context.Context, resolver *Resolver) (ResolveReport, error) {
	start := time.Now()
	rows, err := s.db.QueryContext(ctx, `SELECT source_item_key,ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source FROM ref_entries`)
	if err != nil {
		return ResolveReport{}, err
	}
	type entry struct {
		source string
		ref    Reference
	}
	var entries []entry
	for rows.Next() {
		var e entry
		var authors, source string
		if err = rows.Scan(&e.source, &e.ref.Index, &e.ref.ID, &e.ref.Raw, &e.ref.Title, &authors, &e.ref.Container, &e.ref.Year, &e.ref.Volume, &e.ref.Issue, &e.ref.Pages, &e.ref.DOI, &e.ref.PMID, &e.ref.PMCID, &source); err != nil {
			rows.Close()
			return ResolveReport{}, err
		}
		_ = json.Unmarshal([]byte(authors), &e.ref.Authors)
		e.ref.Source = Source(source)
		entries = append(entries, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ResolveReport{}, err
	}
	report := ResolveReport{Total: len(entries)}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `UPDATE ref_entries SET target_item_key=?,match_method=?,match_score=?,match_status=? WHERE source_item_key=? AND ref_index=?`)
	if err != nil {
		return report, err
	}
	defer stmt.Close()
	for _, e := range entries {
		ref := resolver.Resolve(e.ref, e.source)
		if ref.MatchStatus == "resolved" {
			report.Resolved++
			switch ref.MatchMethod {
			case "doi":
				report.DOI++
			case "pmid":
				report.PMID++
			case "title_exact":
				report.ExactTitle++
			case "title_fuzzy":
				report.FuzzyTitle++
			}
		} else {
			report.Unresolved++
		}
		if _, err = stmt.ExecContext(ctx, ref.TargetItemKey, ref.MatchMethod, ref.MatchScore, ref.MatchStatus, e.source, ref.Index); err != nil {
			return report, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ref_contexts SET target_item_key=COALESCE((SELECT target_item_key FROM ref_entries e WHERE e.source_item_key=ref_contexts.source_item_key AND e.ref_index=ref_contexts.reference_index),'')`); err != nil {
		return report, err
	}
	if err = tx.Commit(); err != nil {
		return report, err
	}
	report.ElapsedMS = time.Since(start).Milliseconds()
	return report, nil
}

func (s *Store) Contexts(ctx context.Context, itemKey string) ([]Context, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reference_id,reference_index,marker,section,paragraph,target_item_key,source FROM ref_contexts WHERE source_item_key=? ORDER BY ordinal`, itemKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Context
	for rows.Next() {
		var c Context
		var source string
		if err := rows.Scan(&c.ReferenceID, &c.ReferenceIndex, &c.Marker, &c.Section, &c.Paragraph, &c.TargetItemKey, &source); err != nil {
			return nil, err
		}
		c.Source = Source(source)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CitedBy(ctx context.Context, targetKey string) ([]CitedBy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.source_item_key,i.title,e.ref_index,e.ref_id,e.raw,e.title,e.authors_json,e.container,e.year,e.volume,e.issue,e.pages,e.doi,e.pmid,e.pmcid,e.source,e.match_method,e.match_score,e.match_status FROM ref_entries e JOIN ref_items i ON i.item_key=e.source_item_key WHERE e.target_item_key=? ORDER BY i.title,e.ref_index`, targetKey)
	if err != nil {
		return nil, err
	}
	var out []CitedBy
	for rows.Next() {
		var x CitedBy
		var authors, source string
		if err = rows.Scan(&x.SourceItemKey, &x.SourceTitle, &x.Reference.Index, &x.Reference.ID, &x.Reference.Raw, &x.Reference.Title, &authors, &x.Reference.Container, &x.Reference.Year, &x.Reference.Volume, &x.Reference.Issue, &x.Reference.Pages, &x.Reference.DOI, &x.Reference.PMID, &x.Reference.PMCID, &source, &x.Reference.MatchMethod, &x.Reference.MatchScore, &x.Reference.MatchStatus); err != nil {
			rows.Close()
			return nil, err
		}
		x.Reference.TargetItemKey = targetKey
		x.Reference.Source = Source(source)
		_ = json.Unmarshal([]byte(authors), &x.Reference.Authors)
		out = append(out, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		cs, e := s.contextsForRef(ctx, out[i].SourceItemKey, out[i].Reference.Index)
		if e != nil {
			return nil, e
		}
		out[i].Contexts = cs
	}
	return out, nil
}
func (s *Store) contextsForRef(ctx context.Context, key string, index int) ([]Context, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reference_id,reference_index,marker,section,paragraph,target_item_key,source FROM ref_contexts WHERE source_item_key=? AND reference_index=? ORDER BY ordinal`, key, index)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Context
	for rows.Next() {
		var c Context
		var src string
		if err = rows.Scan(&c.ReferenceID, &c.ReferenceIndex, &c.Marker, &c.Section, &c.Paragraph, &c.TargetItemKey, &src); err != nil {
			return nil, err
		}
		c.Source = Source(src)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Failed(ctx context.Context) ([]FailedItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_key,title,error,attempts,updated_at FROM ref_items WHERE status='failed' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FailedItem
	for rows.Next() {
		var item FailedItem
		if err := rows.Scan(&item.ItemKey, &item.Title, &item.Error, &item.Attempts, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Path() string { return s.path }

func (s *Store) String() string { return fmt.Sprintf("reference store %s", s.path) }
