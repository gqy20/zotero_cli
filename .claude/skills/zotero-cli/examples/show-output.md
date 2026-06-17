# `zot show ITEMKEY --json` 输出示例

> **默认走 lean 模式**（`LeanItem`，字段与 find 一致）。要 `domain.Item` 完整字段加 `--full`。**不接受多个 key**，批量看摘要用 [`zot abstract KEY1 KEY2 ...`](./abstract-output.md)。

## lean 模式（默认）

```bash
zot show ABCD1234 --json
```

```json
{
  "ok": true,
  "command": "show",
  "data": {
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
    "collections": ["Genetics"],
    "date_added": "2024-03-15T00:00:00Z"
  },
  "meta": {
    "read_source": "web",
    "total": 1
  }
}
```

lean 模式下不包含 `version` / `attachments` / `notes` / `annotations` / `journal_rank` / `abstract` / `matched_on` / `full_text_preview` —— 需要时加 `--full`。

## full 模式

```bash
zot show ABCD1234 --json --full
```

`data` 字段是完整的 `domain.Item`（`internal/domain/types.go:3`），主要字段：

```json
{
  "ok": true,
  "command": "show",
  "data": {
    "key": "ABCD1234",
    "version": 1234,
    "item_type": "journalArticle",
    "title": "CRISPR-Cas9 gene editing: advances and challenges",
    "date": "2024-03",
    "creators": [
      {"name": "Zhang Feng", "creator_type": "author"},
      {"name": "Wang Mei", "creator_type": "author"},
      {"name": "Smith John", "creator_type": "editor"}
    ],
    "container": "Nature Reviews Genetics",
    "volume": "25",
    "issue": "3",
    "pages": "1-20",
    "doi": "10.1038/nrg.2024.001",
    "url": "https://doi.org/10.1038/nrg.2024.001",
    "abstract": "CRISPR-Cas9 has revolutionized genome editing...",
    "date_added": "2024-03-15T00:00:00Z",
    "tags": ["基因编辑", "CRISPR", "综述"],
    "collections": [
      {"key": "ABC12345", "name": "Genetics"}
    ],
    "attachments": [
      {
        "key": "ATTACH_KEY1",
        "item_type": "attachment",
        "title": "Zhang et al_ CRISPR-Cas9 gene editing.pdf",
        "content_type": "application/pdf",
        "link_mode": "imported_file",
        "filename": "crispr.pdf",
        "zotero_path": "storage:ABCD1234/crispr.pdf",
        "resolved_path": "/home/user/Zotero/storage/ABCD1234/crispr.pdf",
        "resolved": true
      }
    ],
    "notes": [
      {
        "key": "NOTE_KEY1",
        "parent_item_key": "ABCD1234",
        "content": "<p>重要笔记：本文综述了 CRISPR 在基因治疗中的应用前景。</p>",
        "preview": "重要笔记：本文综述了 CRISPR 在基因治疗中的应用前景。"
      }
    ],
    "annotations": [
      {
        "key": "ANN_KEY1",
        "type": "highlight",
        "text": "CRISPR-Cas9 has revolutionized genome editing",
        "comment": "核心论点",
        "color": "#ffd400",
        "page_label": "3",
        "page_index": 2,
        "is_external": false
      },
      {
        "key": "ANN_KEY2",
        "type": "note",
        "comment": "需要进一步验证的数据",
        "color": "#ff6666",
        "page_label": "12",
        "page_index": 11,
        "is_external": false
      }
    ],
    "journal_rank": {
      "matched_name": "Nature Reviews Genetics",
      "ranks": {
        "sciif": "53.7",
        "sci":   "Q1",
        "jci":   "3.2",
        "esi":   "HCP",
        "sciUp": "1区"
      }
    }
  },
  "meta": {
    "read_source": "hybrid",
    "total": 1
  }
}
```

## full 模式字段表

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | string | 条目唯一标识 |
| `version` | int | 乐观并发版本号（写操作 `--if-unmodified-since-version` 用） |
| `item_type` | string | 文献类型 |
| `creators` | object[] | 完整作者列表（`name` + `creator_type`） |
| `container` | string | 期刊/书名/会议名 |
| `date_added` | string | 加入 Zotero 的时间 |
| `attachments` | object[] | 附件（PDF 文件路径、大小、解析状态） |
| `notes` | object[] | 子笔记（HTML `content` + 纯文本 `preview`） |
| `annotations` | object[] | PDF 标注（高亮/便签类型、页码、颜色、坐标） |
| `journal_rank` | object/null | 期刊等级（需 EasyScholar 数据），含 `matched_name` 和 `ranks` 字典 |
| `matched_chunk` | object/null | FTS5 命中位置（`text` / `page` / `bbox` / `attachment_key`），仅 `--snippet` 时 |
| `full_text_preview` | string | FTS5 上下文片段，仅 `--snippet` 时 |

## `journal_rank.ranks` 常见键

`ranks` 是 `map[string]string`，常见键（按优先级）：

| 键 | 含义 |
|------|------|
| `sciif` | 中科院分区影响因子（数字） |
| `sci` | 中科院分区（`Q1` / `Q2` ...） |
| `sciUp` | 中科院升级版分区（`1区` / `2区` ...） |
| `jci` | JCR 引文指数 |
| `esi` | ESI 学科（`HCP` / `CP` / `...`） |
| `sciBase` | 中科院基础版（`TOP` / `...`） |
| `sciUpAlt` | 中科院升级版（备用别名） |

> EasyScholar 数据从 `{ZOT_DATA_DIR}/.zotero_cli/zoterostyle.json` 加载；未加载时 `journal_rank` 为 `null`。

## relate --aggregate 输出

```bash
zot relate ABCD1234 --aggregate --json
```

返回 `data.AggregatedRelations`（`internal/domain/types.go:97`）：

```json
{
  "ok": true,
  "command": "relate",
  "data": {
    "self": [
      {"predicate": "dc:relation", "direction": "outgoing",
       "target": {"key": "EFGH5678", "item_type": "journalArticle",
                  "title": "Base editing with CRISPR", "date": "2024-01",
                  "creators": ["Liu David"], "tags": ["基因编辑"]}}
    ],
    "notes": [
      {
        "source": {"key": "NOTE_KEY1", "title": "Reading summary", "date": "2024-04"},
        "preview": "重要笔记：本文综述了...",
        "relations": [
          {"predicate": "dc:relation", "direction": "outgoing",
           "target": {"key": "MNOP3456", "item_type": "journalArticle",
                      "title": "Another paper", "date": "2023-12"}}
        ]
      }
    ],
    "citations": [
      {
        "source_key": "NOTE_KEY1",
        "targets": [
          {"key": "QRST6789", "title": "Cited paper A"},
          {"key": "UVWX0123", "title": "Cited paper B"}
        ]
      }
    ]
  },
  "meta": {
    "read_source": "hybrid"
  }
}
```

> **remote 模式限制**：`--aggregate` / `--add` / `--remove` / `--from-file` 仅 local/hybrid 支持；remote 下 `relate --json` 只返回 `self`（来自 `itemRelations` 表）。
