import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Item } from '@/types/item'
import { formatAuthors } from '@/lib/utils'

export default function ItemDetail() {
  const { key } = useParams<{ key: string }>()
  const { data, isLoading } = useQuery({
    queryKey: ['item', key],
    queryFn: () => api.item(key!),
    enabled: !!key,
  })

  if (isLoading) return <div className="p-6">Loading...</div>
  if (!data?.ok) return <div className="p-6 text-red-500">{data?.error || 'Not found'}</div>

  const item = data.data as Item

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start gap-4">
        <Link to="/library" className="text-sm text-gray-500 hover:text-red-600 transition-colors">
          &larr; 返回文献库
        </Link>
      </div>

      <h1 className="text-xl font-semibold">{item.title}</h1>

      {/* Metadata */}
      <div className="bg-white rounded-lg border border-gray-200 p-5 space-y-3">
        <MetadataRow label="Type" value={item.item_type} />
        <MetadataRow label="Authors" value={formatAuthors(item.creators)} />
        {item.container && <MetadataRow label="Container" value={item.container} />}
        {(item.volume || item.issue) && (
          <MetadataRow label="Vol/Issue" value={[item.volume, item.issue].filter(Boolean).join(' / ')} />
        )}
        {item.pages && <MetadataRow label="Pages" value={item.pages} />}
        {item.date && <MetadataRow label="Date" value={item.date} />}
        {item.doi && (
          <MetadataRow label="DOI" value={
            <a href={`https://doi.org/${item.doi}`} target="_blank" rel="noopener noreferrer" className="text-red-600 hover:underline">{item.doi}</a>
          } />
        )}
        {item.url && (
          <MetadataRow label="URL" value={
            <a href={item.url} target="_blank" rel="noopener noreferrer" className="text-red-600 hover:underline break-all">{item.url}</a>
          } />
        )}

        {item.tags.length > 0 && (
          <div className="flex gap-2 flex-wrap">
            {item.tags.map(tag => (
              <span key={tag} className="px-2 py-0.5 text-xs bg-red-50 text-red-700 rounded-full border border-red-200">{tag}</span>
            ))}
          </div>
        )}
      </div>

      {/* Attachments */}
      {item.attachments.length > 0 && (
        <Section title={`附件 (${item.attachments.length})`}>
          <div className="space-y-2">
            {item.attachments.map(att => (
              <div key={att.key} className="flex items-center gap-2 px-3 py-2 bg-gray-50 rounded-md text-sm">
                <span className="flex-1 truncate">{att.filename || att.title || att.key}</span>
                {att.content_type === 'application/pdf' && (
                  <button className="px-2 py-1 text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors">
                    预览
                  </button>
                )}
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Notes */}
      {item.notes.length > 0 && (
        <Section title={`笔记 (${item.notes.length})`}>
          <div className="space-y-2">
            {item.notes.map(note => (
              <div key={note.key} className="p-3 bg-yellow-50 rounded-md text-sm border border-yellow-200">
                <p dangerouslySetInnerHTML={{ __html: note.content || note.preview || '' }} />
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Annotations */}
      {item.annotations.length > 0 && (
        <Section title={`标注 (${item.annotations.length})`}>
          <div className="space-y-2">
            {item.annotations.map(ann => (
              <div key={ann.key} className="flex items-start gap-2 px-3 py-2 rounded-md text-sm" style={{ backgroundColor: ann.color ? `${ann.color}20` : '#f9fafb' }}>
                <span
                  className="w-2 h-2 rounded-full mt-1.5 shrink-0"
                  style={{ backgroundColor: ann.color || '#fbbf24' }}
                />
                <div className="min-w-0 flex-1">
                  {ann.text && <p className="text-gray-700">{ann.text}</p>}
                  {ann.comment && <p className="text-gray-500 italic mt-0.5">{ann.comment}</p>}
                  <span className="text-[10px] text-gray-400">p.{ann.page_label || ann.page_index}</span>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}
    </div>
  )
}

function MetadataRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[100px_1fr] gap-4 text-sm">
      <dt className="text-gray-500">{label}</dt>
      <dd className="text-gray-900">{value}</dd>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-white rounded-lg border border-gray-200">
      <div className="px-5 py-3 border-b border-gray-200">
        <h2 className="font-medium text-sm">{title}</h2>
      </div>
      <div className="p-5">{children}</div>
    </div>
  )
}
