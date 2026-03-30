# Zotero CLI MVP

## 1. Goal

Build a Go-based, AI-era Zotero CLI that is:

- local-first for day-to-day research work
- scriptable for shell and agent workflows
- safe by default for write operations
- useful before MCP and semantic search are added

The MVP is not "a full Zotero replacement in terminal".
The MVP is "the fastest way to search, inspect, cite, and export items from Zotero in a terminal-friendly and machine-friendly way".


## 2. Product Positioning

`zot` is a command-line research tool for Zotero users who want:

- fast search from terminal
- stable JSON output for scripts and AI agents
- easy citation generation
- easy export of selected items
- a clean path toward future local indexing and MCP support

The MVP should optimize for:

- search speed
- clear output
- low setup friction
- predictable behavior

The MVP should not optimize for:

- full CRUD coverage of every Zotero API object
- local database writes
- complex sync logic
- semantic search
- plugin-level Zotero UI integration


## 3. Target Users

### Primary users

- researchers who already use Zotero and terminal tools
- AI-assisted writing users who need reliable citation and lookup commands
- developers building small scripts, editors, or agent workflows around Zotero

### Secondary users

- students using Zotero with markdown / LaTeX / Pandoc / Typst
- knowledge workers managing PDFs and notes from terminal


## 4. Why Go

We are choosing Go for the product, even though Python has stronger existing Zotero ecosystem support, for these reasons:

- single binary distribution across Windows, macOS, and Linux
- fast startup and low runtime friction
- strong fit for CLI tooling, network clients, watchers, and local services
- good long-term maintainability for a standalone tool
- official MCP Go SDK is now Tier 1, so future MCP support is practical

Implication:

- we should avoid rebuilding ecosystem-heavy pieces that are better delegated to Zotero itself or the Zotero Web API
- citation formatting should lean on Zotero API output where possible
- MVP scope should focus on reliable CLI flows, not advanced bibliographic engines


## 5. MVP Scope

### In scope

- config management for Zotero credentials and library selection
- search items by query
- show one item in detail
- generate citations in a few practical formats
- export matched items
- machine-readable JSON output
- human-readable terminal output
- remote access via Zotero Web API

### Out of scope

- local SQLite reads or writes
- PDF full-text indexing
- note editing
- attachment upload
- full batch mutation workflows
- MCP server
- semantic search
- Better BibTeX-specific direct integration


## 6. Core User Stories

1. As a user, I can configure the CLI once and reuse that configuration safely.
2. As a user, I can search my Zotero library by title, creator, or quick search terms.
3. As a user, I can inspect one item and immediately see the metadata I care about.
4. As a user, I can generate a citation or bibliography snippet in a format my writing workflow needs.
5. As a user, I can export selected items to formats like BibTeX and CSL JSON.
6. As a script or AI agent, I can call the CLI with `--json` and get stable structured output.


## 7. Proposed Commands

The command surface should stay small.

### `zot config init`

Interactive setup for:

- library type: `user` or `group`
- library ID
- API key
- default citation style
- default locale

Behavior:

- stores config in a user config directory
- supports environment variable overrides
- validates credentials with a small test request

### `zot config show`

Shows active profile and non-sensitive config values.

### `zot find <query>`

Searches items using Zotero Web API quick search.

Useful flags:

- `--limit`
- `--start`
- `--sort`
- `--direction`
- `--item-type`
- `--tag`
- `--collection`
- `--json`
- `--fields`

Default human output:

- key
- title
- creators summary
- year
- item type
- publication / container

### `zot show <item-key>`

Shows one item in detail.

Default human output:

- title
- item key
- type
- creators
- date
- publication info
- DOI / URL
- tags
- collections if available
- child attachment summary if retrievable

### `zot cite <item-key|query>`

Generates citation output for one item or the first match for a query.

Useful flags:

- `--style`
- `--locale`
- `--format`
- `--json`

Initial supported formats:

- `citation`
- `bib`
- `csljson`

Notes:

- `citation` and `bib` should rely on Zotero Web API formatting support
- `csljson` should use item JSON transformation rules inside our code only if needed, otherwise prefer API-provided export

### `zot export`

Exports one or more items.

Useful flags:

- `--query`
- `--item-key`
- `--collection`
- `--format`
- `--output`
- `--limit`

Initial supported formats:

- `bibtex`
- `biblatex`
- `csljson`
- `ris`

Behavior:

- write to stdout by default
- write to file when `--output` is provided


## 8. Command Design Principles

### Human and machine modes

Every read command should support:

- default human-readable output
- `--json` for stable structured output

JSON rules:

- do not change field names casually
- prefer explicit nulls over omitted fields when practical
- keep top-level envelope stable

Suggested JSON envelope:

```json
{
  "ok": true,
  "command": "find",
  "data": [],
  "meta": {
    "total": 0,
    "limit": 20,
    "start": 0
  }
}
```

### Safe defaults

- read operations must be side-effect free
- future write operations must require explicit confirmation or `--yes`
- errors should be clear and actionable

### Composability

- stdout should remain useful in pipelines
- stderr should contain operational messages and errors
- exit codes should be predictable


## 9. Architecture

The MVP should use a layered structure.

### Suggested package layout

```text
/cmd/zot
/internal/cli
/internal/config
/internal/zoteroapi
/internal/domain
/internal/output
/internal/app
```

### Responsibilities

#### `/cmd/zot`

- program entrypoint
- bootstraps root command

#### `/internal/cli`

- Cobra commands and flags
- argument validation

#### `/internal/config`

- load/save config
- environment variable overrides
- profile selection

#### `/internal/zoteroapi`

- HTTP client to Zotero Web API
- request builders
- auth headers
- pagination handling
- response decoding

#### `/internal/domain`

- stable internal types such as `Item`, `Creator`, `Tag`, `CollectionRef`
- mapping from API responses to internal models

#### `/internal/output`

- table/text rendering
- JSON rendering
- error formatting

#### `/internal/app`

- use-case layer
- orchestrates config, API client, mapping, and output


## 10. API Strategy

### Primary backend

Use Zotero Web API v3 as the only backend in MVP.

Reasons:

- official and stable for new development
- supports search, sorting, export formats, and formatted citation output
- avoids local-database fragility

### Usage notes

- use conditional requests and version headers later, but not required for first MVP
- prefer small, explicit API wrappers over generic endpoint builders
- keep request logic testable with mocked HTTP responses


## 11. Config Design

### File location

Use OS-specific config dirs.

Examples:

- Windows: `%AppData%\\zotcli\\config.json`
- macOS: `~/Library/Application Support/zotcli/config.json`
- Linux: `~/.config/zotcli/config.json`

### Environment variable overrides

Support:

- `ZOT_LIBRARY_TYPE`
- `ZOT_LIBRARY_ID`
- `ZOT_API_KEY`
- `ZOT_STYLE`
- `ZOT_LOCALE`

### Security

- never print the API key in normal output
- mask secrets in `config show`
- keep file permissions as strict as practical per OS

### Runtime modes

The configuration model should distinguish between two modes:

1. `web` mode
2. `local` mode

For MVP, only `web` mode needs to be implemented.

This distinction should exist in the config model from day one, so we do not need to redesign config later when local indexing is added.

#### `web` mode

This is the default and only required mode in MVP.

Required fields:

- `mode`
- `library_type`
- `library_id`
- `api_key`

Optional fields:

- `style`
- `locale`
- `profile`
- `timeout_seconds`

Behavior:

- uses Zotero Web API only
- does not depend on local Zotero installation
- does not require `zotero.sqlite`
- works best for distributed external users

#### `local` mode

This is a post-MVP extension mode.

Potential future fields:

- `data_dir`
- `sqlite_path`
- `storage_dir`
- `index_dir`
- `enable_local_index`
- `prefer_local_reads`

Behavior:

- may read Zotero local data for faster search and attachment access
- may support offline or hybrid workflows
- must be treated as optional advanced setup, not baseline setup

### Initial config schema

Suggested shape:

```json
{
  "active_profile": "default",
  "profiles": {
    "default": {
      "mode": "web",
      "library_type": "user",
      "library_id": "123456",
      "api_key": "****",
      "style": "apa",
      "locale": "en-US",
      "timeout_seconds": 20
    }
  }
}
```

Note:

- stored value for `api_key` is the real value, but display output must always be masked
- future local-mode fields should live inside the same profile object

### Setup expectations for end users

For MVP, end users should only need:

- a Zotero account or group library
- a Zotero library ID
- a Zotero API key

They should not need:

- Zotero desktop installed locally
- Zotero local API enabled
- Zotero database path
- manual SQLite configuration

This is important for distribution:

- fewer setup failures
- easier documentation
- better supportability across platforms


## 12. Output Model

### Human output

Should be concise and scan-friendly.

For `find`, example shape:

```text
X42A7DEE  Attention Is All You Need        Vaswani et al.   2017  conferencePaper
AB12CD34  BERT: Pre-training of Deep...    Devlin et al.    2019  journalArticle
```

### JSON output

Example for `show`:

```json
{
  "ok": true,
  "command": "show",
  "data": {
    "key": "X42A7DEE",
    "title": "Attention Is All You Need",
    "itemType": "conferencePaper",
    "date": "2017",
    "creators": [
      {
        "name": "Ashish Vaswani",
        "creatorType": "author"
      }
    ],
    "doi": null,
    "url": "https://arxiv.org/abs/1706.03762"
  }
}
```


## 13. Error Handling

Define a small, clear error model.

Examples:

- config not found
- invalid API key
- item not found
- invalid query arguments
- upstream Zotero API error
- network timeout

CLI behavior:

- short human error on stderr
- optional detailed JSON error object when `--json` is set

Suggested exit codes:

- `0` success
- `2` bad user input
- `3` config/auth problem
- `4` remote API/network problem
- `5` not found


## 14. Dependencies

Recommended starting choices:

- `cobra` for command structure
- `viper` only if config complexity grows; otherwise prefer a small custom config layer
- Go standard library `net/http`
- Go standard library `encoding/json`
- table rendering library only if needed; avoid early dependency sprawl

Recommendation:

- start with Cobra
- avoid Viper in v1 unless it clearly saves time
- keep dependencies minimal


## 15. Testing Strategy

### Unit tests

- config parsing and validation
- request construction
- response mapping
- output formatting

### Integration tests

- mock Zotero API via `httptest`
- cover pagination and export responses

### Manual test matrix

- Windows PowerShell
- macOS zsh
- Linux bash

Key manual checks:

- UTF-8 output
- file output behavior
- env var overrides
- invalid credentials handling


## 16. MVP Milestones

### Milestone 1: Skeleton

- repo structure
- root command
- config file support
- API client base

Deliverable:

- `zot version`
- `zot config init`
- `zot config show`

### Milestone 2: Core read path

- `find`
- `show`
- `--json`

Deliverable:

- usable search and inspect workflow

### Milestone 3: Writing workflow helpers

- `cite`
- `export`

Deliverable:

- useful markdown / LaTeX / bibliography workflow

### Milestone 4: Polish

- better errors
- stable JSON schema
- docs
- packaged binaries


## 17. Non-Goals for MVP Review

The MVP is successful if users can say:

- "I can find papers from terminal faster than opening Zotero."
- "I can use this from scripts and AI tools without parsing fragile text."
- "I can generate citations and exports without extra glue code."

The MVP does not need to prove:

- full local-first offline architecture
- advanced PDF intelligence
- complete library management
- deep AI features


## 18. Risks and Mitigations

### Risk: Go citation ecosystem is weaker

Mitigation:

- rely on Zotero API formatting support for citation and bibliography output
- avoid implementing CSL processing in MVP

### Risk: Web API search is less powerful than future local indexing

Mitigation:

- accept this limitation in MVP
- design domain layer so local index backend can be added later

### Risk: output shape drifts as features grow

Mitigation:

- define stable internal item model early
- document JSON schema in repo

### Risk: setup friction around Zotero credentials

Mitigation:

- make `config init` interactive and friendly
- include config validation command


## 19. Post-MVP Direction

After MVP is stable, next likely priorities are:

1. local full-text index for PDFs and notes
2. attachment discovery and snippet extraction
3. batch-safe write commands with `--dry-run`
4. MCP server mode
5. Better BibTeX-aware helpers
6. optional semantic retrieval


## 20. Build Recommendation

Start narrow and finish a good small tool.

Recommended implementation order:

1. `config init`
2. `config show`
3. `find`
4. `show`
5. `cite`
6. `export`
7. polish JSON and errors

If scope pressure appears, cut breadth before cutting quality.
Specifically:

- do not add write commands before read flows feel solid
- do not add local indexing before `find/show/cite/export` are reliable
- do not add MCP before the CLI output model is stable
