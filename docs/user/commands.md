# 命令参考

CLI v2 使用稳定的“资源 + 动作”语法。`zot <resource> --help` 和 `zot <resource> <action> --help` 是参数细节的权威来源。

## 全局选项

```text
--format text|json
--json
--verbose, -v
--no-color
--mode web|local|hybrid|remote
--timeout DURATION
```

JSON 响应统一使用 `{ok, command, data, meta}`；`command` 始终是 canonical path，例如 `item find`、`schema list`。

## 命令树

```text
zot
├── lib show|stats|taste|log
├── item list|find|show|new|edit|delete|tag|untag|import|supp|export
├── coll list|show|new|edit|delete|add|remove
├── note list|show|find|new|edit|delete
├── tag list|replace|apply|clean
├── search list|show|new|edit|delete
├── group list
├── file path|open|show
├── pdf text|figs|open
├── ann list|new|delete
├── ref show|find|related|cited|ctx|links|entities|profile|build|resolve|status
├── index build|status
├── schema list|show
├── config init|show|check
├── serve
├── sync
├── completion
└── version
```

高频快捷入口 `find`、`show`、`export` 分别等价于 `item find`、`item show`、`item export`，但没有独立实现。

## Library 与只读资源

```powershell
zot lib show --json
zot lib stats --json
zot lib taste --json
zot lib taste --init
zot lib taste --path
zot lib log --deleted --json
zot lib log --kind items --since 120 --json

zot item list --limit 20 --json
zot item list --scope trash --json
zot item list --scope pubs --json
zot item find "CRISPR" --in metadata --type article --limit 20 --json
zot item find 'CRISPR AND "gene editing"' --in fulltext --snippet --json
zot item show ITEMKEY --json

zot coll list --json
zot note list --json
zot note find 'methods?|procedure(s)?' --json
zot tag list --json
zot search list --json
zot group list --json
```

`lib show` 分别报告本地数据目录、PDF 全文索引和 `taste.md` 状态，不再用含义模糊的 `Index` 代指 `data_dir`。`lib taste` 显示供用户或 Agent 遵循的长期文献管理偏好；它不是 Zotero 原生设置，CLI 也不会自动执行其中规则。`--init` 创建模板，`--path` 输出实际位置。

统一分页参数是 `--limit`、`--offset`、`--sort`、`--order asc|desc`。`item find` 的轻量结果默认限制 100 条，`--snippet` 或 `--full` 默认限制 20 条；`--all` 仅表示取消上限，并与 `--limit` 互斥。metadata 范围允许省略 QUERY，因此过滤、排序和默认浏览都不需要借用 `--all`。JSON 分页元数据包含 `returned`、`limit`、`offset`、`has_more` 和可选的 `next_offset`。旧 `--start`、`--direction` 已不再接受。

常用 item type 短名会在输入层归一化：`article` → `journalArticle`、`chapter` → `bookSection`、`conf` → `conferencePaper`、`web` → `webpage`、`blog` → `blogPost`。JSON 和持久化数据始终使用 Zotero 官方值。

`item find --in metadata|fulltext` 是唯一的检索范围选择器，默认 `metadata`。fulltext 的 QUERY 原样交给 SQLite FTS5，可使用 `"完整短语"`、`prefix*`、`AND` / `OR` / `NOT` 和括号；`--snippet` 要求 `--in fulltext`。元数据与全文使用不同查询语义，因此不提供含混的合并范围。`note find QUERY` 则使用不区分大小写的 Go 正则；`note list` 只负责枚举，不接受查询参数。

导出只接收明确的 item key，或通过 `--from PATH|-` 接收 key 数组、item 数组及 `find --json` 响应。需要按收藏夹、日期或标签导出时，先运行 `item find --json`，再把结果文件或 stdin 交给 `item export --from`。服务会按 `--batch-size`（默认 100）分批请求；默认输出继续使用统一 JSON/text 契约，显式 `--stream` 则把 BibTeX、RIS 或一个有效的 CSL-JSON 数组直接流式写到 stdout，且不能与 `--json` 同用。

## 写入与安全

```powershell
zot item new --set itemType=article --set title="Example" --dry-run --json
zot item edit ITEMKEY --set title="New title" --if-version 42 --json
zot item delete ITEMKEY --yes --if-version 42 --json
zot item tag KEY1 KEY2 --tag review --json
zot item untag KEY1 KEY2 --tag review --json
zot item import ./paper.pdf --dry-run --json
zot item import ./paper.pdf --json
zot item import ./paper.pdf --collection COLLKEY --json
zot item import ./paper.pdf --collection "研究/植物/栗属" --json
zot item import ./paper1.pdf ./paper2.pdf --collection COLLKEY --json
zot item import --from ./pdf-files.json --dry-run --json
zot item import DOI:10.1000/example --dry-run --json
zot item import PMID:12345678 --json
zot item import https://doi.org/10.1000/example --json
zot item import --from ref-result.json --dry-run --json

# 默认只预览；Go 正则替换支持 $1 等捕获组
zot tag replace --match '^(SV|SV检测)$' --replace '结构变异' --json
zot tag replace --match '^植物/(.+)$' --replace '物种/植物/$1' --yes --json

# 仅清理自动标签；默认限制为只使用一次，并且只预览
zot tag clean --match '^[\x00-\x7F]+$' --max-items 1 --json
zot tag clean --match '^[\x00-\x7F]+$' --max-items 1 --yes --json

# 一次应用多篇文献的多个标签变更；先 dry-run，再正式写入
zot tag apply --from ./tag-plan.json --dry-run --json
zot tag apply --from ./tag-plan.json --json

zot coll new --name "Reviews" --json
zot coll add COLLKEY ITEM1 ITEM2 --json
zot coll remove COLLKEY ITEM1 --json
zot note new --parent ITEMKEY --text "Reading note" --json
zot search new --data '{"name":"Recent","conditions":[]}' --json
```

`tag-plan.json` 是面向条目的操作数组，例如：

```json
[
  {"keys": ["ITEMA001", "ITEMA002"], "add": ["进化", "综述"]},
  {"keys": ["ITEMA001"], "remove": ["旧标签"]},
  {"keys": ["ITEMA002"], "add": ["miR156"], "remove_automatic": ["miR156"]}
]
```

`tag apply` 会合并同一条目的操作，`remove_automatic` 只删除 `type=1` 的自动标签，因此可安全保留同名手动标签；同时添加同名标签可将自动标签提升为手动标签。`tag list --json` 返回 `type` 字段（`0` 手动、`1` 自动）。批量操作按 Zotero 官方上限每 50 条分批写入并统一核验。

写入统一接受 `--set FIELD=VALUE`、`--data JSON`、`--from PATH`；`--from -` 表示 stdin。安全选项统一为 `--dry-run`、`--yes/-y`、`--if-version`。`tag replace` 是例外：不带 `--yes` 时默认只生成预览。

- `ZOT_ALLOW_WRITE=1` 控制创建和修改。
- `ZOT_ALLOW_DELETE=1` 控制删除，默认关闭。
- destructive action 始终执行确认、权限和版本前置条件。

## 附件、PDF 与标注

```powershell
zot item supp ITEMKEY --json
zot file path ITEMKEY --json
zot file path ATTACHKEY --json
zot file open ATTACHKEY --json
zot file show ITEMKEY --json

zot pdf text ITEMKEY --pages 3-8 --grep methods --max-chars 12000 --json
zot pdf text ITEM1 ITEM2 --grep "gene\s+flow|introgression" --json
zot pdf text --collection "研究/植物/栗属" --grep "gene\s+flow|introgression" --json
zot pdf text ITEM1 ITEM2 --output-dir ./markdown --json
zot pdf figs ITEMKEY --output-dir ./figures --json
zot pdf open ITEMKEY --page 5

zot ann list ITEMKEY --type highlight --page 3 --json
zot ann new ITEMKEY --attachment ATTACHMENT_KEY --text "target phrase" --color yellow --json
zot ann delete ITEMKEY --source zotero --type highlight --dry-run --json
zot ann delete ITEMKEY --source zotero --type highlight --yes --json
zot ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --page 3 --yes --json
```

`item supp` 返回补充材料候选的类型、解析状态、置信度、证据、本地路径或在线下载地址；`--online` 只支持单个条目，不能与 `--all` 同用。命令不会自动下载、导入或修改任何补充文件。

`file path` 接受普通 item key 或 attachment key，返回真实本地路径与健康诊断，但不打开或修改文件。`file open` 使用系统默认程序打开附件；普通条目只有一个附件时可直接打开，有多个附件时必须改传明确的 attachment key。`file show` 目前专用于电子表格预览。三者都不再使用 `--item`。

`item import` 在 Zotero 元数据识别完成后，会定位本次导入最终保留的 PDF 附件并执行增量全文索引；指定收藏夹、重复附件清理和索引均以最终附件 key 为准。索引失败会作为 JSON envelope 中的 warning 返回，不会重复写入 stderr，也不会回滚已成功的 Zotero 导入。`find --snippet` 只返回轻量条目信息和以实际命中为中心的约 1200 字符证据，并在 `matched_chunk` 中保留页码、附件 key 与坐标；它与 `--full` 互斥。

`item import --collection` 接受收藏夹 key、唯一名称或完整层级路径。名称存在歧义时命令会列出带 key 的候选项，不会自动猜测。`--dry-run` 不需要开启写权限，只校验 PDF、Zotero Desktop Connector 和目标收藏夹，不上传文件、不创建条目、不分配收藏夹，也不启动元数据识别；真实导入才要求 `ZOT_ALLOW_WRITE=1`。`config check` 会额外报告 `zotero_desktop_connector_available`；Connector 不可用不会使配置检查失败，但导入 PDF 前必须启动 Zotero 桌面端。

批量 PDF 继续使用同一入口：`item import FILE1.pdf FILE2.pdf`，或由 `--from PATH|-` 读取字符串路径数组、`{"path":"..."}` 对象数组及 CLI `data` envelope。批量任务只解析一次公共收藏夹并复用一个 Connector 客户端，按输入顺序逐个上传和完成后处理；每项返回 `ready/success/partial/failed`、阶段、warning、item/attachment key 和索引状态。无效文件或单项上传失败只计入该项，公共配置、写权限、Connector 和收藏夹错误则使整批失败。相同规范路径的重复输入不会再次上传。批量进度写入 stderr，因此 `--json` stdout 始终保持单个有效 JSON envelope。单个输入仍返回原有单条结果，不额外包裹 batch 层。

同一 `item import` 入口也接受 DOI、PMID、doi.org/PubMed URL 和 `--from PATH|-` 的单条引用 JSON。identifier 输入通过 PubMed 解析为最终 Zotero payload，按 DOI、PMID、标题/年份/首位作者去重；唯一命中返回 `existing` 并跳过，多重命中报告歧义且不创建。identifier 导入不会自动下载 PDF，真实创建要求 Web API 写能力（`web`/`hybrid`，或带 API 凭据的 `remote`）；纯 `local` 可执行 dry-run 和本地重复检查。PDF 已被 Connector 接受后，后续收藏夹、识别、重复附件清理或索引失败会返回 `partial` 和分阶段 warning，而不是把已成功上传伪装成整体失败。

local/hybrid 下无过滤条件的 `pdf text ITEMKEY --json` 默认返回 `content_path` 和可选的 `chunks_path`，不再把完整正文复制到 JSON；Agent 应直接读取 `content_path`。这些路径属于 `.zotero_cli/fulltext/cache/<attachment-key>/` 下的提取文本缓存，不是 PDF 二进制副本；源 PDF 使用 `file path` 定位。`--grep` 默认按不区分大小写的 Go 正则解析，可用 `|` 检索多个关键词；无效正则会直接报错。`--collection` 接受收藏夹 key、唯一名称或完整层级路径，并与 item keys、`--all` 三选一。使用 `--grep` 且存在分页缓存时，JSON 按附件和命中页返回 `match_count`、页码与相邻上下文，但不会创建标注。`--grep`、`--pages`、`--max-chars` 返回计算后的文本子集，`--max-chars` 按 Unicode 字符而非 UTF-8 字节计数。多条目和 `--all` 同样返回缓存路径数组，只有显式 `--output-dir` 才导出 Markdown；它不会复制源 PDF。缓存读取、FTS 检索和 `index build` 的跳过判断都会校验 PDF 源文件；文件被替换后旧缓存与旧索引不会继续命中。`index status` 只报告派生索引的位置，并提示通过 `file path` 查询源文件。remote 模式仍返回正文，因为客户端不能直接读取服务器路径。

`ann list`、`ann new`、`ann delete` 明确区分读取、创建和删除。多 PDF 条目默认选择第一个 PDF，也可统一使用 `--attachment ATTACHMENT_KEY` 精确选择。`ann new` 在临时副本中写入并验证后替换原 PDF；实际写入零匹配会报错并保留原文件，`--dry-run` 则允许零匹配。删除必须用 `--source zotero|pdf` 选择来源，建议先用 `--dry-run` 查看精确候选。Zotero 原生标注按 annotation item key 通过 Web API 删除，不再直接修改 SQLite；PDF 内嵌标注按 xref 在临时副本中修改并验证后替换。canonical 语法不接受 `annotations --clear` 或 `annotate --clear`。

## Reference 与全文索引

```powershell
zot ref show ITEMKEY --json
zot ref find "genome assembly" --field mesh --json
zot ref related ITEMKEY --limit 20 --json
zot ref cited ITEMKEY --external --json
zot ref ctx ITEMKEY --json
zot ref links ITEMKEY --json
zot ref entities ITEMKEY --json
zot ref profile ITEMKEY --json

zot ref build --workers 3 --json
zot ref build --scope failed --workers 2 --json
zot ref build --scope contexts --workers 3 --json
zot ref build --scope grobid --limit 5 --json
zot ref resolve --workers 8 --json
zot ref status --json
zot ref status --view failed --json
zot ref status --view unsupported --json
zot ref status --view grobid --json

zot index build --workers 4 --json
zot index status --json
```

`ref` 使用独立的结构化引用索引 `<data-dir>/.zotero_cli/ref/index.sqlite`，保存参考文献、引用语境、本地条目匹配和外部文献发现结果；它与 PDF 全文索引不是同一个数据库。`ref build --scope pending|failed|contexts|grobid` 使用单一构建范围；`ref status --view summary|failed|unsupported|grobid` 使用单一展示视图。GROBID 仅在显式 `--scope grobid` 时启用。完整来源和 fallback 语义见 [引用索引与文献发现](./references.md)。

`index status` 同时统计 `index.sqlite` 和 `.zotero_cli/fulltext/cache/` 提取文本缓存的占用；其中不包含 PDF 二进制文件。源 PDF 继续通过 `file path` 查询。

## Schema、配置与运行时

```powershell
zot schema list types --json
zot schema list fields --json
zot schema list fields article --json
zot schema list roles article --json
zot schema show article --json

zot config init
zot config show --json
zot config show --path
zot config check --json

zot serve
zot sync
zot sync status
zot sync status --full
zot completion powershell
zot completion bash
zot version
```

Schema 响应默认缓存 7 天；`meta.read_source=cache` 表示缓存命中。使用 `--refresh` 强制联网更新。缓存过期且网络不可用时会返回 stale 缓存，并通过 `meta.stale=true` 和 warning 明确提示。

completion 支持 `bash`、`zsh`、`fish`、`powershell`，生成过程不会加载配置或访问网络。

`sync` 是远端到本地的单向增量拉取，包含 SQLite、全文索引、`storage/` imported 附件和可解析的 `linked_file` 外部附件。外链文件保留 `attachments:` 后的相对目录并写入同步端 `~/.zot/sync/attachments/`，与服务端附件根目录的绝对路径无关；缺少安全相对路径、基本目录或源文件的单个外链附件会作为 unavailable 条目输出 warning 并跳过，不回退到 attachment key 镜像，也不阻断其他内容。同步不会改写 SQLite。本地已有的异常附件旧副本会继续保留为 stale。远端删除不会传播为本地附件或全文缓存删除。同步过程按阶段输出整体文件数、实时字节、百分比、速度和 ETA，中断续传的已有字节会计入进度；这些信息写入 stderr，`--json` 的 stdout 仍只包含最终 canonical envelope。同步后可直接用 `zot --mode local ...`，未显式配置 `data_dir` 时会自动识别 `~/.zot/sync`，并从该镜像的 `storage/`、`attachments/` 和 `.zotero_cli/fulltext/` 读取；显式 `data_dir` 必须是绝对路径。`sync status` 执行 SQLite 快速检查并显示上次同步状态；`sync status --full` 额外执行完整 SQLite integrity check，并核对上次成功同步 manifest 中的所有可用文件，包括 `attachments/` 外链附件。

帮助页与 completion 是纯文本产物，不支持 `--json`。`config init --json` 为非交互模式，调用方必须一次提供当前模式所需的完整参数。

## 兼容边界

旧 alias、旧参数翻译和 redirect-only 入口已经移除；旧命令返回 usage error。脚本应直接迁移到 canonical 命令树。详细清单见 [回退与历史兼容](../architecture/fallbacks.md)。

仅 `find`、`show`、`export` 是正式快捷入口，分别等价于 `item find`、`item show`、`item export`，并且只接受 canonical 参数。
