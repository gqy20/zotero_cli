# Zotero CLI Web Frontend - Development Guide

> 临时开发文档，随项目演进持续更新
> Last updated: 2026-04-22

## 1. 技术栈

| 层 | 技术 | 版本 | 用途 |
|----|------|------|------|
| 后端 API | Go + chi | Go 1.26+ | REST API Server |
| 前端框架 | React | 19.x | UI 框架 |
| 构建工具 | Vite | 6.x | 开发/构建 |
| 语言 | TypeScript | 5.x | 类型安全 |
| 样式 | Tailwind CSS | 4.x | 原子化 CSS |
| UI 组件 | shadcn/ui | latest | 可定制组件库 |
| 数据请求 | TanStack Query | 5.x | 服务端状态管理 |
| 路由 | React Router | 7.x | 客户端路由 |
| PDF 预览 | pdf.js | 4.x | PDF 内联预览 |
| 测试 (Go) | 标准库 testing | - | Handler 单元测试 |
| 测试 (前端) | Vitest + Testing Library | latest | 组件测试 |

## 2. 架构概览

```
┌──────────────────────────────────────────────────────┐
│                   Browser                            │
│  React SPA (Vite dev server → proxy to :8080)        │
├──────────────────────────────────────────────────────┤
│              Go HTTP Server (:8080)                  │
│  internal/server/  → chi router → handlers           │
│                      → backend.Reader (已有)          │
│                      → go:embed web/dist/ (静态文件)   │
├──────────────────────────────────────────────────────┤
│              Data Layer (复用现有代码)                 │
│  HybridReader → Local SQLite / Web API               │
└──────────────────────────────────────────────────────┘
```

## 3. 项目目录结构

```
zotero_cli/
├── cmd/
│   ├── zot/main.go              # 现有 CLI（不变）
│   └── server/main.go           # 新增：Web 服务入口
├── internal/
│   ├── backend/                 # 现有（不变）
│   ├── cli/                     # 现有（不变）
│   ├── config/                  # 现有（不变）
│   ├── domain/                  # 现有（不变）
│   ├── server/                  # 新增：HTTP API 层
│   │   ├── server.go            # 路由注册、中间件、启动
│   │   ├── handlers.go          # 读操作 handlers
│   │   ├── write_handlers.go    # 写操作 handlers
│   │   ├── middleware.go        # CORS/日志/错误处理/恢复
│   │   └── responses.go         # 统一响应格式
│   └── zoteroapi/               # 现有（不变）
├── web/                         # 新增：React 前端项目
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   ├── components.json          # shadcn/ui 配置
│   ├── public/
│   └── src/
│       ├── main.tsx             # 入口
│       ├── App.tsx              # 路由 + Provider
│       ├── api/
│       │   ├── client.ts        # fetch 封装
│       │   ├── items.ts         # 文献 API
│       │   ├── collections.ts   # 分类 API
│       │   ├── tags.ts          # 标签 API
│       │   ├── annotations.ts   # 标注 API
│       │   ├── stats.ts         # 统计 API
│       │   └── export.ts        # 导出 API
│       ├── pages/
│       │   ├── Dashboard.tsx
│       │   ├── Library.tsx
│       │   ├── ItemDetail.tsx
│       │   ├── Search.tsx
│       │   ├── Tags.tsx
│       │   └── Export.tsx
│       ├── components/
│       │   ├── Layout.tsx
│       │   ├── Sidebar.tsx
│       │   ├── ItemCard.tsx
│       │   ├── ItemTable.tsx
│       │   ├── CollectionTree.tsx
│       │   ├── TagFilter.tsx
│       │   ├── SearchBar.tsx
│       │   ├── AnnotationPanel.tsx
│       │   ├── PdfViewer.tsx
│       │   ├── StatCard.tsx
│       │   ├── DateFilter.tsx
│       │   ├── Pagination.tsx
│       │   └── EmptyState.tsx
│       ├── hooks/
│       │   ├── useItems.ts
│       │   ├── useCollections.ts
│       │   └── useDebounce.ts
│       ├── types/
│       │   ├── item.ts
│       │   └── api.ts
│       ├── lib/
│       │   └── utils.ts
│       └── components/ui/      # shadcn/ui 生成组件
│           ├── button.tsx
│           ├── input.tsx
│           ├── card.tsx
│           ├── badge.tsx
│           ├── tabs.tsx
│           ├── dialog.tsx
│           ├── select.tsx
│           ├── tooltip.tsx
│           └── ...
├── docs/
│   └── DEV-FRONTEND.md         # 本文档
├── go.mod
└── README.md
```

## 4. REST API 设计

### 4.1 统一响应格式

```json
{
  "ok": true,
  "data": { ... },
  "error": null,
  "meta": {
    "read_source": "local",
    "total": 42
  }
}
```

### 4.2 API 端点列表

#### 读操作

| Method | Path | 对应命令 | 说明 |
|--------|------|----------|------|
| GET | `/api/v1/stats` | `stats` | 图书馆统计 |
| GET | `/api/v1/items` | `find` | 文献搜索（支持 query params） |
| GET | `/api/v1/items/:key` | `show` | 文献详情 |
| GET | `/api/v1/collections` | `collections` | 分类列表 |
| GET | `/api/v1/tags` | `tags` | 标签列表 |
| GET | `/api/v1/items/:key/annotations` | `annotations` | 文献标注 |
| GET | `/api/v1/notes` | `notes` | 笔记列表 |
| GET | `/api/v1/search` | `--fulltext` | 全文搜索 |
| GET | `/api/v1/overview` | `overview` | 总览（统计+最近文献） |

#### 写操作（受 ZOT_ALLOW_WRITE/ZOT_ALLOW_DELETE 保护）

| Method | Path | 对应命令 | 说明 |
|--------|------|----------|------|
| POST | `/api/v1/items` | `create-item` | 创建文献 |
| PUT | `/api/v1/items/:key` | `update-item` | 更新文献 |
| DELETE | `/api/v1/items/:key` | `delete-item` | 删除文献 |
| POST | `/api/v1/collections` | `create-collection` | 创建分类 |
| PUT | `/api/v1/collections/:key` | `update-collection` | 更新分类 |
| DELETE | `/api/v1/collections/:key` | `delete-collection` | 删除分类 |
| POST | `/api/v1/items/:key/tags` | `add-tag` | 添加标签 |
| DELETE | `/api/v1/items/:key/tags/:tag` | `remove-tag` | 移除标签 |

#### 文件服务

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/v1/files/:attachmentKey` | PDF/附件文件流式下载 |

### 4.3 关键 Query Params (GET /api/v1/items)

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| q | string | - | 搜索关键词 |
| item_type | string | - | 文献类型过滤 |
| tag | string | - | 标签过滤（单个） |
| tags | string | - | 标签过滤（逗号分隔，AND） |
| collection | string | - | 分类 key 过滤 |
| date_after | string | - | 日期范围起始 |
| date_before | string | - | 日期范围结束 |
| has_pdf | bool | false | 仅含 PDF |
| limit | int | 25 | 返回条数 |
| start | int | 0 | 偏移量（分页） |
| sort | string | dateAdded | 排序字段 |
| direction | string | desc | 排序方向 |
| full | bool | false | 返回完整字段 |

### 4.4 TypeScript 类型定义（与 Go domain types 对应）

```typescript
// types/item.ts
interface Creator {
  name: string
  creator_type: string
}

interface Attachment {
  key: string
  item_type: string
  title?: string
  content_type?: string
  link_mode?: string
  filename?: string
  zotero_path?: string
  resolved_path?: string
  resolved: boolean
}

interface Note {
  key: string
  parent_item_key?: string
  content?: string
  preview?: string
}

interface Annotation {
  key: string
  type: 'highlight' | 'note' | 'image' | 'ink'
  text?: string
  comment?: string
  color?: string
  page_label?: string
  page_index?: number
  position?: string
  sort_index?: string
  is_external: boolean
  date_added?: string
}

interface Collection {
  key: string
  name: string
  num_items?: number
}

interface Item {
  version?: number
  key: string
  item_type: string
  title: string
  date: string
  creators: Creator[]
  matched_on?: string[]
  full_text_preview?: string
  container?: string
  volume?: string
  issue?: string
  pages?: string
  doi?: string
  url?: string
  tags: string[]
  collections: Collection[]
  attachments: Attachment[]
  notes: Note[]
  annotations: Annotation[]
}

// types/api.ts
interface ApiResponse<T> {
  ok: boolean
  data: T
  error: string | null
  meta: ApiMeta
}

interface ApiMeta {
  read_source?: string
  total?: number
  sqlite_fallback?: boolean
}

interface PaginatedResponse<T> extends ApiResponse<T[]> {
  meta: ApiMeta & { total: number }
}
```

## 5. 开发规范

### 5.1 TDD 流程

```
1. 写失败测试 (Red)
2. 写最少代码让测试通过 (Green)
3. 重构 (Refactor)
4. Commit
```

**Go 后端 TDD：**
- 每个 handler 先写 `xxx_test.go`
- 使用 `net/http/httptest` 构造请求
- 验证 HTTP status code + JSON body

**前端 TDD：**
- 每个组件先写 `*.test.tsx`
- 使用 Vitest + @testing-library/react
- 验证渲染结果 + 用户交互行为

### 5.2 Git 提交规范

使用 conventional commits：
- `feat(server): add /api/v1/stats endpoint`
- `feat(frontend): add Dashboard page with stat cards`
- `fix(server): handle empty collection key gracefully`
- `test(frontend): add ItemCard component tests`
- `chore(frontend): configure Tailwind + shadcn/ui`

### 5.3 分支策略

```
master (main)
  └── feat/web-frontend (功能分支)
       ├── feat/server-base        # Go server 骨架
       ├── feat/api-handlers       # API handlers
       ├── feat/frontend-init      # React 项目初始化
       ├── feat/pages-core         # 核心页面
       └── feat/pdf-preview        # PDF 预览
```

### 5.4 环境变量

```bash
# Server
ZOT_SERVER_PORT=8080          # 监听端口
ZOT_SERVER_HOST=localhost     # 监听地址
ZOT_SERVER_CORS_ORIGIN=*      # CORS 允许来源（开发时用 *）

# 复用现有配置
ZOT_MODE=hybrid               # 运行模式
ZOT_DATA_DIR=...              # 本地数据目录
ZOT_API_KEY=...               # Web API 密钥
ZOT_LIBRARY_ID=...            # 图书馆 ID
```

## 6. 页面设计规格

### 6.1 Dashboard（仪表盘）

```
┌─────────────────────────────────────────────────────┐
│  Zotero Library                    [mode: hybrid]   │
├────────┬────────────────────────────────────────────┤
│        │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐     │
│  导航   │  │ 1,234│ │  45  │ │ 320  │ │  12  │     │
│        │  │文献数│ │分类数│ │标签数│ │搜索数│     │
│  📊 总览│  └──────┘ └──────┘ └──────┘ └──────┘     │
│  📚 文献│                                            │
│  🔍 搜索│  最近添加                                    │
│  🏷️ 标签│  ┌────────────────────────────────────┐   │
│  📤 导出│  │ [1] Deep Learning for NLP (2024)   │   │
│        │  │ [2] Transformer Architecture (2023)  │   │
│        │  │ [3] Attention Is All You Need (2022) │   │
│        │  └────────────────────────────────────┘   │
└────────┴────────────────────────────────────────────┘
```

### 6.2 Library（文献库主页）

```
┌─────────────────────────────────────────────────────┐
│  [🔍 搜索...] [类型▼] [标签▼] [日期▼] [排序▼]      │
├──────────────┬──────────────────────────────────────┤
│  分类树       │  共 1,234 条                          │
│              │                                      │
│  ▶ 我的文库  │  ☐ Title                        Authors │
│    ▶ 论文    │  ☐ Deep Learning...        Zhang et al│
│    ▶ 书籍    │  ☐ Transformer Arch...      Vaswani   │
│    ▶ 会议    │  ☐ Attention Is All Need   Vaswani   │
│  ▶ 已标星    │  ...                                  │
│  🗑️ 回收站   │                                      │
│              │  ← 1 2 3 ... 50 →  每页 [25] ▼        │
└──────────────┴──────────────────────────────────────┘
```

### 6.3 ItemDetail（文献详情）

```
┌─────────────────────────────────────────────────────┐
│  ← 返回    Deep Learning for NLP                    │
├─────────────────────────────────────────────────────┤
│  [元数据] [附件] [标注] [笔记] [引用]                │
│                                                      │
│  Type: journalArticle                               │
│  Authors: Zhang, Li, Wang                           │
│  Container: Nature Machine Intelligence             │
│  Volume: 6  Issue: 1  Pages: 1-25                   │
│  Date: 2024-03                                      │
│  DOI: 10.1038/s42256-024-00846-9                    │
│  Tags: [NLP] [Deep Learning] [Transformer]          │
│  Collections: [论文 → 深度学习]                       │
│                                                      │
│  Attachments:                                       │
│  📄 paper.pdf  [预览] [打开]                         │
│                                                      │
│  Annotations (12):                                   │
│  🔶 p.5 "attention mechanism is key..."             │
│  💛 p.12 "transformer achieves SOTA..."             │
│  📝 p.8 "this suggests that..."                     │
└─────────────────────────────────────────────────────┘
```

## 7. 开发命令速查

```bash
# --- 后端 ---
# 运行 server
go run ./cmd/server

# 运行 server 测试
go test ./internal/server/... -v

# --- 前端 ---
cd web
npm install              # 安装依赖
npm run dev              # 启动开发服务器 (默认 :5173)
npm run build            # 构建生产版本 → dist/
npm run test             # 运行 Vitest
npm run test:ui          # 运行 Vitest UI
npm run lint             # ESLint 检查

# shadcn/ui 添加组件
npx shadcn@latest add button card badge tabs dialog

# --- 全流程 ---
# Terminal 1: 启动后端
go run ./cmd/server

# Terminal 2: 启动前端（自动 proxy API 到后端）
cd web && npm run dev
```

## 8. 依赖清单

### Go 新增依赖

```
github.com/go-chi/chi/v5  # HTTP 路由
```

### 前端 package.json 核心依赖

```json
{
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.0.0",
    "@tanstack/react-query": "^5.0.0",
    "pdfjs-dist": "^4.0.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.0.0",
    "lucide-react": "^0.400.0",
    "@radix-ui/react-dialog": "^1.1.0",
    "@radix-ui/react-tabs": "^1.1.0",
    "@radix-ui/react-select": "^2.1.0",
    "@radix-ui/react-tooltip": "^1.1.0"
  },
  "devDependencies": {
    "vite": "^6.0.0",
    "typescript": "^5.5.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.0.0",
    "tailwindcss": "^4.0.0",
    "@tailwindcss/vite": "^4.0.0",
    "vitest": "^2.0.0",
    "@testing-library/react": "^16.0.0",
    "@testing-library/jest-dom": "^6.0.0",
    "jsdom": "^25.0.0"
  }
}
```

## 9. 实施路线图

### Phase 1: 基础设施（Day 1）
- [x] 编写开发文档
- [ ] Go server 骨架（chi 路由 + 中间件 + embed）
- [ ] React 项目初始化（Vite + TS + Tailwind + shadcn/ui）
- [ ] Layout + Router + API client 基础层

### Phase 2: 核心页面（Day 2-4）
- [ ] Dashboard 页面
- [ ] Library 页面（分类树 + 文献列表 + 筛选 + 分页）
- [ ] ItemDetail 页面（Tab 切换）

### Phase 3: 功能完善（Day 5-6）
- [ ] Search 全文搜索页面
- [ ] Tags 标签管理页面
- [ ] Export 导出中心
- [ ] AnnotationPanel 标注面板

### Phase 4: 打磨（Day 7+）
- [ ] pdf.js PDF 预览集成
- [ ] go:embed 单二进制打包
- [ ] 错误边界 + Loading 状态优化
- [ ] 响应式适配
