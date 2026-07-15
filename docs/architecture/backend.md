# 后端设计

LocalReader 实现细节、SQLite 查询策略和模式边界。

> 架构概览见 [overview](./overview.md)，领域模型见 [domain-model](./domain-model.md)。

> 原始设计文档见 [research/mvp-definition.md](../research/mvp-definition.md)，本文档记录已实现的设计决策。

---

## 当前文件布局

local backend 不再是单个 `local.go`，而是按职责拆分：

| 文件 | 职责 |
|------|------|
| `local.go` | `LocalReader` 生命周期、高层编排、public 方法（FindItems/GetItem/GetRelated/GetLibraryStats） |
| `local_db.go` | SQLite 连接设置、busy/locked 重试检测、snapshot 回退 |
| `local_find.go` | local `find` SQL 构建、后处理过滤/排序/分页、attachment-aware 过滤、matched_on 推导 |
| `local_loaders.go` | item loaders / relation loaders / creators + tags + collections + attachments + notes 并行加载、attachment path resolution |
| `local_prefs.go` | Zotero `prefs.js` 发现和解析、dataDir → profile 映射 |
| `local_utils.go` | 本地专用格式化和归一化工具函数 |
| `remote.go` | `RemoteReader`，HTTP client，remote 模式实现 |

### 扩展规则

| 新功能 | 放入文件 |
|--------|---------|
| 新 find 查询语义 | `local_find.go` |
| 新数据加载逻辑 | `local_loaders.go` |
| SQLite/session 行为 | `local_db.go` |
| Zotero profile/config 发现 | `local_prefs.go` |
| 通用本地工具 | `local_utils.go` |

`local.go` 应保持精简，主要做协调调度。

---

## 模式边界

### Backend-aware 读命令（参与 web/local/hybrid 选择）

`item find` / `item show` / GetRelated（由当前 canonical 业务入口复用）

### Remote API 命令（始终走 Web API）

`item export` / `coll *` / `note *` / `tag list` / `search *` / `lib *` / `schema *` / `config check` / `group list`

### Hybrid 写入命令（自动选择路径）

`item new`（仅 itemType = `note` 时支持 local 写入）

| 模式 | backend-aware 读命令 | hybrid 写入命令 | remote-only 写命令 |
|------|---------------------|-------------------|---------------------|
| `web` | 支持 | → Web API | 支持 |
| `hybrid` | 支持（本地优先） | **Zotero 未运行 → local SQLite；运行中 → Web API** | 支持 |
| `local` | 支持 | **Zotero 未运行 → local SQLite；运行中 → Web API fallback** | 显式拒绝 |
| `remote` | 支持（经由 server 转发） | `ann new` / `ann delete --source pdf` 走 server；Zotero 标注删除需 API key | 其余需 API key（走 remote+web） |

> 写操作安全规则不变：删除默认禁止、版本号乐观锁。Zotero 原生标注删除按 item key 走 Web API，不直接修改 SQLite；PDF 标注按 xref 在临时副本中完成并验证。remote 下的 PDF 标注写入/删除由服务端 `ZOT_ALLOW_WRITE` / `ZOT_ALLOW_DELETE` 门控。

---

## Local DB 关键发现

基于对真实 Zotero 数据目录的检查（6716 条目 / 33 集合 / 6 搜索）：

### 核心表覆盖

| 表 | 用途 | 状态 |
|----|------|------|
| `items` | 主表 | 核心查询 |
| `itemTypes` | 类型定义 | JOIN 映射 |
| `itemData` + `itemDataValues` + `fieldsCombined` | EAV 元数据模型 | 全部使用 |
| `itemCreators` + `creators` + `creatorTypes` | 作者数据 | 并行加载 |
| `itemTags` + `tags` | 标签关联 | 过滤+展示 |
| `itemAttachments` | 附件元数据 | 过滤+路径解析 |
| `itemAnnotations` | PDF 标注 (DB 层) | 读取 |
| `itemNotes` | 子笔记 | 排除过滤 |
| `collections` + `collectionItems` | 收藏夹层级与成员 | 列出+过滤 |
| `itemRelations` | 条目关系 | 查询 |
| `fulltext*` | 全文索引系统 | 独立 FTS5 索引（非 Zotero 内置） |

### 附件路径风格

| 路径格式 | 解析方式 |
|----------|----------|
| `storage:filename.pdf` | → `storage/<attachmentKey>/filename`（可靠） |
| `attachments:relative/path.pdf` | 普通本地库按 Zotero `baseAttachmentPath` 解析；同步镜像按 `data_dir/attachments/` 解析 |
| 空 | 未解析（HTML snapshot / linked URL 等） |

### 可见条目策略

默认 `find` 结果排除：
- `itemAttachments` 中的行
- `itemNotes` 中的行
- `itemAnnotations` 中的行
- `itemType = annotation` 的行

这确保 agent 和用户看到的是"真正的文献条目"而非内部噪声。

---

## 查询策略：为什么不用一个 giant JOIN

推荐多步小查询而非单条大 SQL：

1. **更容易理解** — 每个方法职责单一
2. **更容易测试** — 可针对单个 loader 写测试
3. **更容易维护** — 避免行乘爆炸（metadata × tags × collections × attachments）
4. **对齐 domain model** — 每个 loader 直接映射到 Go struct

典型分解：`GetCoreItemByKey` → `ListCreatorsByItemID` → `ListTagsByItemID` → `ListAttachmentsByParentItemID` → `ListCollectionsByItemID` → `ListNotesByItemID`

---

## 日期规范化

原始 Zotero 日期值可能包含重复或混合格式（如 `2019-03-29 2019-03-29`）。local row mapping 在返回前统一做 normalize，确保 web/local 输出日期语义一致。
