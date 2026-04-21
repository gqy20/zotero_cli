# AI Agent Guide

这份文档面向会调用 `zot` 的 AI agent 或自动化脚本。

如果你的运行环境支持仓库内 skill，也可以直接参考：

- [.codex/skills/zotero-cli/SKILL.md](/d:/C/Documents/Program/Go/zotero_cli/.codex/skills/zotero-cli/SKILL.md)

## 首次使用

先做三步：

```powershell
.\zot.exe init
.\zot.exe config validate
.\zot.exe stats --json
```

如果只是在仓库里开发，也可以直接：

```powershell
go run .\cmd\zot config validate
```

## 推荐调用习惯

### 1. 默认使用 JSON

优先：

```powershell
.\zot.exe find "hybrid speciation" --json
.\zot.exe show SA6DHVIM --json
.\zot.exe relate SA6DHVIM --json
.\zot.exe annotations SA6DHVIM --json       # 读取 PDF 标注（双源，含时间戳）
.\zot.exe select SA6DHVIM                     # 跳转到 Zotero UI 选中条目
.\zot.exe stats --json
```

补充：

- 当你需要精确关系而不是相似性推断时，优先用 `relate --json`
- 当你运行在有本地 Zotero 数据目录的机器上，优先把 `ZOT_MODE` 设成 `hybrid`

### 2. 检索尽量一次拿够字段

如果后续需要 DOI、URL、版本号，不要先跑简版 `find` 再逐条 `show`。优先：

```powershell
.\zot.exe find "hybrid speciation" --json
.\zot.exe find "hybrid speciation" --include-fields url,doi,version
.\zot.exe find "hybrid speciation" --full
```

说明：

- `--json` 会返回完整 `Item` 结构
- `--include-fields` 和 `--full` 主要增强文本模式

### 3. 需要全库扫描时优先显式表达

```powershell
.\zot.exe find --all --json
.\zot.exe find "" --json
.\zot.exe versions items --since 0 --json
```

### 4. 按时间和标签筛选

```powershell
.\zot.exe find "hybrid speciation" --date-after 2020 --json
.\zot.exe find "hybrid speciation" --date-after 2020-01 --date-before 2024-12-31 --json
.\zot.exe find "hybrid speciation" --tag "物种形成" --tag "经典案例" --json
.\zot.exe find "hybrid speciation" --tag "物种形成" --tag "经典案例" --tag-any --json
```

语义：

- 多个 `--tag` 默认是 AND
- `--tag-any` 把多个标签改成 OR
- 日期支持 `YYYY`、`YYYY-MM`、`YYYY-MM-DD`

### 4.1 本地与混合模式

推荐：

```powershell
.\zot.exe config show
.\zot.exe find "hybrid speciation" --json
.\zot.exe show KYL55LW2 --json
.\zot.exe relate BLT3R329 --json
```

说明：

- `local`：只读本地 SQLite 和 `storage/`
- `hybrid`：优先本地，但只在 Web 确实能承接请求时回退
- 本地 `find` 不支持 `--qmode` 和 `--include-trashed`
- `hybrid` 下 `find --qmode` / `find --include-trashed` 可以回退 Web
- `hybrid` 下 `find --fulltext` / `find --snippet` / 附件过滤不会回退 Web
- `relate` 当前依赖本地 SQLite 的 `itemRelations`
- `relate` 和 `extract-text` 在 `hybrid` 下仍然是本地能力，不要假设有远程兜底

### 5. 批量导出收藏夹

```powershell
.\zot.exe export --collection COLL1234 --format csljson --json
```

补充：

- `csljson` 在 `local` / `hybrid` 下优先走本地导出
- `hybrid` 下只有可预期的本地缺失/暂时不可用错误才回退 Web
- 如果本地导出报的是异常错误，应保留错误，不要自动假设 Web 结果等价

## 安全规则

### 删除默认禁止

如果 `ZOT_ALLOW_DELETE=0`，所有 delete 命令都会失败。这是预期行为，不要绕过。

### 写操作前先确认权限

在执行任何 create/update/delete 之前，先确认：

1. `config validate` 通过
2. 当前配置允许写入
3. 删除是否被显式允许
4. 用户是否明确要求修改或删除

### 删除时必须更谨慎

对 agent 来说，下面这些命令都属于高风险：

- `delete-item`
- `delete-collection`
- `delete-search`

执行前应该：

1. 复述目标对象
2. 检查 key 是否正确
3. 检查 version precondition
4. 如有歧义，先问用户

## 常见工作流

### 快速库概览（Agent 入口）

```powershell
.\zot.exe overview --json          # 一次获取统计、收藏夹、标签、最近条目
```

返回 `data.stats` / `data.collections` / `data.tags` / `data.recent_items`，无需多次 API 调用。适合作为 agent 首次连接时的发现命令。

### 查找并查看详情

```powershell
.\zot.exe find "attention is all you need" --json
.\zot.exe show X42A7DEE --json
.\zot.exe relate X42A7DEE --json
```

### 批量打标签

```powershell
.\zot.exe add-tag --items KEY1,KEY2 --tag "to-read" --json
```

### 批量移除标签

```powershell
.\zot.exe remove-tag --items KEY1,KEY2 --tag "obsolete" --json
```

### 按收藏夹导出

```powershell
.\zot.exe export --collection COLL1234 --format bibtex
```

### 同步感知查询

```powershell
.\zot.exe versions items --since 0 --json
.\zot.exe versions items --since 100 --if-modified-since-version 120 --json
```

### PDF 文本提取

```powershell
# 提取单篇 PDF 正文（local / hybrid）
.\zot.exe extract-text ITEMKEY --json
# JSON 输出包含：主附件文本、所有 PDF 附件文本、缓存命中状态、来源元信息
```

> 提取器优先级：**PyMuPDF** → Zotero ft-cache → pdfium WASM。首次使用需 `zot init --pdf` 或在 init 交互中确认安装。

### PDF 标注操作

```powershell
# 双源读取标注（Zotero Reader 数据库 + PDF 文件内）
.\zot.exe annotations ITEMKEY --json
# 按类型/页码过滤
.\zot.exe annotations ITEMKEY --type highlight --page 3 --json
# 删除 PDF 文件内特定类型的标注
.\zot.exe annotations ITEMKEY --clear --type highlight

# 写入高亮标注到 PDF
.\zot.exe annotate ITEMKEY --text "关键概念" --color red --comment "重要"
# 写入下划线
.\zot.exe annotate ITEMKEY --text "speciation" --type underline --color blue
# 在指定位置添加笔记
.\zot.exe annotate ITEMKEY --page 5 --point 150,300 --comment "发现" --type note
```

### 与 Zotero 桌面端联动

```powershell
# 在 Zotero 阅读器中打开 PDF（支持页码跳转）
.\zot.exe open ITEMKEY --page 5
# 在 Zotero 主界面选中条目
.\zot.exe select ITEMKEY
```

### 笔记搜索

```powershell
# 列出所有子笔记
.\zot.exe notes --json
# 按关键词过滤笔记内容
.\zot.exe notes --query "CRISPR" --limit 20 --json
```

### 全文检索最佳实践

```powershell
# local / hybrid 模式下，FTS 索引有数据时自动启用全文检索
.\zot.exe find "同源多倍体" --snippet --json
# 等价于（显式指定）：
.\zot.exe find "同源多倍体" --fulltext --snippet --json

# snippet 默认限制 50 条（批量安全限制）
# 需要更多结果时显式指定 --limit
.\zot.exe find "基因编辑" --snippet --limit 200 --json

# 任一词匹配（而非默认的短语匹配）
.\zot.exe find "基因 编辑" --fulltext-any --snippet --json
```

### 高级过滤组合

```powershell
# 组合多个过滤条件
.\zot.exe find "CRISPR" `
    --collection ABC123 `
    --tag "高引用" `
    --exclude-tag "已读" `
    --date-after 2023 `
    --has-pdf `
    --attachment-name PDF `
    --modified-within 90d `
    --json

# 排除特定类型
.\zot.exe find "review" --no-type "journalArticle" --json

# 标签模糊匹配
.\zot.exe find "" --tag-contains "进化" --json
```

## 性能优化建议

### 检索性能

- **`--snippet` 默认限制 50 条**：使用 `--fulltext --snippet` 时若未指定 `--limit`，自动限制为 50 条以保护批量提取性能。需要更多结果时显式加 `--limit N`。
- **自动全文检索**：local / hybrid 模式下 FTS5 索引有数据时，即使不加 `--fulltext` 也会自动走全文检索路径。无需每次显式指定。
- **`--include-fields` 减少传输**：只需要特定字段时用 `--include-fields url,doi,version` 减少 JSON 体积。
- **优先 `--full` 而非先 `find` 再逐条 `show`**：一次获取完整数据比多次往返更高效。

### API 调优

- **重试参数**（高频脚本场景）：
  - `ZOT_RETRY_MAX_ATTEMPTS=5` — 最大重试次数
  - `ZOT_RETRY_BASE_DELAY_MS=1000` — 基础延迟
  - `ZOT_RETRY_JITTER_FRACTION=0.3` — 抖动比例（避免惊群）
- 遇到 `429` 时自动指数退避 + 抖动，但高频批量脚本仍应主动降速。

### 缓存行为

- `extract-text` 结果会缓存，重复提取同一 PDF 直接命中缓存。
- 全文检索的 snippet 提取也有缓存层，相同查询片段不会重复扫描 PDF。

## 失败时的处理建议

- `401` / `403`
  - 检查 `api_key`、`library_type`、`library_id`
  - 跑一次 `config validate`
- `412`
  - 说明库版本已变化，刷新后重试
- `429`
  - CLI 已有基础重试，但高频脚本仍应降速
- local temporary unavailable
  - 对 `find --fulltext`、`find --snippet`、`relate`、`extract-text` 这类本地能力，优先保留本地错误，不要强行改走 Web
- 配置缺失
  - 运行 `zot init`

### 结构化错误输出

设置 `ZOT_JSON_ERRORS=1` 后，所有错误以 JSON 格式输出到 stdout（而非 stderr 纯文本），便于 agent 可靠解析：

```json
{"ok": false, "command": "show", "data": "item not found: ABCD", "code": 1}
```

- `ok`: 始终 `false`
- `command`: 出错的命令名
- `data`: 错误消息字符串
- `code`: 退出码（`1`=运行时错误, `2`=用法错误, `3`=配置错误）

未设置时保持原有纯文本行为（stderr 输出 + 非零退出码）。

## 优先级建议

如果你是一个通用 agent，不确定该用哪条命令：

1. **发现**：`overview --json`（一站式库快照）
2. **读优先**：`find` / `show` / `relate` / `stats` / `notes`
3. **PDF 读取**：`extract-text` / `annotations` / `open`
4. **导出**：`export --collection` 或 `export --item-key`
5. **变更次之**：`create-*` / `update-*` / `add-tag` / `remove-tag` / `annotate`
5. **删除最后**：只有在用户明确要求时才考虑
