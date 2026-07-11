package references

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	path string
	db   *sql.DB
}

type Status struct {
	IndexedItems             int    `json:"indexed_items"`
	SuccessfulItems          int    `json:"successful_items"`
	FailedItems              int    `json:"failed_items"`
	UnsupportedItems         int    `json:"unsupported_items"`
	TotalReferences          int    `json:"total_references"`
	PMCItems                 int    `json:"pmc_items"`
	PubMedItems              int    `json:"pubmed_items"`
	GrobidItems              int    `json:"grobid_items"`
	ResolvedReferences       int    `json:"resolved_references"`
	UnresolvedReferences     int    `json:"unresolved_references"`
	CitationContexts         int    `json:"citation_contexts"`
	ContextAvailableItems    int    `json:"context_available_items"`
	ContextNotSupportedItems int    `json:"context_not_supported_items"`
	ContextNotFoundItems     int    `json:"context_not_found_items"`
	ContextParseFailedItems  int    `json:"context_parse_failed_items"`
	ContextNotIndexedItems   int    `json:"context_not_indexed_items"`
	ReferencesWithContext    int    `json:"references_with_context"`
	ReferencesWithoutContext int    `json:"references_without_context"`
	LastIndexedAt            string `json:"last_indexed_at,omitempty"`
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
	  pubmed_metadata_json TEXT NOT NULL DEFAULT '{}',
	  metadata_version INTEGER NOT NULL DEFAULT 0,
  strategy TEXT,
  status TEXT NOT NULL,
  reference_count INTEGER NOT NULL DEFAULT 0,
	  context_status TEXT NOT NULL DEFAULT 'not_indexed',
	  context_count INTEGER NOT NULL DEFAULT 0,
	  references_with_context INTEGER NOT NULL DEFAULT 0,
	  references_without_context INTEGER NOT NULL DEFAULT 0,
	  context_coverage REAL NOT NULL DEFAULT 0,
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
	  context_status TEXT NOT NULL DEFAULT 'not_indexed', context_count INTEGER NOT NULL DEFAULT 0,
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
CREATE TABLE IF NOT EXISTS ref_annotations (
  item_key TEXT NOT NULL, ordinal INTEGER NOT NULL, provider TEXT, annotation_type TEXT,
  entity TEXT, label TEXT, section TEXT, exact_text TEXT, prefix TEXT, suffix TEXT,
  PRIMARY KEY(item_key,ordinal)
);
CREATE TABLE IF NOT EXISTS ref_meta (key TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE VIRTUAL TABLE IF NOT EXISTS ref_entries_fts USING fts5(source_item_key UNINDEXED,ref_index UNINDEXED,title,authors,container,doi,pmid,raw);
CREATE VIRTUAL TABLE IF NOT EXISTS ref_contexts_fts USING fts5(source_item_key UNINDEXED,ordinal UNINDEXED,reference_index UNINDEXED,section,marker,paragraph);
CREATE VIRTUAL TABLE IF NOT EXISTS ref_metadata_fts USING fts5(item_key UNINDEXED,mesh,publication_types,keywords,chemicals,grants,corrections);
CREATE VIRTUAL TABLE IF NOT EXISTS ref_annotations_fts USING fts5(item_key UNINDEXED,ordinal UNINDEXED,provider,annotation_type,entity,label,section,exact_text);
CREATE INDEX IF NOT EXISTS idx_ref_entries_doi ON ref_entries(doi);
CREATE INDEX IF NOT EXISTS idx_ref_entries_pmid ON ref_entries(pmid);
CREATE INDEX IF NOT EXISTS idx_ref_items_status ON ref_items(status);
CREATE INDEX IF NOT EXISTS idx_ref_contexts_target ON ref_contexts(target_item_key);
CREATE INDEX IF NOT EXISTS idx_ref_contexts_reference ON ref_contexts(source_item_key,reference_index);
`)
	if err != nil {
		return err
	}
	// Upgrade indexes created by versions predating local citation resolution.
	for _, alter := range []string{
		`ALTER TABLE ref_entries ADD COLUMN target_item_key TEXT`, `ALTER TABLE ref_entries ADD COLUMN match_method TEXT`,
		`ALTER TABLE ref_entries ADD COLUMN match_score REAL NOT NULL DEFAULT 0`, `ALTER TABLE ref_entries ADD COLUMN match_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE ref_entries ADD COLUMN context_status TEXT NOT NULL DEFAULT 'not_indexed'`, `ALTER TABLE ref_entries ADD COLUMN context_count INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, e := s.db.Exec(alter); e != nil && !strings.Contains(strings.ToLower(e.Error()), "duplicate column") {
			return e
		}
	}
	for _, alter := range []string{
		`ALTER TABLE ref_items ADD COLUMN context_status TEXT NOT NULL DEFAULT 'not_indexed'`,
		`ALTER TABLE ref_items ADD COLUMN context_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE ref_items ADD COLUMN references_with_context INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE ref_items ADD COLUMN references_without_context INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE ref_items ADD COLUMN context_coverage REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE ref_items ADD COLUMN pubmed_metadata_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE ref_items ADD COLUMN metadata_version INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, e := s.db.Exec(alter); e != nil && !strings.Contains(strings.ToLower(e.Error()), "duplicate column") {
			return e
		}
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ref_entries_target ON ref_entries(target_item_key)`)
	if err == nil {
		_, err = s.db.Exec(`UPDATE ref_items SET status='unsupported' WHERE status='failed' AND error LIKE 'item % has no DOI, PMID, or PMCID usable by NCBI'`)
	}
	if err == nil {
		_, err = s.db.Exec(`UPDATE ref_items SET
		context_status=CASE WHEN strategy='pubmed' THEN 'not_supported' WHEN strategy='pmc_jats' AND EXISTS(SELECT 1 FROM ref_contexts c WHERE c.source_item_key=ref_items.item_key) THEN 'available' ELSE 'not_indexed' END,
		context_count=(SELECT COUNT(*) FROM ref_contexts c WHERE c.source_item_key=ref_items.item_key),
		references_with_context=(SELECT COUNT(DISTINCT reference_index) FROM ref_contexts c WHERE c.source_item_key=ref_items.item_key AND reference_index>0),
		references_without_context=MAX(reference_count-(SELECT COUNT(DISTINCT reference_index) FROM ref_contexts c WHERE c.source_item_key=ref_items.item_key AND reference_index>0),0),
		context_coverage=CASE WHEN reference_count>0 THEN CAST((SELECT COUNT(DISTINCT reference_index) FROM ref_contexts c WHERE c.source_item_key=ref_items.item_key AND reference_index>0) AS REAL)/reference_count ELSE 0 END
		WHERE status='success' AND context_status='not_indexed'`)
	}
	if err == nil {
		_, err = s.db.Exec(`UPDATE ref_entries SET
		context_count=(SELECT COUNT(*) FROM ref_contexts c WHERE c.source_item_key=ref_entries.source_item_key AND c.reference_index=ref_entries.ref_index),
		context_status=CASE
		 WHEN (SELECT context_status FROM ref_items i WHERE i.item_key=ref_entries.source_item_key)='available' AND EXISTS(SELECT 1 FROM ref_contexts c WHERE c.source_item_key=ref_entries.source_item_key AND c.reference_index=ref_entries.ref_index) THEN 'available'
		 WHEN (SELECT context_status FROM ref_items i WHERE i.item_key=ref_entries.source_item_key)='available' THEN 'not_found'
		 ELSE COALESCE((SELECT context_status FROM ref_items i WHERE i.item_key=ref_entries.source_item_key),'not_indexed') END
		WHERE context_status='not_indexed'`)
	}
	if err == nil {
		err = s.syncReferenceFTS()
	}
	return err
}

func (s *Store) syncReferenceFTS() error {
	var version string
	if err := s.db.QueryRow(`SELECT value FROM ref_meta WHERE key='reference_fts_version'`).Scan(&version); err == nil && version == "2" {
		return nil
	} else if err != nil && err != sql.ErrNoRows {
		return err
	}
	var entries, entryFTS, contexts, contextFTS int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ref_entries`).Scan(&entries); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ref_entries_fts`).Scan(&entryFTS); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ref_contexts`).Scan(&contexts); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ref_contexts_fts`).Scan(&contextFTS); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM ref_entries_fts`); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO ref_entries_fts(source_item_key,ref_index,title,authors,container,doi,pmid,raw) SELECT source_item_key,ref_index,title,authors_json,container,doi,pmid,raw FROM ref_entries`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM ref_contexts_fts`); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO ref_contexts_fts(source_item_key,ordinal,reference_index,section,marker,paragraph) SELECT source_item_key,ordinal,reference_index,section,marker,paragraph FROM ref_contexts`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM ref_metadata_fts`); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT item_key,pubmed_metadata_json FROM ref_items WHERE status='success'`)
	if err != nil {
		return err
	}
	type metadataRow struct {
		key      string
		metadata PubMedMetadata
	}
	var metadataRows []metadataRow
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return err
		}
		var m PubMedMetadata
		_ = json.Unmarshal([]byte(raw), &m)
		metadataRows = append(metadataRows, metadataRow{key, m})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, row := range metadataRows {
		mesh, pt, kw, ch, gr, co := metadataSearchText(row.metadata)
		if _, err = tx.Exec(`INSERT INTO ref_metadata_fts(item_key,mesh,publication_types,keywords,chemicals,grants,corrections) VALUES(?,?,?,?,?,?,?)`, row.key, mesh, pt, kw, ch, gr, co); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO ref_meta(key,value) VALUES('reference_fts_version','2') ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) IsFresh(ctx context.Context, itemKey, fingerprint string) (bool, error) {
	var stored, status string
	var metadataVersion int
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint, status, metadata_version FROM ref_items WHERE item_key=?`, itemKey).Scan(&stored, &status, &metadataVersion)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return stored == fingerprint && (status == "unsupported" || (status == "success" && metadataVersion >= 2)), nil
}

func (s *Store) SaveResult(ctx context.Context, result Result, fingerprint string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	result.ContextSummary = SummarizeContexts(result.Strategy, result.References, result.Contexts)
	if result.ContextError != "" {
		result.ContextSummary.Status = ContextParseFailed
	}
	AnnotateReferenceContexts(result.References, result.Contexts, result.ContextSummary.Status)
	metadataJSON, _ := json.Marshal(result.Metadata)
	_, err = tx.ExecContext(ctx, `INSERT INTO ref_items(item_key,title,fingerprint,doi,pmid,pmcid,pubmed_metadata_json,metadata_version,strategy,status,reference_count,context_status,context_count,references_with_context,references_without_context,context_coverage,error,attempts,updated_at)
VALUES(?,?,?,?,?,?,?,2,?,?,?,?,?,?,?,?,'',1,?) ON CONFLICT(item_key) DO UPDATE SET title=excluded.title,fingerprint=excluded.fingerprint,doi=excluded.doi,pmid=excluded.pmid,pmcid=excluded.pmcid,pubmed_metadata_json=excluded.pubmed_metadata_json,metadata_version=2,strategy=excluded.strategy,status='success',reference_count=excluded.reference_count,context_status=excluded.context_status,context_count=excluded.context_count,references_with_context=excluded.references_with_context,references_without_context=excluded.references_without_context,context_coverage=excluded.context_coverage,error='',attempts=ref_items.attempts+1,updated_at=excluded.updated_at`,
		result.ItemKey, result.ItemTitle, fingerprint, result.Identifiers.DOI, result.Identifiers.PMID, result.Identifiers.PMCID, string(metadataJSON), result.Strategy, "success", len(result.References), result.ContextSummary.Status, result.ContextSummary.ContextCount, result.ContextSummary.ReferencesWithContext, result.ContextSummary.ReferencesWithoutContext, result.ContextSummary.Coverage, now)
	if err != nil {
		return err
	}
	// A metadata/context refresh must not erase local citation resolution.
	// Carry matches forward by stable identifiers (then normalized title).
	existingMatches := map[string]Reference{}
	matchRows, matchErr := tx.QueryContext(ctx, `SELECT doi,pmid,title,COALESCE(target_item_key,''),COALESCE(match_method,''),match_score,COALESCE(match_status,'') FROM ref_entries WHERE source_item_key=? AND match_status='resolved'`, result.ItemKey)
	if matchErr != nil {
		return matchErr
	}
	for matchRows.Next() {
		var old Reference
		if err := matchRows.Scan(&old.DOI, &old.PMID, &old.Title, &old.TargetItemKey, &old.MatchMethod, &old.MatchScore, &old.MatchStatus); err != nil {
			matchRows.Close()
			return err
		}
		for _, key := range referenceMatchKeys(old) {
			existingMatches[key] = old
		}
	}
	if err := matchRows.Err(); err != nil {
		matchRows.Close()
		return err
	}
	matchRows.Close()
	for i := range result.References {
		if result.References[i].MatchStatus == "resolved" {
			continue
		}
		for _, key := range referenceMatchKeys(result.References[i]) {
			if old, ok := existingMatches[key]; ok {
				result.References[i].TargetItemKey = old.TargetItemKey
				result.References[i].MatchMethod = old.MatchMethod
				result.References[i].MatchScore = old.MatchScore
				result.References[i].MatchStatus = old.MatchStatus
				break
			}
		}
	}
	targetByIndex := map[int]string{}
	for _, ref := range result.References {
		if ref.TargetItemKey != "" {
			targetByIndex[ref.Index] = ref.TargetItemKey
		}
	}
	for i := range result.Contexts {
		result.Contexts[i].TargetItemKey = targetByIndex[result.Contexts[i].ReferenceIndex]
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_entries WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_entries_fts WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_contexts_fts WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_contexts WHERE source_item_key=?`, result.ItemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_metadata_fts WHERE item_key=?`, result.ItemKey); err != nil {
		return err
	}
	mesh, pubtypes, keywords, chemicals, grants, corrections := metadataSearchText(result.Metadata)
	if _, err = tx.ExecContext(ctx, `INSERT INTO ref_metadata_fts(item_key,mesh,publication_types,keywords,chemicals,grants,corrections) VALUES(?,?,?,?,?,?,?)`, result.ItemKey, mesh, pubtypes, keywords, chemicals, grants, corrections); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ref_entries(source_item_key,ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source,target_item_key,match_method,match_score,match_status,context_status,context_count) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ref := range result.References {
		authors, _ := json.Marshal(ref.Authors)
		if _, err = stmt.ExecContext(ctx, result.ItemKey, ref.Index, ref.ID, ref.Raw, ref.Title, string(authors), ref.Container, ref.Year, ref.Volume, ref.Issue, ref.Pages, ref.DOI, ref.PMID, ref.PMCID, ref.Source, ref.TargetItemKey, ref.MatchMethod, ref.MatchScore, ref.MatchStatus, ref.ContextStatus, ref.ContextCount); err != nil {
			return err
		}
	}
	entryFTS, err := tx.PrepareContext(ctx, `INSERT INTO ref_entries_fts(source_item_key,ref_index,title,authors,container,doi,pmid,raw) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer entryFTS.Close()
	for _, ref := range result.References {
		authors, _ := json.Marshal(ref.Authors)
		if _, err = entryFTS.ExecContext(ctx, result.ItemKey, ref.Index, ref.Title, string(authors), ref.Container, ref.DOI, ref.PMID, ref.Raw); err != nil {
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
	contextFTS, err := tx.PrepareContext(ctx, `INSERT INTO ref_contexts_fts(source_item_key,ordinal,reference_index,section,marker,paragraph) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer contextFTS.Close()
	for i, c := range result.Contexts {
		if _, err = contextFTS.ExecContext(ctx, result.ItemKey, i+1, c.ReferenceIndex, c.Section, c.Marker, c.Paragraph); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func referenceMatchKeys(ref Reference) []string {
	keys := []string{}
	if doi := normalizeDOI(ref.DOI); doi != "" {
		keys = append(keys, "doi:"+doi)
	}
	if pmid := strings.TrimSpace(ref.PMID); pmid != "" {
		keys = append(keys, "pmid:"+pmid)
	}
	if title := normalizeTitle(ref.Title); title != "" {
		keys = append(keys, "title:"+title)
	}
	return keys
}

func metadataSearchText(m PubMedMetadata) (string, string, string, string, string, string) {
	mesh := []string{}
	for _, x := range m.MeSH {
		mesh = append(mesh, x.UI, x.Name)
		for _, q := range x.Qualifiers {
			mesh = append(mesh, q.UI, q.Name)
		}
	}
	chemicals := []string{}
	for _, x := range m.Chemicals {
		chemicals = append(chemicals, x.UI, x.Name, x.RegistryNumber)
	}
	grants := []string{}
	for _, x := range m.Grants {
		grants = append(grants, x.ID, x.Agency, x.Country, x.Acronym)
	}
	corrections := []string{}
	for _, x := range m.Corrections {
		corrections = append(corrections, x.Type, x.PMID, x.Ref)
	}
	return strings.Join(mesh, " "), strings.Join(m.PublicationTypes, " "), strings.Join(m.Keywords, " "), strings.Join(chemicals, " "), strings.Join(grants, " "), strings.Join(corrections, " ")
}

func (s *Store) SaveFailure(ctx context.Context, itemKey, title, fingerprint string, cause error) error {
	return s.saveIssue(ctx, "failed", itemKey, title, fingerprint, cause)
}

func (s *Store) SaveUnsupported(ctx context.Context, itemKey, title, fingerprint string, cause error) error {
	return s.saveIssue(ctx, "unsupported", itemKey, title, fingerprint, cause)
}

func (s *Store) saveIssue(ctx context.Context, status, itemKey, title, fingerprint string, cause error) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO ref_items(item_key,title,fingerprint,status,error,attempts,updated_at) VALUES(?,?,?,?,?,1,?)
ON CONFLICT(item_key) DO UPDATE SET title=excluded.title,fingerprint=excluded.fingerprint,status=excluded.status,error=excluded.error,attempts=ref_items.attempts+1,updated_at=excluded.updated_at`, itemKey, title, fingerprint, status, cause.Error(), now)
	return err
}

func (s *Store) LoadResult(ctx context.Context, itemKey string) (Result, bool, error) {
	var result Result
	var status string
	var metadataVersion int
	var metadataJSON string
	err := s.db.QueryRowContext(ctx, `SELECT item_key,title,doi,pmid,pmcid,pubmed_metadata_json,metadata_version,strategy,status,context_status,context_count,references_with_context,references_without_context,context_coverage FROM ref_items WHERE item_key=?`, itemKey).Scan(&result.ItemKey, &result.ItemTitle, &result.Identifiers.DOI, &result.Identifiers.PMID, &result.Identifiers.PMCID, &metadataJSON, &metadataVersion, &result.Strategy, &status, &result.ContextSummary.Status, &result.ContextSummary.ContextCount, &result.ContextSummary.ReferencesWithContext, &result.ContextSummary.ReferencesWithoutContext, &result.ContextSummary.Coverage)
	if err == sql.ErrNoRows {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if status != "success" || metadataVersion < 2 {
		return Result{}, false, nil
	}
	_ = json.Unmarshal([]byte(metadataJSON), &result.Metadata)
	rows, err := s.db.QueryContext(ctx, `SELECT ref_index,ref_id,raw,title,authors_json,container,year,volume,issue,pages,doi,pmid,pmcid,source,target_item_key,match_method,match_score,match_status,context_status,context_count FROM ref_entries WHERE source_item_key=? ORDER BY ref_index`, itemKey)
	if err != nil {
		return Result{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref Reference
		var authors, source string
		if err := rows.Scan(&ref.Index, &ref.ID, &ref.Raw, &ref.Title, &authors, &ref.Container, &ref.Year, &ref.Volume, &ref.Issue, &ref.Pages, &ref.DOI, &ref.PMID, &ref.PMCID, &source, &ref.TargetItemKey, &ref.MatchMethod, &ref.MatchScore, &ref.MatchStatus, &ref.ContextStatus, &ref.ContextCount); err != nil {
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
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(status='success'),0),COALESCE(SUM(status='failed'),0),COALESCE(SUM(status='unsupported'),0),COALESCE(SUM(reference_count),0),COALESCE(SUM(strategy='pmc_jats' AND status='success'),0),COALESCE(SUM(strategy='pubmed' AND status='success'),0),COALESCE(SUM(strategy='grobid' AND status='success'),0),COALESCE(MAX(updated_at),'') FROM ref_items`).Scan(&status.IndexedItems, &status.SuccessfulItems, &status.FailedItems, &status.UnsupportedItems, &status.TotalReferences, &status.PMCItems, &status.PubMedItems, &status.GrobidItems, &status.LastIndexedAt)
	if err == nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(match_status='resolved'),0),COALESCE(SUM(match_status!='resolved'),0) FROM ref_entries`).Scan(&status.ResolvedReferences, &status.UnresolvedReferences)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ref_contexts`).Scan(&status.CitationContexts)
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(context_status='available'),0),COALESCE(SUM(context_status='not_supported'),0),COALESCE(SUM(context_status='not_found'),0),COALESCE(SUM(context_status='parse_failed'),0),COALESCE(SUM(context_status='not_indexed'),0),COALESCE(SUM(references_with_context),0),COALESCE(SUM(references_without_context),0) FROM ref_items WHERE status='success'`).Scan(&status.ContextAvailableItems, &status.ContextNotSupportedItems, &status.ContextNotFoundItems, &status.ContextParseFailedItems, &status.ContextNotIndexedItems, &status.ReferencesWithContext, &status.ReferencesWithoutContext)
	}
	return status, err
}

func (s *Store) Resolve(ctx context.Context, resolver *Resolver, workers int) (ResolveReport, error) {
	start := time.Now()
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
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
	type resolvedEntry struct {
		source string
		ref    Reference
	}
	jobs := make(chan entry)
	results := make(chan resolvedEntry, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range jobs {
				results <- resolvedEntry{source: e.source, ref: resolver.Resolve(e.ref, e.source)}
			}
		}()
	}
	go func() {
		for _, e := range entries {
			jobs <- e
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	for e := range results {
		ref := e.ref
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

func (s *Store) SaveAnnotations(ctx context.Context, itemKey string, rows []Annotation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_annotations WHERE item_key=?`, itemKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ref_annotations_fts WHERE item_key=?`, itemKey); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ref_annotations(item_key,ordinal,provider,annotation_type,entity,label,section,exact_text,prefix,suffix) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	fts, err := tx.PrepareContext(ctx, `INSERT INTO ref_annotations_fts(item_key,ordinal,provider,annotation_type,entity,label,section,exact_text) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer fts.Close()
	for i, row := range rows {
		if _, err = stmt.ExecContext(ctx, itemKey, i+1, row.Provider, row.Type, row.Entity, row.Label, row.Section, row.Exact, row.Prefix, row.Suffix); err != nil {
			return err
		}
		if _, err = fts.ExecContext(ctx, itemKey, i+1, row.Provider, row.Type, row.Entity, row.Label, row.Section, row.Exact); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ContextSummary(ctx context.Context, itemKey string) (ContextSummary, bool, error) {
	var summary ContextSummary
	err := s.db.QueryRowContext(ctx, `SELECT context_status,context_count,references_with_context,references_without_context,context_coverage FROM ref_items WHERE item_key=? AND status='success'`, itemKey).Scan(&summary.Status, &summary.ContextCount, &summary.ReferencesWithContext, &summary.ReferencesWithoutContext, &summary.Coverage)
	if err == sql.ErrNoRows {
		return ContextSummary{}, false, nil
	}
	return summary, err == nil, err
}

func (s *Store) CitedBy(ctx context.Context, targetKey string) ([]CitedBy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.source_item_key,i.title,e.ref_index,e.ref_id,e.raw,e.title,e.authors_json,e.container,e.year,e.volume,e.issue,e.pages,e.doi,e.pmid,e.pmcid,e.source,e.match_method,e.match_score,e.match_status,e.context_status,e.context_count FROM ref_entries e JOIN ref_items i ON i.item_key=e.source_item_key WHERE e.target_item_key=? ORDER BY i.title,e.ref_index`, targetKey)
	if err != nil {
		return nil, err
	}
	var out []CitedBy
	for rows.Next() {
		var x CitedBy
		var authors, source string
		if err = rows.Scan(&x.SourceItemKey, &x.SourceTitle, &x.Reference.Index, &x.Reference.ID, &x.Reference.Raw, &x.Reference.Title, &authors, &x.Reference.Container, &x.Reference.Year, &x.Reference.Volume, &x.Reference.Issue, &x.Reference.Pages, &x.Reference.DOI, &x.Reference.PMID, &x.Reference.PMCID, &source, &x.Reference.MatchMethod, &x.Reference.MatchScore, &x.Reference.MatchStatus, &x.Reference.ContextStatus, &x.Reference.ContextCount); err != nil {
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
	return s.issues(ctx, "failed")
}

func (s *Store) Unsupported(ctx context.Context) ([]FailedItem, error) {
	return s.issues(ctx, "unsupported")
}

func (s *Store) ContextPending(ctx context.Context, limit int) ([]FailedItem, error) {
	query := `SELECT item_key,title,'',attempts,updated_at FROM ref_items WHERE status='success' AND strategy='pmc_jats' AND context_status='not_indexed' ORDER BY updated_at`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
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

func (s *Store) issues(ctx context.Context, status string) ([]FailedItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_key,title,error,attempts,updated_at FROM ref_items WHERE status=? ORDER BY updated_at DESC`, status)
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
