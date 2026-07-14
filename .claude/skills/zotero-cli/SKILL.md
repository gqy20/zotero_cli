---
name: zotero-cli
version: "0.0.11"
description: >
  Zotero 文献管理 CLI（zot）v2。使用资源+动作命令检索、查看、导出和安全修改
  Zotero 文献库，读取 PDF/标注、构建引用与全文索引，并支持 web/local/hybrid/remote。
when_to_use: >
  用户要搜索、查看、筛选、导出或修改 Zotero 文献，读取 PDF 正文和标注，查询引用，
  检查配置、同步远端库，或生成结构化 JSON 时使用。
argument-hint: "<resource> <action> [ITEMKEY...] [options]"
---

# Zotero CLI v2

## Library taste（文献管理偏好）

涉及标签、收藏夹、分类层级、命名、合并或其他需要判断用户文献管理偏好的任务时，必须先运行：

```powershell
.\zot.exe lib taste --json
```

当 `data.exists=true` 时，完整读取 `data.content`，并按“当前用户指令 > taste.md > 工具默认行为”的优先级执行。文件不存在时，提示用户运行 `.\zot.exe lib taste init`，但不要阻塞其他安全操作。

优先调用仓库中的 `zot`/`zot.exe`，源码验证时使用 `go run .\cmd\zot`。不要自行重写 Zotero API 调用。

## 执行原则

1. 在仓库根目录工作。
2. Agent 默认加 `--json`，JSON 的 `command` 是 canonical path，例如 `item find`。
3. 首次使用先运行 `zot config check --json`。
4. 不确定参数时运行 `zot <resource> <action> --help`；help 不加载配置或网络。
5. 写入前确认用户意图，并检查 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE`。
6. 只生成 canonical 命令；除正式快捷入口 `find/show/export` 外，不尝试旧命令或旧参数。

## Canonical 命令树

```text
lib show|stats|taste|log
item list|find|show|new|edit|delete|tag|untag|supp|export|import
coll list|show|new|edit|delete|add|remove
note list|show|find|new|edit|delete
tag list|replace|apply|clean
search list|show|new|edit|delete
group list
file show|check
pdf text|figs|open
ann list|new|delete
ref show|find|related|cited|ctx|links|entities|profile|build|resolve|status
index build|status
schema list|show
config init|show|check
serve
sync
completion
version
```

`find`、`show`、`export` 是正式高频快捷入口，分别进入 `item find/show/export`，没有独立实现。

## 推荐入口

```powershell
zot lib show --json
zot lib stats --json
zot item find "query" --in metadata --limit 20 --json
zot item show ITEMKEY --json
zot item export ITEMKEY --as bibtex --json
zot ref show ITEMKEY --json
zot index status --json
```

最近入库使用 `dateAdded`：

```powershell
zot item find --all --sort dateAdded --order desc --limit 10 --json
zot item find --all --added-since 7d --sort dateAdded --order desc --json
```

统一分页参数为 `--limit`、`--offset`、`--sort`、`--order asc|desc`。

`item find` 未显式指定 `--limit` 时，轻量结果默认限制 100 条，`--snippet` 或 `--full` 默认限制 20 条。显式正数 `--limit` 覆盖默认值，只有显式 `--all` 才取消上限；JSON `meta` 提供 `returned`、`limit`、`offset`、`has_more` 和可选的 `next_offset`。

## Item type 归一化

CLI 输入接受短 alias，但应用层和 JSON 始终使用 Zotero 官方值：

| CLI alias | canonical value |
|---|---|
| `article` | `journalArticle` |
| `chapter` | `bookSection` |
| `conf` | `conferencePaper` |
| `web` | `webpage` |
| `blog` | `blogPost` |

例如：

```powershell
zot item find "CRISPR" --type article --json
zot schema list fields article --json
```

## 全文决策顺序

不要把整篇提取当作全文检索的默认入口：

1. 检索正文证据：`item find QUERY --in fulltext --snippet --json`
2. 单篇轻量预览：`item show ITEMKEY --snippet --json`
3. 明确需要整篇：`pdf text ITEMKEY --json`

```powershell
zot item find 'methods OR procedure*' --in fulltext --snippet --limit 20 --json
zot item show ITEMKEY --snippet --json
zot pdf text ITEMKEY --pages 3-8 --grep methods --max-chars 12000 --json
zot pdf text ITEM1 ITEM2 --grep "gene\s+flow|introgression" --json
zot pdf text --collection "研究/植物/栗属" --grep "gene\s+flow|introgression" --json
```

`pdf text` 会优先命中 `.zotero_cli/fulltext` 缓存；只有 cache miss 才重新提取。PDF 路径、大小或高精度修改时间变化时缓存自动失效，FTS 检索也不会返回已过期正文。local/hybrid 下无过滤条件的请求返回 `content_path` 和可选的 `chunks_path`，Agent 直接读取 `content_path`，不要期待 JSON 内嵌整篇正文。只有 `--grep`、`--pages` 或 `--max-chars` 返回文本子集。`--grep` 默认是不区分大小写的 Go 正则；多关键词直接使用 `|`，特殊字符按正则规则转义。`--collection` 接受收藏夹 key、唯一名称或完整层级路径。有分页缓存时，JSON 按附件和命中页返回 `match_count`、页码与上下文；检索不会自动创建标注。只有显式 `--output-dir` 才导出 Markdown；remote 模式仍返回正文。

## PDF、附件与标注

```powershell
zot item supp ITEMKEY --json
zot item import ./paper.pdf --collection "研究/植物/栗属" --json
zot file show ATTACHKEY --json
zot file check ATTACHKEY --json
zot pdf figs ITEMKEY --output-dir .\figures --json
zot pdf open ITEMKEY --page 5

zot ann list ITEMKEY --type highlight --page 3 --json
zot ann list ITEMKEY --attachment ATTACHMENT_KEY --json
zot ann new ITEMKEY --attachment ATTACHMENT_KEY --text "target phrase" --color yellow --json
zot ann delete ITEMKEY --source zotero --type highlight --dry-run --json
zot ann delete ITEMKEY --source zotero --type highlight --yes --json
zot ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --type highlight --yes --json
```

`item import --collection` 接受收藏夹 key、唯一名称或完整层级路径；名称有歧义时会列出带 key 的完整路径候选，不会自动猜测。导入依赖 Zotero 桌面端 Connector，并在识别完成后为最终保留的附件建立增量全文索引。`file check` 只检查附件健康状态；表格预览参数只属于 `file show`。

读取、创建、删除分别使用 `ann list/new/delete`。多 PDF 条目默认选择第一个 PDF，应优先使用 `--attachment ATTACHMENT_KEY` 精确选择。`ann new` 在临时副本中写入并验证后替换，非 dry-run 零匹配会失败且保留原 PDF。删除必须显式选择 `--source zotero|pdf`，并优先用 `--dry-run` 查看精确候选；不要生成 `annotations --clear` 或 `annotate --clear`，也不要直接修改 `itemAnnotations`。

## Reference

```powershell
zot ref show ITEMKEY --json
zot ref find 'genome AND assembl*' --in metadata --field mesh --json
zot ref related ITEMKEY --limit 20 --json
zot ref cited ITEMKEY --external --json
zot ref ctx ITEMKEY --json
zot ref links ITEMKEY --json
zot ref entities ITEMKEY --json
zot ref profile ITEMKEY --json

zot ref build --workers 3 --json
zot ref build --failed --workers 2 --json
zot ref build --contexts --workers 3 --json
zot ref resolve --workers 8 --json
zot ref status --json
zot ref status --failed --json
zot ref status --unsupported --json
```

PMC JATS 优先；没有 PMC 时使用 PubMed，并以 Europe PMC 增强。Europe PMC 失败不得阻断 NCBI 核心结果。GROBID 仅在显式 `--grobid` 时启用。

## 安全写入

```powershell
zot item new --set itemType=article --set title="Example" --dry-run --json
zot item edit ITEMKEY --set title="New" --if-version 42 --json
zot item tag KEY1 KEY2 --tag review --json
zot coll new --name "Reviews" --json
zot note new --parent ITEMKEY --text "Reading note" --json
```

写入输入统一为：

- `--set FIELD=VALUE`（可重复）
- `--data JSON`
- `--from PATH`，`--from -` 表示 stdin

安全参数统一为 `--dry-run`、`--yes/-y`、`--if-version`。

破坏性操作包括 `item/coll/note/search delete` 和 `ann delete`。执行前复述目标 key，确认无歧义，并遵守门控和确认要求。

## 配置与运行时

```powershell
zot config init
zot config show --json
zot config check --json
zot serve
zot sync
zot completion powershell
```

`config check` 会报告 `zotero_desktop_connector_available` 和 Connector 地址。Connector 不可用不会使整个配置检查失败，但执行 `item import` 前必须启动 Zotero 桌面端；导入失败时应优先展示 CLI 给出的启动提示。

模式：

- `web`：Zotero Web API
- `local`：本地 SQLite + storage
- `hybrid`：本地优先，仅在语义可保持时回退 Web
- `remote`：通过 `zot serve` 访问远端数据

pure remote 下 `ann list/new/delete` 可由服务端执行；其余 Web 写操作仍需 API key。

## 已退出稳定 CLI 的入口

旧 alias、旧参数翻译和 redirect-only adapter 已全部移除。`setup`、`abstract`、`key-info`、`select`、`relate` 等旧入口会返回 unknown command；不要生成或尝试执行它们。

仅 `find`、`show`、`export` 是受支持的正式快捷入口，且只接受对应 `item find/show/export` 的 canonical 参数。

详细参数、模式矩阵、兼容清单和 JSON 示例见 [reference.md](reference.md)、[find-output.md](examples/find-output.md) 与 [show-output.md](examples/show-output.md)。
