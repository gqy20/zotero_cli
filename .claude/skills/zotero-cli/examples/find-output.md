# `zot find --json` 输出示例

> **默认走 lean 模式**（蛇形字段，~80% 小）。要原始 `domain.Item` 字段加 `--full`。详细字段表见 [show-output.md](./show-output.md#full-模式字段)。

## 基础搜索（lean 默认）

```bash
zot find "CRISPR gene editing" --limit 2 --json
```

```json
{
  "ok": true,
  "command": "find",
  "data": [
    {
      "key": "ABCD1234",
      "item_type": "journalArticle",
      "title": "CRISPR-Cas9 gene editing: advances and challenges",
      "date": "2024-03",
      "creators": "Zhang Feng, Wang Mei, Smith John (ed.)",
      "container": "Nature Reviews Genetics",
      "volume": "25",
      "issue": "3",
      "pages": "1-20",
      "doi": "10.1038/nrg.2024.001",
      "url": "https://doi.org/10.1038/nrg.2024.001",
      "tags": ["基因编辑", "CRISPR", "综述"],
      "collections": ["Genetics", "Reviews"],
      "date_added": "2024-03-15T00:00:00Z"
    },
    {
      "key": "EFGH5678",
      "item_type": "journalArticle",
      "title": "Base editing with CRISPR: precision without double-strand breaks",
      "date": "2024-01",
      "creators": "Liu David",
      "container": "Cell",
      "doi": "10.1016/j.cell.2024.001",
      "tags": ["基因编辑", "碱基编辑"]
    }
  ],
  "meta": {
    "read_source": "web",
    "total": 47,
    "start": 0,
    "limit": 2
  }
}
```

## 纯过滤搜索（无查询词）

```bash
zot find --tag 基因编辑 --date-after 2024-01 --limit 1 --json
```

只要带了至少一个 filter（`--tag` / `--collection` / `--date-after` 等），**不需要 `--all` 也不需要查询词**，命令自动按过滤条件搜索。

```json
{
  "ok": true,
  "command": "find",
  "data": [
    {
      "key": "ABCD1234",
      "item_type": "journalArticle",
      "title": "CRISPR-Cas9 gene editing: advances and challenges",
      "date": "2024-03",
      "creators": "Zhang Feng",
      "tags": ["基因编辑", "CRISPR"]
    }
  ],
  "meta": {
    "read_source": "hybrid",
    "total": 15
  }
}
```

## FTS 全文检索 + Snippet

```bash
zot find "hybrid speciation" --snippet --limit 1 --json
```

> `--snippet` / `--fulltext` 仅 local/hybrid 模式 + 已有 FTS5 索引时生效。`--snippet` 是布尔开关；未指定 `--limit` 时回退 50 条。

```json
{
  "ok": true,
  "command": "find",
  "data": [
    {
      "key": "IJKL9012",
      "item_type": "journalArticle",
      "title": "Hybrid speciation in plants: genomic perspectives",
      "creators": "Lexer C, Rieseberg L",
      "matched_on": ["title", "fulltext"]
    }
  ],
  "meta": {
    "read_source": "hybrid",
    "total": 8,
    "fulltext_enabled": true
  }
}
```

> Snippet 文本只在使用 `--full` 时才会注入到每个 item 上（作为 `full_text_preview` 字段）；lean 模式只标记 `matched_on`。

## 关键字段说明（lean 模式）

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | string | Zotero 条目唯一标识 |
| `item_type` | string | 文献类型（`journalArticle` / `book` / `note` 等） |
| `title` | string | 条目标题 |
| `creators` | string | 作者摘要（`"Zhang Feng, Wang Mei"`） |
| `container` | string | 期刊/书名/会议名 |
| `tags` | string[] | 标签列表 |
| `collections` | string[] | 所在收藏夹名（不是 key） |
| `date_added` | string | 加入 Zotero 的时间（ISO 8601） |
| `matched_on` | string[] | 匹配来源（`title` / `fulltext` / `tag` 等） |
| `abstract` | string | 仅 `abstract` 命令或带 `--include-fields abstract` 时出现 |

> `version` / `creators[]`（结构化数组）/ `attachments` / `notes` / `annotations` / `journal_rank` 等是 **full 模式**才有的字段，加 `--full` 切换。

## meta 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `read_source` | string | `"web"` / `"local"` / `"hybrid"` / `"remote"` / `"snapshot"` — 实际数据源 |
| `total` | int | 匹配总数（不是当前返回的条数） |
| `start` | int | 分页起点 |
| `limit` | int | 本次返回条数上限 |
| `fulltext_enabled` | bool | FTS5 是否生效 |
