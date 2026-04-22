# 领域模型

核心数据结构定义。完整 Go 类型定义见 `internal/domain/types.go`。

> 架构概览见 [overview](./overview.md)，设计决策见 [decisions](./decisions.md)。

---

## Item

```go
type Item struct {
    Key                  string           `json:"key"`
    Version              int              `json:"version,omitempty"`
    ItemType             string           `json:"item_type"`
    Title                string           `json:"title"`
    Date                 string           `json:"date"`
    Creators             []Creator        `json:"creators"`
    Tags                 []string         `json:"tags,omitempty"`
    Collections          []Collection     `json:"collections,omitempty"`
    Attachments          []Attachment     `json:"attachments,omitempty"`
    Notes                []Note           `json:"notes,omitempty"`
    Annotations          []Annotation      `json:"annotations,omitempty"`
    // 检索元字段
    SearchScore          int              `json:"-"`
    MatchedOn            []string         `json:"matched_on,omitempty"`
    FullTextPreview      string           `json:"full_text_preview,omitempty"`
    SnippetAttachmentKey string           `json:"-"`
    MatchedChunk         *MatchedChunkInfo `json:"matched_chunk,omitempty"`
    // 出版物字段
    Container            string           `json:"container,omitempty"`
    Volume               string           `json:"volume,omitempty"`
    Issue                string           `json:"issue,omitempty"
    Pages                string           `json:"pages,omitempty"`
    DOI                  string           `json:"doi,omitempty"`
    URL                  string           `json:"url,omitempty"`
}
```

## Annotation

```go
type Annotation struct {
    Key        string `json:"key"`
    Type       string `json:"type"`          // highlight | note | image | ink
    Text       string `json:"text,omitempty"`
    Comment    string `json:"comment,omitempty"`
    Color      string `json:"color,omitempty"` // "#ffd400"
    PageLabel  string `json:"page_label,omitempty"`
    PageIndex  int    `json:"page_index"`
    Position   string `json:"position,omitempty"`
    SortIndex  string `json:"sort_index,omitempty"`
    IsExternal bool   `json:"is_external"`
    DateAdded  string `json:"date_added,omitempty"`
}
```

## Attachment

```go
type Attachment struct {
    Key          string `json:"key"`
    ItemType     string `json:"item_type"`
    ContentType  string `json:"content_type,omitempty"`
    LinkMode     string `json:"link_mode,omitempty"`
    Filename     string `json:"filename"`
    ZoteroPath   string `json:"zotero_path,omitempty"`
    ResolvedPath string `json:"resolved_path,omitempty"`
    Resolved     bool   `json:"resolved,omitempty"`
}
```

## 其他核心类型

| 类型 | 用途 |
|------|------|
| `Creator` | 作者（Name + CreatorType） |
| `Collection` | 收藏夹（Key + Name） |
| `Note` | 子笔记 |
| `Relation` | 条目关系（Predicate + Direction + Target ItemRef） |
| `FindOptions` | 检索参数（query/filters/pagination） |
| `LibraryStats` | 库统计（items/collections/searches 计数） |
