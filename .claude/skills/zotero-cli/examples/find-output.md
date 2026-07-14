# `zot item find --json` 输出示例

## 基本检索

```powershell
zot item find "CRISPR gene editing" --limit 2 --json
```

```json
{
  "ok": true,
  "command": "item find",
  "data": [
    {
      "key": "ABCD1234",
      "item_type": "journalArticle",
      "title": "CRISPR-Cas systems for genome editing",
      "creators_summary": "Doe et al.",
      "date": "2024",
      "date_added": "2026-07-10T08:30:00Z",
      "container": "Genome Biology",
      "doi": "10.1000/example"
    }
  ],
  "meta": {
    "total": 1,
    "limit": 2,
    "offset": 0,
    "read_source": "live"
  }
}
```

即使使用正式快捷入口 `zot find ...`，JSON `command` 仍为 `item find`。

## Item type alias

```powershell
zot item find "CRISPR" --type article --json
```

输入 `article` 会在 application service 前转换为 `journalArticle`；输出始终使用官方值：

```json
{
  "ok": true,
  "command": "item find",
  "data": [{"key":"ABCD1234","item_type":"journalArticle","title":"Example"}],
  "meta": {"total":1}
}
```

## 全文片段

```powershell
zot item find '"hybrid speciation"' --in fulltext --snippet --limit 1 --json
```

```json
{
  "ok": true,
  "command": "item find",
  "data": [
    {
      "key": "EFGH5678",
      "item_type": "journalArticle",
      "title": "Genomic evidence for hybrid speciation",
      "matched_on": "fulltext",
      "full_text_preview": "...evidence supporting hybrid speciation..."
    }
  ],
  "meta": {
    "total": 1,
    "fulltext": true,
    "read_source": "live"
  }
}
```

## 关键字段

| 字段 | 含义 |
|---|---|
| `key` | Zotero item key |
| `item_type` | Zotero 官方类型，如 `journalArticle` |
| `date_added` | 入库时间，不是发表时间 |
| `matched_on` | metadata/fulltext 等命中来源 |
| `full_text_preview` | `--snippet` 返回的正文片段 |
| `meta.read_source` | live/snapshot/web 等实际来源 |

需要完整附件、笔记、标注等结构时使用 `--full`；只需要整篇 PDF 时升级到 `pdf text`。
