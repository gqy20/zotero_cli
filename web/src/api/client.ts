import type { ApiResponse } from '@/types/api'
import { logger, logApiRequest, logApiResponse } from '@/lib/logger'

const BASE_URL = '/api/v1'

async function request<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
  const url = `${BASE_URL}${path}`
  const start = performance.now()
  logApiRequest(options?.method || 'GET', url, options)

  try {
    const res = await fetch(url, {
      headers: { 'Content-Type': 'application/json', ...options?.headers },
      ...options,
    })
    const durationMs = performance.now() - start
    logApiResponse(options?.method || 'GET', url, res.status, durationMs)

    if (!res.ok) {
      throw new Error(`API error ${res.status}: ${res.statusText}`)
    }
    return res.json()
  } catch (err) {
    logger.error('API request failed', { path, error: err instanceof Error ? err.message : String(err) })
    throw err
  }
}

export const api = {
  get: <T>(path: string) => request<T>(path),

  stats: () => request<import('@/types/item').LibraryStats>('/stats'),
  overview: () => request<import('@/types/item').OverviewData>('/overview'),
  items: (params?: Record<string, string | number | boolean>) => {
    const query = params
      ? '?' + new URLSearchParams(
          Object.entries(params)
            .filter(([, v]) => v !== undefined && v !== '')
            .map(([k, v]) => [k, String(v)])
        ).toString()
      : ''
    return request<import('@/types/item').Item[]>(`/items${query}`)
  },
  item: (key: string) => request<import('@/types/item').Item>(`/items/${key}`),
  collections: () => request<import('@/types/item').Collection[]>('/collections'),
  tags: () => request<import('@/types/item').Tag[]>('/tags'),
  notes: () => request<import('@/types/item').Note[]>('/notes'),
}
