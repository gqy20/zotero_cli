# `zot show ITEMKEY --json` 输出示例

## 完整条目详情

```bash
zot show ABCD1234 --json
```

```json
{
  "ok": true,
  "command": "show",
  "data": {
    "key": "ABCD1234",
    "version": 1234,
    "itemType": "journalArticle",
    "title": "CRISPR-Cas9 gene editing: advances and challenges",
    "creators": [
      {"creatorType": "author", "lastName": "Zhang", "firstName": "Feng"},
      {"creatorType": "author", "lastName": "Wang", "firstName": "Mei"},
      {"creatorType": "editor", "lastName": "Smith", "firstName": "John"}
    ],
    "date": "2024-03-15",
    "publicationTitle": "Nature Reviews Genetics",
    "volume": "25",
    "issue": "3",
    "pages": "1-20",
    "doi": "10.1038/nrg.2024.001",
    "issn": "1471-0056",
    "url": "https://doi.org/10.1038/nrg.2024.001",
    "abstractNote": "CRISPR-Cas9 has revolutionized genome editing by enabling precise, targeted modifications to DNA sequences...",
    "language": "en",
    "libraryCatalog": "PubMed",
    "accessDate": "2024-04-20",
    "tags": [
      {"tag": "基因编辑"},
      {"tag": "CRISPR"},
      {"tag": "综述"}
    ],
    "collections": ["COLLECTION_1"],
    "relations": {
      "dc:relation": ["EFGH5678", "IJKL9012"]
    },
    "journal_rank": {
      "IF": 53.2,
      "分区": "Q1",
      "JCI": 12.8,
      "ISSN": "1471-0056",
      "期刊名": "Nature Reviews Genetics"
    },
    "attachments": [
      {
        "key": "ATTACH_KEY1",
        "title": "Zhang et al_ CRISPR-Cas9 gene editing.pdf",
        "path": "/path/to/Zotero/storage/ABCD/file.pdf",
        "contentType": "application/pdf",
        "sizeBytes": 2456789
      }
    ],
    "notes": [
      {
        "key": "NOTE_KEY1",
        "note": "<p>重要笔记：本文综述了 CRISPR 在基因治疗中的应用前景。</p>",
        "type": "note"
      }
    ],
    "annotations": [
      {
        "key": "ANN_KEY1",
        "type": "highlight",
        "text": "CRISPR-Cas9 has revolutionized genome editing",
        "comment": "核心论点",
        "color": "#ffff00",
        "pageLabel": "3",
        "author": "User"
      },
      {
        "key": "ANN_KEY2",
        "type": "note",
        "text": "",
        "comment": "需要进一步验证的数据",
        "color": "#ff6666",
        "pageLabel": "12",
        "position": {"pageIndex": 11}
      }
    ]
  }
}
```

### 关键字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | string | 条目唯一标识 |
| `version` | int | 并发版本号（写操作时用于 `--if-unmodified-since-version`） |
| `relations` | object | 显式关联条目（`dc:relation` 等谓词 → key 数组） |
| `journal_rank` | object/null | 期刊等级信息（需 EasyScholar 插件） |
| `attachments` | array | 附件列表（PDF 文件路径、大小等） |
| `notes` | array | 子笔记列表（HTML 格式内容） |
| `annotations` | array | PDF 标注列表（高亮/笔记类型、页码、颜色等） |

## relate --aggregate 输出

```bash
zot relate ABCD1234 --aggregate --json
```

```json
{
  "ok": true,
  "command": "relate",
  "data": {
    "self": {
      "key": "ABCD1234",
      "outgoing": [
        {"target": "EFGH5678", "predicate": "dc:relation"}
      ],
      "incoming": [
        {"source": "IJKL9012", "predicate": "dc:relation"}
      ]
    },
    "notes": [
      {
        "noteKey": "NOTE_KEY1",
        "relations": [
          {"target": "MNOP3456", "predicate": "dc:relation"}
        ]
      }
    ],
    "citations": [
      {
        "source": "NOTE_KEY1",
        "citedKeys": ["QRST6789", "UVWX0123"]
      }
    ]
  }
}
```
