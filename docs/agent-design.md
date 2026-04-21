# Agent Runtime 设计文档

zot AI 文献智能体的运行时架构：Agent Loop、记忆分层、MCP Server、自进化机制。

---

## 1. 架构总览

```
┌─────────────────────────────────────────────────────┐
│                  MCP Server Layer                  │
│  Resources: 库状态 / 论文详情 / 知识图谱 / 工作记忆    │
│  Tools: 检索 / 提取 / 标注 / 关联 / 综合 / 反思      │
│  Prompts: 文献调研 / 综述 / 审稿模板                │
├─────────────────────────────────────────────────────┤
│                 Agent Runtime Layer                 │
│                                                      │
│  ┌──────────────────────────────────────────┐       │
│  │         Working Memory (Core)            │       │
│  │  当前焦点文献、活跃概念、待办队列、决策上下文   │       │
│  └──────────────────┬───────────────────────┘       │
│                     │                                │
│  ┌──────────────────▼───────────────────────┐       │
│  │     Knowledge Graph (Archival Memory)     │       │
│  │  nodes / edges / atoms / concepts           │       │
│  │  (详见 knowledge-graph.md)                   │       │
│  └──────────────────┬───────────────────────┘       │
│                     │                                │
│  ┌──────────────────▼───────────────────────┐       │
│  │       Interaction Log (Recall)             │       │
│  │  不可变操作历史，用于反思和审计              │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  ┌──────────────────────────────────────────┐       │
│  │          Agent Main Loop                   │       │
│  │  PERCEIVE → THINK → ACT → CHECK (循环)     │       │
│  └──────────────────┬───────────────────────┘       │
│                     │                                │
│  ┌──────────────────▼───────────────────────┐       │
│  │          LLM Abstraction                  │       │
│  │  OpenAI / Ollama / Anthropic / Mock        │       │
│  └──────────────────────────────────────────┘       │
├─────────────────────────────────────────────────────┤
│               Infrastructure                      │
│  {DataDir}/.zotero_cli/ai/                       │
│  ├── graph.sqlite      # Knowledge Graph          │
│  ├── working.json       # Working Memory          │
│  ├── events.sqlite     # Interaction Log          │
│  └── rules.json         # Behavior Rules          │
├─────────────────────────────────────────────────────┤
│             Existing zot capabilities               │
│  Reader / Writer / Annotations / Extract-text      │
└─────────────────────────────────────────────────────┘
```

---

## 2. Agent Loop（核心执行引擎）

### 2.1 为什么是 Loop 而不是 Pipeline

| Pipeline（流水线） | Agent Loop（循环） |
|---------------------|---------------|
| 步骤固定，顺序执行 | 每步由 LLM 动态决定 |
| 输入→输出，单次通过 | 可多轮迭代，中间结果影响后续决策 |
| 难以处理异常分支 | 天然支持条件分支、重试、回退 |
| 不具备"自主性" | Agent 自主决定何时停止 |

参考：LangChain Agent Loop、Eino ReAct、OpenAI Agents SDK Handoffs、Letta/MemGPT Core Memory Loop。

### 2.2 Loop 状态机

```
                    ┌──────────────┐
                    │   IDLE        │  ← 等待用户输入或定时触发
                    └──────┬───────┘
                           │ 触发
                           ▼
                    ┌──────────────┐
                    │  PERCEIVE     │  ← 感知阶段
                    │              │
                    │ 读 Working    │
                    │ 读 Graph     │
                    │ 读用户输入    │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   THINK       │  ← 推理阶段（LLM 调用）
                    │              │
                    │ 分析当前状态  │
                    │ 选择下一步动作 │
                    │ 输出决策      │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │   ACT    │  │ FINAL    │  │  ERROR   │
        │ 执行工具  │  │ 返回答案  │  │ 处理错误  │
        └─────┬────┘  └─────┬────┘  └─────┬────┘
              │            │            │
              ▼            ▼            ▼
        ┌──────────────────────────────┐
        │           CHECK               │
        │   任务完成了吗？               │
        │   是 → 返回结果 → IDLE       │
        │   否  → 回到 PERCEIVE          │
        │   错误 → 重试 / 降级 / 报错   │
        └──────────────────────────────┘
```

### 2.3 Loop 实现

```go
// internal/ai/agent_loop.go

package ai

import (
    "context"
    "fmt"
    "time"
)

// LoopConfig 配置 Agent 循环的行为参数
type LoopConfig struct {
    MaxIterations int           // 最大迭代次数（防止无限循环）
    MaxToolCalls  int           // 单次会话最大 Tool 调用次数
    Timeout       time.Duration // 整体超时
    AutoConfirm   bool          // 低风险操作是否自动确认（无需用户介入）
    Verbose       bool          // 是否输出详细推理日志
}

// DefaultLoopConfig 提供合理的默认值
func DefaultLoopConfig() LoopConfig {
    return LoopConfig{
        MaxIterations: 20,
        MaxToolCalls:  50,
        Timeout:       5 * time.Minute,
        AutoConfirm:   false, // 默认需要用户确认写操作
        Verbose:       true,
    }
}

// Run 执行一次完整的 Agent 会话
func (a *Agent) Run(ctx context.Context, userQuery string, cfg LoopConfig) (*RunResult, error) {
    session := NewSession(userQuery)
    ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
    defer cancel()

    for iteration := 0; iteration < cfg.MaxIterations; iteration++ {
        session.Iteration = iteration

        // === PERCEIVE ===
        percept, err := a.perceive(ctx, session)
        if err != nil {
            return nil, fmt.Errorf("perceive failed: %w", err)
        }

        // === THINK ===
        decision, err := a.think(ctx, session, percept)
        if err != nil {
            return nil, fmt.Errorf("think failed: %w", err)
        }

        switch decision.Action {
        case ActionFinal:
            // LLM 认为任务完成，返回最终答案
            session.Result = decision.Content
            a.logEvent(ctx, session, "final_answer", decision.Content)
            return session.ToResult(), nil

        case ActionToolCall:
            // LLM 决定调用一个工具
            if session.ToolCallCount >= cfg.MaxToolCalls {
                session.Result = fmt.Sprintf("达到最大工具调用次数限制 (%d)", cfg.MaxToolCalls)
                return session.ToResult(), nil
            }

            result, err := a.executeTool(ctx, session, decision.ToolCall, cfg.AutoConfirm)
            session.ToolCallCount++
            a.logEvent(ctx, session, "tool_call",
                fmt.Sprintf("%s(%s) → %s", decision.ToolCall.Name, decision.ToolCall.Input, result.Summary))

            if err != nil {
                // 工具执行失败，让 LLM 决定如何处理（重试/换方式/放弃）
                session.LastError = err.Error()
                continue // 回到 PERCEIVE，带上错误信息
            }

            // 将工具结果写入 Working Memory
            session.WorkingMemory.Append(decision.ToolCall.Name, result)

        case ActionNeedInput:
            // LLM 需要更多信息才能继续
            return session.ToResultWithPrompt(decision.Content), nil
        }
    }

    // 达到最大迭代次数
    session.Result = "达到最大迭代次数，返回当前进展"
    return session.ToResult(), nil
}
```

### 2.4 四个阶段的详细设计

#### PERCEIVE — 感知

```go
func (a *Agent) perceive(ctx context.Context, s *Session) (*Percept, error) {
    p := &Percept{}

    // 1. Working Memory 的当前状态
    p.Working = s.WorkingMemory.Snapshot()

    // 2. Knowledge Graph 中的相关知识
    if len(s.WorkingMemory.FocusItems) > 0 {
        // 只加载焦点论文的图谱上下文，不过载全库
        for _, key := range s.WorkingMemory.FocusItems {
            node, _ := a.graph.GetPaperNode(ctx, key)
            if node != nil {
                p.RelevantPapers = append(p.RelevantPapers, node)
            }
            edges, _ := a.graph.GetEdgesFrom(ctx, key)
            p.RelevantEdges = append(p.RelevantEdges, edges...)
        }
    }

    // 3. 用户本次输入
    p.UserInput = s.UserQuery

    // 4. 近期操作历史（最近 10 条 events）
    p.RecentEvents, _ = a.eventLog.Recently(ctx, 10)

    return p, nil
}
```

#### THINK — 推理（LLM 调用）

这是唯一调用 LLM 的地方。LLM 收到感知信息后输出结构化决策：

```go
type Decision struct {
    Action     DecisionAction `json:"action"`      // final | tool_call | need_input
    Content    string         `json:"content"`    // 最终答案 或 prompt
    ToolCall  *ToolCall      `json:"tool_call"` // 要调用的工具及参数
    Reasoning  string         `json:"reasoning"` // 推理过程（用于日志和调试）
}

type DecisionAction string

const (
    ActionFinal     DecisionAction = "final"
    ActionToolCall  DecisionAction = "tool_call"
    ActionNeedInput DecisionAction = "need_input"
)

func (a *Agent) think(ctx context.Context, s *Session, p *Percept) (*Decision, error) {
    // 构建 system prompt（包含行为规则 + 工具列表 + 图谱上下文）
    systemPrompt := a.buildSystemPrompt(s)

    // 构建消息（感知信息 + 历史对话 + 上次决策）
    messages := a.buildMessages(s, p)

    resp, err := a.llm.Call(ctx, LLMRequest{
        SystemPrompt: systemPrompt,
        Messages:     messages,
        Temperature: 0.3, // 偏向确定性输出
    })
    if err != nil {
        return nil, err
    }

    // 解析 LLM 输出为结构化 Decision
    return parseDecision(resp.Content)
}
```

**System Prompt 的关键组成部分**：

```
你是一个 Zotero 文献研究助手。你的能力包括：
1. 检索和分析文献
2. 从 PDF 全文或标注中提取知识点
3. 发现文献间的隐式关联
4. 生成综合分析报告

你的行为规则：
- 优先使用标注提取（有标注时），而非全文提取
- 自动标注前必须确认（除非用户已授权）
- 关系置信度低于 0.7 时建议给用户确认
- 不要编造不存在的引用或数据
- 如果不确定，选择 ask_user 而不是猜测

可用工具：
- zot_find: 检索文献
- zot_extract_atoms: 提取原子知识单元
- zot_link: 创建知识关系
- zot_synthesize: 综合多篇知识
- zot_annotate_ai: AI 标注
- zot_memory_search: 图谱语义搜索
- zot_memory_manage: 管理工作记忆
```

#### ACT — 行动

```go
func (a *Agent) executeTool(ctx context.Context, s *Session, tc *ToolCall, autoConfirm bool) (*ToolResult, error) {
    switch tc.Name {
    case "zot_find":
        return a.toolFind(ctx, tc.Input)

    case "zot_extract_atoms":
        result, err := a.toolExtractAtoms(ctx, tc.Input)
        if err == nil && autoConfirm {
            // 将新创建的 atoms 写入 Working Memory
            s.WorkingMemory.AddExtracted(result.PaperKey, result.AtomCount)
        }
        return result, err

    case "zot_link":
        result, err := a.toolLink(ctx, tc.Input)
        if err == nil && autoConfirm {
            s.WorkingMemory.AddLink(result.FromKey, result.ToKey)
        }
        return result, err

    case "zot_synthesize":
        return a.toolSynthesize(ctx, tc.Input)

    case "zot_annotate_ai":
        return a.toolAnnotateAI(ctx, tc.Input, autoConfirm)

    case "zot_memory_search":
        return a.toolMemorySearch(ctx, tc.Input)

    case "zot_memory_manage":
        return a.toolMemoryManage(ctx, tc.Input)

    default:
        return nil, fmt.Errorf("unknown tool: %s", tc.Name)
    }
}
```

每个 Tool 的实现内部调用对应的 `GraphStore` 方法 + 现有的 `Reader`/`Writer`。

---

## 3. 记忆系统（三层架构）

### 3.1 设计参照

| 层 | 参照系统 | 类比 | 容量 | 持久性 |
|-----|---------|------|------|--------|
| **Working Memory** | Letta/MemGPT Core Memory | RAM | 小（~10 items） | 易失（随 session 结束） |
| **Knowledge Graph** | Zep Temporal KG / Mem0 Long-term | 硬盘 | 大（全库） | 持久 |
| **Interaction Log** | Zep Event Timeline / Letta Recall | 审计日志 | 无限追加 | 不可变 |

### 3.2 Working Memory（工作记忆）

Agent 在单次会话中的"短期记忆"。容量有限，迫使 Agent 做出信息取舍决策——这本身就是一种学习信号。

```go
// internal/ai/memory_working.go

package ai

import "time"

// WorkingMemory 是 Agent 当前的认知工作集
type WorkingMemory struct {
    // 当前关注的文献（最多 10 个）
    FocusItems []FocusItem `json:"focus_items"`

    // 当前活跃的概念（最多 20 个）
    ActiveConcepts []ConceptRef `json:"active_concepts"`

    // 待办任务队列
    PendingTasks []Task `json:"pending_tasks"`

    // 自由格式上下文：Agent 自己维护的"我在做什么"
    Context string `json:"context"`

    // 本次会话的临时笔记（不写入图谱）
    SessionNotes []SessionNote `json:"session_notes"`

    // 统计信息
    ToolCallCount int `json:"tool_call_count"`
    Iteration     int `json:"iteration"`
}

type FocusItem struct {
    Key       string    `json:"key"`
    Title     string    `json:"title"`
    Reason    string    `json:"reason"`       // 为什么关注这篇
    AddedAt   time.Time `json:"added_at"`
}

type ConceptRef struct {
    Key    string `json:"key"`
    Name   string `json:"name"`
    Domain string `json:"domain,omitempty"`
}

type Task struct {
    ID          string    `json:"id"`
    Description string    `json:"description"`
    TargetKey   string    `json:"target_key,omitempty"`
    Priority    int       `json:"priority"` // 1-5
    Status      string    `json:"status"`   // pending | doing | done | blocked
    CreatedAt   time.Time `json:"created_at"`
}

type SessionNote struct {
    At      time.Time `json:"at"`
    Content string    `json:"content"`
    Tags    []string  `json:"tags,omitempty"`
}

// ---- Agent 可调用的记忆管理操作 ----

// Append 向工作记忆追加内容
func (w *WorkingMemory) Append(toolName string, result interface{}) { ... }

// Replace 替换工作记忆中的某段
func (w *WorkingMemory) Replace(section string, content string) { ... }

// Clear 清空工作记忆（开始新会话时调用）
func (w *WorkingMemory) Clear() { ... }

// Snapshot 导出当前快照（用于传入 LLM context）
func (w *WorkingMemory) Snapshot() map[string]interface{} { ... }

// AddExtracted 记录刚提取的知识点数量
func (w *WorkingMemory) AddExtracted(paperKey string, count int) { ... }

// AddLink 记录刚创建的关系
func (w *WorkingMemory) AddLink(fromKey, toKey string) { ... }

// IsFull 判断工作记忆是否接近容量上限
func (w *WorkingMemory) IsFull() bool {
    return len(w.FocusItems) >= 10 ||
        len(w.ActiveConcepts) >= 20 ||
        len(w.PendingTasks) >= 30
}
```

### 3.3 Interaction Log（操作日志）

不可变的审计追踪。所有 Agent 操作都记录在此，用于 Reflect 和问题排查。

```sql
-- 详见 knowledge-graph.md 中的 events 表定义
-- 此处补充写入规范：

CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    -- 'read'         : 读取操作（find/show/extract）
    -- 'extract'      : 知识提取（create atoms）
    -- 'link'         : 关系建立（create edge）
    -- 'synthesize'   : 综合分析
    -- 'annotate'     : AI 标注
    -- 'search'       : 图谱搜索
    -- 'memory_write' : 工作记忆修改
    -- 'memory_read'  : 工作记忆读取
    -- 'reflect'      : 反思执行
    -- 'confirm'      : 用户确认操作
    -- 'reject'       : 用户否决操作
    -- 'error'        : 错误

    target_keys TEXT DEFAULT '[]',
    input        TEXT DEFAULT '',
    output       TEXT DEFAULT '',
    model         TEXT DEFAULT '',
    duration_ms  INTEGER DEFAULT 0,
    status        TEXT DEFAULT 'success',  -- success | error | partial | rejected
    error_msg     TEXT DEFAULT '',
    created_at    TEXT NOT NULL
);
```

**写入规则**：

1. **所有 Tool Call 必须记录** — 无论成功失败
2. **用户交互必须记录** — confirm / reject / 修正
3. **Reflect 结果必须记录** — 包括 Rules 变更
4. **只追加不更新** — event 一旦写入不可修改（保证审计完整性）

---

## 4. Behavior Rules（行为规则）

这是自进化的核心载体。Rules 不是代码逻辑，而是 **Agent 遵循的策略参数**，可以被 Reflect 过程修改。

### 4.1 Rules 结构

```go
// internal/ai/rules.go

package ai

import "encoding/json"

// BehaviorRules 定义 Agent 的行为策略
// 这些规则会在每次 think 时注入 system prompt
// 同时也是 Reflect 过程的目标对象
type BehaviorRules struct {
    // 版本控制（用于回滚）
    Version     int    `json:"version"`
    ModifiedAt string `json:"modified_at"`
    ModifyReason string `json:"modify_reason,omitempty"`

    Extraction ExtractionRules `json:"extraction"`
    Annotation AnnotationRules `json:"annotation"`
    Linking     LinkingRules     `json:"linking"`
    Output      OutputRules      `json:"output"`
    UserPrefs   UserPreferences `json:"user_preferences"`
}

type ExtractionRules struct {
    PreferAnnotations bool    `json:"prefer_annotations"` // 有标注时优先从标注提取
    FulltextMinPages  int     `json:"fulltext_min_pages"`   // 少于 N 页的论文不做全文提取
    MinConfidence     float64 `json:"min_confidence"`      // 低于此值的 atom 不写入图谱
    AutoLinkThreshold float64 `json:"auto_link_threshold"` // 相似度阈值
    MaxAtomsPerPage   int     `json:"max_atoms_per_page"`  // 每篇论文最多提取多少 atom
}

type AnnotationRules struct {
    DefaultColor     string `json:"default_color"`      // 默认标注颜色
    MaxAnnotsPerPage int   `json:"max_annot_per_page"` // 每页最多自动标几个
    RequireConfirm   bool   `json:"require_confirm"`    // 自动标注前是否需确认
    SkipHighlighted  bool   `json:"skip_highlighted"`   // 跳过已有高亮的位置
}

type LinkingRules struct {
    AllowNovelRelations bool    `json:"allow_novel_relations"` // 允许创建新的 relation 类型吗？
    CrossCollectionOnly  bool    `json:"cross_collection_only"`  // 只跨收藏夹建边？
    TemporalBias         float64 `json:"temporal_bias"`        // 时间相近的论文更容易关联
    MinConfidenceToAuto  float64 `json:"min_confidence_to_auto"` // 多高置信度才自动建边
    SuggestBelow         float64 `json:"suggest_below"`        // 低于此值只建议不自动建
}

type OutputRules struct {
    DetailLevel    string `json:"detail_level"`    // brief | moderate | detailed
    Language       string `json:"language"`         // zh | en | mixed
    CitationStyle  string `json:"citation_style"`   // 学术写作风格偏好
    IncludeSource   bool   `json:"include_source"`   // 回答中是否包含原文出处
    MaxCitations   int    `json:"max_citations"`     // 单次回答最多引用几条 atom
}

type UserPreferences struct {
    ResearchFocus     []string `json:"research_focus"`     // 当前研究方向
    AvoidHallucination bool    `json:"avoid_hallucination"` // 对幻觉零容忍
    PreferredFormat   string  `json:"preferred_format"`   // markdown | plain | json
    TimeHorizon       string  `json:"time_horizon"`       // recent | all_time | custom
}

// DefaultRules 返回出厂默认值
func DefaultRules() BehaviorRules {
    return BehaviorRules{
        Version:     1,
        ModifiedAt: timeNow(),
        Extraction: ExtractionRules{
            PreferAnnotations: true,
            FulltextMinPages: 3,
            MinConfidence:     0.4,
            AutoLinkThreshold: 0.7,
            MaxAtomsPerPage:   15,
        },
        Annotation: AnnotationRules{
            DefaultColor:     "orange",
            MaxAnnotsPerPage: 3,
            RequireConfirm:   true,
            SkipHighlighted:  false,
        },
        Linking: LinkingRules{
            AllowNovelRelations: true,
            CrossCollectionOnly:  false,
            TemporalBias:        0.3,
            MinConfidenceToAuto: 0.8,
            SuggestBelow:        0.6,
        },
        Output: OutputRules{
            DetailLevel:    "moderate",
            Language:       "zh",
            CitationStyle:  "academic",
            IncludeSource:   true,
            MaxCitations:   5,
        },
        UserPrefs: UserPreferences{
            ResearchFocus:     []string{},
            AvoidHallucination: true,
            PreferredFormat:   "markdown",
            TimeHorizon:       "all_time",
        },
    }
}
```

### 4.2 Rules 如何影响 Agent 行为

Rules 不是 if-else 分支，而是**注入到 System Prompt 中**，让 LLM 在推理时自然遵守：

```go
func (a *Agent) buildSystemPrompt(s *Session) string {
    rules := a.rules // 当前生效的 Rules

    var sb strings.Builder
    sb.WriteString("你是 Zotero 文献研究助手。\n\n")
    sb.WriteString("## 你的行为规则\n\n")

    // Extraction
    sb.WriteString(fmt.Sprintf(`### 知识提取
- 优先从标注提取: %v
- 少于 %d 页的论文跳过全文提取
- 置信度低于 %.1f 的知识点不存入图谱
- 每篇论文最多提取 %d 条知识单元
`,
        rules.Extraction.PreferAnnotations,
        rules.Extraction.FulltextMinPages,
        rules.Extraction.MinConfidence,
        rules.Extraction.MaxAtomsPerPage))

    // Annotation
    sb.WriteString(fmt.Sprintf(`### 自动标注
- 默认颜色: %s
- 每页最多自动标注 %d 处
- 需要用户确认: %v`,
        rules.Annotation.DefaultColor,
        rules.Annotation.MaxAnnotsPerPage,
        rules.Annotation.RequireConfirm))

    // Linking
    sb.WriteString(fmt.Sprintf(`### 关系推断
- 允许创建新关系类型: %v
- 自动建边最低置信度: %.1f
- 仅建议（不自动建）的最低置信度: %.1f
- 时间偏向权重: %.1f`,
        rules.Linking.AllowNovelRelations,
        rules.Linking.MinConfidenceToAuto,
        rules.Linking.SuggestBelow,
        rules.Linking.TemporalBias))

    // Output
    sb.WriteString(fmt.Sprintf(`### 输出风格
- 详细程度: %s
- 语言: %s
- 每次回答最多引用 %d 条知识点
- %v包含原文出处`,
        rules.Output.DetailLevel,
        rules.Output.Language,
        rules.Output.MaxCitations,
        yesNo(rules.Output.IncludeSource))

    // User Preferences
    if len(rules.UserPrefs.ResearchFocus) > 0 {
        sb.WriteString(fmt.Sprintf("\n### 用户研究方向\n- %s\n",
            joinStrings(rules.UserPrefs.ResearchFocus, ", ")))
    }
    if rules.UserPrefs.AvoidHallucination {
        sb.WriteString("- 绝对不要编造不存在的引用或数据\n")
    }

    // Available Tools
    sb.WriteString("\n## 可用工具\n")
    sb.WriteString(a.toolsDescription())

    return sb.String()
}
```

这样 Rules 的修改会**立即影响下一次 LLM 推理**，不需要改代码或重启。

### 4.3 Rules 持久化

```go
// 存储在 {DataDir}/.zotero_cli/ai/rules.json
// 启动时加载，Reflect 后可被覆写

func LoadRules(path string) (BehaviorRules, error) {
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return DefaultRules(), nil
    }
    var rules BehaviorRules
    if err := json.Unmarshal(data, &rules); err != nil {
        return BehaviorRules{}, err
    }
    return rules, nil
}

func SaveRules(path string, rules BehaviorRules error) {
    data, err := json.MarshalIndent(rules, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0600)
}
```

---

## 5. 自进化机制（Reflect → Adapt）

### 5.1 设计参照

| 机制 | 来源 | 核心思想 |
|------|------|---------|
| **Procedural Memory** | LangMem | Agent 不仅存储事实，还存储"如何行动"的规则，可根据反馈重写 |
| **Self-editing Memory** | Mem0 | 新记忆写入时检测冲突，自动选择 update 或 create |
| **Core Memory Management** | Letta/MemGPT | Agent 自主决定记忆读写时机，有限容量产生自然的信息取舍压力 |
| **Reflection Pattern** | Reflexion-based Agents | 定期回顾近期操作，提取模式并改进策略 |

### 5.2 Reflect 流程

```go
// internal/ai/reflect.go

package ai

import (
    "context"
    "fmt"
    "time"
)

// ReflectResult 反思结果
type ReflectResult struct {
    RanAt         time.Time  `json:"ran_at"`
    PeriodStart   time.Time  `json:"period_start"`
    PeriodEnd     time.Time  `json:"period_end"`
    TotalEvents   int        `json:"total_events"`

    // 统计摘要
    ConfirmationRate   float64            `json:"confirmation_rate"`    // 用户确认率
    RejectionRate      float64            `json:"rejection_rate"`       // 用户否决率
    TopErrors         []ErrorPattern     `json:"top_errors"`          // 最常见错误
    TopSuccesses       []string            `json:"top_successes"`        // 最成功的操作类型

    // Rules 变更建议
    RulesToUpdate     RulesDelta         `json:"rules_update"`        // 要修改的 Rules diff
    PatternsFound     []Pattern          `json:"patterns_found"`      // 发现的模式
    ConfidenceAdjust  map[string]float64 `json:"confidence_adjust"` // 各类操作的置信度修正

    // 建议
    Recommendations   []string            `json:"recommendations"`     // 给用户的建议
    Warnings         []string            `json:"warnings"`           // 风险警告
}

type ErrorPattern struct {
    Pattern  string `json:"pattern"`   // 错误模式描述
    Count    int    `json:"count"`     // 出现次数
    Example  string `json:"example"`  // 具体例子
}

type Pattern struct {
    Description string `json:"description"`
    Evidence    string `json:"evidence"`   // 支撑证据
    Confidence  float64 `json:"confidence"`
}

type RulesDelta struct {
    Field       string  `json:"field"`       // 如 "extraction.auto_link_threshold"
    OldValue    any     `json:"old_value"`
    NewValue    any     `json:"new_value"`
    Reason      string  `json:"reason"`      // 为什么改
}

// Reflect 执行一次反思
func (a *Agent) Reflect(ctx context.Context, since time.Time) (*ReflectResult, error) {
    now := time.Now().UTC()
    period := since.Sub(now).Hours()

    if period < 1.0 {
        return nil, fmt.Errorf("反思窗口至少需要 1 小时，当前仅 %.1f 小时", period)
    }

    // 1. 收集近期事件
    events, err := a.eventLog.Since(ctx, since)
    if err != nil {
        return nil, err
    }
    if len(events) == 0 {
        return &ReflectResult{RanAt: now}, nil
    }

    // 2. 分类统计
    stats := classifyEvents(events)

    // 3. 构建 Reflect Prompt（注入统计 + Rules 当前值 + 典型事件）
    reflectPrompt := a.buildReflectPrompt(stats, a.rules)

    // 4. LLM 分析并输出改进建议
    resp, err := a.llm.Call(ctx, LLMRequest{
        SystemPrompt: "你是一个 Agent 行为分析师。你的任务是分析近期的操作日志，发现模式，提出具体的规则参数调整建议。",
        Messages: []Message{
            {Role: "user", Content: reflectPrompt},
        },
        Temperature: 0.2, // 低温度，偏确定性的分析
    })
    if err != nil {
        return nil, err
    }

    // 5. 解析 LLM 输出为结构化 ReflectResult
    result := parseReflectResult(resp.Content)
    result.RanAt = now
    result.PeriodStart = since
    result.PeriodEnd = now
    result.TotalEvents = len(events)
    result.ConfirmationRate = stats.ConfirmationRate
    result.RejectionRate = stats.RejectionRate

    // 6. 记录 reflect 事件本身
    a.logEvent(ctx, nil, "reflect",
        fmt.Sprintf("events=%d confirm_rate=%.2f reject_rate=%.2f recommendations=%d warnings=%d",
            len(events), result.ConfirmationRate, result.RejectionRate,
            len(result.Recommendations), len(result.Warnings)))

    return result, nil
}
```

### 5.3 Adapt — 应用改进

```go
// ApplyReflection 将 Reflect 结果应用到 Rules 上
func (a *Agent) ApplyReflection(r *ReflectResult) (ApplyResult, error) {
    result := ApplyResult{AppliedAt: time.Now().UTC()}

    for _, delta := range r.RulesToUpdate {
        oldValue := getRuleField(a.rules, delta.Field)
        newValue := setRuleField(a.rules, delta.Field, delta.NewValue)

        // 安全检查：变化幅度是否过大？
        changeMagnitude := calculateChangeMagnitude(delta.Field, oldValue, newValue)
        if changeMagnitude > 0.5 {
            // 大幅变动需要记录但不自动应用
            result.LargeChanges = append(result.LargeChanges, ChangeRecord{
                Field:    delta.Field,
                OldValue: oldValue,
                NewValue: newValue,
                Reason:   delta.Reason,
                RequiresUserApproval: true,
            })
            continue
        }

        // 应用变更
        setRuleField(a.rules, delta.Field, delta.NewValue)
        result.AppliedDeltas = append(result.AppliedDeltas, delta)
    }

    // 更新版本号
    a.rules.Version++
    a.rules.ModifiedAt = time.Now().UTC().UTC()
    if len(r.RulesUpdateReason) > 0 {
        a.rules.ModifyReason = r.RulesUpdateReason[0]
    }

    // 持久化
    if err := SaveRules(a.rulesPath, a.rules); err != nil {
        return result, err
    }

    // 记录到 event log
    a.logEvent(context.Background(), nil, "rules_updated",
        fmt.Sprintf("version=%d changes=%d large_changes=%d",
            a.rules.Version, len(result.AppliedDeltas), len(result.LargeChanges)))

    return result, nil
}
```

### 5.4 进化的完整生命周期示例

```
=== 初始状态 (Day 0) ===
Rules v1 (默认):
  extraction.prefer_annotations = true
  annotation.require_confirm = true
  linking.min_confidence_to_auto = 0.8

=== Day 1-3 运行 ===
事件统计:
  - 30 次 extract 操作, 24 次确认, 6 次拒绝 → 确认率 80%
  - 最常见错误: "hallucinated citation in atom content" (3次)
  - 用户手动关闭了 4 次确认弹窗

=== Day 3 Reflect ===
输出:
  RulesDelta: [
    {field: "annotation.require_confirm", old: true, new: false,
     reason: "用户频繁关闭确认弹窗(4/30)，说明对 highlight 类型标注信任度高"},
    {field: "linking.min_confidence_to_auto", old: 0.8, new: 0.75,
     reason: "80% 确认率表明可以适当放宽自动建边门槛"},
  ]
Patterns: [
    {description: "用户倾向于保留对比类关系", evidence: "...", confidence: 0.82},
    {description: "全文提取时方法类 atom 质量最高", evidence: "...", confidence: 0.91},
  ]
Recommendations: ["考虑降低非 highlight 标注的确认要求"]

=== Adapt 应用 ===
Rules v2:
  annotation.require_confirm = false  ✓ 已应用
  linking.min_confidence_to_auto = 0.75 ✓ 已应用

=== Day 4-7 运行（使用 Rules v2）===
事件统计:
  - 45 次 extract 操作, 40 次确认, 5 次拒绝 → 确认率 89% ↑
  - 确认率提升 9 个百分点
  - 新增错误: "auto-created relation type too specific" (2次)

=== Day 7 Reflect ===
输出:
  RulesDelta: [
    {field: "linking.allow_novel_relations", old: true, new: false,
     reason: "Agent 创建了过细粒度的关系类型(2次)，应限制在预定义集合内"},
    {field: "output.max_citations", old: 5, new: 7,
     reason: "用户反馈希望看到更多引用来源"},
  ]

=== 最终效果 ===
Week 1: 确认率 80%, 用户满意度 3.5/5
Week 2: 确认率 89%, 用户满意度 4.1/5  (+17%)
(持续运行中...)
```

### 5.5 安全护栏

自进化不是无约束的。以下机制防止退化：

| 机制 | 说明 |
|------|------|
| **版本号 + 回滚** | 每次 Rules 变更递增版本号，保留历史版本。`zot ai-graph rollback --to-version N` 可回滚 |
| **变化幅度检查** | 单次参数变动超过阈值（如 confidence 变动 >0.3）需要用户审批 |
| **边界约束** | 所有 Rule 参数都有合法范围（如 confidence ∈ [0, 1]），Adapt 不会写出界值 |
| **退化检测** | 如果连续两次 Reflect 的确认率下降，自动回滚到上一版本并告警 |
| **人工覆盖** | 用户随时可通过 `zot ai-rules edit` 手动编辑 Rules，优先级高于自动进化 |

---

## 6. MCP Server 设计

### 6.1 Server 身份与能力声明

```go
// internal/ai/mcp_server.go

const (
    ServerName    = "zotero-ai"
    ServerVersion = "0.1.0"
    ServerDesc    = "Zotero AI Knowledge Graph & Literature Agent MCP Server"
)

// Server 启动时声明的能力
func serverInfo() map[string]any {
    return map[string]any{
        "name":        ServerName,
        "version":     ServerVersion,
        "description": ServerDesc,
        "capabilities": []string{
            "resources_read",   // 提供 Resources
            "tools",            // 提供 Tools
            "prompts",           // 提供 Prompts
            "logging",          // 支持 logging level
        },
        "tools": []string{
            "zot_find", "zot_extract_atoms", "zot_link",
            "zot_synthesize", "zot_annotate_ai",
            "zot_memory_search", "zot_memory_manage",
            "zot_reflect", "zot_graph_stats",
        },
        "resources": []string{
            "zotero://library/overview",
            "zotero://papers/{key}",
            "zotero://graph/overview",
            "zotero://graph/subgraph",
            "zotero://graph/path",
            "zotero://graph/suggestions",
            "zotero://graph/timeline",
            "zotero://agent/memory",
            "zotero://agent/rules",
        },
        "prompts": []string{
            "zotero://prompts/literature-review",
            "zotero://prompts/paper-critique",
            "zotero://prompts/research-plan",
        },
    }
}
```

### 6.2 Tools 定义

每个 Tool 对应一个 MCP tool schema：

```go
var mcpTools = []MCPTool{
    {
        Name: "zot_find",
        Description: `检索 Zotero 文献库。支持关键词搜索、全文搜索（FTS5）、按日期/标签/收藏夹过滤。
支持 mode 参数切换：keyword（默认）、fulltext（全文匹配）、semantic（后期向量检索）。
返回结构化文献列表，每条包含 title/authors/date/tags/attachments 信息。`,
        InputSchema: findToolSchema,
    },
    {
        Name: "zot_extract_atoms",
        Description: `从论文中提取原子知识单元（Atom）。这是构建知识图谱的基本操作。
strategy 参数控制提取策略：
- "from_annotations": 从用户的 PDF 标注中提炼（推荐，已有标注时使用）
- "from_fulltext": 从 PDF 全文中提取（适用于无标注或标注稀疏的论文）
- "incremental": 增量提取（只处理新增/变更的部分）
输出为结构化的 Atom 列表，每条 Atom 包含内容、角色、页码、溯源信息。
Atom 会持久化到知识图谱中，后续可用于综合分析和问答。`,
        InputSchema: extractAtomsToolSchema,
    },
    {
        Name: "zot_link",
        Description: `在两个节点（论文或概念）之间建立知识关系（Edge）。
relation 参数指定关系类型（如 supports / contradicts / compares / uses_method 等，
也允许自定义新的关系类型）。
evidence 参数提供关系的依据（一句话或引用段落）。
如果 auto=true 且置信度足够高，会直接创建边；
否则返回建议等待确认。`,
        InputSchema: linkToolSchema,
    },
    {
        Name: "zot_synthesize",
        Description: `综合多个节点的知识，生成新的洞察。
keys 参数指定要综合的论文 key 列表。
focus_question 引导综合的方向（如"这些论文在方法论上有什么异同？"）。
depth 控制综合深度（1=表面对比 2=深入分析 3=跨主题综合）。
输出为新的 synthesis 类型内容，可选择写入图谱或直接返回。`,
        InputSchema: synthesizeToolSchema,
    },
    {
        Name: "zot_annotate_ai",
        Description: `基于 AI 分析自动在 PDF 上创建标注。
strategy 控制："highlights"（高亮重要段落）、"notes"（添加批注笔记）、"both"。
paper_key 指定目标论文。
如果 require_confirm=true（默认），会先展示建议标注位置等用户确认后才实际写入 PDF。
此操作通过现有的 annotate 命令实现，标注可在 Zotero 阅读器中查看。`,
        InputSchema: annotateAIToolSchema,
    },
    {
        Name: "zot_memory_search",
        Description: `在知识图谱中进行语义/结构化搜索。
query 为搜索词或自然语言问题。
node_type_filter 限定搜索范围（paper/concept/all）。
limit 控制返回数量。
返回匹配的节点及其关联的 atoms 和 edges。`,
        InputSchema: memorySearchToolSchema,
    },
    {
        Name: "zot_memory_manage",
        Description: `管理工作记忆（Working Memory）。
action 选择操作：
- "append": 追加内容到工作记忆的指定区域
- "replace": 替换工作记忆中的某段内容
- "clear": 清空工作记忆
- "show": 查看当前工作记忆快照
section 指定操作的区域（focus_items / active_concepts / pending_tasks / context / notes）。
content 为要写入的内容。`,
        InputSchema: memoryManageToolSchema,
    },
    {
        Name: "zot_reflect",
        Description: `触发 Agent 反思机制。扫描近期的操作日志，
分析确认率/否决率/常见错误模式，输出 Rules 调整建议。
since 指定反思的时间窗口（如 "24h"、"7d"、"168h"）。
dry_run=true 时只输出建议不实际修改 Rules。
此工具应谨慎使用，建议定期（如每日或每周）触发而非频繁调用。`,
        InputSchema: reflectToolSchema,
    },
    {
        Name: "zot_graph_stats",
        Description: `获取知识图谱的整体统计数据。
包括节点数、边数、atom 数量、确认/否认比例、平均置信度、
各状态的论文分布、各领域的概念分布等。`,
        InputSchema: graphStatsToolSchema,
    },
}
```

### 6.3 Transport 选择

| 场景 | 推荐 Transport | 原因 |
|------|---------------|------|
| Claude Code / Cursor IDE 集成 | stdio | IDE 的 MCP client 通过 stdio 启动子进程 |
| 远程服务 / 多客户端 | Streamable HTTP | 生产环境标准 |
| 本地脚本调用 | 都支持 | 取决于部署方式 |

初始实现优先 stdio（最简单，Claude Code 开箱即用）。

---

## 7. 会话管理

### 7.1 Session 生命周期

```
用户提问 / 定时触发
       │
       ▼
┌──────────────┐
│  Session     │  创建
│  Create      │  - 生成 session_id
│              │  - 加载 Working Memory（空或恢复）
│              │  - 加载当前 Rules
│              │  - 记录 start 事件
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Agent Loop  │  运行（PERCEIVE→THINK→ACT→CHECK 循环）
│  Run         │  - 可能产生多次 Tool Call
│              │  - 每步都记录到 event log
│              │  - Working Memory 持续演化
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Session     │  结束
│  Finalize    │  - 将有价值的 Working Memory 内容持久化到 Graph
│              │    （如新发现的 concept 应写入 graph）
│              │  - 保存 session 统计信息
│              │  - 返回结果
└──────────────┘
```

### 7.2 Session 数据结构

```go
type Session struct {
    ID            string        `json:"id"`             // UUID
    UserQuery     string        `json:"user_query"`     // 原始用户输入
    StartedAt     time.Time     `json:"started_at"`
    EndedAt       *time.Time    `json:"ended_at,omitempty"`

    WorkingMemory *WorkingMemory `json:"working_memory"` // 会话级记忆
    Rules         *BehaviorRules `json:"rules"`           // 快照（防止运行中被修改）

    Result        string        `json:"result,omitempty"` // 最终答案
    Iteration     int           `json:"iteration"`
    ToolCallCount int           `json:"tool_call_count"`
    LastError     string        `json:"last_error,omitempty"`

    // 内部：对话历史（用于构建 LLM messages）
    history []Message `json:"-"` // 不序列化到外部
}
```

### 7.3 Session 持久化

Session 本身不长期保存，但**有价值的内容会被提取并存入 Graph**：

```
Session 结束时:
  1. WorkingMemory.FocusItems 中标记为 "important" 的 → 检查是否需要在 graph 中建立新关联
  2. WorkingMemory.SessionNotes 中有价值的 → 可选转为 user_manual 类型的 atom
  3. Session 统计 → 追加到 event log
  4. Session 本身 → 可选保存到 sessions 表（用于 debug，不影响 graph）
```

---

## 8. 与 knowledge-graph.md 的分工

| 文件 | 负责 | 读者 |
|------|------|------|
| **knowledge-graph.md** | 数据模型、存储方案、CRUD 接口、查询语言、ER 图、MCP Resource 映射 | 想理解"数据长什么样、怎么存怎么查"的人 |
| **agent-design.md** | 运行时架构、Agent Loop、记忆分层、行为规则、自进化机制、MCP Tool 定义、会话管理 | 想理解"Agent 怎么运行、怎么思考、怎么进化"的人 |

两者关系：knowledge-graph 定义的是**"有什么"**（数据和接口），agent-design 定义的是**"怎么用"**（行为和流程）。

---

## 9. 文件结构（新增）

```
internal/ai/
├── types.go              # 所有 Go 类型定义（Node/Edge/Atom/Rules/Session/...）
├── graph_store.go         # GraphStore 接口 + SQLite 实现
├── schema.sql              # DDL（内嵌在 graph_store.go 中）
├── agent_loop.go          # Agent 主循环（PERCEIVE→THINK→ACT→CHECK）
├── perceives.go           # 感知阶段实现
├── think.go               # 推理阶段实现（含 prompt 构建）
├── act.go                 # 行动阶段实现（Tool 路由）
├── tools/                 # 各 Tool 的具体实现
│   ├── tool_find.go
│   ├── tool_extract.go
│   ├── tool_link.go
│   ├── tool_synthesize.go
│   ├── tool_annotate.go
│   ├── tool_memory.go
│   └── tool_reflect.go
├── memory/
│   ├── working.go          # Working Memory 实现
│   └── rules.go             # Behavior Rules + 持久化
├── reflect.go             # Reflect 逻辑
├── adapt.go               # Adapt（应用 Rules 变更）
├── event_log.go           # Interaction Log 读写
├── llm/
│   ├── caller.go           # LLM 抽象接口
│   ├── openai.go           # OpenAI 实现
│   ├── ollama.go           # Ollama（本地模型）实现
│   └── mock.go              # 测试用 Mock
├── mcp_server.go          # MCP Server（stdio + HTTP）
├── prompts/
│   ├── literature_review.go
│   ├── paper_critique.go
│   └── research_plan.go
└── session.go             # Session 管理
```

CLI 入口文件（新增）：

```
internal/cli/
└── commands_ai_graph.go    # zot ai-graph * 命令
└── commands_agent.go      # zot agent * 命令（可选，MCP 是主要入口）
```
