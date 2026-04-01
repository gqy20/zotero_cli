# 0.0.2 Stability Pass

This document records the implementation plan for the first post-`0.0.1`
stability-focused iteration.

## Goal

`0.0.2` should make the current command surface more predictable without
expanding the product in major ways.

The main target is not "more commands".
The main target is "more stable behavior across `web`, `local`, and `hybrid`
read paths".

## Why This Iteration Exists

`0.0.1` already ships meaningful value:

- backend abstraction exists
- `local` and `hybrid` read modes exist
- read commands and common write commands are tested
- CI and release workflows are in place

However, the current implementation still has a few sharp edges that would make
future work riskier if left untouched:

- `hybrid` fallback currently depends on matching error strings
- `find` filtering and date-range logic are implemented in more than one place
- some read commands are backend-aware while others are still explicitly web-only
- write argument validation often falls back to usage text without a precise
  reason

Before adding larger capabilities such as local full-text search or MCP support,
the current behavior should be stabilized.

## Scope

### In scope

- stabilize `hybrid` fallback semantics
- replace brittle string-based fallback checks with explicit backend signals
- reduce duplicated `find` filtering logic across web/local paths
- document which commands are backend-aware and which remain web-only
- improve write-argument error clarity

### Out of scope

- local full-text search
- MCP server support
- semantic search
- local database writes
- large command-surface expansion

## Planned Work

### 1. Stabilize backend fallback behavior

Current issue:

- `HybridReader` decides whether to fall back to web by inspecting `err.Error()`

Desired outcome:

- fallback decisions should be based on stable signals rather than message text

Expected implementation direction:

- introduce small backend-level error/capability markers
- have `local` and `web` readers return those markers where appropriate
- update tests so they assert behavior, not exact fallback wording

### 2. Consolidate `find` semantics

Current issue:

- date parsing and filtering logic exist in both the CLI layer and the local
  backend
- default filtering rules for attachments, notes, and annotations are easy to
  drift over time

Desired outcome:

- one core implementation should define the shared `find` contract where
  practical

Expected implementation direction:

- centralize tag matching
- centralize date-range parsing
- centralize default visible-item policy
- keep backend-specific SQL/query work separate from shared filtering semantics

### 3. Clarify mode boundaries

Current issue:

- `find`, `show`, and `relate` are backend-aware
- commands such as `stats`, `cite`, and `export` still rely directly on the web
  client

Desired outcome:

- docs should clearly describe which commands support `local`/`hybrid` semantics
- unsupported combinations should remain explicit

### 4. Improve write argument errors

Current issue:

- many invalid write command invocations print usage without a specific reason

Desired outcome:

- distinguish cases such as:
  - missing version precondition
  - invalid JSON payload
  - missing file
  - mutually exclusive flags

## TDD Order

Implementation should proceed in this order:

1. add or update tests for backend fallback semantics
2. implement the fallback/error model changes
3. add or update tests for shared `find` semantics where needed
4. consolidate the implementation
5. add or update tests for write-argument error clarity
6. implement parser error improvements
7. run focused tests, then the full suite

## Progress

Completed slices:

- backend fallback now uses explicit backend error markers rather than raw
  string matching
- shared `find` semantics now live in backend-level reusable helpers
- write command parsing now returns specific validation errors before printing
  usage text
- hybrid mode now normalizes remote client calls through the web API path
- local mode now fails fast for web-only API commands with an explicit mode
  boundary error

Write validation cases covered by tests now include:

- `--data` and `--from-file` conflicts
- unreadable `--from-file` paths
- invalid JSON payloads for single and batch writes
- missing `--if-unmodified-since-version` where required

## Definition of Done

`0.0.2` stability work is considered complete when:

- `hybrid` fallback is no longer keyed off raw error strings
- shared `find` behavior is defined in fewer places
- docs clearly state current mode support boundaries
- write argument failures are easier to diagnose
- all tests pass
