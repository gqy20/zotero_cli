# AI Agent 快速上手指南

面向会调用 `zot` 的 AI agent 或自动化脚本。如果你是人工终端用户，请参考 [命令参考](./commands.md)。

如果你的运行环境支持仓库内 skill，也可以直接参考：

- [.codex/skills/zotero-cli/SKILL.md](../../.codex/skills/zotero-cli/SKILL.md)

---

## 首次使用（三步走）

```powershell
.\zot.exe init                 # 一键初始化（交互式：模式、API key、库 ID，可选 PyMuPDF）
.\zot.exe config validate     # 校验配置
.\zot.exe stats --json          # 验证连通性
```

如果在仓库里开发：

```powershell
go run .\cmd\zot config validate
```

## 推荐调用习惯

### 1. 默认使用 JSON

```powershell
.\zot.exe find "hybrid speciation" --json
.\zot.exe show SA6DHVIM --json
.\zot.exe stats --json
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

日期支持 `YYYY` / `YYYY-MM` / `YYYY-MM-DD`。

### 5. 本地与混合模式

推荐设 `ZOT_MODE=hybrid`：

- `local`：只读本地 SQLite + `storage/`
- `hybrid`：优先本地，Web 仅在能承接时回退
- `relate` / `extract-text` 在 hybrid 下仍是本地能力

详见 [架构文档 - 三种模式](../architecture/overview.md#三种模式)。

## 安全规则

| 规则 | 说明 |
|------|------|
| **删除默认禁止** | `ZOT_ALLOW_DELETE=0` 时所有 delete 命令失败，这是预期行为 |
| **写操作前确认** | 先 `config validate`，再检查权限开关和用户意图 |
| **删除需谨慎** | `delete-item` / `delete-collection` / `delete-search` 属高风险操作 |

执行写操作前建议：
1. 复述目标对象
2. 检查 key 是否正确
3. 检查 version precondition
4. 如有歧义，先问用户

## 常见工作流

### Agent 入口：一站式库概览

```powershell
.\zot.exe overview --json   # 统计 + 收藏夹 + 标签 + 最近条目 + 索引状态
```

返回 `data.stats` / `data.collections` / `data.tags` / `data.recent_items`，适合作为首次连接时的发现命令。

### 查找并查看详情

```powershell
.\zot.exe find "attention is all you need" --json
.\zot.exe show X42A7DEE --json
.\zot.exe relate X42A7DEE --json
```

### PDF 操作

```powershell
# 提取文本（PyMuPDF → ft-cache → pdfium WASM）
.\zot.exe extract-text ITEMKEY --json

# 双源读取标注（DB + PDF 文件内）
.\zot.exe annotations ITEMKEY --json

# 写入标注到 PDF
.\zot.exe annotate ITEMKEY --text "关键概念" --color red --comment "重要"

# 在 Zotero 阅读器中打开
.\zot.exe open ITEMKEY --page 5
```

### 批量操作

```powershell
.\zot.exe add-tag --items KEY1,KEY2 --tag "to-read" --json
.\zot.exe export --collection COLL1234 --format csljson --json
```

### 全文检索最佳实践

```powershell
# local / hybrid 模式下 FTS5 有数据时自动启用全文检索
.\zot.exe find "同源多倍体" --snippet --json
# snippet 默认限制 50 条，需要更多结果时显式指定 --limit
.\zot.exe find "基因编辑" --snippet --limit 200 --json
```

## 性能优化建议

| 建议 | 说明 |
|------|------|
| **`--snippet` 默认限 50 条** | 保护批量提取性能 |
| **自动全文检索** | local/hybrid 下 FTS5 有数据时即使不加 `--fulltext` 也走全文路径 |
| **`--include-fields` 减少传输** | 只需特定字段时减少 JSON 体积 |
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
| `401` / `403` | 检查 api_key / library_type / library_id，跑 `config validate` |
| `412` | 库版本已变化，刷新后重试 |
| `429` | CLI 已重试，高频脚本应降速 |
| local temporary unavailable | 保留本地错误，不要强行改走 Web |
| 配置缺失 | 运行 `zot init` |

### 结构化错误输出

设置 `ZOT_JSON_ERRORS=1` 后所有错误以 JSON 输出到 stdout：

```json
{"ok": false, "command": "show", "data": "item not found: ABCD", "code": 1}
```

- `code`: `1`=运行时错误, `2`=用法错误, `3`=配置错误

## 命令优先级

不确定该用哪条命令时按此顺序：

1. **发现**：`overview --json`（一站式快照）
2. **读优先**：`find` / `show` / `relate` / `stats` / `notes`
3. **PDF 读取**：`extract-text` / `annotations` / `open`
4. **导出**：`export --collection` 或 `export --item-key`
5. **变更次之**：`create-*` / `update-*` / `add-tag` / `remove-tag` / `annotate`
6. **删除最后**：仅在用户明确要求时考虑
