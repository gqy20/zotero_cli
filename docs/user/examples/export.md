# export — 导出

## 命令

```bash
zot export --item-key ABC123DE --format csljson --json
```

## CSL-JSON 输出

```json
{
  "ok": true,
  "command": "export",
  "data": "[{\"id\":\"ABC123DE\",\"type\":\"article-journal\",\"title\":\"Homoploid hybrid speciation in action\",\"author\":[{\"family\":\"Smith\",\"given\":\"John\"},{\"family\":\"Wang\",\"given\":\"Li\"}],\"container-title\":\"Nature Ecology & Evolution\",\"volume\":\"8\",\"issue\":\"3\",\"page\":\"456-467\",\"issued\":{\"date-parts\":[[2024,3,15]]},\"DOI\":\"10.1038/s41559-024-01234-x\",\"URL\":\"https://doi.org/10.1038/s41559-024-01234-x\"}]",
  "meta": {
    "format": "csljson",
    "read_source": "local"
  }
}
```

> `data` 为字符串形式的 CSL-JSON 数组。多条目时返回完整数组。

## BibTeX 导出

```bash
zot export --item-key ABC123DE --format bibtex
```

```
@article{Smith2024Homoploid,
  title = {Homoploid hybrid speciation in action},
  author = {Smith, John and Wang, Li},
  journal = {Nature Ecology \& Evolution},
  year = {2024},
  volume = {8},
  number = {3},
  pages = {456--467},
  doi = {10.1038/s41559-024-01234-x}
}
```

## RIS 导出

```bash
zot export --item-key ABC123DE --format ris
```

```
TY  - JOUR
TI  - Homoploid hybrid speciation in action
AU  - Smith, John
AU  - Wang, Li
JO  - Nature Ecology & Evolution
VL  -8
IS  -3
SP  -456
EP  -467
PY  -2024
DO  - 10.1038/s41559-024-01234-x
ER  -
```

## 支持格式

| format | 说明 | JSON 支持 |
|--------|------|-----------|
| `bibtex` / `biblatex` | BibTeX 系列 | 是 |
| `ris` | RIS 格式 | 是 |
| `csljson` | CSL JSON（local/hybrid 优先本地） | 是 |

## 按其他方式导出

```bash
# 按关键词检索后导出
zot export "hybrid speciation" --format csljson --json

# 按收藏夹导出
zot export --collection COLL1234 --format bibtex --json
```
