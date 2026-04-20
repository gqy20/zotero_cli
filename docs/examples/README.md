# JSON 输出示例

本目录包含各命令 `--json` 模式的真实输出示例，供 AI Agent 理解返回结构。

所有命令的 JSON 输出统一包裹在 `jsonResponse` 结构中：

```json
{
  "ok": true,
  "command": "find",
  "data": { ... },
  "meta": { ... }
}
```

- `ok`: 操作是否成功
- `command`: 命令名
- `data`: 具体返回数据（结构因命令而异）
- `meta`: 可选元信息（总数、数据来源等）

## 示例索引

| 文件 | 命令 | 说明 |
|------|------|------|
| [find.md](find.md) | `zot find "query" --json` | 文献检索结果 |
| [show.md](show.md) | `zot show KEY --json` | 单条目详情（含标注/附件） |
| [stats.md](stats.md) | `zot stats --json` | 库统计 |
| [cite.md](cite.md) | `zot cite KEY --format bibtex --json` | 引文生成 |
| [export.md](export.md) | `zot export --item-key KEY --format csljson --json` | CSL-JSON 导出 |
| [relate.md](relate.md) | `zot relate KEY --json` | 条目关系 |
| [annotations.md](annotations.md) | `zot annotations KEY --json` | PDF 标注（双源） |
| [error.md](error.md) | 各类错误场景 | 错误响应格式 |

> 示例中的数据为虚构值，仅展示字段结构和类型。
