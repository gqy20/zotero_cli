---
name: zotero-cli
description: 使用本仓库的 Zotero CLI 工具进行文献检索、管理、导出、PDF 操作、标注分析、引文生成和配置管理。支持 web/local/hybrid 三种模式，内置 HTTP API Server 和 React Web UI。当需要操作 Zotero 数据或进行文献分析时使用。
---

# Zotero CLI (`zot`)

优先使用本地 CLI，不要自行实现 Zotero API 调用。

## 快速开始

```shell
zot init                          # 一键初始化（推荐入口）
zot config validate               # 校验配置有效性
zot overview --json               # Agent 首选：一站式库概览
```

## 工作流程

1. 在项目根目录下工作。
2. 优先使用 `zot`（二进制存在且版本足够）。
3. Agent 工作流**始终加 `--json`**，设置 `ZOT_JSON_ERRORS=1` 获得结构化错误输出。
4. 操作前先运行 `config validate` 确认凭据可用。

### 模式选择

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `web` (默认) | 纯云端 Zotero Web API | 无本地 Zotero 安装 |
| `local` | 读本地 SQLite 数据库 | 大量读操作、PDF 标注/提取 |
| `hybrid` | 本地优先 + Web 回退 | 兼顾速度与完整性 |

> 写操作仅支持 `web` 和 `hybrid`（回退路径）。local 模式的写能力受限。

---

## 核心命令速查

### 库概览（Agent 入口）

```shell
zot overview --json        # 统计 + Top 收藏夹/标签 + 最近条目 + FTS 状态
zot stats --json           # 条目/收藏夹/搜索计数
```

### 文献检索

```shell
zot find --all --json                    # 全部条目
zot find "CRISPR gene editing" --json    # 关键词搜索
zot show ITEMKEY --json                  # 条目详情（含子笔记+标注）
zot relate ITEMKEY --json                # 关联条目
```

**基础过滤：**
- `--date-after YYYY[-MM[-DD]]` / `--date-before YYYY[-MM[-DD]]`
- 多次 `--tag`（AND） / `--tag-any`（OR）
- `--include-trashed` / `--qmode everything`（web only）

**高级过滤（local/hybrid）：**
- `--collection KEY` / `--no-collection KEY`
- `--tag-contains WORD` / `--exclude-tag TAG`
- `--no-type TYPE` / `--has-pdf`
- `--modified-within 7d` / `--added-since 2w`
- `--attachment-name TEXT` / `--attachment-path TEXT`

**输出控制：**
- `--include-fields url,doi,version` / `--full`
- `--sort FIELD` + `--direction asc|desc`
- `--start N` + `--limit N`（分页）

**全文检索（FTS5）：**
- local/hybrid 下 FTS 有数据时**自动启用**，无需手动 `--fulltext`
- `--snippet` 匹配片段预览（默认限制 **50** 条）
- `--fulltext-any` 任一词匹配

### 导出与引用

```shell
zot export --all --format csljson --json   # 全库导出
zot export --collection COLLKEY --format bibtex --json
zot cite ITEMKEY --format apa              # 单条引文
zot cite ITEMKEY --format bibliography     # 参考文献条目
```

### PDF 操作（需 local/hybrid + PyMuPDF）

```shell
# 正文提取（三级优先级：PyMuPDF → ft-cache → pdfium WASM）
zot extract-text ITEMKEY --json

# 双源标注读取（DB 标注 + PDF 文件内标注）
zot annotations ITEMKEY --json
zot annotations ITEMKEY --type highlight --page 3 --json
zot annotations ITEMKEY --author "User" --json          # 按作者过滤

# 写入标注到 PDF（三种模式）
zot annotate ITEMKEY --text "关键词" --color red          # Mode 1: 全文搜索
zot annotate ITEMKEY --page 4 --text "GATK" --comment "..." # Mode 1.5: 单页搜索 (推荐)
zot annotate ITEMKEY --page 3 --rect 100,200,350,220      # Mode 2: 精确坐标
zot annotate ITEMKEY --text "speciation" --type underline   # 下划线

# 清除标注（双层删除：PDF + DB）
zot annotations ITEMKEY --clear                           # 删除全部
zot annotate ITEMKEY --clear --type highlight             # 按类型删
zot annotations ITEMKEY --clear --page 5 --author "User"  # 组合条件删

# 与 Zotero 桌面端联动
zot open ITEMKEY --page 5         # 阅读器中打开 PDF
zot select ITEMKEY                # 主界面选中条目
```

**标注操作要点：**

| 要点 | 说明 |
|------|------|
| **推荐模式** | `--page N --text "keyword"` (Mode 1.5) — 精准定位，避免全文误匹配 |
| **`--point` 注意** | 创建浮动便签(circle)，不附着文本。需文本位置标注用 Mode 1.5 |
| **`--clear` 行为** | 双层删除：PDF 层随时可用，DB 层需要 **Zotero 关闭** |
| **DB 删除失败时** | 输出 warning 不阻断，PDF 层照常删除。关闭 Zotero 后重试即可 |
| **先 extract-text** | 用 `extract-text` 确认页面实际文本，选唯一短关键词做 `--text` |
| **详细文档** | 见 [annotations 示例](docs/user/examples/annotations.md) |

### 元数据 Schema

```shell
zot schema types --json                    # 所有文献类型
zot schema fields-for journalArticle       # 某类型的有效字段
zot schema template book --json            # 创建模板（含必填字段）
zot schema creator-types-for artwork       # 作者角色
```

### 其他只读命令

| 命令 | 说明 |
|------|------|
| `collections` | 收藏夹列表 |
| `tags` | 标签列表 |
| `notes [--query "..."]` | 笔记搜索 |
| `searches` | 已保存搜索 |
| `groups` | 可访问群组 |
| `trash` | 回收站 |
| `collections-top` | 顶层收藏夹 |
| `publications` | 我的发表 |
| `deleted` | 已删除对象 key |
| `versions --since N` | 版本变更检测 |
| `key-info` | API Key 权限信息 |
| `index [build\|status]` | FTS5 全文索引管理 |

---

## 写操作安全

以下命令会修改数据：

- `create-item` / `update-item` / `delete-item`
- `add-tag` / `remove-tag`
- `create-collection` / `update-collection` / `delete-collection`
- `create-search` / `update-search` / `delete-search`
- `annotate`（向 PDF 写入高亮/笔记/下划线）

执行前：
1. 确认用户意图。
2. 检查 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE` 环境变量。
3. 尽可能使用版本前置条件（`--version N`）。

**删除操作额外要求：**
1. 复述目标 key。
2. 无歧义确认。
3. 有任何不确定就先询问用户。

---

## 配置

存储位置：`~/.zot/.env`

```shell
zot init                                    # 交互式初始化
zot init --mode hybrid --api-key KEY ...    # 非交互模式
zot init --check-pdf                        # 诊断 PyMuPDF 状态
zot config show                              # 查看当前配置
zot config validate --json                   # 校验 + 结构化诊断
```

配置缺失时主动初始化，不要绕过错误。

### 环境变量速查

| 变量 | 说明 | 默认 |
|------|------|------|
| `ZOT_MODE` | `web` / `local` / `hybrid` | `web` |
| `ZOT_DATA_DIR` | Zotero 数据目录 | — |
| `ZOT_LIBRARY_ID` | 库 ID | — |
| `ZOT_API_KEY` | API 密钥 | — |
| `ZOT_STYLE` | 引文样式 | `apa` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_ALLOW_WRITE` | 允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 允许删除操作 | `0` |
| `ZOT_RETRY_MAX_ATTEMPTS` | 最大重试次数 | `5` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟 ms | `1000` |
| `ZOT_JSON_ERRORS` | 错误以 JSON 输出到 stdout | `0` |

---

## Web 前端 & API Server（规划中）

> **尚未集成到 CLI**。当前代码库包含前端源码（`web/`）和 Go server 框架，但 `zot web` 命令尚未实现。

规划能力：
- 10 个 REST 端点（GET only）：`health` / `stats` / `overview` / `items` / `collections` / `tags` / `notes` / `files` / `schema` / `export`
- 6 个前端页面：Dashboard / Library / ItemDetail（含 PDF 预览） / Search / Tags / Export
- 技术栈：React 19 + Vite 6 + Tailwind CSS 4 + TanStack Query 5

详见 [roadmap](https://github.com/gqy20/zotero_cli/blob/master/docs/plans/roadmap.md) Phase 1-4 进度。

---

## 性能注意

- `overview` 并行调用 4 个 API（~6s），优于逐个请求
- **local/hybrid 快照缓存**：Zotero 运行时自动走持久化快照缓存（`{dataDir}/.zotero_cli/snapshot/`），首次需复制（~2s），后续调用直接复用（~0.3s）。Zotero 关闭时直连 SQLite（~0ms）
- `extract-text` 结果有缓存，重复提取同一 PDF 直接命中
- `--snippet` 默认 limit 50，需更多结果显式加 `--limit`
- 高频脚本遇 `429` 会自动退避+抖动，但仍应主动降速

---

## 参考文档

| 文档 | 链接 |
|------|------|
| 完整命令参考与所有选项 | https://github.com/gqy20/zotero_cli/blob/master/docs/user/commands.md |
| 快速入门指南 | https://github.com/gqy20/zotero_cli/blob/master/docs/user/quickstart.md |
| 功能概览与安装方式 | https://github.com/gqy20/zotero_cli/blob/master/README.md |
| 版本规划与当前进度 | https://github.com/gqy20/zotero_cli/blob/master/docs/plans/roadmap.md |
| 各命令耗时基线 | https://github.com/gqy20/zotero_cli/blob/master/docs/reference/performance-baseline.md |
