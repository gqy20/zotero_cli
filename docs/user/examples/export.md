# export — 导出

## 命令

```bash
zot item export ABC123DE --as csljson --json
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
zot item export ABC123DE --as bibtex
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
zot item export ABC123DE --as ris
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

## Nature 格式参考文献

```bash
zot item export KEY1 KEY2 KEY3 --as bibliography --style nature
```

该命令将全部 key 一次交给 Zotero Web API 排版，避免数字编号在分批时重新从 1 开始。省略 `--style` 时使用 `ZOT_STYLE`；普通输出为纯文本，需要保留斜体等格式时输出原始 HTML：

```bash
zot item export KEY1 KEY2 KEY3 --as bibliography --style nature --stream > references.html
```

条目必须已经同步到 Zotero Web library。`bibliography` 最多支持 100 个条目，不支持纯 local 模式；大量、未同步或离线导出请改用 CSL-JSON。

## 支持格式

| format | 说明 | JSON 支持 |
|--------|------|-----------|
| `bibtex` / `biblatex` | BibTeX 系列 | 是 |
| `ris` | RIS 格式 | 是 |
| `csljson` | CSL JSON（local/hybrid 优先本地） | 是 |
| `bibliography` | Zotero CSL 排版（Web API，最多 100 条） | 是 |

## 按其他方式导出

```bash
# 按关键词检索后导出
zot item find '"hybrid speciation"' --json > selected.json
zot item export --from selected.json --as csljson

# 按收藏夹导出
zot item find --collection COLL1234 --all --json | zot item export --from - --as bibtex
```
