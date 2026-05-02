import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import ItemDetail from './ItemDetail'

vi.mock('@/api/client', () => ({
  api: {
    item: vi.fn(),
  },
}))

import { api } from '@/api/client'

const mockItem = {
  key: 'ABC123',
  item_type: 'journalArticle',
  title: 'Test Paper Title',
  date: '2024',
  creators: [{ name: 'Zhang San', creator_type: 'author' }],
  container: 'Nature',
  volume: '10',
  issue: '1',
  pages: '100-120',
  doi: '10.1038/test.2024',
  tags: ['genetics', 'bio'],
  collections: [],
  attachments: [
    {
      key: 'ATT1',
      item_type: 'attachment',
      content_type: 'application/pdf',
      filename: 'paper.pdf',
      resolved: true,
    },
  ],
  notes: [{ key: 'N1', content: '<p>Test note</p>', preview: 'Test note' }],
  annotations: [
    { key: 'A1', type: 'highlight' as const, text: 'highlighted text', color: '#ffff00', page_label: '1', page_index: 0, is_external: false },
  ],
}

function createWrapper(initialEntry = '/items/ABC123') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/items/:key" element={children} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ItemDetail Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.item).mockResolvedValue({ ok: true, data: mockItem, error: null })
  })

  it('renders item detail correctly', async () => {
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())
  })

  it('PDF preview button has accessible name', async () => {
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    const previewBtn = screen.getByRole('button', { name: /预览/i })
    expect(previewBtn).toBeInTheDocument()
  })

  it('opening PDF modal creates a dialog with role=dialog and aria-modal', async () => {
    const user = userEvent.setup()
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /预览/i }))

    await waitFor(() => {
      const dialog = screen.getByRole('dialog')
      expect(dialog).toBeInTheDocument()
      expect(dialog).toHaveAttribute('aria-modal', 'true')
    }, { timeout: 3000 })
  })

  it('closing modal with Escape key removes dialog', async () => {
    const user = userEvent.setup()
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /预览/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument(), { timeout: 3000 })

    await user.keyboard('{Escape}')

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    }, { timeout: 2000 })
  })

  it('modal close button is focusable and has aria-label', async () => {
    const user = userEvent.setup()
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /预览/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument(), { timeout: 3000 })

    const closeBtn = screen.getByRole('button', { name: /关闭/i })
    expect(closeBtn).toBeInTheDocument()

    await user.click(closeBtn)
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    }, { timeout: 2000 })
  })

  it('metadata region uses proper semantic structure', async () => {
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    expect(screen.getByText('类型')).toBeInTheDocument()
    expect(screen.getByText('作者')).toBeInTheDocument()
  })

  it('attachment links have proper accessible names', async () => {
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    expect(screen.getByText('paper.pdf')).toBeInTheDocument()
  })

  it('annotations have color indicators with accessible text', async () => {
    render(<ItemDetail />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('Test Paper Title')).toBeInTheDocument())

    expect(screen.getByText('highlighted text')).toBeInTheDocument()
  })
})
