# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.0.1`。

## [Unreleased]

### 新增
- **期刊等级查询**：所有读命令（`show`/`find`）自动展示期刊等级信息（SCI-IF、中科院分区、JCI、ESI、各高校认定等级等），数据来自 [EasyScholar](https://www.easyscholar.cc/console/user/open)（需安装[绿青蛙插件](https://www.easyscholar.cc/blogs/10009)），从 `zotero_file/zoterostyle.json` 自动加载。`show` 和 `find --full` 命令在文本输出中显示等级，`find --json` 在 JSON 中包含 `journal_rank` 字段。支持期刊名模糊匹配（缩写、大小写、中英文）。
- **Relate 命令全面增强**：`zot relate` 从仅支持 local/hybrid 查询自身显式关系，升级为覆盖三种模式、三层聚合、读写一体的完整关系管理工具：
  - **Web API 支持**：web 和 hybrid 模式下通过 Zotero Web API v3 的 `data.relations` 字段解析显式关系，替换原有的 `ErrUnsupportedFeature` stub。hybrid fallback 路径修正为允许 `get_related` 回退到 Web。
  - **三层聚合（`--aggregate`）**：返回条目自身关系 + 子笔记的 itemRelations + 笔记内嵌 citation（`data-citation-items`）的完整关系网络。JSON 输出按 self / notes / citations 分层结构化；文本模式分段展示。
  - **Snapshot 一致性保障**：检测快照新鲜度，JSON 输出 meta 中含 `snapshot_stale` 字段，文本模式在过期时输出警告提示。
  - **ItemRef 信息增强**：目标条目从 key/type/title 三字段扩展至包含 date / creators（`;;`分隔的 lastName|||firstName 格式）/ tags 数组。SQL 使用标量子查询避免 creators × tags 笛卡尔积。
  - **笔记内嵌 Citation 解析**：正则提取笔记 HTML 中 URL 编码的 `data-citation-items` 属性，解析 JSON 后从 URI 列表提取 item keys，批量补全 ItemRef 元信息。
  - **关系写入（`--add` / `--remove`）**：local/hybrid 模式下支持添加和删除显式关系（需 `ZOT_ALLOW_WRITE=1`）。Local 模式直接写入 SQLite `itemRelations` 表；自定义谓词支持（默认 `dc:relation`）。`--dry-run` 预览模式无需写权限。
  - **批量操作（`--from-file`）**：JSON 文件驱动批量 add/remove 操作，格式为 `{action, source, target, predicate}` 数组。支持 `--dry-run` 预览。
  - **Graphviz DOT 可视化（`--dot`）**：输出 Graphviz DOT 格式关系网络图。节点颜色编码：根条目蓝色、笔记橙色、目标灰色；边样式编码：实线=显式关系、点线=父子归属、虚线=内嵌 citation。可与 `--aggregate` 组合使用。
  - **Predicate 过滤（`--predicate`）**：按谓词类型筛选关系输出（如 `dc:relation`、`owl:sameAs`），适用于所有模式（查询/聚合/DOT）。
- **PDF Figure 提取（`extract-figures`）**：新增 `zot extract-figures` 命令，基于 PyMuPDF `cluster_drawings()` v5b 算法从 PDF 中提取科学插图。双路径策略：矢量聚类（Path A）+ 位图锚点回退（Path B）。过滤链包含面积/尺寸/锚点检测/文字密度/caption 模式/全页扫描跳过/去重七步，支持 caption 自动吸附。多篇自动并行（WaitGroup + semaphore），JSON/文本双输出，含 page/source/size/anchors/has_caption 等元信息字段。
- **Hybrid 本地笔记创建**：`create-item` 命令在 Zotero 未运行且 mode 为 local/hybrid 时，笔记类型自动走 SQLite 直写路径（~50ms），无需 Web API（~2s）。通过 `isZoteroRunning()` 自动检测进程状态，`generateItemKey()` 生成符合 Zotero 格式的 item key，`CreateLocalNote()` 在事务中写入 items + itemNotes 两张表并继承父条目 libraryID。Web API 作为 fallback 路径保留。JSON 输出含 `"write_source": "local"` 标识来源。
- **删除操作交互确认**：`delete-item` / `delete-collection` / `delete-search` 命令新增交互式确认提示，执行前显示警告信息并要求 `[y/N]` 确认。取消操作退出码 130。新增 `--yes` / `-y` 标志跳过确认（供脚本/自动化使用）；`--json` 模式自动跳过确认。同时修复 `generateItemKey()` 中的 byte shift overflow 问题（go vet 检出）。
- **Find 自动推断 `--all`**：`zot find` 在仅使用实质性过滤标志（`--tag` / `--date-after` / `--collection` / `--has-pdf` 等 14 种）而无查询词时，自动推断为全量搜索，不再强制要求显式 `--all` 或查询字符串。仅在无查询词、无过滤、无 `--all` 三者同时缺失时报错。

### 变更
- **默认模式改为 hybrid**：`config.Default()` 的默认 Mode 从 `web` 改为 `hybrid`；`zot init` 交互式提示的默认值同步更新为 `[hybrid]`。新用户开箱即用即可享受本地优先 + Web 回退的完整能力。
- **Init 安装后提示索引构建**：PyMuPDF 安装完成后额外提示运行 `zot index build` 以提取全文索引。

### 工具链
- **CI UPX 升级**：Release workflow 从 `apt install upx-ucl`（版本过旧）切换为 `crazy-max/ghaction-upx@v3`（自动拉取最新 UPX release）。Makefile 移除本地 `tools/` 下载逻辑，UPX 现由 GitHub Action 系统级安装提供。

## [0.0.7] - 2026-04-22

### 新增
- **标注系统双层删除**：`annotate --clear` 和 `annotations --clear` 现在同时清理 PDF 文件层和 Zotero DB 层（`itemAnnotations` 表）标注。DB 删除在 Zotero 运行时以 warning 形式非阻断处理，关闭 Zotero 后可重试成功。
- **DB 标注删除接口**：`LocalReader` 新增 `DeleteDBAnnotations` 方法，通过正确的三层 SQL JOIN（`items → itemAttachments → itemAnnotations`）定位并删除 DB 标注，支持按页码/类型/作者组合过滤。
- **SQLite 读写 DSN 分离**：新增 `localSQLiteDSNReadWrite()` 函数（`mode=rwc&_pragma=journal_mode=WAL`），与只读 DSN（`mode=ro&_pragma=query_only=1`）分离，解决写操作 `attempt to write a readonly database` 错误。
- **ANNO_TYPES 完整映射**：PyMuPDF 标注类型从 5 种扩展到 20 种完整映射（highlight/underline/strikeout/squiggly/circle/line/polyline/freetext/stamp 等），覆盖 Zotero 支持的全部标注类型。
- **`--author` 过滤**：`annotations` 命令新增 `--author` 参数，支持按标注作者过滤 DB 层标注输出。
- **本地引文格式化**：`cite` 命令在 local/hybrid 模式下通过 Reader 接口直接从 SQLite 读取作者/日期/标题等元数据生成 APA/BibTeX/Chicago 等引文格式，不再依赖 Web API 回退。
- **SQLite 快照持久化缓存**：Zotero 运行时 local 读命令从每次复制 ~242MB 快照（~2.2s）改为复用持久化缓存（`{dataDir}/.zotero_cli/snapshot/`，基于 mtime 自动失效重建）。busy_timeout 从 5s 缩短至 200ms。collections/tags/notes 等命令从 ~2.2s 降至 ~0.3s（7x 提升）。
- **Web 前端（React SPA）**：全新 Web UI，基于 React 19 + Vite 6 + Tailwind CSS 4 + TanStack Query 5 + React Router 7 技术栈。包含 6 个完整页面：Dashboard（统计总览）、Library（文献列表）、ItemDetail（条目详情 + PDF 预览弹窗）、Search（全文搜索）、Tags（标签管理）、Export（格式导出）。使用 SOTD 风格现代设计语言（圆角卡片、渐变按钮、微交互动效）。
- **HTTP API Server**：新增内置 HTTP 服务端（`zot web` 命令），提供 10 个 REST 端点（health / stats / overview / items / collections / tags / notes / files）。支持结构化 JSON 日志（slog）、请求 ID 追踪、CORS 中间件、panic recovery 和静态文件服务（开发模式热更新）。
- **可复用组件库（TDD）**：从页面内联代码中提取 9 个通用组件和 3 个自定义 Hook：
  - 展示组件：LoadingSpinner / EmptyState / StatCard / MetaRow / Section / TagBadge / SearchInput
  - UI 基础组件：Button（CVA 变体系统）/ Input / Skeleton（shadcn/ui 模式）
  - 自定义 Hooks：useDebounce / useItems / useCollections
- **骨架屏加载系统**：6 个页面级 Skeleton 组件（DashboardSkeleton / LibrarySkeleton 等），匹配各页面真实 DOM 结构，替换原有通用 spinner，消除布局抖动。
- **Toast 通知系统**：基于 Context + useReducer 的轻量通知（useToast hook + Toaster 组件），支持 success / error / warning / info 四种变体，自动消失（4s）+ 手动关闭 + 堆叠展示。
- **PdfViewer 懒加载**：pdfjs-dist 从静态 import 改为动态 `await import()`，按需加载减少首屏 bundle ~1MB。
- **PDF 预览弹窗**：ItemDetail 页面支持 PDF 附件内联预览（基于 pdf.js 渲染到 canvas），模态框支持 backdrop-blur 关闭动画。
- **ErrorBoundary**：全局错误边界组件，防止单个页面崩溃导致整个应用白屏。
- **设计文档**：新增知识图谱设计方案（`docs/knowledge-graph.md`）和智能体运行时架构文档（`docs/agent-design.md`）。

### 变更
- **文档目录重组**：将扁平的 `docs/` 重构为分类目录结构——`docs/user/`（用户指南）、`docs/plans/`（规划）、`docs/reference/`（参考）、`docs/architecture/`（架构）、`docs/dev/`（开发）。净减 ~2000 行冗余内容，新增 quickstart 快速入门页。
- **`zot init` 提示增强**：初始化交互中增加 AI 辅助设置提示，引导用户配置 web 模式相关选项。
- **标注文档完善**：新增 `docs/user/examples/annotations.md` 标注操作完整指南，包含双源架构图、三种标注模式对比表、`--clear` 双层删除流程图、实战案例和 FAQ。commands.md 补充 `annotations`/`annotate` 命令完整参考。

### 修复
- **`findDefaultDataDir()` 语法错误**：函数体缺少闭合 `}` 导致编译失败，已修复。
- **`--clear` 仅删 highlight**：清除模式下 `req.Type` 默认为 `"highlight"` 导致其他类型标注不被删除，已改为 clear 模式下清空 Type 过滤条件。

### 测试
- **前端测试体系**：Vitest + @testing-library/react + jsdom，共 20 个测试文件 / 97 个测试用例，覆盖全部组件、Hook 和 API client。
- **后端服务端测试**：server 包新增 logger_test.go，覆盖结构化日志输出；server_test.go 扩展覆盖 middleware 和 handler 集成场景。

### 工具链
- **pre-commit hook 增强**：检测暂存区无 `.go` 文件时跳过 gofmt/vet/test；无 YAML 时跳过 yamllint；纯前端/文档提交秒过。

## [0.0.6] - 2026-04-22

### 新增
- **统一 `zot init` 入口**：新增一站式初始化命令，替代分散的 `config init` + `setup pdf-extract` 流程。交互式仅提示关键字段（mode / type / id / key），支持 `--mode` / `--api-key` / `--library-id` 等标志实现非交互模式。local/hybrid 模式可选一步安装 PyMuPDF（`--pdf`）。
- **`zot init --check-pdf`**：诊断 PyMuPDF 安装状态（原 `setup pdf-extract --check` 功能迁移）。
- **`config init` 重定向**：运行时提示用户改用 `zot init`，不再执行旧版 7 问题交互流程。已删除 `promptConfigSetup()` 和 `runConfigInit()` 旧代码。
- **`zot overview` 发现命令**：一次性返回库全貌快照（统计 + Top 收藏夹 + Top 标签 + 最近条目 + FTS 索引状态），专为 AI Agent 设计。文本模式输出人类可读摘要，`--json` 返回完整结构化数据含 `meta.index_status` 和 `meta.read_source`。降低 agent 使用门槛，无需多次 API 调用即可获得库概览。
- **结构化 JSON 错误输出**：设置 `ZOT_JSON_ERRORS=1` 后所有命令错误以 `{ "ok": false, "command": "...", "data": "error msg", "code": N }` JSON 格式输出到 stdout，便于 agent 可靠解析。未设置时保持原有 stderr 纯文本行为。`jsonResponse` 新增 `Code` 字段，`printErr` 统一走 `jsonError` 路径。
- **Zotero 路径自动发现**：`zot init` 和 `zot open` / `zot select` 自动检测 Zotero 数据目录和可执行文件路径。Windows 通过注册表 Uninstall key 查询，Linux/macOS 通过常见安装路径探测。减少手动配置 `ZOT_DATA_DIR` 的需求。

### 变更
- **`zot schema` 元数据子命令**：将 6 个碎片化的 schema 内省命令（item-types / item-fields / creator-fields / item-type-fields / item-type-creator-types / item-template）合并为 `zot schema <sub>` 统一入口（types / fields / creator-types / fields-for / creator-types-for / template）。旧命令名已移除，直接报 unknown command。
- **移除复数条目命令**：删除 `create-items` / `update-items` / `delete-items`，统一使用单数形式 `create-item` / `update-item` / `delete-item`。消除智能体的选择困惑，与 collection/search/tag 命令风格保持一致。同时清理了 `parseWriteBatchArgs` 解析函数和 `errEmptyBatchPayload` 错误函数。
- **命令表面精简**：`setup pdf-extract` 安装模式重定向到 `zot init --pdf`；`--check` 诊断模式保留在 `zot setup pdf-extract --check` 和 `zot init --check-pdf` 双入口；`setup` 从主命令路由移除。
- **文档全面同步**：README、AI_AGENT、commands、MVP、architecture、CONTRIBUTING、error 示例、`.claude/` 和 `.codex/` skill 文件中全部 `config init` / `setup pdf-extract` 引用更新为 `zot init` / `zot init --pdf`；commands 写操作章节更新为仅保留单数形式；新增 overview 命令文档和 JSON 错误输出说明。
- **净减代码 ~250 行**：删除 promptConfigSetup()（74 行）、runConfigInit() 含 --example（49 行）、performPdfExtractSetup()（24 行）、3 个复数处理函数（~90 行）、parseWriteBatchArgs（~65 行）及对应 usage 常量/error 函数。
- **测试模块拆分**：将大型测试文件拆分为聚焦模块（commands_read_test → 5 个文件，client_read_test → 3 个文件，find/list 测试独立），提升编译速度和可维护性。

### 性能
- **overview 并行化加速 ~3x**：4 路 API 调用（stats / collections / tags / recent items）由串行改为 `sync.WaitGroup` 并行执行，overview 从 ~20s 降至 ~6s。
- **collections 全本地化**：Reader 接口新增 `ListCollections()` 方法，local 模式直接查 SQLite（JOIN collections + collectionItems），hybrid 模式不再强制走 Web API。
- **SQLite 快照持久化缓存**：Zotero 运行时 local 读命令从每次复制 ~242MB 快照（~2.2s）改为复用持久化缓存（`{dataDir}/.zotero_cli/snapshot/`，基于 mtime 自动失效重建）。busy_timeout 从 5s 缩短至 200ms。Zotero 运行下 collections/tags/notes 等命令从 ~2.2s 降至 ~0.3s（7x 提升）。
- **性能基线文档**：新增 `docs/PERF.md`，记录全部 16 个命令的耗时基线和 P0/P1/P2 优化方向，为后续性能优化提供量化依据。

### 修复
- **web/hybrid 模式子笔记缺失**：`GetItem` 在 web 和 hybrid fallback 路径下现在正确填充子笔记（child notes），与 local 模式的 show 输出保持一致。
- **Zotero 跨平台检测增强**：Windows 改用注册表 Uninstall key 查询定位 Zotero 安装路径；新增 Linux（`/usr/bin/zotero` 等）和 macOS（`/Applications/Zotero.app`）的可执行文件探测支持。

### 工具链
- **pre-commit hook 智能跳过**：检测暂存区文件类型——无 `.go` 文件时跳过 gofmt/vet/test（纯文档提交秒过）；无 YAML 文件时跳过 yamllint 检查。两者均无变更时直接放行。

## [0.0.5] - 2026-04-21

### 新增
- **`find` 高级过滤**：新增 11 个过滤选项，覆盖收藏夹（`--collection` / `--no-collection`）、标签模糊匹配（`--tag-contains`）、排除过滤（`--exclude-tag` / `--no-type`）、相对时间（`--modified-within` / `--added-since`）、附件细节（`--attachment-name` / `--attachment-path`）、排序方向（`--direction`）和分页偏移（`--start`）。
- **自动全文检索**：local / hybrid 模式下 FTS5 索引有数据时，即使不指定 `--fulltext` 也会自动走全文检索路径，降低 agent 使用门槛。
- **Snippet 安全限制**：`--snippet` 未指定 `--limit` 时默认限制为 50 条，防止批量提取意外消耗大量资源。

### 性能
- **Snippet 缓存命中加速 ~20x**：缓存命中时跳过冗余的 `syncIndex` 调用，snippet 响应从秒级降至毫秒级。
- **文本归一化去重**：正文归一化操作提前到缓存保存前仅执行一次，缓存命中路径完全跳过。
- **附件扫描捷径**：使用 `SnippetAttachmentKey` 快捷键跳过冗余的附件元数据扫描。
- **Agent 模式 P1 优化**：reader 层减少不必要的 fallback 判定、web 层精简响应解析、cli 层缩短数据流转路径。

### 修复
- **Annotation 显示截断**：长文本标注不再被截断，完整展示 text 和 comment 内容。
- **Annotation type 映射**：修正 PDF 文件内标注的类型映射，确保 highlight/note/underline/ink 分类准确。
- **PDF 提取优先级**：PyMuPDF 固定为首选提取器，Zotero ft-cache 作为中间回退，pdfium WASM 为最终兜底。此前优先级不稳定可能导致低质量文本输出。
- **Release CI 构建一致性**：CI ldflags 补充 `-s -w`（剥离调试符号），与本地 `make release` 产物大小一致。

### 文档
- **commands.md 全面补全**：find 过滤选项按类别分组表格化（新增 11 个）；输出控制补充 `--direction` / `--start`；全文检索补充 auto-enable 说明 + snippet limit 注意；extract-text 更新三级提取器优先级；cite 重写为正确的 `citation|bib` 格式 + 选项表；notes 补充 `--query` 参数；versions 补充 4 种子类型及完整用法示例；环境变量表新增 3 个 retry 参数。
- **AI_AGENT.md 扩展**：新增 6 个工作流小节（PDF 文本提取、PDF 标注操作、Zotero 桌面端联动、笔记搜索、全文检索最佳实践、高级过滤组合）；新增「性能优化建议」章节（检索性能/API 调优/缓存行为）；优先级建议扩充至 5 级。
- **README 更新**：科研工作流补充高级过滤组合示例和全文检索 auto-enable 说明；cite 示例修正为实际支持的格式；命令速查表 find 描述更新。
- **SKILL.md 同步**（`.claude` + `.codex`）：全面重写，与 commands.md 和 AI_AGENT.md 保持一致，补充全部 find 高级选项、PDF 操作示例、笔记查询、环境变量速查表和性能注意。

### 工具链
- **pre-commit hook 智能跳过**：检测暂存区文件类型——无 `.go` 文件时跳过 gofmt/vet/test（纯文档提交秒过）；无 YAML 文件时跳过 yamllint 检查。两者均无变更时直接放行。

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
