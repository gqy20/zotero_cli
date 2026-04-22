# show — 条目详情

## 命令

```bash
zot show ABC123DE --json
```

## 输出

```json
{
  "ok": true,
  "command": "show",
  "data": {
    "key": "ABC123DE",
    "version": 1234,
    "item_type": "journalArticle",
    "title": "Homoploid hybrid speciation in action",
    "date": "2024-03-15",
    "creators": [
      { "name": "Smith, John", "creator_type": "author" },
      { "name": "Wang, Li", "creator_type": "author" },
      { "name": "Zhang, Wei", "creator_type": "editor" }
    ],
    "tags": ["speciation", "hybrid", "evolution"],
    "collections": [
      { "key": "COLL1", "name": "Active Research" },
      { "key": "COLL2", "name": "To Read" }
    ],
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
        "link_mode": "imported_file",
        "filename": "Smith2024_hybrid_speciation.pdf",
        "zotero_path": "storage:abc123/Smith2024.pdf",
        "resolved_path": "C:\\Users\\user\\Zotero\\storage\\abc123\\Smith2024.pdf",
        "resolved": true
      }
    ],
    "notes": [
      {
        "key": "NOTE1",
        "note": "<p>重点看 Figure 2 的实验设计</p>"
      }
    ],
    "annotations": [
      {
        "key": "ANNO1",
        "type": "highlight",
        "text": "homoploid hybrid speciation requires the coupling of assortative mating and ecological selection",
        "comment": "核心定义，值得引用",
        "color": "#ffd400",
        "page_label": "2",
        "page_index": 1,
        "position": "{\"pageIndex\":1,\"rects\":[[120,200,500,220]]}",
        "sort_index": "00001",
        "is_external": false,
        "date_added": "2024-04-10T14:30:00Z"
      },
      {
        "key": "ANNO2",
        "type": "note",
        "text": "",
        "comment": "这个方法可以复用到我们的实验中",
        "color": "#ff6666",
        "page_label": "5",
        "page_index": 4,
        "position": "{\"pageIndex\":4,\"rects\":[[300,400,320,420]]}",
        "sort_index": "00002",
        "is_external": false,
        "date_added": "2024-04-10T15:22:00Z"
      },
      {
        "key": "ANNO3",
        "type": "underline",
        "text": "reproductive isolation",
        "comment": "",
        "color": "#5c9eff",
        "page_label": "3",
        "page_index": 2,
        "position": "",
        "sort_index": "00003",
        "is_external": false,
        "date_added": "2024-04-11T09:15:00Z"
      }
    ]
  },
  "meta": {
    "total": 1,
    "read_source": "local"
  }
}
```

## 关键字段详解

### Annotation（标注）

| 字段 | 说明 |
|------|------|
| `type` | `highlight`（高亮）/ `note`（笔记）/ `underline`（下划线）/ `image` / `ink` |
| `text` | 被标注的原文（highlight/underline 有值，note 为空） |
| `comment` | 用户手写的批注文字 |
| `color` | 十六进制颜色码 |
| `page_label` | 用户看到的页码（如 "2"、"iv"） |
| `page_index` | 从 0 开始的页码索引 |
| `position` | 标注在页面中的位置坐标（JSON 字符串） |
| `date_added` | 标注创建时间（来自 items 表 dateAdded） |

### Attachment（附件）

| 字段 | 说明 |
|------|------|
| `link_mode` | `imported_file`（已导入）/ `linked_file`（链接）/ `embedded_image` |
| `resolved` | `true` 表示 `resolved_path` 可用 |
