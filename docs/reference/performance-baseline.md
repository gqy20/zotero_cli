# Zotero CLI 性能基线

> 最近完整报告：2026-07-12，3 次迭代。本文只汇总已测数据；当前原始结果见 [`benchmarks/results/latest.json`](../../benchmarks/results/latest.json) 和 [`latest.md`](../../benchmarks/results/latest.md)。

## 当前测量口径

性能验证统一使用 `zot-bench`，不再把零散的手工 `time` 结果当作当前基线：

```powershell
go build -o zot.exe ./cmd/zot
go run ./cmd/zot-bench --binary .\zot.exe --mode all --tier default
```

- **Coverage timing**：对 canonical 叶子命令执行 `--help`，只衡量进程启动、Cobra 命令树构建和注册。
- **Runtime timing**：执行安全的真实只读场景，记录 cold、median、mean、p95 和输出字节数。
- **Net median**：用 `version` median 近似扣除固定启动成本；它是诊断值，不是严格的业务耗时剖析。
- **Configured/local comparison**：同一场景分别走配置模式和显式 `--mode local`，用于区分命令层、本地 SQLite 与网络后端成本。

需要真实条目或附件的 data tier：

```powershell
go run ./cmd/zot-bench --binary .\zot.exe --tier data `
  --var ITEM_KEY=XXXXXXXX `
  --var ATTACHMENT_KEY=YYYYYYYY
```

完整参数、安全边界和清单维护规则见 [CLI performance benchmark](./performance-benchmark.md)。

## 最近实测摘要

以下数字来自 2026-07-12 的 `benchmarks/results/latest.*`。当时 `version` median 为 **125.01 ms**，因此小于该值的 Net ms 会截断为 0。

| 场景 | Median | Net median | 数据源/说明 |
|---|---:|---:|---|
| `config check --json` | 1395.15 ms | 1270.14 ms | 显式在线校验，是当前主要网络热点 |
| `item find QUERY --in metadata --json` | 398.32 ms | 273.31 ms | configured，local SQLite live |
| `item find QUERY --in metadata --mode local --json` | 458.46 ms | 333.46 ms | 显式 local 对照 |
| `item list --limit 5 --json` | 315.73 ms | 190.72 ms | 已下推 LIMIT/OFFSET |
| `lib show --json` | 298.84 ms | 173.84 ms | 聚合读取 |
| `schema list types --json` | 215.85 ms | 90.84 ms | 持久缓存命中 |
| `note find REGEX --json` | 146.85 ms | 21.85 ms | 不区分大小写的 Go 正则 |
| `ref status --json` | 134.12 ms | 9.12 ms | 只读状态缓存命中 |

这些结果不能代表 2026-07-14 新增接口的最新性能；它们只是最后一次已经执行并保存的实测快照。更新代码后应重新运行 benchmark，再提交生成的 `latest.json` / `latest.md`，不要手工修改测量值。

## 当前必须覆盖的查询矩阵

2026-07-14 起，benchmark 场景应使用正式接口：

| 能力 | 基准命令 | 目的 |
|---|---|---|
| 元数据检索 | `item find QUERY --in metadata --limit N --json` | 默认本地条目查询 |
| 全文检索 | `item find 'term* OR "phrase"' --in fulltext --limit N --json` | 单独测 FTS5，不混入元数据路径 |
| 引用检索 | `ref find 'term* OR "phrase"' --in all --json` | 测引用、语境和元数据 FTS5 |
| 笔记检索 | `note find 'term|phrase\s+variant' --json` | 测 Go 正则过滤；不要使用 `note list --query` |
| 明确 key 导出 | `item export ITEMKEY --as bibtex` | 测 Zotero 导出，不包含选择阶段 |
| 选择后导出 | `item find ... --json` + `item export --from PATH|- --as bibtex` | 分开记录选择和导出，禁止在 export 中复制筛选器 |

全文与引用 QUERY 直接使用 SQLite FTS5 语法。笔记和 `pdf text --grep` 使用不区分大小写的 Go 正则，两者不能共享同一组表达式样本。

## 已确认的优化结论

- `item list --limit 5` 曾在加载全库关联数据后才分页；下推 `LIMIT/OFFSET` 后，历史 median 从约 571 ms 降至约 210–316 ms。
- `schema list` 使用 7 天持久缓存后，缓存命中从约 1.2 s 降至约 117–216 ms；`--refresh` 必须作为 extended 网络场景单独测量。
- `search list/show` 在 local/hybrid 读取本地 saved-search 表后，`search list` 历史 median 降至约 85–125 ms。
- `ref status` 不再隐式建表、迁移和同步 FTS；索引未变化时历史 median 降至约 75–134 ms。
- `item find` 的 8 位 Zotero key 使用精确索引快速路径；普通查询只为最终分页结果加载 creators、tags 和 attachments。

## 解读边界

- `--help` 快不代表业务命令快，也不能作为删除命令的依据。
- PDF cache miss、索引首次构建、远端 API 冷启动与缓存命中必须分开报告。
- `item export` 现在只负责导出明确 key；按收藏夹、日期或标签选择文献的耗时属于 `item find`，不能再计入一个隐藏的 export 查询阶段。
- `config check` 的网络耗时是其语义的一部分，不应为了 benchmark 数字改成离线检查。
- Windows 防病毒、Zotero 数据库大小、WAL 状态、网络距离和外部服务限流都会影响绝对数值；优化判断优先看同机、同数据、同参数的前后对照。
