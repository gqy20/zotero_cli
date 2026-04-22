# annotations / annotate — PDF 标注读取与写入（双源）

## 概述

Zotero CLI 的标注系统操作 **两层** 独立数据源：

```
┌─────────────────────────────────────────────┐
│              用户视角 (itemKey)              │
│                                             │
│  ┌──────────────┐    ┌──────────────────┐   │
│  │ PDF 文件层    │    │ Zotero DB 层      │   │
│  │ (PyMuPDF)     │    │ (SQLite)           │   │
│  ├──────────────┤    ├──────────────────┤   │
│  │ highlight      │    │ highlight          │   │
│  │ underline      │    │ note               │   │
│  │ note (sticky)  │    │ image              │   │
│  │ circle/ink ... │    │ ink                │   │
│  └──────┬───────┘    └────────┬─────────┘   │
│         │                     │             │
│  写入: annotate 命令   读/删: annotations   │
│  删除: --clear         删除: --clear       │
└─────────────────────────────────────────────┘
```

**关键约束**：DB 层删除需要 **Zotero 处于关闭状态**（SQLite WAL 锁）。Zotero 运行时 DB 删除会跳过并输出 warning，不影响 PDF 层操作。

---

## 一、读取标注 (`annotations`)

### 基本用法

```bash
# 查看某文献的所有标注（双源）
zot annotations ABC123DE

# JSON 输出（推荐 Agent 使用）
zot annotations ABC123DE --json
```

### 过滤选项

```bash
# 仅查看高亮
zot annotations KEY --type highlight

# 仅查看第 3 页的标注
zot annotations KEY --page 3

# 按作者过滤（DB 层有效）
zot annotations KEY --author "Zotero User"

# 组合过滤
zot annotations KEY --type highlight --page 5
```

### 输出示例（JSON）

```json
{
  "ok": true,
  "command": "annotations",
  "data": {
    "item_key": "ABC123DE",
    "attachment_key": "XYZ789GH",
    "pdf_path": "D:\\zotero\\...\\paper.pdf",
    "pdf_annotations": [
      {
        "page": 3,
        "type": "highlight",
        "text": "homoploid hybrid speciation requires...",
        "color": "#ffd400",
        "rect": [100.5, 200.0, 350.0, 218.0],
        "author": "",
        "mod_date": "2024-04-11T09:15:00"
      }
    ],
    "db_annotations": [
      {
        "type": "highlight",
        "text": "核心定义",
        "comment": "",
        "color": "#ffd400",
        "page_index": 1,
        "date_added": "2024-04-10T14:30:00Z"
      }
    ],
    "total_pdf": 1,
    "total_db": 1
  }
}
```

### 双源对比

| 维度 | DB 层（Zotero Reader） | PDF 层（PyMuPDF） |
|------|----------------------|-------------------|
| **数据来源** | SQLite `itemAnnotations` 表 | PDF 二进制文件扫描 |
| **位置字段** | `page_index`（0-based）+ `position` JSON | `page`（1-based）+ `rect` [x0,y0,x1,y1] |
| **时间戳** | `date_added` | `mod_date` |
| **支持类型** | highlight(0) / note(7) / image(8) | highlight / underline / note / circle / ink / strikeout / squiggly 等 20 种 |
| **删除方式** | `--clear`（需 Zotero 关闭） | `--clear`（随时可用） |
| **写入方式** | 不支持（仅 Zotero UI/Reader 创建） | `annotate` 命令 |

> 注：DB 层 `type` 为整数映射：highlight=0, note=7(sticky), image=8。

---

## 二、写入标注 (`annotate`)

### 三种标注模式

#### Mode 1: 全文文本搜索（所有页面）

```bash
# 在整篇 PDF 中搜索文本并高亮（每处匹配都创建标注）
zot annotate KEY --text "hybrid speciation" --color yellow
```

> **注意**：会匹配文档中**所有出现位置**。如需精确定位到某一页，使用 Mode 1.5。

#### Mode 1.5: 指定页文本搜索（推荐用于精准标注）

```bash
# 仅在第 4 页搜索关键词并高亮，详细说明写在 comment 中
zot annotate KEY --page 4 --text "GATK VariantFiltration" \
  --color yellow \
  --comment "HHS步骤1: SNP calling质控管线 — 6步硬过滤..."
```

**最佳实践**：
- 使用该页面的**唯一短关键词**作为 `--text` 匹配目标
- 将完整方法说明放在 `--comment` 中
- 避免全文搜索导致的多余匹配

#### Mode 2: 精确坐标（rect / point）

```bash
# 在指定页面的精确矩形区域创建高亮
zot annotate KEY --page 3 --rect 100,200,350,220 --color red

# 在指定位置创建便签笔记（point 模式自动填充默认 comment）
zot annotate KEY --page 5 --point 300,400 --comment "重要发现"
```

### 样式选项

```bash
# 颜色（支持颜色名或 hex）
--color yellow        # 高亮黄
--color red           # 强调红
--color "#ff6600"     # 自定义橙色
--color blue          # 下划线蓝

# 类型
--type highlight       # 高亮（默认）
--type underline       # 下划线
--type text            # 便签笔记（配合 --point 使用）

# 注释文字
--comment "方法要点总结"
```

### 实际案例：HHS 方法论论文标注

```bash
# P4: SNP 质控管线
zot annotate SXJ9FYTK --page 4 --text "GATK VariantFiltration" \
  --color yellow \
  --comment "HHS步骤1: SNP calling质控管线"

# P5: 渗入检测套件
zot annotate SXJ9FYTK --page 5 --text "QuIBL Analysis" \
  --color orange \
  --comment "HHS步骤2: QuIBL/D-statistic/HyDe/LOTER"

# P6: 核心方法 - HKA test
zot annotate SXJ9FYTK --page 6 --text "HKA test" \
  --color red \
  --comment "HHS核心: 交替继承等位基因鉴定"

# P8: 全基因组冲突信号
zot annotate SXJ9FYTK --page 8 --text "phylogenetic conflict" \
  --color cyan \
  --comment "HHS步骤4: 多证据收敛排除纯ILS"
```

---

## 三、清除标注 (`--clear`)

### 双层删除行为

`--clear` 会**同时尝试删除两层**标注：

```bash
# 删除所有标注（PDF + DB）
zot annotations KEY --clear

# 通过 annotate 命令也可清除
zot annotate KEY --clear

# 按条件删除
zot annotations KEY --clear --type highlight    # 仅删高亮
zot annotations KEY --clear --page 5           # 仅删第5页
zot annotations KEY --clear --author "User"    # 按作者删除
zot annotations KEY --clear --type note --page 3  # 组合条件
```

### 执行结果示例

**Zotero 已关闭时**（双层成功）：
```
Deleted 33 annotation(s) from SXJ9FYTK
PDF: D:\zotero\...\paper.pdf
```
→ PDF 层 + DB 层均删除成功

**Zotero 运行时**（仅 PDF 成功）：
```
warning: could not delete DB annotations (Zotero may be running): attempt to write a readonly database (8)
Deleted 2 annotation(s) from SXJ9FYTK
PDF: D:\zotero\...\paper.pdf
```
→ PDF 层已删除，DB 层跳过（不报错退出）。关闭 Zotero 后重新执行即可清理 DB 层。

### 清除流程图

```
用户执行 --clear
       │
       ▼
┌──────────────────┐     ┌─────────────────────┐
│  PDF 层删除       │     │  DB 层删除           │
│  (PyMuPDF 直接写) │     │  (SQLite DELETE)     │
│                  │     │                     │
│  ✅ 始终可用      │     │  ⚠️ 需要 Zotero 关闭  │
│  按 page/type/    │     │  按 page/type/author │
│  author 过滤      │     │  过滤                │
└────────┬─────────┘     └──────────┬──────────┘
         │                          │
         │                    ┌─────┴─────┐
         │                    │            │
         │                  成功          失败
         │                    │            │
         │                    ▼            ▼
         │              累加计数      输出 warning
         │                          (不阻断)
         ▼                          │
    汇总输出 totalDeleted ◄─────────┘
```

---

## 四、常见问题

### Q: 为什么 `--text "长句"` 匹配不到？

PDF 文本层与显示文本可能不一致（CJK 字体子集、连字、特殊编码）。建议：
1. 使用**短且唯一**的关键词（英文优先）
2. 先用 `extract-text` 确认该页实际文本内容
3. 用 `--page N` 缩小搜索范围减少误匹配

### Q: `--point x,y` 和 `--rect x0,y0,x1,y2` 的区别？

| 模式 | 效果 | 适用场景 |
|------|------|----------|
| `--point` | 在坐标点创建圆形便签（circle type） | 浮动笔记 |
| `--rect` | 在矩形区域创建高亮/下划线 | 文本定位标注 |

> **注意**：`--point` 创建的是**浮动便签**，不会附着在具体文本上。如需在特定句子旁标注，优先用 `--page N --text "keyword"`。

### Q: `annotate --clear` 和 `annotations --clear` 有区别吗？

功能完全相同，都会执行双层删除。选择哪个取决于使用习惯：
- `annotate --clear` — 如果正在做标注工作流
- `annotations --clear` — 如果正在查看/管理标注

### Q: DB 层的 30 条旧标注怎么来的？如何彻底清除？

如果之前用 `--point` 反复尝试创建了大量 circle/image 类型的 DB 标注：

```bash
# 1. 关闭 Zotero
# 2. 执行清除（此时双层均可删除）
zot annotate KEY --clear

# 3. 验证清空
zot annotations KEY --json | python -c "import sys,json; d=json.load(sys.stdin); print(f'PDF:{d[\"data\"][\"total_pdf\"]} DB:{d[\"data\"][\"total_db\"]}')"
```

预期输出：`PDF:0 DB:0`
