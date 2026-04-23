# Zotero CLI 详细参考

> 本文件为 SKILL.md 的补充参考，包含决策树、陷阱规避、默认行为、JSON 格式规范和模式差异表。
> SKILL.md 中的「详细参考」链接指向此处。

## Agent 决策树

```
用户想要…？
│
├─ 了解库整体情况 ──────→ zot overview --json          # 首选入口：统计+Top条目+FTS状态
│
├─ 搜索/筛选文献 ───────→ zot find [查询词] [过滤条件] --json
│   ├─ 有具体关键词？    → 加查询词（支持中英文）
│   ├─ 只有过滤条件？    → 直接加 --tag/--collection/--date-after 等（无需 --all）
│   ├─ 要全量？         → --all
│   └─ 要全文检索？     → local/hybrid 下有 FTS 索引时自动启用，无需额外标志
│
├─ 查看单条目详情 ─────→ zot show ITEMKEY --json       # 含子笔记+标注+期刊等级
│
├─ 导出文献数据 ───────→ zot export --[all|collection KEY|item-key KEY] --format FORMAT --json
│   ├─ 引文管理器导入   → --format bibtex 或 biblatex
│   ├─ 程序化处理       → --format csljson
│   └─ 其他工具         → --format ris
│
├─ 生成引用/参考文献 ──→ zot cite ITEMKEY --format citation|bib [--style STYLE]
│   ├─ 正文引用         → --format citation（默认）
│   └─ 参考文献列表     → --format bib
│
├─ 管理条目关系 ───────→ zot relate ITEMKEY [选项] --json
│   ├─ 基础查询         → 默认（显式关系）
│   ├─ 含笔记+引用      → --aggregate（local/hybrid）
│   ├─ 可视化           → --dot > output.dot
│   └─ 写入关系         → --add / --remove / --from-file（需写权限）
│
├─ 操作 PDF ───────────→ （需 local/hybrid + PyMuPDF）
│   ├─ 提取正文         → zot extract-text ITEMKEY --json
│   ├─ 提取图表         → zot extract-figures ITEMKEY [-o DIR] [-w N]
│   ├─ 读取标注         → zot annotations ITEMKEY [--type|--page|--author] --json
│   └─ 写入标注         → zot annotate ITEMKEY --page N --text "关键词" --color red
│                          （推荐 Mode 1.5，避免全文误匹配）
│
├─ 写入/修改数据 ──────→ （需 ZOT_ALLOW_WRITE=1）
│   ├─ 创建条目/笔记    → zot create-item --data 'JSON' --json
│   ├─ 更新条目         → zot update-item KEY --data 'JSON' --json
│   ├─ 批量打标签       → zot add-tag --items KEYS --tag TAG --json
│   └─ 创建收藏夹       → zot create-collection --data 'JSON' --json
│
├─ 删除数据 ───────────→ （需 ZOT_ALLOW_DELETE=1 + 确认）
│   └─ delete-item / delete-collection / delete-search
│
└─ 其他只读操作 ───────→ collections / tags / notes / searches / groups /
                         trash / schema types / versions / key-info / index build
```

## 常见陷阱

| 陷阱 | 正确做法 | 原因 |
|------|----------|------|
| `zot cite KEY --format apa` | `zot cite KEY --style apa` | `--format` 只接受 `citation`\|`bib`；样式用 `--style` |
| `zot cite KEY --format bibliography` | `zot cite KEY --format bib` | 不存在 `bibliography` 值，正确值为 `bib` |
| `zot annotate ... --dry-run` | 先 `extract-text` 确认文本位置，再直接执行 | annotate **不支持** `--dry-run`，见 roadmap P0 |
| `zot index status` | `zot index build [--force]` | 不存在 `status` 子命令，只有 `build` |
| `zot find --json`（无查询词无过滤） | 加 `--all` 或至少一个过滤条件 | 无查询词+无过滤会报错，防止意外返回全库 |
| `zot versions --since 100` | `zot versions items --since 100` | `versions` 必须指定类型：`collections\|searches\|items\|items-top` |
| 搜索结果含不相关条目 | FTS 自动启用后搜索范围含 PDF 全文正文 | 非仅元数据匹配。纯元数据搜索临时设 `ZOT_MODE=web` |
| 使用旧版二进制测试新功能 | `go build -o zot.exe ./cmd/zot` 后用 `./zot.exe` | PATH 中可能是旧版本（如 v0.0.7），缺少后续新增功能 |
| 写操作未设置环境变量 | 确保 `ZOT_ALLOW_WRITE=1` / `ZOT_ALLOW_DELETE=1` | 所有写操作和删除操作会检查这些变量 |
| DB 层标注删除失败 | 关闭 Zotero 后重试 | `--clear` 双层删除中 DB 层需要 Zotero 未运行，PDF 层不受限 |

## 默认行为速查

| 维度 | 默认值 | 说明 |
|------|--------|------|
| **模式** | `hybrid` | 本地优先 + Web 回退，兼顾速度与完整性 |
| **输出格式** | 文本 | Agent 工作流应**始终加 `--json`** |
| **引文样式** | `apa` | 通过 `--style` 指定，不影响 `--format` |
| **引文格式** | `citation` | `--format` 的默认值，生成正文内引用 |
| **搜索推断** | 自动 | 有过滤标志时自动设 `All=true`，无需手动 `--all` |
| **FTS 全文检索** | 自动启用 | local/hybrid 下 `{dataDir}/.zotero_cli/fulltext/index.sqlite` > 4KB 时 |
| **Snippet 条数** | 50 | `--snippet` 默认上限，更多结果需显式 `--limit` |
| **写权限** | 关闭 | 需 `ZOT_ALLOW_WRITE=1`，否则拒绝写入 |
| **删除权限** | 关闭 | 需 `ZOT_ALLOW_DELETE=1`，且交互模式需确认 |
| **删除确认** | `[y/N]` 提示 | `--yes`/`-y` 跳过；`--json` 模式自动跳过 |
| **快照缓存** | 自动 | hybrid 下 Zotero 运行时使用持久化快照（~0.3s/次） |
| **重试** | 5 次 | 遇 429 等可重试错误自动退避+抖动 |

## 写操作 JSON 输入格式

所有写操作通过 `--data 'JSON'` 或 `--from-file file.json` 传入数据，JSON 结构遵循 [Zotero Web API 规范](https://www.zotero.org/support/dev/web_api/v3/write_item)：

```shell
# 创建条目
zot create-item --data '{"itemType":"journalArticle","title":"...","creators":[{"creatorType":"author","lastName":"Doe","firstName":"Jane"}],"date":"2024-01"}' --if-unmodified-since-version 59186 --json

# 创建笔记（hybrid/local 下 Zotero 未运行时可直写 SQLite ~50ms）
zot create-item --data '{"itemType":"note","parentItem":"ITEMKEY","note":"<p>笔记内容</p>"}' --json

# 更新条目（只需传入要修改的字段）
zot update-item ITEMKEY --data '{"abstractNote":"更新后的摘要"}' --if-unmodified-since-version 59186 --json

# 创建收藏夹
zot create-collection --data '{"name":"新收藏夹","parentCollection":"PARENT_KEY"}' --if-unmodified-since-version 59186 --json

# 批量添加/删除标签
zot add-tag --items KEY1,KEY2 --tag "新标签" --json
zot remove-tag --items KEY1,KEY2 --tag "旧标签" --json

# relate 批量操作（--from-file 格式）
# batch.json 内容：
#   [{"action":"add","source":"KEY_A","target":"KEY_B","predicate":"dc:relation"},
#    {"action":"remove","source":"KEY_A","target":"KEY_C","predicate":"dc:relation"}]
zot relate KEY_A --from-file batch.json --dry-run          # 预览
zot relate KEY_A --from-file batch.json                     # 执行
```

**版本前置条件**：`--if-unmodified-since-version N` 用于乐观并发控制，从最近一次 `overview` 或 `versions items --since N` 获取当前 library version 传入即可省略（不保证原子性）。

## Relate 三层关系模型与模式支持

### 三层模型

| 层 | 来源 | 命令选项 |
|----|------|----------|
| ① 显式关系 | `itemRelations` 表 | 默认查询 |
| ② 子笔记关系 | 子笔记的 `itemRelations` | `--aggregate` 的 Notes 字段 |
| ③ 内嵌 Citation | 笔记 HTML `data-citation-items` | `--aggregate` 的 Citations 字段 |

### 模式支持矩阵

| 功能 | local | hybrid | web |
|------|-------|--------|-----|
| 查询显式关系 | ✅ | ✅ | ✅ |
| `--aggregate` 聚合 | ✅ | ✅ | ❌ |
| `--dot` 可视化 | ✅ | ✅ | ✅ |
| `--add` / `--remove` | ✅ | ✅ | ❌ |
| `--from-file` 批量 | ✅ | ✅ | ❌ |
| `--predicate` 过滤 | ✅ | ✅ | ✅ |

## 模式行为差异速查

| 功能 | web | local | hybrid |
|------|-----|-------|--------|
| **搜索范围** | 元数据 + 远程全文 | 本地元数据 | 元数据；有 FTS 索引时自动扩展至 PDF 全文 |
| **过滤参数** | 基础过滤全部可用 | 全部过滤可用 | 全部过滤可用 |
| **高级过滤** | ❌ 不支持 collection/no-collection/tag-contains 等 | ✅ | ✅（回退到 web 时降级为仅基础过滤） |
| **FTS 全文检索** | ❌ | ✅（需先 `index build`） | ✅（同 local） |
| **PDF 操作** | ❌ | ✅ | ✅ |
| **写操作** | ✅ 全部 | 仅笔记创建 + PDF 标注 | ✅ 全部（笔记可本地直写） |
| **relate 写入** | ❌ | ✅ | ✅ |
| **快照缓存** | — | — | ✅ Zotero 运行时 ~0.3s/次 |

## 环境变量完整速查

存储位置：`~/.zot/.env`

| 变量 | 说明 | 默认 |
|------|------|------|
| `ZOT_MODE` | `web` / `local` / `hybrid` | **`hybrid`** |
| `ZOT_DATA_DIR` | Zotero 数据目录 | — |
| `ZOT_LIBRARY_ID` | 库 ID | — |
| `ZOT_API_KEY` | API 密钥 | — |
| `ZOT_STYLE` | 引文样式 | `apa` |
| `ZOT_TIMEOUT_SECONDS` | API 超时秒数 | `20` |
| `ZOT_ALLOW_WRITE` | 允许写操作 | `1` |
| `ZOT_ALLOW_DELETE` | 允许删除操作 | `0` |
| `ZOT_RETRY_MAX_ATTEMPTS` | 最大重试次数 | `5` |
| `ZOT_RETRY_BASE_DELAY_MS` | 重试基础延迟 ms | `1000` |
| `ZOT_JSON_ERRORS` | 错误以 JSON 输出到 stdout | `0` |

## FTS5 全文检索详解

- **自动启用条件**：local/hybrid 模式下 `{dataDir}/.zotero_cli/fulltext/index.sqlite` 文件大小 > 4096 bytes (4KB)
- **搜索范围变化**：启用后 `find` 命令从纯元数据匹配切换到 **PDF 全文内容** 匹配，结果可能包含正文中提及关键词但主题不直接相关的条目
- **禁用方法**：临时设 `ZOT_MODE=web`，或删除索引目录 `{dataDir}/.zotero_cli/fulltext/` 后重建
- **Snippet**：`--snippet` 显示匹配片段预览，默认限制 **50** 条，更多结果需显式加 `--limit`
- **匹配模式**：默认所有词匹配（AND）；`--fulltext-any` 切换为任一词匹配（OR）

## Hybrid 笔记创建特殊行为

`create-item` 创建笔记时（`itemType: "note"`），若满足以下条件会走本地 SQLite 直写而非 Web API：

1. 当前模式为 `local` 或 `hybrid`
2. Zotero 桌面端**未运行**
3. 数据中包含有效的 `parentItem` 和 `note` 字段

直写性能约 **~50ms**，JSON 输出含 `"write_source": "local"` 标识。Web API 作为 fallback 保留（若 SQLite 直写失败则自动回退）。
