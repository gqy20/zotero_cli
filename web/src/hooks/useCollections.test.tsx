import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import useCollections from './useCollections'

vi.mock('@/api/client', () => ({
  api: {
    collections: vi.fn(),
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

describe('useCollections', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches collections from api.collections', async () => {
    const mockCollections = [
      { key: 'COL1', name: 'Papers', num_items: 10 },
      { key: 'COL2', name: 'Books', num_items: 3 },
    ]
    vi.mocked(api.collections).mockResolvedValueOnce({ ok: true, data: mockCollections, error: null, meta: {} })

    const { result } = renderHook(() => useCollections(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.collections).toEqual(mockCollections)
  })

  it('returns empty array on error', async () => {
    vi.mocked(api.collections).mockResolvedValueOnce({ ok: false, data: [], error: 'err', meta: {} })

    const { result } = renderHook(() => useCollections(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.collections).toEqual([])
  })
})
