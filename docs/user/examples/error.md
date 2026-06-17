# error — 错误响应

CLI 默认把错误写到 **stderr**。设置 `ZOT_JSON_ERRORS=1` 后，错误也以 JSON 形式输出到 stdout（Agent 解析用），结构与成功响应同形。

## 文本模式（默认）

```
error: zotero api not found (404): no such item: ABCD1234
```

退出码见 [Exit Code 规范](#exit-code-规范)。

## JSON 模式（`ZOT_JSON_ERRORS=1`）

`error` 字段嵌套在 `data` 下，另有机器可读的 `type` 类别与 `code` 退出码：

```json
{
  "ok": false,
  "command": "find",
  "data": {
    "error": "human-readable message",
    "type": "forbidden",
    "code": 1
  },
  "code": 1
}
```

`type` 取值（与 [exit code](#exit-code-规范) 正交，方便分支处理）：

| type | 触发场景 |
|------|----------|
| `not_found` | 条目 / 收藏夹 / 搜索不存在 |
| `unauthorized` | HTTP 401 |
| `forbidden` | HTTP 403（API key 无权限） |
| `conflict` | HTTP 409（key 冲突） |
| `precondition_failed` | HTTP 412（`--if-unmodified-since-version` 不满足） |
| `rate_limited` | HTTP 429（带 Retry-After） |
| `unsupported_feature` | 当前 mode 不支持该能力 |
| `temporarily_unavailable` | SQLite 被锁 / 模式降级 |
| `bad_request` / `payload_too_large` / `method_not_allowed` | HTTP 4xx |
| `server_error` / `server_error_500` ... | HTTP 5xx |
| `unknown` | 兜底 |

## Exit Code 规范

| Code | 含义 | AI 处理建议 |
|------|------|-------------|
| **0** | 成功 | 正常消费 data |
| **1** | 运行时错误 | 向用户报告 error 内容；按 `type` 决定是否重试 |
| **2** | 参数/用法错误 | 显示 usage 信息，提示正确参数 |
| **3** | 配置错误 | 引导用户运行 `zot init` 或检查环境变量 |

## AI Agent 错误处理建议

```
1. 解析 ok 字段：false → 有错误
2. 读取 data.error：向用户展示具体原因
3. 读取 data.type：决定分支（precondition_failed → 拉新 version 重试；rate_limited → 退避后重试；not_found → 不重试）
4. 读不到 type 时回退到 code：1=运行时错误/重试边界由调用者决定；2=参数错误/不重试；3=配置错误/不重试
```
