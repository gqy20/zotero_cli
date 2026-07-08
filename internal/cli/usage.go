package cli

// CommandSpec 是 zot 所有顶层命令的展示元数据单一来源。
//
// 顶层 -h / 子命令 -h / 错误信息 / schema 子分支都从这里读。
// 新增命令 = 在 commandRegistry 里加一行 + 在 cli.go 的 dispatch switch 加一个 case。
//
// 注：Run 字段不放在这里，以避免 commandRegistry 与 runXxx 方法形成
// 初始化/调用环；命令行为分发由 cli.go 的 dispatch 单独负责。
type CommandSpec struct {
	Name     string   // 顶层命令名，如 "create-item"
	Short    string   // 顶层 -h 列表里那一行的右半边
	Long     string   // 子命令 -h 输出的完整 usage 块（可含 Examples / See also / Hybrid 路由说明等）
	Category Category // 顶层 -h 的分组
	SeeAlso  []string // 顶层 -h 末尾交叉引用（出现在短描述尾部）
	Hidden   bool     // 隐藏：不进顶层 -h 列表，但仍可调用（用于已废弃命令的向后兼容）
}

// Category 把顶层 -h 的命令按风险和意图分组渲染。
type Category int

const (
	CatRead Category = iota
	CatAnnotate
	CatWrite
	CatDestructive
	CatSetup
)

func (c Category) String() string {
	switch c {
	case CatRead:
		return "Read"
	case CatAnnotate:
		return "Annotate"
	case CatWrite:
		return "Write"
	case CatDestructive:
		return "Destructive"
	case CatSetup:
		return "Setup"
	}
	return ""
}

// commandRegistry 是所有顶层命令的注册表（单一来源）。
// 顺序即顶层 -h 的渲染顺序；分组渲染时按 Category 聚合。
var commandRegistry = []CommandSpec{
	{
		Name:     "version",
		Category: CatSetup,
		Short:    "Show CLI version",
		Long:     "usage: zot version [--check] [--json]\n\n--check queries GitHub for the latest release and prints an upgrade hint.",
	},
	{
		Name:     "init",
		Category: CatSetup,
		Short:    "Initialize ~/.zot/.env (streamlined setup with mode selection)",
		Long:     usageInit,
	},
	{
		Name:     "config",
		Category: CatSetup,
		Short:    "Inspect or validate ~/.zot/.env",
		Long: `usage: zot config <subcommand>

Subcommands:
  path       Print config file path
  show       Show active config with masked secrets
  validate   Validate library_id and api_key against Zotero`,
	},
	{
		Name:     "index",
		Category: CatSetup,
		Short:    "Build or manage full-text search index",
		Long:     "usage: zot index build [--force] [--workers N] [--json]\n\nBuilds the FTS5 index over local PDF text. Requires local/hybrid mode.",
	},
	{
		Name:     "setup",
		Category: CatSetup,
		Short:    "Old PDF setup helper (use `zot init --pdf`)",
		Long:     "usage: zot setup pdf-extract [--check]\n\nDeprecated. Use `zot init --pdf` to install PyMuPDF, or `zot init --check-pdf` to check status.",
		Hidden:   true,
	},
	{
		Name:     "server",
		Category: CatSetup,
		Short:    "Start the HTTP API server (remote-mode backend)",
		Long:     usageServer,
	},
	{
		Name:     "sync",
		Category: CatSetup,
		Short:    "Pull the remote library (sqlite + storage) for offline local use",
		Long:     usageSync,
	},

	{
		Name:     "find",
		Category: CatRead,
		Short:    "Search items in the configured Zotero library",
		Long:     usageFind,
	},
	{
		Name:     "show",
		Category: CatRead,
		Short:    "Show item details",
		Long:     usageShow,
	},
	{
		Name:     "supplements",
		Category: CatRead,
		Short:    "Find local supplementary/data attachments",
		Long:     usageSupplements,
	},
	{
		Name:     "inspect-attachment",
		Category: CatRead,
		Short:    "Preview local spreadsheet attachments",
		Long:     usageInspectAttachment,
	},
	{
		Name:     "extract-text",
		Category: CatRead,
		Short:    "Extract text from local PDF attachments",
		Long:     usageExtractText,
	},
	{
		Name:     "extract-figures",
		Category: CatRead,
		Short:    "Extract scientific figures from PDF attachments",
		Long:     usageExtractFigures,
	},
	{
		Name:     "open",
		Category: CatRead,
		Short:    "Open a PDF attachment in the default viewer",
		Long:     usageOpen,
	},
	{
		Name:     "select",
		Category: CatRead,
		Short:    "Select an item in the Zotero UI",
		Long:     usageSelect,
	},
	{
		Name:     "annotations",
		Category: CatRead,
		Short:    "List PDF annotations (highlights, notes, underlines)",
		Long:     usageAnnotations,
	},
	{
		Name:     "abstract",
		Category: CatRead,
		Short:    "View item abstract(s); supports multiple keys",
		Long:     usageAbstract,
	},
	{
		Name:     "relate",
		Category: CatRead,
		Short:    "Show explicit item relations",
		Long:     usageRelate,
	},
	{
		Name:     "export",
		Category: CatRead,
		Short:    "Export bibliography entries",
		Long:     usageExport,
	},
	{
		Name:     "collections",
		Category: CatRead,
		Short:    "List collections",
		Long:     usageCollections,
	},
	{
		Name:     "notes",
		Category: CatRead,
		Short:    "List notes (see also: create-item to add)",
		Long:     usageNotes,
		SeeAlso:  []string{"create-item"},
	},
	{
		Name:     "tags",
		Category: CatRead,
		Short:    "List tags",
		Long:     usageTags,
	},
	{
		Name:     "searches",
		Category: CatRead,
		Short:    "List saved searches",
		Long:     usageSearches,
	},
	{
		Name:     "deleted",
		Category: CatRead,
		Short:    "Show deleted object keys",
		Long:     usageDeleted,
	},
	{
		Name:     "stats",
		Category: CatRead,
		Short:    "Show library item, collection, and search counts",
		Long:     usageStats,
	},
	{
		Name:     "changes",
		Category: CatRead,
		Short:    "Show changed objects since a library version",
		Long:     usageChanges,
	},
	{
		Name:     "schema",
		Category: CatRead,
		Short:    "Introspect Zotero metadata schema (types, fields, templates)",
		Long:     usageSchema,
	},
	{
		Name:     "overview",
		Category: CatRead,
		Short:    "One-shot library overview (stats, collections, tags, recent items)",
		Long:     usageOverview,
	},
	{
		Name:     "key-info",
		Category: CatRead,
		Short:    "Show the owner and privileges for an API key",
		Long:     usageKeyInfo,
	},
	{
		Name:     "groups",
		Category: CatRead,
		Short:    "List groups accessible to a user",
		Long:     usageGroups,
	},
	{
		Name:     "trash",
		Category: CatRead,
		Short:    "List items currently in the trash",
		Long:     usageTrash,
	},
	{
		Name:     "collections-top",
		Category: CatRead,
		Short:    "List only top-level collections",
		Long:     usageCollectionsTop,
	},
	{
		Name:     "publications",
		Category: CatRead,
		Short:    "List items in My Publications",
		Long:     usagePublications,
	},

	{
		Name:     "annotate",
		Category: CatAnnotate,
		Short:    "Add highlights/underlines/notes to a PDF",
		Long:     usageAnnotate,
	},

	{
		Name:     "create-item",
		Category: CatWrite,
		Short:    "Create a new item (e.g. note) from JSON data",
		Long:     usageCreateItem,
		SeeAlso:  []string{"notes", "update-item"},
	},
	{
		Name:     "update-item",
		Category: CatWrite,
		Short:    "Update an existing item from JSON data",
		Long:     usageUpdateItem,
		SeeAlso:  []string{"create-item"},
	},
	{
		Name:     "add-tag",
		Category: CatWrite,
		Short:    "Add a tag to multiple items",
		Long:     usageAddTag,
	},
	{
		Name:     "remove-tag",
		Category: CatWrite,
		Short:    "Remove a tag from multiple items",
		Long:     usageRemoveTag,
	},
	{
		Name:     "create-collection",
		Category: CatWrite,
		Short:    "Create a collection from JSON data",
		Long:     usageCreateCollection,
	},
	{
		Name:     "update-collection",
		Category: CatWrite,
		Short:    "Update a collection from JSON data",
		Long:     usageUpdateCollection,
	},
	{
		Name:     "create-search",
		Category: CatWrite,
		Short:    "Create a saved search from JSON data",
		Long:     usageCreateSearch,
	},
	{
		Name:     "update-search",
		Category: CatWrite,
		Short:    "Update a saved search from JSON data",
		Long:     usageUpdateSearch,
	},

	{
		Name:     "delete-item",
		Category: CatDestructive,
		Short:    "Delete an item using a version precondition",
		Long:     usageDeleteItem,
		SeeAlso:  []string{"create-item", "update-item"},
	},
	{
		Name:     "delete-collection",
		Category: CatDestructive,
		Short:    "Delete a collection using a version precondition",
		Long:     usageDeleteCollection,
	},
	{
		Name:     "delete-search",
		Category: CatDestructive,
		Short:    "Delete a saved search using a version precondition",
		Long:     usageDeleteSearch,
	},
}

// lookupCommand 返回指定 name 的 spec；找不到返回 nil。
func lookupCommand(name string) *CommandSpec {
	for i := range commandRegistry {
		if commandRegistry[i].Name == name {
			return &commandRegistry[i]
		}
	}
	return nil
}

// schemaSubUsages 把 schema 子命令名映射到各自的 sub-usage 文本。
// runSchema 用它打印"子命令缺少必填参数"或"未知子命令"时的针对性提示，
// 避免在 runSchema 内部硬编码 "usage: zot schema xxx" 字符串。
var schemaSubUsages = map[string]string{
	"types":             usageItemTypes,
	"fields":            usageItemFields,
	"creator-types":     usageCreatorFields,
	"fields-for":        usageItemTypeFields,
	"creator-types-for": usageItemTypeCreatorTypes,
	"template":          usageItemTemplate,
}
