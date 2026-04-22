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
      "journal_rank": {
        "matched_name": "Nature Ecology & Evolution",
        "ranks": {
          "sciif": "15.7",
          "sci": "Q1",
          "jci": "3.34",
          "sciUp": "生物1区",
          "esi": "环境／生态学"
        }
      },
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
|------|------|
| `key` | string | Zotero 条目唯一标识 |
| `item_type` | string | 文献类型（journalArticle / book / preprint 等） |
| `matched_on` | []string | 命中原因（title / creators / tags / fulltext） |
| `attachments[].resolved` | bool | 附件路径是否可解析 |
| `journal_rank` | object | 期刊等级信息（仅期刊文章有，依赖 `zoterostyle.json` 数据） |
| `meta.read_source` | string | 数据来源（local / web / hybrid） |

### journal_rank 字段说明

> **数据来源**：`journal_rank` 数据来自 [EasyScholar](https://www.easyscholar.cc/console/user/open)，需要在 Zotero 中安装 [绿青蛙插件](https://www.easyscholar.cc/blogs/10009) 并同步数据。确保 `ZOT_DATA_DIR` 指向包含 `zoterostyle.json` 的目录即可自动加载。

当文献类型为 `journalArticle` 且匹配到期刊排名数据时，会自动附加 `journal_rank` 字段：

```json
"journal_rank": {
  "matched_name": "Nature Ecology & Evolution",
  "ranks": {
    "sciif": "15.7",      // SCI 影响因子
    "sci": "Q1",          // SCI 分区
    "jci": "3.34",        // JCI 指数
    "sciUp": "生物1区",    // 中科院升级版分区
    "esi": "环境／生态学"  // ESI 学科分类
  }
}
```

支持的排名字段包括：

| 字段 | 说明 | 字段 | 说明 |
|------|------|------|------|
| `sciif` | SCI 影响因子 | `jci` | JCI 指数 |
| `sciif5` | SCI 五年影响因子 | `esi` | ESI 学科分类 |
| `sci` | SCI 分区 (Q1-Q4) | `ssci` | SSCI 分区 |
| `sciUp` | 中科院升级版分区 | `sciUpSmall` | 中科院小类分区 |
| `sciUpTop` | 中科院 TOP 分区 | `sciBase` | 中科院基础版分区 |
| `ccf` | CCF 计算机分级 | `eii` | EI 检索 |
| `cscd` | CSCD 核心 | `cssci` | CSSCI 南大核心 |
| `pku` | 北大核心 | `zhongguokejihexin` | 中国科技核心期刊 |
| `swjtu/sdufe/sdufe/hhu/nju/fdu/scu/cug/zju/xju/sjtu/xmu/cufe/uibe/ruc/cqu/cju/cpu` | 各高校期刊分级 | | |

## 带过滤的示例

```bash
# 全文检索 + 片段预览（local/hybrid）
zot find "CRISPR gene editing" --fulltext --snippet --json

# 按日期和标签筛选
zot find "evolution" --date-after 2023 --tag "review" --json

# 返回完整字段
zot find "speciation" --full --json
```
