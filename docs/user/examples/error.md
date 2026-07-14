# error — 错误响应

文本模式把错误写到 stderr。使用 `--json`、`--format json` 或 `ZOT_OUTPUT=json` 时，stdout 始终是一个 JSON envelope，stderr 保持为空。

```json
{
  "ok": false,
  "command": "item find",
  "error": {
    "type": "usage",
    "message": "--full and --snippet are mutually exclusive"
  },
  "code": 2
}
```

错误响应不包含 `data`，退出码只在顶层 `code` 出现一次。常见 `type` 包括 `usage`、`config`、`not_found`、`unauthorized`、`forbidden`、`conflict`、`precondition_failed`、`rate_limited`、`unsupported_feature`、`temporarily_unavailable` 和 `server_error_*`。

| Code | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 运行时错误 |
| 2 | 参数或用法错误 |
| 3 | 配置错误 |
| 130 | 操作被取消 |

帮助页和 shell completion 是纯文本产物；不要给它们添加 `--json`。`config init --json` 不会显示交互提示，必须一次传齐当前模式需要的参数。
