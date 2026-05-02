import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import Library from './Library'

vi.mock('@/api/client', () => ({
  api: {
    items: vi.fn(),
    collections: vi.fn(),
  },
}))

import { api } from '@/api/client'

const mockItems = [
  {
    key: 'ITEM1', title: 'Paper A', item_type: 'journalArticle', date: '2024',
    creators: [], tags: [], collections: [], attachments: [], notes: [], annotations: [],
  },
  {
    key: 'ITEM2', title: 'Paper B', item_type: 'journalArticle', date: '2025',
    creators: [], tags: [], collections: [], attachments: [], notes: [], annotations: [],
  },
]

const mockCollections = [
  { key: 'COL1', name: '遗传学', num_items: 10 },
  { key: 'COL2', name: '植物学', num_items: 5 },
]

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('Library', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.items).mockResolvedValue({ ok: true, data: mockItems, error: null, meta: { total: 100 } })
    vi.mocked(api.collections).mockResolvedValue({ ok: true, data: mockCollections, error: null, meta: {} })
  })

  it('renders collections and items on mount', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Paper A')).toBeInTheDocument())
    expect(screen.getByText('全部文献')).toBeInTheDocument()
    expect(screen.getByText('遗传学')).toBeInTheDocument()
    expect(screen.getByText('植物学')).toBeInTheDocument()
  })

  it('calls api.items with collection param when a collection is clicked', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())

    await user.click(screen.getByText('遗传学'))

    await waitFor(() => {
      const calls = vi.mocked(api.items).mock.calls
      const lastCall = calls[calls.length - 1]?.[0]
      expect(lastCall).toEqual(expect.objectContaining({ collection: 'COL1' }))
    })
  })

  it('highlights selected collection', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())
    await user.click(screen.getByText('遗传学'))

    const selectedEl = screen.getByText('遗传学').closest('div')
    expect(selectedEl?.className).toContain('from-red-50')
  })

  it('resets to all items when clicking 全部文献', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())
    await user.click(screen.getByText('遗传学'))
    await waitFor(() => expect(api.items).toHaveBeenCalledTimes(2))

    await user.click(screen.getByText('全部文献'))

    await waitFor(() => {
      const calls = vi.mocked(api.items).mock.calls
      const lastCall = calls[calls.length - 1]?.[0]
      expect(lastCall?.collection).toBeUndefined()
    })
  })

  it('resets page to 0 when switching collection', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())

    // go to page 2
    const nextBtns = screen.getAllByRole('button', { name: /下一页/ })
    await user.click(nextBtns[0])
    await waitFor(() => expect(api.items).toHaveBeenCalledTimes(2))

    // click collection should reset page
    await user.click(screen.getByText('植物学'))
    await waitFor(() => {
      const calls = vi.mocked(api.items).mock.calls
      const lastCall = calls[calls.length - 1]?.[0]
      expect(lastCall).toEqual(expect.objectContaining({ collection: 'COL2', start: 0 }))
    })
  })
})
