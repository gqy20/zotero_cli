# Zotero CLI 性能基线报告

> 测试时间: 2026-04-21 | 模式: hybrid (local SQLite + Web fallback) | 二进制: v0.0.5-10-g6a27af5

## 基线数据

### 读命令（按耗时排序）

| 排名 | 命令 | 耗时 | 瓶路 |
|------|------|------|------|
| 1 | `export KEY --format bibtex` | **19.1s** | Web API (含 items + collections 多轮) |
| 2 | `show ITEMKEY` | **7.7s** | Reader → hybrid (本地+Web) |
| 3 | `overview` / `overview --json` | **6.3~6.4s** | **4 路并行，全部本地 SQLite** |
| 4 | `find "" --full --limit 5` | **6.3s** | Reader → hybrid |
| 5 | `find ""` | **6.2s** | Reader → hybrid |
| 6 | `annotations ITEMKEY` | **6.0s** | Reader → local (PyMuPDF) |
| 7 | `relate ITEMKEY` | **6.0s** | Reader → hybrid |
| 8 | `tags` | **6.0s** | Reader → local SQLite |
| 9 | `notes` | **6.0s** | Reader → local SQLite |
| 10 | `stats` | **6.0s** | Reader → hybrid (versions×3) |
| 11 | `versions items-top --since 0` | **5.8s** | Web API (versions + collections + searches) |
| 12 | `collections` | **3.3s** | **Web API** (Reader 无此方法前) |
| 13 | `cite ITEMKEY` | **4.9s** | Web API |
| 14 | `deleted` | **1.7s** | Web API |
| 15 | `schema types` | **1.6s** | Web API |

### 写命令（未测，通常 <2s）

| 命令 | 预估 | 链路 |
|------|------|------|
| `create-item` | ~2s | Web API POST |
| `update-item` | ~2s | Web API PATCH |
| `delete-item` | ~1s | Web API DELETE |
| `add-tag` / `remove-tag` | ~2s | Web API PATCH |
| `create-collection` | ~2s | Web API POST |
| `annotate` | ~1s | local PyMuPDF |

## 分析与优化方向

### P0 — 可直接优化的瓶颈

#### 1. `export` (19.1s) — 最大单点
- 当前：先 `find` 拿 key 列表，再逐条调 Web API 导出
- 方案：`--from-find` 内部管道，或批量导出 API

#### 2. `show` (7.7s) — 慢于预期
- 当前：hybrid 下可能走了 Web fallback 或多次查询
- 方案：检查是否命中本地缓存路径，减少冗余 API 调用

#### 3. `collections` (3.3s) — 已走 Web API
- **已完成优化**：刚加入 Reader 接口的 ListCollections，下次测试应降至 ~6s 级别（与其他本地调用对齐）

### P1 — 架构级优化

#### 4. 全局 HTTP 连接复用
- 每次 `time ./zot.exe` 都重新建立 TCP 连接（TLS 握手 ~200ms）
- 如果 agent 场景连续调用多个命令，连接复用可省 1-2s/次
- 方案：长运行模式 / daemon 模式 / 连接池

#### 5. stats 内部去重 (6s)
- `GetLibraryStats` 内部调了 3 次 Web API：items versions + collections versions + searches versions
- overview 并行调 stats 时，这 3 次是串行的
- 方案：stats 内部也并行化，或 overview 复用 stats 的中间结果

#### 6. FTS 全文检索 (find --fulltext ~6s)
- 本地 FTS5 索引查询本身很快 (~50ms)，但 snippet 缓存未命中时要扫描 PDF
- 方案：预热策略、后台索引构建通知

### P2 — 缓存层

#### 7. 内存缓存热门结果
- `stats` / `collections` / `tags` 变化频率低（分钟级），适合短时缓存
- TTL 60s 可覆盖大多数 agent 连续调用场景
- 方案：Reader 层加可选内存缓存，`ZOT_CACHE_TTL=0` 关闭

#### 8. 概览预聚合
- overview 是 agent 最可能的第一个调用，首屏体验关键
- 方案：后台定时任务（如每 5min）预计算结果，overview 直接返回缓存

## 测量方法

```bash
# 单命令计时
time ./zot.exe overview

# 对比优化前后
time ./zot.exe overview    # 优化前
time ./zot.exe overview    # 优化后

# JSON vs 文本差异
time ./zot.exe overview       # 文本格式化开销
time ./zot.exe overview --json # JSON 序列化开销
```

> 注：所有耗时包含进程启动 + 配置加载 + TLS 连接（~300ms 固定开销）。
> Agent 场景下若使用 daemon 模式可消除此开销。
