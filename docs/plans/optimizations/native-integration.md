# Zotero 原生能力对接优化方案

> 基于 Zotero Web API v3 + Zotero 9.0 本地数据库的深度调研，梳理可利用的原生能力及项目优化方向。
> 调研时间: 2026-04-22 | 当前版本: Zotero 9.0 (2026-04-10 发布)

---

## 一、Zotero API 能力全景

### 1.1 Web API v3 核心端点

| 能力域 | 端点 | 方法 | 项目现状 |
|--------|------|------|----------|
| 条目 CRUD | `/users/{id}/items` | GET/POST/PUT/PATCH/DELETE | ✅ 已实现 |
| 集合 CRUD | `/users/{id}/collections` | GET/POST/PUT/PATCH/DELETE | ✅ 已实现 |
| 标签 | `/users/{id}/tags` | GET | ✅ 已实现 |
| 已保存搜索 | `/users/{id}/searches` | GET | ✅ 已实现 |
| 引文生成 | `/users/{id}/items/{key}` | `?format=bib` | ✅ 已实现 |
| **全文内容** | `/users/{id}/items/{key}/fulltext` | **GET/PUT** | ❌ 未利用 |
| **批量版本检查** | `/users/{id}/items?format=versions&since=N` | GET | ⚠️ 部分利用 |
| **流式推送** | `wss://stream.zotero.org` | WebSocket | ❌ 未实现 |
| **导出格式透传** | `?format=ris\|mods\|csljson\|...` | GET | ⚠️ 仅部分 |

### 1.2 本地 SQLite 核心表（Zotero 9 兼容验证通过）

> 2026-04-22 实测：所有查询在 Zotero 9 数据库上正常运行，6716 条目 / 33 集合 / 6 搜索。

| 表名 | 用途 | 项目使用情况 |
|------|------|-------------|
| `items` | 主表 (key, version, itemTypeID, dateAdded, dateModified) | ✅ 核心查询 |
| `itemTypes` | 类型定义 (typeName) | ✅ JOIN 映射 |
| `itemData` + `itemDataValues` | 字段值 EAV 模型 | ✅ 全部元数据字段 |
| `fieldsCombined` | 字段名联合视图 | ✅ 大量使用 |
| `itemCreators` + `creators` | 作者数据 | ✅ 并行加载 |
| `itemTags` + `tags` | 标签关联 | ✅ 过滤+展示 |
| `collections` + `collectionItems` | 集合层级与成员 | ✅ 列出+过滤 |
| `itemAttachments` | 附件元数据 | ✅ 过滤+解析路径 |
| `itemAnnotations` | PDF 标注 (DB 层) | ✅ 读取 |
| `itemNotes` | 子笔记 | ✅ 排除过滤 |
| `itemRelations` | 条目关系 | ✅ 查询 |
| `deletedItems` | 软删除追踪 | ❌ 未使用 |

---

## 二、优化方案（按优先级排序）

### P0 — 低成本高收益

#### 2.1 条件请求缓存（304 Not Modified）

**问题**: HybridReader 每次 find/show 都完整请求 Web API 数据，即使数据未变化。

**Zotero 原生能力**: `If-Modified-Since-Version: {version}` 请求头 → 返回 `304 Not Modified`。
条件请求 **不计入速率限制且无调用次数上限**。

**实现思路**:

```
WebReader.FindItems()
  ├─ 1. 先发 HEAD (或 limit=0) 获取当前 Last-Modified-Version
  ├─ 2. 对比缓存中的 version
  ├─ 3. 若未变 → 直接返回缓存结果 (零网络开销)
  └─ 4. 若已变 → 正常请求，更新缓存
```

**涉及文件**: `internal/backend/web.go`, `internal/backend/reader.go`

**预期收益**: Agent 场景下连续调用时，重复查询从 ~0.4s（缓存命中）降至 <100ms（Zotero 关闭时直连）。

---

#### 2.2 导出格式透传

**问题**: 导出功能可能仅支持 BibTeX/RIS/CSL-JSON 等少数格式，本地实现转换逻辑。

**Zotero 原生能力**: 服务端支持 **20+ 种导出格式**，仅需 `?format=xxx` 参数：

| 格式 | 参数值 | 适用场景 |
|------|--------|----------|
| BibTeX | `bibtex` | LaTeX 用户 |
| RIS | `ris` | EndNote / Reference Manager |
| CSL-JSON | `csljson` | 引用管理器通用交换 |
| MODS | `mods` | 图书馆系统 |
| MARC | `marc` | 编目 |
| RDF (Bibliontology) | `rdf_bibliontology` | 语义网 |
| TEI | `tei` | 数字人文 |
| NLM | `nlm` | 医学文献 |
| Zotero XML | `zotero-xml` | 完整数据备份 |
| Wikipedia | `wikipedia` | 维基百科引用 |
| Note | `note` | 纯文本笔记导出 |
| TXT | `txt` | 简单文本 |

额外参数:
- `&exportNotes=1` — 包含子笔记
- `&exportFile=1` — 包含附件链接
- `&linkwrap=1` — BibTeX 中包裹 URL

**实现思路**: 在 `client_export.go` 中新增 `Format` 参数，直接透传给 API，无需本地解析。

**预期收益**: 零开发成本获得 20+ 格式支持。

---

#### 2.3 批量写入 50 对象打包

**问题**: 批量标签操作、批量移动集合等可能逐条发送请求。

**Zotero 原生能力**: 单次 POST/PUT/PATCH 支持 **最多 50 个对象**的原子事务（全成功或全失败）。

```json
// 单次请求体示例: 批量添加标签
[
  { "key": "ABCD1234", "version": 123, "tags": [{"tag": "important"}] },
  { "key": "EFGH5678", "version": 124, "tags": [{"tag": "important"}] },
  // ... 最多 50 个
]
```

配合:
- `Zotero-Write-Token: {32-char-hex}` — 幂等性保证，安全重试
- `If-Unmodified-Since-Version: {version}` — 冲突检测

**涉及文件**: `internal/zoteroapi/client_items.go`

**预期收益**: 批量操作性能提升 10-50x（减少 HTTP 往返）。

---

### P1 — 功能增强

#### 2.4 Full-text Content API 对接

**问题**: 全文搜索仅在 Local 模式可用（FTS5），Hybrid/Web 模式无法搜索 PDF 内容。

**Zotero 原生能力**:

```
GET  /users/{uID}/items/{itemKey}/fulltext          # 获取单条全文
GET  /users/{uID}/fulltext?since={version}           # 批量获取变更
PUT  /users/{uID}/items/{itemKey}/fulltext          # 上传全文
```

响应格式:
```json
{
  "content": "Full text content string...",
  "indexedChars": 12345,
  "indexedPages": 10
}
```

**实现思路**:

1. **读取侧**: Hybrid 模式下，Web 端 fallback 可走 Full-text API 实现 `--fulltext` 搜索
2. **写入侧**: Local 模式提取的全文可通过 API 上传到云端 (`PUT fulltext`)，实现"本地提取、云端可用"
3. **同步策略**: `GET /fulltext?since={version}` 可检测哪些条目的全文有变更

**新增命令建议**:
```
zot fulltext push [KEY...]     # 将本地提取的全文上传到云端
zot fulltext pull [--since N]  # 拉取云端全文变更到本地缓存
zot fulltext status            # 显示全文同步状态统计
```

**预期收益**: Hybrid 模式补全全文检索能力；多设备间全文索引同步。

---

#### 2.5 OAuth 登录流程

**问题**: 当前仅支持预共享 API Key（`ZOT_API_KEY`），用户需手动去网页生成。

**Zotero 原生能力**: OAuth 1.0a 授权流程:

```
1. POST /oauth/request  → request_token + secret
2. 引导用户打开: https://www.zotero.org/oauth/authorize?oauth_token=xxx
3. 用户授权后回调 (或手动输入 verifier)
4. POST /oauth/access   → access_token (等效于 API Key)
```

**优势**:
- 支持双因素认证 (2FA) — Z9 新增了 Web 登录流程
- 无需用户手动复制 Key
- 可集成到 `zot init` 交互向导中

**新增命令**:
```
zot auth login             # 交互式 OAuth 授权
zot auth status            # 查看当前认证状态
zot auth logout            # 清除本地凭证
```

**预期收益**: 降低首次使用门槛，特别是对非技术用户。

---

#### 2.6 Translation Server URL 导入

**问题**: 从网页导入文献需手动输入元数据，或依赖不稳定的爬虫。

**Zotero 原生能力**: Translation Server（Docker 化）提供与浏览器 Connector 同引擎的元数据抽取:

```bash
docker run -p 1969:1969 zotero/translation-server

# 调用示例
curl -X POST http://localhost:1969/web \
  -H "Content-Type: text/plain" \
  -d "https://www.nature.com/articles/s12345"
# 返回: 结构化文献元数据 (标题/作者/期刊/DOI/日期...)
```

**新增命令**:
```
zot import <url> [--save]    # 从 URL 提取元数据并显示/保存
zot import --from-clipboard  # 从剪贴板读取 URL
zot import --batch urls.txt  # 批量导入
```

**实现要点**:
- Translation Server 可本地 Docker 部署（隐私友好）
- 抽取结果可直接映射为 domain.Item 并调用 create-item
- 支持数百个网站的知识库（与 Zotero Connector 共享）

**预期收益**: 核心新功能——一键从任意网页导入规范文献记录。

---

### P2 — 架构升级

#### 2.7 Streaming WebSocket API — 实时监听

**问题**: 无法感知远程 library 变更（队友新增/修改/删除条目），只能主动轮询。

**Zotero 原生能力**: `wss://stream.zotero.org` 推送式通知:

```javascript
// 订阅
ws.send(JSON.stringify({
  action: "subscribe",
  topics: ["/users/12345", "/groups/67890"]
}))

// 收到事件
{ "event": "topicUpdated", "topic": "/users/12345" }
```

**适用场景**:
- 多人协作群组库时实时感知变更
- 触发本地缓存自动刷新
- 配合 daemon 模式实现推送通知

**新增命令**:
```
zot watch [--group ID]       # 订阅变更并打印事件流
zot watch --on-change "cmd"  # 变更时执行自定义命令
```

**注意**: 此功能更适合长运行模式 (daemon)，不适合一次性 CLI 调用。

**预期收益**: 协作场景从"轮询"变为"实时推送"，消除延迟。

---

#### 2.8 完整 5 步同步协议

**问题**: 当前后端写操作有版本乐观锁，但缺少完整的冲突解决和增量同步流程。

**Zotero 原生同步协议**:

```
Step 1: GET /users/{id}/items?limit=0
        → 读取 Last-Modified-Version (当前库版本号)

Step 2: GET /users/{id}/items?since={lastSyncVersion}&limit=100
        → 拉取自上次同步以来的远程变更

Step 3: POST/PUT/PATCH (本地待上传的变更)
        → 带 If-Unmodified-Since-Version 头

Step 4: 处理 409/412 冲突
        → 重新获取最新状态 → 合并非冲突字段 → 重试

Step 5: 存储 Last-Modified-Version 到本地
        → 作为下次同步的基准点
```

**关键机制**:
- **条件请求不限频**: `If-Modified-Since-Version` 检查不计入速率限制
- **versions 端点轻量**: `?format=versions&since=N` 仅返回 `{key: version}` 映射，无完整数据
- **Last-Writer-Wins**: 服务端基于版本号判定胜者

**新增命令**:
```
zot sync                    # 执行完整的双向同步
zot sync --dry-run          # 预览将要同步的变更
zot sync --conflict=local   # 冲突解决策略: local|remote|merge
```

**预期收益**: 离线编辑后可靠同步，多端编辑冲突自动解决。

---

#### 2.9 Zotero 9 新字段利用

**问题**: Zotero 9 新增的字段未被项目查询利用。

以下字段位置已从 Zotero 源码 (`resource/schema/userdata.sql` + 迁移步骤) 中确认：

##### 2.9.1 `itemAttachments.lastRead` — 最近阅读时间

- **位置**: `itemAttachments.lastRead` (INTEGER, Unix 时间戳)
- **来源**: Schema 迁移 **step 124**
- **索引**: `CREATE INDEX itemAttachments_lastRead ON itemAttachments(lastRead)` ✅ 已有索引
- **含义**: 用户在 Zotero PDF Reader 中最后打开/阅读该附件的时间

**当前状态**: 项目 `loadAttachmentsByParentItemIDs`（`local_loaders.go:345`）未查询此列。

**实现方式**: 在附件查询中增加 `ia.lastRead` 列：

```sql
-- 当前查询 (local_loaders.go:345) 增加 COALESCE(ia.lastRead, 0)
SELECT
    ia.parentItemID,
    i.key,
    it.typeName,
    ...
    COALESCE(ia.lastRead, 0)          -- ← 新增
FROM itemAttachments ia
...
```

**新增 find 参数**:
```
zot find --query "xxx" --sort last-read           # 按最近阅读排序
zot find --all --recently-read --within 7d        # 最近 7 天读过的
zot show KEY --full                               # 显示 lastRead 时间
```

---

##### 2.9.2 Citation Key — 引文键

- **位置**: **非独立列**，存储在 EAV 模型中：`fieldsCombined.fieldName = 'citationKey'`
- **查询方式**: 与 title/DOI 等字段相同，通过 `itemData → itemDataValues → fieldsCombined` JOIN

**当前状态**: 项目的 `localFindQuery`（`local_find.go:14`）已支持任意 `fieldName` 的 EAV 查询模式，但 `find` 命令前端未暴露 citation-key 过滤。

**实现方式**: 无需改 SQL，仅需在 CLI 层增加参数映射：

```go
// commands_find.go 中新增
if opts.CitationKey != "" {
    // 复用现有 query match 机制，将 citationKey 加入搜索字段列表
    // 或在 localFindQuery 中增加专门条件:
    // AND EXISTS (SELECT 1 FROM itemData ... WHERE f.fieldName = 'citationKey' AND v.value = ?)
}
```

**新增命令/参数**:
```
zot find --citation-key "lee2026science"         # 按 Citation Key 精确查找
zot show KEY --full                              # 显示 Citation Key 字段
```

---

##### 2.9.3 `groupItems` — 群组库归属追踪

- **位置**: `groupItems` 表
- **列**: `createdByUserID INT`, `lastModifiedByUserID INT`
- **外键**: → `users(userID)` ON DELETE SET NULL
- **说明**: 仅群组库有数据，个人库无意义

```sql
CREATE TABLE groupItems (
    itemID INTEGER PRIMARY KEY,
    createdByUserID INT,
    lastModifiedByUserID INT,
    FOREIGN KEY (itemID) REFERENCES items(itemID) ON DELETE CASCADE,
    FOREIGN KEY (createdByUserID) REFERENCES users(userID),
    FOREIGN KEY (lastModifiedByUserID) REFERENCES users(userID)
);
```

**当前状态**: 项目完全未使用此表。

**适用场景**: 团队协作用户追踪谁添加/修改了哪些文献。

**新增命令**:
```
zot find --query "xxx" --added-by USER_ID          # 筛选某用户添加的
zot find --query "xxx" --modified-by USER_ID       # 筛选某用户修改的
zot show KEY --full                                # 显示 Added By / Modified By
```

---

### P1-Local — 本地数据库优化（纯 Local/Hybrid 模式）

> 以下优化针对 `LocalReader` 的 SQLite 查询层，不依赖网络，对 local 和 hybrid 模式直接生效。

#### 2.10 利用 Zotero 内置全文词表（`fulltextWords` / `fulltextItemWords`）

**现状**: 项目维护独立的 FTS5 索引（存放在 `{data_dir}/.zotero_cli/fulltext/`），需要自行提取 PDF 文本并建索引。

**Zotero 内置索引** — Zotero 自身维护一套全文词表系统（非 FTS5，而是倒排索引）：

| 表名 | 用途 |
|------|------|
| `fulltextWords` | 词字典 `(wordID PK, word UNIQUE)` |
| `fulltextItemWords` | 词-条目映射 `(wordID, itemID) PK` |
| `fulltextItems` | 索引元数据 `(itemID, indexedPages, indexedChars, version, synced)` |

**可用查询示例**:
```sql
-- 搜索包含指定词汇的附件
SELECT i.key, COUNT(fiw.wordID) as match_count
FROM fulltextItemWords fiw
JOIN fulltextWords fw ON fiw.wordID = fw.wordID
JOIN items i ON fiw.itemID = i.itemID
WHERE fw.word IN ('nitrate', 'escherichia', 'kidney')
GROUP BY fiw.itemID
ORDER BY match_count DESC
```

**对比分析**:

| 维度 | 项目现有 FTS5 | Zotero 内置 fulltext* |
|------|--------------|----------------------|
| 索引类型 | FTS5 虚拟表 | 倒排词表 (B-tree) |
| 分词支持 | ✅ SQLite 内置分词器 | ❌ 仅精确单词匹配 |
| 短语搜索 | ✅ `MATCH '"phrase"'` | ❌ 不支持 |
| 排序/相关度 | ✅ FTS5 rank/bm25 | ⚠️ 仅词频计数 |
| Snippet 提取 | ✅ `snippet()` | ❌ 无位置信息 |
| 数据来源 | 自行 PyMuPDF 提取 | Zotero PDF Reader 自动建立 |
| 索引维护成本 | 高（需自行提取+建索引） | **零成本（Zotero 已维护）** |

**结论**: Zotero 内置词表**适合作为预筛选层**，不适合替代现有 FTS5。

**推荐策略 — 双层全文检索**:

```
用户执行: zot find --fulltext "kidney disease"

Layer 1: 快速预筛 (Zotero 内置词表)
  ├─ 查询 fulltextWords + fulltextItemWords
  ├─ 返回候选 itemID 列表 (毫秒级，零 I/O)
  └─ 若结果集为空 → 跳到 Layer 2

Layer 2: 精确检索 (现有 FTS5 缓存)
  ├─ 对候选结果查 FTS5 索引获取 snippet
  ├─ 若 FTS5 未命中 → 触发 PDF 文本提取
  └─ 返回带上下文片段的结果
```

**预期收益**:
- 大幅减少 FTS5 未命中时的 PDF 扫描次数（内置词表已有数据时直接返回）
- 对于已在 Zotero 中打开过的 PDF，无需重新提取文本
- 预估 `--fulltext` 查询延迟从 ~6s 降至 <1s（命中内置词表时）

**涉及文件**: `internal/backend/local_fulltext.go`, `internal/backend/local.go`

---

#### 2.11 `deletedItems` 表 — 已删除条目查询与恢复

**现状**: 项目完全未使用 `deletedItems` 表。

**Schema** (来自 `userdata.sql`):
```sql
CREATE TABLE deletedItems (
    itemID INTEGER PRIMARY KEY,
    dateDeleted DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY (itemID) REFERENCES items(itemID) ON DELETE CASCADE
);
CREATE INDEX deletedItems_dateDeleted ON deletedItems(dateDeleted);
```

**行为说明**:
- 条目被删除时插入一行（`dateDeleted = CURRENT_TIMESTAMP`）
- `items` 表中对应行同时被删除（FK CASCADE）
- 注意：当 Zotero 执行 purge（彻底清除）时，`deletedItems` 行也会被级联删除
- 主要用于本地撤销/回收站功能

**利用场景**:

| 场景 | 实现方式 |
|------|----------|
| 列出最近删除的条目 | `SELECT * FROM deletedItems ORDER BY dateDeleted DESC` |
| 按时间范围筛选删除记录 | `WHERE dateDeleted >= date('now', '-30 days')` |
| 统计删除频率 | 配合 `stats` 命令展示删除趋势 |
| 回收站浏览 | 类似 Zotero 主界面的 Trash 视图 |

**新增命令**:
```
zot trash list [--within 30d]     # 浏览回收站（已删除但未 purge）
zot trash stats                   # 删除统计（总数、按日期分布）
```

**注意**: 此功能仅适用于 Zotero 尚未 purge 的条目。一旦 purge，数据不可恢复（除非走 Web API 版本历史）。

**涉及文件**: 新增 `commands_trash.go`, `local_loaders.go` 增加查询方法

---

#### 2.12 `itemAnnotations.authorName` — 标注作者字段

**发现**: 项目代码（`local_loaders.go:499`, `local_loaders.go:565`）**已经查询了 `authorName` 列**：
```sql
COALESCE(ia.authorName, '')
```
但 `domain.Annotation` 结构体（`domain/types.go`）**没有 `AuthorName` 字段**，Scan 结果赋值给了局部变量后未被使用。

**修复方案**: 在 domain model 中补全字段即可零成本获得此信息：

```go
// internal/domain/types.go - Annotation 结构体
type Annotation struct {
    // ... 现有字段 ...
    AuthorName string `json:"author_name,omitempty"`  // ← 新增
}
```

同时在 Scan 后赋值: `a.AuthorName = authorName`

**预期收益**: 零 SQL 改动，仅增加一个结构体字段。可区分用户自己创建的标注 vs 外部导入的标注（如 Read Aloud 自动生成的标注）。

**涉及文件**: `internal/domain/types.go`, `internal/backend/local_loaders.go` (2 处 Scan)

---

#### 2.13 附件查询增强 — `syncState` / `contentType` 索引利用

**现状**: `loadAttachmentsByParentItemIDs`（`local_loaders.go:345`）查询了 `contentType` 和 `linkMode`，但未利用以下已有索引：

| 索引名 | 定义 | 可用于 |
|--------|------|--------|
| `itemAttachments_contentType` | `(contentType)` | 按 MIME 类型快速过滤 (`--attachment-type pdf`) |
| `itemAttachments_syncState` | `(syncState)` | 区分同步状态（本地新增 vs 已同步） |
| `itemAttachments_lastProcessedModificationTime` | `(lastProcessedModificationTime)` | 检测文件是否在外部被修改 |

**优化方向**:

1. **`--attachment-type` 过滤下推到 SQL**: 当前 `matchesAttachmentFilters` 是 Go 层过滤。对于 `--has-pdf` 这类高频场景，可在 WHERE 子句中增加 `AND ia.contentType = 'application/pdf'`，利用 `itemAttachments_contentType` 索引提前缩减结果集。

2. **`syncState` 暴露**: 可让用户识别哪些附件尚未同步到云端：
   ```
   zot find --query "xxx" --unsynced-only    # 仅显示未同步的附件
   ```

3. **文件变更检测**: `lastProcessedModificationTime` 可用于判断 PDF 是否在被外部工具修改后需要重新提取全文。

**涉及文件**: `internal/backend/local_find.go` (localFindQuery), `internal/backend/local_loaders.go`

---

#### 2.14 未使用的 Zotero 表 — 潜在价值评估

以下表存在于 Z9 schema 中，项目当前未使用：

| 表名 | 说明 | 利用价值 |
|------|------|----------|
| `retractedItems` | 论文撤回追踪 | **高** — 学术诚信检查，find 时标记已撤回论文 |
| `syncQueue` | 上传队列管理 | **中** — 查看/管理待同步项，调试同步问题 |
| `feeds` / `feedItems` | RSS 订阅源 | **低—中** — RSS 文献自动入库 |
| `publicationsItems` | "My Publications" 关联 | **低** — 个人出版物列表 |
| `deletedCollections` | 已删除集合 | **中** — 与 deletedItems 配合的完整回收站 |
| `deletedSearches` | 已保存搜索 | **低** — 使用频率低 |

**推荐优先实现 `retractedItems`**:

```sql
-- 撤回检测示例
SELECT i.key, r.doi, r.dateRetracted
FROM retractedItems r
JOIN itemData d ON d.itemID = r.itemID
JOIN itemDataValues v ON v.valueID = d.valueID
JOIN fieldsCombined f ON f.fieldID = d.fieldID
WHERE f.fieldName = 'DOI' AND v.value = ?
```

**集成点**: `zot show KEY` 或 `zot find` 输出中标注 `[RETRACTED]` 警告。

---

## 三、Connector API (端口 23119) 评估

### 3.1 能力概述

Zotero 启动后在 `127.0.0.1:23119` 提供 HTTP 服务，主要用于文字处理器集成。

**协议特点**:
- 事务型：一次会话包含多轮 request→sub-command→respond
- 设计目标：Word/LibreOffice 插件插入引文
- 不适合常规数据查询

### 3.2 可利用的场景

| 场景 | 可行性 | 说明 |
|------|--------|------|
| 检测 Zotero 是否运行 | 高 | 健康检查端点 |
| 获取当前选中条目 | 中 | 取决于是否有公开接口 |
| 触发同步 | 低 | 未公开此接口 |
| 插入引文 Word | 低 | 需要完整的事务协议实现 |

**结论**: 对 CLI 工具价值有限，优先级放低。如果未来需要深度桌面集成再考虑。

---

## 四、实施路线图

### Phase 1 — Web API 快速收益（预计 2-3 天）

> 走 Web API，需网络连接。

| 任务 | 文件 | 工作量 |
|------|------|--------|
| 条件请求 304 缓存 | `web.go`, `reader.go` | 0.5d |
| 导出格式透传 (20+格式) | `client_export.go`, `commands_*.go` | 0.5d |
| 批量写入打包 (50对象) | `client_items.go` | 1d |

### Phase 1-Local — 本地数据库快速收益（预计 1-2 天）

> 纯本地 SQLite 操作，无需网络，local/hybrid 模式直接受益。

| 任务 | 文件 | 工作量 | 说明 |
|------|------|--------|------|
| 补全 `authorName` 字段 | `domain/types.go`, `local_loaders.go` | **0.1d** | 已查 SQL，仅补 domain struct |
| `lastRead` 列查询与排序 | `local_loaders.go`, `local_find.go`, `commands_find.go` | 0.5d | 有索引，直接可用 |
| Citation Key 过滤/显示 | `local_find.go`, `commands_find.go` | 0.5d | EAV 复用现有模式 |
| `--has-pdf` SQL 下推 | `local_find.go` | 0.25d | 利用 contentType 索引预筛 |
| `deletedItems` 回收站命令 | 新增 `commands_trash.go` | 0.5d | 简单单表查询 |

### Phase 2 — 功能增强（预计 5-7 天）

| 任务 | 工作量 | 依赖 | 类型 |
|------|--------|------|------|
| Full-text API 对接 | 2d | 无 | Web |
| OAuth 登录流程 | 2d | 无 | Web |
| Translation Server 导入 | 2d | Docker 环境 | Web |
| Zotero 内置词表双层检索 | 1.5d | 无 | **Local** |
| `retractedItems` 撤回检测 | 0.5d | 无 | **Local** |
| `groupItems` 协作归属查询 | 1d | 无 | **Local** |

### Phase 3 — 架构升级（预计 7-10 天）

| 任务 | 工作量 | 依赖 | 类型 |
|------|--------|------|------|
| WebSocket 实时监听 | 3d | daemon 模式 | Web |
| 完整同步协议 | 4d | Phase 1 | Web |
| 附件 syncState / 变更检测 | 1d | 无 | **Local** |
| RSS feeds 集成 | 2d | 无 | **Local** |

---

## 五、风险与注意事项

### 5.1 API 速率限制 (Web)

- 官方未公布确切数字，必须严格遵循 `Backoff` / `Retry-After` 响应头
- **条件请求不受限** — 这是使用 2.1 方案的核心理由
- 建议在 client 层增加自适应限速器

### 5.2 SQLite 锁竞争 (Local)

- 直读 `zotero.sqlite` 要求 Zotero 空闲或关闭
- 当前项目的 **持久化快照缓存** 已处理此场景（实测已验证：Zotero 运行中自动走缓存，后续调用 ~0.3s）
- 写入操作切勿直接操作 SQLite，必须走 Web API
- **Journal 模式注意**: Zotero 使用 journal 模式（非 WAL），snapshot 复制时需复制 `-journal` 文件；快照缓存在 `{dataDir}/.zotero_cli/snapshot/` 下，基于 mtime 自动失效重建

### 5.3 版本兼容性

- Web API v3 向后兼容，REST 接口稳定
- 本地 SQLite schema 在 Z7→Z8→Z9 之间保持兼容（**2026-04-22 实测验证通过**，6716 条目/33 集合）
- 关键 schema 事实（从源码确认）：
  - `fieldsCombined` 是 **TABLE**（非 VIEW），启动时从 `fields` + `customFields` 合并填充
  - `items` 表结构在 Z7→Z9 期间 **无新增列**（Last Read 在 `itemAttachments` 上，Citation Key 走 EAV）
  - `itemAnnotations.authorName` 列于迁移 step 119/120 加入，项目已查询但未使用
- 未来 Z10 可能引入 breaking changes，需关注 changelog

### 5.4 Translation Server 文档缺失

- 官方文档页面返回 404
- 需参考 Docker 镜像源码或社区实践
- 建议先做 PoC 验证可行性

### 5.5 Zotero 内置全文词表限制

- **非 FTS5**: 是简单的倒排索引（wordID → itemID 映射），不支持：
  - 分词/词干提取（仅精确单词匹配）
  - 短语搜索 / 近邻搜索
  - 排序算法（仅有词频计数）
  - Snippet / 上下文片段提取
- **不能替代**现有 FTS5 缓存，但可作为**预筛选层**减少 PDF 扫描
- 词表数据依赖 Zotero PDF Reader 打开过该 PDF 才会建立；未打开过的 PDF 无词表数据

### 5.6 deletedItems 数据生命周期

- `deletedItems` 行在 Zotero **purge 后级联删除**，不可恢复
- 个人库的 purge 可能自动触发（取决于用户设置和存储配额）
- 不应作为长期审计日志使用，仅适合短期回收站浏览
- 可靠的删除追踪应依赖 Web API 的版本历史 (`?since=`)

---

## 六、参考资源

### Web API 文档

| 资源 | URL |
|------|-----|
| Web API v3 基础 | https://www.zotero.org/support/dev/web_api/v3/basics |
| 写操作规范 | https://www.zotero.org/support/dev/web_api/v3/write_requests |
| 同步协议 | https://www.zotero.org/support/dev/web_api/v3/syncing |
| 全文内容 API | https://www.zotero.org/support/dev/web_api/v3/fulltext_content |
| 流式 WebSocket API | https://www.zotero.org/support/dev/web_api/v3/streaming_api |
| OAuth 1.0a | https://www.zotero.org/support/dev/web_api/v3/oauth |
| v2→v3 变更 | https://www.zotero.org/support/dev/web_api/v3/changes_from_v2 |
| Connector 协议 | https://www.zotero.org/support/dev/client_coding/http_integration_protocol |

### 本地 Schema 源码（Zotero GitHub）

| 资源 | URL |
|------|-----|
| 基础 Schema 定义 (`userdata.sql`) | https://github.com/zotero/zotero/blob/master/resource/schema/userdata.sql |
| Schema 迁移脚本 (`schema.js`) | https://github.com/zotero/zotero/blob/master/schema.js |
| 完整迁移步骤列表 | 搜索 `_migrateUserDataSchema` in `schema.js` |

### 其他

| 资源 | URL |
|------|-----|
| Zotero 9 Changelog | https://www.zotero.org/support/changelog |
| pyzotero (Python 参考) | https://github.com/urschrei/pyzotero |
| zotero-api-client (JS 参考) | https://github.com/zotero/zotero-api-client |
