import { useEffect, useRef, useState } from 'react'
import * as pdfjsLib from 'pdfjs-dist'

pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.mjs',
  import.meta.url,
).toString()

interface PdfViewerProps {
  url: string
  scale?: number
}

export default function PdfViewer({ url, scale = 1.5 }: PdfViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

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
        const loadingTask = pdfjsLib.getDocument(url)
        const pdf = await loadingTask.promise
        if (cancelled) return

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

  if (!url) return <div className="p-4 text-sm text-gray-400">No PDF available</div>

  return (
    <div className="flex flex-col items-center gap-3 bg-gray-100 rounded-lg p-4">
      <canvas ref={canvasRef} role="presentation" className="max-w-full shadow-md border border-gray-200 rounded" />
      {loading && <span className="text-xs text-gray-500">Loading PDF...</span>}
      {error && <span className="text-xs text-red-500">{error}</span>}
    </div>
  )
}
