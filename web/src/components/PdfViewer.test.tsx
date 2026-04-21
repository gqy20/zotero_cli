import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PdfViewer from './PdfViewer'

const { mockGetPage, mockGetDocument } = vi.hoisted(() => {
  const mockGetPage = vi.fn().mockResolvedValue({
    getViewport: vi.fn().mockReturnValue({ width: 100, height: 150 }),
    render: vi.fn().mockResolvedValue(undefined),
  })
  const mockGetDocument = vi.fn().mockResolvedValue({
    getPage: mockGetPage,
  })
  return { mockGetPage, mockGetDocument }
})

vi.mock('pdfjs-dist', () => ({
  default: {
    GlobalWorkerOptions: { workerSrc: '' },
    getDocument: mockGetDocument,
  },
  GlobalWorkerOptions: { workerSrc: '' },
  getDocument: mockGetDocument,
}))

describe('PdfViewer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders canvas element when URL is provided', async () => {
    render(<PdfViewer url="/api/v1/files/test123" />)
    await new Promise(r => setTimeout(r, 10))
    expect(screen.getByRole('presentation')).toBeInTheDocument()
  })

  it('renders error message when URL is empty', () => {
    render(<PdfViewer url="" />)
    expect(screen.getByText('No PDF available')).toBeInTheDocument()
  })

  it('calls getDocument with correct URL', async () => {
    render(<PdfViewer url="/api/v1/files/ABC123" />)
    await new Promise(r => setTimeout(r, 10))
    expect(mockGetDocument).toHaveBeenCalledWith('/api/v1/files/ABC123')
  })
})
