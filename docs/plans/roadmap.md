# 路线图

当前版本目标与执行顺序。历史迭代见 [archive/](../archive/)。

---

## 当前焦点

### v0.0.x — Agent 可用性增强（进行中）

**总目标**：不大幅扩张命令面，聚焦三件事：

1. **写操作安全性** — `--dry-run` 模式、管道连接
2. **多模态能力** — PDF 图片提取与分析
3. **体验打磨** — 降低 agent 使用门槛的持续改进

#### 执行顺序

| 阶段 | 内容 | 状态 |
|------|------|------|
| 阶段 1 | 标注 `--dry-run` 预览模式 + comment 截断修复 | 待开始 |
| 阶段 2 | 批量标注（`--from-file` JSON 驱动） | 待开始 |
| 阶段 3 | `find` → `export` 管道连接（`--from-find`） | 待开始 |
| 阶段 4 | 图片解析与分析（`extract-images`） | 待开始 |

> **本版不做**：local full-text search 增强 / MCP server / 大规模命令扩张 / 本地数据库写入（基础能力已就绪）

#### 标注系统后续优化（从实战中识别）

v0.0.4 的 annotate/annotations 命令已完成核心功能，实际使用中暴露以下改进点：

| 优先级 | 改进项 | 说明 | 阻塞阶段 |
|--------|--------|------|----------|
| **P0** | `--dry-run` 预览模式 | annotate 不执行写入，仅返回匹配结果（文本+位置+上下文），解决盲目写入痛点 | 阶段 1 |
| **P0** | comment 截断去除 | Python 脚本 `comment[:200]` 硬截断，方法论笔记被截断影响核心用途 | 阶段 1 |
| **P1** | 批量标注 `--from-file` | JSON 数组描述多条标注点（page/text/color/comment），一次 CLI 调用完成整篇论文标注 | 阶段 2 |
| **P1** | DB type 完整映射 | 当前仅 highlight/note/image 3 种，补全 underline/strikeout/squiggly 等 | 阶段 2 |
| **P2** | 标注前 PDF 快照 | `--clear` 前自动备份 PDF，支持回滚 | 阶段 3+ |
| **P2** | 匹配结果上下文展示 | 返回匹配文本前后 N 字符辅助判断正确性 | 阶段 2 |

### 已完成迭代

| 版本 | 目标 | 结果 |
|------|------|------|
| MVP (v0.0.1) | 基础读路径 | shipped |
| Stability Pass (v0.0.2) | hybrid fallback 稳定 + find 语义收敛 + 写错误改善 | completed |
| PDF & Annotation (v0.0.3) | extract-text / show 标注 / local find 附件感知 / FTS5 全文检索扩展 | completed |
| Write & Integrate (v0.0.4) | annotate/open/select/annotations 命令 + Makefile + pre-commit + Exit Code + 文档体系 + --clear双层删除/Mode 1.5/--author/DeleteDBAnnotations/ANNO_TYPES完整映射/三份文档更新 | completed |
| Find Enhancement (v0.0.5) | find 高级过滤 11 项 + auto fulltext + snippet 缓存 ~20x + agent 模式 P1 优化 | completed |
| Unified Init (v0.0.6) | `zot init` 一站式入口 + `overview` 命令 + JSON 结构化错误 + schema 统一 + 测试拆分 + 并行加速 ~3x + Web 前端 MVP | completed |

---

## 下一阶段（并行方向）

### A. Zotero 原生能力深化（规划中）

基于对 Zotero Web API v3 和 Zotero 7/9 本地数据库的深度调研，以下为可显著提升 CLI 能力上限的方向。

详细方案见 [optimizations/native-integration.md](./optimizations/native-integration.md)。

#### P0 — 立即可做（高 ROI）

| 方向 | 说明 | 预期收益 |
|------|------|----------|
| **条件请求缓存** | `If-Modified-Since-Version` / ETag → 304 Not Modified | 未变更数据零网络开销，stats/collections/tags 等高频查询提速数倍 |
| **批量写入合并** | 单次 API 请求打包最多 50 个对象 | create/update 批量操作从 O(n) 降至 O(n/50) |
| **导出格式透传** | 直接透传 Zotero 支持的 20+ 导出格式（BibTeX/RIS/TEI 等） | 无需本地实现 CSL-JSON 以外的任何格式转换 |

#### P1 — 中期增强

| 方向 | 说明 | 前置依赖 |
|------|------|----------|
| **Full-text Content API** | hybrid 模式下全文检索的 Web 回退路径 | FTS5 本地索引已就绪 |
| **OAuth 授权流程** | 替代手动 API Key 生成，浏览器授权一键完成 | 新增 HTTP callback server |
| **Translation Server 对接** | 利用 Zotero 网页翻译器从 URL 自动提取文献元数据 | 需要 translator 目录或远程服务 |
| **Zotero 9 新字段利用** | `lastRead` / `citationKey` / `groupItems` 等新 schema 字段 | local schema 兼容层已建立 |

#### P2 — 长期方向

| 方向 | 说明 | 复杂度 |
|------|------|--------|
| **WebSocket 实时推送** | 订阅库变更事件，替代轮询 | 需长连接管理 |
| **完整同步协议** | 双向 5 步同步（本地 ↔ 云端） | 涉及冲突解决策略 |
| **本地全文字表复用** | 复用 Zotero 内置 `fulltextWords` / `fulltextItemWords` 表 | 替代自建 FTS5 或与之互补 |

### B. Web 前端完善（规划中）

前端在 v0.0.6 已交付 MVP（6 页面 + Go server + pdf.js 预览），但设计文档 [frontend.md](../dev/frontend.md) 中规划的组件和功能仅部分实现。以下为从"可用"到"好用"的演进路线。

#### 当前状态评估

| 层面 | 设计文档规划 | v0.0.6 实际状态 | 差距 |
|------|-------------|-----------------|------|
| **页面** | Dashboard / Library / ItemDetail / Search / Tags / Export（6 个） | 6 个全部实现 | 无 |
| **基础组件** | Layout / Sidebar / ErrorBoundary / PdfViewer / StatCard / EmptyState | Layout + ErrorBoundary + PdfViewer（3 个） | 缺 Sidebar/StatCard/EmptyState |
| **数据组件** | ItemCard / ItemTable / CollectionTree / TagFilter / SearchBar / AnnotationPanel / Pagination / DateFilter | 均未独立抽取，内联在各页面中 | 缺 8 个可复用组件 |
| **shadcn/ui** | button / input / card / badge / tabs / dialog / select / tooltip（8+ 个） | 未安装 `components/ui/` 目录不存在 | 缺全部基础 UI 组件 |
| **自定义 Hooks** | useItems / useCollections / useDebounce | `hooks/` 目录为空 | 缺全部 |
| **写操作 UI** | 创建/编辑/删除文献表单、标签管理、收藏夹管理 | 后端 `write_handlers.go` 不存在 | 缺前后端 |
| **测试** | 组件测试 + Handler 测试 | 4 个组件测试 + 6 个 handler 测试 | 覆盖率偏低 |

#### Phase 1 — 组件基建（补齐设计文档缺口）

| 任务 | 说明 | 优先级 |
|------|------|--------|
| **shadcn/ui 初始化** | 运行 `npx shadcn@latest add` 安装 button/input/card/badge/tabs/dialog/select/tooltip/skeleton/toast | P0 — 所有页面共享基础 |
| **抽取可复用组件** | 从各页面内联代码中提取 ItemCard / ItemTable / CollectionTree / TagFilter / SearchBar / Pagination / StatCard / EmptyState / DateFilter | P0 — 消除重复、统一交互 |
| **自定义 Hooks** | 实现 `useItems`（含筛选/排序/分页）、`useCollections`（树形展开）、`useDebounce`（搜索防抖） | P1 — 封装 TanStack Query 调用模式 |

#### Phase 2 — 体验打磨 ✅ (v0.0.7)

> 已完成：Skeleton 骨架屏（6 页面布局匹配）、Toast 通知系统（Context+Reducer，4 变体）、PdfViewer 动态 import() 懒加载。97 测试全绿。

| 方向 | 具体措施 | 状态 |
|------|----------|------|
| **Skeleton 加载** | `ui/skeleton.tsx` + `PageSkeletons.tsx`（6 个页面骨架布局） | ✅ 完成 |
| **Toast 通知系统** | `hooks/useToast.tsx` + `components/Toaster.tsx`（success/error/warning/info） | ✅ 完成 |
| **空状态设计** | EmptyState 组件（Phase 1 已交付） | ✅ 完成 |
| **PdfViewer 懒加载** | 静态 `import` → 动态 `await import('pdfjs-dist')` | ✅ 完成 |
| **列表虚拟化** | Library 页面 >100 条时启用 `@tanstack/react-virtual` | ⏸ 延后（当前数据量无需） |

#### Phase 3 — 写操作与交互深化 ⏸ (阻塞：后端无写基础设施)

> **前置依赖未满足**。当前后端 `handlers.go` 仅注册 GET 路由（10 个只读端点），`backend.Reader` 接口不含任何写方法，不存在 `Writer` 接口或 `write_handlers.go`。
>
> Phase 3 全部功能（条目 CRUD、标签管理、收藏夹操作、标注写回）均依赖后端写层，需先完成以下基建才能启动：
>
> 1. 定义 `backend.Writer` 接口（CreateItem / UpdateItem / DeleteItem / AddTag / RemoveTag 等）
> 2. 实现 Zotero Web API 的写调用封装
> 3. 在 `handlers.go` 中注册 POST/PUT/DELETE 路由
> 4. 前端 API client 扩展写方法 + Dialog 表单组件
>
> **结论：Phase 3 整体延后，等后端写层就绪后再排期。**

| 功能 | 说明 | 复杂度 | 阻塞原因 |
|------|------|--------|---------|
| **条目创建/编辑弹窗** | Dialog 表单覆盖核心字段（title/authors/date/DOI/itemType） | 中 | 后端无 POST 路由 |
| **标签快速管理** | 条目详情页标签区域支持添加/删除 tag，带 autocomplete | 低 | 后端无 PUT 路由 |
| **收藏夹树交互** | CollectionTree 支持拖拽排序、右键菜单（新建子收藏夹/重命名/删除） | 中 | 后端无 Collection 写接口 |
| **标注面板增强** | AnnotationPanel 支持点击跳转 PDF 对应位置、颜色筛选、按类型分组 | 中 | 后端无 Annotation 写接口 |
| **导出实时预览** | Export 页面选择格式后即时渲染 BibTeX/RIS/CSL-JSON 预览 | 低 | 纯前端可实现，不依赖后端写 |

#### Phase 4 — 性能与工程化

| 方向 | 措施 | 收益 |
|------|------|------|
| **路由级 Code Splitting** | 各页面 `React.lazy()` + `Suspense` | 初始 bundle 从 ~500KB 降至 ~150KB |
| **API 响应缓存策略** | TanStack Query `staleTime` + `gcTime` 分层配置（stats=5min, items=30s, detail=0） | 减少冗余请求 |
| **ESLint + Prettier** | 统一代码风格，pre-commit 自动化 | 可维护性 |
| **测试覆盖率提升** | 页面级集成测试（Library 筛选流程、ItemDetail Tab 切换、Search 全文查询） | 回归安全网 |
| **无障碍（a11y）** | ARIA 标签补全、键盘导航（Tab/Arrow/Esc）、焦点陷阱（Dialog 内）、色彩对比度检查 | WCAG 2.1 AA 合规 |

### 性能基线参考

当前命令耗时（web 模式，详见 [performance-baseline.md](../reference/performance-baseline.md)）：

| 耗量级 | 命令 | 当前耗时 | 优化目标 |
|--------|------|----------|----------|
| 最慢 | export | ~19s | < 5s（条件缓存 + 批量） |
| 慢 | show | ~8s | < 3s（并行加载） |
| 中等 | find / collections | 0.3-0.5s | 保持（快照缓存已生效） |
| 快 | stats / schema / delete | 1-2s | 保持 |

限流分层方案见 [optimizations/rate-limiting.md](./optimizations/rate-limiting.md)（Retry-After → Jitter → Token Bucket → ETag 缓存 → 熔断器）。

---

## 中期：知识图谱与智能体基础设施（未排期）

### Knowledge Graph

详细设计见 [knowledge-graph.md](./knowledge-graph.md)。

| 阶段 | 内容 | 说明 |
|------|------|------|
| P0 | Schema 定义 + GraphStore 接口 | SQLite 图存储、节点/边/Atom 数据模型 CRUD |
| P1 | 标注→Atom 自动提取 | 从 PDF highlight/note 自动生成最小知识单元 |
| P2 | 关系推理 + 置信度评分 | 概念涌现、用户反馈闭环、来源追溯至原始标注 |
| P3 | MCP Resource 映射 | 将图谱节点暴露为 MCP 可访问资源 |

### Agent Runtime

详细设计见 [agent-runtime.md](./agent-runtime.md)。

| 组件 | 说明 |
|------|------|
| **Agent Loop** | PERCEIVE → THINK → ACT → CHECK 四阶段循环，LLM 驱动决策 |
| **三层记忆** | Working Memory（会话上下文）+ Knowledge Graph（持久知识）+ Interaction Log（历史行为） |
| **Behavior Rules** | 可配置的提取/标注/链接策略参数 |
| **自进化机制** | Reflect → Adapt：根据执行结果自动调整行为规则 |
| **MCP Server** | stdio/HTTP 传输，9 个工具 + 8 个资源 |

---

## 远期：智能体笔记系统（向后推迟）

> 此部分不在当前开发计划中，作为远期愿景记录。

### 设计理念

智能体笔记不是简单的"笔记编辑器"，而是以 **Annotation Atom 为核心** 的半自动化学术写作辅助系统：

```
PDF 标注 (highlight/note/image)
    ↓ 自动提取
Atom（最小知识单元：一段文本 + 来源定位 + 初始标签）
    ↓ Agent 辅助整理
Note Card（带结构化元数据的笔记卡片）
    ↓ 关联聚合
Literature Review Chapter（文献综述章节草稿）
```

### 核心能力（按实现顺序）

| 能力 | 说明 | 依赖 |
|------|------|------|
| **标注归一化** | 统一 DB 标注与 PDF 标注的数据模型，消除双源差异 | v0.0.4.1 已实现：双层读取/写入/删除，`--clear` 双层清理 |
| **Atom 提取引擎** | 从 highlight/note/image 标注自动提取文本片段、位置、页码 | Knowledge Graph P1 |
| **智能分类** | LLM 辅助标注角色分类（方法/结果/背景/讨论/定义） | Agent Runtime P0 |
| **笔记模板** | 结构化笔记卡片（问题-方法-结果-启示 四象限等） | Atom 引擎 |
| **跨条目关联** | 基于 content similarity + citation network 自动建议关联条目 | Knowledge Graph P2 |
| **写作导出** | 从 Note Cards 生成 Markdown/LaTeX 大纲或段落草稿 | 以上全部 |

### 与现有命令的关系

| 现有命令 | 在智能体笔记中的角色 |
|----------|---------------------|
| `extract-text` | 提供 PDF 正文上下文，供 Atom 补充周边内容 |
| `annotations` | Atom 的主要数据来源（DB + PDF 双源标注） |
| `annotate` | Agent 可通过此命令写入新的结构化标注 |
| `show` | 展示条目及其子笔记和标注的完整视图 |
| `relate` | 发现条目间已有关系，为跨条目关联提供基础 |
| `find` | 按主题/标签检索相关条目，支撑文献综述范围确定 |
| `export` | 导出 CSL-JSON/BibTeX，对接外部写作工具链 |

### 设计约束

- **不替代 Zotero 笔记编辑器** — Zotero 自身已有完善的笔记功能，本系统聚焦于 **从标注到写作** 的中间环节
- **人机协作优先** — Agent 提议、人类确认，不追求全自动
- **本地优先** — 所有数据存储在本地 SQLite，不依赖云服务
- **格式中立** — 输出 Markdown/LaTeX/纯文本，不绑定特定写作工具
