# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.0.1`。

## [Unreleased]

### 新增
- **限定语料的正则全文取证**：`pdf text --grep` 默认按不区分大小写的 Go 正则解析，支持用 `|` 一次检索多个关键词；新增 `--collection`，接受收藏夹 key、唯一名称或完整层级路径。有分页全文缓存时，JSON 按附件和命中页返回 `match_count`、页码与相邻上下文，检索保持只读。
- **CLI v2 阶段 6 长尾命令与运行时能力**：迁移 `schema list/show`、`server start`、`sync pull`，加入 bash/zsh/fish/PowerShell completion；旧 schema 变体、裸 `server` 与裸 `sync` 统一翻译为 canonical invocation，sync 下载生命周期下沉至应用层。
- **CLI v2 阶段 5 Reference 与 Index**：迁移 `ref show/find/related/cited/ctx/links/entities/profile/build/resolve/status` 与 `index build/status`，Reference/Index 的 store、client、reader 和构建生命周期下沉至应用层。
- **CLI v2 阶段 1 基础设施**：引入 Cobra 命令树、canonical Invocation、应用服务 Result 与统一 JSON/text/error renderer；`version`、`config init/show/check` 已迁移，旧 `zot init`、`config validate/path` 通过参数翻译进入同一实现。
- **CLI v2 阶段 2 只读资源切片**：迁移 `lib show/stats/log`、`item list --scope trash|pubs`、`coll/tag/note/search/group list`；所有入口统一经过 typed request、应用服务和 renderer，旧命令仅翻译参数并返回 canonical JSON command。
- **CLI v2 阶段 3 资源与安全写入**：迁移 `item find/show/new/edit/delete/tag/untag/supp/export`、Collection/Note/Saved Search 的 show/find/CRUD 及集合成员操作；统一 `--data`、`--from`、`--set`、批量位置 key、`--dry-run`、`--yes`、`--if-version`，未显式给版本时由应用服务读取当前 library version。
- **CLI v2 阶段 4 PDF 与标注切片**：迁移 `file show/check`、`pdf text/figs/open`、`ann list/new/delete`；保留全文缓存、页码/grep/字符限制、Markdown 输出、批量 worker、远端 PDF 路由和双源标注能力，标注删除统一经过显式 delete action 与安全门。
- **`extract-text` 文件输出与全库批量**：`-o/--output-dir` 可把单篇 PDF 全文写成 Markdown；`--all` 支持批量导出本地所有带 PDF 的条目，默认落盘为 Markdown，JSON 模式返回 manifest，避免把全文正文塞进响应。
- **`extract-figures --all`**：支持直接批量处理本地所有带 PDF 的条目，复用现有 worker pool、页数排序和缓存路径。
- **附件健康检查**：`inspect-attachment` 新增 `--health`，可诊断本地附件路径未解析、文件缺失、路径是目录、文件名过长、非法字符、异常空格、PDF 缺 `.pdf` 后缀和泛化命名等问题；`find` 新增 `--missing-attachment`、`--bad-attachment-name`、`--attachment-health critical|error|warning|info` 用于批量定位异常附件。

### 修复
- **`extract-figures` 跨页同尺寸误去重**：跨页 dedup 仅用于小面积/页边且无 caption 的重复元素，避免 BMC/Nature 风格正文大图因宽高相近被当作页眉页脚重复图过滤。

### 变更
- **查询范围参数去布尔堆积**：`ref build` 使用 `--scope pending|failed|contexts|grobid`，`ref status` 使用 `--view summary|failed|unsupported|grobid`。`item find --all` 仅取消结果上限并与 `--limit` 互斥；metadata 范围可省略 QUERY，过滤和排序不再借用内部 `All/ExplicitAll` 状态。
- **查询与导出接口收敛**：`item find` 用单一 `--in metadata|fulltext` 取代四个全文布尔参数，避免把两套不同查询语言伪装成可合并范围；`ref find` 用 `--in all|references|contexts|metadata` 取代三个范围布尔参数。全文与引用 QUERY 原样交给 SQLite FTS5。`note find` 统一为不区分大小写的 Go 正则，`note list` 不再承担查询。`item export` 只接受明确 key 或 `--from PATH|-` 的结构化选择结果，不再内置第二套筛选器。
- **CLI v2 阶段 7 完成**：Cobra tree 成为唯一执行内核；旧命令只在纯 argv translator 中映射到 canonical invocation，`setup/select/abstract/relate/key-info` 收敛为隐藏的 redirect-only adapter，主帮助、completion、JSON command 和用户文档统一使用 v2 语法。
- **PDF 提取参数收敛**：移除 `extract-text --markdown/--md` 兼容入口，文件输出统一由 `-o/--output-dir` 或 `--all` 触发；`extract-figures --max-per-page` 保留为高级调参项，不再出现在主用法路径中。

### 性能

### 移除
- 删除 `commandRegistry`、手写 `dispatch`、旧 usage 常量、通用旧参数解析器、CLI 内重复 JSON/text renderer，以及已迁移或退出稳定语法的旧业务 handler。
- 删除阶段 5 的 reference/index 旧业务 handler，以及阶段 6 的 server/sync 手写参数解析与 CLI 内业务编排。
- 删除阶段 2 对应的旧只读 handler、旧 dispatch 分支与 `changes` 手写参数解析器，避免新旧两套业务逻辑并存。
- 删除阶段 3 对应的 find/show/export/supplements/写操作旧 handler 与手写参数解析器；高频旧入口统一翻译到 canonical invocation。
- 删除阶段 4 对应的附件预览、PDF 提取/打开、标注读写旧 handler 与手写参数解析器；旧 `--clear` 仅翻译为 canonical `ann delete`。

### 工具链

### 文档

## [0.0.11] - 2026-07-08

### 新增
- **`zot sync` 子命令**：把远端 `zot server` 的整库一次性同步到本地（默认 `~/.zot/sync/`），之后可用 `ZOT_MODE=local ZOT_DATA_DIR=...` 离线工作，无需远程服务常开。同步内容包含 `zotero.sqlite`、`storage/` 附件原文件和 `.zotero_cli/fulltext/` 全文索引缓存；server 端新增 `/api/v1/sync/manifest`、`/sync/sqlite-file/{name}`、`/sync/storage/{key}/{file}`、`/sync/fulltext/{path...}` 端点，并复用 `ZOT_SERVER_AUTH_KEY` 鉴权。
- **远程 PDF 能力补全**：remote 模式支持全文提取、标注读取、`annotate` 写入和 `annotations --clear` 清除；服务端负责执行 PDF 文件操作，客户端不再需要额外配置 `ZOT_API_KEY`。
- **`extract-text` 轻量输出控制**：新增 `--pages`、`--grep`、`--max-chars`，可只提取指定页码、按关键词过滤段落并限制输出长度，避免方法段/结果段分析时一次性吐出整篇 PDF。
- **本地补充材料发现 (`supplements`)**：新增 `zot supplements`，扫描 Zotero 本地附件中的 Supplementary tables、Source data、Reporting summary、`MOESM` / `mmc` / 表格数据文件等候选，并返回 `kind`、`confidence`、`evidence`、`local_path`、`resolution_status` 等字段。
- **表格附件预览 (`inspect-attachment`)**：新增 `zot inspect-attachment`，可预览本地 `.xlsx` / `.xlsm` 附件的 sheet、行列规模、前几行内容和表头候选，便于在多个补充表格中快速定位应分析的 sheet。
- **在线补充材料发现**：`zot supplements KEY --online --json` 支持 Zenodo、Figshare 和 Nature/Springer provider。Zenodo/Figshare 走公开 API；Nature/Springer 解析页面里的 `static-content.springer.com` 链接，并对 `#MOESM` 锚点做轻量 HEAD 探测以升级为 direct download metadata。在线发现只返回 metadata/link，不自动下载文件内容。
- **Lean JSON 默认输出**：`find`、`show`、`abstract` 的 JSON 默认返回轻量字段，避免默认 payload 带出大型附件、笔记、标注和期刊等级对象；需要完整数据时继续使用 `--full`。
- **remote/hybrid 读写能力增强**：补齐 remote 下打开文件、可见性提示、读能力边界和 HybridReader figure 支持；服务端增加鉴权与更多内容端点。

### 修复
- **PDF 提取与 JSON 诊断**：修复 `extract-text` usage 与解析行为不一致、错误 JSON 中 `command` 为空、lean JSON 缺少附件省略提示等问题；错误响应保留结构化 `type` / `code`，便于 agent 判断失败原因。
- **全文索引陈旧问题**：修复 stale fulltext index 条目，增强索引 freshness 检查，减少 PDF 文件变化后命中旧文本的情况。
- **最近入库排序**：修复 local find 中最近条目的排序语义，补充 `dateAdded` 相关测试。
- **帮助与命令导航**：修正顶层 `-h` / `--help` 信息、命令表与实际 dispatch 的偏差，完善 agent 使用路径。
- **remote annotation 边界**：收紧 remote 标注读写能力边界，避免客户端能力判断与服务端实际支持不一致。
- **跨平台构建**：Makefile 使用 `GOEXE`，修复 Windows 等平台下构建产物扩展名不一致的问题。

### 变更
- **`zot-server` 合并为 `zot server`**：单一 `zot` 二进制即可启动服务端（`zot server [--port PORT]`，支持 Ctrl+C 优雅关闭）。删除独立 `cmd/server` 入口和 server 专用 release 构建目标，发布包只产出一个 `zot`。
- **所有命令帮助重组**：usage 常量与对应 `runXxx` 实现 co-locate，减少 help 文案与实际解析器漂移。
- **前端 embed 路径对齐**：vite `outDir` 改为 `../internal/server/web/dist`，匹配 `internal/server/embed.go` 的 embed 路径，使 `go build -tags embed` 能正确打包 Web UI。

### 性能
- **`zot sync` 增量和并发优化**：同步按 size+mtime 跳过未变文件，默认并发 8，HTTP 连接池 `MaxIdleConnsPerHost=32` 复用 keep-alive；SQLite manifest 拆分主库与 `-wal` / `-shm` / `-journal`，WAL 模式下二次同步通常只传小 wal 文件。
- **`zot sync` 稳定性优化**：SQLite 文件使用 staging 目录原子 swap，下载 fail-fast，启动时清理上次中断残留 `.tmp` / staging 目录，降低中断后主库与 wal 不一致的风险。
- **全文索引 freshness 加速**：优化 fulltext index 新鲜度检查，减少重复扫描和不必要的缓存刷新。
- **Release 构建并行化**：并行构建发布产物，缩短 release workflow 时间。

### 工具链
- **提交信息规范检查**：新增 `scripts/commit-msg`，并在 `scripts/pre-commit` 中校验本地是否已安装 commit-msg hook，限制 Conventional Commits 格式。

### 文档
- 同步 README、commands、quickstart、backend 架构文档，以及 `.claude` / `.codex` skills，覆盖 lean JSON、remote 标注读写、`zot sync`、`extract-text` 输出控制、本地/在线补充材料发现和表格附件预览等新能力。
- 新增统一平台规划文档，并更新 agent 面向的 find/show 输出示例，避免仍按旧的完整 JSON 结构解析。

## [0.0.10] - 2026-05-07

### 新增
- **`date_added` 字段**：`find` 和 `show` 输出现在包含条目入库时间（`date_added` 字段），支持全部四种模式。find 表格输出新增"入库时间"列，show --full 新增 Date Added 区块
- **`abstract` 摘要字段与命令**：条目数据新增 `abstract`（摘要）字段，在 find/show/abstract 命令中均可获取。新增独立 `zot abstract <key> [keys...] [--json]` 命令用于批量查看条目摘要。支持 `--include abstract` 字段过滤和 `--full` 默认展示
- **remote 模式**：新增 `remote` 运行模式，CLI 通过 HTTP 连接远程 `zot-server` 实例访问 Zotero 数据。支持 `zot init --mode remote --server-addr URL` 初始化。服务器端默认端口 8021（`ZOT_SERVER_PORT`）
- **remote + web 双通道**：remote 模式可额外配置 `ZOT_API_KEY` + `ZOT_LIBRARY_ID`，使写操作和 web-only 命令直连 Zotero Web API，读操作仍走远程服务器
- **zot-server 独立二进制**：新增 `cmd/server/main.go`，构建 `zot-server` 提供 REST API（`/api/v1/items`、`/stats`、`/tags`、`/collections`、`/notes`、`/files/{key}` 等），支持 CORS、请求日志、错误恢复
- **Web 前端增强**：无障碍访问、错误恢复、移动端响应式布局、PDF 查看器、收藏夹筛选

### 修复
- **`find -l` 短标志**：`--limit` 选项现在支持 `-l` 短标志（此前仅支持完整 `--limit`）
- **FindOptions 序列化**：修复 remote 模式下 FindOptions 序列化不完整导致过滤参数丢失的问题
- **Content-Disposition 安全**：修复文件名含双引号时下载响应头解析错误

### 文档
- 全面更新 10 个文档文件以覆盖 remote 模式（SKILL.md、reference.md、README.md、架构文档、CHANGELOG、快速上手、命令参考、路线图）

## [0.0.9] - 2026-04-25

### 新增
- **`extract-figures` 每页上限（`--max-per-page`）**：当 PyMuPDF `cluster_drawings` 将密集矢量图分割为数百个碎片时，按像素面积保留每页最大的 N 张图片，自动删除磁盘上的多余文件。默认值 25，可通过 `-m N` 调整
- **`extract-figures` 多 PDF 附件支持**：不再仅处理第一个 PDF，而是遍历条目所有 PDF 附件分别提取
- **`extract-figures` 输出目录结构优化**：默认输出至 `{DataDir}/.zotero_cli/figures/{attachmentKey}/`，与全文缓存布局一致，避免多附件命名冲突
- **`extract-figures` Caption 检测修复**：修复 Go 原始字符串中正则双反斜杠转义错误（`\\s` → `\s`），导致 FIGURE/图 编号正则永远不匹配；新增中文 caption 支持（`图\d`）；合并 `calc_text_density` 的 `is_caption` 信号到 `has_caption` 输出字段；`attach_caption` 新增区域内 caption 检测分支
- **`extract-figures` 参数优化**：最小文件大小阈值从 15KB 提升至 35KB，过滤页眉/页脚/标尺等小尺寸噪音；最小输出尺寸从 120×100 提升至 150×120
- **`extract-figures` 磁盘缓存**：为每个 PDF 附件生成 `{DataDir}/.zotero_cli/figures_cache/{attachmentKey}/manifest.json`，按附件 key、文件路径、mtime、size 和脚本版本校验缓存新鲜度；缓存命中时跳过 Python 提取并复用已有 PNG 输出

### 修复
- **`--help` 全面支持**：所有 17 个子命令统一支持 `--help`/`-h`，包括混合 flag 场景（如 `find --json --help`）；新增 `containsHelp()` 扫描全部参数；共享解析器（`parseJSONOnlyArgs`/`parseJSONAndLimitArgs`/`parseSingleValueCommand`）内置 help 识别
- **主帮助二进制名修正**：Windows 下不再显示 `zot.exe`，统一显示 `zot`
- **`setup` 路由恢复**：`zot setup` 返回弃用提示而非 "unknown command"
- **`zot help <command>` 路由修正**：递归调用目标子命令的 `--help` 输出，而非始终显示主帮助页
- **`deleted` 命令 400 错误**：Zotero API 要求 `since` 参数，已补全默认值 `since=0`
- **`key-info` 命令 404 错误**：无参数时自动使用配置中的 API Key，无需手动传入
- **`add-tag`/`remove-tag` 405 错误**：Zotero Web API 不支持 PATCH 方法，批量更新改用 POST、单项更新改用 PUT
- **`export` 默认格式修正**：默认导出格式从 `bib`（CSL 引用 HTML）改为 `bibtex`，与数据导出定位一致；移除 `bib` 作为有效选项值
- **CI Release 环境变量作用域修复**：job-level env 无法引用 `${{ env.* }}`，改用 step output 传递版本前缀
- **统一 JSON 输出信封**：所有 `--json` 输出强制使用 `{ok, command, data, meta}` 信封结构，消除 6 处 raw data 直接输出（config show / setup pdf-extract / item-template / key-info access / local export / web export）；智能体解析逻辑统一为 `resp["ok"] && resp["data"]`
- **标准化错误 JSON 格式**：错误响应 Data 从纯字符串升级为结构化 `{error, type, code}` 对象；新增错误分类（`not_found`/`unsupported_feature`/`temporarily_unavailable`/API 状态码映射如 403→forbidden、429→rate_limited、412→precondition_failed），智能体可按 `type` 字段程序化处理不同错误类别
- **`extract-figures` 输出稳定性**：修复 PyMuPDF/native warning 混入 stdout 导致 JSON 解析失败的问题；成功但 0 图的结果也会写入缓存；缓存命中时校验目标输出目录中的 PNG 是否存在，避免跨目录复用 manifest 造成缺图
- **`extract-figures` 部分失败保留结果**：多 PDF 附件中部分附件失败时仍返回已成功提取的图片；JSON 输出包含 0 图成功条目和 partial success figures，便于批量任务统计
- **`extract-figures` 低质量误检过滤**：过滤封面/末页 logo、出版社标识、作者头像、扫描书碎片等低质量 raster 假阳性；长文档中低面积 raster 候选提高门槛，减少正文局部截图误提取
- **`extract-figures` 重复 PDF 去重**：同一 Zotero 条目下多个 PDF 附件按 SHA-256 内容去重，只提取第一份相同内容 PDF，避免重复图片和重复耗时

### 变更
- **`versions` 命令重命名为 `changes`**：消除与 `version`（CLI 版本号）命令的命名混淆，usage/help/路由/测试/文档全面同步

### 性能
- **`extract-figures` 长尾优化**：对图片碎片页和极端矢量页加入阈值保护，跳过病态 `get_image_bbox`/`cluster_drawings` 路径并使用低置信 fallback；200 篇并行 15 的冷运行从约 495 秒降至约 63 秒量级
- **`extract-figures` 图片锚点加速**：优先使用 PyMuPDF `page.get_image_rects(xref)` 替代慢路径 `get_image_bbox(name)`，复杂页单页锚点定位从秒级降至百毫秒级
- **`extract-figures` 大 raster 渲染降采样**：大面积 raster 候选改用 150dpi 渲染，cluster 和小图仍保留 200dpi，降低扫描图和页级大图的渲染尾延迟
- **`extract-figures` 批量调度优化**：按实际 PDF 页数排序后分配并发任务，页数获取失败时回退估算，避免少数长文档拖慢批处理尾部

### 移除
- **删除 `cite` 命令**：引用格式化功能对智能体无价值（返回 HTML 噪噪或纯文本），且与 `export --format bibtex` 功能重叠；需要引用格式的场景统一使用 `export`（csljson/bibtex/ris），需要人类可读引用的场景极少且可用 `export` + 后处理替代

### 工具链
- **`version.json` 发布清单**：Release workflow 自动上传 version.json（含全平台下载 URL、skill_version、base_url）至七牛 CDN；SKILL.md frontmatter 增加 version 字段支持增量检测
- **Gitee 镜像源支持**：`zot init` 安装提示同时提供 GitHub 和 Gitee raw URL，国内用户可切换至 Gitee 加速下载
- **七牛 CDN 自动上传**：Release 发布后自动将所有 artifacts 上传至七牛 CDN，需配置 `QINIU_ACCESS_KEY` / `QINIU_SECRET_KEY` secrets

### 文档
- **Gitee 链接修正**：移除错误的 Gitee Releases URL（镜像仅同步 git 仓库），保留 Gitee raw 链接用于 skill/文档下载
- **CDN 下载链接**：README AI 配置提示和手动安装章节增加七牛 CDN 备选下载源

## [0.0.8] - 2026-04-23

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
- **版本检查功能**：`zot version --check` 检测 GitHub Releases 是否有新版本可用。

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
- **显式全文检索**：local / hybrid 模式通过 `item find QUERY --in fulltext` 使用 FTS5 索引；默认 `metadata` 不会根据索引状态隐式改变查询语义。
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
