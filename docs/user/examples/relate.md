# relate — 条目关系

查询和管理 Zotero 条目之间的显式关联、笔记内嵌引用，以及可视化关系网络。

## 命令

```bash
zot relate <item-key> [选项]
```

## 选项

| 选项 | 说明 |
|------|------|
| `--json` | JSON 格式输出 |
| `--aggregate` | 聚合模式：返回自身关系 + 子笔记关系 + 内嵌 citation（需 local/hybrid） |
| `--dot` | 输出 Graphviz DOT 格式，可用于渲染关系网络图 |
| `--predicate PRED` | 按谓词过滤（如 `dc:relation`、`owl:sameAs`） |
| `--add TARGET` | 添加关系到目标条目（需 `ZOT_ALLOW_WRITE=1`） |
| `--remove TARGET` | 删除与目标条目的关系（需 `ZOT_ALLOW_WRITE=1`） |
| `--dry-run`, `-n` | 预览写入操作但不执行 |
| `--from-file PATH` | 从 JSON 文件批量执行 add/remove 操作 |

## 三层关系模型

Zotero 中存在三种独立的关系机制，`relate` 命令全部覆盖：

```
┌─────────────────────────────────────────────────────────────┐
│ ① itemRelations — 显式关系（用户手动建立）                   │
│    zot relate <key>  默认查询此层                            │
│    颜色: #4a90d9 (蓝色)                                     │
├─────────────────────────────────────────────────────────────┤
│ ② 子笔记的 itemRelations — 笔记上建立的关系                 │
│    zot relate <key> --aggregate  Notes 字段                     │
│    颜色: #e8913a (橙色)                                     │
├─────────────────────────────────────────────────────────────┤
│ ③ 内嵌 Citation — 笔记正文中通过 Zotero 引用插件插入的文献   │
│    zot relate <key> --aggregate  Citations 字段               │
│    颜色: #7bc96f (绿色), 虚线边                             │
└─────────────────────────────────────────────────────────────┘
```

## 基础用法：查询显式关系

```bash
# 文本模式
zot relate SXJ9FYTK

# JSON 模式
zot relate SXJ9FYTK --json
```

### 输出示例（JSON）

```json
{
  "ok": true,
  "command": "relate",
  "data": [
    {
      "predicate": "dc:relation",
      "direction": "incoming",
      "target": {
        "key": "RG9BVZDH",
        "item_type": "thesis",
        "title": "鹅耳枥属和铁木属系统发育基因组学研究",
        "date": "2018-00-00 2018",
        "creators": ["刘建全 ", "杨勇志 ", "邱强 "],
        "tags": ["物种形成", "杂交"]
      }
    },
    {
      "predicate": "dc:relation",
      "direction": "outgoing",
      "target": {
        "key": "RG9BVZDH",
        "item_type": "thesis",
        "title": "鹅耳枥属和铁木属系统发育基因组学研究",
        "date": "2018-00-00 2018",
        "creators": ["刘建全 ", "杨勇志 ", "邱强 "],
        "tags": ["物种形成", "杂交"]
      }
    }
  ],
  "meta": { "read_source": "local" }
}
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `predicate` | 关系类型：`dc:relation`（相关）/ `owl:sameAs`（等同）/ `dc:isPartOf` 等 |
| `direction` | `outgoing`（本条目 → 目标）/ `incoming`（目标 → 本条目） |
| `target` | 关联条目信息：key / type / title / date / creators / tags |

## 聚合模式：三层关系一览

`--aggregate` 返回该条目及其所有子笔记的完整关系网络。

```bash
# 文本模式：分段展示 self / notes / citations
zot relate SXJ9FYTK --aggregate

# JSON 模式：结构化数据
zot relate SXJ9FYTK --aggregate --json
```

### 聚合输出结构

```json
{
  "ok": true,
  "command": "relate",
  "data": {
    "self": [
      { "predicate": "dc:relation", "direction": "incoming",
        "target": { "key": "RG9BVZDH", "title": "...", ... } }
    ],
    "notes": [
      {
        "source": { "key": "J5NT956U", "preview": "文献总结：羊驼..." },
        "relations": [
          { "predicate": "dc:relation", "direction": "outgoing",
            "target": { "key": "VCHM5KRK", "title": "多倍化...", ... } }
        ]
      }
    ],
    "citations": [
      { "source_key": "VQ58RITC",
        "targets": [
          { "key": "SXJ9FYTK", "title": "Hybrid origin...", ... },
          { "key": "2MZSH8IW", "title": "Documenting HHS...", ... }
        ]
      }
    ]
  }
}
```

### 各字段含义

| 字段 | 类型 | 来源 | 说明 |
|------|------|------|------|
| `self` | `Relation[]` | `itemRelations` 表 | 条目自身的显式关系 |
| `notes` | `NoteRelations[]` | 子笔记 `itemRelations` | 每个子笔记的关系列表 |
| `notes[].source` | `ItemRef` | `items` 表 | 笔记标识和内容预览 |
| `notes[].relations` | `Relation[]` | `itemRelations` 表 | 该笔记指向的其他条目 |
| `citations` | `CitationSource[]` | 笔记 HTML `data-citation-items` | 笔记中内嵌的引用链接 |
| `citations[].targets` | `ItemRef[]` | URI 解析 → `items` 表 | 被引用的目标条目 |

## 可视化：DOT 图输出

`--dot` 输出 Graphviz DOT 格式描述文件，可直接渲染为关系网络图。

```bash
# 基础模式：仅显式关系
zot relate SXJ9FYTK --dot > basic.dot

# 聚合模式：完整三层网络
zot relate SXJ9FYTK --aggregate --dot > full.dot
```

### 渲染为图片

```bash
# 安装 Graphviz 后
dot -Tpng full.dot -o relation-network.png

# 或粘贴到 https://viz-js.com 在线预览
```

### DOT 输出节点颜色约定

| 节点类型 | 形状 | 颜色 | 含义 |
|----------|------|------|------|
| 根条目（查询 key） | box | `#4a90d9` 蓝色 | 网络中心 |
| 普通条目 | box | `#f0f0f0` 灰色 | 关联目标 |
| 笔记 | note | `#e8913a` 橙色 | 子笔记 |
| Citation 目标 | box | `#f0f0f0` 灰色 | 内嵌引用目标 |

### 边样式约定

| 关系类型 | 样式 | 颜色 | 方向 |
|----------|------|------|------|
| 显式关系 (`dc:relation`) | 实线 | `#4a90d9` 蓝 | 双向/单向 |
| 笔记关系 | 实线 | `#e8913a` 橙 | 双向/单向 |
| 父子归属 (`parent`) | 点线 | `#999999` 灰 | → (forward) |
| 内嵌 Citation | 虚线 | `#7bc96f` 绿 | → (forward) |

## 写入操作

### 添加/删除单条关系

```bash
# 添加（需要 ZOT_ALLOW_WRITE=1）
ZOT_ALLOW_WRITE=1 zot relate SXJ9FYTK --add RG9BVZDH

# 删除
ZOT_ALLOW_WRITE=1 zot relate SXJ9FYTK --remove RG9BVZDH

# 预览不执行
zot relate SXJ9FYTK --add RG9BVZDH --dry-run

# 自定义谓词（默认 dc:relation）
zot relate SXJ9FYTK --add RG9BVZDH --predicate "owl:sameAs"
```

### 批量操作

从 JSON 文件读取操作列表：

```bash
# batch-relations.json
[
  {"action": "add",    "source": "SXJ9FYTK", "target": "RG9BVZDH"},
  {"action": "add",    "source": "SXJ9FYTK", "target": "VCHM5KRK"},
  {"action": "remove", "source": "SXJ9FYTK", "target": "2MZSH8IW"}
]

# Dry-run 预览
zot relate --from-file batch-relations.json --dry-run

# 实际执行（需要 ZOT_ALLOW_WRITE=1）
ZOT_ALLOW_WRITE=1 zot relate --from-file batch-relations.json

# JSON 输出
zot relate --from-file batch-relations.json --json
```

### 写入安全约束

- 必须设置 `ZOT_ALLOW_WRITE=1` 环境变量
- `--dry-run` 模式不需要写权限，适合预览
- Local 模式直接写 SQLite；Hybrid 模式优先本地写
- 不支持 Web-only 模式的写入（Web API 的 relations PATCH 权限粒度较粗）

## 过滤

```bash
# 仅查看 dc:relation 类型的关系
zot relate SXJ9FYTK --predicate "dc:relation"

# 聚合模式下同样支持过滤
zot relate SXJ9FYTK --aggregate --predicate "dc:relation"

# DOT 输出也支持过滤
zot relate SXJ9FYTK --dot --predicate "dc:relation"
```

## 模式支持

| 功能 | local | hybrid | web |
|------|-------|--------|-----|
| 查询显式关系 | ✅ | ✅ | ✅ |
| `--aggregate` 聚合 | ✅ | ✅ | ❌ |
| `--dot` 可视化 | ✅ | ✅ | ✅ |
| `--add` / `--remove` 写入 | ✅ | ✅ | ❌ |
| `--from-file` 批量 | ✅ | ✅ | ❌ |
| `--predicate` 过滤 | ✅ | ✅ | ✅ |
| Snapshot staleness 检测 | ✅ | ✅ | N/A |

> **注意**：`--aggregate` 和写入操作依赖 SQLite 本地数据库，因此 web 模式不可用。
