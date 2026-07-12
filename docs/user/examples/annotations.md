# PDF 标注：`ann list/new/delete`

Zotero CLI 使用一组明确的 canonical 命令读取、创建和删除 PDF 标注：

```text
ann list    读取标注
ann new     创建标注
ann delete  删除标注
```

## 读取标注

```powershell
zot ann list ITEMKEY --json
zot ann list ITEMKEY --type highlight --json
zot ann list ITEMKEY --page 3 --json
zot ann list ITEMKEY --author "Zotero User" --json
```

结果可能同时包含 Zotero 数据库标注和直接写入 PDF 文件的标注。使用 `--type`、`--page`、`--author` 缩小结果范围。

## 创建标注

按文本定位：

```powershell
zot ann new ITEMKEY --text "target phrase" --color yellow --json
zot ann new ITEMKEY --page 4 --text "GATK VariantFiltration" --comment "方法要点" --json
```

按坐标定位：

```powershell
zot ann new ITEMKEY --page 3 --rect 100,200,350,220 --color red --json
zot ann new ITEMKEY --page 5 --point 300,400 --comment "重要发现" --json
```

批量输入：

```powershell
zot ann new ITEMKEY --from annotations.json --dry-run --json
zot ann new ITEMKEY --from annotations.json --json
```

先使用 `--dry-run` 检查批量输入。文本定位可能匹配多处；需要精确结果时同时指定 `--page`。

## 删除标注

删除是显式 destructive action：

```powershell
zot ann delete ITEMKEY --type highlight --yes --json
zot ann delete ITEMKEY --page 3 --yes --json
```

删除操作受 `ZOT_ALLOW_DELETE` 控制。省略 `--yes` 时，交互终端会要求确认；自动化流程必须显式提供 `--yes`。

## Remote 模式

在 remote 模式下，`ann list/new/delete` 由运行 `zot serve` 的机器执行，因此 PDF 文件不需要复制到客户端。服务端写入和删除仍分别受 `ZOT_ALLOW_WRITE` 与 `ZOT_ALLOW_DELETE` 控制。

## 常见错误

| 错误写法 | 正确写法 |
|---|---|
| `annotations ITEMKEY` | `ann list ITEMKEY` |
| `annotate ITEMKEY ...` | `ann new ITEMKEY ...` |
| `annotations ITEMKEY --clear` | `ann delete ITEMKEY --yes` |

旧入口已经移除，不会再被自动翻译。
