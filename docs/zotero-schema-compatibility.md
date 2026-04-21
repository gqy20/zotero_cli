# Zotero 7→9 SQLite Schema 兼容性参考

> 基线: Zotero 9 `userdata.sql` schema version **125** | 编译日期: 2026-04-22

本文档整理 Zotero 7 至 Zotero 9 之间所有已知的 SQL Schema 变更，供 CLI 项目做版本兼容处理时查阅。

---

## 总览

| 维度 | 状态 |
|------|------|
| 核心表 `items` | **稳定** — Z7→Z9 无列变更 |
| EAV 元数据模型 (`itemData`/`itemDataValues`) | **稳定** — 字段名通过 `fieldsCombined` 查 |
| 标注表 `itemAnnotations` | **有变更** — 新增 `authorName`, `type` 改为 INTEGER |
| 附件表 `itemAttachments` | **有变更** — 新增 `lastRead`, `syncState` 等 |
| 全文索引 | **独立系统** — 非 FTS5，Zotero 自研倒排索引 |
| 组协作表 `groupItems` | **新增** — Z8/Z9 协作功能 |

---

## 表级变更清单

### 1. `items` — 条目主表 ✅ 稳定

| 列名 | 类型 | Z7 | Z8 | Z9 | 说明 |
|------|------|----|----|----|------|
| itemID | INTEGER PK | ✓ | ✓ | ✓ | 自增主键 |
| itemTypeID | INTEGER FK | ✓ | ✓ | ✓ | → itemTypes.itemTypeID |
| dateAdded | TEXT | ✓ | ✓ | ✓ | ISO 8601 |
| dateModified | TEXT | ✓ | ✓ | ✓ | ISO 8601 |
| version | INTEGER | ✓ | ✓ | ✓ | 同步版本号 |
| syncState | INTEGER | ✓ | ✓ | ✓ | 0=synced, 1=modified, etc. |
| key | TEXT UNIQUE | ✓ | ✓ | ✓ | Zotero itemKey (8位) |

> **CLI 影响**: 无。当前查询完全兼容。

### 2. `itemData` / `itemDataValues` / `fieldsCombined` — EAV 元数据模型 ✅ 稳定

这是 Zotero 存储条目元数据的核心模式：

```
items.itemID → itemData(itemID, fieldID, valueID)
                          ↓              ↓
                    fieldsCombined    itemDataValues(valueID, value)
```

**关键事实**: `fieldsCombined` 是一个 **TABLE（不是 VIEW）**，在 Zotero 启动时由 `fields` + `customFields` 合并填充。

CLI 当前通过以下方式查询元数据：

```sql
-- local_find.go 中的典型查询模式
LEFT JOIN itemData d ON d.itemID = i.itemID
LEFT JOIN itemDataValues v ON v.valueID = d.valueID
LEFT JOIN fieldsCombined f ON f.fieldID = d.fieldID
--然后用 CASE WHEN f.fieldName = 'title' THEN v.value END 聚合
```

**常用 fieldName 列表（全部通过 EAV 存储）**:

| fieldName | 用途 | CLI 是否使用 |
|-----------|------|-------------|
| title | 标题 | ✓ |
| date | 出版日期 | ✓ |
| DOI | DOI | ✓ |
| url | URL | ✓ |
| publicationTitle | 期刊/出版物名 | ✓ |
| proceedingsTitle | 会议录名 | ✓ |
| bookTitle | 书名 | ✓ |
| volume | 卷 | ✓ |
| issue | 期 | ✓ |
| pages | 页码 | ✓ |
| abstractNote | 摘要 | ✗ |
| citationKey | 引用键 (Better BibTeX 等) | ✗ |
| shortTitle | 短标题 | ✗ (仅查询条件) |
| filename | 附件文件名 | ✓ (附件查询) |

> **CLI 影响**: 无。EAV 模式本身跨版本稳定。
>
> **注意**: Citation Key 不是独立列，存储为 `fieldName='citationKey'` 的 EAV 行。如需支持需走同一查询路径。

### 3. `itemAnnotations` — 标注表 ⚠️ 有变更

| 列名 | 类型 | Z7 | Z8 | Z9 | 迁移步骤 | 说明 |
|------|------|----|----|----|---------|------|
| itemID | INTEGER PK/FK | ✓ | ✓ | ✓ | — | → items.itemID |
| parentItemID | INTEGER FK | ✓ | ✓ | ✓ | — | 父条目(通常是附件) |
| type | **INTEGER** | TEXT | INT | INT | ~119/120 | **类型从 TEXT 改为 INTEGER** |
| text | TEXT | ✓ | ✓ | ✓ | — | 高亮文本 |
| comment | TEXT | ✓ | ✓ | ✓ | — | 用户备注 |
| color | TEXT | ✓ | ✓ | ✓ | — | "#ffd400" |
| pageLabel | TEXT | ✓ | ✓ | ✓ | — | "2" |
| position | TEXT (JSON) | ✓ | ✓ | ✓ | — | 坐标 JSON |
| sortIndex | TEXT | ✓ | ✓ | ✓ | — | 排序键 |
| **authorName** | **TEXT** | ✗ | ✓ | ✓ | **119/120** | **Z8+ 新增** |
| isExternal | INTEGER | ✗ | ✓ | ✓ | ~119/120 | 外部标注标记 |

**type 字段值映射 (INTEGER)**:

| 值 | 含义 | CLI 映射 |
|----|------|---------|
| 0 | highlight | `"highlight"` |
| 1 | note | `"note"` |
| 2 | image | `"image"` |
| 3 | ink | `"ink"` |
| 4 | area | `"area"` |

> **CLI 影响分析**:
>
> - `authorName`: 已在 `loadAnnotations()` 和 `loadAnnotationsByItemIDs()` 中查询 (`COALESCE(ia.authorName, '')`)，但扫描到局部变量后**未赋值给 domain.Annotation**（Annotation 结构体无 AuthorName 字段）。**零成本修复**: 在 Annotation struct 中添加 `AuthorName string` 字段即可。
> - `isExternal`: 已正确使用，映射到 `domain.Annotation.IsExternal`。
> - `type` 为 INTEGER: 已通过 `annotationTypeString(int)` 正确处理。
>
> **兼容建议**: 如需支持 Z7 旧库，对 `authorName` 和 `isExternal` 列做 `COALESCE` 保护（当前代码已这样做）。

### 4. `itemAttachments` — 附件表 ⚠️ 有变更

| 列名 | 类型 | Z7 | Z8 | Z9 | 迁移步骤 | 说明 |
|------|------|----|----|----|---------|------|
| itemID | INTEGER PK/FK | ✓ | ✓ | ✓ | — | → items.itemID |
| parentItemID | INTEGER FK | ✓ | ✓ | ✓ | — | 父条目 |
| contentType | TEXT | ✓ | ✓ | ✓ | — | MIME 类型 |
| path | TEXT | ✓ | ✓ | ✓ | — | storage:/attachments: 路径 |
| linkMode | INTEGER | ✓ | ✓ | ✓ | — | 0=imported, 1=linked, etc. |
| **lastRead** | **INTEGER** | ✗ | ✗ | ✓ | **124** | **Z9 新增**，Unix 时间戳 |
| syncState | INTEGER | ✓ | ✓ | ✓ | — | 同步状态 |
| storageModTime | INTEGER | ✓ | ✓ | ✓ | — | 存储修改时间 |
| storageHash | TEXT | ✓ | ✓ | ✓ | — | 内容哈希 |
| lastProcessedModificationTime | INTEGER | ✗ | ✓ | ✓ | — | 最后处理时间 |

**linkMode 枚举值**:

| 值 | 含义 | CLI 映射 |
|----|------|---------|
| 0 | imported_file | `"imported"` |
| 1 | linked_file | `"linked"` |
| 2 | embedded_image | `"embedded"` |
| 3 | imported_url | `"imported_url"` |

> **CLI 影响**:
>
> - `lastRead` (迁移步骤 124): **Z9 新增**，CLI 当前未使用。可用于"最近阅读"排序或过滤。
> - 其余字段: CLI 已正确查询 `contentType`, `linkMode`, `path`。
>
> **优化机会**: 利用 `lastRead` 实现 `find --recently-read` 或按阅读时间排序。

### 5. `collections` / `collectionItems` — 收藏夹 ✅ 稳定

| 表 | 关键列 | 状态 |
|----|--------|------|
| collections | collectionID(PK), key, collectionName, parentCollectionID, version | 稳定 |
| collectionItems | collectionID(FK), itemID(FK) | 稳定 |

> **CLI 影响**: 无。`ListCollections()` 查询完整可用。

### 6. `tags` / `itemTags` — 标签 ✅ 稳定

| 表 | 关键列 | 状态 |
|----|--------|------|
| tags | tagID(PK), name(UNIQUE) | 稳定 |
| itemTags | itemID(FK), tagID(FK), type(INTEGER) | 稳定 |

> **CLI 影响**: 无。

### 7. `creators` / `itemCreators` / `creatorTypes` — 作者 ✅ 稳定

| 表 | 关键列 | 状态 |
|----|--------|------|
| creators | creatorID(PK), firstName, lastName | 稳定 |
| itemCreators | itemID(FK), creatorID(FK), creatorTypeID(FK), orderIndex | 稳定 |
| creatorTypes | creatorTypeID(PK), creatorType, custom(INTEGER) | 稳定 |

> **CLI 影响**: 无。作者拼接逻辑 `TRIM(COALESCE(firstName,'') || ' ' || COALESCE(lastName,''))` 跨版本一致。

### 8. `savedSearches` / `savedSearchConditions` — 已保存搜索 ✅ 稳定

| 表 | 关键列 | 状态 |
|----|--------|------|
| savedSearches | searchID(PK), key, name, version, conditions(JSON) | 稳定 |
| savedSearchConditions | searchID(FK), conditionID, operator, value, required | 稳定 |

### 9. `itemRelations` / `relationPredicates` — 关系 ✅ 稳定

| 表 | 关键列 | 状态 |
|----|--------|------|
| itemRelations | itemID(FK), predicateID(FK), object(TEXT URI) | 稳定 |
| relationPredicates | predicateID(PK), predicate(UNIQUE) | 稳定 |

> **CLI 影响**: 无。关系查询使用 LIKE 匹配 `/items/{key}` 格式。

### 10. `groupItems` — 组协作 ⚠️ 可能新增

| 列名 | 类型 | Z7 | Z9 | 说明 |
|------|------|----|----|------|
| itemID | INTEGER PK/FK | ✓ | ✓ | → items.itemID |
| createdByUserID | INTEGER | ? | ✓ | 创建者用户 ID |
| lastModifiedByUserID | INTEGER | ? | ✓ | 最后修改者用户 ID |

> **CLI 影响**: 个人库不涉及此表。仅在 group library 场景下需要关注。

### 11. `itemNotes` — 子笔记 ✅ 稳定

| 列名 | 类型 | 状态 |
|------|------|------|
| itemID (FK), parentItemID (FK), note (TEXT/HTML), noteItemType (TEXT) | 全部稳定 | |

> **CLI 影响**: 无。

---

## 辅助表（*Combined 系列）

这些是运行时合并表，Zotero 启动时从内置 + 自定义字段填充：

| 表名 | 来源 | 用途 | CLI 使用情况 |
|------|------|------|-------------|
| fieldsCombined | fields + customFields | 元数据字段名 ↔ fieldID 映射 | **核心依赖** — 所有元数据查询都 JOIN 此表 |
| itemTypesCombined | itemTypes + customItemTypes | 条目类型 | 间接使用 (JOIN itemTypes) |
| creatorTypesCombined | creatorTypes + customCreatorTypes | 作者角色类型 | 间接使用 |

> **重要**: `fieldsCombined` 是 TABLE 不是 VIEW。这意味着:
> - 可以直接 JOIN（CLI 当前做法正确）
> - 不需要担心 VIEW 定义变更
> - 但字段集取决于 Zotero 版本（新版本可能添加新的内置字段）

---

## 全文检索系统（非标准 FTS5）

Zotero 的全文检索 **不是** SQLite FTS5，而是自研的倒排索引系统：

| 表 | 用途 |
|----|------|
| fulltextWords | 词表 (wordID, word) |
| fulltextItemWords | 词条-条目映射 (wordID, itemID, slp) |
| fulltextItems | 全文项元数据 (itemID, itemIDVersion, indexedWords, indexedPages, totalCharacters) |

**CLI 策略**: 项目自行构建了 FTS5 缓存索引 (`local_fulltext.go`)，不依赖 Zotero 内置全文系统。两者独立运作。

---

## 数据库 PRAGMA 设置

来自 Zotero 源码 `db.js`:

```sql
PRAGMA locking_mode = EXCLUSIVE;   -- 独占锁（Zotero 运行时独占 DB）
PRAGMA journal_mode = WAL;         -- WAL 日志模式
PRAGMA synchronous = NORMAL;       -- 同步级别
PRAGMA cache_size = -8000;         -- 8MB 缓存 (~8000KB)
PRAGMA foreign_keys = true;        -- 外键约束
```

> **CLI 影响**:
> - `locking_mode = EXCLUSIVE` 是 CLI 做 snapshot 回退的根本原因 — Zotero 运行时无法获得读锁
> - WAL 模式允许并发读者，但 EXCLUSIVE 锁优先级更高
> - CLI 的 `withReadableDB()` → live 尝试 → snapshot 回退策略正是为此设计

---

## 已知迁移步骤速查

| 步骤号 | 变更内容 | 影响表 | CLI 处理状态 |
|--------|---------|--------|-------------|
| 119/120 | `itemAnnotations` 新增 `authorName`, `isExternal`; `type` 从 TEXT 改为 INTEGER | itemAnnotations | ✅ 已处理 (COALESCE + int 映射) |
| 124 | `itemAttachments` 新增 `lastRead` (INTEGER) | itemAttachments | ⚠️ 未使用 (可扩展) |
| 125 | 当前最新 schema 版本 | — | 基线 |

---

## CLI 代码与 Schema 对照

### 当前查询覆盖度

| 表 | CLI 方法 | 查询的列 | 未用但有价值的列 |
|----|---------|----------|-----------------|
| items | FindItems, GetItem | itemID, key, version, itemTypeID, dateAdded, dateModified | — (全量使用) |
| itemData + values + fields | FindItems, GetItem | EAV 聚合 (title/date/DOI/url/...) | abstractNote, citationKey |
| itemAttachments | loadAttachments, loadAttachmentsByParentIDs | itemID, parentItemID, contentType, linkMode, path | **lastRead**, syncState, storageHash |
| itemAnnotations | loadAnnotations, loadAnnotationsByItemIDs | itemID, parentItemID, type, text, comment, color, pageLabel, position, sortIndex, isExternal, **authorName**(查了未存) | — |
| itemNotes | loadNotes | itemID, parentItemID, note | noteItemType |
| collections + ci | ListCollections, loadCollections | collectionID, key, collectionName, parentCollectionID | version |
| tags + it | ListTags, loadTags | tagID, name | type (in itemTags) |
| creators + ic | loadCreators, loadCreatorsByItemIDs | creatorID, firstName, lastName, orderIndex | — |
| itemRelations | loadOutgoing/IncomingRelations | itemID, predicateID, object | — |
| savedSearches | GetLibraryStats | COUNT(*) only | — |

### 零成本改进项

| # | 改进 | 文件 | 工作量 |
|---|------|------|--------|
| 1 | `Annotation` 添加 `AuthorName` 字域，`loadAnnotations` 中赋值 | `types.go` + `local_loaders.go` | ~5 行 |
| 2 | `Attachment` 添加 `LastRead` 字域，查询时读取 | `types.go` + `local_loaders.go` | ~10 行 |
| 3 | `FindOptions` 支持 `--sort recently-read` | `local_find.go` | ~15 行 |

---

## 版本检测策略

CLI 可通过以下方式检测 Zotero/schema 版本：

```sql
-- 方法 1: 检测字段是否存在 (推荐)
SELECT COUNT(*) FROM pragma_table_info('itemAnnotations') WHERE name = 'authorName';
-- 返回 1 = Z8+, 0 = Z7 或更早

-- 方法 2: 检测列类型变化
SELECT typeof(type) FROM itemAnnotations LIMIT 1;
-- 返回 "integer" = Z8+, "text" = Z7

-- 方法 3: 检测 lastRead (Z9 特有)
SELECT COUNT(*) FROM pragma_table_info('itemAttachments') WHERE name = 'lastRead';
-- 返回 1 = Z9+, 0 = Z8 或更早
```

> **当前策略**: CLI 统一使用 `COALESCE(column, '')` / `COALESCE(column, 0)` 做防御性查询，即使列不存在也不会报错（纯 Go SQLite 驱动行为可能不同，需实测验证）。
>
> **建议**: 对于必须区分版本的场景，先用 `pragma_table_info()` 探测再决定查询语句。

---

## 附录: 完整 Z9 DDL 参考 (v125)

以下是从 Zotero 9 源码提取的核心表 DDL，供快速对照。

```sql
-- === 核心实体表 ===
CREATE TABLE items (
    itemID          INTEGER PRIMARY KEY,
    itemTypeID      INTEGER NOT NULL,
    dateAdded       TEXT NOT NULL DEFAULT '',
    dateModified    TEXT NOT NULL DEFAULT DEFAULT_TIMESTAMP,
    version         INTEGER NOT NULL DEFAULT 0,
    syncState       INTEGER NOT NULL DEFAULT 0,
    key             TEXT NOT NULL DEFAULT '' UNIQUE
);

CREATE TABLE itemTypes (
    itemTypeID      INTEGER PRIMARY KEY,
    typeName        TEXT NOT NULL UNIQUE,
    custom          INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE itemData (
    itemID          INTEGER NOT NULL REFERENCES items(itemID),
    fieldID         INTEGER NOT NULL,
    valueID         INTEGER NOT NULL,
    PRIMARY KEY (itemID, fieldID)
);

CREATE TABLE itemDataValues (
    valueID         INTEGER PRIMARY KEY,
    value           TEXT NOT NULL
);

CREATE TABLE fieldsCombined (
    fieldID         INTEGER PRIMARY KEY,
    fieldName       TEXT NOT NULL UNIQUE,
    custom          INTEGER NOT NULL DEFAULT 0,
    label           TEXT
);

-- === 附件 ===
CREATE TABLE itemAttachments (
    itemID                              INTEGER PRIMARY KEY REFERENCES items(itemID),
    parentItemID                        INTEGER REFERENCES items(itemID),
    contentType                         TEXT,
    path                               TEXT,
    linkMode                           INTEGER NOT NULL DEFAULT 0,
    lastRead                           INTEGER,
    syncState                          INTEGER NOT NULL DEFAULT 0,
    storageModTime                     INTEGER,
    storageHash                        TEXT,
    lastProcessedModificationTime       INTEGER
);

-- === 标注 ===
CREATE TABLE itemAnnotations (
    itemID          INTEGER PRIMARY KEY REFERENCES items(itemID),
    parentItemID    INTEGER NOT NULL REFERENCES items(itemID),
    type            INTEGER NOT NULL DEFAULT 0,
    text           TEXT,
    comment        TEXT,
    color          TEXT,
    pageLabel      TEXT,
    position       TEXT,
    sortIndex      TEXT,
    authorName     TEXT,
    isExternal     INTEGER NOT NULL DEFAULT 0
);

-- === 笔记 ===
CREATE TABLE itemNotes (
    itemID          INTEGER PRIMARY KEY REFERENCES items(itemID),
    parentItemID    INTEGER NOT NULL REFERENCES items(itemID),
    note           TEXT,
    noteItemType   TEXT
);

-- === 收藏夹 ===
CREATE TABLE collections (
    collectionID        INTEGER PRIMARY KEY,
    collectionName      TEXT NOT NULL,
    parentCollectionID  INTEGER REFERENCES collections(collectionID),
    key                 TEXT NOT NULL DEFAULT '' UNIQUE,
    version             INTEGER NOT NULL DEFAULT 0,
    syncState           INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE collectionItems (
    collectionID    INTEGER NOT NULL REFERENCES collections(collectionID),
    itemID          INTEGER NOT NULL REFERENCES items(itemID),
    PRIMARY KEY (collectionID, itemID)
);

-- === 标签 ===
CREATE TABLE tags (
    tagID   INTEGER PRIMARY KEY,
    name    TEXT NOT NULL COLLATE NOCASE
);

CREATE TABLE itemTags (
    itemID  INTEGER NOT NULL REFERENCES items(itemID),
    tagID   INTEGER NOT NULL REFERENCES tags(tagID),
    type    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (itemID, tagID)
);

-- === 作者 ===
CREATE TABLE creators (
    creatorID   INTEGER PRIMARY KEY,
    firstName   TEXT,
    lastName    TEXT
);

CREATE TABLE itemCreators (
    itemID          INTEGER NOT NULL REFERENCES items(itemID),
    creatorID       INTEGER NOT NULL REFERENCES creators(creatorID),
    creatorTypeID   INTEGER NOT NULL,
    orderIndex      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (itemID, creatorID, orderIndex)
);

CREATE TABLE creatorTypes (
    creatorTypeID   INTEGER PRIMARY KEY,
    creatorType     TEXT NOT NULL UNIQUE,
    custom          INTEGER NOT NULL DEFAULT 0
);

-- === 关系 ===
CREATE TABLE itemRelations (
    itemID      INTEGER NOT NULL REFERENCES items(itemID),
    predicateID INTEGER NOT NULL,
    object      TEXT NOT NULL,
    PRIMARY KEY (itemID, predicateID, object)
);

CREATE TABLE relationPredicates (
    predicateID  INTEGER PRIMARY KEY,
    predicate    TEXT NOT NULL UNIQUE
);

-- === 已保存搜索 ===
CREATE TABLE savedSearches (
    searchID      INTEGER PRIMARY KEY,
    key           TEXT NOT NULL DEFAULT '' UNIQUE,
    name          TEXT NOT NULL,
    version       INTEGER NOT NULL DEFAULT 0,
    conditions    TEXT,
    syncState     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE savedSearchConditions (
    searchID      INTEGER NOT NULL REFERENCES savedSearches(searchID),
    conditionID   INTEGER NOT NULL,
    operator      TEXT,
    value         TEXT,
    required      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (searchID, conditionID)
);

-- === 全文索引 (Zotero 自研，非 FTS5) ===
CREATE TABLE fulltextWords (
    wordID  INTEGER PRIMARY KEY,
    word    TEXT NOT NULL
);

CREATE TABLE fulltextItemWords (
    wordID  INTEGER NOT NULL,
    itemID  INTEGER NOT NULL REFERENCES items(itemID),
    slp     BLOB NOT NULL,
    PRIMARY KEY (wordID, itemID)
);

CREATE TABLE fulltextItems (
    itemID              INTEGER PRIMARY KEY REFERENCES items(itemID),
    itemIDVersion       INTEGER NOT NULL DEFAULT 0,
    indexedWords        INTEGER NOT NULL DEFAULT 0,
    indexedPages        INTEGER NOT NULL DEFAULT 0,
    totalCharacters     INTEGER NOT NULL DEFAULT 0
);
```

---

> **维护说明**: 当 Zotero 发布新版本时，对比 `resource/schema/userdata.sql` 的 schema version 和 DDL 变更，更新本文档对应章节。
