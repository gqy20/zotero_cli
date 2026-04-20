# AI Agent Guide

这份文档面向会调用 `zot` 的 AI agent 或自动化脚本。

如果你的运行环境支持仓库内 skill，也可以直接参考：

- [.codex/skills/zotero-cli/SKILL.md](/d:/C/Documents/Program/Go/zotero_cli/.codex/skills/zotero-cli/SKILL.md)

## 首次使用

先做三步：

```powershell
.\zot.exe config init
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
- `delete-items`
- `delete-collection`
- `delete-search`

执行前应该：

1. 复述目标对象
2. 检查 key 是否正确
3. 检查 version precondition
4. 如有歧义，先问用户

## 常见工作流

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
  - 运行 `zot config init`

## 优先级建议

如果你是一个通用 agent，不确定该用哪条命令：

1. 读优先：`find` / `show` / `relate` / `stats`
2. 导出优先：`export --collection` 或 `export --item-key`
3. 变更次之：`create-*` / `update-*`
4. 删除最后：只有在用户明确要求时才考虑
