# 回退与历史兼容

本文档是 Zotero CLI 回退行为和历史兼容承诺的维护者参考。它回答四个问题：何时切换路径、切换到哪里、能力是否损失，以及旧入口或旧数据何时可以删除。

> 模式和组件关系见 [架构概览](./overview.md)，错误类型和设计理由见 [设计决策](./decisions.md)，Zotero 数据库版本差异见 [Zotero Schema 兼容性](../reference/zotero-schema.md)。

---

## 术语

以下机制不要混为一谈：

| 机制 | 含义 | 例子 |
|---|---|---|
| 回退（fallback） | 首选路径无法承接请求时，切换到能力已知的次选路径 | hybrid 从 LocalReader 切到 WebReader |
| 重试（retry） | 同一路径发生暂时性失败后再次执行 | Web API 遇到限流或暂时性网络错误 |
| 迁移（migration） | 将旧的持久化结构升级到当前结构 | reference index 启动时补充缺失列 |
| 历史兼容（compatibility） | 继续接受旧命令、旧输入或旧数据 | `setup pdf-extract --check` |
| 重建（rebuild） | 派生数据无法安全迁移时丢弃并重新生成 | 不兼容的全文 FTS 表 |

回退不是“捕获任意错误后换一个后端”。每条回退必须有明确触发条件，且目标路径必须能够表达原请求。参数错误、权限错误、损坏数据和本地独有能力失败通常不应静默回退。

---

## 运行时回退矩阵

### Hybrid 读取：LocalReader → WebReader

`hybrid` 模式先读本地 Zotero 数据，只有 Web API 能承接同一语义时才回退。

| 操作 | 允许回退的主要条件 | 不回退的情况 |
|---|---|---|
| `find` | 本地暂时不可用；本地不支持且 Web 支持的 `--qmode`、`--include-trashed` | 全文、附件路径、附件健康等 local-only 条件；普通参数错误 |
| `show` / GetItem | 本地未找到、本地不支持或暂时不可用 | 请求包含 Web 无法补足的本地内容时，由命令层保留能力边界 |
| `relate` / GetRelated | 本地关系读取失败后可尝试 Web 显式关系 | Web 也无法承接时返回实际错误 |
| `stats`、notes、tags、collections | 本地不支持或暂时不可用 | 参数或配置错误 |

实现入口为 `internal/backend/reader.go` 中的 `readWithFallbackUsingPolicy`、`shouldFallbackToWeb` 和 `shouldFallbackFindToWeb`。新增查询选项时，必须同时检查 `SupportsWebFind`，避免把 local-only 请求错误地退到 Web。

如果 LocalReader 初始化失败，hybrid 可以只持有 WebReader；这并不意味着 PDF、全文或附件文件能力会由 Web 自动补齐。

### SQLite 读取：live → snapshot

Zotero 运行时可能锁定 SQLite 数据库。本地读取先尝试 live 数据库；遇到可识别的 busy/locked 情况时，改读临时或缓存快照。

快照回退具有以下可见语义：

- `read_source` 为 `snapshot`
- `sqlite_fallback` 为 `true`
- 快照可能落后于当前库时，`snapshot_stale` 为 `true`
- 文本输出会在 stderr 提示正在使用 snapshot；陈旧时追加 warning

快照是只读降级，不得用于绕过写入冲突，也不得把非锁定类 SQLite 错误一律视为可回退。

### 全文读取：缓存 → 提取器链

单篇全文读取先检查 `.zotero_cli/fulltext` 缓存。缓存有效且包含正文时直接返回，不再调用提取器。缓存未命中时，提取链为：

1. PyMuPDF：首选，提供结构化 block、页码和几何信息
2. Zotero `.zotero-ft-cache`：快速文本后备，但质量和结构信息可能较弱
3. pdfium WASM：最终本地兜底

成功结果通过 `full_text_source`、`full_text_engine`、`full_text_attachment_key` 和 `full_text_cache_hit` 暴露实际来源。提取成功后会写入项目自己的全文缓存和 FTS5 索引。

只有 `content.txt`、没有 `chunks.json` 的旧缓存仍可提供整篇文本，但不能可靠支持 `--pages`。此时应明确要求重建，不能伪造页码。

### Reference 构建：PMC → PubMed + Europe PMC 增强

默认 `ref build` 路由为：

1. 能解析到 PMCID 时，首选 PMC JATS，同时获取参考文献和正文引用上下文
2. 没有 PMCID 但有 PMID 时，使用 PubMed references
3. PubMed 路径成功后，尽力合并 Europe PMC 开放参考文献并去重
4. Europe PMC 不可用时，保留 NCBI 核心结果，不把增强层失败升级为整条任务失败

PMC JATS 成功时不会重复请求 Europe PMC references。GROBID 是显式、实验性的 PDF 后备，不属于默认构建链，不能在普通 `ref build` 中静默启用。

### PDF Figure 提取：矢量聚类 → 位图锚点

`extract-figures` 首选 PyMuPDF `cluster_drawings()` 聚合矢量图形。未被矢量候选覆盖的大尺寸嵌入图片可通过位图锚点路径补充；极端高密度页面还会使用受阈值约束的低置信候选。

该路径是同一提取器内的算法降级，不是格式兼容。结果中的 `source` 和统计字段应保留实际路径，避免把低置信结果描述为等价输出。

### Python 环境安装：uv → 系统 Python

自动配置 PyMuPDF 时优先使用 `uv` 创建隔离环境；找不到 `uv` 时使用可用的系统 Python。两条路径最终都必须验证 `fitz` 可导入，安装命令退出成功但验证失败不能视为成功。

### 笔记写入：local SQLite → Web API

`create-item` 创建 note 时，local/hybrid 在 Zotero 未运行时可以直接写 SQLite；Zotero 正在运行时转到 Web API，避免与桌面端并发修改数据库。

这条回退仍受写入门控、API 凭据和版本前置条件约束。不得因为本地写入失败而绕过权限或乐观锁要求。其他 item type 不自动获得 local 写入能力。

### Web UI 路由：静态资源 → SPA index

嵌入式 Web UI 对非 API、非静态资源路由返回 `index.html`，供前端路由接管。`/api/*` 错误不得落入 SPA fallback，否则客户端会收到 HTML 而非 API 错误。

---

## 明确不回退的边界

以下情况应保留真实错误或给出迁移提示：

- `find --fulltext`、附件路径/类型/健康过滤等本地独有查询不能退到 Zotero Web API
- PDF 文件、PDF 标注和本地附件读取不能因 LocalReader 缺失而假装由 WebReader 提供
- 无效参数、认证失败、权限不足、版本冲突和删除门控失败不能通过替换后端绕过
- 损坏或语义不完整的旧缓存不能生成看似可靠的页码、坐标或引用上下文
- GROBID 不得作为默认、静默的 reference fallback
- snapshot 不得用于写操作

新增 fallback 时应优先扩展稳定错误标记，而不是匹配 `err.Error()` 文本。

---

## 当前历史兼容清单

### CLI 入口

| 旧入口 | 当前行为 | 当前替代 | 状态 |
|---|---|---|---|
| `zot setup pdf-extract --check` | 隐藏于顶层帮助之外，但仍执行 PyMuPDF 状态检查 | `zot init --check-pdf` | deprecated，保留兼容 |
| `zot setup pdf-extract` | 不再安装，返回迁移提示 | `zot init --pdf` | redirect-only |
| `zot config init` | 不再运行旧的交互配置流程，返回迁移提示 | `zot init` | redirect-only |

已移除且直接报 unknown command 的旧 schema 内省命令不属于当前兼容面；统一使用 `zot schema <sub>`。

### 持久化数据

| 数据 | 兼容方式 | 能力边界 |
|---|---|---|
| 旧 reference index | `store.init()` 以幂等 `ALTER TABLE` 补列、创建新表并修正派生状态 | `metadata_version` 过旧的成功记录会重新构建，而不是按新格式直接读取 |
| 旧 unsupported/failed reference 记录 | 初始化迁移会按当前状态语义重新分类 | 仅迁移可识别的历史记录 |
| 旧 NCBI HTTP 缓存键 | 客户端继续识别旧缓存键；新 provider 使用独立命名空间 | Europe PMC 缓存不会与 NCBI 旧键混用 |
| 旧全文 FTS schema | 可补的普通列使用 `ALTER TABLE`；核心 FTS 表缺少必要列时重建派生索引 | 索引可重建，不承诺保留派生排序状态 |
| 仅有 `content.txt` 的全文缓存 | 保留整篇文本读取 | 不支持可靠的逐页过滤，需重建生成 `chunks.json` |

### Zotero 数据与输入格式

- 本地查询面向 Zotero 7→9 的已知 SQLite schema；具体列变化和验证范围以 [Zotero Schema 兼容性](../reference/zotero-schema.md) 为准。
- 日期过滤接受 `YYYY`、`YYYY-MM`、`YYYY-MM-DD`，local/hybrid 还会归一化 Zotero 常见的部分日期表示，例如 `YYYY-MM-00 YYYY-MM` 和 `MM/YYYY`。
- `storage:` 附件路径是可靠映射；`attachments:` 链接路径只做 best-effort 解析，不应把无法解析视为旧格式已完整兼容。

---

## 可观测性契约

只要 fallback 会改变来源、新鲜度或结果质量，就应对调用者可见。

| 字段或信号 | 含义 |
|---|---|
| `read_source` | `live`、`snapshot`、`web` 等实际读取来源 |
| `sqlite_fallback` | 是否从 live SQLite 切换到 snapshot |
| `snapshot_stale` | 快照是否可能比源数据库陈旧 |
| `full_text_source` | 项目缓存、Zotero ft-cache、PyMuPDF 或 pdfium 等正文来源 |
| `full_text_engine` | 实际使用的提取引擎 |
| `full_text_cache_hit` | 是否直接命中项目全文缓存 |
| `write_source` | 写操作实际使用 local 或 web |
| stderr note/warning | 文本模式下提示来源切换或质量风险；stdout 保持可解析 |

JSON 新字段应保持可选和向后兼容。不要通过改变既有字段含义来表达新的 fallback 状态。

---

## 新增与删除规则

### 新增 fallback

每条新 fallback 至少需要：

1. 明确的源路径和目标路径
2. 基于稳定错误类型或能力检查的触发条件
3. 不回退条件和语义损失说明
4. JSON 元数据或日志等可观测信号
5. 首选成功、允许回退、禁止回退、目标也失败四类测试
6. 本文档及相关用户命令文档更新

### 保留历史兼容

引入 deprecated 入口或数据迁移时，应记录：

- 首次废弃版本
- 推荐替代方式
- 当前是完整兼容、部分兼容还是仅重定向
- 警告出现位置
- 最早允许删除的版本或判断条件
- 覆盖该承诺的测试

### 删除历史兼容

满足以下条件后才能删除：

1. 替代入口已至少发布一个明确的迁移周期
2. CHANGELOG 和命令帮助曾提供迁移提示
3. 持久化格式能够安全迁移、重建，或有明确的不支持声明
4. 删除同步更新测试、本文档、命令参考和 CHANGELOG

没有计划删除时间的兼容层也应标记为“长期支持”，避免形成无人知晓的永久分支。
