# 引用索引与文献发现

`zot ref` 把 Zotero 条目连接到 PubMed、PubMed Central（PMC）和 Europe PMC，并维护一个可全文检索的本地引用索引。NCBI 是权威核心来源；Europe PMC 是自动或按需的增强层；GROBID 仅是显式启用的实验性 PDF 后备。

## 数据流与优先级

```text
Zotero item
  └─ DOI / PMID / PMCID 识别
      ├─ PubMed EFetch
      │   └─ 书目、MeSH、文献类型、关键词、化学物质、基金、勘误/撤稿
      ├─ PMC JATS（有 PMCID 时优先）
      │   └─ 完整参考文献 + 正文引用上下文
      ├─ PubMed ELink（没有 PMC 全文时）
      │   └─ 可映射到 PMID 的参考文献
      │       └─ Europe PMC references 补充 DOI、预印本和非 PubMed 引用
      ├─ Europe PMC（增强）
      │   └─ 外部被引、数据库链接、annotations、版本、评价、OA、基金
      └─ GROBID（实验性、显式调用）
          └─ 本地 PDF → TEI → 参考文献与上下文
```

默认构建不会调用 GROBID。PMC JATS 成功时也不会重复调用 Europe PMC references；只有 PubMed fallback 路线才进行参考文献补全。Europe PMC 暂时不可用时，NCBI 核心结果仍然有效。

## 常用工作流

### 查看和构建引用

```powershell
zot ref show ITEMKEY --json
zot ref build --workers 3 --json
zot ref status --json
zot ref status --failed --json
zot ref build --failed --workers 2 --json
```

`ref ITEMKEY` 等同于 `ref show ITEMKEY`。`ref build` 扫描符合条件的顶层条目，按 Zotero 指纹和索引数据版本增量跳过。索引格式升级时，即使 Zotero 条目没有变化，旧成功记录也会自动补建一次。

确定缺少 DOI、PMID 和 PMCID、无法进入 NCBI 的条目记录为 `unsupported`，不会被 `retry` 反复请求：

```powershell
zot ref status --unsupported --json
```

### 解析本地引用图

```powershell
zot ref resolve --workers 8 --json
zot ref cited ITEMKEY --json
zot ref ctx ITEMKEY --json
```

`resolve` 依次使用 DOI、PMID、规范化标题和保守模糊标题匹配，将引用链接回本地 Zotero item key。默认 `cited-by` 只查询本地索引，因此能返回引用来源条目和正文引用上下文。

Europe PMC 外部被引网络按需查询：

```powershell
zot ref cited ITEMKEY --external --limit 100 --json
```

外部结果可能包含 `MED`（PubMed）和 `PPR`（预印本）记录。Europe PMC 的总被引数与当前页可返回数量可能不同；JSON 的 `meta.total` 和 `meta.returned` 分别表示两者。

### 搜索参考文献、上下文和主题

```powershell
zot ref find 'genome AND assembl*' --json
zot ref find '"we used" OR protocol*' --in contexts --section methods --json
zot ref find 'genome AND assembl*' --in references --json
zot ref find '"Hendra Virus"' --in metadata --field mesh --json
zot ref find 'Review' --in metadata --field publication_types --json
zot ref find 'BRCA1' --in metadata --field annotations --json
```

QUERY 直接使用 SQLite FTS5 语法，支持 `"完整短语"`、`prefix*`、`AND` / `OR` / `NOT` 和括号。默认搜索结构化参考文献、引用上下文和 PubMed 元数据。可用范围与字段：

| 选项 | 范围 |
|---|---|
| `--in contexts` | PMC/GROBID 正文引用语境 |
| `--in references` | 结构化参考文献 |
| `--in metadata` | PubMed 元数据 |
| `--in all` | 合并以上范围（默认） |
| `--field mesh` | MeSH descriptor、UI、qualifier |
| `--field publication_types` | Journal Article、Review、Clinical Trial 等 |
| `--field keywords` | 作者/出版方关键词 |
| `--field chemicals` | 化学物质、MeSH UI、登记号 |
| `--field grants` | 基金编号、机构、国家、缩写 |
| `--field corrections` | 勘误、撤稿、更新关系 |
| `--field annotations` | 已按需保存的 Europe PMC 实体与关系 |

`--source` 支持 `pmc`、`pubmed`、`europepmc` 和 `grobid`；还可以用 `--target ITEMKEY`、`--section TEXT` 和 `--limit N` 过滤。

### PubMed 文献发现

```powershell
zot ref related ITEMKEY --limit 20 --json
zot ref related ITEMKEY --also-viewed --limit 20 --json
```

默认使用 PubMed Similar Articles 的官方顺序，并排除原文。`--also-viewed` 使用 frequently viewed together；它是行为数据，覆盖不稳定，空结果不表示错误。

### 关联生物医学资源

```powershell
zot ref links ITEMKEY --json
```

命令并行查询 NCBI 的 PMC、Gene、GEO DataSets、SRA、BioProject、BioSample、ClinVar、Assembly，再自动合并 Europe PMC database links。相同数据库和 accession 会去重。文本模式只展示每类前 10 个 ID，JSON 保留完整列表。

### Europe PMC annotations

```powershell
zot ref entities ITEMKEY --json
zot ref find "GCA_009914755" --field annotations --json
```

annotations 可能包括基因/蛋白、疾病、化学物质、物种、GO、实验方法、突变、基因—疾病关系、蛋白互作、细胞系、数据 accession 和研究资源。每次获取会写入本地 `ref_annotations` 与 FTS 表；重复获取替换该条目的旧 annotations，不会重复累积。

annotations 属于文本挖掘结果，可能存在误报。为控制数据量，它不参加默认全库 `ref build`，只在显式运行 `ref entities` 时抓取。

注意不要把它和顶层的 PDF 标注命令混淆：

```powershell
zot ann list ITEMKEY       # Zotero/PDF 人工标注
zot ref entities ITEMKEY   # Europe PMC 文本挖掘实体
```

### Europe PMC 开放科学画像

```powershell
zot ref profile ITEMKEY --json
```

`profile` 汇总：

- MED/PPR 标识、PMID、PMCID、DOI；
- 预印本与正式发表版本关系；
- Europe PMC evaluations（存在时）；
- 基金编号、机构和缩写；
- 开放获取状态与许可证；
- Europe PMC 被引数；
- 是否具有参考文献、数据链接和文本挖掘结果。

它优先使用 PMID；没有 PMID 的预印本可以通过 DOI 在 Europe PMC 中解析。基金详情来自文章 core metadata；如需项目摘要、PI、起止时间和机构详情，应再使用 Europe PMC GRIST 数据，而不是把每个基金的额外请求放进默认构建。

## PubMed 元数据模型

成功解析的 PubMed 条目会保存：

- MeSH descriptor：标准 UI、名称、`MajorTopicYN`；
- MeSH qualifier：UI、名称、是否 major topic；
- publication types；
- author/publisher keywords；
- chemicals：名称、MeSH UI、registry number；
- grants：ID、agency、country、acronym；
- comments/corrections：类型、关联 PMID、原始来源。

这些字段来自引用流程已经使用的 PubMed EFetch XML，不为每个字段增加独立网络请求。

## 引用上下文完整性

每个成功索引条目记录：

| 状态 | 含义 |
|---|---|
| `available` | 已解析至少一个正文引用上下文 |
| `not_supported` | 来源没有正文上下文，例如 PubMed ELink |
| `not_found` | 全文可解析，但没有找到上下文 |
| `parse_failed` | 全文存在但上下文解析失败 |
| `not_indexed` | 历史记录尚未补建 |

同时保存上下文总数、有/无上下文的参考文献数量和覆盖率。历史 PMC 条目可单独补建：

```powershell
zot ref build --contexts --workers 3 --json
```

默认复用缓存；只有 `--refresh` 才重新请求网络。

## 缓存、限速与性能

结构化索引位于：

```text
<Zotero data dir>/.zotero_cli/ref/index.sqlite
```

NCBI 与 Europe PMC HTTP 响应位于：

```text
<Zotero data dir>/.zotero_cli/ref/ncbi/
```

Europe PMC 使用独立的缓存键命名空间，不会使已有 NCBI 缓存失效。GROBID TEI 使用单独的 `grobid` 缓存目录。

NCBI 默认请求间隔为 400 ms；设置 `ZOT_NCBI_API_KEY` 后为 125 ms。可用 `ZOT_NCBI_RATE_MS` 覆盖。所有并发 worker 共用一个客户端限速器，增加 `--workers` 不会绕过 NCBI 限速。

当前实测参考值（网络和数据覆盖会变化）：

| 操作 | 首次 | 缓存命中 |
|---|---:|---:|
| PubMed Similar Articles（10 条） | 约 1.9 s | 约 130 ms |
| NCBI + Europe PMC links | 约 5.8 s | 约 74–113 ms |
| 本地 MeSH FTS 搜索 | 约 256 ms | 本地操作 |

`--refresh` 会绕过 HTTP 与条目索引缓存，应只用于明确需要更新或诊断的场景。

## GROBID 的边界

```powershell
zot ref status --grobid --json
zot ref build --grobid --limit 5 --workers 1 --json
```

GROBID 不属于默认构建路线。公共演示服务不保证配额、稳定性、隐私或完成时间；敏感 PDF 应使用本地 GROBID。`--all` 才允许显式全量运行。

## 机器可读输出

Agent 工作流应优先使用 `--json`。常见 `meta` 字段包括：

- `strategy`：`pmc_jats`、`pubmed` 或 `grobid`；
- `index_hit`、`cache_hits`、`network_calls`、`elapsed_ms`；
- `context_summary`；
- `source: europe_pmc`；
- `total`、`returned`、`limit`、`mode`。

Europe PMC 是增强来源，不会把主条目的 `strategy` 改成 `europe_pmc`；只有由它补入的单条参考文献使用 `source: europe_pmc`。
