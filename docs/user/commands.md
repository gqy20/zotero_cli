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
