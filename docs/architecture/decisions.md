# 设计决策

项目关键架构决策及其理由。

> 架构概览见 [overview](./overview.md)，领域模型见 [domain-model](./domain-model.md)。

---

## 1. 接口驱动，零 CGO

- `Reader` 接口使三种模式可互换，CLI 层不关心具体实现
- SQLite 使用纯 Go 驱动 `modernc.org/sqlite`，无 CGO 依赖，跨平台编译零障碍
- PDF 处理通过 Python 子进程（PyMuPDF）而非 Go 绑定，避免 CGO

## 2. Hybrid 回退策略

不是简单的"本地失败就走 Web"，而是**能力感知回退**：

| 本地能力 | 回退行为 |
|----------|----------|
| 不支持的功能（如 `--qmode`） | 可回退 Web |
| 本地独有的功能（如 `--fulltext`） | **不回退**，保留错误 |
| 本地临时不可用（如数据库锁定） | 视操作类型决定 |

## 3. Python 子进程模式

PDF 操作（文本提取、标注读写）通过 stdin 传递脚本、argv 传递路径、stdout 返回 JSON：

```
CLI → (Python script via stdin, PDF path via argv) → JSON stdout → Go 解析
```

Python 环境自动管理在 `{ZOT_DATA_DIR}/.zotero_cli/venv/`，优先使用 `uv` 包管理器。

## 4. 写操作安全模型

| 机制 | 说明 |
|------|------|
| 环境变量门控 | `ZOT_ALLOW_WRITE=1` / `ZOT_ALLOW_DELETE=0` |
| 版本号乐观锁 | 所有写操作要求 `--if-unmodified-since-version N` |
| 删除默认禁止 | 删除操作需显式开启 |

## 5. zotero:// 协议集成

`open` 和 `select` 命令直接调用系统 URI 协议，无需加载数据库或配置：

| 命令 | 协议 | 说明 |
|------|------|------|
| `open` | `zotero://open-pdf/library/items/{attachmentKey}?page=N` | 在阅读器中打开 PDF，需传入**附件 itemKey**（非父条目 key） |
| `select` | `zotero://select/library/items/{key}` | 在主界面选中条目 |

## 6. 错误体系

```go
// 后层错误
type unsupportedFeatureError struct { Backend, Feature, Hint string }
type itemNotFoundError struct { Object, Key string }
type dbLockedError struct { Path string }

// API 错误（zoteroapi 包）
type APIError struct { Status int; Message string }
// 401 → 认证失败, 403 → 权限不足, 412 → 版本冲突, 429 → 限流
```

每种错误携带足够上下文，使 CLI 输出可操作的提示。
