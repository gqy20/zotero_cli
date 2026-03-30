# zot

一个面向终端与 AI 工作流的 Zotero CLI。

当前版本先把最常用的几件事做好：

- 在命令行里搜索文献
- 查看单条文献详情和附件
- 生成单条引用
- 导出 bibliography 文本
- 查看 collections 和 notes
- 提供稳定的 `--json` 输出，方便脚本和 AI 调用

项目还在 MVP 阶段，但核心工作流已经可以使用。

## 适合谁用

如果你有下面这些需求，这个工具会比较合适：

- 已经在用 Zotero 管理论文
- 想在终端里快速查文献，而不是频繁切回 Zotero 窗口
- 想把 Zotero 接入脚本、自动化流程、编辑器或者 AI agent

## 现在能做什么

当前已经可用的命令有：

- `config show`：查看当前生效配置
- `find`：搜索文献
- `show`：查看单条文献详情
- `cite`：生成单条引用
- `export`：导出 bibliography
- `collections`：列出文献夹
- `notes`：列出笔记

已经验证过的真实工作流：

1. 用 `find` 搜索文献
2. 从结果里拿到条目 `key`
3. 用 `show` 查看详情和附件
4. 用 `cite` 生成单条引用
5. 用 `export` 导出 bibliography

## 快速开始

### 1. 克隆并编译

```powershell
git clone https://github.com/gqy20/zotero_cli.git
cd zotero_cli
go build -o zot.exe .\cmd\zot
```

编译完成后，就可以直接使用：

```powershell
.\zot.exe version
.\zot.exe find 2024
.\zot.exe show SA6DHVIM --json
```

### 2. 在项目根目录创建 `.env`

```env
ZOT_MODE=web
ZOT_LIBRARY_TYPE=user
ZOT_LIBRARY_ID=123456
ZOT_API_KEY=replace-me
```

最少需要这几个字段：

- `ZOT_LIBRARY_TYPE`
- `ZOT_LIBRARY_ID`
- `ZOT_API_KEY`

说明：

- `.env` 已加入 Git 忽略，不会默认提交
- 环境变量会覆盖配置文件中的同名字段
- 如果你更喜欢配置文件，也可以用 `zot config init`

### 3. 验证配置

```powershell
.\zot.exe config show
```

如果输出里能看到掩码后的 `api_key`、`library_id` 和 `library_type`，说明配置已经读取成功。

## 常用命令

### 查看版本

```powershell
.\zot.exe version
```

### 搜索文献

```powershell
.\zot.exe find 2024
.\zot.exe find "species formation"
.\zot.exe find 2024 --item-type journalArticle --limit 5
.\zot.exe find 2024 --json
```

当前 `find` 的行为：

- 默认过滤 `attachment` 和 `note`
- 支持 `--item-type`
- 支持 `--limit`
- 支持 `--json`

### 查看单条文献详情

```powershell
.\zot.exe show SA6DHVIM
.\zot.exe show SA6DHVIM --json
```

当前 `show` 会展示这些信息：

- 标题
- 条目类型
- 作者
- 日期
- 期刊或容器信息
- DOI
- URL
- tags
- 子附件信息

### 生成引用

```powershell
.\zot.exe cite SA6DHVIM
.\zot.exe cite SA6DHVIM --format bib
.\zot.exe cite SA6DHVIM --json
```

当前 `cite` 的行为：

- 默认输出单条 citation
- 支持 `--format citation|bib`
- 支持 `--style`
- 支持 `--locale`
- 支持 `--json`

### 导出 bibliography

```powershell
.\zot.exe export --item-key SA6DHVIM
.\zot.exe export mixed --limit 1
.\zot.exe export --item-key SA6DHVIM --json
```

当前 `export` 的行为：

- 支持按 `--item-key` 导出单条 bibliography
- 支持按查询结果批量导出 bibliography
- 支持 `--limit`
- 支持 `--json`

### 查看 collections

```powershell
.\zot.exe collections
.\zot.exe collections --json
```

当前会返回文献夹的：

- `key`
- `name`
- `parent_key`
- `num_collections`
- `num_items`

### 查看 notes

```powershell
.\zot.exe notes
.\zot.exe notes --json
```

当前 `notes` 的行为：

- `--json` 返回完整笔记数据
- 默认文本模式会尽量隐藏明显的机器笔记
- 如果当前库里只有机器笔记，文本模式会给出提示，避免把日志样内容直接铺满终端

## 推荐使用方式

目前最顺手的使用方式是：

```powershell
.\zot.exe find 2024
.\zot.exe show SA6DHVIM --json
.\zot.exe cite SA6DHVIM
.\zot.exe export --item-key SA6DHVIM
```

也就是：

1. 先用 `find` 找到条目
2. 再用返回的 `key` 调 `show`
3. 需要单条引用时用 `cite`
4. 需要 bibliography 时用 `export`

## 当前限制

当前版本还没有这些能力：

- BibTeX / BibLaTeX 导出
- 本地全文索引
- 写操作
- MCP server
- 更细的 notes / annotations 支持

当前后端仍然是：

- Zotero Web API

## 开发

如果你是在开发这个项目，而不是日常使用，更适合用 `go run`：

```powershell
go run .\cmd\zot version
go run .\cmd\zot find 2024
```

运行测试：

```powershell
go test ./...
```

重新编译：

```powershell
go build -o zot.exe .\cmd\zot
```

## 文档

更细的技术和工程说明放在 `docs` 目录：

- [MVP 设计文档](/D:/C/Documents/Program/Go/zotero_cli/docs/MVP.md)
- [GitHub Actions 说明](/D:/C/Documents/Program/Go/zotero_cli/docs/github-actions.md)

## 下一步

当前更可能优先推进的方向：

- 继续完善 `export` 的格式支持
- 继续打磨 `collections` / `notes` 的可用性
- 稳定 JSON schema
- 继续优化错误提示和终端体验
