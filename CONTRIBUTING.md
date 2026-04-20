# 贡献指南

感谢你对 zot 的关注！无论是修复 bug、新增功能、改进文档还是优化测试，都欢迎贡献。

## 开发环境

### 前置要求

- **Go 1.26+**（项目使用 `go.mod` 指定版本）
- **Git**
- **Make**（可选，Windows 可用 `choco install make` 或直接用 `go` 命令）

### 快速搭建

```bash
# 1. 克隆仓库
git clone https://github.com/gqy20/zotero_cli.git && cd zotero_cli

# 2. 安装 pre-commit hook（推荐）
make install-hooks

# 3. 验证环境
make check          # fmt-check + vet + test
```

> 如果没有 Make，等效命令：`gofmt -w ./internal/ ./cmd/ && go vet ./... && go test ./... -short`

### 本地运行

```bash
# 构建
go build -o zot.exe ./cmd/zot

# 运行（需要先配置）
./zot.exe config init
./zot.exe version
```

## 开发规范

### 代码风格

- 使用 `gofmt` 格式化（`make fmt` 一键处理）
- 通过 `go vet` 静态检查（`make vet`）
- 新命令文件命名：`commands_<name>.go`
- 参数解析使用 `nextFlag` 状态机模式（参考现有命令）

### Exit Code 规范

所有命令必须使用统一的退出码常量（定义在 `internal/cli/types.go`）：

| 常量 | 值 | 含义 | 使用场景 |
|------|-----|------|----------|
| `ExitOK` | 0 | 成功 | 正常返回 |
| `ExitError` | 1 | 运行时错误 | API 失败、条目不存在、权限拒绝 |
| `ExitUsage` | 2 | 用法错误 | 缺少参数、未知选项 |
| `ExitConfig` | 3 | 配置错误 | 配置缺失、认证失败 |

```go
// 正确
if len(args) == 0 {
    return ExitUsage
}
// 错误（不要硬编码数字）
if err != nil {
    return 1  // 应该用 ExitError
}
```

### JSON 输出

所有支持 `--json` 的命令统一包裹在 `jsonResponse` 中：

```go
return c.writeJSON(jsonResponse{
    OK:      true,
    Command: "find",
    Data:    items,
    Meta:    meta,
})
```

错误响应：

```go
return c.writeJSON(jsonResponse{
    OK:      false,
    Command: "find",
    Error:   "条目不存在",
})
```

### 测试

- 新功能必须附带测试
- 测试文件与源码同目录：`commands_find_test.go`
- 使用项目已有的 test helper（`test_helpers_test.go`）
- 跑全量测试：`make test`；跑快速测试：`make test-short`

### 提交信息格式

```
<类型>: <简短描述>

<可选的详细说明>

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
```

类型前缀：

| 前缀 | 用途 |
|------|------|
| `feat:` | 新功能 |
| `fix:` | Bug 修复 |
| `docs:` | 文档变更 |
| `refactor:` | 重构（不改变行为） |
| `test:` | 测试相关 |
| `chore:` | 构建/工具链 |

## 提交流程

### 1. Fork & 分支

```bash
git checkout -b feature/your-feature-name
```

分支命名：`feature/xxx` / `fix/xxx` / `docs/xxx`

### 2. 开发 & 自检

```bash
make check        # 必须通过
make build        # 确保编译成功
```

pre-commit hook 会在 `git commit` 时自动拦截未通过的检查。

### 3. 提交 PR

- 填写 PR 模板中的所有必填项
- 关联相关的 Issue（如有）
- 确保 CI 绿灯（自动触发）

### 4. Code Review

- 至少一位维护者 review 后合并
- 小改动可快速合并，大改动可能需要讨论

## 项目结构速查

```
internal/
├── cli/           # 命令层（参数解析、输出格式化）
│   ├── cli.go     # 主调度器
│   ├── types.go   # 类型定义 + Exit Code 常量
│   └── commands_*.go  # 各命令实现
├── backend/       # 数据访问层（web/local/hybrid）
├── domain/        # 领域模型（Item, Annotation 等）
├── config/        # 配置管理
└── zoteroapi/     # Zotero API 客户端
docs/
├── examples/      # JSON 输出示例（AI Agent 参考）
├── commands.md    # 完整命令参考
├── architecture.md # 技术架构
└── AI_AGENT.md    # Agent 使用指南
.claude/skills/    # Claude Code skill
.codex/skills/     # Codex skill
```

## 需要帮助？

- 提 **Issue** 描述问题或建议
- 在 **Discussions** 中提问（如果有开启）
- 查看 [JSON 输出示例](docs/examples/) 了解各命令的数据结构
- 阅读 [AI Agent 指南](docs/AI_AGENT.md) 了解设计意图
