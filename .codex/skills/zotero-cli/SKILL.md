---
name: zotero-cli
description: 使用本仓库的本地 Zotero CLI 工具进行文献检索、查看、导出、配置校验和安全写操作。当需要通过 `zot.exe` 或 `go run .\\cmd\\zot` 操作 Zotero 数据时使用，适用于 `find`、`show`、`export`、stats、元数据查询、批量标签、PDF 操作和受保护的写/删工作流。
---

# Zotero CLI

优先使用本地 CLI，不要自行实现 Zotero API 调用。

## 工作流程

1. 在项目根目录下工作。
2. 优先使用 `.\zot.exe`（如果二进制文件存在且版本足够）。
3. 验证源码变更或二进制缺失时回退到 `go run .\cmd\zot ...`。
4. Agent 工作流优先使用 `--json`。
5. 假设凭据可用前先运行 `config validate`。

## 读优先默认命令

```powershell
.\zot.exe overview --json                   # 一站式库概览（Agent 入口推荐）
.\zot.exe stats --json
.\zot.exe find --all --json
.\zot.exe find "query" --json
.\zot.exe show ITEMKEY --json
.\zot.exe export --collection COLLKEY --format csljson --json
.\zot.exe annotations ITEMKEY --json          # 读取 PDF 标注（双源）
.\zot.exe select ITEMKEY                     # 跳转到 Zotero UI 选中条目
```

利用 `find` 的过滤能力减少额外请求：

**基础过滤：**
- `--date-after YYYY[-MM[-DD]]`
- `--date-before YYYY[-MM[-DD]]`
- 多次使用 `--tag`（AND） / `--tag-any`（OR）
- `--include-trashed`（web only）
- `--qmode everything`（web only）

**高级过滤（local / hybrid）：**
- `--no-type TYPE` — 排除某文献类型
- `--tag-contains WORD` — 标签模糊匹配
- `--exclude-tag TAG` — 排除含某标签
- `--collection KEY` — 按收藏夹过滤
- `--no-collection KEY` — 排除收藏夹
- `--modified-within DURATION` — 最近修改（如 `7d`、`2w`）
- `--added-since DURATION` — 最近添加
- `--has-pdf` — 仅有 PDF 附件的条目
- `--attachment-name TEXT` — 附件文件名匹配
- `--attachment-path TEXT` — 附件路径匹配

**输出控制：**
- `--include-fields url,doi,version` — 指定返回字段
- `--full` — 完整字段 + 附件详情
- `--sort FIELD` + `--direction asc|desc` — 排序
- `--start N` + `--limit N` — 分页

**全文检索：**
- `--fulltext` — FTS5 全文搜索（FTS 有数据时自动启用）
- `--fulltext-any` — 任一词匹配
- `--snippet` — 匹配片段预览（默认限制 **50** 条，需更多时加 `--limit N`）

文本模式辅助选项：

- `--include-fields url,doi,version`
- `--full`

## PDF 操作（需 local 模式 + PyMuPDF）

```powershell
# 提取 PDF 正文（PyMuPDF → ft-cache → pdfium WASM）
.\zot.exe extract-text ITEMKEY --json

# 双源读取标注
.\zot.exe annotations ITEMKEY --json
.\zot.exe annotations ITEMKEY --type highlight --page 3 --json
# 删除 PDF 文件内标注
.\zot.exe annotations ITEMKEY --clear --type highlight

# 写入标注到 PDF
.\zot.exe annotate ITEMKEY --text "关键概念" --color red --comment "重要"
.\zot.exe annotate ITEMKEY --text "speciation" --type underline --color blue

# 与 Zotero 桌面端联动
.\zot.exe open ITEMKEY --page 5        # 阅读器中打开 PDF
.\zot.exe select ITEMKEY               # 主界面选中条目
```

## 笔记查询

```powershell
.\zot.exe notes --json
.\zot.exe notes --query "CRISPR" --limit 20 --json
```

## 写操作安全

以下命令属于**写操作**：

- `create-item` / `update-item`
- `create-items` / `update-items`
- `add-tag` / `remove-tag`
- `create-collection` / `update-collection`
- `create-search` / `update-search`
- `annotate`（向 PDF 文件写入高亮/笔记）

以下命令属于**破坏性操作**：

- `delete-item` / `delete-items`
- `delete-collection` / `delete-search`

执行任何写操作前：

1. 确认用户确实要修改数据。
2. 检查 `ZOT_ALLOW_WRITE` 和 `ZOT_ALLOW_DELETE` 是否允许该操作。
3. 尽可能使用版本前置条件。

执行任何删除操作前：

1. 复述目标 key 或 keys。
2. 确认无歧义。
3. 请求有任何不确定就先询问用户。

## 配置

CLI 配置存储在 `~/.zot/.env`。

常用命令：

```powershell
.\zot.exe init                    # 一键初始化（推荐，含模式选择和可选 PyMuPDF 安装）
.\zot.exe init --mode hybrid --api-key ...  # 非交互模式
.\zot.exe config show       # 查看当前配置
.\zot.exe config validate   # 校验配置有效性
```

配置缺失时，主动初始化而不是绕过错误。

### 环境变量速查

| 变量 | 说明 | 默认 |
|------|------|------|
| `ZOT_MODE` | `web` / `local` / `hybrid` | `web` |
| `ZOT_DATA_DIR` | Zotero 数据目录 | — |
| `ZOT_LIBRARY_ID` | 库 ID | — |
| `ZOT_API_KEY` | API 密钥 | — |
| `ZOT_STYLE` | 引文样式 | `apa` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_ALLOW_WRITE` | 允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 允许删除操作 | `0` |
| `ZOT_RETRY_MAX_ATTEMPTS` | 最大重试次数 | `5` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟 ms | `1000` |
| `ZOT_JSON_ERRORS` | 错误以 JSON 输出到 stdout（agent 解析用） | `0` |

## 性能注意

- `--snippet` 默认 limit 50，需要更多结果显式加 `--limit`
- local/hybrid 下 FTS 有数据时自动启用全文检索，无需手动加 `--fulltext`
- `extract-text` 结果有缓存，重复提取同一 PDF 直接命中
- 高频脚本遇 `429` 会自动退避+抖动，但仍应主动降速

## 参考文档

按需查阅：

- `docs/AI_AGENT.md` — Agent 使用模式与安全规范（完整版）
- `docs/commands.md` — 完整命令参考与所有选项说明
- `README.md` — 用户快速开始与功能概览
