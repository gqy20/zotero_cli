import type { ApiResponse } from '@/types/api'

const BASE_URL = '/api/v1'

async function request<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
  const url = `${BASE_URL}${path}`
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })
  if (!res.ok) {
    throw new Error(`API error ${res.status}: ${res.statusText}`)
  }
  return res.json()
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
