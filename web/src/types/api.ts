export interface ApiResponse<T> {
  ok: boolean
  data: T
  error: string | null
  meta: ApiMeta
}

export interface ApiMeta {
  read_source?: string
  total?: number
  sqlite_fallback?: boolean
}

export interface PaginatedMeta extends ApiMeta {
  total: number
}

export interface PaginatedResponse<T> extends ApiResponse<T[]> {
  meta: PaginatedMeta
}
