# zot

一个面向终端、脚本和 AI agent 的 Zotero CLI 工具。

`zot` 覆盖了完整的 Zotero Web API 读写能力，支持从本地 SQLite 数据库直接读取（含全文检索、PDF 文本提取、高亮/注释提取），并提供 `hybrid` 模式实现本地优先 + 云端回退。它适合这些场景：

- 在终端里快速检索、查看和导出 Zotero 条目
- 离线浏览本地库：条目、附件、笔记、关系、**PDF 高亮与批注**
- 提取 PDF 全文或附件文本，供 AI agent / LLM 消费
- 给脚本或 agent 提供稳定的 `--json` 输出
- 批量打标签、更新、删除条目
- 做库统计、版本同步和配置校验

## 快速开始

### 构建

```powershell
git clone https://github.com/gqy20/zotero_cli.git
cd zotero_cli
go build -o zot.exe .\cmd\zot
```

> 需要 Go 1.26+。项目使用 `modernc.org/sqlite`（纯 Go 实现）和 `go-pdfium`（WASM PDF 引擎），无需 CGO，跨平台编译。

### 初始化配置

```powershell
.\zot.exe config init
```

交互式引导填写并写入 `~/.zot/.env`：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ZOT_MODE` | 运行模式：`web` / `local` / `hybrid` | — |
| `ZOT_DATA_DIR` | Zotero 数据目录路径（local/hybrid 必填） | — |
| `ZOT_LIBRARY_TYPE` | 库类型：`user` 或 `group` | — |
| `ZOT_LIBRARY_ID` | 库 ID（数字） | — |
| `ZOT_API_KEY` | Zotero Web API 密钥（web/hybrid 必填） | — |
| `ZOT_STYLE` | 引文格式（APA、Chicago 等） | `apa` |
| `ZOT_LOCALE` | 输出语言区域 | `en-US` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_RETRY_MAX_ATTEMPTS` | 最大重试次数 | `3` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟 (ms) | `250` |
| `ZOT_ALLOW_WRITE` | 是否允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 是否允许删除操作 | `0` |

初始化过程中会打印以下页面链接：

- API keys: https://www.zotero.org/settings/keys
- Group IDs: https://www.zotero.org/groups
- Web API basics: https://www.zotero.org/support/dev/web_api/v3/basics

#### 三种运行模式

**Web 模式** — 纯云端 API：

```env
ZOT_MODE=web
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
ZOT_API_KEY=replace-me
```

**Local 模式** — 纯本地 SQLite，离线可用：

```env
ZOT_MODE=local
ZOT_DATA_DIR=D:\zotero\zotero_file
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
```

**Hybrid 模式**（推荐）— 本地优先，但只会在 **Web 确实能够承接该请求** 时回退 Web API：

```env
ZOT_MODE=hybrid
ZOT_DATA_DIR=D:\zotero\zotero_file
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
ZOT_API_KEY=replace-me
```

#### Hybrid 回退规则

- `find` 在 `--qmode`、`--include-trashed` 这类 Web 能力上，会在 `hybrid` 下自动回退到 Web。
- `find` 在 `--fulltext`、`--snippet`、附件过滤这类 local-only 能力上，`hybrid` 不会伪装成 Web 查询；如果本地临时不可用，会直接保留本地错误。
- `relate` 依赖本地 SQLite `itemRelations`，`hybrid` 不会回退到 `web` 模式制造误导性结果。
- `export --format csljson` 在 `hybrid` 下会优先尝试本地导出；只有遇到可预期的本地缺失/暂时不可用场景时才回退到 Web。

#### 安全策略

- 创建 / 更新默认允许 (`ZOT_ALLOW_WRITE=1`)
- 删除默认禁止 (`ZOT_ALLOW_DELETE=0`)
- 写操作使用版本号乐观锁 (`--if-unmodified-since-version`)

### 验证配置

```powershell
.\zot.exe config show      # 显示当前配置
.\zot.exe config validate  # 校验配置有效性
.\zot.exe version          # 显示版本信息
```

---

## 命令参考

### 检索 (`find`)

```powershell
# 基本检索
.\zot.exe find "hybrid speciation"
.\zot.exe find ""                          # 列出所有条目
.\zot.exe find "hybrid speciation" --all   # 不限制数量

# 过滤选项
.\zot.exe find "genome" --item-type journalArticle
.\zot.exe find "genome" --tag "物种形成" --tag "经典案例"
.\zot.exe find "genome" --tag "物种形成" --tag-any    # 任一标签匹配
.\zot.exe find "genome" --date-after 2020
.\zot.exe find "genome" --date-after 2020-01 --date-before 2024-12-31
.\zot.exe find "genome" --has-pdf                      # 仅返回有 PDF 附件的条目
.\zot.exe find "genome" --attachment-type pdf           # 按附件类型过滤

# 全文检索（local/hybrid 模式）
.\zot.exe find "speciation" --fulltext                  # Zotero 内置全文索引
.\zot.exe find "speciation" --fulltext --fulltext-any   # 任一词匹配

# 全文片段（需要本地全文可读）
.\zot.exe find "speciation" --fulltext --snippet

# 实验性全文索引（需设置 ZOT_EXPERIMENTAL_FTS=1）
.\zot.exe find "CRISPR" --full                          # 使用本地构建的 PDF 全文索引

# 输出控制
.\zot.exe find "genome" --json                          # JSON 格式
.\zot.exe find "genome" --include-fields url,doi,version
.\zot.exe find "genome" --start 0 --limit 20            # 分页
.\zot.exe find "genome" --sort date-desc                # 排序
```

**支持的选项一览**：

| 选项 | 说明 | 模式限制 |
|------|------|----------|
| `--all` | 不限制返回数量 | 全部 |
| `--item-type` | 按文献类型过滤 | 全部 |
| `--tag` / `--tag-any` | 标签过滤（AND / OR） | web / local / hybrid |
| `--date-after` / `--date-before` | 日期范围 | web / local / hybrid |
| `--has-pdf` / `--attachment-type` | 附件过滤 | local / hybrid |
| `--qmode` | 查询模式（titleCreatorYear / everything） | web only |
| `--include-trashed` | 包含回收站 | web only |
| `--include-fields` | 指定包含字段 | 全部 |
| `--fulltext` | Zotero 全文索引搜索 | local / hybrid |
| `--fulltext-any` | 全文任一词匹配 | local / hybrid |
| `--snippet` | 返回全文匹配片段 | local / hybrid |
| `--full` | 完整字段 + 附件详情 | 全部 |
| `--sort` | 排序方式 | local / hybrid |
| `--start` / `--limit` | 分页 | 全部 |
| `--json` | JSON 输出 | 全部 |

**`find` 的模式边界**：

- `web`：支持标准 Web API 查询语义，包括 `--qmode`、`--include-trashed`
- `local`：支持 metadata 检索、标签/日期过滤、附件过滤、全文检索、snippet，但不支持 `--qmode`、`--include-trashed`
- `hybrid`：优先本地；仅在 `web` 能正确承接请求时才回退。也就是说：
  - `find --qmode ...`、`find --include-trashed` 可回退到 Web
  - `find --fulltext`、`find --snippet`、附件过滤不可回退到 Web

### 查看条目 (`show`)

```powershell
.\zot.exe show ITEM1234                    # 文本格式
.\zot.exe show ITEM1234 --json             # JSON 格式
```

文本输出展示：

```
Key: ITEM1234
Title: Annotated Paper
Type: journalArticle
Creators: Jane Doe
Tags: genomics
Collections: Research
Attachments: 1
  - [pdf] ATTACHPDF (ATTACHPDF)
    path: D:\zotero\storage\ATTACHPDF\paper.pdf
Notes: 1
  - NOTE1234: A regular note
Annotations: 4
  - [note] color=#ffd400 page=2: Key finding about genome assembly | This is important for the discussion section
  - [note] color=#ff6666 page=3: Another highlighted passage | Need to verify this claim
  - [highlight] color=#5c9eff page=5: A pure highlight text
  - [ink] color=#000000 page=1:  | A hand-drawn sketch note
```

JSON 输出包含完整字段：`creators`、`tags`、`collections`、`attachments`（含 resolved path）、`notes`、**`annotations`**（含高亮文本、批注、颜色、页码、位置 JSON）。

### 关系查询 (`relate`)

```powershell
.\zot.exe relate ITEM1234              # 文本格式
.\zot.exe relate ITEM1234 --json       # JSON 格式
```

读取 Zotero `itemRelations` 表中的显式关系（如 "related to"、"containing"），返回谓词、方向（outgoing/incoming）、目标条目简要信息。

> 仅 **local** 和 **hybrid** 模式支持；`web` 模式暂未实现。  
> 在 `hybrid` 下如果本地关系读取失败，不会伪装回退成 `web relate`。

### 引文生成 (`cite`)

```powershell
.\zot.exe cite ITEM1234                        # APA 格式（默认）
.\zot.exe cite ITEM1234 --format chicago       # 指定引文格式
.\zot.exe cite ITEM1234 --format bib           # BibTeX
```

### 导出 (`export`)

```powershell
# 按关键词导出
.\zot.exe export "hybrid speciation" --format bibtex

# 按 item key 导出
.\zot.exe export --item-key SA6DHVIM --format ris

# 按收藏夹导出
.\zot.exe export --collection COLL1234 --format csljson --json
```

**支持格式**：`bib`、`bibtex`、`biblatex`、`csljson`、`ris`

**导出模式说明**：

- `bib` / `bibtex` / `biblatex` / `ris` 走 Web API。
- `csljson` 在 `local` / `hybrid` 下优先使用本地导出。
- `hybrid` 下的 `csljson` 只会在可预期的本地缺失或暂时不可用场景下回退到 Web；不会吞掉异常的本地错误。

### PDF 文本提取 (`extract-text`)

> 仅 **local** 和 **hybrid** 模式支持。使用 go-pdfium WASM 引擎提取 PDF 正文。  
> `hybrid` 下该命令仍依赖本地数据，不会回退到 Web。

```powershell
# 提取主附件全文（文本输出）
.\zot.exe extract-text ITEM1234

# 提取所有 PDF 附件的文本（JSON 输出，含缓存命中状态）
.\zot.exe extract-text ITEM1234 --json
```

JSON 输出示例：

```json
{
  "ok": true,
  "command": "extract-text",
  "data": {
    "item_key": "ITEM1234",
    "primary_attachment_key": "ATTACHPDF",
    "text": "...完整正文...",
    "attachments": [
      {
        "attachment_key": "ATTACHPDF",
        "title": "paper.pdf",
        "filename": "paper.pdf",
        "resolved_path": "D:\\zotero\\storage\\ATTACHPDF\\paper.pdf",
        "text": "...该附件的提取文本...",
        "total": 12345,
        "full_text_source": "pdfium",
        "full_text_cache_hit": false
      }
    ]
  },
  "meta": { "total": 12345 }
}
```

### 高亮与注释提取

在 `show` 命令中自动加载并展示 Zotero Reader 的 PDF 注解数据（local/hybrid 模式）。支持四种注解类型：

| 类型 | 说明 |
|------|------|
| `highlight` | 高亮标注（含原始文本） |
| `note` | 批注笔记（含高亮文本 + 用户评论） |
| `image` | 图片标注 |
| `ink` | 手绘/墨迹 |

每条注解包含：key、type、text（高亮原文）、comment（用户批注）、color（十六进制颜色）、page_label、page_index（从 position JSON 解析）、position（原始坐标 JSON）、is_external。

---

## 列表命令

```powershell
.\zot.exe collections          # 所有收藏夹
.\zot.exe collections-top      # 仅顶层收藏夹
.\zot.exe notes                # 所有子笔记
.\zot.exe tags                 # 所有标签
.\zot.exe searches             # 已保存的搜索
.\zot.exe trash                # 回收站中的条目
.\zot.exe publications         # My Publications
.\zot.exe deleted              # 已删除对象 key（自某版本以来）
.\zot.exe stats                # 库统计（总条目/收藏夹/搜索数）
.\zot.exe versions items --since 0   # 版本变更列表
```

以上均支持 `--json` 输出。

## 元数据命令

```powershell
.\zot.exe item-types                       # 可用文献类型列表
.\zot.exe item-fields                      # 可用字段列表
.\zot.exe creator-fields                   # 可用作者字段列表
.\zot.exe item-type-fields journalArticle   # 某类型的有效字段
.\zot.exe item-type-creator-types book     # 某类型的有效作者角色
.\zot.exe item-template journalArticle     # 新建条目的模板
.\zot.exe key-info YOUR_API_KEY            # API key 权限信息
.\zot.exe groups                           # 可访问的群组库
```

## 写操作

写操作受权限开关保护。默认策略：**允许创建/更新，禁止删除**。

### 条目 CRUD

```powershell
# 创建单个条目
.\zot.exe create-item --from-file item.json --if-unmodified-since-version 41

# 批量创建
.\zot.exe create-items --from-file items.json --if-unmodified-since-version 41 --json

# 更新
.\zot.exe update-item ABCD2345 --from-file patch.json --if-unmodified-since-version 42
.\zot.exe update-items --from-file patches.json --json

# 删除（需 ZOT_ALLOW_DELETE=1）
.\zot.exe delete-item ABCD2345 --if-unmodified-since-version 43
.\zot.exe delete-items --items KEY1,KEY2 --if-unmodified-since-version 43
```

### 标签操作

```powershell
.\zot.exe add-tag --items KEY1,KEY2 --tag "to-read"
.\zot.exe remove-tag --items KEY1,KEY2 --tag "obsolete"
```

### 收藏夹与搜索

```powershell
.\zot.exe create-collection --name "New Collection" --parent COLL_PARENT
.\zot.exe update-collection COLL1234 --name "Renamed"
.\zot.exe delete-collection COLL1234 --if-unmodified-since-version 10

.\zot.exe create-search --from-file search.json
.\zot.exe update-search SRCH1234 --from-file search.json
.\zot.exe delete-search SRCH1234 --if-unmodified-since-version 10
```

---

## 架构概览

```
zot (CLI)
├── cmd/zot/main.go          # 入口
├── internal/
│   ├── cli/                 # 命令解析与执行
│   │   ├── cli.go           # 命令注册与路由
│   │   ├── commands_read.go # find, show, relate, cite, extract-text
│   │   ├── commands_write.go# create/update/delete, tag ops
│   │   ├── commands_list.go # collections, notes, tags, stats...
│   │   ├── commands_config.go# config init/show/validate
│   │   └── commands_pdf.go  # PDF 文本提取
│   ├── backend/             # 数据访问层
│   │   ├── reader.go        # Reader 接口定义
│   │   ├── local.go         # LocalReader (SQLite)
│   │   ├── local_find.go    # 本地查询构建
│   │   ├── local_loaders.go # 数据加载器 (creators, tags, attachments, notes, annotations...)
│   │   ├── web.go           # WebReader (Zotero Web API)
│   │   ├── hybrid.go        # HybridReader (本地优先 + 回退)
│   │   └── pdfium.go        # PDF 文本提取引擎 (go-pdfium WASM)
│   ├── domain/types.go      # 领域模型 (Item, Annotation, Attachment, Note...)
│   ├── config/              # 配置加载与环境变量
│   └── zoteroapi/           # Zotero Web API 客户端
└── docs/                    # 设计文档
```

### 三层后端架构

| 层 | 模式 | 数据源 | 特性 |
|----|------|--------|------|
| **WebReader** | `web` | Zotero Cloud API | 完整 API 能力，需网络 |
| **LocalReader** | `local` | 本地 SQLite | 离线、快速、全文检索、PDF 提取、注解读取 |
| **HybridReader** | `hybrid` | Local → Web fallback | 兼顾速度与完整性 |

**LocalReader** 的核心能力：
- 从 `zotero.sqlite` 直接读取元数据、创建者、标签、收藏夹、附件、笔记
- 通过 `itemAnnotations` 表读取 PDF 高亮/批注/手绘（经附件子项链路）
- 通过 `itemRelations` 表读取显式条目关系
- PDF 全文提取（go-pdfium WASM 引擎，带缓存）
- Zotero 内置全文索引搜索 (`--fulltext`)
- 自动快照回退机制（当数据库被 Zotero 锁定时）

### 领域模型

```go
Item {
    Key, Version, ItemType, Title, Date,
    Volume, Issue, Pages, DOI, URL, Container,
    Creators [], Tags [], Collections [],
    Attachments []{Key, Title, Filename, ContentType,
                    LinkMode, ZoteroPath, ResolvedPath, Resolved},
    Notes []{Key, Preview},
    Annotations []{Key, Type, Text, Comment, Color,
                    PageLabel, PageIndex, Position, SortIndex, IsExternal},
    SearchScore, MatchedOn, FullTextPreview, SnippetAttachmentKey
}
```

---

## AI / Agent 使用建议

`zot` 天然适合作为 AI agent 的 Zotero 操作接口：

- **默认加 `--json`** — 结构化输出便于解析
- **检索文献**：`find "query" --json --full` 获取完整元数据
- **获取全文**：`extract-text ITEM_KEY --json` 提取 PDF 正文供 LLM 阅读
- **获取高亮批注**：`show ITEM_KEY --json` 的 `annotations` 字段包含用户的阅读标记
- **查关系**：`relate KEY --json` 发现文献间的显式关联
- **批量导出**：`export --collection COLL --format csljson --json`
- **安全前提**：执行前先跑 `config validate`；删除前确认 version precondition

更详细的 agent 集成指南：

- [AI Agent Guide](docs/AI_AGENT.md)
- [.codex/skills/zotero-cli/SKILL.md](.codex/skills/zotero-cli/SKILL.md)

---

## 开发

```powershell
# 测试
go test ./...                         # 全量测试
go test ./internal/cli/ -run TestLoad  # 运行特定测试
go test ./internal/cli/ -v -run TestShowLocal.*Annotation  # verbose 运行注解相关测试

# 构建
go build -o zot.exe .\cmd\zot
.\zot.exe version
```

### 项目依赖

| 依赖 | 用途 |
|------|------|
| `modernc.org/sqlite` | 纯 Go SQLite 驱动（无需 CGO） |
| `klippa-app/go-pdfium` | PDF 文本提取（WASM 引擎，跨平台） |

### 关键设计决策

- **纯 Go 工具链**：无 CGO 依赖，交叉编译友好
- **SQLite 快照回退**：当 Zotero 持有数据库锁时，自动复制快照读取
- **TDD 开发流程**：核心功能均有对应测试覆盖
- **权限门控**：写/删除操作通过环境变量开关控制，防止误操作

---

## 文档

| 文档 | 说明 |
|------|------|
| [AI Agent Guide](docs/AI_AGENT.md) | AI agent 集成详细指南 |
| [MVP 设计文档](docs/MVP.md) | 最小可行产品设计 |
| [Local 后端设计](docs/local-backend-design.md) | SQLite 读取层架构 |
| [PDF 处理研究](docs/pdf-processing-research.md) | PDF 文本提取技术选型 |
| [Rate Limit 优化](docs/rate-limit-optimization.md) | API 限流处理策略 |
| [Roadmap 0.0.3](docs/roadmap-0.0.3.md) | 版本路线图 |
| [Stability Pass](docs/stability-pass-0.0.2.md) | 稳定性改进记录 |
