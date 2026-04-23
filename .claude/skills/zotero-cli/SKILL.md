---
name: zotero-cli
description: >
  Zotero 文献管理 CLI（`zot`）。文献检索(find/show)、导出(export bibtex/csljson)、
  引文生成(cite)、PDF 标注提取(annotations/extract-text/extract-figures)、
  关系网络(relate --aggregate --dot)、写操作(create-item/update-item/add-tag)。
  支持 web/local/hybrid 三种模式。搜索/管理/分析 Zotero 文献库时使用。
when_to_use: >
  触发关键词：Zotero、文献管理、参考文献、PDF 标注、引文格式、文献检索、
  学术数据库、bibtex、文献关系网络。
  示例："搜 CRISPR 相关文献"、"导出 bibtex"、"查看 PDF 标注"、
  "生成 APA 引文"、"查找条目关联"、"提取论文图表"
argument-hint: "[command] [ITEMKEY] [options]"
---

# Zotero CLI (`zot`)

优先使用本地 CLI，不要自行实现 Zotero API 调用。

## 快速开始

```shell
zot init                          # 一键初始化（推荐入口）
zot config validate               # 校验配置有效性
zot overview --json               # Agent 首选：一站式库概览
zot version --check               # 检查是否有新版可更新
```

## 工作流程

1. 在项目根目录下工作。
2. 优先使用 `zot`（二进制存在且版本足够）。
3. Agent 工作流**始终加 `--json`**，设置 `ZOT_JSON_ERRORS=1` 获得结构化错误输出。
4. 操作前先运行 `config validate` 确认凭据可用。
5. **不确定时用 `zot <command> --help` 核对** — skill 文件可能与当前二进制版本不同步，以 `--help` 输出的选项和用法为准。

### 模式选择

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `hybrid` (默认) | 本地优先 + Web 回退 | **推荐默认**，兼顾速度与完整性 |
| `local` | 读本地 SQLite 数据库 | 大量读操作、PDF 标注/提取 |
| `web` | 纯云端 Zotero Web API | 无本地 Zotero 安装 |

> 写操作：`web` 和 `hybrid` 支持全部写操作；`local` 支持**笔记创建**（Zotero 未运行时自动走 SQLite 直写 ~50ms）和 PDF 标注写入。

---

## 核心命令速查

### 库概览（Agent 入口）

```shell
zot overview --json        # 统计 + Top 收藏夹/标签 + 最近条目 + FTS 状态
zot stats --json           # 条目/收藏夹/搜索计数
```

### 文献检索

> **有过滤标志时无需 `--all` 也无需查询词，自动按纯过滤条件搜索。** 仅无查询词且无任何过滤时才报错。

```shell
zot find "CRISPR gene editing" --json              # 关键词搜索
zot find --tag TAG1 --tag TAG2 --json              # 纯标签过滤
zot find --collection KEY --date-after 2024-01 --json  # 组合过滤
zot find --all --json                              # 全部条目
zot show ITEMKEY --json                            # 条目详情（含子笔记+标注+期刊等级）
zot relate ITEMKEY --json                          # 关联条目
```

**基础过滤：** `--date-after` / `--date-before` / 多次 `--tag`(AND) / `--tag-any`(OR)

**高级过滤（local/hybrid）：** `--collection` / `--no-collection` / `--tag-contains` / `--exclude-tag` / `--no-type` / `--has-pdf` / `--modified-within` / `--added-since` / `--attachment-name` / `--attachment-path`

**输出控制：** `--include-fields` / `--full` / `--sort` + `--direction` / `--start` + `--limit`

**FTS5 全文检索：** local/hybrid 下有索引时**自动启用**，搜索范围扩展至 PDF 全文内容。纯元数据搜索可临时设 `ZOT_MODE=web`。`--snippet` 默认限制 **50** 条。

### 导出与引用

```shell
# 导出格式：csljson / bibtex / biblatex / ris
zot export --all --format csljson --json
zot export --collection COLLKEY --format bibtex --json
zot export --item-key KEY --format biblatex --json

# 引文（--format: citation | bib；样式用 --style）
zot cite ITEMKEY --format citation        # 正文引用（默认）
zot cite ITEMKEY --format bib              # 参考文献条目
zot cite ITEMKEY --style apa               # 指定引文样式（默认 apa）
```

### 条目关系 (`relate`)

```shell
zot relate ITEMKEY --json                           # 显式关系
zot relate ITEMKEY --aggregate --json               # 三层聚合（local/hybrid）
zot relate ITEMKEY --dot > network.dot              # DOT 可视化
zot relate ITEMKEY --add TARGET --dry-run            # 预览添加
zot relate ITEMKEY --from-file batch.json --dry-run   # 批量操作
```

三层模型：①显式关系(itemRelations) → ②子笔记关系 → ③内嵌Citation（笔记HTML）。详见 [reference.md](reference.md) 模式支持矩阵。

### PDF 操作（需 local/hybrid + PyMuPDF）

```shell
zot extract-text ITEMKEY --json                       # 正文提取
zot extract-figures ITEMKEY -o ./figures -w 4         # Figure 提取
zot annotations ITEMKEY --json                         # 读取标注
zot annotate ITEMKEY --page 4 --text "GATK" --color red # 写入标注（推荐 Mode 1.5）
zot open ITEMKEY --page 5                              # Zotero 阅读器中打开
```

**标注要点：** 推荐 `--page N --text "keyword"` (Mode 1.5)；`--clear` 双层删除需 Zotero 关闭才能删 DB 层；详细文档见 [annotations 示例](https://github.com/gqy20/zotero_cli/blob/master/docs/user/examples/annotations.md)（[Gitee 镜像](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/examples/annotations.md)）。

### 元数据 Schema 与其他只读命令

```shell
zot schema types --json                    # 所有文献类型
zot schema fields-for journalArticle       # 某类型的有效字段
zot schema template book --json            # 创建模板
```

| 命令 | 说明 |
|------|------|
| `collections` / `collections-top` | 收藏夹列表 |
| `tags` | 标签列表 |
| `notes [--query "..."]` | 笔记搜索 |
| `searches` | 已保存搜索 |
| `groups` | 可访问群组 |
| `trash` / `publications` / `deleted` | 回收站 / 我的发表 / 已删除 key |
| `versions <type> --since N` | 版本变更（type: collections\|searches\|items\|items-top） |
| `key-info` | API Key 权限信息 |
| `index build [--force]` | FTS5 全文索引构建 |

---

## 写操作安全

以下命令会修改数据：`create-item` / `update-item` / `delete-item` / `add-tag` / `remove-tag` / `create-collection` / `update-collection` / `delete-collection` / `create-search` / `update-search` / `delete-search` / `annotate` / `relate --add` / `relate --remove`

执行前：
1. 确认用户意图。
2. 检查 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE` 环境变量。
3. 尽可能使用版本前置条件（`--if-unmodified-since-version N`）。
4. 优先用 `--dry-run` 预览（relate 支持；annotate 暂不支持）。

删除操作额外要求：复述目标 key → 无歧义确认 → 有不确定就先询问。`--json` 模式自动跳过确认提示。

完整 JSON 输入格式示例见 [reference.md](reference.md)「写操作 JSON 输入格式」章节。

---

## 配置

存储位置：`~/.zot/.env`

```shell
zot init                                    # 交互式初始化（默认 mode=hybrid）
zot config show                              # 查看当前配置
zot config validate --json                   # 校验 + 结构化诊断
zot version --check [--json]                # 检查新版 + 更新指引
```

配置缺失时主动初始化，不要绕过错误。环境变量完整速查见 [reference.md](reference.md)。

---

## 性能注意

- `overview` 并行调用 4 个 API（~6s），优于逐个请求
- **快照缓存**：hybrid 下 Zotero 运行时自动走持久化快照（~0.3s/次）；关闭时直连 SQLite（~0ms）
- `extract-text` 结果有缓存；`--snippet` 默认 limit 50；高频脚本遇 `429` 自动退避
- `extract-figures` 多篇自动并行，单篇 ~0.6s

---

## Additional resources

> **国内用户**：以下链接如访问缓慢，可在浏览器中将 `github.com/gqy20/zotero_cli/blob` 替换为 `gitee.com/gqy20/zotero_cli/blob` 访问镜像。

- 详细参考（决策树/陷阱/默认值/JSON 格式/模式差异表）：[reference.md](reference.md)
- 命令输出示例：[examples/](examples/)
- 完整命令参考：[commands.md](https://github.com/gqy20/zotero_cli/blob/master/docs/user/commands.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/commands.md)
- 快速入门：[quickstart.md](https://github.com/gqy20/zotero_cli/blob/master/docs/user/quickstart.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/quickstart.md)
- Relate 实战案例：[relate.md](https://github.com/gqy20/zotero_cli/blob/master/docs/user/examples/relate.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/examples/relate.md)
- 标注操作详解：[annotations.md](https://github.com/gqy20/zotero_cli/blob/master/docs/user/examples/annotations.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/examples/annotations.md)
- 版本规划：[roadmap.md](https://github.com/gqy20/zotero_cli/blob/master/docs/plans/roadmap.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/plans/roadmap.md)
- Zotero 安装配置：[zotero-setup-guide.md](https://github.com/gqy20/zotero_cli/blob/master/docs/user/zotero-setup-guide.md) / [Gitee](https://gitee.com/gqy20/zotero_cli/blob/master/docs/user/zotero-setup-guide.md)
