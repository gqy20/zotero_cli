import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PdfViewer from './PdfViewer'

// Mock canvas getContext for jsdom
beforeEach(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue({
    fillRect: vi.fn(),
    clearRect: vi.fn(),
    getImageData: vi.fn(),
    putImageData: vi.fn(),
    createImageData: vi.fn(),
    setTransform: vi.fn(),
    drawImage: vi.fn(),
    save: vi.fn(),
    restore: vi.fn(),
    scale: vi.fn(),
    rotate: vi.fn(),
    translate: vi.fn(),
    beginPath: vi.fn(),
    closePath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    stroke: vi.fn(),
    fill: vi.fn(),
    measureText: vi.fn().mockReturnValue({ width: 0 }),
    font: '',
    fillStyle: '',
    strokeStyle: '',
  })
})

const { mockGetPage, mockGetDocument } = vi.hoisted(() => {
  const mockRender = vi.fn().mockResolvedValue(undefined)
  const mockGetPage = vi.fn().mockResolvedValue({
    getViewport: vi.fn().mockReturnValue({ width: 100, height: 150 }),
    render: mockRender,
  })
  const pdfDoc = { getPage: mockGetPage, numPages: 3 }
  // pdfjs-dist getDocument returns a loadingTask whose .promise resolves to pdfDoc
  const mockGetDocument = vi.fn().mockReturnValue({
    promise: Promise.resolve(pdfDoc),
  })
  return { mockGetPage, mockGetDocument, mockRender }
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

  it('shows loading state initially', () => {
    render(<PdfViewer url="/api/v1/files/test" />)
    expect(screen.getByText('Loading PDF...')).toBeInTheDocument()
  })

  it('shows error when pdfjs fails to load', async () => {
    mockGetDocument.mockRejectedValueOnce(new Error('Network error'))
    render(<PdfViewer url="/api/v1/files/bad" />)
    await new Promise(r => setTimeout(r, 50))
    expect(screen.getByText(/Cannot read|error|failed/i)).toBeInTheDocument()
  })

  // --- New tests for pagination ---

  it('shows page navigation controls', async () => {
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByTitle('上一页')).toBeInTheDocument(), { timeout: 3000 })
    expect(screen.getByTitle('下一页')).toBeInTheDocument()
  })

  it('shows current page and total pages', async () => {
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByText('1 / 3')).toBeInTheDocument(), { timeout: 3000 })
  })

  it('disables prev button on first page', async () => {
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => {
      const btn = screen.getByTitle('上一页')
      expect(btn).toBeDisabled()
    }, { timeout: 3000 })
  })

  it('navigates to next page when clicking next button', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByText('1 / 3')).toBeInTheDocument(), { timeout: 3000 })

    await user.click(screen.getByTitle('下一页'))
    await waitFor(() => expect(screen.getByText('2 / 3')).toBeInTheDocument(), { timeout: 3000 })
    expect(mockGetPage).toHaveBeenCalledWith(2)
  })

  it('navigates to previous page when clicking prev button', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByText('1 / 3')).toBeInTheDocument(), { timeout: 3000 })

    // go to page 2
    await user.click(screen.getByTitle('下一页'))
    await waitFor(() => expect(screen.getByText('2 / 3')).toBeInTheDocument(), { timeout: 3000 })

    // go back to page 1
    await user.click(screen.getByTitle('上一页'))
    await waitFor(() => expect(screen.getByText('1 / 3')).toBeInTheDocument(), { timeout: 3000 })
    expect(mockGetPage).toHaveBeenLastCalledWith(1)
  })

  it('disables next button on last page', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByText('1 / 3')).toBeInTheDocument(), { timeout: 3000 })

    // go to page 3 (last)
    await user.click(screen.getByTitle('下一页'))
    await user.click(screen.getByTitle('下一页'))
    await waitFor(() => {
      const btn = screen.getByTitle('下一页')
      expect(btn).toBeDisabled()
    }, { timeout: 3000 })
  })

  // --- New tests for zoom ---

  it('shows zoom controls', async () => {
    render(<PdfViewer url="/api/v1/files/test" />)
    await waitFor(() => expect(screen.getByTitle('缩小')).toBeInTheDocument(), { timeout: 3000 })
    expect(screen.getByTitle('放大')).toBeInTheDocument()
    expect(screen.getByTitle('适应宽度')).toBeInTheDocument()
  })

  it('increases scale when clicking zoom in', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" scale={1.5} />)
    await waitFor(() => expect(screen.getByTitle('放大')).toBeInTheDocument(), { timeout: 3000 })

    await user.click(screen.getByTitle('放大'))
    await waitFor(() => {
      expect(screen.getByText('175%')).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('decreases scale when clicking zoom out', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" scale={2} />)
    await waitFor(() => expect(screen.getByTitle('缩小')).toBeInTheDocument(), { timeout: 3000 })

    await user.click(screen.getByTitle('缩小'))
    await waitFor(() => {
      expect(screen.getByText('175%')).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('fits to width when clicking fit-width button', async () => {
    const user = userEvent.setup()
    render(<PdfViewer url="/api/v1/files/test" scale={0.5} />)
    await waitFor(() => expect(screen.getByTitle('适应宽度')).toBeInTheDocument(), { timeout: 3000 })

    // mock containerRef for fitWidth
    vi.spyOn(HTMLDivElement.prototype, 'clientWidth', 'get').mockReturnValue(468)
    await user.click(screen.getByTitle('适应宽度'))
    await waitFor(() => {
      // fitWidth calculates scale from container width / viewport width (100)
      expect(screen.getByText(/%/)).toBeInTheDocument()
    }, { timeout: 3000 })
  })
})
