# cite — 引文生成

## 命令

```bash
zot cite ABC123DE --format bibtex --json
```

## 输出

```json
{
  "ok": true,
  "command": "cite",
  "data": "@article{Smith2024Homoploid,\n  title = {Homoploid hybrid speciation in action},\n  author = {Smith, John and Wang, Li},\n  journal = {Nature Ecology \\& Evolution},\n  year = {2024},\n  volume = {8},\n  number = {3},\n  pages = {456--467},\n  doi = {10.1038/s41559-024-01234-x}\n}",
  "meta": {}
}
```

> 注意：`data` 字段为**纯文本字符串**（引文内容本身），不是嵌套 JSON 对象。

## 支持格式

| format 参数 | 输出格式 | 示例 |
|-------------|----------|------|
| （默认） | APA | Smith, J., & Wang, L. (2024). Homoploid... *Nat. Ecol. Evol.*, 8(3), 456–467. |
| `bibtex` | BibTeX | `@article{...}` |
| `chicago` | Chicago 注释 | ①... |
| `biblatex` | BibLaTeX | 类 BibTeX 但兼容 biblatex |

## 其他用法

```bash
# 默认 APA 格式（文本输出）
zot cite ABC123DE

# 指定风格和语言
zot cite ABC123DE --style nature --locale en-US --json
```
