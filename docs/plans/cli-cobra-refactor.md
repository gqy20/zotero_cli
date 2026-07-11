# CLI v2：Cobra 与应用层重构计划

> 状态：拟实施
> 范围：`cmd/zot`、`internal/cli`，以及从 CLI handler 中抽出的应用编排层
> 不包含：新增 Zotero 业务能力、重写 backend、改变 Web/remote API 协议

## 1. 背景

当前 CLI 已经超过手写命令分发和局部参数解析适合承载的规模：

- 46 个顶层命令
- `internal/cli` 约 10,776 行生产代码、8,338 行测试
- 85 个 `runXxx` 方法
- `commandRegistry` 与 `dispatch` 分别维护展示和执行
- 大量命令自行解析 `--json`、`--limit`、`--workers`、`--force` 等重复参数
- handler 同时承担参数解析、依赖加载、业务编排、输出渲染和错误处理
- 读写语义在部分命令中混合，例如 `relate --add/--remove`、`annotations --clear`
- Zotero API、backend 模式和缓存实现细节大量暴露在用户命令面

结果是用户需要记住大量不规则命令，开发者也无法通过已有命令推导新命令的参数和行为。继续增加顶层命令、包装旧 handler 或添加 alias 只能缓解表面问题，不能降低系统复杂度。

本计划以 Cobra 替换手写命令解析层，同时建立 canonical invocation、应用服务和统一 renderer。目标不是把现有命令逐个改写成 `cobra.Command`，而是重建 CLI 与业务逻辑之间的边界。

---

## 2. 目标与非目标

### 2.1 目标

1. 将用户命令收敛为稳定、可预测的“资源 + 动作”语法。
2. 使用 Cobra 统一命令树、全局 flags、参数校验、帮助和 completion。
3. 所有新旧语法都先转换为 canonical invocation，再进入应用层。
4. 将 backend/store/client 生命周期和业务编排移出 CLI handler。
5. 文本、JSON、错误、warning 和 exit code 由统一输出层处理。
6. 每迁移一个领域，同时删除对应的旧 parser 和重复 renderer。
7. 保持 backend 行为、fallback、安全门控和 JSON 数据结构的可验证兼容。
8. 让新增一个常规资源动作只需要声明命令、定义 request 并调用应用服务。

### 2.2 非目标

- 不在本次重构中新增 Zotero API 功能。
- 不重写 `internal/backend`、`internal/references` 或 PDF 算法。
- 不以 Cobra handler 取代旧 handler 后就宣布完成。
- 不引入 Viper；继续使用现有 `internal/config` 和 `~/.zot/.env`。
- 不使用 `cobra-cli` 生成器或 package 级全局 command/flag 变量。
- 不强求所有资源拥有没有实际意义的完整 CRUD。
- 不在迁移过程中静默改变写入权限、删除门控或 fallback 语义。

---

## 3. 核心设计决策

### 3.1 使用 Cobra，但限制在 adapter 层

固定引入：

```text
github.com/spf13/cobra v1.10.2
```

Cobra 负责：

- 命令树和子命令
- persistent/local flags
- 位置参数数量校验
- required、mutually exclusive 和 required-together 约束
- help、usage、suggestion、completion
- 将已解析参数转换为 canonical request

Cobra 不负责：

- 打开 SQLite 或 reference store
- 创建 Zotero/NCBI/Europe PMC client
- fallback 决策
- 权限和版本冲突策略
- 拼装 JSON 业务响应
- 直接执行业务算法

### 3.2 单向执行流水线

```text
argv
  │
  ├─ Canonical Cobra tree ─┐
  │                        ├─→ Invocation → Application Service → Result → Renderer
  └─ Legacy translator ────┘
```

不得出现以下反向依赖：

- `internal/app` import Cobra
- backend 返回 CLI 专用 response
- renderer 打开配置、数据库或网络 client
- legacy translator 直接执行旧业务 handler

### 3.3 Canonical invocation

所有入口转换成统一调用描述：

```go
type Invocation struct {
    Path    CommandPath
    Keys    []string
    Query   string
    Input   Input
    Filters Filters
    Output  OutputOptions
    Safety  SafetyOptions
    Runtime RuntimeOptions
}

type CommandPath struct {
    Resource string
    Action   string
}
```

业务服务可以使用更具体的 request 类型；`Invocation` 是 CLI adapter 与应用 dispatcher 之间的边界，不应演化成存放任意 `map[string]any` 的万能对象。

### 3.4 统一结果与输出

应用服务返回：

```go
type Result struct {
    Data     any
    Meta     ResultMeta
    Warnings []Warning
}
```

统一 renderer 负责：

- JSON envelope：`ok`、`command`、`data`、`meta`
- 文本视图
- warning 到 stderr
- JSON error envelope
- exit code
- quiet/verbose/color 策略

业务服务不得调用 `fmt.Fprint` 或 `writeJSON`。

### 3.5 Canonical command 名称

JSON `command` 使用 canonical path，例如：

```json
{"command":"item show"}
{"command":"ref build"}
{"command":"ann delete"}
```

迁移期如需保持旧 agent 的严格字段断言，可临时增加：

```json
{"meta":{"legacy_command":"show"}}
```

最终只承诺 canonical `command`。具体切换版本必须在阶段 0 冻结，并记录于 CHANGELOG。

---

## 4. 命名预算与 Canonical 命令树

以下是计划基线。阶段 0 可以调整命名，但进入阶段 1 后不得无 RFC 变更随意改动。

### 4.1 命名预算

CLI v2 不采用“每个 token 最多 5 个字符”的机械限制，而采用以下预算：

1. 高频资源名和动作优先不超过 5 个字符。
2. 常见完整词原则上不超过 7 个字符。
3. `zot` 后的常用固定命令路径原则上不超过 15 个字符。
4. 常用命令不超过三层；业务参数、key、查询文本和路径不计入固定路径。
5. 清晰度和安全性优先于长度；破坏性动作不得使用不透明缩写。
6. 低频、领域精确术语可以超过预算，不为形式整齐制造黑话。

推荐资源词汇：

```text
lib item coll note tag search group file pdf ann ref index config
```

推荐通用动作词汇：

```text
list show find new edit delete init
add remove build status open check sync
```

`delete`、`remove`、`status` 虽超过 5 个字符，仍保留完整形式：

- `delete` 明确表示删除对象或数据，不能缩写为含义较弱的 `rm`。
- `remove` 表示解除成员或关系，与 `delete` 有意区分。
- `status` 避免与统计语义 `stats` 混淆。

禁止为了长度使用难以独立理解的缩写，例如 `stat`、`upd`、`crt`、`rmv`。允许已经形成稳定领域含义的短词，例如 `ref`、`ann`、`coll`、`ctx`、`figs`。

长度约束针对用户命令面，不要求内部 Go 类型、文件名或 JSON 数据字段同步缩写。内部仍使用 `ReferenceService`、`CollectionService` 等完整名称。

### 4.2 Canonical 主名称与 alias

每个命令只有一个 canonical path。主帮助、主文档和 JSON `command` 只展示 canonical path。完整长名可以作为 alias 或 legacy 入口存在，但不得与短名并列成为两套平等语法。

示例：

```text
canonical: zot ref ctx ITEMKEY
alias:     zot reference contexts ITEMKEY
legacy:    zot ref contexts ITEMKEY
JSON:      {"command":"ref ctx"}
```

Cobra help 可以在详细页列出 alias，根帮助只展示 canonical 名。Shell completion 应优先补全 canonical 名。

### 4.3 子命令语法

Canonical 命令遵循：

```text
zot <resource> <action> [object...] [flags]
```

层级规则：

1. `resource` 后必须是动作，不能一部分放动作、一部分放资源名。
2. 常规命令固定为两级：`resource action`。
3. action 下不再创建 action 子命令；范围和模式使用 typed flags 表达。
4. 禁止 `ref contexts build`、`config auth login refresh` 这类动词继续嵌套的路径。
5. 只有帮助主题和 completion 可以反映命令层级，本身不形成业务调用语法。
6. 领域专用动作必须在该领域内具有稳定、单一的含义，并在资源帮助页单独列出。

`version` 和 `completion` 是 CLI 自身的根级工具，不属于 Zotero 业务资源，允许作为单层例外。其他业务命令必须有资源归属。

通用动作的参数位置固定：

| 动作 | 固定形式 | 固定语义 |
|---|---|---|
| `list` | `<resource> list [filters]` | 列出零到多个对象，不要求 key |
| `show` | `<resource> show [key]` | 查看一个明确对象；集合资源接受一个 key，`lib/config` 等单例资源不要求 key |
| `find` | `<resource> find <query> [filters]` | 按内容搜索，query 使用第一个位置参数 |
| `new` | `<resource> new [input]` | 创建对象，不接受已有对象 key |
| `edit` | `<resource> edit <key> [input]` | 修改一个已有对象 |
| `delete` | `<resource> delete <key...>` | 删除一个或多个对象，应用统一安全策略 |
| `add` | `<resource> add <source-key> <target-key...>` | 向容器或关系增加成员 |
| `remove` | `<resource> remove <source-key> <target-key...>` | 解除成员或关系，不删除目标对象 |
| `build` | `<resource> build [scope flags]` | 构建可重建派生数据 |
| `status` | `<resource> status` | 查看状态，不修改数据 |
| `check` | `<resource> check [target]` | 校验配置、文件或依赖，不修改目标 |

各资源只选择实际支持的动作，不为矩阵整齐制造空命令。领域专用动作如 `pdf text`、`ref cited` 可以存在，但不得改变通用动作的既定语义。

### 4.4 Canonical 命令树

```text
zot
├── lib
│   ├── show
│   ├── stats
│   └── log
├── item
│   ├── list
│   ├── find
│   ├── show
│   ├── new
│   ├── edit
│   ├── delete
│   ├── tag
│   ├── untag
│   ├── supp
│   └── export
├── coll
│   ├── list
│   ├── show
│   ├── new
│   ├── edit
│   ├── delete
│   ├── add
│   └── remove
├── note
│   ├── list
│   ├── find
│   ├── show
│   ├── new
│   ├── edit
│   └── delete
├── tag
│   └── list
├── search
│   ├── list
│   ├── show
│   ├── new
│   ├── edit
│   └── delete
├── group
│   └── list
├── file
│   ├── show
│   └── check
├── pdf
│   ├── text
│   ├── figs
│   └── open
├── ann
│   ├── list
│   ├── new
│   └── delete
├── ref
│   ├── show
│   ├── find
│   ├── related
│   ├── cited
│   ├── ctx
│   ├── links
│   ├── entities
│   ├── profile
│   ├── build
│   ├── resolve
│   └── status
├── index
│   ├── build
│   └── status
├── schema
│   ├── list
│   └── show
├── config
│   ├── init
│   ├── show
│   └── check
├── server
│   └── start
├── sync
│   └── pull
├── completion
└── version
```

`search` 在 canonical tree 中专指 Zotero saved search；文献检索固定为 `item find`，引用检索固定为 `ref find`。不得新增裸 `zot search QUERY`，以免重新引入歧义。

`lib log` 统一承载旧 `changes` 和 `deleted` 的版本/删除记录查询，通过明确 filters 选择范围；具体 flag 在阶段 0 冻结。

旧 `trash` 和 `publications` 不再作为伪动作挂在 `lib` 下，统一为 item 列表范围：

```text
zot item list --scope trash
zot item list --scope pubs
```

群组是可列出的真实资源，使用 `group list`，不再使用 `lib groups`。当前没有稳定的单群组详情能力，因此不为矩阵整齐虚构 `group show`。

附件是独立于 PDF 的真实资源。表格预览和附件健康检查分别使用 `file show`、`file check`；`pdf` 只保留确实要求 PDF 的 `text`、`figs`、`open`。

补充材料发现以文献条目为起点，归入 `item supp`。导出对象也是文献条目集合，canonical 路径为 `item export`；高频旧入口 `zot export` 可以像 `find/show` 一样保留为正式快捷入口，但不能拥有独立实现。

Schema 固定使用 `list/show` 动作，schema 类别和可选 item type 使用位置参数，不再为同一动作创建 `*-for` 变体：

```text
zot schema list types
zot schema list fields
zot schema list fields article
zot schema list roles
zot schema list roles article
zot schema show book
```

上述 `types|fields|roles` 是 `schema list` 的位置参数，不是继续嵌套的 Cobra 子命令。`schema show <item-type>` 返回用于创建该类型对象的模板。

配置路径并入 `config show --path`，不再把 `path` 作为不规则动作。普通 `config show` 返回已脱敏的有效配置；`--path` 只返回配置文件路径。

### 4.5 资源动作矩阵

用户学会一个通用动作后，应能推断它在其他资源上的语义。空白表示该资源没有对应能力，而不是换用另一个同义动作。

| 资源 | list | show | find | new | edit | delete | 领域动作 |
|---|---:|---:|---:|---:|---:|---:|---|
| `lib` |  | ✓ |  |  |  |  | `stats`、`log` |
| `item` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | `tag`、`untag`、`supp`、`export` |
| `coll` | ✓ | ✓ |  | ✓ | ✓ | ✓ | `add`、`remove` |
| `note` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| `tag` | ✓ |  |  |  |  |  |  |
| `search` | ✓ | ✓ |  | ✓ | ✓ | ✓ |  |
| `group` | ✓ |  |  |  |  |  |  |
| `file` |  | ✓ |  |  |  |  | `check` |
| `pdf` |  |  |  |  |  |  | `text`、`figs`、`open` |
| `ann` | ✓ |  |  | ✓ |  | ✓ |  |
| `ref` |  | ✓ | ✓ |  |  |  | `related`、`cited`、`ctx`、`links`、`entities`、`profile`、`build`、`resolve`、`status` |
| `index` |  |  |  |  |  |  | `build`、`status` |
| `schema` | ✓ | ✓ |  |  |  |  |  |
| `config` |  | ✓ |  |  |  |  | `init`、`check` |
| `server` |  |  |  |  |  |  | `start` |
| `sync` |  |  |  |  |  |  | `pull` |

一致性规则：

- 同一动作在不同资源上保持相同参数顺序和风险等级。
- 同一语义不再出现 `get/view/show`、`create/new`、`update/edit`、`delete/clear` 等多套词汇。
- `new/edit/delete` 仅用于有身份和生命周期的对象；关联成员使用 `add/remove`。
- `build/status/check` 仅用于派生数据、运行状态或校验，不作为 CRUD 同义词。
- 领域动作不能偷偷承担另一个通用动作，例如 `ref profile` 只读取画像，不能顺带刷新索引；刷新应使用明确 flag。

代表性常用调用应保持紧凑且可直接读懂：

```text
zot lib show
zot item find CRISPR
zot item show ABCD
zot coll list
zot note new --parent ABCD
zot file check ATTACHKEY
zot pdf text ABCD
zot pdf figs ABCD
zot ann new ABCD --text "target"
zot ref ctx ABCD
zot ref build --failed
```

### 4.6 高频快捷入口

`find` 和 `show` 可以作为刻意设计的高频短入口保留，但必须映射到 canonical invocation：

```text
zot find ...  == zot item find ...
zot show KEY  == zot item show KEY
zot export ... == zot item export ...
```

它们不是独立 handler，也不拥有独立参数语义。

### 4.7 Reference 构建范围

不再将范围表达为不规则动作：

```text
ref retry
ref contexts build
```

统一为：

```text
ref build
ref build --failed
ref build --contexts
ref build --force
```

`--failed`、`--contexts` 应互斥，除非应用层明确实现组合语义。

### 4.8 标注风险边界

读取、创建和删除从命令路径上区分：

```text
ann list
ann new
ann delete
```

不在 canonical 命令中保留 `annotations --clear` 或 `annotate --clear`。

---

## 5. 统一参数规范

### 5.1 根级 persistent flags

```text
--format text|json
--json                 --format json 的快捷方式
--quiet
--verbose / -v
--no-color
--mode MODE            可选的单次调用覆盖
--timeout DURATION     可选的单次调用覆盖
```

默认输出可由 `ZOT_OUTPUT=text|json` 设置。子命令不得重复声明或自行解析 `--json`。

### 5.2 列表与分页

```text
--limit N
--offset N
--sort FIELD
--order asc|desc
```

迁移时将旧 `--start` 翻译为 canonical `offset`；canonical 层只保留一个名称。

### 5.3 外部枚举值与短 alias

命令路径的长度预算不直接适用于 Zotero schema 标识、item key、文件路径和用户查询文本。例如 `journalArticle` 是 Zotero 的官方 item type，不是 CLI 子命令，内部 domain、API 请求和 JSON 输出必须继续使用官方值。

但官方 camelCase 枚举仍会增加输入和记忆成本。CLI v2 为高频外部枚举提供短、稳定的输入 alias：

| CLI 输入 | Zotero canonical 值 |
|---|---|
| `article` | `journalArticle` |
| `chapter` | `bookSection` |
| `conf` | `conferencePaper` |
| `web` | `webpage` |
| `blog` | `blogPost` |

alias 表以实际 Zotero schema 为准，阶段 0 冻结完整清单。不得为不明确的自然语言建立 alias，例如 `paper` 可能表示 journal article、conference paper、preprint 或普通 document。

统一规则：

1. 所有官方 Zotero 值始终可作为输入，保证脚本和 API 用户无需翻译。
2. CLI alias 使用小写短词，原则上不超过 8 个字符；不使用难懂首字母缩写。
3. alias 只存在于输入归一化层，进入 application service 前转换为官方 canonical 值。
4. JSON、domain model、Web/remote API 和持久化数据始终输出官方值，例如 `journalArticle`。
5. help 和 completion 优先展示短 alias，并在说明中同时显示官方值。
6. unknown alias 返回可操作错误和候选建议，不静默猜测。
7. 同一 alias 在所有命令中含义一致，例如 `--type article` 和 `schema list fields article` 使用同一 registry。

示例：

```text
zot item list --type article
zot item find CRISPR --type article
zot schema list fields article
```

三条命令在解析后都使用：

```text
itemType = journalArticle
```

建议建立单一归一化入口：

```go
type EnumRegistry interface {
    NormalizeItemType(value string) (canonical string, err error)
    ItemTypeAliases() []EnumAlias
}
```

不得由 `item find`、`schema list`、`item new` 分别维护 alias 表。

### 5.4 对象与批量输入

- 单个和多个 key 优先使用位置参数。
- 批量文件统一为 `--from PATH`。
- `--from -` 表示 stdin。
- 旧 `--items KEY1,KEY2`、`--item-key`、`--from-file` 仅由 legacy translator 接受。

示例：

```text
zot item delete KEY1 KEY2
zot item tag KEY1 KEY2 --tag review
zot pdf text KEY1 KEY2
zot item delete --from keys.txt
```

### 5.5 写入数据

```text
--set FIELD=VALUE       简单字段，可重复
--data JSON             完整 JSON
--from PATH             JSON 或批量输入文件
```

简单资源应提供符合用户意图的 flags：

```text
zot coll new --name "Reviews"
zot note new --parent KEY --text "Reading note"
```

用户不应为了常规操作了解 `itemType=note` 等底层结构。

### 5.6 安全参数

```text
--dry-run
--yes / -y
--if-version N
```

`--if-version` 是旧 `--if-unmodified-since-version` 的 canonical 名称。长期目标是普通交互式写入由应用服务读取当前 library version 并自动附加前置条件；调用者显式传 `--if-version` 时严格使用该值。自动获取不得演化为冲突后的静默覆盖或无界重试。

---

## 6. 目标代码结构

```text
cmd/zot/
└── main.go

internal/cli/
├── root.go
├── streams.go
├── global_options.go
├── errors.go
├── completion.go
├── item.go
├── collection.go
├── note.go
├── saved_search.go
├── pdf.go
├── annotation.go
├── reference.go
├── library.go
├── index.go
├── config.go
├── schema.go
├── export.go
├── server.go
└── legacy/
    ├── root.go
    ├── item.go
    ├── pdf.go
    ├── reference.go
    └── write.go

internal/app/
├── app.go
├── invocation.go
├── result.go
├── items.go
├── collections.go
├── notes.go
├── saved_searches.go
├── pdf.go
├── annotations.go
├── references.go
├── library.go
├── index.go
├── inputs.go
└── safety.go

internal/render/
├── renderer.go
├── json.go
├── text.go
├── warnings.go
└── errors.go
```

目录可以根据实现反馈微调，但依赖方向不可改变：

```text
cmd → cli → app → backend/references/zoteroapi
               ↘ render（或 cli 持有 render 接口）
```

`backend`、`references` 和 `zoteroapi` 不得 import `cli`、Cobra 或 render。

---

## 7. 七阶段实施计划

### 阶段 0：冻结 CLI v2 契约

交付：

- 审定本文件的 canonical 命令树
- 审定全局和公共 flags
- 完成旧命令 → canonical command 映射表
- 决定 JSON `command` 切换策略和版本
- 记录废弃周期
- 为代表性命令编写 golden invocation 样例

代码量：

| 类型 | 规模 |
|---|---:|
| 生产代码 | 0 |
| 契约测试样例 | 100–200 行 |
| 文档 | 300–500 行 |

退出条件：命令树和参数规范获确认；阶段 1 后变更必须更新本计划并说明迁移影响。

### 阶段 1：Cobra、Invocation 与 Renderer 基础设施（已完成）

交付：

- 引入 Cobra v1.10.2
- `NewRoot`、`ExecuteContext` 和 streams 注入
- root persistent flags
- canonical `Invocation`、`Result`、typed error
- JSON/text/error renderer
- exit code 映射
- help 不加载配置或网络
- 迁移 `version`、`config init/show/check` 验证基础设施

代码量：

| 类型 | 规模 |
|---|---:|
| 新增/重写生产代码 | 350–550 行 |
| 测试 | 250–400 行 |
| 删除旧代码 | 100–200 行 |
| 净增加 | 250–450 行 |

退出条件：新 command tree 可在同一进程重复构建；JSON 错误稳定；现有配置测试通过。

### 阶段 2：只读资源垂直切片（已完成）

迁移：

- `lib show/stats/log`
- `item list --scope trash|pubs`
- `coll list`
- `tag list`
- `note list`
- `search list`
- `group list`

每个命令必须贯通：

```text
Cobra → typed request → app service → Result → renderer
```

旧入口必须通过 legacy translator 进入相同 typed request。

| 类型 | 规模 |
|---|---:|
| 新生产代码 | 450–700 行 |
| 新测试 | 500–750 行 |
| 删除旧生产代码 | 300–500 行 |
| 删除重复测试 | 100–200 行 |

退出条件：对应旧 handler 不再负责参数解析、依赖加载或渲染；新旧入口业务结果等价。

### 阶段 3：Item、Collection、Note 与写操作

迁移：

- item list/find/show/new/edit/delete/tag/untag
- item supp/export
- coll show/new/edit/delete/add/remove
- note show/find/new/edit/delete
- search show/new/edit/delete
- 高频 `find`、`show` 快捷入口

同时完成：

- typed filters
- `--set`、`--data`、`--from`
- 多 key 位置参数
- SafetyOptions
- `--dry-run`、`--yes`、`--if-version`
- 应用层写入门控和版本前置条件

| 类型 | 规模 |
|---|---:|
| 新生产代码 | 650–1,000 行 |
| 新测试 | 700–1,100 行 |
| 删除旧生产代码 | 500–800 行 |
| 删除重复测试 | 200–350 行 |

退出条件：不存在 canonical 写操作绕过 `ZOT_ALLOW_WRITE`/`ZOT_ALLOW_DELETE`；冲突测试和确认测试覆盖新旧入口。

### 阶段 4：PDF 与 Annotation

迁移：

- file show/check
- pdf text/figs/open
- ann list/new/delete
- local/hybrid/remote 路由
- 批量 worker、page range、grep 和输出目录参数

旧 `annotations --clear`、`annotate --clear` 仅由 legacy translator 接受，canonical 层必须使用显式 `ann delete` action。

| 类型 | 规模 |
|---|---:|
| 新生产代码 | 450–750 行 |
| 新测试 | 550–850 行 |
| 删除旧生产代码 | 400–700 行 |
| 删除重复测试 | 150–250 行 |

退出条件：PDF 算法代码不依赖 Cobra；`ann delete` 的服务端门控与本地安全语义保持一致。

### 阶段 5：Reference 与 Index

迁移：

- ref show/find/related/cited/ctx/links/entities/profile
- ref build/resolve/status
- index build/status
- `build --failed`、`build --contexts` 范围语义

ReferenceService 统一持有 reader、store、NCBI/Europe PMC client、builder 和 resolver 生命周期。

| 类型 | 规模 |
|---|---:|
| 新生产代码 | 550–900 行 |
| 新测试 | 650–1,000 行 |
| 删除旧生产代码 | 450–750 行 |
| 删除重复测试 | 150–300 行 |

退出条件：`ref retry`、`ref contexts build` 和 `ref KEY` 只存在于 legacy translator；默认 PMC/PubMed/Europe PMC/GROBID 路由不变。

### 阶段 6：长尾命令与运行时能力

迁移：

- export
- supplements/attachment inspect 旧入口的 legacy 映射
- schema
- config init
- server start
- sync pull
- completion
- version

| 类型 | 规模 |
|---|---:|
| 新生产代码 | 400–650 行 |
| 新测试 | 450–700 行 |
| 删除旧生产代码 | 350–600 行 |
| 删除重复测试 | 100–200 行 |

退出条件：所有公开命令已进入 Cobra tree 或被明确列为 legacy-only；PowerShell/bash/zsh/fish completion 可生成。

### 阶段 7：删除旧 CLI 内核并完成迁移

删除：

- `commandRegistry`
- 手写 `dispatch`
- 可由 Cobra 生成的 usage 常量
- `isHelpOnly`、`containsHelp`
- `parseJSONOnlyArgs`、`parseJSONAndLimitArgs`、`parseSingleValueCommand`
- handler 内重复 `case "--json"`
- handler 内 JSON/text 分支
- 已迁移的旧业务 handler
- 重复的 config/client/store 生命周期代码

保留的 legacy 层只能做：

- 识别旧命令
- 解析旧参数
- 输出 deprecation warning
- 转换 canonical request

同步更新 README、quickstart、commands、references、setup guide、skill、CHANGELOG 和 [回退与历史兼容](../architecture/fallbacks.md)。

| 类型 | 规模 |
|---|---:|
| Legacy translator | 250–450 行 |
| Legacy 测试 | 350–550 行 |
| 删除旧生产代码 | 750–1,250 行 |
| 删除重复测试 | 200–400 行 |
| 文档 | 400–700 行 |

退出条件：旧 handler 不再承载业务；compatibility inventory 与实际 translator 一致；CLI v2 成为唯一主文档语法。

---

## 8. 总代码量预算

| 类型 | 预计规模 |
|---|---:|
| 新增/重写生产代码 | 3,050–5,000 行 |
| 新增/重写测试 | 3,450–5,350 行 |
| 删除旧生产代码 | 2,850–4,800 行 |
| 删除重复测试 | 900–1,600 行 |
| 文档变更 | 700–1,200 行 |
| 总 churn | 约 11,000–17,000 行 |
| 最终生产代码净变化 | 约 -500 至 +1,500 行 |

代码量是约束而不是目标。如果某一阶段新增大量 facade、adapter 和旧 handler，却没有同步删除 parser/renderer，应停止并重新审查边界。

---

## 9. 测试策略

### 9.1 分层测试

| 层 | 测试重点 |
|---|---|
| Cobra command | path、位置参数、flags、help、互斥/必填校验 |
| Legacy translator | 旧 argv → canonical request 的精确映射 |
| Application service | backend 选择、store/client 生命周期、fallback、安全策略 |
| Renderer | JSON envelope、文本、warnings、errors、exit code |
| End-to-end | 代表性新旧命令结果等价 |

### 9.2 每个迁移命令的最小矩阵

1. canonical 成功路径
2. help 不加载依赖
3. 缺参数和未知 flag
4. text 与 JSON
5. config/backend 错误
6. legacy 等价路径
7. 写操作额外覆盖 dry-run、门控、确认、版本冲突
8. 有 fallback 的命令覆盖首选成功、允许回退、禁止回退、目标失败

### 9.3 Golden tests

对根帮助、主要资源帮助、JSON success/error envelope 和 legacy warning 使用少量 golden tests。避免为所有文案建立脆弱 snapshot；业务测试断言结构和行为，不断言完整帮助文本。

---

## 10. 兼容与废弃策略

迁移遵守 [回退与历史兼容](../architecture/fallbacks.md) 的生命周期规则。

### 10.1 兼容等级

| 等级 | 行为 |
|---|---|
| canonical | 主帮助和主文档展示；长期支持 |
| deprecated alias | 可调用；输出一次迁移 warning；不进入主帮助 |
| redirect-only | 不执行旧行为，只提示替代命令 |
| removed | 返回 unknown command，并在 CHANGELOG 有删除记录 |

### 10.2 迁移周期

- canonical 命令发布时，旧入口至少保留一个明确版本周期。
- Agent 高频旧命令可保留更长，但必须走 translator。
- destructive 旧语法不得因兼容而绕过 canonical safety policy。
- legacy warning 写 stderr，不能污染 stdout JSON。
- 删除前统计文档、skill、测试和示例中的旧命令引用。

---

## 11. 风险与控制

| 风险 | 控制措施 |
|---|---|
| 只是把旧命令包装进 Cobra | 每阶段强制删除旧 parser 和 renderer；禁止 `RunE: runOldCommand` 作为完成状态 |
| JSON `command` 破坏 agent | 阶段 0 冻结切换策略；提供兼容元数据或版本周期；增加 contract tests |
| Cobra 默认错误污染 JSON | root 设置 `SilenceErrors`、`SilenceUsage`，统一外层 renderer |
| package 全局 flag 导致测试泄漏 | 全部 command 使用构造函数；每次测试创建新 root |
| help 触发配置或网络 | 不在 init/command construction 加载依赖；仅 RunE 内调用应用服务 |
| 写操作安全回归 | SafetyService 单点门控；canonical 与 legacy 共用；破坏性测试矩阵 |
| 迁移期代码膨胀 | 领域按垂直切片迁移；旧 handler 在同阶段删除；监控净行数和重复 parser |
| 资源语法过度形式化 | 高频 `find`/`show` 保留正式快捷入口；只为真实能力定义 action |
| Cobra 渗入业务层 | CI/静态检查或架构测试禁止 `internal/app`、backend import Cobra |

---

## 12. 阶段完成定义

一个领域只有同时满足以下条件才算迁移完成：

1. canonical 命令已进入 Cobra tree。
2. 参数被解析为 typed request。
3. 业务编排位于应用服务，而非 Cobra RunE。
4. JSON/text/error 由统一 renderer 输出。
5. 旧命令只通过 legacy translator 转换 request。
6. 对应旧 parser 和重复输出代码已删除。
7. 新旧入口等价测试通过。
8. fallback 和安全测试通过。
9. 用户文档、skill 和兼容清单同步更新。
10. `go test ./...`、`go vet ./...` 和格式检查通过。

以下形式明确不算完成：

```go
RunE: func(cmd *cobra.Command, args []string) error {
    return exitCodeToError(oldCLI.runExtractText(args))
}
```

---

## 13. 首轮实施边界

第一轮只执行阶段 0、1、2，用于验证架构：

| 类型 | 预计规模 |
|---|---:|
| 新生产代码 | 800–1,250 行 |
| 测试 | 750–1,150 行 |
| 删除旧代码 | 400–700 行 |
| 文档 | 400–650 行 |
| 总 churn | 约 2,350–3,750 行 |

首轮结束后进行一次设计复盘，回答：

1. 新增常规 list/show 动作是否明显减少 parser 和 renderer 重复？
2. legacy translator 是否保持为纯转换层？
3. application service 是否可被 CLI 之外的 server/Web 复用？
4. JSON 和错误契约是否更稳定？
5. 新 root 是否改善 help，而没有暴露全部长尾复杂度？
6. 继续迁移 item/write 前是否需要调整 canonical grammar？

只有复盘通过，才进入阶段 3。
