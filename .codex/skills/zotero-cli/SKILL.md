---
name: zotero-cli
description: Work with the local Zotero CLI in this repository to search, inspect, export, validate configuration, and safely mutate Zotero libraries. Use when Codex needs to operate on Zotero data through `zot.exe` or `go run .\\cmd\\zot`, especially for `find`, `show`, `export`, stats, metadata inspection, batch tagging, or guarded write/delete workflows.
---

# Zotero CLI

Use the local CLI instead of re-implementing Zotero API calls.

## Workflow

1. Start in the repository root.
2. Prefer `.\zot.exe` if the binary exists and is current enough for the task.
3. Fall back to `go run .\cmd\zot ...` when validating source changes or when the binary is missing.
4. Prefer `--json` for agent workflows.
5. Run `config validate` before assuming credentials are usable.

## Read-first defaults

Prefer these commands:

```powershell
.\zot.exe stats --json
.\zot.exe find --all --json
.\zot.exe find "query" --json
.\zot.exe show ITEMKEY --json
.\zot.exe export --collection COLLKEY --format csljson --json
```

Use `find` features to avoid extra round trips:

- `--date-after YYYY[-MM[-DD]]`
- `--date-before YYYY[-MM[-DD]]`
- repeated `--tag`
- `--tag-any`
- `--include-trashed`
- `--qmode everything`

Text mode helpers:

- `--include-fields url,doi,version`
- `--full`

## Write safety

Treat these commands as write operations:

- `create-item`
- `update-item`
- `create-items`
- `update-items`
- `add-tag`
- `remove-tag`
- `create-collection`
- `update-collection`
- `create-search`
- `update-search`

Treat these commands as destructive:

- `delete-item`
- `delete-items`
- `delete-collection`
- `delete-search`

Before any write:

1. Confirm the user actually wants data changed.
2. Check whether `ZOT_ALLOW_WRITE` and `ZOT_ALLOW_DELETE` permit the operation.
3. Use version preconditions when available.

Before any delete:

1. Restate the exact target key or keys.
2. Verify there is no ambiguity.
3. Ask the user before proceeding if the request is even slightly unclear.

## Configuration

The CLI stores config in `~/.zot/.env`.

Useful commands:

```powershell
.\zot.exe config init
.\zot.exe config show
.\zot.exe config validate
```

If config is missing, initialize it instead of hand-waving around the failure.

## References

Read these only when needed:

- `docs/AI_AGENT.md` for agent usage patterns and safety expectations
- `README.md` for human-facing quick start and command coverage
