---
name: zotero-cli
description: 使用本仓库的本地 Zotero CLI 工具进行文献检索、查看、导出、配置校验和安全写操作。当需要通过 `zot.exe` 或 `go run .\\cmd\\zot` 操作 Zotero 数据时使用，适用于 `find`、`show`、`export`、stats、元数据查询、批量标签、PDF 操作和受保护的写/删工作流。支持 web/local/hybrid/remote 四种模式。
---

# Zotero CLI

## Library taste（文献管理偏好）

涉及标签、收藏夹、分类层级、命名、合并或其他需要判断用户文献管理偏好的任务时，必须先运行：

```powershell
.\zot.exe lib taste --json
```

当 `data.exists=true` 时，完整读取 `data.content`，并按“当前用户指令 > taste.md > 工具默认行为”的优先级执行。文件不存在时，提示用户运行 `.\zot.exe lib taste init`，但不要阻塞其他安全操作。

优先使用本地 CLI，不要自行实现 Zotero API 调用。

## 工作流程

1. 在项目根目录下工作。
2. 优先使用 `.\zot.exe`（如果二进制文件存在且版本足够）。
3. 验证源码变更或二进制缺失时回退到 `go run .\cmd\zot ...`。
4. Agent 工作流优先使用 `--json`。
5. 假设凭据可用前先运行 `config check`。

## 读优先默认命令

```powershell
.\zot.exe lib show --json                   # 一站式库概览（Agent 入口推荐）
.\zot.exe lib taste --json                  # 读取当前文献库的长期管理偏好
.\zot.exe lib stats --json
.\zot.exe tag list --json                   # JSON 包含 type：0=手动，1=自动
.\zot.exe tag clean --match '^[\x00-\x7F]+$' --max-items 1 --json # 默认仅预览低频自动英文标签
.\zot.exe find --all --json
.\zot.exe find --sort dateAdded --order desc --limit 10 --json  # 最近入库
.\zot.exe find "query" --json
.\zot.exe show ITEMKEY --json
.\zot.exe ref show ITEMKEY --json              # PMC/PubMed 核心 + Europe PMC 自动补全
.\zot.exe ref build --workers 3 --json         # 全库增量参考文献索引
.\zot.exe ref resolve --workers 8 --json        # 并行将参考文献匹配回本地条目
.\zot.exe ref find '"phrase" OR prefix*' --in all --json # 原生 SQLite FTS5 查询
.\zot.exe ref find '"query"' --in metadata --field mesh --json # 精确搜索 PubMed MeSH
.\zot.exe ref related ITEMKEY --limit 20 --json   # PubMed 官方相似文献
.\zot.exe ref links ITEMKEY --json                # 合并 NCBI 与 Europe PMC 生物医学资源
.\zot.exe ref cited ITEMKEY --external --json  # Europe PMC 外部被引网络
.\zot.exe ref entities ITEMKEY --json          # Europe PMC 实体/关系并写入按需索引
.\zot.exe ref profile ITEMKEY --json              # 版本、评价、基金与开放获取画像
.\zot.exe ref status --view unsupported --json                # 查看 NCBI 不支持、待 GROBID 的条目
.\zot.exe ref cited ITEMKEY --json            # 查询哪些已索引条目引用该文献
.\zot.exe ref ctx ITEMKEY --json            # 查询 PMC 正文引用语境
.\zot.exe ref build --scope contexts --workers 3 --json # 补建历史 PMC 引用语境
.\zot.exe ref status --view grobid --json              # 实验性：检查可选 PDF 后备
.\zot.exe ref build --scope grobid --limit 5 --json     # 实验性：显式小批量解析
.\zot.exe find --collection COLLKEY --all --json | .\zot.exe item export --from - --as csljson
.\zot.exe ann list ITEMKEY --json          # 读取 PDF 标注（双源）
```

`ref` 路由规则：PMC JATS 优先；没有 PMC 时使用 PubMed ELink，并以 Europe PMC references 补充非 PMID 引用。Europe PMC 失败不得阻断 NCBI 核心结果。`ref entities` 是 Europe PMC 文本挖掘实体，顶层 `annotations` 才是 Zotero/PDF 人工标注。完整说明见 `docs/user/references.md`。

## 全文相关决策规则

对“全文”请求，优先区分 **检索**、**预览**、**整篇读取** 三种意图，不要默认直接跑 `pdf text`。

### 1. 基于全文检索

当用户是在问“哪些文献提到 X”“正文里有没有 X”“找相关段落/证据”时，优先使用：

```powershell
.\zot.exe find '"phrase" OR prefix*' --in fulltext --snippet --json
```

适用场景：

- 查关键词是否出现在 PDF 正文中
- 找命中段落或证据片段
- 先缩小候选文献范围

默认规则：

- local / hybrid 下优先走这条路径
- 已有 snippet 足够回答时，不再升级到 `pdf text`
- 需要更多结果时再显式增加 `--limit`

### 2. 单篇正文预览

当用户已经指定条目，但只是想快速看正文相关内容，而不是读取整篇正文时，优先使用：

```powershell
.\zot.exe show ITEMKEY --snippet --json
```

适用场景：

- “先看看这篇讲了什么”
- “给我这篇的正文预览”
- “先看一下正文里和这个主题最相关的一段”

默认规则：

- 能用 `show --snippet` 回答时，不直接跑 `pdf text`
- 需要时可先配合 `show ITEMKEY --json` 看摘要、附件、标签、元数据

### 3. 整篇正文读取

只有在用户明确需要整篇正文，或 snippet / abstract 不足以完成任务时，才使用：

```powershell
.\zot.exe pdf text ITEMKEY --json
.\zot.exe pdf text ITEM1 ITEM2 --grep "gene\s+flow|introgression" --json
.\zot.exe pdf text --collection "研究/植物/栗属" --grep "gene\s+flow|introgression" --json
```

local / hybrid 下该命令默认确保项目全文缓存存在，并返回 `content_path` / `chunks_path`；Agent 直接读取 `content_path`，不要期待 JSON 内嵌整篇正文。只有使用 `--grep`、`--pages` 或 `--max-chars` 时才返回文本子集；`--grep` 默认解析为不区分大小写的 Go 正则，可直接用 `|` 表达多个候选词，特殊字符按正则规则转义。指定条目范围时可传多个 item key，或用 `--collection` 传收藏夹 key、唯一名称或完整层级路径。有分页缓存时，JSON 按附件和命中页返回 `match_count`、页码与相邻上下文；检索保持只读，不会自动创建标注。只有显式 `--output-dir` 才导出 Markdown。remote 模式因客户端无法访问服务端路径，仍返回正文。

适用场景：

- 明确要求“读取全文/整篇正文”
- 逐段总结、方法细读、结果抽取
- 需要把全文作为长上下文交给后续分析

默认规则：

- `pdf text` 是重操作，不要把它当作全文检索的默认入口
- 它会优先读取 `.zotero_cli/fulltext` 缓存；只有缓存 miss 时才重新提取。PDF 路径、大小或高精度修改时间变化时缓存自动失效
- 除非用户明确要整篇，或轻量路径不够，否则不要直接调用

### 4. 推荐路由顺序

优先顺序始终是：

1. `find --in fulltext --snippet`
2. `show ITEMKEY --snippet`
3. `pdf text ITEMKEY`

如果用户需求仍然模糊，默认先走更轻的路径，而不是先拿整篇正文。

## 时间查询决策规则

### 最近入库 / 最近添加

用户问“最近入库”“今天刚添加”“最新加入 Zotero 的文献”时，用 `dateAdded`，不要用发表日期 `date`：

```powershell
.\zot.exe find --sort dateAdded --order desc --limit 10 --json
.\zot.exe find --all --added-since 7d --sort dateAdded --order desc --json
```

如果只是快速人工扫标题，优先用文本模式的轻量字段：

```powershell
.\zot.exe find --sort dateAdded --order desc --limit 10 --include-fields title,date_added,container
```

注意：`--include-fields` 主要影响文本模式；`--json` 默认返回完整 Item 结构。

### 发表时间范围

用户问“某个时间范围内发表的文献”时，用发表日期过滤：

```powershell
.\zot.exe find --all --date-after 2026-03 --date-before 2026-03 --sort date --order desc --json
```

日期输入支持 `YYYY` / `YYYY-MM` / `YYYY-MM-DD`。local/hybrid 会兼容 Zotero 常见的部分日期字符串，如 `YYYY-MM-00 YYYY-MM` 和 `MM/YYYY`。

### 最近修改

用户问“最近修改/最近更新过元数据”时，用：

```powershell
.\zot.exe find --all --modified-within 7d --sort dateModified --order desc --json
```

利用 `find` 的过滤能力减少额外请求：

**基础过滤：**
- `--date-after YYYY[-MM[-DD]]`
- `--date-before YYYY[-MM[-DD]]`
- 多次使用 `--tag`（AND） / `--tag-any`（OR）
- `--include-trashed`（web only）
- `--qmode everything`（web only）

**高级过滤（local / hybrid）：**
- `--no-type TYPE` — 排除某文献类型
- `--tag-contains WORD` — 标签模糊匹配
- `--exclude-tag TAG` — 排除含某标签
- `--collection KEY` — 按收藏夹过滤
- `--no-collection KEY` — 排除收藏夹
- `--modified-within DURATION` — 最近修改（如 `7d`、`2w`）
- `--added-since DURATION` — 最近添加
- `--has-pdf` — 仅有 PDF 附件的条目
- `--attachment-name TEXT` — 附件文件名匹配
- `--attachment-path TEXT` — 附件路径匹配

**输出控制：**
- `--include-fields url,doi,version` — 文本模式指定额外字段；JSON 默认返回完整 Item
- `--full` — 完整字段 + 附件详情
- `--sort FIELD` + `--order asc|desc` — 排序
- `--offset N` + `--limit N` — 分页

**全文检索：**
- `--in metadata|fulltext` — 唯一的检索范围选择器；默认 `metadata`，全文范围只支持 local/hybrid
- fulltext 的 QUERY 直接使用 SQLite FTS5：`"完整短语"`、`prefix*`、`AND` / `OR` / `NOT`、括号
- `--snippet` — 启用 FTS5 匹配片段预览，且要求 `--in fulltext`（未指定 `--limit` 时默认 20 条）

文本模式辅助选项：

- `--include-fields url,doi,version`
- `--full`

## PDF 操作（需 local 模式 + PyMuPDF）

```powershell
# 提取 PDF 正文（重操作；先读缓存，miss 时才重新提取）
.\zot.exe pdf text ITEMKEY --json
.\zot.exe pdf text ITEM1 ITEM2 --grep "hybridization|introgression" --json
.\zot.exe pdf text --collection "研究/植物/栗属" --grep "hybridization|introgression" --json

# 双源读取标注
.\zot.exe ann list ITEMKEY --json
.\zot.exe ann list ITEMKEY --type highlight --page 3 --json
.\zot.exe ann list ITEMKEY --attachment ATTACHMENT_KEY --json
# 分来源预览并删除标注
.\zot.exe ann delete ITEMKEY --source zotero --type highlight --dry-run --json
.\zot.exe ann delete ITEMKEY --source zotero --type highlight --yes --json
.\zot.exe ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --type highlight --yes --json

# 写入标注到 PDF
.\zot.exe ann new ITEMKEY --attachment ATTACHMENT_KEY --text "关键概念" --color red --comment "重要"
.\zot.exe ann new ITEMKEY --text "speciation" --type underline --color blue

# 与 Zotero 桌面端联动
.\zot.exe pdf open ITEMKEY --page 5        # 阅读器中打开 PDF
```

## 笔记查询

```powershell
.\zot.exe note list --json
.\zot.exe note find 'CRISPR|gene\s+editing' --limit 20 --json
```

`note find QUERY` 默认按不区分大小写的 Go 正则解析；`note list` 只枚举笔记。导出只接收明确 item key，或 `--from PATH|-` 读取 key 数组、item 数组及 `find --json` 响应；所有筛选先由 `find` 完成。

## 写操作安全

以下命令属于**写操作**：

- `item new` / `item edit` / `item tag` / `item untag`
- `tag replace` / `tag apply` / `tag clean --yes`
- `coll new` / `coll edit` / `coll add` / `coll remove`
- `note new` / `note edit`
- `search new` / `search edit`
- `ann new`（向 PDF 文件写入高亮/笔记）

以下命令属于**破坏性操作**：

- `item delete` / `coll delete` / `note delete` / `search delete`
- `ann delete`

条目有多个 PDF 时，`ann list/new/delete` 默认使用第一个 PDF；应优先用 `--attachment ATTACHMENT_KEY` 精确选择。`ann new` 在临时副本中写入并验证后替换，非 dry-run 零匹配会失败且保留原 PDF。`ann delete` 必须显式使用 `--source zotero|pdf`，并优先执行 `--dry-run`。Zotero 来源按 annotation item key 走 Web API，PDF 来源按 xref 事务式修改文件；不要直接修改 `itemAnnotations`。

执行任何写操作前：

1. 确认用户确实要修改数据。
2. 检查 `ZOT_ALLOW_WRITE` 和 `ZOT_ALLOW_DELETE` 是否允许该操作。
3. 尽可能使用版本前置条件。

> **remote 模式**：当配置了 `ZOT_API_KEY` 时，remote 模式（remote+web）同样支持写操作，遵循与 web 模式相同的写/删安全规范。

> **补充**：`ann list/new/delete` 属于例外。它们在 pure remote 下也可通过远端 `zot server` 执行，不要求客户端配置 `ZOT_API_KEY`；是否允许写入/删除由服务端 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE` 控制。

执行任何删除操作前：

1. 复述目标 key 或 keys。
2. 确认无歧义。
3. 请求有任何不确定就先询问用户。

## 配置

CLI 配置存储在 `~/.zot/.env`。

常用命令：

```powershell
.\zot.exe config init                    # 一键初始化（推荐，含模式选择和可选 PyMuPDF 安装）
.\zot.exe config init --mode hybrid --api-key ...  # 非交互模式
.\zot.exe config init --mode remote --server-addr http://192.168.1.100:8021
.\zot.exe sync --server-addr http://host:8021 # 整库同步到本地 ~/.zot/sync/，之后 ZOT_MODE=local 离线用
.\zot.exe config show       # 查看当前配置
.\zot.exe config check   # 校验配置有效性
```

配置缺失时，主动初始化而不是绕过错误。

### 环境变量速查

| 变量 | 说明 | 默认 |
|------|------|------|
| `ZOT_MODE` | `web` / `local` / `hybrid` / `remote` | `web` |
| `ZOT_DATA_DIR` | Zotero 数据目录 | — |
| `ZOT_LIBRARY_ID` | 库 ID | — |
| `ZOT_API_KEY` | API 密钥 | — |
| `ZOT_SERVER_ADDR` | remote 模式服务器地址 | — |
| `ZOT_STYLE` | 引文样式 | `apa` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_ALLOW_WRITE` | 允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 允许删除操作 | `0` |
| `ZOT_RETRY_MAX_ATTEMPTS` | 最大重试次数 | `5` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟 ms | `1000` |
| `ZOT_JSON_ERRORS` | 错误以 JSON 输出到 stdout（agent 解析用） | `0` |

## 性能注意

- `find` 轻量结果未指定 `--limit` 时默认 100 条；`--snippet` 或 `--full` 默认 20 条；`--all` 仅取消上限并与 `--limit` 互斥，metadata 范围可省略 QUERY
- local/hybrid 下必须显式使用 `--in fulltext` 才查询全文索引；默认 `metadata` 不会根据索引状态改变查询语义
- `pdf text` 结果有缓存，重复提取同一 PDF 直接命中；local / hybrid 的无过滤请求默认返回缓存路径
- 高频脚本遇 `429` 会自动退避+抖动，但仍应主动降速

## 参考文档

按需查阅：

- `docs/AI_AGENT.md` — Agent 使用模式与安全规范（完整版）
- `docs/commands.md` — 完整命令参考与所有选项说明
- `README.md` — 用户快速开始与功能概览
