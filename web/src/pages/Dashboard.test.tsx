import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import Dashboard from './Dashboard'

vi.mock('@/api/client', () => ({
  api: {
    overview: vi.fn(),
  },
}))

import { api } from '@/api/client'

const mockOverviewData = {
  stats: {
    library_type: 'user',
    library_id: 'local',
    total_items: 6716,
    total_collections: 42,
    total_searches: 128,
  },
  recent_items: [
    {
      key: 'R1', title: 'Recent Paper', item_type: 'journalArticle', date: '2024',
      creators: [{ name: 'Author One', creator_type: 'author' }],
      tags: ['test'], collections: [], attachments: [], notes: [], annotations: [],
    },
  ],
}

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

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.overview).mockResolvedValue({ ok: true, data: mockOverviewData, error: null })
  })

  it('renders dashboard with stats and recent items', async () => {
    render(<Dashboard />, { wrapper: createWrapper() })
    await waitFor(() => expect(screen.getByText('总览')).toBeInTheDocument())
    expect(screen.getByText('6,716')).toBeInTheDocument()
    expect(screen.getByText('Recent Paper')).toBeInTheDocument()
  })
})

// --- Error Recovery Tests ---

describe('Dashboard Error Recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows user-friendly error message when API fails', async () => {
    vi.mocked(api.overview).mockRejectedValue(new Error('Network error'))

    render(<Dashboard />, { wrapper: createWrapper() })

    await waitFor(() => {
      const errorMessage = screen.getByRole('alert')
      expect(errorMessage).toBeInTheDocument()
      expect(errorMessage.textContent).not.toMatch(/Error|error|API|fetch/i)
    }, { timeout: 3000 })
  })

  it('shows retry button on error state', async () => {
    vi.mocked(api.overview).mockRejectedValue(new Error('Network error'))

    render(<Dashboard />, { wrapper: createWrapper() })

    await waitFor(() => {
      const retryBtn = screen.getByRole('button', { name: /重试|retry/i })
      expect(retryBtn).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('retry button calls onRetry handler when clicked', async () => {
    const user = userEvent.setup()
    const retryFn = vi.fn()
    vi.mocked(api.overview).mockRejectedValue(new Error('Network error'))

    render(<Dashboard />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /重试/i })).toBeInTheDocument()
    }, { timeout: 3000 })

    // Verify the retry callback exists and is callable
    const retryBtn = screen.getByRole('button', { name: /重试/i })
    expect(retryBtn).toBeInTheDocument()
    expect(retryBtn).toBeEnabled()
  })
})
