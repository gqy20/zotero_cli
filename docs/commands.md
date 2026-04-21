# 命令参考

完整用法、选项说明、模式边界和输出示例。

---

## 检索 (`find`)

```bash
zot find "query" [options]
```

### 基本用法

```bash
zot find "hybrid speciation"              # 关键词检索
zot find ""                            # 列出所有条目
zot find "query" --all                  # 不限数量
```

### 过滤选项

| 选项 | 说明 | 模式 |
|------|------|------|
| **元数据过滤** |||
| `--item-type TYPE` | 按文献类型过滤 | 全部 |
| `--no-type TYPE` | 排除某文献类型 | 全部 |
| **标签过滤** |||
| `--tag TAG` | 标签过滤（AND，可多个） | web / local / hybrid |
| `--tag-any` | 标签过滤（OR） | 同上 |
| `--tag-contains WORD` | 标签模糊匹配（包含指定词） | local / hybrid |
| `--exclude-tag TAG` | 排除含某标签的条目 | local / hybrid |
| **收藏夹过滤** |||
| `--collection KEY` | 仅返回指定收藏夹内的条目 | local / hybrid |
| `--no-collection KEY` | 排除某收藏夹内的条目 | local / hybrid |
| **日期过滤** |||
| `--date-after DATE` | 起始日期 `YYYY` / `YYYY-MM` / `YYYY-MM-DD` | web / local / hybrid |
| `--date-before DATE` | 截止日期 | 同上 |
| `--modified-within DURATION` | 最近修改时间范围（如 `7d`、`2w`） | local / hybrid |
| `--added-since DURATION` | 最近添加时间范围 | local / hybrid |
| **附件过滤** |||
| `--has-pdf` | 仅返回有 PDF 附件的条目 | local / hybrid |
| `--attachment-type TYPE` | 附件类型过滤 | local / hybrid |
| `--attachment-name TEXT` | 附件文件名包含指定文本 | local / hybrid |
| `--attachment-path TEXT` | 附件路径包含指定文本 | local / hybrid |
| **Web 专属** |||
| `--qmode MODE` | 查询模式：`titleCreatorYear` / `everything` | **web only** |
| `--include-trashed` | 包含回收站条目 | **web only** |
| **字段控制** |||
| `--include-fields FIELDS` | 指定返回字段（逗号分隔） | 全部 |

### 全文检索（local / hybrid）

| 选项 | 说明 |
|------|------|
| `--fulltext` | PDF 全文搜索（FTS5 索引） |
| `--fulltext-any` | 全文任一词匹配 |
| `--snippet` | 返回全文匹配片段预览 |

> **自动启用**：在 local / hybrid 模式下，如果 FTS5 全文索引已有数据，即使不指定 `--fulltext`，查询也会自动走全文检索路径。显式指定 `--fulltext` 可确保始终使用全文搜索。

### 输出控制

| 选项 | 说明 |
|------|------|
| `--json` | JSON 格式输出 |
| `--full` | 完整字段 + 附件详情 |
| `--sort FIELD` | 排序方式（如 `date`） |
| `--direction asc\|desc` | 排序方向（默认 `desc`） |
| `--start N` | 分页偏移量（从第 N 条开始） |
| `--limit N` | 返回结果数量上限 |

> **注意**：使用 `--snippet` 时，若未指定 `--limit`，默认限制为 **50** 条（批量提取安全限制）。

### 模式边界

- **web**：支持 `--qmode`、`--include-trashed`
- **local**：支持 metadata/标签/日期/附件过滤、全文检索、snippet；不支持 `--qmode`、`--include-trashed`
- **hybrid**：优先本地；仅 Web 能正确承接的请求才回退（如 `--qmode` 可回退，`--fulltext` 不可回退）

---

## 查看条目 (`show`)

```bash
zot show ITEMKEY [--json] [--snippet]
```

文本输出包含：Key、Title、Type、Creators、Tags、Collections、Attachments（含 resolved path）、Notes、Annotations。

Annotations 展示示例：

```
Annotations: 4
  - [note] color=#ffd400 page=2: Key finding | This is important
  - [highlight] color=#5c9eff page=5: A pure highlight text
  - [ink] color=#000000 page=1:  | Hand-drawn sketch
```

JSON 输出包含完整字段 + annotations（含 type/text/comment/color/pageIndex/position/dateAdded）。

---

## PDF 文本提取 (`extract-text`)

> 仅 **local** 和 **hybrid** 模式。依赖 PyMuPDF（需先 `zot init --pdf` 或在 init 交互中安装）。

```bash
zot extract-text ITEMKEY          # 提取主附件文本
zot extract-text ITEMKEY --json   # JSON（含缓存状态）
```

提取器优先级：**PyMuPDF**（首选）→ Zotero ft-cache（本地缓存回退）→ pdfium WASM（最终回退）。

---

## PDF 标注写入 (`annotate`)

> 仅 **local** 和 **hybrid** 模式。依赖 PyMuPDF。

```bash
# 文本模式：全页搜索该文本并标注
zot annotate KEY --text "homoploid hybrid" --color red --comment "关键概念"

# 矩形模式：指定页码+矩形区域
zot annotate KEY --page 3 --rect 100,200,400,250 --color yellow

# 点位模式：指定页码+位置添加笔记
zot annotate KEY --page 5 --point 150,300 --comment "重要发现" --type note

# 下划线
zot annotate KEY --text "speciation" --type underline --color blue
```

### 选项一览

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--text TEXT` | 要标注的原文（PyMuPDF 搜索匹配） | — |
| `--page N` | 目标页码（配合 rect/point） | — |
| `--rect x0,y0,x1,y2` | 标注区域矩形坐标 | — |
| `--point x,y` | 笔记/便签位置 | — |
| `--color` | 颜色：命名色或 `#rrggbb` | `yellow` |
| `--comment TEXT` | 批注文字 | — |
| `--type` | `highlight` / `underline` / `note` | `highlight` |
| `--json` | JSON 输出 | — |

三种定位模式互斥：提供 `--text` 时忽略 page/rect/point；提供 `--page` 时必须配合 `--rect` 或 `--point`。

---

## 打开 PDF (`open`)

```bash
zot open ITEMKEY           # 在 Zotero 阅读器中打开 PDF
zot open ITEMKEY --page 5 # 带页码跳转
```

行为逻辑：
- **Zotero 运行中** → `zotero://open-pdf/library/items/{attachmentKey}?page=N` 协议，复用已有实例
- **Zotero 未运行** → 启动新实例（优先 `zotero.exe --browser`，回退系统默认程序）

---

## 跳转到 Zotero UI (`select`)

```bash
zot select ITEMKEY    # 在已运行的 Zotero 中选中该条目
```

通过 `zotero://select/library/items/{key}` 协议实现。无需加载配置或连接数据库。

---

## PDF 标注读取 (`annotations`)

> 仅 **local** 和 **hybrid** 模式。依赖 PyMuPDF。

```bash
zot annotations KEY                    # 双源列出所有标注
zot annotations KEY --type highlight    # 仅高亮
zot annotations KEY --page 3            # 仅第 3 页
zot annotations KEY --json             # JSON（分组 db/pdf）
zot annotations KEY --clear --type highlight  # 删除 PDF 文件内的标注
```

### 双源输出

| 来源 | 数据来源 | 包含信息 |
|------|----------|----------|
| **Zotero Reader Annotations** (db) | SQLite `itemAnnotations` 表 | type, text, comment, color, page, **dateAdded** |
| **PDF File Annotations** (pdf) | PyMuPDF `page.annots()` 扫描 | type, color, rect, author, **modDate** |

`--clear` 只删除 PDF **文件内**的标注（PyMuPDF 写入的），不影响 Zotero 阅读器数据库中的标注。

### 选项

| 选项 | 说明 |
|------|------|
| `--json` | JSON 格式（分组显示 db/pdf 两源） |
| `--clear` | 删除模式 |
| `--page N` | 按页码过滤 |
| `--type TYPE` | 按类型过滤（highlight/note/underline） |

---

## 引文生成 (`cite`)

```bash
zot cite KEY                        # citation 格式（默认 apa 样式）
zot cite KEY --format bib           # BibTeX 格式
zot cite KEY --style chicago --locale zh-CN   # 指定引文样式和语言
zot cite KEY --json
```

### 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--format` | `citation`（内联引文）或 `bib`（参考文献条目） | `citation` |
| `--style` | CSL 引文样式名称（如 `apa`、`chicago`、`ieee`） | `apa` |
| `--locale` | 输出语言（如 `en-US`、`zh-CN`） | `en-US` |

---

## 导出 (`export`)

```bash
zot export "query" --format bibtex                    # 按关键词
zot export --item-key KEY --format ris                 # 按 key
zot export --collection COLL --format csljson --json     # 按收藏夹
```

**支持格式**：`bib` / `bibtex` / `biblatex` / `csljson` / `ris`

- `bib` / `bibtex` / `biblatex` / `ris` → Web API
- `csljson` → local / hybrid 优先本地导出，仅在预期缺失时回退 Web

---

## 关系查询 (`relate`)

```bash
zot relate KEY [--json]
```

读取 `itemRelations` 表中的显式关系（related to / containing 等）。**仅 local / hybrid 支持**。

---

## 列表命令

以下均支持 `--json`：

| 命令 | 说明 |
|------|------|
| `collections` | 收藏夹列表 |
| `collections-top` | 仅顶层收藏夹 |
| `notes` | 子笔记列表（支持 `--query` 过滤） |
| `tags` | 所有标签 |
| `searches` | 已保存搜索 |
| `trash` | 回收站条目 |
| `publications` | My Publications |
| `deleted` | 已删除对象 key |
| `stats` | 库统计（条目/收藏夹/搜索数） |
| `versions <type> --since N` | 版本变更列表（`items` / `collections` / `searches` / `items-top`） |

用法：`zot collections [--limit N] [--json]`、`zot notes [--query QUERY] [--limit N] [--json]`

### `notes` 过滤

```bash
zot notes                          # 列出所有子笔记
zot notes --query "CRISPR"        # 按关键词过滤笔记内容
zot notes --query "method" --limit 20 --json
```

### `versions` 子类型

```bash
zot versions items --since 100 --json              # 条目变更
zot versions collections --since 100 --json        # 收藏夹变更
zot versions searches --since 100 --json           # 已保存搜索变更
zot versions items-top --since 100 --json          # 顶层条目变更
zot versions items --since 0 --include-trashed --json   # 含回收站
zot versions items --since 100 --if-modified-since-version 120 --json  # 增量同步
```

---

## 元数据命令

| 命令 | 说明 |
|------|------|
| `item-types` | 可用文献类型 |
| `item-fields` | 可用字段 |
| `creator-fields` | 可用作者角色 |
| `item-type-fields TYPE` | 某类型的有效字段 |
| `item-type-creator-types TYPE` | 某类型的有效作者角色 |
| `item-template TYPE` | 新建条目模板 |
| `key-info API_KEY` | API key 权限信息 |
| `groups` | 可访问群组 |

用法：`zot item-types [--json]`

---

## 写操作

所有写操作受环境变量保护：
- 创建/更新默认允许（`ZOT_ALLOW_WRITE=1`）
- 删除默认禁止（`ZOT_ALLOW_DELETE=0`）
- 使用版本号乐观锁（`--if-unmodified-since-version N`）

### 条目 CRUD

```bash
zot create-item (--data JSON \| --from-file PATH) --if-unmodified-since-version N [--json]
zot update-item KEY (--data JSON \| --from-file PATH) --if-unmodified-since-version N [--json]
zot delete-item KEY --if-unmodified-since-version N [--json]
zot create-items (--data JSON \| --from-file PATH) --if-unmodified-since-version N [--json]
zot update-items (--data JSON \| --from-file PATH) [--if-unmodified-since-version N] [--json]
zot delete-items --items K1,K2 [--if-unmodified-since-version N] [--json]
```

### 标签操作

```bash
zot add-tag --items K1,K2 --tag TAG [--if-unmodified-since-version N]
zot remove-tag --items K1,K2 --tag TAG [--if-unmodified-since-version N]
```

### 收藏夹与搜索

```bash
zot create-collection (--data JSON \| --from-file PATH) --if-unmodified-since-version N [--json]
zot update-collection KEY (--data JSON \| --from-file PATH) [--if-unmodified-since-version N] [--json]
zot delete-collection KEY --if-unmodified-since-version N [--json]
zot create-search (--data JSON \| --from-file PATH) --if-unmodified-since-version N [--json]
zot update-search KEY (--data JSON \| --from-file PATH) [--if-unmodified-since-version N] [--json]
zot delete-search KEY --if-unmodified-since-version N [--json]
```

---

## 配置与环境

```bash
zot config path        # 配置文件路径
zot init               # 一键初始化 ~/.zot/.env（含模式选择和可选 PyMuPDF）
zot config show        # 当前配置（密钥脱敏）
zot config validate  # 校验 API key 和 library ID
zot version            # 版本信息
```

### 环境变量

| 变量 | 说明 | 默认 |
|------|------|------|
| `ZOT_MODE` | `web` / `local` / `hybrid` | `web` |
| `ZOT_DATA_DIR` | Zotero 数据目录（local/hybrid 必填） | — |
| `ZOT_LIBRARY_ID` | 库 ID（数字） | — |
| `ZOT_LIBRARY_TYPE` | `user` / `group` | `user` |
| `ZOT_API_KEY` | Zotero Web API 密钥 | — |
| `ZOT_STYLE` | 引文格式 | `apa` |
| `ZOT_LOCALE` | 输出语言 | `en-US` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_ALLOW_WRITE` | 允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 允许删除操作 | `0` |
| `ZOT_RETRY_MAX_ATTEMPTS` | API 请求最大重试次数（429 等可重试错误） | `5` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟毫秒数 | `1000` |
| `ZOT_RETRY_JITTER_FRACTION` | 重试抖动比例（避免惊群效应） | `0.3` |

### PDF 环境

```bash
zot init --check-pdf            # 检查 PyMuPDF 就绪状态
zot init --pdf                 # 安装 PyMuPDF（含配置初始化）
zot init --mode hybrid ... --pdf  # 非交互：一步完成配置 + PyMuPDF
```

自动在 `{ZOT_DATA_DIR}/.zotero_cli/venv/` 管理 Python 环境，优先 `uv`。

### 全文索引

```bash
zot index build [--force] [--workers N] [--json]
```
