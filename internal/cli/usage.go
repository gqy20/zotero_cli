package cli

const (
	usageFind        = "usage: zot find <query> [--json] [--full] [--snippet] [--fulltext] [--fulltext-any] [--include-fields FIELD[,FIELD...]] [--item-type TYPE] [--no-type TYPE] [--tag TAG ...] [--tag-any] [--tag-contains WORD ...] [--exclude-tag TAG ...] [--collection KEY ...] [--no-collection KEY ...] [--date-after YYYY[-MM[-DD]]] [--date-before YYYY[-MM[-DD]]] [--modified-within DURATION] [--added-since DURATION] [--limit N] [--has-pdf] [--attachment-name TEXT] [--attachment-path TEXT] [--attachment-type TEXT] [--qmode titleCreatorYear|everything] [--include-trashed] | zot find --all [--json] [--full] [--snippet] [--include-fields FIELD[,FIELD...]] [--item-type TYPE] [--no-type TYPE] [--tag TAG ...] [--tag-any] [--tag-contains WORD ...] [--exclude-tag TAG ...] [--collection KEY ...] [--no-collection KEY ...] [--date-after YYYY[-MM[-DD]]] [--date-before YYYY[-MM[-DD]]] [--modified-within DURATION] [--added-since DURATION] [--limit N] [--has-pdf] [--attachment-name TEXT] [--attachment-path TEXT] [--attachment-type TEXT] [--qmode titleCreatorYear|everything] [--include-trashed]"
	usageShow        = "usage: zot show <item-key> [--json] [--snippet]"
	usageExtractText = "usage: zot extract-text <item-key> [--json]"
	usageRelate      = "usage: zot relate <item-key> [--json] [--aggregate] [--predicate PRED]"
	usageCite        = "usage: zot cite <item-key> [--format citation|bib] [--style STYLE] [--locale LOCALE] [--json]"
	usageExport      = "usage: zot export <query> [--limit N] [--format bib|bibtex|biblatex|csljson|ris] [--json] | zot export --item-key KEY [--format bib|bibtex|biblatex|csljson|ris] [--json] | zot export --collection KEY [--format bib|bibtex|biblatex|csljson|ris] [--json]"
	usageCollections = "usage: zot collections [--limit N] [--json]"
	usageNotes       = "usage: zot notes [--query QUERY] [--limit N] [--json]"
	usageTags        = "usage: zot tags [--limit N] [--json]"
	usageSearches    = "usage: zot searches [--limit N] [--json]"
	usageDeleted     = "usage: zot deleted [--json]"
	usageStats       = "usage: zot stats [--json]"
	usageVersions    = "usage: zot versions <collections|searches|items|items-top> --since N [--include-trashed] [--if-modified-since-version N] [--json]"
	usageSchema      = `usage: zot schema <subcommand> [args] [--json]

Introspect Zotero metadata schema.

Subcommands:
  types                     List all Zotero item types
  fields                    List all Zotero item fields
  creator-types             List all Zotero creator fields (roles)
  fields-for <type>         List valid fields for a specific item type
  creator-types-for <type>  List valid creator roles for a specific item type
  template <type>           Show JSON template for creating a new item

Examples:
  zot schema types                          # List all item types
  zot schema types --json                   # JSON output
  zot schema fields-for journalArticle      # Fields for journal articles
  zot schema template book --json           # Template for a new book
  zot schema creator-types-for artwork      # Creator roles for artwork
`
	usageItemTypes            = "usage: zot schema types [--json]"
	usageItemFields           = "usage: zot schema fields [--json]"
	usageCreatorFields        = "usage: zot schema creator-types [--json]"
	usageItemTypeFields       = "usage: zot schema fields-for <item-type> [--json]"
	usageItemTypeCreatorTypes = "usage: zot schema creator-types-for <item-type> [--json]"
	usageItemTemplate         = "usage: zot schema template <item-type> [--json]"
	usageKeyInfo              = "usage: zot key-info <api-key> [--json]"
	usageGroups               = "usage: zot groups [--json]"
	usageTrash                = "usage: zot trash [--limit N] [--json]"
	usageCollectionsTop       = "usage: zot collections-top [--json]"
	usagePublications         = "usage: zot publications [--limit N] [--json]"
	usageCreateItem           = "usage: zot create-item (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]"
	usageUpdateItem           = "usage: zot update-item <item-key> (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]"
	usageDeleteItem           = "usage: zot delete-item <item-key> --if-unmodified-since-version N [--json] [-y|--yes]"
	usageAddTag               = "usage: zot add-tag --items KEY1,KEY2 --tag TAG [--if-unmodified-since-version N] [--json]"
	usageRemoveTag            = "usage: zot remove-tag --items KEY1,KEY2 --tag TAG [--if-unmodified-since-version N] [--json]"
	usageCreateCollection     = "usage: zot create-collection (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]"
	usageUpdateCollection     = "usage: zot update-collection <collection-key> (--data JSON | --from-file PATH) [--if-unmodified-since-version N] [--json]"
	usageDeleteCollection     = "usage: zot delete-collection <collection-key> --if-unmodified-since-version N [--json] [-y|--yes]"
	usageCreateSearch         = "usage: zot create-search (--data JSON | --from-file PATH) --if-unmodified-since-version N [--json]"
	usageUpdateSearch         = "usage: zot update-search <search-key> (--data JSON | --from-file PATH) [--if-unmodified-since-version N] [--json]"
	usageDeleteSearch         = "usage: zot delete-search <search-key> --if-unmodified-since-version N [--json] [-y|--yes]"
	usageIndex                = "usage: zot index build [--force] [--workers N] [--json]"
	usageOverview             = `usage: zot overview [--json]

One-shot library overview for agents. Returns stats, top collections,
top tags, recent items, and FTS index status in a single call.

Examples:
  zot overview                          # Text summary
  zot overview --json                     # Full structured data for agents

This command is designed for AI agents that need a quick library
snapshot without making multiple API calls.`
	usageAnnotate       = "usage: zot annotate <item-key> (--text TEXT | --page N (--rect x0,y0,x1,y2 | --point x,y)) [--color COLOR] [--comment TEXT] [--type TYPE] [--clear] [--author AUTHOR] [--json]"
	usageOpen           = "usage: zot open <item-key> [--page N]"
	usageSelect         = "usage: zot select <item-key>"
	usageAnnotations    = "usage: zot annotations <item-key> [--json] [--clear] [--page N] [--type TYPE] [--author AUTHOR]"
	usageExtractFigures = "usage: zot extract-figures <item-key> [...] [--output-dir DIR] [--json] [--workers N]"
)
