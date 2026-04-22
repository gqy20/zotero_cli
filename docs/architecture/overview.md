# 架构概览

项目结构、分层设计、三种运行模式和关键接口。

> 后端实现细节见 [后端设计](./backend.md)，领域模型见 [领域模型](./domain-model.md)，设计决策见 [设计决策](./decisions.md)。

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
│   ├── pdf_read_annotations.go # PDF 标注读取/删除（PyMuPDF）
│   ├── pdf_write_annotation.go # PDF 标注写入（PyMuPDF）
│   │   ├── errors.go              # 后端错误类型
│   │   └── open_select.go         # zotero:// 协议调用
│   ├── cli/                       # 命令层
│   │   ├── cli.go                 # 主调度器 + 命令注册
│   │   ├── runtime.go             # 配置加载、后端创建
│   │   ├── types.go               # CLI 专用类型
│   │   ├── usage.go               # 帮助文本
│   │   └── commands_*.go          # 其余命令（每命令一文件）
│   ├── config/                    # 配置管理
│   │   └── config.go              # .env 加载 / 环境变量 / 校验
│   ├── domain/                    # 领域模型
│   │   └── types.go               # 核心数据结构
│   └── zoteroapi/               # Zotero API 客户端
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

---

## 三种模式

| 模式 | 实现 | 数据源 | PDF 能力 |
|------|------|--------|----------|
| `web` | `WebReader` | Zotero Cloud API | 无 |
| `local` | `LocalReader` | SQLite + `storage/` | PyMuPDF |
| `hybrid` | `HybridReader` | 本地优先，Web 回退 | 同 local |

### HybridReader 策略

1. 先尝试 LocalReader
2. 仅当本地返回 `unsupportedFeatureError` 且 Web 能承接时回退
3. 本地独有能力（全文检索、PDF 标注、附件过滤）不回退
4. 返回结果携带 `read_source` 元数据标记实际来源

详见 [设计决策 - Hybrid 回退策略](./decisions.md)。

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

详见 [后端设计 - 当前文件布局](./backend.md#当前-backend-file-布局)。
