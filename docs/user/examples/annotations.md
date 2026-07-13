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
zot ann list ITEMKEY --attachment ATTACHMENT_KEY --json
```

结果可能同时包含 Zotero 数据库标注和直接写入 PDF 文件的标注。使用 `--type`、`--page`、`--author` 缩小结果范围。条目有多个 PDF 时默认读取第一个 PDF；使用 `--attachment ATTACHMENT_KEY` 后，数据库和 PDF 文件标注都会限定到该附件。

## 创建标注

按文本定位：

```powershell
zot ann new ITEMKEY --text "target phrase" --color yellow --json
zot ann new ITEMKEY --attachment ATTACHMENT_KEY --page 4 --text "GATK VariantFiltration" --comment "方法要点" --json
```

按坐标定位：

```powershell
zot ann new ITEMKEY --page 3 --rect 100,200,350,220 --color red --json
zot ann new ITEMKEY --page 5 --point 300,400 --type note --comment "重要发现" --json
```

批量输入：

```powershell
zot ann new ITEMKEY --from annotations.json --dry-run --json
zot ann new ITEMKEY --from annotations.json --json
```

先使用 `--dry-run` 检查批量输入。文本定位可能匹配多处；需要精确结果时同时指定 `--page`。实际写入在同目录临时副本中完成并重新打开验证，成功后才替换原 PDF；零匹配会报错且保留原文件。`--dry-run` 不修改文件，并允许零匹配以便检查条件。

## 删除标注

删除是显式 destructive action，必须区分 Zotero 原生标注和 PDF 内嵌标注：

```powershell
zot ann delete ITEMKEY --source zotero --type highlight --dry-run --json
zot ann delete ITEMKEY --source zotero --type highlight --yes --json
zot ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --page 3 --dry-run --json
zot ann delete ITEMKEY --source pdf --attachment ATTACHMENT_KEY --page 3 --yes --json
```

`--dry-run` 返回即将删除的精确 key 或 PDF xref。Zotero 来源使用标准 Web API 删除 annotation items，不直接写 SQLite；PDF 来源先修改同目录临时副本，验证成功后才替换原文件。删除操作受 `ZOT_ALLOW_DELETE` 控制。省略 `--yes` 时，交互终端会要求确认。

## Remote 模式

在 remote 模式下，`ann list/new` 与 `ann delete --source pdf` 由运行 `zot serve` 的机器执行，因此 PDF 文件不需要复制到客户端。`ann delete --source zotero` 仍需要客户端 Web API 凭据。

## 常见错误

| 错误写法 | 正确写法 |
|---|---|
| `annotations ITEMKEY` | `ann list ITEMKEY` |
| `annotate ITEMKEY ...` | `ann new ITEMKEY ...` |
| `annotations ITEMKEY --clear` | `ann delete ITEMKEY --source zotero --dry-run` |

旧入口已经移除，不会再被自动翻译。
