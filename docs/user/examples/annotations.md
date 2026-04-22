# annotations — PDF 标注读取（双源）

## 命令

```bash
zot annotations ABC123DE --json
```

## 输出

```json
{
  "ok": true,
  "command": "annotations",
  "data": {
    "db": [
      {
        "source": "zotero_reader",
        "key": "ANNO1",
        "type": "highlight",
        "text": "homoploid hybrid speciation requires the coupling of assortative mating and ecological selection",
        "comment": "核心定义",
        "color": "#ffd400",
        "page_label": "2",
        "page_index": 1,
        "date_added": "2024-04-10T14:30:00Z"
      },
      {
        "source": "zotero_reader",
        "key": "ANNO2",
        "type": "note",
        "text": "",
        "comment": "方法可复用",
        "color": "#ff6666",
        "page_label": "5",
        "page_index": 4,
        "date_added": "2024-04-10T15:22:00Z"
      }
    ],
    "pdf": [
      {
        "source": "pdf_file",
        "type": "highlight",
        "text": "reproductive isolation barriers",
        "color": "#ffd400",
        "page": 3,
        "rect": [100.5, 200.0, 350.0, 218.0],
        "author": "",
        "mod_date": "2024-04-11T09:15:00Z"
      },
      {
        "source": "pdf_file",
        "type": "ink",
        "text": "",
        "color": "#000000",
        "page": 1,
        "rect": [50.0, 600.0, 200.0, 750.0],
        "author": "",
        "mod_date": "2024-04-12T16:40:00Z"
      }
    ],
    "summary": {
      "db_count": 2,
      "pdf_count": 2,
      "total": 4
    }
  },
  "meta": {
    "attachment_key": "ATTACH1",
    "read_source": "local"
  }
}
```

## 双源对比

| 维度 | db（Zotero Reader） | pdf（PDF 文件内） |
|------|---------------------|-------------------|
| **数据来源** | SQLite `itemAnnotations` 表 | PyMuPDF 扫描 PDF 二进制 |
| **时间戳** | `date_added`（创建时间） | `mod_date`（修改时间） |
| **标注类型** | highlight / note / image / ink | highlight / note / underline / strikeout / squiggly / circle / polygon / ink |
| **位置信息** | `page_label` + `position` (JSON) | `page` + `rect` ([x0,y0,x1,y2]) |
| **作者信息** | 无 | `author`（PDF 属性中记录） |
| **删除能力** | 不支持（通过 Zotero UI 删除） | `--clear` 可删除 |

## 过滤与删除

```bash
# 仅查看高亮
zot annotations KEY --type highlight --json

# 仅查看第 3 页
zot annotations KEY --page 3 --json

# 删除 PDF 文件内的所有高亮
zot annotations KEY --clear --type highlight
```
