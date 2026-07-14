# AI Agent 快速上手指南

面向会调用 `zot` 的 AI agent 或自动化脚本。如果你是人工终端用户，请参考 [命令参考](./commands.md)。

如果你的运行环境支持仓库内 skill，也可以直接参考：

- [.codex/skills/zotero-cli/SKILL.md](../../.codex/skills/zotero-cli/SKILL.md)

---

## 首次使用（三步走）

```powershell
.\zot.exe config init                 # 一键初始化（交互式：模式、API key、库 ID，可选 PyMuPDF）
.\zot.exe config check     # 校验配置
.\zot.exe lib stats --json          # 验证连通性
```

如果在仓库里开发：

```powershell
go run .\cmd\zot config check
```

## 推荐调用习惯

### 1. 默认使用 JSON

```powershell
.\zot.exe find "hybrid speciation" --json
.\zot.exe show SA6DHVIM --json
.\zot.exe lib stats --json
```

### 2. 检索尽量一次拿够字段

```powershell
.\zot.exe find "hybrid speciation" --include-fields url,doi,version
.\zot.exe find "hybrid speciation" --full
```

默认 `find/show --json` 返回稳定的轻量结构：作者摘要使用 `creator_summary`，收藏夹名称使用 `collection_names`。只有显式 `--full` 才返回完整 Item（其中 `creators`、`collections` 为对象数组）；`--snippet` 返回轻量元数据和 `matched_chunk` 证据，不能与 `--full` 同时使用。

### 3. 全库扫描时显式表达

```powershell
.\zot.exe find --all --json
.\zot.exe find "" --json
```

### 4. 按时间和标签筛选

```powershell
.\zot.exe find "query" --date-after 2020 --tag "物种形成" --tag "经典案例" --json
.\zot.exe find "query" --tag "A" --tag "B" --tag-any --json    # OR
```

`--date-after` / `--date-before` 过滤的是发表日期。日期支持 `YYYY` / `YYYY-MM` / `YYYY-MM-DD`；local/hybrid 也兼容 Zotero 常见的部分日期字符串，如 `YYYY-MM-00 YYYY-MM` 和 `MM/YYYY`。

### 5. 最近入库和最近修改

```powershell
# 最近加入 Zotero 的条目
.\zot.exe find --sort dateAdded --order desc --limit 10 --json

# 只看最近 7 天入库
.\zot.exe find --all --added-since 7d --sort dateAdded --order desc --json

# 最近修改元数据的条目
.\zot.exe find --all --modified-within 7d --sort dateModified --order desc --json
```

快速人工浏览标题时，可用文本模式控制字段：

```powershell
.\zot.exe find --sort dateAdded --order desc --limit 10 --include-fields title,date_added,container
```

### 6. 运行模式选择

推荐设 `ZOT_MODE=hybrid`：

- `web`：纯 Zotero Web API，无本地依赖
- `local`：只读本地 SQLite + `storage/`
- `hybrid`：优先本地，Web 仅在能承接时回退
- `remote`：通过 HTTP 连接远程 `zot server`，适合无本地 Zotero 的设备

`ref related` / `pdf text` 在 hybrid 下仍可使用本地能力。remote 模式通过服务器代理访问数据；其中 `ann list/new/delete` 走服务器端 PDF 能力，其他普通写操作仍需额外配置 `ZOT_API_KEY`。

结构化参考文献、PubMed 主题和 Europe PMC 文献发现使用 `zot ref`。第一次使用建议先运行小批量增量构建：

```powershell
.\zot.exe ref build --workers 3 --limit 20 --json
.\zot.exe ref status --json
.\zot.exe ref find "genome assembly" --json
```

`config check` 会同时报告 Zotero 桌面 Connector 是否可用。Connector 未启动不会使整体配置检查失败，但导入 PDF 前必须启动 Zotero 桌面端。

完整说明见 [引用索引与文献发现](./references.md)。

详见 [架构文档 - 四种模式](../architecture/overview.md#四种模式)。

## 安全规则

| 规则 | 说明 |
|------|------|
| **删除默认禁止** | `ZOT_ALLOW_DELETE=0` 时所有 delete 命令失败，这是预期行为 |
| **写操作前确认** | 先 `config check`，再检查权限开关和用户意图 |
| **删除需谨慎** | `delete-item` / `delete-collection` / `delete-search` 属高风险操作 |

执行写操作前建议：
1. 复述目标对象
2. 检查 key 是否正确
3. 检查 version precondition
4. 如有歧义，先问用户

## 常见工作流

### Agent 入口：一站式库概览

```powershell
.\zot.exe lib show --json   # 统计 + 收藏夹 + 标签 + 最近条目 + 索引状态
```

返回 `data.stats` / `data.collections` / `data.tags` / `data.recent_items`，适合作为首次连接时的发现命令。

### 查找并查看详情

```powershell
.\zot.exe find "attention is all you need" --json
.\zot.exe show X42A7DEE --json
.\zot.exe item show X42A7DEE --json
```

### PDF 操作

```powershell
# 提取文本（PyMuPDF → ft-cache → pdfium WASM）
.\zot.exe pdf text ITEMKEY --json  # 返回本地全文缓存路径，直接读取 content_path
.\zot.exe pdf text ITEMKEY --json --pages 3-8 --grep methods --max-chars 12000
.\zot.exe pdf text ITEM1 ITEM2 --json --grep "gene\s+flow|introgression"
.\zot.exe pdf text --collection "研究/植物/栗属" --json --grep "gene\s+flow|introgression"

# 查找本地已保存的 Supplementary / Source data / 表格数据附件
.\zot.exe item supp ITEMKEY --json
.\zot.exe item supp ITEMKEY --online --json
.\zot.exe item supp --all --json --limit 50
.\zot.exe file show ATTKEY --json
.\zot.exe file show --item ITEMKEY --json
.\zot.exe file show --item ITEMKEY --health --json
.\zot.exe find --missing-attachment --json

# 双源读取标注（DB + PDF 文件内）
.\zot.exe ann list ITEMKEY --json
.\zot.exe ann list ITEMKEY --attachment ATTACHMENT_KEY --json  # 多 PDF 时精确选择

# 事务式写入标注到 PDF（临时副本验证后替换）
.\zot.exe ann new ITEMKEY --attachment ATTACHMENT_KEY --text "关键概念" --color red --comment "重要"

# 删除必须选择 Zotero 或 PDF 来源，并建议先预览
.\zot.exe ann delete ITEMKEY --source zotero --type highlight --dry-run --json
.\zot.exe ann delete ITEMKEY --source zotero --type highlight --yes --json

# remote 模式下以上两条也可用，但实际执行发生在 zot server 所在机器

# 在 Zotero 阅读器中打开
.\zot.exe pdf open ITEMKEY --page 5
```

`--grep` 默认按不区分大小写的 Go 正则解析；有分页缓存时结果包含附件 key、命中页、`match_count` 和上下文。它只检索，不会自动创建标注。

全文缓存会校验 PDF 的路径、大小和高精度修改时间；附件被替换后，下一次读取或索引会重新提取，不会继续返回旧正文。

### 批量操作

```powershell
.\zot.exe item tag KEY1 KEY2 --tag "to-read" --json
.\zot.exe find --collection COLL1234 --all --json > selected.json
.\zot.exe item export --from selected.json --as csljson
.\zot.exe item import .\paper.pdf --collection "研究/植物/栗属" --json

# 正则批量改名：默认预览，加 --yes 才写入
.\zot.exe tag replace --match '^(Gene Flow|Gene flow|gene flow)$' --replace 'Gene Flow'
.\zot.exe tag replace --match '^植物/(.+)$' --replace '物种/植物/$1' --yes

# tag-plan.json 可为不同条目同时新增或移除多个标签
.\zot.exe tag apply --from .\tag-plan.json --dry-run --json
.\zot.exe tag apply --from .\tag-plan.json --json
```

批量写操作自动按 Zotero 官方上限每 50 条切分，并在批次之间衔接库版本号。`tag apply` 在全部批次完成后统一读取并核验最终标签。

导入的 `--collection` 支持收藏夹 key、唯一名称或完整层级路径。同名时命令会列出带 key 的候选路径，不会自动猜测；导入完成后会为最终保留的附件增量建立全文索引。

### 全文检索最佳实践

```powershell
# local / hybrid 模式下，显式选择 FTS5 全文检索
.\zot.exe find '"同源多倍体"' --in fulltext --snippet --json
# snippet 默认限制 20 条，需要更多结果时显式指定 --limit
.\zot.exe find '"基因编辑"' --in fulltext --snippet --limit 200 --json
```

默认 `--in metadata` 始终查询元数据；全文查询必须显式使用 `--in fulltext`，不会因本地索引状态而改变语义。

## 性能优化建议

| 建议 | 说明 |
|------|------|
| **默认分页上限** | 轻量 `find` 默认 100 条，`--snippet` / `--full` 默认 20 条；只有显式 `--all` 才取消上限 |
| **显式全文检索** | local/hybrid 下使用 `--in fulltext` 查询 FTS5；默认元数据查询不隐式切换路径 |
| **`--include-fields` 控制文本输出** | 快速人工浏览时只展示指定字段；JSON 需要完整 Item 时显式使用 `--full` |
| **优先 `--full`** | 一次获取完整数据比多次往返更高效 |

### API 调优环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ZOT_RETRY_MAX_ATTEMPTS` | 5 | 最大重试次数 |
| `ZOT_RETRY_BASE_DELAY_MS` | 1000 | 基础延迟 |
| `ZOT_RETRY_JITTER_FRACTION` | 0.3 | 抖动比例（避免惊群） |

遇到 429 时 CLI 已有指数退避 + 抖动，但高频脚本仍应主动降速。

## 失败处理

| 错误 | 处理方式 |
|------|----------|
| `401` / `403` | 检查 api_key / library_type / library_id，跑 `config check` |
| `412` | 库版本已变化，刷新后重试 |
| `429` | CLI 已重试，高频脚本应降速 |
| local temporary unavailable | 保留本地错误，不要强行改走 Web |
| 配置缺失 | 运行 `zot config init` |
| Zotero Desktop Connector 不可用 | 启动 Zotero 桌面端后重试 `item import` |

### 结构化错误输出

命令使用 `--json`（或 `--format json` / `ZOT_OUTPUT=json`）时，错误也以 JSON 输出到 stdout；文本模式错误写入 stderr：

```json
{"ok":false,"command":"item show","error":{"type":"not_found","message":"item not found: ABCD"},"code":1}
```

- `code`: `1`=运行时错误, `2`=用法错误, `3`=配置错误

## 命令优先级

不确定该用哪条命令时按此顺序：

1. **发现**：`lib show --json`（一站式快照）
2. **读优先**：`find` / `show` / `ref related` / `lib stats` / `note list`
3. **PDF 读取**：`pdf text` / `ann list` / `pdf open`
4. **导出**：`item export ITEMKEY`，或先 `find --json` 再 `item export --from PATH|-`
5. **变更次之**：`item new/edit/tag/untag` / `ann new`
6. **删除最后**：仅在用户明确要求时考虑
