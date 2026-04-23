# zot

[![CI](https://github.com/gqy20/zotero_cli/actions/workflows/ci.yml/badge.svg)](https://github.com/gqy20/zotero_cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/gqy20/zotero_cli)](https://github.com/gqy20/zotero_cli/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)

**AI 原生的 Zotero 命令行工具。** 为 Claude Code、Codex 等 AI agent 设计，让 AI 能直接操作你的 Zotero 文献库 — 检索、阅读 PDF、管理标注、生成引文，覆盖从文献调研到论文写作的完整科研工作流。

同时也支持终端手动使用和脚本自动化。

## 目录

- [为什么用 zot](#为什么用-zot)
- [快速开始](#快速开始)
- [AI Agent 集成](#ai-agent-集成-claude-code--codex)
- [科研工作流](#科研工作流)
- [安装方式](#安装方式)
- [运行模式](#运行模式)
- [命令速查](#命令速查)

## 为什么用 zot

| 传统方式 | 用 zot + AI |
|----------|-------------|
| 手动在 Zotero UI 里翻找文献 | `zot find "关键词" --json` → AI 直接消费结构化结果 |
| 逐篇打开 PDF 找内容 | `zot find "概念" --fulltext --snippet` → 全库全文检索 |
| 复制粘贴引文格式 | `zot cite KEY --format bibtex` → AI 自动插入正确格式 |
| 标注散落在各处无法汇总 | `zot annotations KEY --json` → 双源（DB+PDF）统一输出，支持按类型/页码/作者过滤，双层清除 |
| 批量打标签靠手点 | `zot add-tag --items K1,K2,K3 --tag "to-read"` | 一条命令 |

**核心设计原则：**

- **JSON 优先** — 所有命令支持 `--json`，输出结构化数据供 AI 直接解析
- **Skill 自动发现** — 内置 `.claude/skills/` 和 `.codex/skills/`，AI 助手开箱即懂
- **安全写操作** — 删除默认禁止、版本号乐观锁，防止 AI 误操作
- **本地能力优先** — hybrid 模式下本地 SQLite 全文检索、PDF 标注/笔记读写不走网络（Zotero 未运行时自动切换）

## 快速开始

### AI 助手一键配置（推荐）

在 **Claude Code** 或 **Codex** 中发送以下内容，AI 会自动完成全部安装和配置：

```
帮我安装并配置 zot CLI 工具，按顺序执行：

1. 检测当前平台（Windows/macOS/Linux），下载最新 zot 二进制：
   - 方式一（推荐）：从 GitHub Release 下载
     https://github.com/gqy20/zotero_cli/releases
   - 方式二（国内更快）：先查版本号，再从七牛 CDN 下载
     _VER=$(curl -sL https://qny.gqy20.top/github/zotero_cli/latest)
     然后根据平台下载对应文件，例如 Windows:
     curl -fsSL "https://qny.gqy20.top/github/zotero_cli/${_VER}/zot_${_VER}_windows_amd64.exe" -o zot.exe
   放到 PATH 目录（Windows: ~/.local/bin/ 或已存在的 PATH 目录；macOS/Linux: /usr/local/bin/ 或 ~/.local/bin/）

2. 运行 zot init 引导式配置，mode 选 hybrid。它会自动提示安装 Skill 文件，按提示执行即可。
   需要提供 API Key 和库 ID 时会提示我。
```

AI 会依次完成：检测平台 → 下载二进制 → 安装 PATH → `zot init` 配置 + 自动装 skill → `config validate` 校验。你只需在提示时输入 API Key 和库 ID。

### 手动安装

**Windows：**

| 来源 | 地址 |
|------|------|
| GitHub | 从 [Releases](https://github.com/gqy20/zotero_cli/releases) 下载 `zot.exe` |
| **七牛 CDN（国内推荐）** | 从 [CDN](https://qny.gqy20.top/github/zotero_cli/) 下载对应版本的 `zot_*_windows_amd64.exe` |

放到 `~/.local/bin/` 或任意已在 PATH 中的目录。

**macOS：**

```bash
brew install gqy20/tap/zotcli
```

**Linux：**

```bash
# 方式一：GitHub 下载
curl -fsSL https://github.com/gqy20/zotero_cli/releases/latest/download/zot-linux-amd64 -o ~/.local/bin/zot && chmod +x ~/.local/bin/zot

# 方式二：七牛 CDN 下载（国内更快）
_VER=$(curl -sL https://qny.gqy20.top/github/zotero_cli/latest)
curl -fsSL "https://qny.gqy20.top/github/zotero_cli/${_VER}/zot_${_VER}_linux_amd64.tar.gz" \
  | tar xz -C ~/.local/bin zot && chmod +x ~/.local/bin/zot

# 方式三：Homebrew
brew install gqy20/tap/zotcli
```

**后续步骤（所有平台相同）：**

```bash
zot version              # 验证安装
zot init                 # 交互式配置（mode 选 hybrid）
zot config validate       # 校验配置
zot overview --json        # 一站式库概览
```

`zot init` 配置项说明：

| 配置项 | 获取方式 |
|--------|----------|
| `ZOT_API_KEY` | [zotero.org/settings/keys](https://www.zotero.org/settings/keys) 创建 API key |
| `ZOT_LIBRARY_ID` | Zotero 首页 → 右键库 → Advanced → 数字 ID |
| `ZOT_DATA_DIR` | Zotero → 编辑 → 首选项 → 高级 → 数据目录路径 |
| `ZOT_MODE` | 推荐 `hybrid`（本地优先 + Web 回退） |

> local/hybrid 下 `zot init` 会询问是否安装 PyMuPDF，也可事后 `zot init --pdf` 安装或 `zot init --check-pdf` 诊断。
>
> 完整配置指南（含 API Key 获取、文件重命名模板、推荐插件）：见 [配置指南](docs/user/zotero-setup-guide.md)。

### 源码构建

```bash
git clone https://github.com/gqy20/zotero_cli.git && cd zotero_cli
go build -o zot.exe ./cmd/zot     # Go 1.26+，无 CGO 依赖
```

## AI Agent 集成（Claude Code / Codex）

zot 内置 **Skill 文件**，让 Claude Code、Codex 等 AI 助手开箱即懂 Zotero 操作。当然你也可以直接在终端用 `zot`，Skill 只是用 AI 时的便捷增强层。

### 内置 Skill 包含什么

`.claude/skills/zotero-cli/` 目录：

| 文件 | 作用 |
|------|------|
| `SKILL.md` | 主文件：核心命令速查 + 工作流规则 + 写操作安全策略（~185 行） |
| `reference.md` | 详细参考：决策树 / 常见陷阱 / 默认值 / JSON 格式 / 模式差异表 |
| `examples/` | `find` 和 `show` 的 JSON 输出示例 |

AI 加载后自动知道：该用什么命令、哪些参数必填、`--json` 何时加、写操作前要检查什么权限。

### 安装 Skill

**推荐：让 AI 助手帮你装**

在 Claude Code / Codex 中运行 `zot init`，初始化完成后会自动提示安装 skill，直接复制执行即可。

**手动安装**

```bash
mkdir -p ~/.claude/skills/zotero-cli/examples

# GitHub（默认）
_RAW="https://raw.githubusercontent.com/gqy20/zotero_cli/master"
# Gitee（国内更快，仅限源码文件，不含 Release 二进制）
# _RAW="https://gitee.com/gqy20/zotero_cli/raw/master"

curl -fsSL ${_RAW}/.claude/skills/zotero-cli/SKILL.md \
  -o ~/.claude/skills/zotero-cli/SKILL.md
curl -fsSL ${_RAW}/.claude/skills/zotero-cli/reference.md \
  -o ~/.claude/skills/zotero-cli/reference.md
curl -fsSL ${_RAW}/.claude/skills/zotero-cli/examples/find-output.md \
  -o ~/.claude/skills/zotero-cli/examples/find-output.md
curl -fsSL ${_RAW}/.claude/skills/zotero-cli/examples/show-output.md \
  -o ~/.claude/skills/zotero-cli/examples/show-output.md
```

> **Gitee 镜像说明**：Gitee 同步了仓库源码（Skill 文件、文档等），但 **不会同步 GitHub Releases 的二进制附件**。因此 Release 下载只能用 GitHub；Skill 文件/文档可切换到 Gitee raw 源加速。

也可在浏览器打开 [skill 目录](https://github.com/gqy20/zotero_cli/tree/master/.claude/skills/zotero-cli)（或 [Gitee 镜像](https://gitee.com/gqy20/zotero_cli/tree/master/.claude/skills/zotero-cli)），逐个文件点 **Raw** 后另存为。4 个文件建议全部下载。

**前提：** 确保已安装 `zot` 并完成 `zot init` 配置（见上方[快速开始](#快速开始)）。Skill 只是指令文件，实际执行依赖 `zot` 二进制。

验证：在 Claude Code 中说"搜一下我的文献"，AI 应自动调用 `zot find ... --json`。

### 用自然语言操作

```text
你说的                                    → AI 调用的命令
─────────────────────────────────────────────────────────────
"搜 CRISPR 基因编辑相关文献"              → zot find "CRISPR gene editing" --tag 基因编辑 --json
"导出最近半年为 bibtex"                   → zot export --date-after 2025-10 --format bibtex --json
"看这篇的 PDF 标注"                       → zot annotations KEY --json
"生成 APA 引文"                           → zot cite KEY --style apa --format citation
"把这两篇关联起来"                         → zot relate KEY_A --add KEY_B --dry-run
"提取论文图表"                            → zot extract-figures KEY -o ./figures --json
```

AI 自动处理：追加 `--json`、省略冗余 `--all`、写前检查权限、删除前确认、标注优先 Mode 1.5。

### 自定义你的 Skill

内置 Skill 是起点，不是限制。你可以基于它定制自己的工作流：

```bash
# 复制一份作为定制基础（从已安装位置或项目目录）
cp -r ~/.claude/skills/zotero-cli ~/.claude/skills/zotero-cli-custom

# 编辑 SKILL.md，加入你的习惯：
#   - 固定常用标签或收藏夹
#   - 预设导出格式和目标目录
#   - 加入领域特定的检索模板（如 "帮我找近两年 Nature/Cell 上关于 XX 的综述"）
#   - 定义多步骤工作流（如 文献调研→筛选→导出→生成报告）
```

自定义场景举例：

- **课题组共享模板**：预设团队收藏夹 key、统一标签体系、批量导出格式
- **写作辅助流**：定义"选题调研→文献筛选→标注提取→引文插入"的标准流程
- **期刊投稿追踪**：结合 `journal_rank` 字段自动筛选目标期刊分区

Skill 文件遵循 [Agent Skills 开放标准](https://github.com/anthropics/skills)，也兼容 Codex、Cursor 等支持 skill 机制的 AI 工具。

## 科研工作流

### 文献调研

```bash
# 关键词检索，返回结构化 JSON
zot find "hybrid speciation" --json

# 按时间范围筛选
zot find "CRISPR" --date-after 2023 --date-before 2025 --json

# 按标签过滤（AND / OR）
zot find "基因编辑" --tag "高引用" --tag "综述" --json
zot find "基因编辑" --tag "高引用" --tag-any --json

# 高级过滤：收藏夹、排除标签、附件名、最近修改
zot find "CRISPR" --collection ABC123 --exclude-tag "已读" --attachment-name PDF --modified-within 30d --json

# 全文搜索 PDF 内容（local / hybrid）
# 注意：FTS 索引有数据时会自动启用全文检索，无需手动加 --fulltext
zot find "同源多倍体" --fulltext --snippet --json
# snippet 默认限制 50 条，需要更多结果时显式指定 --limit
zot find "同源多倍体" --snippet --limit 200 --json
```

### PDF 阅读与标注

```bash
# 提取 PDF 正文供 AI 分析（PyMuPDF 优先 → ft-cache 回退 → pdfium WASM 兜底）
zot extract-text KEY --json

# 查看 PDF 标注（双源：Zotero 阅读器 DB 标注 + PDF 文件内标注）
zot annotations KEY --json
zot annotations KEY --type highlight --page 3 --json   # 按类型/页码过滤
zot annotations KEY --author "User" --json              # 按作者过滤

# 写入标注到 PDF（推荐 Mode 1.5：单页搜索，精准定位）
zot annotate KEY --page 4 --text "GATK" --color red --comment "关键方法"
zot annotate KEY --text "speciation" --type underline     # 下划线
zot annotate KEY --page 3 --rect 100,200,350,220         # Mode 2: 精确坐标

# 清除标注（双层删除：PDF 文件 + Zotero DB，DB 删除非阻断）
zot annotations KEY --clear                              # 删除全部
zot annotate KEY --clear --type highlight                # 按类型删
zot annotations KEY --clear --page 5 --author "User"     # 组合条件删

# 在 Zotero 阅读器中打开 PDF（跳转到指定页）
zot open KEY --page 5

# 在 Zotero 主界面选中该条目
zot select KEY
```

#### 与 Zotero 桌面端联动

zot 不是独立工具，它直接读写你的 **Zotero 本地数据目录**，并通过 `zotero://` 协议与运行中的 Zotero 桌面端交互：

| 命令 | 联动方式 | 效果 |
|------|----------|------|
| `zot open KEY` | `zotero://open-pdf` 协议 | 在已运行的 Zotero **阅读器**中打开 PDF，支持页码跳转 |
| `zot select KEY` | `zotero://select` 协议 | 在已运行的 Zotero **主界面**中选中该条目 |
| `zot annotations KEY` | SQLite + PyMuPDF 双源读取 | 同时获取 DB 层标注 **和** PDF 文件内嵌入的标注，支持 `--clear` 双层清除 |
| `zot annotate KEY` | PyMuPDF 直接写入 PDF | 3 种定位模式写入标注，支持 `--clear` 双层删除（DB 删除非阻断） |

数据来源：

- **Zotero Reader 标注** → 读取 `zotero.sqlite` 的 `itemAnnotations` 表（含你手动添加的高亮、笔记、时间戳）
- **PDF 文件内标注** → 通过 PyMuPDF 扫描 `storage/` 目录下的 PDF 二进制数据（含位置、颜色、作者信息）
- **附件路径解析** → 自动将 Zotero 内部路径映射为本地文件系统真实路径

### 笔记整理与导出

```bash
# 查看条目完整信息（含标注、附件、笔记）
zot show KEY --json

# 创建子笔记（hybrid 自动选择路径）
echo '{"itemType":"note","parentItem":"KEY","note":"<p>我的笔记</p>"}' > note.json
zot create-item --from-file note.json --if-unmodified-since-version N --json

# 查询文献间的显式关系
zot relate KEY --json

# 按收藏夹批量导出
zot export --collection COLLKEY --format csljson --json

# 生成引文
zot cite KEY                        # citation 格式（默认 apa）
zot cite KEY --format bib           # BibTeX 格式
zot cite KEY --style chicago        # Chicago 样式
```

### 库管理

```bash
zot collections --json         # 收藏夹列表
zot tags --json                # 所有标签
zot stats --json               # 库统计
zot versions items --since 0 --json  # 版本变更记录
```

## 安装方式

| 平台 | 方式 | 命令 |
|------|------|------|
| **macOS / Linux** | Homebrew（推荐） | `brew install gqy20/tap/zotcli` |
| **Windows** | 手动下载 | [GitHub Releases](https://github.com/gqy20/zotero_cli/releases) 或 [七牛 CDN](https://qny.gqy20.top/github/zotero_cli/) → `zot.exe` 放入 PATH |
| **macOS / Linux** | 手动下载 | [GitHub](https://github.com/gqy20/zotero_cli/releases) 或 [七牛 CDN](https://qny.gqy20.top/github/zotero_cli/) → `chmod +x zot && mv zot /usr/local/bin/` |
| **任意平台** | 源码构建 | `git clone https://github.com/gqy20/zotero_cli.git`（国内可用 `https://gitee.com/gqy20/zotero_cli.git`）→ `go build -o zot ./cmd/zot` |

将可执行文件放入 PATH 目录即可全局使用：

- Windows: `C:\Users\<用户名>\.local\bin\`
- macOS/Linux: `/usr/local/bin/` 或 `~/.local/bin/`

> 自定义目录加入 PATH：Windows 在系统环境变量中添加；macOS/Linux 在 shell 配置文件中追加 `export PATH="$HOME/.local/bin:$PATH"`。

## 运行模式

| 模式 | 数据源 | 需要 | 适用场景 |
|------|--------|------|----------|
| `web` | Zotero Cloud API | API key | 远程检索、云端管理 |
| `local` | 本地 SQLite + storage/ | ZOT_DATA_DIR | 离线操作、PDF 处理、全文搜索 |
| `hybrid`（推荐） | 本地优先，Web 回退 | 两者都要 | 日常使用，兼顾速度与完整性 |

通过 `ZOT_MODE` 环境变量或 `zot init` 设置。hybrid 模式下：
- **读操作**：本地优先（全文检索、PDF 标注读取）不误回退 Web
- **写操作**（笔记）：Zotero 未运行时走 SQLite 直写（~50ms），运行时自动 fallback Web API

## 命令速查

| 类别 | 命令 | 说明 |
|------|------|------|
| **检索** | `find` | 关键词/全文搜索，支持日期/标签/收藏夹/附件/类型等 20+ 过滤选项 |
| **查看** | `show` | 条目详情（含标注/附件/笔记） |
| **关系** | `relate` | 条目间显式关系查询 |
| **PDF** | `extract-text` | 提取 PDF 正文 |
| **PDF** | `annotate` | 写入高亮/下划线/笔记标注（3 种定位模式，推荐 Mode 1.5） |
| **PDF** | `annotations` | 读取/删除 PDF 标注（双源 + 双层清除） |
| **PDF** | `open` | 在 Zotero 阅读器中打开 |
| **导航** | `select` | 跳转到 Zotero UI 选中条目 |
| **引文** | `cite` | APA / BibTeX / Chicago 等格式 |
| **导出** | `export` | BibTeX / RIS / CSL-JSON |
| **写操作** | `create-item` / `update-item` / `delete-item` | 条目 CRUD（笔记支持 hybrid 本地写入） |
| **标签** | `add-tag` / `remove-tag` | 批量标签管理 |
| **收藏夹** | `collections` / `create-collection` | 收藏夹查看与创建 |
| **配置** | `init / config show / validate` | 配置管理 |
| **其他** | `stats` / `tags` / `notes` / `searches` / `trash` | 库信息查看 |

完整选项说明见 [命令参考](docs/user/commands.md)，AI Agent 使用规范见 [快速入门](docs/user/quickstart.md)，技术架构见 [架构概览](docs/architecture/overview.md)。完整文档导航见 [文档中心](docs/README.md)。

Licensed under the [MIT License](LICENSE).
