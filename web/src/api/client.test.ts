import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api } from './client'

// Mock fetch globally
const mockFetch = vi.fn()
;((globalThis as any).fetch) = mockFetch

describe('API Client', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    // Default successful response
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ ok: true, data: [], error: null, meta: {} }),
    })
  })

  it('calls correct URL for stats', async () => {
    await api.stats()
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/stats', expect.objectContaining({
      headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
    }))
  })

  it('calls correct URL for items with params', async () => {
    await api.items({ limit: 10, start: 5 })
    const url = mockFetch.mock.calls[0][0]
    expect(url).toContain('/api/v1/items?limit=10&start=5')
  })

  it('throws on non-ok response', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Error',
    })
    await expect(api.stats()).rejects.toThrow('API error 500')
  })

  it('parses JSON response correctly', async () => {
    const testData = { total_items: 42 }
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ ok: true, data: testData, error: null, meta: {} }),
    })
    const result = await api.stats()
    expect(result.ok).toBe(true)
    expect(result.data).toEqual(testData)
  })
})
