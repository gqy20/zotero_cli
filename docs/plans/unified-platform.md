# 平台统一化 + 手柄集成方案

> 三层（CLI / REST API / Web 前端）功能覆盖度分析、补齐路线图、代码量估算、以及 Gamepad 手柄交互增强设计。

---

## 一、现状总览

### 1.1 各层代码基线

| 层 | 文件数 | 总行数 | 职责 |
|---|--------|--------|------|
| CLI (`internal/cli/`) | ~30 | 14,541 | 功能完备的核心引擎 |
| Backend (`internal/backend/`) | ~20 | 11,110 | 数据层抽象（Local/Web/Hybrid/Remote） |
| Server (`internal/server/`) | 8 | 1,471 | 只读 HTTP API（12 端点） |
| Web 前端 (`web/src/`) | ~25 | 4,092 | React SPA（6 页面） |
| Domain / Config | 3 | 499 | 类型定义与配置 |
| **合计** | **~86** | **~31,213** | |

### 1.2 功能覆盖热力图

```
功能域              CLI      API      Web      统一?
─────────────────────────────────────────────────────
文献搜索           ████████ ████████ ██       ❌
条目详情           ████████ ██████   █████    ⚠️
PDF 预览           ████     ███      ████     ⚠️
导出引用           ████████ ❌无     🔴空壳   ❌
标签管理           ██████   ███(只读) ██(只看)  ❌
关联网络           ██████   █(只读)  ❌无     ❌
图表提取           █████    ███      ❌无     ❌
标注 CRUD          █████    ❌无     ❌无     ❌
写操作 CRUD        ████████ ❌无     ❌无     ❌
全文检索           █████    ███      ██       ⚠️
统计概览           █████    ████     ███      ✅
收藏集导航         ████     ███      ███      ⚠️
```

### 1.3 关键数字

| 指标 | 数值 |
|------|------|
| CLI 命令总数 | 44 个 |
| 有 API 对应的 CLI 命令 | ~10 个（仅 23%） |
| API 端点总数 | 12 个 |
| Web UI 实际使用的 API | 7 个（58%，5 个死代码） |
| 写操作端点 | 0 个（Server 纯只读） |
| Web 页面功能完整 | 2/6（Dashboard、Library 基本可用） |

---

## 二、六大核心问题

### 问题 1：导出功能完全空壳

`Export.tsx` 有完整 UI（格式选择 + 条目勾选），但**导出按钮没有 onClick handler**。同时服务端**根本没有 export 端点**。

```tsx
// Export.tsx:146-151 — 按钮是装饰品
<button disabled={selectedItems.size === 0} className="...">
  导出为 {selectedFormat}   // ← 点击无反应
</button>
```

### 问题 2：写操作全线缺失

15 个写操作全部只有 CLI，API 和 Web 都没有：

| 操作 | CLI | API | Web |
|------|-----|-----|-----|
| create/update/delete item | ✅ | ❌ | ❌ |
| add/remove tag | ✅ | ❌ | ❌ |
| create/update/delete collection | ✅ | ❌ | ❌ |
| create/update/delete search | ✅ | ❌ | ❌ |
| relate --add/--remove | ✅ | ❌ | ❌ |
| annotate PDF | ✅ | ❌ | ❌ |

Server 是一个**纯只读 HTTP 接口**。

### 问题 3：Library 搜索框是摆设

```tsx
// Library.tsx:100 — SearchInput 渲染了但没接入任何状态
<SearchInput placeholder="搜索文献..." />
// 无 value, 无 onChange, 无 state, 不触发 API 调用
```

API 的 `GET /api/v1/items` 支持 **30+ 查询参数**，Web 只传了 `start` + `limit` + `collection` 三个。

### 问题 4：已实现的 API 端点未被 Web 使用

| 已有端点 | 状态 |
|----------|------|
| `GET /items/{key}/related` | ItemDetail 未调用，无关联展示区 |
| `GET /items/{key}/figures` | 无图表画廊/查看器 UI |
| `GET /notes` | client.ts 定义了方法但没有页面调用 |
| `GET /stats` | Dashboard 用 overview() 代替，stats() 是死代码 |

### 问题 5：数据层不一致

| 差异点 | CLI | API | Web |
|--------|-----|-----|-----|
| 期刊排名 (SCI-IF/JCI) | show 调用 enrichWithJournalRank() | handler 未调用 | 无法显示 |
| Collections 字段 | 返回 num_children（支持层级树） | 只返回 num_items | 扁平列表 |
| Overview 响应 | 含 collections/tags/index_status/meta | 只有 stats + recent_items | 只用了 stats + recent_items |
| Notes 过滤 | 支持 --query 参数 | 无查询参数 | 无 Notes 专用页 |

### 问题 6：UI 完成度缺陷

| 页面 | 问题 |
|------|------|
| Dashboard | 标签数硬编码 `"-"`；footer 条目数硬编码；无刷新按钮 |
| ItemDetail | 不显示 abstract/date_added/version；Notes 用 dangerouslySetInnerHTML（XSS）；无相关文献区块 |
| Search | 无错误处理；未使用已有的 useDebounce hook；无高级搜索选项 |
| Tags | 点击标签无响应（cursor-pointer 但无 onClick）；纯只读 |
| PdfViewer | 缩放时重新加载整个 PDF；无键盘快捷键；无文字选择层 |
| 全局 | Toast 系统完整实现但**零个页面使用**；无 404 页面 |

---

## 三、补齐路线图

### Phase 1：让现有功能真正能用（~180 行净增）

目标：所有已渲染的 UI 元素都能正常工作。

| # | 改动 | 文件 | 行数 | 说明 |
|---|------|------|------|------|
| 1 | 导出 API 端点 | `server/handlers.go` | +40 | `POST /api/v1/export`，复用 CLI export 逻辑 |
| 2 | 导出 client 方法 | `api/client.ts` | +8 | `export(format, keys)` → 触发文件下载 |
| 3 | Export.tsx 接通按钮 | `pages/Export.tsx` | +15 | onClick → api.export() |
| 4 | Library 搜索接入 | `pages/Library.tsx` | +30 | searchQuery state + useDebounce + 传 q 参数 |
| 5 | Tags 点击跳转 | `pages/Tags.tsx` | +10 | onClick → navigate(`/library?tag=xxx`) |
| 6 | Dashboard 标签数 | `handlers.go` + `Dashboard.tsx` | +15 | stats 加 total_tags 字段 |
| 7 | ItemDetail 补 abstract | `pages/ItemDetail.tsx` | +8 | 加一个 MetaRow |
| 8 | Layout footer 动态化 | `components/Layout.tsx` | +5 | 从 overview 获取条目数 |

**产出**：导出能用了、搜索能用了、标签能点了、数据不硬编码了。

### Phase 2：接通已有但未用的 API（~600 行）

目标：功能覆盖度 > 80%。

| # | 改动 | 文件 | 行数 | 说明 |
|---|------|------|------|------|
| 9 | Related Items 区块 | `ItemDetail.tsx` | +80 | 调 `/related` + 渲染关联列表 |
| 10 | Figure Gallery 组件 | 新建 `FigureGallery.tsx` | +120 | 图片网格预览 + 点击放大 modal |
| 11 | ItemDetail 内嵌图表区 | `ItemDetail.tsx` | +25 | "图表" Section + figures API |
| 12 | Journal Rank 增强 | `server/handlers.go` getItem | +10 | handler 内调 enrichWithJournalRank() |
| 13 | ItemDetail 显示期刊信息 | `ItemDetail.tsx` | +35 | SCI-IF/JCI/分区 badge |
| 14 | Search 高级过滤面板 | 新建 `FilterPanel.tsx` | +150 | 标签/日期/类型/排序 |
| 15 | Library 排序控制 | `Library.tsx` toolbar | +40 | sort dropdown + direction toggle |
| 16 | Search 错误处理 | `Search.tsx` | +15 | ErrorFallback |
| 17 | Tags 分页/搜索 | `Tags.tsx` | +45 | 分页 + 本地过滤 |
| 18 | PdfViewer 性能优化 | `PdfViewer.tsx` | +30 | 缓存 pdfjs document，zoom 不重载 |
| 19 | client 补全方法 | `api/client.ts` | +20 | related(), figures() |
| 20 | 404 页面 | 新建 `NotFound.tsx` + `App.tsx` | +30 | catch-all route |

**产出**：关联文献可看了、图表可浏览了、搜索有过滤了、期刊排名显示了。

### Phase 3：写操作 API 化（~900 行）

目标：完整 CRUD 闭环。需要 Server + Client + UI 三层同时加。

#### 3A. Server 写端点（~300 行）

| 端点 | 行数 | 说明 |
|------|------|------|
| `POST /api/v1/items` | ~40 | create-item |
| `PUT /api/v1/items/{key}` | ~40 | update-item |
| `DELETE /api/v1/items/{key}` | ~30 | delete-item + 确认检查 |
| `POST /api/v1/items/{key}/tags` | ~25 | add-tag |
| `DELETE /api/v1/items/{key}/tags/{tag}` | ~20 | remove-tag |
| `POST /api/v1/collections` | ~30 | create-collection |
| `PUT /api/v1/collections/{key}` | ~25 | update-collection |
| `DELETE /api/v1/collections/{key}` | ~20 | delete-collection |
| `POST /api/v1/items/{key}/relations` | ~25 | relate --add |
| `DELETE /api/v1/items/{key}/relations` | ~20 | relate --remove |
| 共用校验 middleware | ~25 | JSON body parse + validate + auth check |

参考值：现有每个 read handler 平均 15-25 行，write handler 需额外 body parsing + auth + version conflict 处理，平均 30-40 行。

#### 3B. Client 方法（~40 行）

`api/client.ts` 增加 POST/PUT/DELETE 封装 + 各写操作方法。

#### 3C. Web UI 编辑界面（~560 行）

| 组件 | 行数 | 说明 |
|------|------|------|
| `ItemEditForm.tsx` (新建) | ~180 | 标题/作者/容器/日期/DOI 编辑表单 |
| `CollectionForm.tsx` (新建) | ~60 | 收藏集名称/父级选择 |
| `TagManager.tsx` (新建) | ~80 | 条目标签添加/删除弹窗 |
| `ConfirmDialog.tsx` (新建) | ~40 | 删除确认对话框 |
| ItemDetail 加编辑入口 | ~20 | 编辑/删除按钮 |
| Library 加新建入口 | ~15 | "新建条目" 按钮 |
| Toast 通知接入 | ~30 | 各页面写操作后 useToast() |
| Tags 管理功能 | ~80 | 重命名/合并/删除 |
| 路由配置 | ~15 | `/items/:key/edit` route |

**产出**：Web 可以增删改查了，不再是个只读查看器。

### Phase 4：统一数据层（~250 行）

目标：三层响应格式一致，CLI 特有能力透传到 API。

| # | 改动 | 行数 | 说明 |
|---|------|------|------|
| Collections 返回 num_children | ~20 | server + local_db SELECT 加子集计数 |
| Overview 补全 collections/tags/meta | ~30 | handlers.go 合并多 query 到单响应 |
| Response wrapper 统一 | ~15 | meta.total 等字段一致 |
| Notes API 加 query 参数 | ~40 | handlers.go + local: `?q=xxx` 过滤 |
| Search/Trash/Changes/Schema 端点 | ~120 | 4 个新的只读端点 |
| Layout 集成 settings 入口 | ~25 | 账号信息/状态显示 |

**产出**：API 返回的数据和 CLI `--json` 输出对齐，Web 能展示完整信息。

---

## 四、手柄（Gamepad）集成设计

### 4.1 为什么是手柄？

zotero-cli 的 Web 前端已有 PDF 阅读器和 6 个页面，具备"阅读终端"的基础形态。手柄可以将它升级为一个**沉浸式论文消费终端**——类似 Steam 大屏幕模式或主机媒体中心体验，适合躺椅/沙发上/投影场景下读论文、翻图表、做标注。

技术基础：浏览器原生 [Gamepad API](https://developer.mozilla.org/en-US/docs/Web/API/Gamepad_API)，零依赖，纯前端增量。

### 4.2 核心场景设计

#### 场景 A：PDF 沉浸式阅读（最自然 fit）

当前 `PdfViewer.tsx` 已有 `goNext/goPrev/zoomIn/zoomOut/fitWidth` 方法：

```
┌──────────────────────────────────────────────────┐
│  🎮 Xbox/PS 手柄映射                              │
├──────────────────────────────────────────────────┤
│  D-Pad ↑↓     → 翻页（上一页/下一页）               │
│  LT/RT 扳机    → 缩放（放大/缩小）                  │
│  A/× 确认      → 适合宽度                          │
│  B/○ 返回      → 关闭预览 / 返回列表                │
│  LB/RB 肩键    → 切换文献（上一篇/下一篇）            │
│  Left Stick   → 平移 PDF 视图                     │
│  Start/Options→ 切换全屏阅读模式                   │
│  Select/Touch → 切换标注显示                       │
└──────────────────────────────────────────────────┘
```

#### 场景 B：文献快速浏览 + 标注工作流

```
Y/△        → 快速添加标签（弹出标签选择轮盘）
X/□        → 高亮当前段落（调用 annotate API）
View/Back  → 打开侧边栏（摘要/元数据/引用）
D-Pad ←/→  → 在搜索结果中快速切换
Long A      → 收藏/星标该文献
```

把手柄变成一个**"论文遥控器"**——不需要键盘鼠标就能完成 读→标→存 的核心循环。

#### 场景 C：图表提取幻灯片模式

项目已有 `extract-figures` 功能和图表缓存：

```
RB/LB     → 切换图表（Figure 1, Figure 2, ...）
A         → 放大查看细节
B         → 返回缩略图网格视图
D-Pad     → 在大图上平移（类似游戏地图查看）
LT+D-Pad  → 精细移动
Start     → 全屏展示（适合投屏/演示）
```

**使用场景**：组会汇报时，用手柄像翻 PPT 一样浏览论文图表。

#### 场景 D：多模态快捷操作面板

利用手柄组合键能力：

```
LT + A → 导出当前条目为 BibTeX
LT + B → 复制引用到剪贴板
LT + X → 在浏览器中打开 DOI
LT + Y → 创建关联笔记
RT + A → 搜索相似文献
RT + B → 查看引用网络
RT + X → 提取全文
RT + Y → 切换收藏状态
```

### 4.3 架构设计

```
web/src/
├── hooks/
│   └── useGamepad.ts              ← 新建：Gamepad API 轮询 + 按键映射 + 事件分发
├── components/
│   ├── PdfViewer.tsx              ← 修改：绑定手柄输入
│   ├── GamepadOverlay.tsx         ← 新建：全屏"主机阅读模式"UI
│   ├── FigureSlideshow.tsx        ← 新建：图表幻灯片模式
│   ├── GamepadQuickMenu.tsx       ← 新建：组合键快捷菜单
│   └── GamepadSettings.tsx        ← 新建：按键映射设置
└── pages/
    └── ReaderMode.tsx             ← 可选：独立的手柄优先阅读页面
```

核心是 `useGamepad()` hook：

```typescript
// 设计接口概念
interface GamepadConfig {
  mapping: GamepadMapping           // 按键 → action 映射
  deadzone: number                  // 摇杆死区
  vibration: boolean                // 触觉反馈
}

interface GamepadAction {
  type: 'press' | 'hold' | 'combo'
  button: ButtonCode               // 标准化的按键标识
  modifier?: ButtonCode[]          // 组合键修饰键
  handler: () => void              // 回调
}

function useGamepad(config?: GamepadConfig): {
  isConnected: boolean
  currentAction: GamepadAction | null
  bind: (action: string, handler: () => void) => void
  unbind: (action: string) => void
}
```

### 4.4 手柄功能代码量估算

| 模块 | 行数 | 说明 |
|------|------|------|
| `useGamepad()` hook | ~80 | 轮询 Gamepad API + 按键映射 + 事件分发 |
| PdfViewer 手柄绑定 | ~40 | D-Pad 翻页、扳机缩放、A 适合宽度 |
| 全屏阅读模式 overlay | ~200 | 大字体 UI、手柄导航焦点系统、页面切换 |
| 图表幻灯片模式 | ~100 | RB/LB 切换图、D-Pad 平移 |
| 手柄快捷操作面板 | ~120 | 组合键 → 导出/标注/收藏等操作 |
| 手柄设置/映射页面 | ~150 | 自定义按键映射的设置 UI |
| **小计** | **~690** | **全部在 web/src/ 内，零后端改动** |

### 4.5 实施优先级

| 优先级 | 场景 | 工作量 | 价值 |
|--------|------|--------|------|
| **P0** | Web PdfViewer 手柄支持 | ~120 行 | 直接提升阅读体验 |
| **P1** | 全屏"主机阅读模式" UI | ~200 行 | 差异化卖点 |
| **P2** | 图表幻灯片模式 | ~100 行 | 组会场景实用 |
| **P3** | 标注工作流手柄化 | ~160 行 | 提升效率 |
| **P4** | CLI TUI 阅读器 | ~1000+ 行 | 高但依赖多（需引入 TUI 框架） |

**推荐起步**：从 P0 开始，在现有 PdfViewer 上叠加手柄输入，验证 Gamepad API 在目标浏览器上的兼容性后，再扩展到全屏模式和更多场景。

---

## 五、总汇总

### 代码量一览

```
┌─────────────────────┬──────────┬────────┬──────────────────────────┐
│ Phase               │ 新增代码 │ 修改   │ 核心产出                 │
├─────────────────────┼──────────┼────────┼──────────────────────────┤
│ Phase 1: 功能打通    │  ~130行  │  ~50行  │ 所有 UI 能跑通            │
│ Phase 2: API 接通    │  ~600行  │  ~80行  │ 覆盖度 > 80%             │
│ Phase 3: 写操作闭环  │  ~900行  │  ~30行  │ 完整 CRUD                │
│ Phase 4: 数据统一    │  ~250行  │  ~40行  │ 三层格式对齐              │
├─────────────────────┼──────────┼────────┼──────────────────────────┤
│ 平台统一小计         │ ~1,880行 │ ~200行  │                          │
├─────────────────────┼──────────┼────────┼──────────────────────────┤
│ 手柄功能增量         │  ~690行  │  ~80行  │ 仅前端                    │
├─────────────────────┼──────────┼────────┼──────────────────────────┤
│ 合计                 │ ~2,570行 │ ~280行  │                          │
└─────────────────────┴──────────┴────────┴──────────────────────────┘
```

### 占比变化

```
Web 前端:     4,092 行 → ~6,800 行  (+66%)
Server:       1,471 行 → ~2,700 行  (+83%)
全项目:      31,200 行 → ~34,000 行  (+9%)
工作量估计:   3-5 天（单人全职）
```

### 与现有 Roadmap 的关系

本方案与 [`roadmap.md`](./roadmap.md) 的对应关系：

| 本方案 Phase | 对应 Roadmap 章节 | 关系 |
|-------------|-------------------|------|
| Phase 1-2 | B: Web 前端完善 (Phase 1-2) | **细化并扩展**：roadmap 只提到"写操作 UI 待后端路由"，本方案给出了完整的端点清单和代码量 |
| Phase 3 | B: Web 前端完善 (Phase 2) | **直接承接**：roadmap 明确说"⏸ 后端路由待注册"，本方案 Phase 3A 就是这部分 |
| Phase 4 | A: Zotero 原生能力深化 | **补充**：roadmap 侧重性能优化，本方案侧重数据一致性 |
| 手柄功能 | roadmapped 中无 | **新增方向**：差异化创新点 |

### 建议执行顺序

```
Week 1:
  ├─ Day 1-2:  Phase 1（立竿见影，快速建立信心）
  └─ Day 3-5:  Phase 2 + 手柄 P0（并行，前后端不冲突）

Week 2:
  ├─ Day 1-3:  Phase 3A+B（Server 写端点 + Client 封装）
  └─ Day 4-5:  Phase 3C（Web 编辑 UI）+ 手柄 P1

Week 3（可选）:
  ├─ Phase 4（数据层统一）
  └─ 手柄 P2-P3（图表幻灯片 + 标注工作流）
```
