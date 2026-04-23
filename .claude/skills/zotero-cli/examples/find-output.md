# `zot find --json` 输出示例

## 基础搜索

```bash
zot find "CRISPR gene editing" --limit 2 --json
```

```json
{
  "ok": true,
  "command": "find",
  "data": {
    "items": [
      {
        "key": "ABCD1234",
        "version": 1234,
        "itemType": "journalArticle",
        "title": "CRISPR-Cas9 gene editing: advances and challenges",
        "creators": [
          {"creatorType": "author", "lastName": "Zhang", "firstName": "Feng"}
        ],
        "date": "2024-03",
        "publicationTitle": "Nature Reviews Genetics",
        "volume": "25",
        "pages": "1-20",
        "doi": "10.1038/nrg.2024.001",
        "url": "https://doi.org/10.1038/nrg.2024.001",
        "abstractNote": "CRISPR-Cas9 has revolutionized genome editing...",
        "tags": [{"tag": "基因编辑"}, {"tag": "CRISPR"}],
        "collections": ["COLLECTION_KEY"],
        "journal_rank": {"IF": 53.2, "分区": "Q1", "JCI": 12.8}
      },
      {
        "key": "EFGH5678",
        "version": 1235,
        "itemType": "journalArticle",
        "title": "Base editing with CRISPR: precision without double-strand breaks",
        "creators": [
          {"creatorType": "author", "lastName": "Liu", "firstName": "David"}
        ],
        "date": "2024-01",
        "publicationTitle": "Cell",
        "doi": "10.1016/j.cell.2024.001",
        "tags": [{"tag": "基因编辑"}, {"tag": "碱基编辑"}],
        "journal_rank": {"IF": 64.5, "分区": "Q1", "JCI": 15.2}
      }
    ],
    "total": 47,
    "start": 0,
    "limit": 2,
    "query": "CRISPR gene editing"
  }
}
```

## 纯过滤搜索（无查询词）

```bash
zot find --tag 基因编辑 --date-after 2024-01 --limit 1 --json
```

```json
{
  "ok": true,
  "command": "find",
  "data": {
    "items": [
      {
        "key": "ABCD1234",
        "version": 1234,
        "itemType": "journalArticle",
        "title": "CRISPR-Cas9 gene editing: advances and challenges",
        "tags": [{"tag": "基因编辑"}, {"tag": "CRISPR"}],
        "date": "2024-03"
      }
    ],
    "total": 15,
    "filters": {"tags": ["基因编辑"], "dateAfter": "2024-01-01"}
  }
}
```

## FTS 全文检索 + Snippet

```bash
zot find "hybrid speciation" --snippet --limit 1 --json
```

```json
{
  "ok": true,
  "command": "find",
  "data": {
    "items": [
      {
        "key": "IJKL9012",
        "title": "Hybrid speciation in plants: genomic perspectives",
        "snippet": "...evidence for <match>hybrid speciation</match> has accumulated from genomic studies...<match>hybrid</match> zones show elevated recombination rates..."
      }
    ],
    "total": 8,
    "fulltextEnabled": true
  }
}
```

### 关键字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | string | Zotero 条目唯一标识 |
| `version` | int | 乐观并发版本号 |
| `itemType` | string | 文献类型（journalArticle/book 等） |
| `creators` | array | 作者列表，含 creatorType/lastName/firstName |
| `tags` | array | 标签列表 |
| `journal_rank` | object/null | 期刊等级（需 EasyScholar 插件），含 IF/分区/JCI |
| `snippet` | string | FTS 匹配片段（仅 `--snippet` 时出现） |
| `total` | int | 符合条件的总条目数 |
