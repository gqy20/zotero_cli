# 命令参考

CLI v2 使用稳定的“资源 + 动作”语法。`zot <resource> --help` 和 `zot <resource> <action> --help` 是参数细节的权威来源。

## 全局选项

```text
--format text|json
--json
--quiet, -q
--verbose, -v
--no-color
--mode web|local|hybrid|remote
--timeout DURATION
```

JSON 响应统一使用 `{ok, command, data, meta}`；`command` 始终是 canonical path，例如 `item find`、`schema list`。

## 命令树

```text
zot
├── lib show|stats|log
├── item list|find|show|new|edit|delete|tag|untag|supp|export
├── coll list|show|new|edit|delete|add|remove
├── note list|show|find|new|edit|delete
├── tag list
├── search list|show|new|edit|delete
├── group list
├── file show|check
├── pdf text|figs|open
├── ann list|new|delete
├── ref show|find|related|cited|ctx|links|entities|profile|build|resolve|status
├── index build|status
├── schema list|show
├── config init|show|check
├── server start
├── sync pull
├── completion
└── version
```

高频快捷入口 `find`、`show`、`export` 分别等价于 `item find`、`item show`、`item export`，但没有独立实现。

## Library 与只读资源

```powershell
zot lib show --json
zot lib stats --json
zot lib log --deleted --json
zot lib log --kind items --since 120 --json

zot item list --limit 20 --json
zot item list --scope trash --json
zot item list --scope pubs --json
zot item find "CRISPR" --type article --limit 20 --json
zot item show ITEMKEY --json

zot coll list --json
zot note list --json
zot note find "methods" --json
zot tag list --json
zot search list --json
zot group list --json
```

统一分页参数是 `--limit`、`--offset`、`--sort`、`--order asc|desc`。旧 `--start`、`--direction` 已不再接受。

常用 item type 短名会在输入层归一化：`article` → `journalArticle`、`chapter` → `bookSection`、`conf` → `conferencePaper`、`web` → `webpage`、`blog` → `blogPost`。JSON 和持久化数据始终使用 Zotero 官方值。

## 写入与安全

```powershell
zot item new --set itemType=article --set title="Example" --dry-run --json
zot item edit ITEMKEY --set title="New title" --if-version 42 --json
zot item delete ITEMKEY --yes --if-version 42 --json
zot item tag KEY1 KEY2 --tag review --json
zot item untag KEY1 KEY2 --tag review --json

zot coll new --name "Reviews" --json
zot coll add COLLKEY ITEM1 ITEM2 --json
zot coll remove COLLKEY ITEM1 --json
zot note new --parent ITEMKEY --text "Reading note" --json
zot search new --data '{"name":"Recent","conditions":[]}' --json
```

写入统一接受 `--set FIELD=VALUE`、`--data JSON`、`--from PATH`；`--from -` 表示 stdin。安全选项统一为 `--dry-run`、`--yes/-y`、`--if-version`。

- `ZOT_ALLOW_WRITE=1` 控制创建和修改。
- `ZOT_ALLOW_DELETE=1` 控制删除，默认关闭。
- destructive action 始终执行确认、权限和版本前置条件。

## 附件、PDF 与标注

```powershell
zot item supp ITEMKEY --json
zot file show ATTACHKEY --json
zot file check ATTACHKEY --json

zot pdf text ITEMKEY --pages 3-8 --grep methods --max-chars 12000 --json
zot pdf text ITEM1 ITEM2 --workers 4 --output-dir ./markdown --json
zot pdf figs ITEMKEY --output-dir ./figures --json
zot pdf open ITEMKEY --page 5

zot ann list ITEMKEY --type highlight --page 3 --json
zot ann new ITEMKEY --text "target phrase" --color yellow --json
zot ann delete ITEMKEY --type highlight --yes --json
```

`ann list`、`ann new`、`ann delete` 明确区分读取、创建和删除。canonical 语法不接受 `annotations --clear` 或 `annotate --clear`。

## Reference 与全文索引

```powershell
zot ref show ITEMKEY --json
zot ref find "genome assembly" --field mesh --json
zot ref related ITEMKEY --limit 20 --json
zot ref cited ITEMKEY --external --json
zot ref ctx ITEMKEY --json
zot ref links ITEMKEY --json
zot ref entities ITEMKEY --json
zot ref profile ITEMKEY --json

zot ref build --workers 3 --json
zot ref build --failed --workers 2 --json
zot ref build --contexts --workers 3 --json
zot ref build --grobid --limit 5 --json
zot ref resolve --workers 8 --json
zot ref status --json
zot ref status --failed --json
zot ref status --unsupported --json
zot ref status --grobid --json

zot index build --workers 4 --json
zot index status --json
```

`ref build --failed` 与 `ref build --contexts` 是互斥范围；GROBID 仅在显式 `--grobid` 时启用。完整来源和 fallback 语义见 [引用索引与文献发现](./references.md)。

## Schema、配置与运行时

```powershell
zot schema list types --json
zot schema list fields --json
zot schema list fields article --json
zot schema list roles article --json
zot schema show article --json

zot config init
zot config show --json
zot config show --path
zot config check --json

zot server start --port 8021
zot sync pull --server-addr http://host:8021 --data-dir ~/.zot/sync
zot completion powershell
zot completion bash
zot version
```

Schema 响应默认缓存 7 天；`meta.read_source=cache` 表示缓存命中。使用 `--refresh` 强制联网更新。缓存过期且网络不可用时会返回 stale 缓存，并通过 `meta.stale=true` 和 warning 明确提示。

completion 支持 `bash`、`zsh`、`fish`、`powershell`，生成过程不会加载配置或访问网络。

## 兼容边界

旧 alias、旧参数翻译和 redirect-only 入口已经移除；旧命令返回 usage error。脚本应直接迁移到 canonical 命令树。详细清单见 [回退与历史兼容](../architecture/fallbacks.md)。

仅 `find`、`show`、`export` 是正式快捷入口，分别等价于 `item find`、`item show`、`item export`，并且只接受 canonical 参数。
