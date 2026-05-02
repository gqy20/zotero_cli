import { useEffect, useRef, useState, useCallback } from 'react'
import { ChevronLeft, ChevronRight, ZoomIn, ZoomOut, Maximize2 } from 'lucide-react'

interface PdfViewerProps {
  url: string
  scale?: number
}

const MIN_SCALE = 0.25
const MAX_SCALE = 4
const SCALE_STEP = 0.25

export default function PdfViewer({ url, scale: initialScale = 1.5 }: PdfViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const pdfRef = useRef<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(0)
  const [scale, setScale] = useState(initialScale)

  const renderPage = useCallback(async (pageNum: number, renderScale: number) => {
    if (!pdfRef.current || !canvasRef.current) return
    try {
      const page = await pdfRef.current.getPage(pageNum)
      const viewport = page.getViewport({ scale: renderScale })
      const canvas = canvasRef.current
      canvas.width = viewport.width
      canvas.height = viewport.height
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      await page.render({ canvasContext: ctx, viewport }).promise
    } catch {
      // silently fail on render errors during navigation
    }
  }, [])

  useEffect(() => {
    if (!url) {
      setError('No PDF available')
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)
    setError(null)

    const loadPdf = async () => {
      try {
        const pdfjsLib = await import('pdfjs-dist')
        pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
          'pdfjs-dist/build/pdf.worker.mjs',
          import.meta.url,
        ).toString()

        const loadingTask = pdfjsLib.getDocument(url)
        const pdf = await loadingTask.promise
        if (cancelled) return

        pdfRef.current = pdf
        setTotalPages(pdf.numPages)
        setCurrentPage(1)

        const page = await pdf.getPage(1)
        if (cancelled) return

        const viewport = page.getViewport({ scale })
        const canvas = canvasRef.current
        if (!canvas) return

        canvas.width = viewport.width
        canvas.height = viewport.height

        const ctx = canvas.getContext('2d')
        if (!ctx) return

        await page.render({ canvasContext: ctx, viewport }).promise
        if (!cancelled) setLoading(false)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load PDF')
          setLoading(false)
        }
      }
    }

    loadPdf()
    return () => { cancelled = true }
  }, [url, scale])

  // Re-render when currentPage changes
  useEffect(() => {
    if (pdfRef.current && !loading && currentPage >= 1 && currentPage <= totalPages) {
      renderPage(currentPage, scale)
    }
  }, [currentPage, scale, loading, totalPages, renderPage])

  const goNext = () => setCurrentPage(p => Math.min(p + 1, totalPages))
  const goPrev = () => setCurrentPage(p => Math.max(p - 1, 1))
  const zoomIn = () => setScale(s => Math.min(s + SCALE_STEP, MAX_SCALE))
  const zoomOut = () => setScale(s => Math.max(s - SCALE_STEP, MIN_SCALE))
  const fitWidth = () => {
    if (!containerRef.current || !pdfRef.current) return
    const containerWidth = containerRef.current.clientWidth - 32 // padding
    pdfRef.current.getPage(currentPage).then((page: any) => {
      const viewport = page.getViewport({ scale: 1 })
      const fitScale = containerWidth / viewport.width
      setScale(Math.round(fitScale * 100) / 100)
    })
  }

  if (!url) return <div className="p-4 text-sm text-gray-400">No PDF available</div>

  return (
    <div className="flex flex-col items-center gap-3 bg-gray-100 rounded-lg p-4" ref={containerRef}>
      {/* Toolbar */}
      {totalPages > 0 && (
        <div className="flex items-center gap-3 w-full max-w-full">
          {/* Page nav */}
          <div className="flex items-center gap-1.5">
            <button
              onClick={goPrev}
              disabled={currentPage <= 1}
              title="上一页"
              className="p-1.5 rounded-lg border border-gray-200 bg-white hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="text-xs text-gray-600 min-w-[48px] text-center tabular-nums font-medium">
              {currentPage} / {totalPages}
            </span>
            <button
              onClick={goNext}
              disabled={currentPage >= totalPages}
              title="下一页"
              className="p-1.5 rounded-lg border border-gray-200 bg-white hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          <div className="flex-1" />

          {/* Zoom */}
          <div className="flex items-center gap-1.5">
            <button onClick={zoomOut} disabled={scale <= MIN_SCALE} title="缩小"
              className="p-1.5 rounded-lg border border-gray-200 bg-white hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all">
              <ZoomOut className="w-4 h-4" />
            </button>
            <span className="text-xs text-gray-500 min-w-[44px] text-center tabular-nums font-mono">
              {Math.round(scale * 100)}%
            </span>
            <button onClick={zoomIn} disabled={scale >= MAX_SCALE} title="放大"
              className="p-1.5 rounded-lg border border-gray-200 bg-white hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all">
              <ZoomIn className="w-4 h-4" />
            </button>
            <button onClick={fitWidth} title="适应宽度"
              className="p-1.5 rounded-lg border border-gray-200 bg-white hover:bg-gray-50 transition-all ml-1">
              <Maximize2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Canvas */}
      <canvas ref={canvasRef} role="presentation" className="max-w-full shadow-md border border-gray-200 rounded" />

      {/* Status */}
      {loading && <span className="text-xs text-gray-500">Loading PDF...</span>}
      {error && <span className="text-xs text-red-500">{error}</span>}
    </div>
  )
}
