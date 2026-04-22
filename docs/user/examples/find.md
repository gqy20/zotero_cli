# find — 文献检索

## 命令

```bash
zot find "hybrid speciation" --json
```

## 输出

```json
{
  "ok": true,
  "command": "find",
  "data": [
    {
      "key": "ABC123DE",
      "version": 1234,
      "item_type": "journalArticle",
      "title": "Homoploid hybrid speciation in action",
      "date": "2024-03",
      "creators": [
        { "name": "Smith, John", "creator_type": "author" },
        { "name": "Wang, Li", "creator_type": "author" }
      ],
      "tags": ["speciation", "hybrid", "evolution"],
      "collections": [{ "key": "COLL1", "name": "My Papers" }],
      "container": "Nature Ecology & Evolution",
      "volume": "8",
      "issue": "3",
      "pages": "456-467",
      "doi": "10.1038/s41559-024-01234-x",
      "url": "https://doi.org/10.1038/s41559-024-01234-x",
      "attachments": [
        {
          "key": "ATTACH1",
          "item_type": "attachment",
          "title": "Smith2024_hybrid_speciation.pdf",
          "content_type": "application/pdf",
          "filename": "Smith2024_hybrid_speciation.pdf",
          "resolved_path": "C:\\Users\\user\\Zotero\\storage\\abc123\\Smith2024.pdf",
          "resolved": true
        }
      ],
      "matched_on": ["title", "creators"]
    },
    {
      "key": "XYZ789FG",
      "version": 1200,
      "item_type": "journalArticle",
      "title": "Polyploid speciation: a review of mechanisms",
      "date": "2023-11",
      "creators": [
        { "name": "Chen, Mei", "creator_type": "author" }
      ],
      "tags": ["speciation", "review", "polyploid"],
      "collections": [],
      "container": "Trends in Plant Science",
      "volume": "28",
      "issue": "11",
      "pages": "1123-1135",
      "doi": "10.1016/j.tplants.2023.09.001",
      "attachments": [],
      "matched_on": ["title"]
    }
  ],
  "meta": {
    "total": 2,
    "read_source": "local"
  }
}
```

## 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | string | Zotero 条目唯一标识 |
| `item_type` | string | 文献类型（journalArticle / book / preprint 等） |
| `matched_on` | []string | 命中原因（title / creators / tags / fulltext） |
| `attachments[].resolved` | bool | 附件路径是否可解析 |
| `meta.read_source` | string | 数据来源（local / web / hybrid） |

## 带过滤的示例

```bash
# 全文检索 + 片段预览（local/hybrid）
zot find "CRISPR gene editing" --fulltext --snippet --json

# 按日期和标签筛选
zot find "evolution" --date-after 2023 --tag "review" --json

# 返回完整字段
zot find "speciation" --full --json
```
