---
name: zotero-cli
description: 使用本仓库的本地 Zotero CLI 工具进行文献检索、查看、导出、配置校验和安全写操作。当需要通过 `zot.exe` 或 `go run .\\cmd\\zot` 操作 Zotero 数据时使用，适用于 `find`、`show`、`export`、stats、元数据查询、批量标签、PDF 操作和受保护的写/删工作流。支持 web/local/hybrid/remote 四种模式。
---

# Zotero CLI

优先使用本地 CLI，不要自行实现 Zotero API 调用。

## 工作流程

1. 在项目根目录下工作。
2. 优先使用 `.\zot.exe`（如果二进制文件存在且版本足够）。
3. 验证源码变更或二进制缺失时回退到 `go run .\cmd\zot ...`。
4. Agent 工作流优先使用 `--json`。
5. 假设凭据可用前先运行 `config validate`。

## 读优先默认命令

```powershell
.\zot.exe overview --json                   # 一站式库概览（Agent 入口推荐）
.\zot.exe stats --json
.\zot.exe find --all --json
.\zot.exe find --all --sort dateAdded --direction desc --limit 10 --json  # 最近入库
.\zot.exe find "query" --json
.\zot.exe show ITEMKEY --json
.\zot.exe export --collection COLLKEY --format csljson --json
.\zot.exe annotations ITEMKEY --json          # 读取 PDF 标注（双源）
.\zot.exe select ITEMKEY                     # 跳转到 Zotero UI 选中条目
```

## 全文相关决策规则

对“全文”请求，优先区分 **检索**、**预览**、**整篇读取** 三种意图，不要默认直接跑 `extract-text`。

### 1. 基于全文检索

当用户是在问“哪些文献提到 X”“正文里有没有 X”“找相关段落/证据”时，优先使用：

```powershell
.\zot.exe find "query" --fulltext --snippet --json
```

适用场景：

- 查关键词是否出现在 PDF 正文中
- 找命中段落或证据片段
- 先缩小候选文献范围

默认规则：

- local / hybrid 下优先走这条路径
- 已有 snippet 足够回答时，不再升级到 `extract-text`
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

- 能用 `show --snippet` 回答时，不直接跑 `extract-text`
- 需要时可先配合 `show ITEMKEY --json` 看摘要、附件、标签、元数据

### 3. 整篇正文读取

只有在用户明确需要整篇正文，或 snippet / abstract 不足以完成任务时，才使用：

```powershell
.\zot.exe extract-text ITEMKEY --json
```

适用场景：

- 明确要求“读取全文/整篇正文”
- 逐段总结、方法细读、结果抽取
- 需要把全文作为长上下文交给后续分析

默认规则：

- `extract-text` 是重操作，不要把它当作全文检索的默认入口
- 它会优先读取 `.zotero_cli/fulltext` 缓存；只有缓存 miss 时才重新提取
- 除非用户明确要整篇，或轻量路径不够，否则不要直接调用

### 4. 推荐路由顺序

优先顺序始终是：

1. `find --fulltext --snippet`
2. `show ITEMKEY --snippet`
3. `extract-text ITEMKEY`

如果用户需求仍然模糊，默认先走更轻的路径，而不是先拿整篇正文。

## 时间查询决策规则

### 最近入库 / 最近添加

用户问“最近入库”“今天刚添加”“最新加入 Zotero 的文献”时，用 `dateAdded`，不要用发表日期 `date`：

```powershell
.\zot.exe find --all --sort dateAdded --direction desc --limit 10 --json
.\zot.exe find --all --added-since 7d --sort dateAdded --direction desc --json
```

如果只是快速人工扫标题，优先用文本模式的轻量字段：

```powershell
.\zot.exe find --all --sort dateAdded --direction desc --limit 10 --include-fields title,date_added,container
```

注意：`--include-fields` 主要影响文本模式；`--json` 默认返回完整 Item 结构。

### 发表时间范围

用户问“某个时间范围内发表的文献”时，用发表日期过滤：

```powershell
.\zot.exe find --all --date-after 2026-03 --date-before 2026-03 --sort date --direction desc --json
```

日期输入支持 `YYYY` / `YYYY-MM` / `YYYY-MM-DD`。local/hybrid 会兼容 Zotero 常见的部分日期字符串，如 `YYYY-MM-00 YYYY-MM` 和 `MM/YYYY`。

### 最近修改

用户问“最近修改/最近更新过元数据”时，用：

```powershell
.\zot.exe find --all --modified-within 7d --sort dateModified --direction desc --json
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
- `--sort FIELD` + `--direction asc|desc` — 排序
- `--start N` + `--limit N` — 分页

**全文检索：**
- `--fulltext` — FTS5 全文搜索；local/hybrid 下有 query 且非 `--all` 时可自动启用
- `--fulltext-any` — 任一词匹配
- `--snippet` — 布尔开关，启用 FTS5 匹配片段预览（未指定 `--limit` 时回退 50 条）

文本模式辅助选项：

- `--include-fields url,doi,version`
- `--full`

## PDF 操作（需 local 模式 + PyMuPDF）

```powershell
# 提取 PDF 正文（重操作；先读缓存，miss 时才重新提取）
.\zot.exe extract-text ITEMKEY --json

# 双源读取标注
.\zot.exe annotations ITEMKEY --json
.\zot.exe annotations ITEMKEY --type highlight --page 3 --json
# 删除 PDF 文件内标注
.\zot.exe annotations ITEMKEY --clear --type highlight

# 写入标注到 PDF
.\zot.exe annotate ITEMKEY --text "关键概念" --color red --comment "重要"
.\zot.exe annotate ITEMKEY --text "speciation" --type underline --color blue

# 与 Zotero 桌面端联动
.\zot.exe open ITEMKEY --page 5        # 阅读器中打开 PDF
.\zot.exe select ITEMKEY               # 主界面选中条目
```

## 笔记查询

```powershell
.\zot.exe notes --json
.\zot.exe notes --query "CRISPR" --limit 20 --json
```

## 写操作安全

以下命令属于**写操作**：

- `create-item` / `update-item`
- `create-items` / `update-items`
- `add-tag` / `remove-tag`
- `create-collection` / `update-collection`
- `create-search` / `update-search`
- `annotate`（向 PDF 文件写入高亮/笔记）

以下命令属于**破坏性操作**：

- `delete-item` / `delete-items`
- `delete-collection` / `delete-search`

执行任何写操作前：

1. 确认用户确实要修改数据。
2. 检查 `ZOT_ALLOW_WRITE` 和 `ZOT_ALLOW_DELETE` 是否允许该操作。
3. 尽可能使用版本前置条件。

> **remote 模式**：当配置了 `ZOT_API_KEY` 时，remote 模式（remote+web）同样支持写操作，遵循与 web 模式相同的写/删安全规范。

> **补充**：`annotations` / `annotate` 属于例外。它们在 pure remote 下也可通过远端 `zot server` 执行，不要求客户端配置 `ZOT_API_KEY`；是否允许写入/清除由服务端 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE` 控制。

执行任何删除操作前：

1. 复述目标 key 或 keys。
2. 确认无歧义。
3. 请求有任何不确定就先询问用户。

## 配置

CLI 配置存储在 `~/.zot/.env`。

常用命令：

```powershell
.\zot.exe init                    # 一键初始化（推荐，含模式选择和可选 PyMuPDF 安装）
.\zot.exe init --mode hybrid --api-key ...  # 非交互模式
.\zot.exe init --mode remote --server-addr http://192.168.1.100:8021
.\zot.exe config show       # 查看当前配置
.\zot.exe config validate   # 校验配置有效性
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

- `--snippet` 是布尔开关（启用片段预览）；未指定 `--limit` 时回退 50 条
- local/hybrid 下有 query 且 FTS 有数据时自动启用全文检索；`--all` / 纯过滤列表不会自动走全文索引
- `extract-text` 结果有缓存，重复提取同一 PDF 直接命中
- 高频脚本遇 `429` 会自动退避+抖动，但仍应主动降速

## 参考文档

按需查阅：

- `docs/AI_AGENT.md` — Agent 使用模式与安全规范（完整版）
- `docs/commands.md` — 完整命令参考与所有选项说明
- `README.md` — 用户快速开始与功能概览
