# Zotero CLI 文档

AI 原生的 Zotero 命令行工具。为 Claude Code、Codex 等 AI agent 设计，让 AI 能直接操作你的 Zotero 文献库。

> 项目 README 见 [../README.md](../README.md)

---

## 文档导航

### 用户 & Agent

| 文档 | 说明 |
|------|------|
| [快速上手](user/quickstart.md) | 首次使用、初始化配置、Agent 调用最佳实践 |
| [命令参考](user/commands.md) | 全部命令的完整用法、选项和输出示例 |
| [使用示例](user/examples/) | 按场景分类的实际使用案例 |

### 架构与设计

| 文档 | 说明 |
|------|------|
| [架构概览](architecture/overview.md) | 目录结构、三层架构（CLI/Backend/Domain）、三种运行模式 |
| [后端设计](architecture/backend.md) | LocalReader 实现细节、SQLite 查询策略、附件路径解析 |
| [领域模型](architecture/domain-model.md) | 核心数据结构：Item / Annotation / Attachment / FindOptions |
| [设计决策](architecture/decisions.md) | 关键技术决策及其理由 |

### 参考资料

| 文档 | 说明 |
|------|------|
| [Zotero Schema 兼容性](reference/zotero-schema.md) | Zotero 7→9 SQLite schema 变更对照，Z9 DDL 参考 |
| [性能基线](reference/performance-baseline.md) | 各命令耗时基线测量数据与优化方向 |

### 调研报告（历史存档）

| 文档 | 说明 |
|------|------|
| [PDF 处理能力调研](research/pdf-processing.md) | pdfcpu / PyMuPDF / go-pdfium 等方案对比实测 |
| [MVP 定义](research/mvp-definition.md) | 产品定位与原始设计范围（v0.0.1 已完成） |

### 规划中

| 文档 | 说明 | 状态 |
|------|------|------|
| [路线图](plans/roadmap.md) | 当前版本目标与执行顺序 | 活跃 |
| [知识图谱设计](plans/knowledge-graph.md) | 数据模型、GraphStore 接口、ER 图 | 规划中 |
| [Agent 运行时](plans/agent-runtime.md) | Agent Loop、记忆分层、行为规则、自进化 | 规划中 |
| [限流优化](plans/optimizations/rate-limiting.md) | 重试/限流/缓存/熔断器分层方案 | 待实施 |
| [原生能力对接](plans/optimizations/native-integration.md) | Zotero API/Web 本地 DB 优化机会 | 待实施 |

### 开发指南

| 文档 | 说明 |
|------|------|
| [前端开发指南](dev/frontend.md) | Web 前端技术栈、API 设计、开发命令速查 |
| [CI/CD](dev/ci-cd.md) | GitHub Actions 工作流说明 |

### 归档

| 文档 | 说明 |
|------|------|
| [0.0.2 Stability Pass](archive/stability-pass-0.0.2.md) | hybrid fallback 稳定性修复记录 |
