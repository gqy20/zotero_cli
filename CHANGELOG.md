# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.0.1`。

## [Unreleased]

### 后续改进计划（Agent 可用性增强）
以下为规划中的改进方向，按优先级排序：
1. **结构化错误输出**：支持 `--json` / `ZOT_JSON_ERRORS=1` 让错误以 `{ "ok": false, "error": "...", "code": N }` JSON 格式输出，便于 agent 可靠解析。
2. **`zot overview` 发现命令**：一次性返回 library 全貌（总条目数、热门标签、集合树、最近添加、索引状态），降低 agent 使用门槛。
3. **写操作 `--dry-run` 模式**：所有写命令支持预览将要执行的操作而不实际修改数据，提升安全性。
4. **`find` → `export` 管道连接**：`export` 新增 `--from-find` 参数，内部执行搜索后直接导出，无需手动传递 key 列表。

## [0.0.4] - 2026-04-20

### 新增
- 新增 `zot annotate` 命令，支持通过 PyMuPDF 向 PDF 写入高亮、下划线和笔记标注。支持三种定位模式：文本搜索（全页）、矩形坐标、点位便签。
- 新增 `zot open` 命令，在 Zotero 阅读器中打开 PDF 附件。Zotero 运行时通过 `zotero://open-pdf` 协议复用已有实例并支持页码跳转；未运行时启动新实例。
- 新增 `zot select` 命令，通过 `zotero://select` 协议在已运行的 Zotero UI 中选中指定条目。
- 新增 `zot annotations` 命令，双源读取 PDF 标注：Zotero Reader 数据库标注（含 dateAdded 时间戳）+ PDF 文件内标注（PyMuPDF 扫描）。支持按页码/类型过滤、JSON 输出、以及 `--clear` 删除 PDF 文件内的标注。
- `domain.Annotation` 类型新增 `DateAdded` 字段，SQL 查询增加 `dateAdded` 列。
- 新增 Makefile 构建系统，支持 `make build` / `make release` / `make check` / `make fmt` 等目标。release 目标自动下载 UPX 并压缩 Windows 二进制至 ~6MB。
- 新增 pre-commit hook（gofmt + go vet + go test），通过 `make install-hooks` 安装。
- 新增 Exit Code 规范常量（ExitOK/ExitError/ExitUsage/ExitConfig），统一所有命令的退出码语义。
- 新增 `docs/examples/` 目录，包含 8 个命令的完整 JSON 输出示例，供 AI Agent 参考数据结构。
- 新增 `docs/architecture.md` 技术架构文档和 `docs/commands.md` 完整命令参考。
- 新增 `CONTRIBUTING.md` 贡献指南及 GitHub PR/Issue 模板。
- 新增 `.claude/skills/zotero-cli/SKILL.md` Claude Code skill 文件（中文版）。

### 变更
- README 重构为 AI 原生产品首页：按科研工作流组织内容、新增功能对照表、与 Zotero 桌面端联动说明、多平台安装方式（含 Homebrew）。
- SKILL.md 文件全部改为中文，与项目文档语言一致。
- CI workflow 改为使用 make 目标（fmt-check / vet / test / build）；release workflow 新增 UPX 压缩步骤。
- `zot open` 改进：检测 Zotero 是否运行，运行中用 `zotero://open-pdf` 协议（传附件 key 而非父条目 key），未运行时启动新实例。`--page` 参数现在真正生效（通过 URI query 参数传递）。
- 构建流程优化：`make build` / `make release` 在构建前自动清理旧产物；UPX 压缩直接覆盖为最终 `zot.exe`（通过临时文件中转）；CI workflow 同步更新。

## [0.0.3] - 2026-04-17

### 新增
- 新增 `extract-text` 命令，可在 `local` / `hybrid` 模式下提取本地 PDF 正文。
- `extract-text --json` 现在会返回主附件文本、所有 PDF 附件文本、缓存命中状态和全文来源元信息。
- `show` 的本地输出现在会加载并展示 Zotero Reader 的 PDF 注释与高亮数据。
- 本地 `find` 现在支持附件感知过滤，包括 `--has-pdf`、`--attachment-type`、附件路径/名称相关匹配，以及更明确的 `matched_on` 信号。
- 本地全文检索进一步扩展，支持 snippet 预览、附件感知片段、实验性 FTS 索引查询和更丰富的全文元信息。
- 新增 PDF 处理研究文档，记录全文提取与渲染路线的实现背景。

### 变更
- `hybrid` 模式下的本地读 fallback 与 `read_source` 元数据进一步稳定化，本地缺失、暂时不可用和能力边界现在会给出更一致的信号。
- `find` 的共享语义进一步收敛，统一了查询参数规范化、标签去重归一化、日期过滤和默认可见条目策略。
- `hybrid` 模式的 fallback 现在不仅看错误类型，还会看 Web 是否真的能够承接该请求，避免 local-only 查询被误退回到 Web。
- `relate` 在 `hybrid` 下不再误回退到 `web`，本地关系读取失败时会保留真实本地错误。
- `export --format csljson` 在 `local` / `hybrid` 下优先使用本地导出；`hybrid` 只会在可预期的本地缺失或暂时不可用场景下回退到 Web。
- PDF 全文提取优先级调整为更偏向主 PDF；正文归一化、去重、补空格和多附件返回行为也进一步改进。
- CLI 内部结构完成一轮较大整理，包括命令方法化、依赖注入收敛、局部工具函数清理，以及移除旧的兼容入口。
- 命令帮助、字段选择、错误输出和 agent 友好型元信息继续增强，便于脚本和自动化工具消费。
- CLI help 现在补充了 modes 和 environment 说明，GitHub release 工作流的展示也做了整理。

### 文档
- `README.md` 现在明确记录了 `find`、`relate`、`extract-text` 和 `csljson export` 在 `web` / `local` / `hybrid` 下的能力边界与回退规则。
- `docs/AI_AGENT.md` 更新了 agent 调用建议，补充了 local-only 能力与 `hybrid` 回退约束。
- 新增 `docs/roadmap-0.0.3.md`，记录语义一致性与 fallback 稳定性的推荐推进顺序。
- 使用 `CHANGELOG.md` 驱动 GitHub Release 发布说明。

## [0.0.2] - 2026-04-01

### 变更
- `hybrid` 模式下的远程 fallback 现在统一走归一化后的 Web client 路径，在本地库不可用或不支持某项能力时，读命令仍能稳定回退到 Zotero Web API。
- 写命令参数校验现在会更明确地区分缺失前置版本、`--data` 与 `--from-file` 冲突、输入文件不可读和 JSON 无效等错误。
- `local` 模式下访问仅支持 Web API 的命令时，现在会返回清晰的模式边界错误，而不是泛化的 unsupported mode 失败。
- `trash`、`collections-top`、`publications` 这些只读列表命令不再错误要求写入或删除权限。
- `collections`、`tags`、`searches`、`groups`、`trash`、`collections-top`、`publications` 在文本模式下遇到空结果时，现在会输出明确提示，而不是静默返回空白。
- `config validate --json` 现在会返回额外的诊断元信息，包括配置路径、当前模式、是否配置了 `data_dir`，以及 local reader 是否可用。

### 文档
- 记录了 `0.0.2` 稳定性整理的阶段性结果，并补充了当前模式边界说明。

## [0.0.1] - 2026-03-31

### 新增
- 首次公开发布 `zot` 命令行工具。
- 提供 Linux、macOS 和 Windows 的跨平台发布产物。

### 变更
- 发布压缩包现在会包含项目 `LICENSE`。
- 发布流程会在进入构建矩阵前统一执行一次测试。
- 发布二进制现在会注入稳定的 UTC 构建时间。
