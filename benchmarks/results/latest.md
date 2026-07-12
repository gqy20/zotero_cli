# CLI benchmark report

Generated: 2026-07-12 13:22:12+08:00  
Binary: `.\zot.exe`  
Mode: `all`  
Iterations: 3

## Runtime scenarios

| Scenario | Command | Status | Source | Cold ms | Median ms | Net ms | P95 ms | Note |
|---|---|---:|---|---:|---:|---:|---:|---|
| version | `version` | ok |  | 111.65 | 125.01 | 0.00 | 131.52 |  |
| config-show | `config show` | ok |  | 115.79 | 117.75 | 0.00 | 167.81 |  |
| config-check | `config check` | ok |  | 1502.50 | 1395.15 | 1270.14 | 1417.73 |  |
| library-show | `lib show` | ok | live | 299.83 | 298.84 | 173.84 | 332.75 |  |
| library-show-local | `lib show` | ok | live | 302.85 | 358.85 | 233.84 | 404.64 |  |
| library-stats | `lib stats` | ok | live | 120.33 | 156.82 | 31.82 | 159.24 |  |
| library-stats-local | `lib stats` | ok | live | 145.65 | 143.18 | 18.17 | 154.28 |  |
| item-list | `item list` | ok | live | 321.68 | 315.73 | 190.72 | 317.80 |  |
| item-list-local | `item list` | ok | live | 352.62 | 354.59 | 229.58 | 431.80 |  |
| item-find | `item find` | ok | live | 455.95 | 398.32 | 273.31 | 460.12 |  |
| item-find-local | `item find` | ok | live | 578.23 | 458.46 | 333.46 | 488.15 |  |
| note-list | `note list` | ok | live | 163.63 | 166.55 | 41.55 | 167.52 |  |
| note-list-local | `note list` | ok | live | 173.89 | 187.81 | 62.81 | 194.70 |  |
| note-find | `note find` | ok | live | 136.67 | 146.85 | 21.85 | 156.33 |  |
| note-find-local | `note find` | ok | live | 142.44 | 138.25 | 13.25 | 168.43 |  |
| collection-list | `coll list` | ok | live | 116.03 | 123.18 | 0.00 | 128.28 |  |
| tag-list | `tag list` | ok | live | 129.82 | 131.75 | 6.74 | 142.74 |  |
| search-list | `search list` | ok | live | 131.01 | 125.32 | 0.31 | 150.84 |  |
| search-show | `search show` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| group-list | `group list` | failed |  | 1365.83 | 0.00 | 0.00 | 0.00 | exit status 1 |
| index-status | `index status` | ok |  | 218.98 | 186.49 | 61.48 | 200.56 |  |
| ref-status | `ref status` | ok |  | 121.93 | 134.12 | 9.12 | 152.87 |  |
| schema-types | `schema list` | ok | cache | 199.09 | 215.85 | 90.84 | 221.34 |  |
| schema-types-refresh | `schema list` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier extended not enabled |
| completion-powershell | `completion` | ok |  | 123.85 | 113.18 | 0.00 | 128.19 |  |
| item-show | `item show` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| file-show | `file show` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| file-check | `file check` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| pdf-text | `pdf text` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| annotation-list | `ann list` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| item-new-dry-run | `item new` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |
| item-edit-dry-run | `item edit` | skipped |  | 0.00 | 0.00 | 0.00 | 0.00 | tier data not enabled |

## Runtime hotspots

| Scenario | Source | Net median ms | Output bytes |
|---|---|---:|---:|
| config-check |  | 1270.14 | 307 |
| item-find-local | live | 333.46 | 3461 |
| item-find | live | 273.31 | 3461 |
| library-show-local | live | 233.84 | 12074 |
| item-list-local | live | 229.58 | 10293 |
| item-list | live | 190.72 | 10293 |
| library-show | live | 173.84 | 12074 |
| schema-types | cache | 90.84 | 2875 |

Net median subtracts the `version` median as an estimate of fixed process startup and command construction cost.

## Backend comparisons

| Comparison | Variant | Source | Median ms | Net ms |
|---|---|---|---:|---:|
| item-find-mode | configured | live | 398.32 | 273.31 |
| item-find-mode | local | live | 458.46 | 333.46 |
| item-list-mode | configured | live | 315.73 | 190.72 |
| item-list-mode | local | live | 354.59 | 229.58 |
| library-show-mode | configured | live | 298.84 | 173.84 |
| library-show-mode | local | live | 358.85 | 233.84 |
| library-stats-mode | configured | live | 156.82 | 31.82 |
| library-stats-mode | local | live | 143.18 | 18.17 |
| note-find-mode | configured | live | 146.85 | 21.85 |
| note-find-mode | local | live | 138.25 | 13.25 |
| note-list-mode | configured | live | 166.55 | 41.55 |
| note-list-mode | local | live | 187.81 | 62.81 |

## Command coverage and necessity audit

| Command | Help ms | Status | Necessity | Overlap / replacement |
|---|---:|---|---|---|
| `lib show` | 117.29 | ok | review | lib stats, item list -> keep only if one-call summary is materially faster or clearer |
| `lib stats` | 150.41 | ok | keep |  |
| `lib log` | 146.26 | ok | keep |  |
| `item list` | 174.65 | ok | review | item find -> item find with an explicit all-items mode |
| `item find` | 160.17 | ok | keep |  |
| `item show` | 136.71 | ok | keep |  |
| `item new` | 124.06 | ok | keep |  |
| `item edit` | 121.69 | ok | keep |  |
| `item delete` | 132.20 | ok | keep |  |
| `item tag` | 142.85 | ok | keep |  |
| `item untag` | 121.67 | ok | keep |  |
| `item supp` | 174.15 | ok | keep |  |
| `item export` | 125.95 | ok | keep |  |
| `coll list` | 120.55 | ok | keep |  |
| `coll show` | 123.18 | ok | keep |  |
| `coll new` | 133.35 | ok | keep |  |
| `coll edit` | 141.65 | ok | keep |  |
| `coll delete` | 169.89 | ok | keep |  |
| `coll add` | 148.23 | ok | keep |  |
| `coll remove` | 127.00 | ok | keep |  |
| `note list` | 127.34 | ok | review | note find -> note find with an explicit all-notes mode |
| `note show` | 136.08 | ok | keep |  |
| `note find` | 114.48 | ok | keep |  |
| `note new` | 125.18 | ok | keep |  |
| `note edit` | 118.55 | ok | keep |  |
| `note delete` | 103.89 | ok | keep |  |
| `tag list` | 93.87 | ok | keep |  |
| `search list` | 84.18 | ok | keep |  |
| `search show` | 99.77 | ok | keep |  |
| `search new` | 92.40 | ok | keep |  |
| `search edit` | 123.44 | ok | keep |  |
| `search delete` | 114.36 | ok | keep |  |
| `group list` | 89.71 | ok | keep |  |
| `file show` | 92.33 | ok | keep |  |
| `file check` | 101.73 | ok | keep | file show |
| `pdf text` | 106.27 | ok | keep |  |
| `pdf figs` | 85.24 | ok | keep |  |
| `pdf open` | 101.14 | ok | review | file show -> expose a file path from file show when shell integration is sufficient |
| `ann list` | 125.05 | ok | keep |  |
| `ann new` | 128.28 | ok | keep |  |
| `ann delete` | 203.59 | ok | keep |  |
| `ref show` | 136.55 | ok | keep |  |
| `ref find` | 128.84 | ok | keep |  |
| `ref related` | 131.79 | ok | keep |  |
| `ref cited` | 121.99 | ok | keep |  |
| `ref ctx` | 108.93 | ok | keep |  |
| `ref links` | 105.55 | ok | review | ref related, ref cited |
| `ref entities` | 105.09 | ok | keep |  |
| `ref profile` | 105.59 | ok | review | ref show, ref entities -> compose only if latency and output remain acceptable |
| `ref build` | 104.66 | ok | keep |  |
| `ref resolve` | 115.19 | ok | keep |  |
| `ref status` | 109.64 | ok | keep |  |
| `index build` | 125.64 | ok | keep |  |
| `index status` | 121.71 | ok | keep |  |
| `schema list` | 124.44 | ok | keep |  |
| `schema show` | 95.15 | ok | keep |  |
| `config init` | 120.45 | ok | keep |  |
| `config show` | 102.78 | ok | keep |  |
| `config check` | 121.70 | ok | keep | config show |
| `server start` | 115.21 | ok | keep |  |
| `sync pull` | 121.83 | ok | keep |  |
| `completion` | 118.14 | ok | keep |  |
| `version` | 105.88 | ok | keep |  |

## Interpretation

Help timings measure process startup and command construction, not Zotero operation latency. Removal decisions require runtime data plus capability overlap; latency alone is not sufficient.
