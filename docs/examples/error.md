# error — 错误响应

所有命令的错误响应统一格式：

```json
{
  "ok": false,
  "command": "find",
  "error": "错误描述信息",
  "meta": {}
}
```

## 常见错误场景

### 1. 认证失败（exit code: 3）

```bash
zot find "test" --json
```

```json
{
  "ok": false,
  "command": "find",
  "error": "API 认证失败 (HTTP 401): 请检查 ZOT_API_KEY 是否正确",
  "meta": {}
}
```

### 2. 条目未找到（exit code: 1）

```bash
zot show NONEXISTENT --json
```

```json
{
  "ok": false,
  "command": "show",
  "error": "条目不存在: NONEXISTENT",
  "meta": {}
}
```

### 3. 参数错误（exit code: 2）

```bash
zot show
```

```json
{
  "ok": false,
  "command": "show",
  "error": "usage: zot show <item-key> [--json] [--snippet]",
  "meta": {}
}
```

### 4. 权限不足 — 写操作被禁止（exit code: 1）

```bash
ZOT_ALLOW_WRITE=0 zot create-item --from-file data.json
```

```json
{
  "ok": false,
  "command": "create-item",
  "error": "写操作已被禁用（ZOT_ALLOW_WRITE=0）。如需执行写操作，请设置 ZOT_ALLOW_WRITE=1",
  "meta": {}
}
```

### 5. 权限不足 — 删除被禁止（exit code: 1）

```bash
zot delete-item KEY --if-unmodified-since-version 100
```

```json
{
  "ok": false,
  "command": "delete-item",
  "error": "删除操作已被禁用（ZOT_ALLOW_DELETE=0）。如需执行删除操作，请设置 ZOT_ALLOW_DELETE=1",
  "meta": {}
}
```

### 6. 版本冲突（exit code: 1）

```bash
zot update-item KEY --from-file patch.json --if-unmodified-since-version 100
```

```json
{
  "ok": false,
  "command": "update-item",
  "error": "版本冲突 (HTTP 412): 库已变更，请重新获取最新版本号后重试。当前版本: 150",
  "meta": {}
}
```

### 7. 本地模式不支持的功能（exit code: 1）

```bash
ZOT_MODE=local zot find "test" --qmode everything --json
```

```json
{
  "ok": false,
  "command": "find",
  "error": "local 模式不支持: find --qmode。此功能仅 web/hybrid 模式可用",
  "meta": {}
}
```

### 8. 配置缺失（exit code: 3）

```bash
zot find "test" --json
```

```json
{
  "ok": false,
  "command": "find",
  "error": "配置文件不存在: ~/.zot/.env。请先运行 'zot init' 初始化配置",
  "meta": {}
}
```

## Exit Code 规范

| Code | 含义 | AI 处理建议 |
|------|------|-------------|
| **0** | 成功 | 正常消费 data |
| **1** | 运行时错误 | 向用户报告 error 内容，建议修复方案 |
| **2** | 参数/用法错误 | 显示 usage 信息，提示正确参数 |
| **3** | 配置错误 | 引导用户运行 `zot init` 或检查环境变量 |

## AI Agent 错误处理建议

```
1. 解析 ok 字段：false → 有错误
2. 读取 error 字段：向用户展示具体原因
3. 根据 exit code 决定是否可自动重试：
   - code 1（版本冲突）：重新获取版本号后重试
   - code 2（参数错误）：修正参数后重试
   - code 3（配置错误）：引导用户初始化配置
   - code 1（认证失败）：引导用户检查 API key
```
