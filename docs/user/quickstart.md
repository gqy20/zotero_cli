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

`--json` 已返回完整 Item 结构；`--include-fields` 和 `--full` 主要增强文本模式。

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
.\zot.exe find --all --sort dateAdded --order desc --limit 10 --json

# 只看最近 7 天入库
.\zot.exe find --all --added-since 7d --sort dateAdded --order desc --json

# 最近修改元数据的条目
.\zot.exe find --all --modified-within 7d --sort dateModified --order desc --json
```

快速人工浏览标题时，可用文本模式控制字段：

```powershell
.\zot.exe find --all --sort dateAdded --order desc --limit 10 --include-fields title,date_added,container
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
.\zot.exe pdf text ITEMKEY --json
.\zot.exe pdf text ITEMKEY --json --pages 3-8 --grep methods --max-chars 12000

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

# 写入标注到 PDF
.\zot.exe ann new ITEMKEY --text "关键概念" --color red --comment "重要"

# remote 模式下以上两条也可用，但实际执行发生在 zot server 所在机器

# 在 Zotero 阅读器中打开
.\zot.exe pdf open ITEMKEY --page 5
```

### 批量操作

```powershell
.\zot.exe item tag KEY1 KEY2 --tag "to-read" --json
.\zot.exe item export --collection COLL1234 --as csljson --json

# 正则批量改名：默认预览，加 --yes 才写入
.\zot.exe tag replace --match '^(Gene Flow|Gene flow|gene flow)$' --replace 'Gene Flow'
.\zot.exe tag replace --match '^植物/(.+)$' --replace '物种/植物/$1' --yes
```

### 全文检索最佳实践

```powershell
# local / hybrid 模式下，有 query 且 FTS5 有数据时自动启用全文检索
.\zot.exe find "同源多倍体" --snippet --json
# snippet 默认限制 50 条，需要更多结果时显式指定 --limit
.\zot.exe find "基因编辑" --snippet --limit 200 --json
```

`find --all` 或纯时间/标签列表不会自动走全文索引，适合最近入库、发表时间范围等元数据查询。

## 性能优化建议

| 建议 | 说明 |
|------|------|
| **`--snippet` 默认限 50 条** | 保护批量提取性能 |
| **自动全文检索** | local/hybrid 下有 query 且 FTS5 有数据时可自动走全文路径；`--all` 不自动走全文 |
| **`--include-fields` 控制文本输出** | 快速人工浏览时只展示指定字段；`--json` 默认返回完整 Item |
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

### 结构化错误输出

设置 `ZOT_JSON_ERRORS=1` 后所有错误以 JSON 输出到 stdout：

```json
{"ok": false, "command": "show", "data": "item not found: ABCD", "code": 1}
```

- `code`: `1`=运行时错误, `2`=用法错误, `3`=配置错误

## 命令优先级

不确定该用哪条命令时按此顺序：

1. **发现**：`lib show --json`（一站式快照）
2. **读优先**：`find` / `show` / `ref related` / `lib stats` / `note list`
3. **PDF 读取**：`pdf text` / `ann list` / `pdf open`
4. **导出**：`item export --collection COLLKEY` 或 `item export ITEMKEY`
5. **变更次之**：`item new/edit/tag/untag` / `ann new`
6. **删除最后**：仅在用户明确要求时考虑
