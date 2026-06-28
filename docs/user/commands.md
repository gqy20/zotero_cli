# 命令参考

完整用法、选项说明、模式边界和输出示例。

> **模式说明**：支持 `web`、`local`、`hybrid`、`remote` 四种运行模式。`remote` 模式通过 HTTP 连接 `zot server`（端口 8021），reader 路径命令（find/show/stats/tags/notes/overview 等）经由服务器代理；`annotations` / `annotate` 也会经由服务器执行。其余普通写操作和 web-only 命令仍需额外配置 `ZOT_API_KEY` 后可用。详见 [架构文档](../architecture/overview.md)。

> AI Agent 使用规范见 [AI Agent 指南](./quickstart.md)，技术架构见 [架构文档](../architecture/overview.md)。
> 标注操作详细文档见 [annotations 示例](./examples/annotations.md)。

---

## 目录

- [检索 (`find`)](#检索-find)
- [JSON 输出总览](#json-输出总览)
- [关系 (`relate`)](#关系-relate)
- [标注读取 (`annotations`)](#标注读取-annotations)
- [标注写入 (`annotate`)](#标注写入-annotate)
- [创建条目 (`create-item`)](#创建条目-create-item)
- [文本提取 (`extract-text`)](#文本提取-extract-text)
- [图片提取 (`extract-figures`)](#图片提取-extract-figures)
- [其他命令索引](#其他命令索引)

---

## 检索 (`find`)

用法和示例见 [find 示例](./examples/find.md)。

时间字段提示：`--date-after` / `--date-before` 过滤 **加入 Zotero 的时间（`dateAdded`）**，不是发表日期。发表日期过滤请用 `--sort dateAdded` 配合 `--direction desc`，或先用 `zot find --all --json | jq` 后处理。最近入库列表建议显式使用 `find --all --sort dateAdded --direction desc`。

## JSON 输出总览

所有 `read` 路径命令（`find` / `show` / `abstract` / `relate` / `notes` / `tags` / `stats` / `overview` 等）支持 `--json`，统一信封为 `{ok, command, data, meta, code}`。

**默认走 lean 模式**（`LeanItem`，蛇形字段，约原始 1/5 体积，详见 [`internal/cli/lean.go`](../../internal/cli/lean.go)）。要原始 `domain.Item` 完整字段加 `--full`：

| 命令 | lean 默认 | full 切换 |
|------|----------|----------|
| `find` | ✅ | `--full` |
| `show` | ✅ | `--full` |
| `abstract` | ✅ | `--full` |
| `relate` | ❌（直接 `AggregatedRelations`） | — |
| `notes` / `tags` / `stats` / `overview` | 不涉及（结构本身小） | — |

完整字段表见：
- lean：`internal/cli/lean.go` 的 `LeanItem` 定义
- full：`internal/domain/types.go` 的 `Item` 定义
- 错误信封：`internal/cli/types.go` 的 `errorData` 定义

完整 JSON 示例：
- [find-output.md](https://github.com/gqy20/zotero_cli/blob/master/.claude/skills/zotero-cli/examples/find-output.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/.claude/skills/zotero-cli/examples/find-output.md)
- [show-output.md](https://github.com/gqy20/zotero_cli/blob/master/.claude/skills/zotero-cli/examples/show-output.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/.claude/skills/zotero-cli/examples/show-output.md)
- [error.md](./examples/error.md)

---

## 关系 (`relate`)

查询和管理条目之间的显式关联、笔记内嵌引用，支持关系网络可视化。

详细用法和示例见 [relate 示例](./examples/relate.md)。

### 用法

```
zot relate <item-key> [--json] [--aggregate] [--dot] [--predicate PRED]
         [--add TARGET] [--remove TARGET] [--dry-run] [--from-file PATH]
```

### 核心能力

| 功能 | 选项 | 说明 |
|------|------|------|
| 查询显式关系 | （默认） | 从 `itemRelations` 表读取 outgoing + incoming |
| 三层聚合 | `--aggregate` | 自身关系 + 子笔记关系 + 内嵌 citation |
| 可视化 | `--dot` | 输出 Graphviz DOT 格式关系网络图 |
| 写入 | `--add` / `--remove` | 添加/删除显式关系（需 `ZOT_ALLOW_WRITE=1`） |
| 批量操作 | `--from-file PATH` | JSON 文件驱动批量 add/remove |
| 预览 | `--dry-run` / `-n` | 不执行写入，仅展示将做的变更 |
| 过滤 | `--predicate PRED` | 按谓词类型筛选（如 `dc:relation`） |

---

## 标注读取 (`annotations`)

读取 PDF 标注（双源：PDF 文件层 + Zotero DB 层），支持过滤和删除。

在 `remote` 模式下，本命令通过 `zot server` 在服务端读取/清除标注；清除是否允许由服务端 `ZOT_ALLOW_DELETE` 控制。

### 用法

```
zot annotations <item-key> [--json] [--clear] [--page N] [--type TYPE] [--author AUTHOR]
```

### 选项

| 选项 | 说明 |
|------|------|
| `--json` | JSON 格式输出 |
| `--clear` | **删除**标注（双层：PDF + DB） |
| `--page N` | 仅显示/删除第 N 页 |
| `--type TYPE` | 按类型过滤/删除（highlight / note / image 等） |
| `--author AUTHOR` | 按作者过滤/删除（DB 层有效） |

### 输出

文本模式按源分组展示；JSON 模式返回结构化数据含 `pdf_annotations` 和 `db_annotations` 两个数组。

### `--clear` 行为

- 始终尝试**双层删除**
- PDF 层（PyMuPDF）：随时可用
- DB 层（SQLite）：需要 Zotero 关闭，否则输出 warning 不阻断

详细示例见 [annotations 示例](./examples/annotations.md)。

---

## 标注写入 (`annotate`)

向 PDF 文件写入高亮、下划线或便签笔记。

在 `remote` 模式下，本命令通过 `zot server` 在服务端写入 PDF 标注；是否允许写入由服务端 `ZOT_ALLOW_WRITE` 控制，不依赖客户端 `ZOT_API_KEY`。

### 用法

```
zot annotate <item-key> (--text TEXT | --page N (--rect x0,y0,x1,y2 | --point x,y)) \
  [--color COLOR] [--comment TEXT] [--type TYPE] [--clear] [--author AUTHOR] [--json]
```

### 三种标注模式

| 模式 | 触发条件 | 说明 |
|------|----------|------|
| Mode 1: 全文搜索 | `--text` （无 `--page`） | 所有页面匹配，每处创建标注 |
| **Mode 1.5: 单页搜索** | **`--page N --text`** | **仅指定页搜索（推荐）** |
| Mode 2: 精确坐标 | `--page N --rect ...` 或 `--point ...` | 矩形区域或坐标点 |

### 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--text TEXT` | 要搜索的文本 | — |
| `--page N` | 目标页码 | — |
| `--rect x0,y0,x1,y2` | 高亮矩形区域 (PDF 坐标) | — |
| `--point x,y` | 便签位置 (逗号分隔!) | — |
| `--color COLOR` | 颜色（名称或 #hex） | `yellow` |
| `--type TYPE` | highlight / underline / text | `highlight` |
| `--comment TEXT` | 便签内容（point 模式默认 "Note"） | — |
| `--clear` | 删除而非创建 | — |
| `--author AUTHOR` | 删除时按作者过滤 | — |
| `--json` | JSON 输出 | — |

### 最佳实践

1. **优先用 Mode 1.5**（`--page N --text "keyword"`）— 精准定位，避免全文误匹配
2. 详细说明放 `--comment`，`--text` 只用短唯一关键词
3. 先用 `extract-text` 确认该页实际文本再选关键词
4. 清理旧标注用 `--clear`，Zotero 关闭后执行可彻底清除双层

详细案例见 [annotations 示例](./examples/annotations.md)。

---

## 创建条目 (`create-item`)

通过 JSON 数据创建新条目（笔记、文献等）。支持 **hybrid write routing**：Zotero 未运行时自动走本地 SQLite 直写。

### 用法

```
zot create-item (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]
```

### Hybrid write routing

| 条件 | 路径 | 输出标识 |
|------|------|----------|
| mode = `local`/`hybrid` + Zotero **未运行** + itemType = `note` | local SQLite direct write (~50ms) | `"write_source": "local"` |
| 其他情况 | Web API POST (~2s) | 正常 API 响应 |

> 自动检测通过 `isZoteroRunning()` 检查进程状态，无需手动指定路径。

### 创建笔记示例

```bash
# 准备笔记 JSON
cat > note.json << 'EOF'
{
  "itemType": "note",
  "parentItem": "SXJ9FYTK",
  "note": "<h1>阅读总结</h1><p>这是我的笔记内容</p>"
}
EOF

# 创建（自动选择 local 或 web 路径）
zot create-item --from-file note.json --if-unmodified-since-version 59156 --json
```

JSON 字段说明：

| 字段 | 必填 | 说明 |
|------|------|------|
| `itemType` | 是 | `"note"` （当前仅笔记支持 local 写入） |
| `parentItem` | 是 (local) | 父条目 key，local 模式下用于关联 |
| `note` | 是 (local) | HTML 格式笔记内容 |
| `--if-unmodified-since-version N` | 是 | 库版本号（乐观锁，防止并发冲突） |

---

## 文本提取 (`extract-text`)

从 PDF 附件中提取全文内容。

### 用法

```
zot extract-text <item-key> [--json]
```

### 输出

| 模式 | 说明 |
|------|------|
| 文本模式 | 直接输出纯文本到 stdout |
| JSON 模式 | 返回结构化数据：`text`、`attachments[]`（含 `attachment_key`、`text`、`resolved_path` 等） |

### 依赖

需要 Python + PyMuPDF（通过 `findPythonCommandFunc` 自动检测）。

---

## 图片提取 (`extract-figures`)

从 PDF 附件中提取科学插图（Figure），基于 PyMuPDF `cluster_drawings()` 矢量聚类 + 位图锚点回退。

### 用法

```
zot extract-figures <item-key> [...] [--output-dir DIR] [--json] [--workers N]
```

### 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `<item-key>` [...] | 一个或多个条目 key（多篇自动并行） | — |
| `--output-dir`, `-o` | 输出目录 | `./figures` |
| `--json`, `-j` | JSON 格式输出 | 文本模式 |
| `--workers`, `-w` | 并行 worker 数 | CPU 核数（min 2, max 8） |

### 提取算法（v5b）

**双路径策略**：

| 路径 | 方法 | 适用场景 |
|------|------|----------|
| Path A（矢量） | `cluster_drawings()` 聚类矢量图形 | 矢量 PDF（LaTeX/Word 生成） |
| Path B（位图回退） | 大尺寸图片锚点，未被 Path A 覆盖的独立大图 | 扫描件 / 含嵌入图片的 PDF |

**过滤链**：

1. 面积/尺寸过滤（< 5000pt² 或 < 120×100px）
2. 锚点检测（大面积无锚点 = Abstract 等非 Figure 区域）
3. 文字密度检测（文字占比 > 35% = 纯文本区）
4. Caption 模式检测（"FIGURE N" 无锚点的 caption 块）
5. 全页扫描跳过（> 90% 页面面积）
6. 去重（重叠 > 50px）
7. **Caption 吸附**：自动搜索周围 "FIGURE N" 文本并扩展包含

### 文本输出示例

```
[AB123CD] 2 figure(s) in 1.6s
  p1_fig1.png  p.1 V1292x238  23.0kB anchors=0
  p1_fig2.png  p.1 V1292x1287  540.6kB anchors=1 +caption
```

列含义：`文件名 页码 来源(V=矢量/R=位图) 尺寸 大小 锚点数 [+caption]`

### JSON 输出字段

```json
{
  "item_key": "AB123CD",
  "pdf": "paper.pdf",
  "total_pages": -1,
  "figures": [
    {
      "id": 1, "file": "p1_fig1.png", "page": 1,
      "source": "cluster", "size_px": "1292x238",
      "size_pt": "465x85", "kb": 23.0, "anchors": 0,
      "has_caption": false, "text_ratio": 0.0, "pct_page": 8.5
    }
  ],
  "elapsed_sec": 1.6, "method": "cluster_drawings_v5b"
}
```

### 多篇并行

传入多个 item-key 时自动并行处理（WaitGroup + semaphore），单篇时直接执行避免 goroutine 开销。

### 依赖

- mode 必须为 `local` 或 `hybrid`
- 需要 Python + PyMuPDF
- 仅处理第一个 PDF 附件

---

## 其他命令索引

上方 8 个章节覆盖了最常用的命令。`zot` 还有以下命令，每个命令的完整 `-h` 是最权威的参考（参见 `internal/cli/commandRegistry` 单源）。

### Setup（初始化与配置）

| 命令 | 用途 |
|------|------|
| `init` | 交互式初始化 `~/.zot/.env`（mode 选择、API key、library id、PyMuPDF 一站式） |
| `config <sub>` | `path` / `show` / `validate` 三个子命令，校验当前凭据 |
| `index build` | 构建 FTS5 全文索引（`find --fulltext` / `show --snippet` 的前置） |
| `setup pdf-extract` | **旧命令**，已被 `zot init --pdf` 替代（保留兼容） |
| `version [--check]` | 显示当前版本；`--check` 查 GitHub 最新 release 并给出升级提示 |

### Read（只读）

| 命令 | 用途 | 关键标志 |
|------|------|----------|
| `find` | 主检索命令（详见上文） | `--all` / `--tag` / `--date-after` / `--snippet` |
| `show` | 单条目详情（详见上文） | `--full` / `--snippet` |
| `abstract` | 条目摘要，支持 **多 key 批量** | `--json` |
| `relate` | 关系查询（详见上文） | `--aggregate` / `--dot` |
| `export` | 导出引文（csljson/bibtex/biblatex/ris） | `--format` / `--collection` |
| `collections` | 列出全部收藏夹（含嵌套） | `--json` |
| `collections-top` | 仅顶级收藏夹 | `--json` |
| `notes` | 列出笔记（可 `--query` 过滤） | `--query` / `--json` |
| `tags` | 列出所有标签 | `--json` |
| `searches` | 列出已保存的搜索 | `--json` |
| `groups` | 列出可访问的群组 | `--json` |
| `publications` | 列出 "My Publications" | `--json` |
| `trash` | 列出回收站条目 | `--json` |
| `deleted` | 已删除 key（Zotero API 软删除） | `--json` |
| `changes <type> --since N` | 自版本 N 以来的变更（`items` / `items-top` / `collections` / `searches`） | `--since` / `--if-modified-since-version` |
| `stats` | 库内条目/收藏夹/搜索/附件/笔记计数 | `--json` |
| `overview` | 一站式库概览（**Agent 入口**）：stats + Top 收藏夹 + Top 标签 + 最近条目 + FTS 状态 | `--json` |
| `key-info [KEY]` | API key 权限与所属用户查询 | `--json` |
| `schema <sub>` | 元数据 schema 自省（`types` / `fields` / `creator-types` / `fields-for` / `creator-types-for` / `template`） | `--json` |
| `extract-text` | 从 PDF 提取全文（详见上文） | `--json` |
| `extract-figures` | 从 PDF 提取科学插图（详见上文） | `--output-dir` / `--workers` / `--max-per-page` / `--json` |
| `open` | 在系统默认 PDF 阅读器中打开条目附件 | — |
| `select` | 在 Zotero UI 中定位条目 | — |

### Annotate（标注操作）

| 命令 | 用途 |
|------|------|
| `annotations` | 读取/清除 PDF 标注（详见上文；`--clear` 双层删除） |
| `annotate` | 写入 PDF 标注（详见上文；Mode 1 / 1.5 / 2） |

### Write（写操作 — 需 `ZOT_ALLOW_WRITE=1`）

所有写命令接受 `--data <json>` / `--from-file <path>` 之一，并通过 `--if-unmodified-since-version N` 做乐观锁。

| 命令 | 用途 |
|------|------|
| `create-item` | 创建条目（笔记 / 普通条目）。hybrid 模式下 Zotero 关闭 + itemType=note 走 SQLite 直写 |
| `update-item` | 局部更新条目（patch 语义） |
| `add-tag` | 批量给条目打标签 |
| `remove-tag` | 批量移除标签 |
| `create-collection` | 创建收藏夹 |
| `update-collection` | 局部更新收藏夹 |
| `create-search` | 创建已保存搜索 |
| `update-search` | 局部更新已保存搜索 |

### Destructive（不可逆 — 需 `ZOT_ALLOW_WRITE=1` + `--if-unmodified-since-version`）

| 命令 | 用途 |
|------|------|
| `delete-item` | 删除条目 |
| `delete-collection` | 删除收藏夹 |
| `delete-search` | 删除已保存搜索 |

> 删除前必须 `get-show` 复述目标 key → 无歧义确认 → 有不确定先询问。`--json` 模式自动跳过确认提示。

---

## 输出 JSON 字段出处速查

为方便 agent 写解析逻辑，所有 JSON 字段定义都集中在三个 Go 结构体里（不要在文档里逐字段列）：

| 命令 | lean 模式 | full 模式 / 原始结构 |
|------|----------|-------------------|
| `find` / `show` / `abstract` | `internal/cli/lean.go:8` (`LeanItem`) | `internal/domain/types.go:3` (`Item`) |
| `relate --aggregate` | — | `internal/domain/types.go:97` (`AggregatedRelations`) |
| 错误信封 | `internal/cli/types.go:35` (`errorData`) | — |

任何字段名疑问，去看 struct tag，不要凭记忆。


---
