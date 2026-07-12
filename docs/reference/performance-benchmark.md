# CLI performance benchmark

CLI 性能现在由可重复执行的框架衡量，而不是继续增加手工计时样例。框架严格分开两类数据：

- **Coverage timing**：每个 canonical 叶子命令执行 `--help`，检测进程启动、Cobra 构建和命令注册回归；它不是 Zotero 业务操作耗时。
- **Runtime timing**：重复执行安全的真实场景，记录首轮、median、mean 和 p95；缺少样本 key 时明确标记 skipped。

## 运行

先独立构建，避免把编译时间混入结果：

```powershell
go build -o zot.exe ./cmd/zot
go run ./cmd/zot-bench --binary .\zot.exe --mode all --tier default
```

提供代表性数据后启用 data tier：

```powershell
go run ./cmd/zot-bench --binary .\zot.exe --tier data `
  --var ITEM_KEY=XXXXXXXX `
  --var ATTACHMENT_KEY=YYYYYYYY
```

结果写入 `benchmarks/results/latest.json` 和 `benchmarks/results/latest.md`。可调整 `--iterations`、`--warmup`、`--timeout`、`--mode`，用 `--case item-list` 聚焦一个场景，并重复传入 `--var KEY=VALUE`。

报告中的 `Net ms` 用 `version` 的 median 近似扣除固定进程启动和命令构建成本。`Source` 来自命令响应的 `meta.read_source`。清单中的 comparison group 会自动比较 configured backend 与显式 local backend，帮助判断慢点来自命令层还是远端数据源。

## 安全边界

自动执行仅允许只读场景，以及声明为 `dry-run` 且参数确实包含 `--dry-run` 的写场景。配置写入、删除、外部程序打开、常驻 server、同步和索引重建默认只做 coverage。

## 命令存在必要性审计

`benchmarks/cli/manifest.json` 同时是完整命令目录。每个叶子命令记录能力、当前保留判断、语义重叠和可能的替代方案。删除命令前需要同时回答：

1. 是否提供唯一能力？
2. 替代命令能否用自然参数得到相同结果，而不是堆叠复杂 flags？
3. 两种方案的延迟、输出大小和后端访问次数如何？
4. 使用频率是否足以抵消额外的学习成本？

速度本身不是删除依据。例如 `lib show`、`ref profile` 虽可由多个原子命令拼合，但一次聚合调用可能更快、更稳定。相反，如果 `item find` 和 `note find` 获得清晰的“列出全部”语义，`item list`、`note list` 就值得进入合并评估。

新增叶子命令时必须加入 `commands`；能安全执行时再增加 runtime scenario。数据参数使用 `{{ITEM_KEY}}` 形式并列入 `requires`，不能用 help 耗时替代被跳过的真实场景。

## 2026-07-12 首轮发现

- 固定进程启动与 Cobra 构建通常约为 50–90 ms。
- `item list --limit 5` 曾在 SQL 查询和关联数据加载完成后才分页，实际处理全库。安全下推 `LIMIT/OFFSET` 后，median 从约 571 ms 降至约 210 ms；`lib show` 因复用相同路径，从约 624 ms 降至约 232–277 ms。
- configured hybrid 与显式 local 的 item/note 对照接近，说明这些命令的瓶颈位于本地 SQLite 路径，而不是 hybrid 路由。
- `group list` 曾先请求 key-info，再请求 groups。user library 已有 `library_id`，复用后 median 从约 1564 ms 降至约 1262 ms。剩余部分主要是一次网络往返。
- `schema list` 已加入 7 天持久缓存、`--refresh` 和 stale fallback；本机缓存命中 median 从约 1243 ms 降至约 117 ms，净业务耗时约 51 ms。extended tier 保留 `schema-types-refresh` 场景用于监测真实网络性能。
- `search list/show` 已在 local/hybrid 模式下读取本地 `savedSearches` 与 `savedSearchConditions`；本机 `search list` median 从约 1232 ms 降至约 85 ms，净业务耗时约 16 ms。Web 模式保持 API 行为，本地 schema 不兼容时 hybrid 明确回退 Web。
- `ref status` 已改为只读打开，不再在每次状态查询时执行建表、迁移、全表 UPDATE 和 FTS 同步。精确统计以 DB/WAL 文件指纹缓存；索引未变化时 median 从约 378 ms 降至约 75 ms，首次失效重算约 283 ms。旧 schema 会回退一次完整迁移，缺失索引只报告 `initialized=false`，不再由 status 隐式创建。
- `item find` 对无过滤的 8 位大写 Zotero key 使用索引快速路径，本机从约 312 ms 降至约 90–130 ms。普通有限结果查询先对基础候选排序分页，再只为最终页加载 creators、tags 和 attachments；高命中词的净业务耗时约下降 16%。曾验证的独立候选-ID SQL 在零命中和稀有标题场景产生 25%–83% 回归，已撤回，不进入生产路径。
- `config check` 仍是纯 Web 热点，但它是显式在线验证，不应为了 benchmark 改成离线命令。
