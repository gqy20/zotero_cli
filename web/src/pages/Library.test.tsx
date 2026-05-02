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

// --- Accessibility Tests ---

describe('Library Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.items).mockResolvedValue({ ok: true, data: mockItems, error: null, meta: { total: 100 } })
    vi.mocked(api.collections).mockResolvedValue({ ok: true, data: mockCollections, error: null, meta: {} })
  })

  it('collection items are keyboard accessible (role=button, tabIndex=0)', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())

    const collectionButtons = screen.getAllByRole('button', { name: /遗传学|植物学|全部文献/ })
    expect(collectionButtons.length).toBeGreaterThanOrEqual(3)
    collectionButtons.forEach(btn => {
      expect(btn).toHaveAttribute('tabindex', '0')
    })
  })

  it('collection items can be activated via Enter key', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('遗传学')).toBeInTheDocument())

    const colBtn = screen.getByRole('button', { name: /遗传学/ })
    colBtn.focus()
    await user.keyboard('{Enter}')

    await waitFor(() => {
      const calls = vi.mocked(api.items).mock.calls
      const lastCall = calls[calls.length - 1]?.[0]
      expect(lastCall).toEqual(expect.objectContaining({ collection: 'COL1' }))
    })
  })

  it('collection items can be activated via Space key', async () => {
    const user = userEvent.setup()
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('植物学')).toBeInTheDocument())

    const colBtn = screen.getByRole('button', { name: /植物学/ })
    colBtn.focus()
    await user.keyboard(' ')

    await waitFor(() => {
      const calls = vi.mocked(api.items).mock.calls
      const lastCall = calls[calls.length - 1]?.[0]
      expect(lastCall).toEqual(expect.objectContaining({ collection: 'COL2' }))
    })
  })

  it('pagination buttons have aria-labels', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Paper A')).toBeInTheDocument())

    const prevBtn = screen.getByRole('button', { name: /上一页/ })
    const nextBtn = screen.getByRole('button', { name: /下一页/ })
    expect(prevBtn).toBeInTheDocument()
    expect(nextBtn).toBeInTheDocument()
  })

  it('table has proper column headers with scope', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Paper A')).toBeInTheDocument())

    const table = screen.getByRole('table')
    expect(table).toBeInTheDocument()

    const headers = screen.getAllByRole('columnheader')
    expect(headers.length).toBe(4)
    headers.forEach(th => {
      expect(th).toHaveAttribute('scope', 'col')
    })
  })

  it('sidebar has navigation landmark role', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Paper A')).toBeInTheDocument())

    // Sidebar contains a <nav> with aria-label for collections
    const collectionNav = screen.getByRole('navigation', { name: /集合|分类/ })
    expect(collectionNav).toBeInTheDocument()
  })

  it('search input has associated label', async () => {
    render(<Library />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Paper A')).toBeInTheDocument())

    const searchInput = screen.getByPlaceholderText('搜索文献...')
    expect(searchInput).toBeInTheDocument()
    expect(searchInput.closest('div')?.querySelector('label') || searchInput.getAttribute('aria-label') || searchInput.id).toBeTruthy()
  })
})

// --- Error Recovery Tests ---

describe('Library Error Recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows user-friendly error message when items API fails', async () => {
    vi.mocked(api.items).mockRejectedValue(new Error('Network error'))
    vi.mocked(api.collections).mockResolvedValue({ ok: true, data: mockCollections, error: null, meta: {} })

    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => {
      const alert = screen.getByRole('alert')
      expect(alert).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('shows retry button on API failure', async () => {
    vi.mocked(api.items).mockRejectedValue(new Error('Network error'))
    vi.mocked(api.collections).mockResolvedValue({ ok: true, data: mockCollections, error: null, meta: {} })

    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => {
      const retryBtn = screen.getByRole('button', { name: /重试|retry/i })
      expect(retryBtn).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('retry button is enabled and clickable on error state', async () => {
    vi.mocked(api.items).mockRejectedValue(new Error('Network error'))
    vi.mocked(api.collections).mockResolvedValue({ ok: true, data: mockCollections, error: null, meta: {} })

    render(<Library />, { wrapper: createWrapper() })

    await waitFor(() => {
      const retryBtn = screen.getByRole('button', { name: /重试/i })
      expect(retryBtn).toBeInTheDocument()
      expect(retryBtn).toBeEnabled()
    }, { timeout: 3000 })
  })
})
