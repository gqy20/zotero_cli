# 技术架构

项目结构、分层设计、领域模型和关键决策。

---

## 目录结构

```
zotero_cli/
├── cmd/zot/
│   └── main.go                    # 入口：解析参数 → CLI.Run()
├── internal/
│   ├── backend/                   # 数据访问层
│   │   ├── reader.go              # Reader 接口 + HybridReader
│   │   ├── web.go                 # WebReader（Zotero Web API）
│   │   ├── local.go               # LocalReader（SQLite + storage/）
│   │   ├── local_loaders.go       # SQLite 查询实现
│   │   ├── local_fulltext.go      # FTS5 全文检索
│   │   ├── local_export.go        # 本地 CSL-JSON 导出
│   │   ├── pdf_extract_text.go    # PDF 文本提取（PyMuPDF / pdfium）
│   │   ├── pdf_read_annotations.go # PDF 标注读取/删除（PyMuPDF）
│   │   ├── pdf_write_annotation.go # PDF 标注写入（PyMuPDF）
│   │   ├── errors.go              # 后端错误类型
│   │   └── open_select.go         # zotero:// 协议调用
│   ├── cli/                       # 命令层
│   │   ├── cli.go                 # 主调度器 + 命令注册
│   │   ├── runtime.go             # 配置加载、后端创建
│   │   ├── types.go               # CLI 专用类型
│   │   ├── usage.go               # 帮助文本
│   │   ├── commands_find.go       # find 命令
│   │   ├── commands_show.go       # show 命令
│   │   ├── commands_*.go          # 其余命令（每命令一文件）
│   │   └── args_*.go              # 参数解析辅助
│   ├── config/                    # 配置管理
│   │   └── config.go              # .env 加载 / 环境变量 / 校验
│   ├── domain/                    # 领域模型
│   │   └── types.go               # 核心数据结构
│   └── zoteroapi/                 # Zotero API 客户端
│       ├── types.go               # API 类型定义
│       ├── client.go              # HTTP 客户端核心
│       ├── client_items.go        # 条目 CRUD
│       ├── client_collections.go  # 收藏夹 CRUD
│       ├── client_tags.go         # 标签操作
│       ├── client_searches.go     # 已保存搜索
│       ├── client_cite.go         # 引文生成
│       ├── client_export.go       # 导出
│       ├── client_full.go         # 完整条目查询
│       ├── client_map.go          # domain ↔ API 映射
│       └── errors.go              # API 错误处理
├── docs/                          # 文档
├── scripts/                       # 构建脚本
├── .github/workflows/             # CI/CD
├── go.mod / go.sum
└── README.md
```

---

## 三层架构

| 层 | 职责 | 关键文件 |
|----|------|----------|
| **CLI 层** (`internal/cli/`) | 参数解析、命令调度、输出格式化 | `cli.go`, `commands_*.go` |
| **Backend 层** (`internal/backend/`) | 数据访问抽象，三种模式统一接口 | `reader.go`, `web.go`, `local.go` |
| **Domain 层** (`internal/domain/` + `internal/zoteroapi/`) | 领域实体、API 客户端、配置 | `types.go`, `config.go`, `client_*.go` |

### Reader 接口

```go
type Reader interface {
    FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error)
    GetItem(ctx context.Context, key string) (domain.Item, error)
    GetRelated(ctx context.Context, key string) ([]domain.Relation, error)
    GetLibraryStats(ctx context.Context) (LibraryStats, error)
}
```

### 三种模式

| 模式 | 实现 | 数据源 | PDF 能力 |
|------|------|--------|----------|
| `web` | `WebReader` | Zotero Cloud API | 无 |
| `local` | `LocalReader` | SQLite + `storage/` 目录 | PyMuPDF |
| `hybrid` | `HybridReader` | 本地优先，Web 回退 | 同 local |

**HybridReader 策略**：

1. 先尝试 LocalReader
2. 仅当本地返回 `unsupportedFeatureError` 且 Web 能承接时回退
3. 本地独有能力（全文检索、PDF 标注、附件过滤）不回退
4. 返回结果携带 `read_source` 元数据标记实际来源

---

## LocalReader 核心能力

| 能力 | 实现方式 | 说明 |
|------|----------|------|
| 条目检索 | SQLite JOIN 查询 | 支持 title/creator/date/tag 过滤 |
| 全文检索 | FTS5 虚拟表 | `--fulltext` / `--fulltext-any` / `--snippet` |
| 附件感知 | `storage/` 目录扫描 | `--has-pdf` / `--attachment-type` / resolved path |
| 标注读取（DB） | `itemAnnotations` 表 JOIN | 含 dateAdded 时间戳 |
| 标注读取（PDF） | PyMuPDF `page.annots()` | 扫描 PDF 文件内嵌入标注 |
| 标注写入 | PyMuPDF `add_*_annot()` | highlight / underline / note 三种模式 |
| 标注删除 | PyMuPDF `page.delete_annot()` | 按 page/type/author 过滤 |
| 文本提取 | PyMuPDF → pdfium WASM 回退 | 带 venv 自动安装机制 |
| 关系查询 | `itemRelations` 表 | related / containing 等 |
| 导出 | 本地数据组装 | CSL-JSON 优先本地导出 |
| 快照回退 | SQLite 临时副本 | 数据库被锁时自动降级为只读副本 |

### 并行加载

Creators、Tags、Attachments 通过 goroutine + WaitGroup 并行加载，减少 SQLite I/O 等待。

---

## 领域模型

### Item

```go
type Item struct {
    Key                  string           `json:"key"`
    Version              int              `json:"version,omitempty"`
    ItemType             string           `json:"item_type"`
    Title                string           `json:"title"`
    Date                 string           `json:"date"`
    Creators             []Creator        `json:"creators"`
    Tags                 []string         `json:"tags,omitempty"`
    Collections          []Collection     `json:"collections,omitempty"`
    Attachments          []Attachment     `json:"attachments,omitempty"`
    Notes                []Note           `json:"notes,omitempty"`
    Annotations          []Annotation     `json:"annotations,omitempty"`
    // 检索元字段
    SearchScore          int              `json:"-"`
    MatchedOn            []string         `json:"matched_on,omitempty"`
    FullTextPreview      string           `json:"full_text_preview,omitempty"`
    SnippetAttachmentKey string           `json:"-"`
    MatchedChunk         *MatchedChunkInfo `json:"matched_chunk,omitempty"`
    // 出版物字段
    Container            string           `json:"container,omitempty"`
    Volume               string           `json:"volume,omitempty"`
    Issue                string           `json:"issue,omitempty"`
    Pages                string           `json:"pages,omitempty"`
    DOI                  string           `json:"doi,omitempty"`
    URL                  string           `json:"url,omitempty"`
}
```

### Annotation

```go
type Annotation struct {
    Key        string `json:"key"`
    Type       string `json:"type"`          // highlight | note | image | ink
    Text       string `json:"text,omitempty"`
    Comment    string `json:"comment,omitempty"`
    Color      string `json:"color,omitempty"` // "#ffd400"
    PageLabel  string `json:"page_label,omitempty"`
    PageIndex  int    `json:"page_index,omitempty"`
    Position   string `json:"position,omitempty"`
    SortIndex  string `json:"sort_index,omitempty"`
    IsExternal bool   `json:"is_external"`
    DateAdded  string `json:"date_added,omitempty"`
}
```

### Attachment

```go
type Attachment struct {
    Key          string `json:"key"`
    ItemType     string `json:"item_type"`
    ContentType  string `json:"content_type,omitempty"`
    LinkMode     string `json:"link_mode,omitempty"`
    Filename     string `json:"filename,omitempty"`
    ZoteroPath   string `json:"zotero_path,omitempty"`
    ResolvedPath string `json:"resolved_path,omitempty"`
    Resolved     bool   `json:"resolved,omitempty"`
}
```

### 其他核心类型

| 类型 | 用途 |
|------|------|
| `Creator` | 作者（Name + CreatorType） |
| `Collection` | 收藏夹（Key + Name） |
| `Note` | 子笔记 |
| `Relation` | 条目关系（Predicate + Direction + Target ItemRef） |
| `FindOptions` | 检索参数（query/filters/pagination） |
| `LibraryStats` | 库统计（items/collections/searches 计数） |

---

## 关键设计决策

### 1. 接口驱动，零 CGO

- `Reader` 接口使三种模式可互换，CLI 层不关心具体实现
- SQLite 使用纯 Go 驱动 `modernc.org/sqlite`，无 CGO 依赖，跨平台编译零障碍
- PDF 处理通过 Python 子进程（PyMuPDF）而非 Go 绑定，避免 CGO

### 2. Hybrid 回退策略

不是简单的"本地失败就走 Web"，而是**能力感知回退**：
- 本地不支持的功能（如 `--qmode`）→ 可回退
- 本地独有的功能（如 `--fulltext`）→ 不回退，保留错误
- 本地临时不可用（如数据库锁定）→ 视操作类型决定

### 3. Python 子进程模式

PDF 操作（文本提取、标注读写）通过 stdin 传递脚本、argv 传递路径、stdout 返回 JSON：

```
CLI → (Python script via stdin, PDF path via argv) → JSON stdout → Go 解析
```

Python 环境自动管理在 `{ZOT_DATA_DIR}/.zotero_cli/venv/`，优先使用 `uv` 包管理器。

### 4. 写操作安全模型

- 环境变量门控：`ZOT_ALLOW_WRITE=1` / `ZOT_ALLOW_DELETE=0`
- 版本号乐观锁：所有写操作要求 `--if-unmodified-since-version N`
- 删除操作默认禁止，需显式开启

### 5. zotero:// 协议集成

`open` 和 `select` 命令直接调用系统 URI 协议，无需加载数据库或配置：

| 命令 | 协议 | 说明 |
|------|------|------|
| `open` | `zotero://open-pdf/library/items/{attachmentKey}?page=N` | 在阅读器中打开 PDF |
| `select` | `zotero://select/library/items/{key}` | 在主界面选中条目 |

关键细节：`open-pdf` 协议要求传入**附件 itemKey**（非父条目 key），因为 Zotero 内部会检查 `isFileAttachment()`。

---

## 项目依赖

### 直接依赖

| 包 | 用途 |
|----|------|
| `modernc.org/sqlite` | 纯 Go SQLite 驱动 |
| `github.com/klippa-app/go-pdfium` | PDF 文本提取回退（WASM） |

### 标准库扩展

| 包 | 用途 |
|----|------|
| `golang.org/x/net` | HTTP 相关 |
| `github.com/mattn/go-isatty` | 终端检测 |
| `github.com/dustin/go-humanize` | 人性化数字格式化 |

### 外部工具（运行时）

| 工具 | 用途 | 安装方式 |
|------|------|----------|
| PyMuPDF | PDF 标注读写、文本提取首选 | `zot setup pdf-extract` 自动安装 |
| pdfium WASM | 文本提取回退 | Go 模块内置 |
| `uv` / `pip` | Python 包管理 | 自动检测，优先 uv |

---

## 错误体系

```go
// 后层错误
type unsupportedFeatureError struct { Backend, Feature, Hint string }
type itemNotFoundError struct { Object, Key string }
type dbLockedError struct { Path string }

// API 错误（zoteroapi 包）
type APIError struct { Status int; Message string }
// 401 → 认证失败, 403 → 权限不足, 412 → 版本冲突, 429 → 限流
```

每种错误都携带足够的上下文信息，使 CLI 输出可操作的提示（而非泛化的 "operation failed"）。
