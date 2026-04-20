# relate — 条目关系

## 命令

```bash
zot relate ABC123DE --json
```

## 输出

```json
{
  "ok": true,
  "command": "relate",
  "data": [
    {
      "predicate": "related to",
      "direction": "outgoing",
      "target": {
        "key": "XYZ789FG",
        "item_type": "journalArticle",
        "title": "Polyploid speciation: a review of mechanisms",
        "creators": [
          { "name": "Chen, Mei", "creator_type": "author" }
        ],
        "date": "2023-11",
        "tags": ["speciation", "review"]
      }
    },
    {
      "predicate": "related to",
      "direction": "incoming",
      "target": {
        "key": "LMN456HI",
        "item_type": "book",
        "title": "Speciation: An Introduction",
        "creators": [
          { "name": "Jerry A. Coyne", "creator_type": "author" },
          { "name": "H. Allen Orr", "creator_type": "author" }
        ],
        "date": "2004",
        "tags": []
      }
    }
  ],
  "meta": {
    "total": 2,
    "read_source": "local"
  }
}
```

## 字段说明

| 字段 | 说明 |
|------|------|
| `predicate` | 关系类型：`related to`（相关）/ `containing`（包含）等 |
| `direction` | `outgoing`（本条目 → 目标）/ `incoming`（目标 → 本条目） |
| `target` | 关联条目的摘要信息（不含 annotations/attachments 等详细字段） |

> 仅 **local** 和 **hybrid** 模式支持。数据来源是 SQLite 的 `itemRelations` 表。
