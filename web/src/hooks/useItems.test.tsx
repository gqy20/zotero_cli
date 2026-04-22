import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import useItems from './useItems'

vi.mock('@/api/client', () => ({
  api: {
    items: vi.fn(),
  },
}))

import { api } from '@/api/client'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useItems', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls api.items on mount', async () => {
    const mockItems = [{ key: 'ABC123', title: 'Test Paper', item_type: 'journalArticle', date: '', creators: [], tags: [], collections: [], attachments: [], notes: [], annotations: [] }]
    vi.mocked(api.items).mockResolvedValueOnce({ ok: true, data: mockItems, error: null, meta: {} })

    const { result } = renderHook(() => useItems(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(api.items).toHaveBeenCalled()
    expect(result.current.items).toEqual(mockItems)
  })

  it('passes params to api.items', async () => {
    vi.mocked(api.items).mockResolvedValueOnce({ ok: true, data: [], error: null, meta: {} })

    renderHook(() => useItems({ start: 25, limit: 25 }), { wrapper: createWrapper() })

    await waitFor(() => expect(api.items).toHaveBeenCalled())
    const callArgs = vi.mocked(api.items).mock.calls[0][0]
    expect(callArgs).toEqual(expect.objectContaining({ start: 25, limit: 25 }))
  })

  it('returns empty array when response is not ok', async () => {
    vi.mocked(api.items).mockResolvedValueOnce({ ok: false, data: [], error: 'fail', meta: {} })

    const { result } = renderHook(() => useItems(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.items).toEqual([])
  })

  it('exposes isLoading state while loading', () => {
    vi.mocked(api.items).mockReturnValue(new Promise(() => {}))

    const { result } = renderHook(() => useItems(), { wrapper: createWrapper() })
    expect(result.current.isLoading).toBe(true)
  })

  it('exposes total from meta', async () => {
    vi.mocked(api.items).mockResolvedValueOnce({
      ok: true, data: [], error: null, meta: { total: 42 },
    })

    const { result } = renderHook(() => useItems(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.total).toBe(42)
  })

  it('does not call api.items when disabled', async () => {
    vi.mocked(api.items).mockResolvedValueOnce({ ok: true, data: [], error: null, meta: {} })

    renderHook(() => useItems({}, false), { wrapper: createWrapper() })

    expect(api.items).not.toHaveBeenCalled()
  })
})
