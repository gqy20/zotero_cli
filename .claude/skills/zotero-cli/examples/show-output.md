# `zot item show --json` 输出示例

```powershell
zot item show ABCD1234 --json
```

```json
{
  "ok": true,
  "command": "item show",
  "data": {
    "key": "ABCD1234",
    "item_type": "journalArticle",
    "title": "CRISPR-Cas systems for genome editing",
    "creators_summary": "Doe et al.",
    "date": "2024",
    "date_added": "2026-07-10T08:30:00Z",
    "container": "Genome Biology",
    "doi": "10.1000/example",
    "abstract": "CRISPR-Cas9 has transformed genome editing..."
  },
  "meta": {
    "read_source": "live"
  }
}
```

正式快捷入口 `zot show ABCD1234 --json` 返回相同数据，`command` 仍为 `item show`。

## 正文预览

```powershell
zot item show ABCD1234 --snippet --json
```

当任务只是确认正文相关内容时优先使用 `--snippet`；只有明确需要整篇正文时才使用：

```powershell
zot pdf text ABCD1234 --json
```

## 完整结构

```powershell
zot item show ABCD1234 --full --json
```

完整结果可能额外包含：

- `creators[]`
- `attachments[]`
- `notes[]`
- `annotations[]`
- `collections[]`
- `journal_rank`
- `version`

不要再使用已退出稳定 CLI 的 `abstract` 或 `relate` 命令；摘要在 `item show` 数据中，关系元数据也应从条目详情或导出数据中读取。
