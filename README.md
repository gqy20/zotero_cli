# zot

当前版本：`0.0.1`

一个面向终端、脚本和 AI agent 的 Zotero CLI。

`zot` 现在已经覆盖了较完整的 Zotero Web API 读能力，提供了受权限开关保护的常见写操作，并支持真实 Zotero 本地库上的 `local` / `hybrid` 读路径。它适合这些场景：

- 在终端里快速检索、查看和导出 Zotero 条目
- 在本地库里离线查看条目、附件、笔记和显式关系
- 给脚本或 agent 提供稳定的 `--json` 输出
- 批量给条目打标签、更新、删除
- 做基础的库统计、版本同步和配置校验

## 0.0.1 范围

`0.0.1` 关注的是“准确读取 Zotero 信息并稳定暴露给人和自动化系统”，当前已实现：

- `web`、`local`、`hybrid` 三种读模式
- `find` / `show` / `relate` 的稳定 JSON 输出
- 本地 `show` 的 collections、attachments、notes、路径解析
- 本地 `find` 的 metadata 查询、tag/date 过滤、排序和分页
- 基于本地 SQLite `itemRelations` 的显式关系读取
- 受权限开关保护的常见写操作

## 快速开始

### 构建

```powershell
git clone https://github.com/gqy20/zotero_cli.git
cd zotero_cli
go build -o zot.exe .\cmd\zot
```

### 初始化配置

推荐直接运行交互式初始化：

```powershell
.\zot.exe config init
```

它会引导你填写并写入 `~/.zot/.env`：

- `library_type`
- `library_id`
- `api_key`
- `style`
- `locale`
- 是否允许写操作
- 是否允许删除操作

初始化过程中会打印这些 Zotero 页面，方便直接打开：

- API keys: `https://www.zotero.org/settings/keys`
- Group IDs: `https://www.zotero.org/groups`
- Web API basics: `https://www.zotero.org/support/dev/web_api/v3/basics`

也可以手工写 `~/.zot/.env`。

Web 模式：

```env
ZOT_MODE=web
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
ZOT_API_KEY=replace-me
ZOT_STYLE=apa
ZOT_LOCALE=en-US
ZOT_TIMEOUT_SECONDS=20
ZOT_RETRY_MAX_ATTEMPTS=3
ZOT_RETRY_BASE_DELAY_MS=250
ZOT_ALLOW_WRITE=1
ZOT_ALLOW_DELETE=0
```

Hybrid 模式：

```env
ZOT_MODE=hybrid
ZOT_DATA_DIR=D:\zotero\zotero_file
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
ZOT_API_KEY=replace-me
```

Local 模式：

```env
ZOT_MODE=local
ZOT_DATA_DIR=D:\zotero\zotero_file
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
```

默认安全策略：

- `ZOT_ALLOW_WRITE=1`
- `ZOT_ALLOW_DELETE=0`

也就是说，创建和更新默认允许，删除默认禁止。

### 验证配置

```powershell
.\zot.exe config show
.\zot.exe config validate
```

## 常用命令

### 检索

```powershell
.\zot.exe find "hybrid speciation"
.\zot.exe find --all --json
.\zot.exe find "" --json
.\zot.exe find "hybrid speciation" --date-after 2020
.\zot.exe find "hybrid speciation" --date-after 2020-01 --date-before 2024-12-31
.\zot.exe find "hybrid speciation" --tag "物种形成" --tag "经典案例"
.\zot.exe find "hybrid speciation" --tag "物种形成" --tag "经典案例" --tag-any
.\zot.exe find "hybrid speciation" --json
.\zot.exe find "hybrid speciation" --include-fields url,doi,version
.\zot.exe find "hybrid speciation" --full
```

`find` 当前支持：

- `--all`
- 显式空查询 `find ""`
- `--item-type`
- 重复 `--tag`
- `--tag-any`
- `--date-after`
- `--date-before`
- `--qmode`
- `--include-trashed`
- `--include-fields`
- `--full`
- `--json`

模式说明：

- `web`：完整支持 Web API 查询语义
- `local`：支持 metadata 查询、`--item-type`、`--tag`、`--tag-any`、日期过滤、排序、分页
- `hybrid`：优先走 local；本地不支持的个别场景才回退 Web

### 查看与导出

```powershell
.\zot.exe show SA6DHVIM
.\zot.exe show SA6DHVIM --json
.\zot.exe relate SA6DHVIM
.\zot.exe relate SA6DHVIM --json
.\zot.exe cite SA6DHVIM
.\zot.exe cite SA6DHVIM --format bib
.\zot.exe export --item-key SA6DHVIM --format bibtex
.\zot.exe export "hybrid speciation" --format ris
.\zot.exe export --collection COLL1234 --format csljson --json
```

`export` 当前支持：

- 按 query 导出
- 按单个 `--item-key` 导出
- 按 `--collection` 导出
- `bib`
- `bibtex`
- `biblatex`
- `csljson`
- `ris`

`show` 当前支持：

- Web / local / hybrid 统一条目详情接口
- 文本模式显示 creators、tags、collections、attachments、notes
- 本地模式显示附件 `zotero_path`、`resolved_path`、`resolved`
- 对重复附件名显示附件 key，便于精确定位

`relate` 当前支持：

- 本地 SQLite `itemRelations` 显式关系读取
- 当前会返回 `predicate`、`direction`、目标条目简要信息
- `hybrid` 下优先使用本地关系
- `web` 模式暂不支持

### 列表和元数据

```powershell
.\zot.exe collections --json
.\zot.exe collections-top --json
.\zot.exe notes --json
.\zot.exe tags --json
.\zot.exe searches --json
.\zot.exe trash --json
.\zot.exe publications --json
.\zot.exe deleted --json
.\zot.exe stats --json
.\zot.exe versions items --since 0 --json
.\zot.exe item-types --json
.\zot.exe item-fields --json
.\zot.exe creator-fields --json
.\zot.exe item-template journalArticle --json
.\zot.exe key-info YOUR_API_KEY --json
.\zot.exe groups --json
```

### 写操作

```powershell
.\zot.exe create-item --from-file item.json --if-unmodified-since-version 41
.\zot.exe update-item ABCD2345 --from-file patch.json --if-unmodified-since-version 42
.\zot.exe create-items --from-file items.json --if-unmodified-since-version 41 --json
.\zot.exe update-items --from-file patches.json --json
.\zot.exe add-tag --items KEY1,KEY2 --tag "to-read"
.\zot.exe remove-tag --items KEY1,KEY2 --tag "obsolete"
```

删除命令默认被权限开关拦住；只有在 `ZOT_ALLOW_DELETE=1` 时才允许执行：

```powershell
.\zot.exe delete-item ABCD2345 --if-unmodified-since-version 43
.\zot.exe delete-items --items KEY1,KEY2 --if-unmodified-since-version 43
.\zot.exe delete-collection COLL1234 --if-unmodified-since-version 10
.\zot.exe delete-search SRCH1234 --if-unmodified-since-version 10
```

## AI / Agent 使用建议

- 默认优先加 `--json`
- 当需要精确文献关系时，优先用 `relate --json`
- 当需要 DOI、URL、版本号时，优先用 `find --json`，必要时加 `--include-fields` 或 `--full`
- 批量导出收藏夹优先用 `export --collection`
- 在首次执行前先跑 `config validate`
- 删除前先确认：
  - library
  - object key
  - version precondition
  - 用户是否明确授权

更完整的 agent 说明见：

- [AI Agent Guide](/d:/C/Documents/Program/Go/zotero_cli/docs/AI_AGENT.md)
- 仓库内 skill: [.codex/skills/zotero-cli/SKILL.md](/d:/C/Documents/Program/Go/zotero_cli/.codex/skills/zotero-cli/SKILL.md)

## 开发

```powershell
go test ./...
go build -o zot.exe .\cmd\zot
.\zot.exe version
```

## 文档

- [AI Agent Guide](/d:/C/Documents/Program/Go/zotero_cli/docs/AI_AGENT.md)
- [Repo Skill](/d:/C/Documents/Program/Go/zotero_cli/.codex/skills/zotero-cli/SKILL.md)
- [MVP 设计文档](/d:/C/Documents/Program/Go/zotero_cli/docs/MVP.md)
- [Rate Limit Optimization Notes](/d:/C/Documents/Program/Go/zotero_cli/docs/rate-limit-optimization.md)
## 0.0.2 Stability Pass Update

Recent `0.0.2` stability work focuses on making mode behavior and CLI failures
more predictable rather than adding a large new command surface.

Completed in this pass:

- `hybrid` read flows now normalize remote fallback through the Web API client
- write commands now return clearer argument-validation errors before usage text
- current mode boundaries are documented explicitly for `web`, `local`, and `hybrid`

Current mode summary:

- backend-aware commands: `find`, `show`, `relate`
- Web API commands: `cite`, `export`, list/meta commands, and all write commands
- `local` mode rejects Web API-only commands with an explicit mode-boundary error
- `hybrid` mode can still use remote API paths when local support is unavailable

For the detailed design notes, see:

- [0.0.2 Stability Pass](/D:/C/Documents/Program/Go/zotero_cli/docs/stability-pass-0.0.2.md)
- [Local Backend Design](/D:/C/Documents/Program/Go/zotero_cli/docs/local-backend-design.md)
