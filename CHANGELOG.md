# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.0.1`。

## [Unreleased]

### 新增
- 使用 `CHANGELOG.md` 驱动 GitHub Release 发布说明。

### 计划中
- 启动 `0.0.2` 稳定性迭代，重点收敛 `hybrid` fallback 语义。
- 计划去除依赖错误文案的 fallback 判断，改为稳定的 backend 能力/错误信号。
- 计划统一 `find` 的日期过滤、标签过滤和默认可见条目语义，减少 web/local 路径重复实现。
- 计划改进写命令参数错误信息，让脚本和 agent 更容易定位失败原因。

## [0.0.1] - 2026-03-31

### 新增
- 首次公开发布 `zot` 命令行工具。
- 提供 Linux、macOS 和 Windows 的跨平台发布产物。

### 变更
- 发布压缩包现在会包含项目 `LICENSE`。
- 发布流程会在进入构建矩阵前统一执行一次测试。
- 发布二进制现在会注入稳定的 UTC 构建时间。
## 0.0.2 Stability Updates (In Progress)

### Changed
- `hybrid` remote fallback now routes through a normalized web client path, so hybrid read flows can still reach the Zotero Web API when local data is unavailable or unsupported.
- write command validation now reports specific argument errors for missing precondition versions, conflicting `--data` and `--from-file` flags, unreadable payload files, and invalid JSON payloads.
- `local` mode now fails fast for Web API-only commands with an explicit mode-boundary error instead of a generic unsupported-mode failure.

### Docs
- documented the `0.0.2` stability pass progress in `docs/stability-pass-0.0.2.md`
- documented current mode boundaries in `docs/local-backend-design.md`
