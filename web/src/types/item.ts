export interface Creator {
  name: string
  creator_type: string
}

export interface Attachment {
  key: string
  item_type: string
  title?: string
  content_type?: string
  link_mode?: string
  filename?: string
  zotero_path?: string
  resolved_path?: string
  resolved: boolean
}

export interface Note {
  key: string
  parent_item_key?: string
  content?: string
  preview?: string
}

export interface Annotation {
  key: string
  type: 'highlight' | 'note' | 'image' | 'ink'
  text?: string
  comment?: string
  color?: string
  page_label?: string
  page_index?: number
  position?: string
  sort_index?: string
  is_external: boolean
  date_added?: string
}

export interface Collection {
  key: string
  name: string
  num_items?: number
}

export interface Item {
  version?: number
  key: string
  item_type: string
  title: string
  date: string
  creators: Creator[]
  matched_on?: string[]
  full_text_preview?: string
  container?: string
  volume?: string
  issue?: string
  pages?: string
  doi?: string
  url?: string
  tags: string[]
  collections: Collection[]
  attachments: Attachment[]
  notes: Note[]
  annotations: Annotation[]
}

export interface LibraryStats {
  library_type: string
  library_id: string
  total_items: number
  total_collections: number
  total_searches: number
  last_library_version?: number
}

export interface Tag {
  name: string
  num_items?: number
}

export interface OverviewData {
  stats: LibraryStats
  recent_items: Item[]
}
