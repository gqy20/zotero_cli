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
- [科研工作流](#科研工作流)
- [安装方式](#安装方式)
- [运行模式](#运行模式)
- [命令速查](#命令速查)

---

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
- **本地能力优先** — hybrid 模式下本地 SQLite 全文检索、PDF 标注读写不走网络

---

## 快速开始

### 推荐：让 AI 助手引导配置

如果你使用 **Claude Code** 或 **Codex**，直接告诉它：

> "帮我初始化 zot 的配置，我的 Zotero 数据目录在 XXX"

AI 会读取内置 skill 文件，自动完成 `zot init`（含模式选择和可选 PyMuPDF 安装）→ `config validate` 全流程。

### 手动安装

```powershell
# 1. 安装（任选一种）
#    macOS / Linux:   brew install gqy20/tap/zotcli
#    Windows:         从 Releases 下载 zot.exe 放到 PATH 目录

# 2. 验证
zot version

# 3. 初始化配置
zot init                 # 一键初始化（交互式：模式、API key、库 ID，local/hybrid 可选 PyMuPDF）

# 4. 验证配置
zot config validate

# 5. 一站式库概览（AI Agent 推荐入口）
zot overview --json
```

`zot init` 的交互提示：

| 配置项 | 获取方式 | 说明 |
|--------|----------|------|
| `ZOT_API_KEY` | [zotero.org/settings/keys](https://www.zotero.org/settings/keys) | 创建一个 API key |
| `ZOT_LIBRARY_ID` | Zotero 首页 → 右键库 → Advanced | 数字 ID |
| `ZOT_DATA_DIR` | Zotero → 编辑 → 首选项 → 高级 | 本地数据目录路径 |
| `ZOT_MODE` | 选择 | `web` / `local` / `hybrid`（推荐） |

> local/hybrid 模式下 `zot init` 会询问是否安装 PyMuPDF，也可事后执行 `zot init --pdf` 安装，或 `zot init --check-pdf` 诊断状态。

### 源码构建

```bash
git clone https://github.com/gqy20/zotero_cli.git && cd zotero_cli
go build -o zot.exe ./cmd/zot     # Go 1.26+，无 CGO 依赖
```

---

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

---

## 安装方式

| 平台 | 方式 | 命令 |
|------|------|------|
| **macOS / Linux** | Homebrew（推荐） | `brew install gqy20/tap/zotcli` |
| **Windows** | 手动下载 | 从 [Releases](https://github.com/gqy20/zotero_cli/releases) 下载 `zot.exe` → 放入 PATH 目录 |
| **macOS / Linux** | 手动下载 | 从 [Releases](https://github.com/gqy20/zotero_cli/releases) 下载对应二进制 → `chmod +x zot && mv zot /usr/local/bin/` |
| **任意平台** | 源码构建 | 见上方「手动安装」|

将可执行文件放入 PATH 目录即可全局使用：

- Windows: `C:\Users\<用户名>\.local\bin\`
- macOS/Linux: `/usr/local/bin/` 或 `~/.local/bin/`

> 自定义目录加入 PATH：Windows 在系统环境变量中添加；macOS/Linux 在 shell 配置文件中追加 `export PATH="$HOME/.local/bin:$PATH"`。

---

## 运行模式

| 模式 | 数据源 | 需要 | 适用场景 |
|------|--------|------|----------|
| `web` | Zotero Cloud API | API key | 远程检索、云端管理 |
| `local` | 本地 SQLite + storage/ | ZOT_DATA_DIR | 离线操作、PDF 处理、全文搜索 |
| `hybrid`（推荐） | 本地优先，Web 回退 | 两者都要 | 日常使用，兼顾速度与完整性 |

通过 `ZOT_MODE` 环境变量或 `zot init` 设置。hybrid 模式下本地独有能力（全文检索、PDF 标注）不会误回退 Web。

---

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
| **写操作** | `create-item` / `update-item` / `delete-item` | 条目 CRUD（受权限保护） |
| **标签** | `add-tag` / `remove-tag` | 批量标签管理 |
| **收藏夹** | `collections` / `create-collection` | 收藏夹查看与创建 |
| **配置** | `init / config show / validate` | 配置管理 |
| **其他** | `stats` / `tags` / `notes` / `searches` / `trash` | 库信息查看 |

完整选项说明见 [命令参考](docs/user/commands.md)，AI Agent 使用规范见 [快速入门](docs/user/quickstart.md)，技术架构见 [架构概览](docs/architecture/overview.md)。完整文档导航见 [文档中心](docs/README.md)。

---

Licensed under the [MIT License](LICENSE).
