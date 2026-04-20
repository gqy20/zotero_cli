# stats — 库统计

## 命令

```bash
zot stats --json
```

## 输出

```json
{
  "ok": true,
  "command": "stats",
  "data": {
    "library_type": "user",
    "library_id": "123456",
    "total_items": 1847,
    "total_collections": 23,
    "total_searches": 8,
    "last_library_version": 9876
  },
  "meta": {
    "total": 1847,
    "read_source": "local"
  }
}
```

## 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `library_type` | string | `user`（个人库）或 `group`（群组库） |
| `library_id` | string | 库 ID |
| `total_items` | int | 条目总数（含回收站） |
| `total_collections` | int | 收藏夹总数 |
| `total_searches` | int | 已保存搜索数 |
| `last_library_version` | int | 库版本号（用于乐观锁） |

> web 模式下 `last_library_version` 可能不返回。
