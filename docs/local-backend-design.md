# Local Backend Design

## Current Status

This design has now been partially implemented in `0.0.1`.

For the next follow-up iteration, see also:

- [0.0.2 Stability Pass](D:/C/Documents/Program/Go/zotero_cli/docs/stability-pass-0.0.2.md)

Implemented:

- `internal/domain` and backend abstraction
- runtime mode selection for `web`, `local`, and `hybrid`
- local `show`
- local `find` MVP
- attachment path reporting in local `show`
- note summaries in local `show`
- explicit local `relate` based on `itemRelations`
- local attachment-aware `find` filters and match reasons
- Zotero `prefs.js` discovery for `dataDir` and `baseAttachmentPath`

Not implemented yet:

- local full-text search
- web-side `relate`
- inferred relation layers based on tags / collections / note context

### Current Backend File Layout

The local backend is no longer intended to grow inside a single `local.go`.

Current file responsibilities:

- `internal/backend/local.go`
  - `LocalReader` lifecycle and high-level orchestration
  - `NewLocalReader`
  - public local read methods such as `FindItems`, `GetItem`, `GetRelated`, and `GetLibraryStats`
  - read-session behavior such as live DB first and snapshot fallback

- `internal/backend/local_db.go`
  - SQLite connection setup
  - busy/locked retry detection
  - snapshot-copy helpers
  - low-level DB lifecycle utilities

- `internal/backend/local_find.go`
  - local `find` SQL construction
  - local post-filtering, ordering, pagination
  - attachment-aware local filters
  - local `matched_on` derivation

- `internal/backend/local_loaders.go`
  - item loaders
  - relation loaders
  - creators / tags / collections / attachments / notes loaders
  - attachment path resolution

- `internal/backend/local_prefs.go`
  - Zotero `prefs.js` discovery and parsing
  - mapping from configured `dataDir` to the matching Zotero profile when possible

- `internal/backend/local_utils.go`
  - shared local-only formatting and normalization helpers
  - small path and text helpers used by multiple local backend files

Expansion rule:

- new `find` query semantics should go to `local_find.go`
- new data loading routines should go to `local_loaders.go`
- new SQLite/session behavior should go to `local_db.go`
- new Zotero profile/config discovery should go to `local_prefs.go`
- `local.go` should stay small and should mostly coordinate the other pieces

This split is intentional.
It should be treated as part of the local backend design rather than as a temporary refactor.

## Goal

This document defines a practical design for adding a local read-only backend to `zot`.

It is based on:

- the current codebase structure
- the existing `web`-first CLI behavior
- inspection of a real Zotero local data directory at `D:\zotero\zotero_file`

The purpose of this document is to serve as the implementation reference for the next phase of the project.

The local backend should optimize for agent-usable CLI behavior:

- stable JSON contracts
- explicit failure modes
- deterministic attachment path reporting
- backend-independent item/domain shapes


## Summary

The current project is built around Zotero Web API access.
That is still the right default for distribution and low-friction setup.

However, the inspected local Zotero data directory is complete enough to support a strong local read-only mode:

- a full main database exists at `D:\zotero\zotero_file\zotero.sqlite`
- an attachment store exists at `D:\zotero\zotero_file\storage`
- metadata, collections, tags, notes, attachments, annotations, and full-text index tables all exist

This means a local backend is now realistic and valuable.

The recommended direction is:

1. Keep `web` as the default portable mode
2. Add a `local` read-only backend for `find` and `show`
3. Add `hybrid` later, so local reads can be preferred while remote remains a fallback

Update:

- `hybrid` is now implemented as a pragmatic local-first read path with selective remote fallback


## Current Project State

The current CLI flow is still single-backend oriented:

- [cli.go](D:/C/Documents/Program/Go/zotero_cli/internal/cli/cli.go) dispatches commands
- [runtime.go](D:/C/Documents/Program/Go/zotero_cli/internal/cli/runtime.go) loads config and directly constructs the Web API client
- stable item types are still defined in [types.go](D:/C/Documents/Program/Go/zotero_cli/internal/zoteroapi/types.go)
- default read filtering logic still lives in [filter.go](D:/C/Documents/Program/Go/zotero_cli/internal/cli/filter.go)

This is lightweight and good for the current `web` mode, but it creates structural limitations:

- CLI depends directly on the remote backend
- item/domain types are coupled to the API package
- behavior such as default item filtering is not owned by a backend-neutral layer
- local and hybrid modes cannot be added cleanly without an abstraction layer

Before or alongside local-mode implementation, the project should move toward:

- a backend interface
- a stable domain model
- separate mapping layers for `web` and `local`
- backend-owned read semantics for `find` and `show`


## Real Local Zotero Data Findings

The inspected Zotero data directory at `D:\zotero\zotero_file` contains:

- `zotero.sqlite`
- `storage/`
- Better BibTeX data
- plugin data
- cached files
- backups and sync-conflict copies

Important findings from the main local database:

- `itemAttachments`: `3016`
- `itemNotes`: `1416`
- `itemAnnotations`: `233`
- `collections`: `33`
- `tags`: `619`
- top-level primary items are approximately `3286` after excluding attachments, notes, and annotations

Important findings from the full-text subsystem:

- `fulltextItems` exists
- `fulltextWords` exists
- `fulltextItemWords` exists

Important findings from attachment storage:

- the standard `storage/` structure exists
- `itemAttachments.path` includes values such as:
  - `storage:zotero-style.html`
  - `attachments:<library-relative-path>.pdf`
  - empty values for some `text/html` attachment records
- observed `storage:` paths can be resolved reliably using `storage/<attachmentKey>/<filename>`
- observed `attachments:` paths should be treated as best-effort only and not assumed to resolve under `data_dir`

Important findings from attachment link modes:

- `linkMode = 2`: `1629`
- `linkMode = 3`: `1230`
- `linkMode = 1`: `120`
- `linkMode = 0`: `37`

Important schema signals:

- top-level items live in `items`
- attachments are identified via `itemAttachments`
- notes are identified via `itemNotes`
- annotations are identified via `itemAnnotations` and `itemType = annotation`
- metadata fields use the Zotero EAV structure:
  - `itemData`
  - `itemDataValues`
  - `fieldsCombined`
- collections and tags are fully representable locally
- item dates are not always normalized and may need cleanup before CLI exposure

Conclusion:

- the local database is good enough for a real read-only backend
- local `show` is especially attractive because attachment and collection data are already present
- local search can begin with metadata search before full-text search is added
- annotation handling and date normalization must be explicit in the design


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
- visible-item filtering for primary items
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

Update:

- explicit relation lookup is now in scope for local mode and implemented through `itemRelations`

### Why this scope

This scope gives immediate value without turning the project into a Zotero Desktop integration layer too early.

It also matches the current CLI value proposition:

- fast lookup
- stable JSON output
- scriptable read paths


## CLI Contract Priorities

The local backend should preserve the current CLI contract shape wherever practical.

Priority rules:

- `find --json` and `show --json` should keep the same top-level response envelope
- local mode should add fields conservatively rather than redefining existing ones
- unsupported commands in `local` mode should fail explicitly rather than silently falling back to `web`
- path resolution failure should not be treated as item lookup failure

## Current Mode Boundaries

The command surface is now split into three practical groups.

### Backend-aware read commands

These commands participate in `web` / `local` / `hybrid` mode selection through
the backend reader:

- `find`
- `show`
- `relate`

Behavior summary:

- `web`: remote-only behavior
- `local`: local SQLite-backed behavior where implemented
- `hybrid`: prefer local, then fall back to remote for supported cases

### Remote API commands

These commands still depend on the Zotero Web API client:

- `cite`
- `export`
- `collections`
- `collections-top`
- `notes`
- `tags`
- `searches`
- `deleted`
- `stats`
- `versions`
- `item-types`
- `item-fields`
- `creator-fields`
- `item-type-fields`
- `item-type-creator-types`
- `item-template`
- `key-info`
- `groups`
- `trash`
- `publications`
- all create/update/delete commands

Behavior summary:

- `web`: supported
- `hybrid`: supported through the remote client path
- `local`: explicitly rejected with a mode-boundary error

### Known limitation

`relate` remains local/hybrid only for now.
Pure `web` mode still reports it as unsupported until a remote relation strategy
is implemented.

For attachments, the local backend should expose enough information for agents to reason about file availability:

- `key`
- `item_type`
- `title`
- `content_type`
- `link_mode`
- `filename`
- `zotero_path`
- `resolved_path`
- `resolved`


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

type CollectionRef struct {
    Key  string
    Name string
}

type Attachment struct {
    Key          string
    ItemType     string
    Title        string
    ContentType  string
    LinkMode     string
    ZoteroPath   string
    ResolvedPath string
    Resolved     bool
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


### 4. Move read semantics out of the CLI layer

The CLI should not own backend-sensitive read logic such as visible-item filtering.

In particular:

- primary-item filtering should not remain solely in [filter.go](D:/C/Documents/Program/Go/zotero_cli/internal/cli/filter.go)
- `web`, `local`, and `hybrid` should converge on one semantic contract for `find`
- backend-specific mapping should happen before rendering, not during rendering


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

Recommended future config fields for the first local version:

```json
{
  "mode": "local",
  "data_dir": "D:\\zotero\\zotero_file"
}
```

Derived values:

- `sqlite_path = <data_dir>\\zotero.sqlite`
- `storage_dir = <data_dir>\\storage`

Implementation notes:

- first local version should only require `data_dir`
- `sqlite_path` and `storage_dir` can be derived automatically
- `prefer_local_reads` should be introduced later with `hybrid`, not in the local MVP


## Table Strategy

### Core tables for local `find`

- `items`
- `itemTypes`
- `itemData`
- `itemDataValues`
- `fieldsCombined`
- `itemCreators`
- `creators`
- `creatorTypes`
- `itemTags`
- `tags`
- `itemAttachments`
- `itemNotes`
- `itemAnnotations`

### Core tables for local `show`

- all of the above, plus:
- `collections`
- `collectionItems`

### Core tables for local attachment behavior

- `itemAttachments`
- `items`
- `storage/`

### Core tables for future local full-text behavior

- `fulltextItems`
- `fulltextWords`
- `fulltextItemWords`


## Query Design

### Visible item policy

Local `find` should not treat every row in `items` as a visible library item.

Default visible items should exclude:

- rows present in `itemAttachments`
- rows present in `itemNotes`
- rows present in `itemAnnotations`
- rows with `itemType = annotation`

Practical rule:

- default `find` results should represent primary library records for human and agent workflows

This mirrors the current CLI expectation that `find` returns main library items by default while avoiding noisy result sets.


### Local `find`

#### First version goals

- search metadata, not full-text
- support `--item-type`
- support `--limit`
- default to visible primary items only

#### Searchable fields in first version

- `title`
- `shortTitle`
- `publicationTitle`
- `bookTitle`
- `proceedingsTitle`
- creator names
- tag names
- item key
- date or year

#### Query strategy

Use a base item query that:

- joins `items` to `itemTypes`
- filters out attachments, notes, and annotations
- optionally filters by item type
- applies limit

Then enrich results with:

- title/date/container fields from `itemData` + `itemDataValues`
- creators from `itemCreators` + `creators` + `creatorTypes`
- tags from `itemTags` + `tags`

#### Why not one giant SQL query

For the first version, multiple smaller queries are preferable because they are:

- easier to understand
- easier to test
- easier to maintain
- easier to align with the same domain model used by the remote backend
- less likely to produce row multiplication when item metadata, tags, collections, and attachments are joined together


### Local `show`

#### First version goals

- fetch one item by Zotero key
- return complete CLI detail view
- include attachment info
- include collections
- optionally include notes summary

#### Suggested query decomposition

1. Main item
- query `items`, `itemTypes`, `itemData`, `itemDataValues`, `fieldsCombined`

2. Creators
- query `itemCreators`, `creators`, `creatorTypes`
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

Suggested repository methods:

- `GetCoreItemByKey`
- `ListCreatorsByItemID`
- `ListTagsByItemID`
- `ListAttachmentsByParentItemID`
- `ListCollectionsByItemID`
- `ListNotesByParentItemID`


### Date normalization

Local date values should not be exposed to the CLI without normalization.

Observed values can contain duplicated or mixed representations such as:

- `2019-03-29 2019-03-29`
- `2024-01-08 2024-01-08 00:00:00`

Implications:

- local row mapping should normalize date strings before returning domain items
- date filtering should operate on normalized values
- web and local backends should converge on the same visible date semantics where practical


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
- if resolution fails, keep the raw path and mark `resolved = false`

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
Failure to resolve should not fail `show`.

#### empty path

Treat as unresolved.
This commonly corresponds to HTML snapshots, linked URLs, or records without a local file payload.


## Full-text Search

### Recommendation

Do not add full-text search in the first local milestone.

### Why

Even though `fulltext*` tables exist and are valuable, adding them immediately would require:

- learning Zotero's full-text layout in detail
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

Deliverable:

- no user-facing behavior change
- codebase is ready for multiple backends


### Phase 2: Backend selection with web parity

- create a `web` backend adapter over existing Web API behavior
- move CLI read paths to backend selection
- keep current `web` behavior intact

Deliverable:

- CLI uses backend abstraction
- `web` mode behavior remains unchanged


### Phase 3: Local `show`

- implement local DB opening in read-only mode
- fetch one item by key
- return creators, tags, attachments, and collections
- wire `mode=local` into `show`

Deliverable:

- `zot show KEY` works in local mode

Why this phase first:

- success criteria are clear
- queries are bounded
- local value is immediately obvious


### Phase 4: Attachment path resolution

- resolve `storage:` paths to local files
- expose raw path, resolved path, and resolved status in JSON
- improve human output for attachments

Deliverable:

- local `show` provides meaningful local file visibility


### Phase 5: Local `find`

- implement visible-item filtering
- exclude attachment, note, and annotation items by default
- search metadata fields
- support `--item-type`
- support `--limit`

Deliverable:

- `zot find QUERY` works in local mode


### Phase 6: Hybrid mode

- define read preference behavior
- use local for primary reads
- optionally fall back to remote

Deliverable:

- offline-friendly but still robust behavior


### Phase 7: Full-text search

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


### Risk: Visible-item rules are noisier locally than they appear remotely

Annotations can leak into local result sets if filtering is too naive.

Mitigation:

- treat visible-item filtering as an explicit policy
- test attachment, note, and annotation exclusion together
- avoid relying only on `items` row presence


### Risk: Local dates are not normalized

Raw Zotero local date values may not match CLI expectations.

Mitigation:

- normalize date values in one mapper layer
- test date filtering against observed local variants
- avoid spreading date cleanup logic across CLI rendering code


### Risk: Local DB differences across machines

Not every user will have the same data directory layout.

Mitigation:

- treat `local` as an advanced mode
- prefer `data_dir` over auto-discovery for first implementation
- keep `web` mode as the default path


### Risk: Coupling implementation to the current inspected DB only

The inspected DB is real and useful, but it should not be assumed to represent every Zotero installation perfectly.

Mitigation:

- code against Zotero schema patterns, not one sample's exact values
- test against multiple sample item types
- isolate DB access in one package


## Testing Recommendations

### Unit tests

- local row-to-domain mapping
- visible-item filtering
- attachment path resolution
- field extraction from EAV structure
- date normalization

### Integration tests

- read-only queries against a fixture sqlite DB
- `find` output parity between web and local backends where practical
- `show` attachment rendering with representative path variants
- exclusion of attachment, note, and annotation items from default local `find`

### Manual checks

- local `show` on items with PDF attachments
- local `show` on items with HTML snapshot attachments
- local `find` with creator, title, and tag matches
- local collection visibility
- behavior when `zotero.sqlite` is missing
- behavior when `storage/` is missing
- behavior when an attachment path cannot be resolved


## Recommended Next Step

The next implementation step should be:

1. define the stable domain model
2. define the backend interface
3. move CLI reads onto the backend abstraction
4. implement local `show`

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
