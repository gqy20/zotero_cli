# Knowledge Graph 设计文档

zot AI 笔记系统的核心数据层设计：基于知识图谱的文献智能体记忆系统。

---

## 1. 背景与动机

### 1.1 现状

zot 当前已具备完整的数据访问能力：

| 能力 | 命令 | 数据来源 |
|------|------|---------|
| 文献检索 | `find` / `show` | Zotero API / SQLite |
| PDF 全文读取 | `extract-text` | PyMuPDF / pdfium / ft-cache |
| 标注双源读写 | `annotations` / `annotate` | SQLite + PyMuPDF |
| 元数据管理 | `add-tag` / `create-collection` / `relate` | Zotero API |
| 引文导出 | `cite` / `export` | Zotero API |

但**缺少一个层**来持久化 AI 从这些数据中提取的知识。每次 AI（或用户）需要理解库中的内容时，都要重新：

```
find → show → extract-text → annotations → 让 LLM 理解 → 输出结果 → 丢弃
```

没有积累，没有连接，没有进化。

### 1.2 目标

构建一个**本地优先、可演化、与 Zotero 深度集成**的知识图谱，使 zot 从"AI 友好的数据管道"升级为**具备持久记忆的文献智能体**。

### 1.3 设计原则

1. **图谱即记忆** — 不再单独设计"笔记表"，节点和边就是全部。Atom 是最小知识单元，Edge 是一等公民。
2. **不重复存储元数据** — 论文的标题、作者、DOI 等仍在 Zotero 原始数据库中，图谱只存 AI 增强信息。
3. **Agent 自主管理** — 图谱的写入由 Agent Loop 驱动，不是预定义流水线。Agent 决定何时提取、何时关联、何时综合。
4. **可追溯** — 每条知识都可回溯到原始标注位置或原文段落。
5. **可进化** — 通过 Behavior Rules + Reflect 循环实现决策质量持续改进（详见 [agent-design](./agent-design.md)）。
6. **MCP-first 对外接口** — 图谱能力通过 MCP Server 标准协议暴露，任何 Agent 都能使用。

---

## 2. 数据模型

### 2.1 核心概念

```
┌─────────────────────────────────────────────────────┐
│                   Knowledge Graph                     │
│                                                      │
│    ┌──────┐    supports     ┌──────┐                │
│    │ PAPER │───────────────→│ PAPER │               │
│    │  A   │                │  B   │               │
│    └──┬───┘                └──┬───┘                │
│       │ uses_method           │                      │
│       ▼                       ▼                      │
│    ┌──────┐   discusses   ┌──────────┐             │
│    │CONCEPT│──────────────→│   ATOM   │             │
│    │ CRISPR│              │ claim:.. │             │
│    └──────┘              │ method:..│             │
│                           │ result:.. │             │
│                           └──────────┘             │
│                                                      │
│  Node (节点)          Edge (边)         Atom (原子知识) │
│  - paper              - 关系类型        - 最小知识单元   │
│  - concept            - 有向+有属性      - 属于某个 node │
│  - (future: note)     - 可被确认/否认    - 有角色分类   │
│                        - 有置信度        - 可溯源        │
└─────────────────────────────────────────────────────┘
```

### 2.2 节点（Node）

图谱中有三类节点，共享同一张 `nodes` 表，通过 `type` 字段区分：

#### Paper 节点

代表 Zotero 库中的一篇文献。**不重复存储元数据**，只存 key 和 AI 增强字段。

```sql
CREATE TABLE IF NOT EXISTS nodes (
    key         TEXT PRIMARY KEY,       -- Zotero item key（唯一标识）
    type        TEXT NOT NULL DEFAULT 'paper',
                                        -- 'paper' | 'concept'
    title       TEXT,                    -- 冗余缓存（从 Zotero 读取），加速查询
    embedding   BLOB,                    -- 预留：768-dim 向量 (FLOAT32 数组)
                                        -- 后期语义搜索用，P0 不实现
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
```

Paper 节点的 AI 增强信息存在关联表中：

```sql
CREATE TABLE IF NOT EXISTS node_paper_attrs (
    key             TEXT PRIMARY KEY,
    summary_ai      TEXT DEFAULT '',        -- AI 生成的摘要（区别于 Zotero 原始 abstract）
                                        -- abstract 是作者写的，summary_ai 是 AI 理解后重写的
    key_claims      TEXT DEFAULT '[]',      -- JSON array: 核心论点（每条 1-2 句话）
    methods         TEXT DEFAULT '[]',      -- JSON array: 方法要点
    limitations     TEXT DEFAULT '[]',      -- JSON array: 局限性
    open_questions  TEXT DEFAULT '[]',      -- JSON array: 未解决的问题
    status          TEXT NOT NULL DEFAULT 'new',
                                        -- 'new'        : 尚未处理
                                        -- 'extracted'  : 已提取 atoms
                                        -- 'annotated'  : 已有 AI 标注
                                        -- 'synthesized': 已参与过综合
                                        -- 'archived'   : 已归档（不再活跃演化）
    last_ai_run_at  TEXT,
    ai_version     INTEGER DEFAULT 0,      -- 处理代数（每次重新提取 +1）
    FOREIGN KEY (key) REFERENCES nodes(key) ON DELETE CASCADE
);
```

#### Concept 节点

代表从论文中自动抽取或用户定义的概念实体。Concept 是跨论文的：

```sql
-- Concept 节点也用 nodes 表，type = 'concept'
-- 额外属性存在 node_concept_attrs 中：

CREATE TABLE IF NOT EXISTS node_concept_attrs (
    key             TEXT PRIMARY KEY,       -- concept 的 key（自动生成，格式: "C_{uuid_short}"）
    aliases         TEXT DEFAULT '[]',      -- JSON array: 别名（如 "gene drive" = ["基因驱动", "驱动基因"]）
    definition      TEXT DEFAULT '',        -- AI 生成的概念定义
    domain          TEXT DEFAULT '',        -- 所属领域（如 "分子生物学"、"计算机科学"）
    first_seen_in   TEXT NOT NULL,          -- 首次出现在哪篇论文的 key
    paper_count     INTEGER DEFAULT 0,      -- 涉及多少篇论文
    note_count      INTEGER DEFAULT 0,      -- 多少条 atom 提及此概念
    is_user_created INTEGER DEFAULT 0,     -- 0=AI 自动创建 1=用户手动创建
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    FOREIGN KEY (key) REFERENCES nodes(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_concept_attrs_domain ON node_concept_attrs(domain);
```

**Concept 不是预先定义的分类体系**，而是从论文内容中自然涌现的。例如：

- 处理 5 篇 CRISPR 论文后，Agent 可能自动创建 concept "CRISPR-Cas9"
- 处理 3 篇关于杂交物种形成的论文后，可能创建 concept "homoploid hybrid speciation"
- 用户也可以手动创建 concept 来组织自己的研究方向

### 2.3 边（Edge）— 图谱的一等公民

这是和传统"笔记 + 引用"设计的**最大区别**。在知识图谱中，关系不是附属品，而是核心数据。

```sql
CREATE TABLE IF NOT EXISTS edges (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    from_key    TEXT NOT NULL,            -- 起点 node key
    to_key      TEXT NOT NULL,            -- 终点 node key
    relation    TEXT NOT NULL,            -- 关系类型（开放字符串，非固定 enum）
    evidence    TEXT DEFAULT '',           -- 为什么存在这个关系（一句话或引用段落）
    confidence  REAL DEFAULT 0.5,          -- 置信度 0-1
                                        -- AI 初始给 0.5，随反馈调整：
                                        --   用户确认 → +0.1~0.3
                                        --   用户否认 → -0.1~-0.3
                                        --   多次确认 → 逐步趋近 1.0
    source      TEXT DEFAULT '',           -- 来源
                                        -- 'auto_extract'  : 从单篇论文提取时自动发现
                                        -- 'user_created'   : 用户手动建立
                                        -- 'ai_synthesis'   : Agent 综合多篇时推断
                                        -- 'reflect'        : Reflect 过程中发现
    atom_ids    TEXT DEFAULT '[]',         -- JSON: 支持此关系的 atom ID 列表
                                        -- （哪些具体知识点支撑这个关系）
    created_at  TEXT NOT NULL,
    confirmed   INTEGER DEFAULT 0,         -- 三态确认标志
                                        --  0 = 待确认（默认）
                                        --  1 = 用户确认
                                        -- -1 = 用户否认
    confirmed_by TEXT DEFAULT '',          -- 谁/什么操作确认的
    confirmed_at TEXT DEFAULT '',          -- 何时确认的

    UNIQUE(from_key, to_key, relation),
    FOREIGN KEY (from_key) REFERENCES nodes(key) ON DELETE CASCADE,
    FOREIGN KEY (to_key) REFERENCES nodes(key) ON DELETE CASCADE
);

-- 查询优化索引
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_key);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_key);
CREATE INDEX IF NOT EXISTS edges_relation ON edges(relation);
CREATE INDEX IF NOT EXISTS edges_confirmed ON edges(confirmed);
CREATE INDEX IF NOT EXISTS edges_source ON edges(source);
```

#### 预定义关系类型

Agent 可以自由创造新的 relation 类型，但以下作为初始种子：

| 关系类型 | 含义 | 典型场景 |
|----------|------|---------|
| `supports` | 支持 | 论文 A 的方法支持论文 B 的结论 |
| `contradicts` | 反驳 | 两篇论文结论矛盾 |
| `extends` | 扩展 | A 在 B 的基础上做了扩展 |
| `uses_method` | 使用相同方法 | 两篇用了类似的技术路线 |
| `compares` | 对比 | 显式对比研究 |
| `discusses` | 讨论 | 论文讨论了某概念 |
| `cites` | 引用 | 基于 DOI 或标题匹配的引用关系 |
| `follows_up` | 后续跟进 | A 是 B 的后续工作 |
| `based_on` | 基于 | A 的方法基于 B |
| `shares_data` | 共享数据集 | 用了相同的实验数据 |
| `related` | 相关 | 通用弱关系 |
| `opposes` | 对立 | 学术观点对立 |

**重要**：relation 字段是**开放字符串**，不限制为上述列表。Agent 在运行中可以根据需要创建新的关系类型，如 `shares_author_with`、`same_experimental_setup` 等。这比固定的 enum 更灵活，也更符合真实学术关系的多样性。

### 2.4 原子知识单元（Atom）

从论文全文或标注中提取的最小知识点。这是系统的**基本构成单位**。

```sql
CREATE TABLE IF NOT EXISTS atoms (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_key    TEXT NOT NULL,             -- 属于哪个 paper 节点
    content     TEXT NOT NULL,             -- Markdown 正文（知识点的完整表述）
    role        TEXT NOT NULL,             -- 语义角色
    page        INTEGER DEFAULT 0,         -- 所在页码（0 表示无法确定页码）
    annot_key   TEXT DEFAULT '',            -- 来源标注 key（空 = 来自全文提取）
    source_text TEXT DEFAULT '',            -- 原文片段（用于溯源，展示"这句话出自哪里"）
    source_type TEXT DEFAULT '',            -- 'annotation' | 'fulltext' | 'user_manual'
    confidence  REAL DEFAULT 0.5,           -- AI 对此知识点的置信度
    embedding   BLOB,                      -- 预留：向量（后期语义搜索）
    concept_keys TEXT DEFAULT '[]',        -- JSON: 涉及的 concept key 列表
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,

    FOREIGN KEY (node_key) REFERENCES nodes(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_atoms_node ON atoms(node_key);
CREATE INDEX IF NOT EXISTS idx_atoms_role ON atoms(role);
CREATE INDEX IF NOT EXISTS idx_atoms_annot ON atoms(annot_key);
CREATE INDEX IF NOT EXISTS idx_atoms_concept ON atoms(concept_keys);
```

#### Atom 角色（Role）

每个 Atom 有一个明确的语义角色，决定它在知识网络中的用途：

| Role | 说明 | 示例 |
|------|------|------|
| `claim` | 核心论点/主张 | "We demonstrate that CRISPR-Cas9 can achieve >90% knockout efficiency in Anopheles gambiae" |
| `method` | 方法论描述 | "Used a self-limiting gene drive system with dCas9-based anti-drive" |
| `result` | 实验结果/发现 | "In cage trials, population suppression reached 85% by generation 8" |
| `limitation` | 局限性 | "Resistance alleles emerged in ~3% of offspring; long-term efficacy unknown" |
| `comparison` | 与其他工作的对比 | "Unlike Whalen et al. 2020, our approach does not require fitness cost" |
| `question` | 未解决的问题/开放问题 | "It remains unclear whether resistance can be fully suppressed in wild populations" |
| `insight` | 个人洞察/联想 | "This method's split-drive architecture could apply to our malaria vector control project" |
| `citation` | 可引用的原文段落 | 直接摘录的原文（带精确出处） |
| `definition` | 概念定义 | "Gene drive: a genetic system that biases inheritance to spread a trait through a population" |
| `data_point` | 数据要点 | "Sample size: n=500 mosquitoes across 5 replicate cages" |

**为什么需要 Role？**

不同角色的 Atom 在下游使用方式不同：

- `claim` + `result` → 自动汇入论文摘要
- `limitation` + `question` → 自动生成"未来工作方向"笔记
- `insight` → 个人知识积累，跨论文关联的核心素材
- `citation` → 直接用于论文写作的引证
- `data_point` → 自动汇总成对比表格

#### Atom 的溯源设计

每个 Atom 都可以追溯到它的来源：

```go
type AtomSource struct {
    // 来源类型
    Type string // "annotation" | "fulltext" | "user_manual"

    // 如果来自标注
    AnnotKey string   // Zotero 标注 key
    Page      int      // 标注所在页码
    Text      string   // 原始高亮文本

    // 如果来自全文
    FullTextRange string // "page 3, paragraph 2" 之类的定位描述

    // 如果是用户手动创建
    ManualNote string // 用户附注
}
```

这意味着用户看到一条 Atom 时，可以：

```
Atom: "CRISPR-Cas9 achieved >90% knockout efficiency"
  ↓ 溯源于
Annotation #ANNOT_003 on Paper KEY_ABC, page 4:
  "We demonstrate that CRISPR-Cas9 can achieve >90% knockout..."
  ↓ 一键跳转
zot open ABC --page 4    ← 在 Zotero 阅读器中打开并跳转
```

### 2.5 完整 ER 图

```
nodes (key PK, type, title, embedding)
  │
  ├── node_paper_attrs (key PK/FK → nodes)
  │     summary_ai, key_claims[], methods[], limitations[],
  │     open_questions[], status, last_ai_run_at, ai_version
  │
  └── node_concept_attrs (key PK/FK → nodes)
        aliases[], definition, domain, first_seen_in,
        paper_count, note_count, is_user_created

edges (id PK, from_key FK→nodes, to_key FK→nodes,
       relation, evidence, confidence, source,
       atom_ids[], confirmed, confirmed_by, confirmed_at)

atoms (id PK, node_key FK→nodes,
       content, role, page, annot_key, source_text,
       source_type, confidence, embedding,
       concept_keys[])
```

---

## 3. 存储方案

### 3.1 文件位置

遵循现有 `.zotero_cli/` 目录约定：

```
{ZOT_DATA_DIR}/.zotero_cli/
├── fulltext/                  # 已有：PDF 全文缓存
│   ├── {attachmentKey}/
│   │   ├── content.txt
│   │   └── meta.json
│   └── index.sqlite           # FTS5 全文索引
│
└── ai/                        # 新增：AI 知识图谱
    ├── graph.sqlite           # Knowledge Graph 主数据库
    │   ├── nodes 表
    │   ├── node_paper_attrs 表
    │   ├── node_concept_attrs 表
    │   ├── edges 表
    │   └── atoms 表
    │
    ├── working.json            # Working Memory（会话级，见 agent-design.md）
    │
    ├── events.sqlite           # Interaction Log（不可变审计日志）
    │
    └── embeddings/             # 后期：向量文件（可选）
        └── {node_key}.bin     # 每个 node 一个向量文件
```

### 3.2 graph.sqlite 初始化

```go
// internal/ai/graph_store.go

const currentSchemaVersion = 1

func InitDB(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
    if err != nil {
        return nil, err
    }

    // 启用 WAL 模式（允许并发读）
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec("PRAGMA synchronous=NORMAL")

    if err := ensureSchema(db); err != nil {
        db.Close()
        return nil, err
    }
    return db, nil
}

func ensureSchema(db *sql.DB) error {
    version := 0
    db.QueryRow("SELECT value FROM schema_version LIMIT 1").Scan(&version)

    if version < 1 {
        // 创建所有表（见上方 SQL DDL）
        tx, _ := db.Begin()
        // ... CREATE TABLE statements ...
        tx.Exec(`INSERT INTO schema_version (value) VALUES (1)`)
        tx.Commit()
    }
    // 未来版本迁移: else if version < 2 { ... }
    return nil
}
```

### 3.3 与现有 fulltext 缓存的关系

```
fulltext/                    ai/
├── content.txt  (原始全文)    ├── atoms (从全文/标注中提炼的结构化知识)
├── meta.json   (元数据)       ├── edges (知识间的关系)
└── index.sqlite (FTS5 索引)  └── graph.sqlite (知识图谱)
                                ↑
                    atoms.content 是对 fulltext content 的
                    高层次结构化提炼，不是简单复制
```

- `fulltext/content.txt` 存的是原始全文（用于 FTS5 关键词搜索）
- `atoms.content` 存的是 AI 提炼后的知识点（用于推理和问答）
- 两者共存，各司其职

---

## 4. CRUD 接口设计

### 4.1 GraphStore 接口

```go
// internal/ai/graph_store.go

package ai

import (
    "context"
    "database/sql"
    "time"
)

// GraphStore 是知识图谱的全部数据访问抽象
// 实现细节隐藏在内部，外部只通过此接口操作
type GraphStore interface {
    // ====== Node 操作 ======

    // EnsurePaperNode 确保 paper 节点存在（不存在则创建）
    EnsurePaperNode(ctx context.Context, key string, title string) (*PaperNode, error)

    // GetPaperNode 获取论文节点及其增强属性
    GetPaperNode(ctx context.Context, key string) (*PaperNode, error)

    // UpdatePaperAttrs 更新论文的 AI 增强属性
    UpdatePaperAttrs(ctx context.Context, attrs *PaperNodeAttrs) error

    // ListPapersByStatus 按状态列出论文节点
    ListPapersByStatus(ctx context.Context, status string, limit, offset int) ([]*PaperNode, error)

    // UpsertConcept 创建或更新概念节点
    UpsertConcept(ctx context.Context, c *ConceptNode) error

    // GetConcept 获取概念详情
    GetConcept(ctx context.Context, key string) (*ConceptNode, error)

    // SearchConcepts 模糊搜索概念（名称/别名匹配）
    SearchConcepts(ctx context.Context, query string, limit int) ([]*ConceptNode, error)

    // ListConceptsByDomain 按领域列出概念
    ListConceptsByDomain(ctx context.Context, domain string) ([]*ConceptNode, error)

    // ====== Edge 操作 ======

    // AddEdge 创建新的关系边（如果已存在则更新 evidence/confidence）
    AddEdge(ctx context.Context, e *Edge) (*Edge, error)

    // GetEdgesFrom 获取从某节点出发的所有边
    GetEdgesFrom(ctx context.Context, key string, opts ...EdgeQueryOpt) ([]*Edge, error)

    // GetEdgesBetween 获取两节点间的所有关系
    GetEdgesBetween(ctx context.Context, fromKey, toKey string) ([]*Edge, error)

    // ConfirmEdge 确认/否认一条边
    ConfirmEdge(ctx context.Context, edgeID int, confirmed int, reason string) error

    // UpdateEdgeConfidence 调整边的置信度
    UpdateEdgeConfidence(ctx context.Context, edgeID int, delta float64) error

    // ====== Atom 操作 ======

    // CreateAtom 创建原子知识单元
    CreateAtom(ctx context.Context, a *Atom) (*Atom, error)

    // CreateAtoms 批量创建
    CreateAtoms(ctx context.Context, atoms []*Atom) ([]*Atom, error)

    // GetAtomsByNode 获取某篇论文的所有 atoms
    GetAtomsByNode(ctx context.Context, nodeKey string, opts ...AtomQueryOpt) ([]*Atom, error)

    // GetAtomByID 获取单条 atom
    GetAtomByID(ctx context.Context, id int) (*Atom, error)

    // UpdateAtom 更新 atom 内容
    UpdateAtom(ctx context.Context, a *Atom) error

    // DeleteAtom 删除 atom（软删除或硬删除）
    DeleteAtom(ctx context.Context, id int) error

    // SearchAtoms 在所有 atoms 中全文搜索
    SearchAtoms(ctx context.Context, query string, limit int) ([]*Atom, error)

    // GetAtomsByRole 按角色筛选 atoms
    GetAtomsByRole(ctx context.Context, role string, limit int) ([]*Atom, error)

    // ====== 图谱查询（高级）=====

    // Subgraph 获取以某节点为中心的子图（N 跳邻居）
    Subgraph(ctx context.Context, centerKey string, depth int, edgeFilter []string) (*SubgraphResult, error)

    // FindPath 寻找两节点之间的最短路径
    FindPath(ctx context.Context, fromKey, toKey string) ([]*Edge, error)

    // FindConceptChain 发现概念链：A → concept X → concept Y → B
    FindConceptChain(ctx context.Context, fromKey, toKey string, maxDepth int) (*ChainResult, error)

    // GetUnconnectedNodes 发现孤立节点（没有边的节点）
    GetUnconnectedNodes(ctx context.Context) ([]string, error)

    // SuggestLinks 基于内容相似建议新边（不直接写入，返回建议列表）
    SuggestLinks(ctx context.Context, nodeKey string, limit int) ([]*LinkSuggestion, error)

    // ====== 统计与分析 ======

    // Stats 获取图谱整体统计
    Stats(ctx context.Context) (*GraphStats, error)

    // ActivityTimeline 获取近期活动时间线
    ActivityTimeline(ctx context.Context, since time.Time) ([]*TimelineEvent, error)

    // ExportSubgraph 导出子图为可视化格式（DOT/JSON/GEXF）
    ExportSubgraph(ctx context.Context, centerKey string, depth int, format string) ([]byte, error)
}
```

### 4.2 核心数据结构（Go 类型）

```go
package ai

import "time"

// ---- Node ----

type NodeType string

const (
    NodeTypePaper    NodeType = "paper"
    NodeTypeConcept NodeType = "concept"
)

type Node struct {
    Key        string    `json:"key"`
    Type       NodeType `json:"type"`
    Title      string    `json:"title,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

type PaperNode struct {
    Node
    Attrs *PaperNodeAttrs `json:"attrs,omitempty"`
}

type PaperNodeAttrs struct {
    SummaryAI     string   `json:"summary_ai,omitempty"`
    KeyClaims     []string `json:"key_claims,omitempty"`
    Methods       []string `json:"methods,omitempty"`
    Limitations   []string `json:"limitations,omitempty"`
    OpenQuestions []string `json:"open_questions,omitempty"`
    Status        string   `json:"status"` // new|extracted|annotated|synthesized|archived
    LastAIRunAt  *time.Time `json:"last_ai_run_at,omitempty"`
    AIVersion    int      `json:"ai_version"`
}

type ConceptNode struct {
    Node
    Attrs *ConceptNodeAttrs `json:"attrs,omitempty"`
}

type ConceptNodeAttrs struct {
    Aliases       []string `json:"aliases,omitempty"`
    Definition    string   `json:"definition,omitempty"`
    Domain        string   `json:"domain,omitempty"`
    FirstSeenIn   string   `json:"first_seen_in"` // paper key
    PaperCount    int      `json:"paper_count"`
    NoteCount     int      `json:"note_count"`
    IsUserCreated bool     `json:"is_user_created"`
}

// ---- Edge ----

type Edge struct {
    ID          int       `json:"id"`
    FromKey     string    `json:"from_key"`
    ToKey       string    `json:"to_key"`
    Relation    string    `json:"relation"`
    Evidence    string    `json:"evidence,omitempty"`
    Confidence  float64   `json:"confidence"`
    Source      string    `json:"source"` // auto_extract|user_created|ai_synthesis|reflect
    AtomIDs     []int     `json:"atom_ids,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    Confirmed   int       `json:"confirmed"`  // 0=pending 1=confirmed -1=rejected
    ConfirmedBy string    `json:"confirmed_by,omitempty"`
    ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

type EdgeQueryOpt struct {
    Relation  string   // 过滤关系类型
    Confirmed *int     // 过滤确认状态
    MinConf  float64  // 最小置信度
    Source   string   // 过滤来源
}

// ---- Atom ----

type AtomRole string

const (
    RoleClaim      AtomRole = "claim"
    RoleMethod     AtomRole = "method"
    RoleResult     AtomRole = "result"
    RoleLimitation AtomRole = "limitation"
    RoleComparison AtomRole = "comparison"
    RoleQuestion   AtomRole = "question"
    RoleInsight    AtomRole = "insight"
    RoleCitation   AtomRole = "citation"
    RoleDefinition  AtomRole = "definition"
    RoleDataPoint  AtomRole = "data_point"
)

type Atom struct {
    ID          int       `json:"id"`
    NodeKey     string    `json:"node_key"`
    Content     string    `json:"content"`
    Role        AtomRole  `json:"role"`
    Page        int       `json:"page"`
    AnnotKey    string    `json:"annot_key,omitempty"`
    SourceText  string    `json:"source_text,omitempty"`
    SourceType  string    `json:"source_type"` // annotation|fulltext|user_manual
    Confidence  float64   `json:"confidence"`
    ConceptKeys []string  `json:"concept_keys,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type AtomQueryOpt struct {
    Roles      []AtomRole // 按角色过滤
    HasAnnot   bool        // 是否有关联标注
    MinConf     float64     // 最小置信度
    Since      *time.Time  // 创建时间下限
}

// ---- 查询结果 ----

type SubgraphResult struct {
    Center  *PaperNode   `json:"center"`
    Nodes   []*Node      `json:"nodes"`    // 包含 depth 跳内的所有节点
    Edges   []*Edge      `json:"edges"`    // 这些节点之间的边
    Atoms   []*Atom      `json:"atoms"`    // 这些节点的 atoms（可选，按需加载）
}

type ChainResult struct {
    FromKey  string   `json:"from_key"`
    ToKey    string   `json:"to_key"`
    Path     []*Edge `json:"path"`      // 经过的边序列
    Concepts []string `json:"concepts"` // 经过的概念
    Length   int      `json:"length"`
}

type LinkSuggestion struct {
    FromKey   string  `json:"from_key"`
    ToKey     string  `json:"to_key"`
    ToTitle   string  `json:"to_title,omitempty"`
    Relation  string  `json:"suggested_relation"`
    Reason    string  `json:"reason"`          // 为什么建议
    Confidence float64 `json:"confidence"`    // 建议置信度
    SharedAtoms []int   `json:"shared_atoms"`  // 共同涉及的 atom IDs
}

type GraphStats struct {
    TotalNodes      int `json:"total_nodes"`
    TotalPapers     int `json:"total_papers"`
    TotalConcepts   int `json:"total_concepts"`
    TotalEdges      int `json:"total_edges"`
    TotalAtoms      int `json:"total_atoms"`
    ConfirmedEdges  int `json:"confirmed_edges"`
    RejectedEdges   int `json:"rejected_edges"`
    PendingEdges    int `json:"pending_edges"`
    AvgConfidence  float64 `json:"avg_confidence"`
    TopRelations   []RelationCount `json:"top_relations"`
    StatusBreakdown map[string]int `json:"status_breakdown"` // 各状态的论文数量
    DomainBreakdown  map[string]int `json:"domain_breakdown"`  // 各领域的概念数量
}

type RelationCount struct {
    Relation string `json:"relation"`
    Count    int    `json:"count"`
}

type TimelineEvent struct {
    Time      time.Time `json:"time"`
    Type      string    `json:"type"`      // node_created|edge_added|atom_created|confirmed|rejected
    TargetKey string    `json:"target_key"`
    Detail    string    `json:"detail"`
}
```

---

## 5. 与现有系统的集成点

### 5.1 数据流向

```
Zotero 数据源                  Knowledge Graph
=================          =========================

zotero.sqlite                 graph.sqlite
├── items 表                   ├── nodes (paper)
│   ├── title      ──────→     │   └── title (冗余缓存)
│   ├── abstract   ──────→     │       (不复制，AI 重写后存 summary_ai)
│   ├── doi                     │
│   └── tags                    │
├── itemAnnotations 表         ├── atoms
│   ├── text       ──────→     │   ├── content (AI 提炼)
│   ├── comment    ──────→     │   ├── annot_key (回溯)
│   ├── position               │   └── source_text (原文片段)
│   └── dateAdded              │
├── itemRelations 表           ├── edges
│   ──────────────→            │   ├── (显式关系直接导入)
│                              │
storage/{key}/.pdf            ├── atoms
├── extract-text 输出  ──────→ │   ├── content (从全文提炼)
│                              │   ├── source_type = "fulltext"
│                              │   └── page (来自 chunk 信息)
                              │
用户手动输入                   ├── atoms / edges / concepts
├── zot ai-notes create  ───→ │   ├── source_type = "user_manual"
├── zot link --manual  ────→ │   └── source = "user_created"
```

### 5.2 不做的事

| 不做 | 原因 |
|------|------|
| 不修改 zotero.sqlite | 避免污染原始数据，独立存储 |
| 不替代现有 Reader/Writer 接口 | 图谱是上层建筑，调用现有接口获取数据 |
| 不在 P0 实现 embedding | 先用 FTS + 结构化查询跑通，向量检索是 P5 |
| 不做实时同步 | 图谱是异步构建的，批量处理 + 增量更新 |
| 不引入 LLM 依赖到存储层 | GraphStore 是纯数据操作，LLM 在上层 |

### 5.3 配置扩展

在现有 `.env` 配置中新增 AI 相关字段：

```bash
# === Existing config ===
ZOT_MODE=hybrid
ZOT_DATA_DIR=D:\Zotero\zotero
ZOT_LIBRARY_ID=12345
ZOT_API_KEY=abc123
# ...

# === New: AI / Knowledge Graph ===
# AI 数据目录（默认 {DataDir}/.zotero_cli/ai/）
ZOT_AI_DIR=

# LLM 配置（P1+ 使用，P0 不需要）
ZOT_LLM_PROVIDER=          # openai | ollama | anthropic
ZOT_LLM_BASE_URL=          # API base URL
ZOT_LLM_MODEL=             # 模型名称
ZOT_LLM_API_KEY=            # API key

# AI 行为规则（有合理默认值，用户可覆盖）
ZOT_AI_EXTRACT_PREFER_ANNOTATIONS=true
ZOT_AI_AUTO_LINK_THRESHOLD=0.7
ZOT_AI_ANNOT_REQUIRE_CONFIRM=true
ZOT_AI_DEFAULT_ANNOT_COLOR=orange
```

---

## 6. MCP Resource 映射

Knowledge Graph 作为 MCP Server 暴露时的 Resource 设计：

| Resource URI | 返回内容 | 对应的 GraphStore 方法 |
|-------------|---------|------------------------|
| `zotero://graph/overview` | 图谱统计概览 | `Stats()` |
| `zotero://graph/papers/{key}` | 单篇论文的完整图谱上下文（节点 + 出入边 + atoms） | `GetPaperNode()` + `GetEdgesFrom()` + `GetAtomsByNode()` |
| `zotero://graph/concepts/{key}` | 概念详情 + 涉及的论文和 atoms | `GetConcept()` |
| `zotero://graph/subgraph?center={key}&depth={N}` | 子图（N 跳邻居） | `Subgraph()` |
| `zotero://graph/path?from={A}&to={B}` | 两节点间最短路径 | `FindPath()` |
| `zotero://graph/suggestions?node={key}` | 关系建议（不写入） | `SuggestLinks()` |
| `zotero://graph/timeline?since={ISO8601}` | 近期活动时间线 | `ActivityTimeline()` |
| `zotero://graph/unconnected` | 孤立节点列表 | `GetUnconnectedNodes()` |
| `zotero://graph/export?center={key}&depth={N}&format=dot` | 可视化导出 | `ExportSubgraph()` |

---

## 7. 查询示例

### 7.1 场景：用户问"我之前读过的 CRISPR 相关论文之间有什么关系？"

```go
// 1. 找到所有涉及 "CRISPR" 概念的论文节点
papers, _ := store.SearchConcepts(ctx, "CRISPR", 20)

// 2. 构建这些论文之间的子图（depth=2）
var centerKey string
if len(papers) > 0 {
    centerKey = papers[0].Key
}
subgraph, _ := store.Subgraph(ctx, centerKey, 2, nil)

// 3. 结果包含：
// - 所有相关论文节点
// - 它们之间的关系边（supports / contradicts / compares / ...）
// - 每篇论文的关键 atoms
// 可直接用于生成回答或可视化
```

### 7.2 场景：发现隐式关联

```go
// Agent 定期运行：
suggestions, _ := store.SuggestLinks(ctx, "PAPER_XYZ", 10)

// 返回：
// [
//   {ToKey: "PAPER_ABC", Relation: "uses_similar_method",
//    Reason: "Both use dCas9-based anti-drive with split-site targeting",
//    Confidence: 0.82, SharedAtoms: [atom_42, atom_103]},
//   {ToKey: "PAPER_DEF", Relation: "contradicts",
//    Reason: "ABC claims 90% efficiency while DEF reports only 45% in similar setup",
//    Confidence: 0.76, SharedAtoms: [atom_15]},
// ]
//
// Agent 可以选择：
// a) 自动创建边（高置信度的）
// b) 展示给用户确认（中等置信度的）
// c) 忽略（低置信度的）
```

### 7.3 场景：从标注自动构建 Atom

```go
// 用户在论文 K 上做了 5 个标注
annots, _ := reader.Annotations(ctx, "K")

// Agent 将标注聚类并提炼为 atoms
atoms := []*Atom{
    {
        NodeKey:    "K",
        Content:    "本文提出了一种基于 dCas9 的分割位点基因驱动系统...",
        Role:       RoleClaim,
        Page:       3,
        AnnotKey:   "ANNOT_001",
        SourceText: "We propose a novel split-site gene drive...",
        SourceType: "annotation",
        Confidence: 0.8,
    },
    {
        NodeKey:    "K",
        Content:    "在笼式实验中，按代次种群抑制率达到 85%",
        Role:       RoleResult,
        Page:       6,
        AnnotKey:   "ANNOT_003",
        SourceText: "Population suppression reached 85% by generation 8",
        SourceType: "annotation",
        Confidence: 0.9,
    },
    // ...
}
created, _ := store.CreateAtoms(ctx, atoms)
// atoms 现在永久存储在知识图谱中，可被检索、关联、综合
```

---

## 8. 与 Agent Runtime 的关系

Knowledge Graph 是 Agent Runtime 的**持久化层**（Archival Memory）。完整的 Agent 架构参见 [agent-design](./agent-design.md)，此处仅说明交互边界：

```
┌─────────────────────────────────────────┐
│           Agent Main Loop               │
│                                          │
│  Working Memory (会话级, 易失)           │
│  ├─ FocusItems: 当前关注的论文           │
│  ├─ ActiveConcepts: 活跃概念             │
│  └─ PendingTasks: 待办队列               │
│                                          │
│  ┌───────────────────────────────────┐   │
│  │  LLM 决策                          │   │
│  │                                     │   │
│  │  "我需要了解论文 X 的核心观点"     │   │
│  │       ↓                            │   │
│  │  ToolCall: memory_search          │   │
│  │       ↓                            │   │
│  └───┼───────────────────────────────┘   │
│       │                                 │
├───────┼─────────────────────────────────┤
│       ▼                                 │
│  ┌───────────────────────────────────┐   │
│  │     Knowledge Graph Store          │   │
│  │     (Archival Memory, 持久化)      │   │
│  │                                   │   │
│  │  GetPaperNode("X")                │   │
│  │  GetAtomsByNode("X")              │   │
│  │  GetEdgesFrom("X")                │   │
│  │  → 返回结构化知识                  │   │
│  └───────────────────────────────────┘   │
└──────────────────────────────────────────┘
```

Agent 通过 Tool Call 访问 GraphStore，GraphStore 本身不包含任何 LLM 逻辑，是纯数据操作层。

---

## 9. 实施计划

| 阶段 | 内容 | 涉及文件 | 依赖 |
|------|------|---------|------|
| **P0** | Schema 建表 + GraphStore 接口 + 基础 CRUD | `internal/ai/graph_store.go`, `internal/ai/types.go`, `internal/ai/schema.sql` | 仅 SQLite |
| **P1** | `ai-notes` CLI 命令（list/show/stats/generate） | `internal/cli/commands_ai_notes.go` | P0 |
| **P2** | MCP Server 骨架（Resources + 2-3 个 Tools） | `internal/ai/mcp_server.go` | P0 |
| **P3** | 标注→Atom 提取管线（含 LLM 调用） | `internal/ai/pipeline_extract.go`, `internal/ai/llm/` | P0 + P1 + LLM provider |
| **P4** | 自动关联发现（SuggestLinks → AutoLink） | `internal/ai/pipeline_link.go` | P0 + P3 |
| **P5** | 语义搜索（Embedding + 向量索引） | `internal/ai/vector_store.go` | P0 + embedding model |
| **P6** | Reflect + Behavior Rules 进化机制 | `internal/ai/reflect.go`, `internal/ai/rules.go` | P0 + P3 + events log |

**P0 交付标准**：

- [x] `graph.sqlite` 可自动创建，schema 版本管理就绪
- [x] `GraphStore` 接口全部方法的签名定义完成
- [x] `EnsurePaperNode` / `CreateAtom` / `AddEdge` 可正确写入和读取
- [x] `Subgraph` / `Stats` / `SearchAtoms` 查询可用
- [ ] 基础测试覆盖（CRUD + 查询）
- [ ] `zot ai-graph init` 命令可初始化空的图谱数据库
- [ ] `zot ai-graph stats --json` 可查看空图谱状态
