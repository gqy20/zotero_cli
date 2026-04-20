---
name: zotero-cli
description: 使用本仓库的本地 Zotero CLI 工具进行文献检索、查看、导出、配置校验和安全写操作。当需要通过 `zot.exe` 或 `go run .\\cmd\\zot` 操作 Zotero 数据时使用，适用于 `find`、`show`、`export`、stats、元数据查询、批量标签和受保护的写/删工作流。
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
.\zot.exe stats --json
.\zot.exe find --all --json
.\zot.exe find "query" --json
.\zot.exe show ITEMKEY --json
.\zot.exe export --collection COLLKEY --format csljson --json
.\zot.exe annotations ITEMKEY --json          # 读取 PDF 标注（双源）
.\zot.exe select ITEMKEY                     # 跳转到 Zotero UI 选中条目
```

利用 `find` 的过滤能力减少额外请求：

- `--date-after YYYY[-MM[-DD]]`
- `--date-before YYYY[-MM[-DD]]`
- 多次使用 `--tag`
- `--tag-any`
- `--include-trashed`
- `--qmode everything`

文本模式辅助选项：

- `--include-fields url,doi,version`
- `--full`

## 写操作安全

以下命令属于**写操作**：

- `create-item` / `update-item`
- `create-items` / `update-items`
- `add-tag` / `remove-tag`
- `create-collection` / `update-collection`
- `create-search` / `update-search`
- `annotate`（向 PDF 文件写入高亮/笔记）

以下命令属于 **PDF 操作**（需 local 模式 + PyMuPDF）：

- `extract-text` — 提取 PDF 文本
- `annotate` — 向 PDF 写入标注
- `annotations` — 读取/删除 PDF 文件内标注
- `open` — 在 Zotero 阅读器中打开 PDF

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
.\zot.exe config init       # 初始化配置
.\zot.exe config show       # 查看当前配置
.\zot.exe config validate   # 校验配置有效性
```

配置缺失时，主动初始化而不是绕过错误。

## 参考文档

按需查阅：

- `docs/AI_AGENT.md` — Agent 使用模式与安全规范
- `README.md` — 用户快速开始与功能概览
