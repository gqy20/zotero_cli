import { useState, useRef, useEffect, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Item } from '@/types/item'
import { formatAuthors } from '@/lib/utils'
import PdfViewer from '@/components/PdfViewer'
import MetaRow from '@/components/MetaRow'
import Section from '@/components/Section'
import ErrorFallback from '@/components/ErrorFallback'
import { ItemDetailSkeleton } from '@/components/PageSkeletons'
import TagBadge from '@/components/TagBadge'
import {
  ArrowLeft, FileText, ExternalLink, Paperclip, StickyNote,
  Highlighter, X, Calendar, BookMarked, User, Hash, Link2
} from 'lucide-react'

export default function ItemDetail() {
  const { key } = useParams<{ key: string }>()
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const modalRef = useRef<HTMLDivElement>(null)
  const closeBtnRef = useRef<HTMLButtonElement>(null)
  const previousActiveElement = useRef<HTMLElement | null>(null)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['item', key],
    queryFn: () => api.item(key!),
    enabled: !!key,
    retry: 1,
  })

  const openModal = useCallback((url: string) => {
    previousActiveElement.current = document.activeElement as HTMLElement
    setPreviewUrl(url)
  }, [])

  const closeModal = useCallback(() => {
    setPreviewUrl(null)
  }, [])

  // Focus trap and Escape key handling for modal
  useEffect(() => {
    if (!previewUrl) {
      // Return focus when modal closes
      if (previousActiveElement.current) {
        previousActiveElement.current.focus()
      }
      return
    }

    // Focus close button when modal opens
    const timer = setTimeout(() => {
      closeBtnRef.current?.focus()
    }, 50)

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        closeModal()
        return
      }

      // Focus trap: keep Tab inside modal
      if (e.key === 'Tab' && modalRef.current) {
        const focusableElements = modalRef.current.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        const firstEl = focusableElements[0]
        const lastEl = focusableElements[focusableElements.length - 1]

        if (e.shiftKey && document.activeElement === firstEl) {
          e.preventDefault()
          lastEl?.focus()
        } else if (!e.shiftKey && document.activeElement === lastEl) {
          e.preventDefault()
          firstEl?.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    // Prevent body scroll when modal is open
    document.body.style.overflow = 'hidden'

    return () => {
      clearTimeout(timer)
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = ''
    }
  }, [previewUrl, closeModal])

  if (isLoading) return <ItemDetailSkeleton />
  if (isError) return <ErrorFallback message={error instanceof Error ? error.message : undefined} onRetry={() => refetch()} className="p-8" />
  if (!data?.ok) return <ErrorFallback message={data?.error || 'Not found'} onRetry={() => refetch()} className="p-8" />

  const item = data.data as Item

  return (
    <div className="p-4 md:p-8 space-y-6 md:space-y-8">
      {/* Breadcrumb */}
      <Link to="/library" className="inline-flex items-center gap-1.5 text-sm text-gray-400 hover:text-red-500 transition-colors group">
        <ArrowLeft className="w-3.5 h-3.5 group-hover:-translate-x-0.5 transition-transform" aria-hidden="true" />
        返回文献库
      </Link>

      {/* Title */}
      <div className="space-y-3">
        <h1 className="text-2xl font-bold text-gray-900 leading-relaxed tracking-tight">{item.title}</h1>
        <TagBadge tags={item.tags ?? []} variant="styled" />
      </div>

      {/* Metadata */}
      <div className="bg-white rounded-2xl border border-gray-100 p-4 md:p-6 shadow-sm">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 md:gap-x-10 gap-y-4">
          <MetaRow icon={<BookMarked className="w-4 h-4" />} label="类型" value={item.item_type} />
          <MetaRow icon={<User className="w-4 h-4" />} label="作者" value={formatAuthors(item.creators)} />
          {item.container && <MetaRow icon={<FileText className="w-4 h-4" />} label="容器" value={item.container} />}
          {(item.volume || item.issue) && (
            <MetaRow icon={<Hash className="w-4 h-4" />} label="卷/期" value={[item.volume, item.issue].filter(Boolean).join(' / ')} />
          )}
          {item.pages && <MetaRow icon={<Hash className="w-4 h-4" />} label="页码" value={item.pages} />}
          {item.date && <MetaRow icon={<Calendar className="w-4 h-4" />} label="日期" value={item.date} />}
          {item.doi ? (
            <MetaRow icon={<ExternalLink className="w-4 h-4" />} label="DOI"
              value={<a href={`https://doi.org/${item.doi}`} target="_blank" rel="noopener noreferrer" className="text-red-600 hover:text-red-700 hover:underline font-mono text-xs">{item.doi}</a>}
            />
          ) : null}
          {item.url ? (
            <MetaRow icon={<Link2 className="w-4 h-4" />} label="URL"
              value={<a href={item.url} target="_blank" rel="noopener noreferrer" className="text-red-600 hover:text-red-700 hover:underline break-all text-xs max-w-[280px] truncate block">{item.url}</a>}
            />
          ) : null}
        </div>
      </div>

      {/* Attachments */}
      {(item.attachments ?? []).length > 0 && (
        <Section title="附件" icon={<Paperclip className="w-4 h-4" />} count={(item.attachments ?? []).length}>
          <div className="grid gap-2">
            {(item.attachments ?? []).map(att => (
              <div key={att.key} className="flex items-center gap-3 px-4 py-3 bg-gray-50/80 rounded-xl text-sm group">
                <div className="w-8 h-8 rounded-lg bg-white border border-gray-200 flex items-center justify-center shrink-0" aria-hidden="true">
                  <FileText className="w-4 h-4 text-gray-400" />
                </div>
                <span className="flex-1 truncate text-gray-700">{att.filename || att.title || att.key}</span>
                {att.content_type === 'application/pdf' && (
                  <button
                    onClick={() => openModal(`/api/v1/files/${att.key}`)}
                    aria-label={`预览 ${att.filename || att.title || 'PDF 文件'}`}
                    className="px-3 py-1.5 text-xs bg-gradient-to-r from-red-500 to-rose-500 text-white rounded-lg hover:shadow-md hover:shadow-red-500/25 hover:-translate-y-0.5 active:translate-y-0 transition-all duration-200 font-medium"
                  >
                    预览
                  </button>
                )}
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Notes */}
      {(item.notes ?? []).length > 0 && (
        <Section title="笔记" icon={<StickyNote className="w-4 h-4" />} count={(item.notes ?? []).length}>
          <div className="space-y-3">
            {(item.notes ?? []).map(note => (
              <div key={note.key} className="p-4 bg-gradient-to-br from-yellow-50/80 to-amber-50/40 rounded-xl text-sm border border-yellow-100/60 leading-relaxed">
                <p dangerouslySetInnerHTML={{ __html: note.content || note.preview || '' }} />
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Annotations */}
      {(item.annotations ?? []).length > 0 && (
        <Section title="标注" icon={<Highlighter className="w-4 h-4" />} count={(item.annotations ?? []).length}>
          <div className="space-y-2">
            {(item.annotations ?? []).map(ann => (
              <div
                key={ann.key}
                className="flex items-start gap-3 px-4 py-3 rounded-xl text-sm"
                style={{ backgroundColor: ann.color ? `${ann.color}08` : '#fafafa' }}
              >
                <span
                  className="w-2.5 h-2.5 rounded-full mt-1.5 shrink-0 ring-2 ring-offset-1"
                  style={{
                    backgroundColor: ann.color || '#fbbf24',
                    '--tw-ring-color': `${(ann.color || '#fbbf24')}30`,
                  } as React.CSSProperties}
                  aria-hidden="true"
                />
                <div className="min-w-0 flex-1 space-y-1">
                  {ann.text && <p className="text-gray-700 leading-relaxed">{ann.text}</p>}
                  {ann.comment && <p className="text-gray-400 italic text-xs pl-2 border-l-2 border-gray-200">{ann.comment}</p>}
                  <span className="text-[10px] text-gray-300 tabular-nums">p.{ann.page_label || ann.page_index}</span>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* PDF Preview Modal - Accessible Dialog */}
      {previewUrl && (
        <div
          className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm flex items-center justify-center p-2 animate-in fade-in duration-200"
          onClick={() => setPreviewUrl(null)}
          role="presentation"
        >
          <div
            ref={modalRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="pdf-modal-title"
            className="bg-white rounded-2xl shadow-2xl w-[95vw] h-[95vh] flex flex-col animate-in zoom-in-95 duration-200 outline-none"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100 shrink-0">
              <div className="flex items-center gap-2" id="pdf-modal-title">
                <FileText className="w-4 h-4 text-red-500" aria-hidden="true" />
                <h3 className="font-semibold text-sm text-gray-900">PDF 预览</h3>
              </div>
              <button
                ref={closeBtnRef}
                onClick={() => setPreviewUrl(null)}
                aria-label="关闭预览"
                className="w-8 h-8 rounded-full hover:bg-gray-100 flex items-center justify-center text-gray-400 hover:text-gray-600 transition-colors"
              >
                <X className="w-4 h-4" aria-hidden="true" />
              </button>
            </div>
            <div className="flex-1 overflow-auto p-3 bg-gray-50/50">
              <PdfViewer url={previewUrl} />
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
