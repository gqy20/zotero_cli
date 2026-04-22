# 命令参考

完整用法、选项说明、模式边界和输出示例。

> AI Agent 使用规范见 [AI Agent 指南](./quickstart.md)，技术架构见 [架构文档](../architecture/overview.md)。
> 标注操作详细文档见 [annotations 示例](./examples/annotations.md)。

---

## 目录

- [检索 (`find`)](#检索-find)
- [查看 (`show`)](#查看-show)
- [标注读取 (`annotations`)](#标注读取-annotations)
- [标注写入 (`annotate`)](#标注写入-annotate)
- [文本提取 (`extract-text`)](#文本提取-extract-text)
- [图片提取 (`extract-figures`)](#图片提取-extract-figures)
- [打开/选中 (`open` / `select`)](#打开选中-open--select)
- [导出 (`export`)](#导出-export)
- [引用 (`cite`)](#引用-cite)
- [其他命令](#其他命令)

---

## 检索 (`find`)

用法和示例见 [find 示例](./examples/find.md)。

---

## 标注读取 (`annotations`)

读取 PDF 标注（双源：PDF 文件层 + Zotero DB 层），支持过滤和删除。

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

通过 JSON 数据创建新条目（笔记、文献等）。支持 **hybrid 写入**：Zotero 未运行时自动走本地 SQLite 直写。

### 用法

```
zot create-item (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]
```

### Hybrid 写入行为

| 条件 | 路径 | 输出标识 |
|------|------|----------|
| mode = `local`/`hybrid` + Zotero **未运行** + itemType = `note` | local SQLite 直写（~50ms） | `"write_source": "local"` |
| 其他情况 | Web API POST（~2s） | 正常 API 响应 |

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
