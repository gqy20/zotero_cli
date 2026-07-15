# Zotero CLI v2 Reference

本文件是 Claude skill 的扩展参考，集中说明参数约定、输出结构和运行模式。

## 任务路由

```text
了解库整体情况 ─────────→ zot lib show --json
统计库规模 ─────────────→ zot lib stats --json
搜索/筛选文献 ──────────→ zot item find QUERY [filters] --json
查看单条目 ─────────────→ zot item show ITEMKEY --json
导出引用数据 ───────────→ zot item export [KEYS...] --as FORMAT --json
补充材料/附件 ──────────→ zot item supp / zot file path|show|check
正文检索 ───────────────→ item find --in fulltext --snippet
整篇正文 ───────────────→ zot pdf text ITEMKEY
图表提取 ───────────────→ zot pdf figs ITEMKEY
标注读取/写入/删除 ─────→ zot ann list/new/delete
引用数据 ───────────────→ zot ref show|find|related|cited|ctx|...
全文索引 ───────────────→ zot index build|status
配置检查 ───────────────→ zot config check
远端同步 ───────────────→ zot sync
```

## 全局输出与错误

全局 flags：

```text
--format text|json
--json
--verbose, -v
--no-color
--mode MODE
--timeout DURATION
```

成功 JSON：

```json
{
  "ok": true,
  "command": "item find",
  "data": [],
  "meta": {"total": 0}
}
```

失败 JSON 使用同一 envelope，并在 `data` 中提供 `error`、`type`、`code`。canonical `command` 不随旧入口变化。

## Item 查询

```powershell
zot item list --limit 20 --offset 0 --json
zot item list --scope trash --json
zot item list --scope pubs --json
zot item find "CRISPR" --type article --json
zot item find --sort dateAdded --order desc --limit 10 --json
zot item show ITEMKEY --json
zot item show ITEMKEY --snippet --json
```

常用过滤：

- `--date-after` / `--date-before`
- `--tag`（重复为 AND）/ `--tag-any`
- `--collection` / `--no-collection`
- `--type` / `--no-type`
- `--modified-within` / `--added-since`
- `--has-pdf`
- `--attachment-name` / `--attachment-path`
- `--in metadata|fulltext`（默认 metadata；全文查询直接使用 SQLite FTS5 语法）
- `--snippet`

分页和排序只使用 `--limit`、`--offset`、`--sort`、`--order`。`item find` 的轻量结果默认限制 100 条，`--snippet` 或 `--full` 默认限制 20 条；`--all` 仅取消上限，并与 `--limit` 互斥。metadata 范围允许省略 QUERY。JSON `meta` 提供 `has_more` 和可选的 `next_offset`。

## Item type alias

| 输入 | application/API/JSON |
|---|---|
| `article` | `journalArticle` |
| `chapter` | `bookSection` |
| `conf` | `conferencePaper` |
| `web` | `webpage` |
| `blog` | `blogPost` |

官方值也始终可以直接输入。不要将 `journalArticle` 缩写写入 JSON 或持久化数据。

## 导出

```powershell
zot item export ITEMKEY --as bibtex --json
zot item export KEY1 KEY2 --as csljson --json
zot item find --collection COLLKEY --all --json > selected.json
zot item export --from selected.json --as ris
zot item find '"hybrid speciation"' --in metadata --json | zot item export --from - --as biblatex
```

格式：`csljson`、`bibtex`、`biblatex`、`ris`。canonical 语法只接受位置 key，或用 `--from PATH|-` 读取 key 数组、item 数组及 `find --json` 响应；筛选统一先交给 `item find`，导出本身不再复制筛选参数。

## 写操作

```powershell
zot item new --set itemType=article --set title="Example" --dry-run --json
zot item new --data '{"itemType":"journalArticle","title":"Example"}' --json
zot item edit ITEMKEY --set abstractNote="Updated" --if-version 42 --json
zot item delete ITEMKEY --if-version 42 --yes --json
zot item tag KEY1 KEY2 --tag review --json
zot item untag KEY1 KEY2 --tag review --json
zot item import ./paper.pdf --collection "研究/植物/栗属" --json

zot coll new --name "Inbox" --json
zot coll add COLLKEY ITEM1 ITEM2 --json
zot coll remove COLLKEY ITEM1 --json
zot note new --parent ITEMKEY --text "Reading note" --json
zot search new --data '{"name":"Recent","conditions":[]}' --json
```

输入统一为 `--set`、`--data`、`--from`。安全参数统一为 `--dry-run`、`--yes`、`--if-version`。

`item import --collection` 接受收藏夹 key、唯一名称或完整层级路径。同名时返回带 key 的完整路径候选；导入依赖 Zotero 桌面端 Connector，并会为最终保留的附件增量建立全文索引。`--dry-run` 不要求写权限，只校验 PDF、Connector 和收藏夹，不上传文件或创建条目。

权限：

- `ZOT_ALLOW_WRITE=1`：允许创建和修改。
- `ZOT_ALLOW_DELETE=1`：允许删除，默认关闭。
- destructive action 必须明确目标并确认，不允许因 `--json` 或 legacy translation 静默绕过。

## PDF 与标注

```powershell
zot pdf text ITEMKEY --json
zot pdf text ITEMKEY --pages 3-8 --grep methods --max-chars 12000 --json
zot pdf text ITEM1 ITEM2 --grep "gene\s+flow|introgression" --json
zot pdf text --collection "研究/植物/栗属" --grep "gene\s+flow|introgression" --json
zot pdf text KEY1 KEY2 --output-dir ./markdown --json
zot file path ATTACHMENT_KEY --json
zot pdf figs ITEMKEY --output-dir ./figures --json
zot pdf open ITEMKEY --page 5

zot ann list ITEMKEY --type highlight --page 3 --json
zot ann list ITEMKEY --attachment ATTACHMENT_KEY --json
zot ann new ITEMKEY --attachment ATTACHMENT_KEY --page 4 --text "GATK" --color red --json
zot ann delete ITEMKEY --source zotero --type highlight --dry-run --json
zot ann delete ITEMKEY --source zotero --type highlight --yes --json
zot ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --type highlight --yes --json
```

全文路由优先级：

1. `item find --in fulltext --snippet`
2. `item show --snippet`
3. `pdf text`

local/hybrid 下，无过滤条件的 `pdf text --json` 返回 `content_path` 和可选的 `chunks_path`，Agent 直接读取提取文本缓存；它们不是 PDF 二进制副本，源文件用 `file path` 查询。`--grep`、`--pages`、`--max-chars` 才返回文本子集。`--grep` 默认按不区分大小写的 Go 正则解析；`--collection` 接受 key、唯一名称或完整层级路径。有分页缓存时结果包含附件、命中页、`match_count` 与上下文。检索保持只读。remote 模式仍返回正文。该命令不支持 worker 并发参数。

多 PDF 条目默认选择第一个 PDF；`ann list/new/delete` 可用 `--attachment ATTACHMENT_KEY` 精确选择。`ann new` 在临时副本中写入并验证后替换，实际写入零匹配时保留原文件并报错。`ann delete` 是唯一 canonical 删除入口，必须显式选择 `--source zotero|pdf`。Zotero 来源按 annotation item key 走 Web API，PDF 来源按 xref 在临时副本中修改并验证后替换；不要组合 `list/new` 与 `--clear`。

## Reference

```powershell
zot ref show ITEMKEY --json
zot ref find "query" --json
zot ref related ITEMKEY --limit 20 --json
zot ref cited ITEMKEY --external --json
zot ref ctx ITEMKEY --json
zot ref links ITEMKEY --json
zot ref entities ITEMKEY --json
zot ref profile ITEMKEY --json
zot ref build --workers 3 --json
zot ref build --scope failed --workers 2 --json
zot ref build --scope contexts --workers 3 --json
zot ref build --scope grobid --limit 5 --json
zot ref resolve --workers 8 --json
zot ref status --json
zot ref status --view failed --json
zot ref status --view unsupported --json
zot ref status --view grobid --json
```

来源规则：PMC JATS 优先；没有 PMC 时使用 PubMed；Europe PMC 是增强层，失败不应使 NCBI 核心结果失败；GROBID 是显式实验性后备。

## Schema

```powershell
zot schema list types --json
zot schema list fields --json
zot schema list fields article --json
zot schema list roles article --json
zot schema show article --json
```

`types|fields|roles` 是 `schema list` 的位置参数，不是更深一层 Cobra 子命令。

## 模式矩阵

| 能力 | web | local | hybrid | remote |
|---|---:|---:|---:|---:|
| Web 元数据读取 | ✅ | — | fallback | 取决于服务端/凭据 |
| 本地 SQLite / FTS | — | ✅ | ✅ | 由服务端提供 |
| 本地附件/PDF | — | ✅ | ✅ | 由服务端提供 |
| 普通 Web API 写入 | ✅ | 受限 | ✅ | 需 API key |
| `ann list/new/delete` | — | ✅ | ✅ | 由服务端门控 |

Hybrid 只在目标 backend 能保持请求语义时回退；全文、附件路径、附件健康等 local-only 请求不能伪装成 Web 能力。

`config check` 还会报告 `zotero_desktop_connector_available`、Connector 地址及不可用原因。Connector 不可用不影响其他配置项通过，但 `item import` 前必须启动 Zotero 桌面端。

## Remote

```powershell
# 服务端
zot serve

# 客户端配置
zot config init --mode remote --server-addr http://HOST:8021

# 一次性拉取到本地镜像
zot sync
zot sync status
zot sync status --full
zot --mode local find "query"
```

`sync` 使用配置中的服务器地址和默认镜像目录，单向增量拉取 SQLite、storage、可解析的 linked_file 外部附件和全文索引；外链文件保留 `attachments:` 后的安全相对路径并写入同步端 `~/.zot/sync/attachments/`，不依赖服务端绝对路径，也不改写 SQLite。远端删除不会清理本地附件或全文缓存，SQLite sidecar 除外；缺失的外链源文件不会中止同步，已有本地副本继续保留。同步后 `--mode local` 自动识别 `~/.zot/sync`。中断的大文件会自动续传，仅在需要重新下载全部文件时使用 `--force`。`sync status` 合并状态与校验；`--full` 会执行完整 SQLite 和上次 manifest 校验；具体附件位置使用 `file path` 查询。

## 兼容边界

旧 alias、旧参数翻译和 redirect-only adapter 已移除，旧入口返回 usage error。正式快捷入口只有 `find/show/export`，并且只接受 `item find/show/export` 的 canonical 参数。

## 常见错误

| 错误写法 | 正确写法 |
|---|---|
| `export --item-key KEY --format bibtex` | `item export KEY --as bibtex` |
| `add-tag --items A,B --tag T` | `item tag A B --tag T` |
| `extract-text KEY` | `pdf text KEY` |
| `annotations KEY --clear` | `ann delete KEY --source zotero --dry-run` |
| `schema fields-for journalArticle` | `schema list fields article` |
| `server start` | `serve` |
| `sync pull` | `sync` |
| `--start/--direction` | `--offset/--order` |
| `--if-unmodified-since-version` | `--if-version` |
