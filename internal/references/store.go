package references

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	path string
	db   *sql.DB
}

type Status struct {
	IndexedItems    int    `json:"indexed_items"`
	SuccessfulItems int    `json:"successful_items"`
	FailedItems     int    `json:"failed_items"`
	TotalReferences int    `json:"total_references"`
	PMCItems        int    `json:"pmc_items"`
	PubMedItems     int    `json:"pubmed_items"`
	LastIndexedAt   string `json:"last_indexed_at,omitempty"`
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
  PRIMARY KEY(source_item_key, ref_index),
  FOREIGN KEY(source_item_key) REFERENCES ref_items(item_key) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ref_entries_doi ON ref_entries(doi);
CREATE INDEX IF NOT EXISTS idx_ref_entries_pmid ON ref_entries(pmid);
CREATE INDEX IF NOT EXISTS idx_ref_items_status ON ref_items(status);
`)
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
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ref_entries(source_item_key,ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ref := range result.References {
		authors, _ := json.Marshal(ref.Authors)
		if _, err = stmt.ExecContext(ctx, result.ItemKey, ref.Index, ref.ID, ref.Raw, ref.Title, string(authors), ref.Container, ref.Year, ref.Volume, ref.Issue, ref.Pages, ref.DOI, ref.PMID, ref.PMCID, ref.Source); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source FROM ref_entries WHERE source_item_key=? ORDER BY ref_index`, itemKey)
	if err != nil {
		return Result{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref Reference
		var authors, source string
		if err := rows.Scan(&ref.Index, &ref.ID, &ref.Raw, &ref.Title, &authors, &ref.Container, &ref.Year, &ref.Volume, &ref.Issue, &ref.Pages, &ref.DOI, &ref.PMID, &ref.PMCID, &source); err != nil {
			return Result{}, false, err
		}
		_ = json.Unmarshal([]byte(authors), &ref.Authors)
		ref.Source = Source(source)
		result.References = append(result.References, ref)
	}
	return result, true, rows.Err()
}

func (s *Store) Status(ctx context.Context) (Status, error) {
	var status Status
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(status='success'),0),COALESCE(SUM(status='failed'),0),COALESCE(SUM(reference_count),0),COALESCE(SUM(strategy='pmc_jats' AND status='success'),0),COALESCE(SUM(strategy='pubmed' AND status='success'),0),COALESCE(MAX(updated_at),'') FROM ref_items`).Scan(&status.IndexedItems, &status.SuccessfulItems, &status.FailedItems, &status.TotalReferences, &status.PMCItems, &status.PubMedItems, &status.LastIndexedAt)
	return status, err
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
