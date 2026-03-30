# Local Backend Design

## Goal

This document defines a practical design for adding a local read-only backend to `zot`.

It is based on:

- the current codebase structure
- the existing `web`-first CLI behavior
- inspection of a real Zotero local data directory at `D:\zotero\zotero_file`

The purpose of this document is to serve as the implementation reference for the next phase of the project.


## Summary

The current project is built around Zotero Web API access.
That is still the right default for distribution and low-friction setup.

However, the inspected local Zotero data directory is complete enough to support a strong local read-only mode:

- a full main database exists at `D:\zotero\zotero_file\zotero.sqlite`
- an attachment store exists at `D:\zotero\zotero_file\storage`
- metadata, collections, tags, notes, attachments, and full-text index tables all exist

This means a local backend is now realistic and valuable.

The recommended direction is:

1. Keep `web` as the default portable mode
2. Add a `local` read-only backend for `find` and `show`
3. Add `hybrid` later, so local reads can be preferred while remote remains a fallback


## Current Project State

The current CLI flow is still single-backend oriented:

- [cli.go](D:/C/Documents/Program/Go/zotero_cli/internal/cli/cli.go) loads config
- it directly constructs [client.go](D:/C/Documents/Program/Go/zotero_cli/internal/zoteroapi/client.go)
- internal item models are still defined in the API package

This is lightweight and good for the current `web` mode, but it creates a structural limitation:

- CLI depends directly on the remote backend
- item/domain types are coupled to the API package
- local and hybrid modes cannot be added cleanly without an abstraction layer

Before or alongside local-mode implementation, the project should move toward:

- a backend interface
- a stable domain model
- separate mapping layers for `web` and `local`


## Real Local Zotero Data Findings

The inspected Zotero data directory at `D:\zotero\zotero_file` contains:

- `zotero.sqlite`
- `storage/`
- Better BibTeX data
- plugin data
- cached files
- backups and sync-conflict copies

Important findings from the main local database:

- `items`: `6603`
- `itemAttachments`: `2973`
- `itemNotes`: `1401`
- `collections`: `33`
- `collectionItems`: `1651`
- `itemTags`: `3063`
- `syncCache`: `5079`
- `syncDeleteLog`: `5540`

Important findings from the full-text subsystem:

- `fulltextItems`: `1683`
- `fulltextWords`: `480092`
- `fulltextItemWords`: `3850047`

Important findings from attachment storage:

- the standard `storage/` structure exists
- `itemAttachments.path` includes values such as:
  - `storage:2014_10.1093-bioinformatics-btt656.pdf`
  - `attachments:Q_生物科学/.../file.pdf`
  - empty values for some `text/html` attachment records

Important findings from attachment link modes:

- `linkMode = 2`: `1616`
- `linkMode = 3`: `1200`
- `linkMode = 1`: `120`
- `linkMode = 0`: `37`

Important schema signals:

- top-level items live in `items`
- attachments are identified via `itemAttachments`
- notes are identified via `itemNotes`
- metadata fields use the Zotero EAV structure:
  - `itemData`
  - `itemDataValues`
  - `fieldsCombined`
- collections and tags are fully representable locally

Conclusion:

- the local database is good enough for a real read-only backend
- local `show` is especially attractive because attachment and collection data are already present
- local search can begin with metadata search before full-text search is added


## Recommended Product Direction

### Mode strategy

Recommended runtime modes:

1. `web`
2. `local`
3. `hybrid`

Where:

- `web`: read from Zotero Web API only
- `local`: read from Zotero local database and attachment store only
- `hybrid`: prefer local reads, with optional remote fallback

`web` should remain the default for general users because it:

- avoids Desktop dependency
- works across machines
- is easier to document

`local` should be introduced now because it:

- improves speed
- improves offline usability
- enables local attachment visibility
- exposes richer read-only data than the current API path

`hybrid` should come later because it:

- needs clearer precedence rules
- needs fallback behavior design
- is easier after both `web` and `local` backends are already working


## Recommended Scope for Local MVP

### In scope

- local `find`
- local `show`
- top-level item filtering
- local creators, tags, and collections
- local attachment listing
- attachment path resolution when possible

### Out of scope

- writes to `zotero.sqlite`
- writes to `storage/`
- sync conflict resolution
- local note editing
- full-text search in the first local milestone
- real-time watch/sync behavior

### Why this scope

This scope gives immediate value without turning the project into a Zotero Desktop integration layer too early.

It also matches the current CLI value proposition:

- fast lookup
- stable JSON output
- scriptable read paths


## Architecture Recommendation

### 1. Introduce a domain layer

Move stable internal models out of `internal/zoteroapi`.

Recommended package:

```text
/internal/domain
```

Recommended core types:

```go
type Item struct {
    Key         string
    ItemType    string
    Title       string
    Date        string
    Container   string
    DOI         string
    URL         string
    Creators    []Creator
    Tags        []string
    Collections []CollectionRef
    Attachments []Attachment
    Notes       []NoteRef
    UpdatedAt   string
    Version     int
}

type Creator struct {
    Name        string
    CreatorType string
}

type Attachment struct {
    Key          string
    ItemType     string
    Title        string
    ContentType  string
    LinkMode     string
    ZoteroPath   string
    ResolvedPath string
    Filename     string
}
```

This model should represent CLI needs, not backend storage details.


### 2. Introduce a backend interface

Recommended package:

```text
/internal/backend
```

Recommended first interface:

```go
type Reader interface {
    FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error)
    GetItem(ctx context.Context, key string) (domain.Item, error)
}
```

Potential implementations:

- `webBackend`
- `localBackend`
- `hybridBackend`


### 3. Keep CLI independent from backend details

The CLI should stop constructing the Web API client directly.

Instead:

- config is loaded
- mode is selected
- a backend is created
- CLI only calls the backend interface

This is the key structural change that makes local mode realistic.


## Local Backend Design

### Data source

Primary local data sources:

- `zotero.sqlite`
- `storage/`

Optional future sources:

- `fulltext*` tables
- Better BibTeX artifacts
- plugin-generated metadata

The local backend should ignore plugin-specific data for the first version unless it is strictly necessary.


### Configuration

Recommended future config fields:

```json
{
  "mode": "local",
  "data_dir": "D:\\zotero\\zotero_file",
  "sqlite_path": "D:\\zotero\\zotero_file\\zotero.sqlite",
  "storage_dir": "D:\\zotero\\zotero_file\\storage",
  "prefer_local_reads": true
}
```

Implementation note:

- first local version should only require `data_dir`
- `sqlite_path` and `storage_dir` can be derived automatically


## Table Strategy

### Core tables for local `find`

- `items`
- `itemTypes`
- `itemData`
- `itemDataValues`
- `fieldsCombined`
- `itemCreators`
- `creators`
- `itemTags`
- `tags`
- `itemAttachments`
- `itemNotes`

### Core tables for local `show`

- all of the above, plus:
- `collections`
- `collectionItems`

### Core tables for future local attachment behavior

- `itemAttachments`
- `items`
- `storage/`

### Core tables for future local full-text behavior

- `fulltextItems`
- `fulltextWords`
- `fulltextItemWords`


## Query Design

### Top-level item filtering

Local `find` should not treat every row in `items` as a visible library item.

Default visible items should exclude:

- rows present in `itemAttachments`
- rows present in `itemNotes`

Practical rule:

- top-level items are items not represented as attachments and not represented as notes

This mirrors the current CLI expectation that `find` returns main library items by default.


### Local `find`

#### First version goals

- search metadata, not full-text
- support `--item-type`
- support `--limit`
- default to top-level items only

#### Searchable fields in first version

- `title`
- `shortTitle`
- `publicationTitle`
- `bookTitle`
- `proceedingsTitle`
- creator names
- tag names
- item key
- optionally year/date

#### Query strategy

Use a base item query that:

- joins `items` to `itemTypes`
- filters out attachments and notes
- optionally filters by item type
- applies limit

Then enrich results with:

- title/date/container fields from `itemData` + `itemDataValues`
- creators from `itemCreators` + `creators`
- tags from `itemTags` + `tags`

#### Why not one giant SQL query

For the first version, multiple smaller queries are preferable because they are:

- easier to understand
- easier to test
- easier to maintain
- easier to align with the same domain model used by the remote backend


### Local `show`

#### First version goals

- fetch one item by Zotero key
- return complete CLI detail view
- include attachment info
- include collections
- optionally include notes summary

#### Suggested query decomposition

1. Main item
- query `items`, `itemTypes`, `itemData`, `itemDataValues`

2. Creators
- query `itemCreators`, `creators`
- preserve order

3. Tags
- query `itemTags`, `tags`

4. Attachments
- query `itemAttachments` joined to child `items`

5. Collections
- query `collectionItems`, `collections`

6. Notes
- query `itemNotes`

This decomposition should map naturally into a small local repository layer.


## Attachment Path Resolution

This is one of the most valuable local-only features.

Observed attachment path styles:

- `storage:filename.pdf`
- `attachments:relative/or/logical/path.pdf`
- empty paths for some web attachments or snapshots

### First version strategy

For each attachment:

- return the raw Zotero path string
- attempt to resolve to a real local filesystem path
- if resolution fails, keep the raw path and mark resolved path as empty

### Resolution rules

#### `storage:...`

Likely resolution:

- use the attachment item key
- map to `storage/<attachmentKey>/filename`

This should work for imported-storage style attachments.

#### `attachments:...`

Treat as a separate path family.

Do not assume the same resolution as `storage:`.
Support can begin as best-effort only.

#### empty path

Treat as unresolved.
This commonly corresponds to HTML snapshots, linked URLs, or records without a local file payload.


## Full-text Search

### Recommendation

Do not add full-text search in the first local milestone.

### Why

Even though `fulltext*` tables exist and are valuable, adding them immediately would require:

- learning Zotero’s full-text layout in detail
- defining ranking behavior
- deciding how metadata search and full-text search interact

### Better sequence

1. local metadata search
2. stable local `show`
3. attachment resolution
4. optional full-text command or flag

Future command options:

- `zot find --fulltext`
- `zot grep`


## Implementation Phases

### Phase 1: Structural preparation

- add `/internal/domain`
- add `/internal/backend`
- move stable item types out of `zoteroapi`
- keep existing `web` behavior intact

Deliverable:

- no user-facing behavior change
- codebase is ready for multiple backends


### Phase 2: Local `show`

- implement local DB opening in read-only mode
- fetch one item by key
- return creators, tags, attachments, collections
- wire `mode=local` into `show`

Deliverable:

- `zot show KEY` works in local mode

Why this phase first:

- success criteria are clear
- queries are bounded
- local value is immediately obvious


### Phase 3: Local `find`

- implement top-level filtering
- search metadata fields
- support `--item-type`
- support `--limit`

Deliverable:

- `zot find QUERY` works in local mode


### Phase 4: Attachment path resolution

- resolve `storage:` paths to local files
- expose raw and resolved path in JSON
- improve human output for attachments

Deliverable:

- local `show` provides meaningful local file visibility


### Phase 5: Hybrid mode

- define read preference behavior
- use local for primary reads
- optionally fall back to remote

Deliverable:

- offline-friendly but still robust behavior


### Phase 6: Full-text search

- evaluate query model over `fulltext*`
- add optional CLI support

Deliverable:

- local search stronger than Web API quick search


## Risks and Mitigations

### Risk: Zotero local schema complexity

Zotero uses an EAV-like metadata model and multiple relation tables.

Mitigation:

- avoid one-query-everything design
- centralize mapping logic
- write tests against a representative fixture DB


### Risk: Attachment path semantics vary

Not every attachment path is a simple storage file.

Mitigation:

- expose raw path first
- add best-effort resolution
- never fail item rendering because a path could not be resolved


### Risk: Local DB differences across machines

Not every user will have the same data directory layout.

Mitigation:

- treat `local` as an advanced mode
- prefer `data_dir` over auto-discovery for first implementation
- keep `web` mode as the default path


### Risk: Coupling implementation to current inspected DB only

The inspected DB is real and useful, but it should not be assumed to represent every Zotero installation perfectly.

Mitigation:

- code against Zotero schema patterns, not one sample’s exact values
- test against multiple sample item types
- isolate DB access in one package


## Testing Recommendations

### Unit tests

- local row-to-domain mapping
- top-level item filtering
- attachment path resolution
- field extraction from EAV structure

### Integration tests

- read-only queries against a fixture sqlite DB
- `find` output parity between web and local backends where practical
- `show` attachment rendering with representative path variants

### Manual checks

- local `show` on items with PDF attachments
- local `show` on items with HTML snapshot attachments
- local `find` with creator/title/tag matches
- local collection visibility
- behavior when `zotero.sqlite` is missing
- behavior when `storage/` is missing


## Recommended Next Step

The next implementation step should be:

1. define the stable domain model
2. define the backend interface
3. implement local `show`

This sequence gives the highest value with the lowest architectural risk.

It also creates the cleanest path toward:

- local `find`
- hybrid mode
- future full-text support


## Final Recommendation

The project should not jump directly to a full local-first architecture.

Instead, it should add a local read-only backend in a staged way:

- preserve `web` as the general default
- add `local` for strong single-machine read performance and offline use
- add `hybrid` after both are stable

Given the inspected Zotero data directory and database contents, this is now a practical and worthwhile next phase.
