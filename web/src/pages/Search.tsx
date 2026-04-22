import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Item } from '@/types/item'
import { formatAuthors, formatDate } from '@/lib/utils'
import { FileText, ArrowRight } from 'lucide-react'
import SearchInput from '@/components/SearchInput'
import EmptyState from '@/components/EmptyState'
import { SearchSkeleton } from '@/components/PageSkeletons'

export default function Search() {
  const [query, setQuery] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['search', query],
    queryFn: () => api.items({ q: query, limit: 50 }),
    enabled: query.length > 0,
  })

  const items = data?.ok ? data.data : []

  return (
    <div className="p-8 space-y-8 max-w-4xl">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 tracking-tight">全文搜索</h1>
        <p className="text-sm text-gray-400 mt-1">在文献库中快速定位所需内容</p>
      </div>

      {/* Search bar */}
      <SearchInput placeholder="输入关键词搜索文献..." variant="prominent" value={query} onChange={setQuery} autoFocus />

      {/* Results */}
      {isLoading ? (
        <SearchSkeleton />
      ) : query && items.length === 0 ? (
        <EmptyState icon={FileText} message="未找到匹配的文献" description="尝试使用不同的关键词" />
      ) : items.length > 0 ? (
        <div className="space-y-4">
          <div className="flex items-center gap-2 text-xs text-gray-400">
            <span>找到</span>
            <span className="font-semibold text-red-600">{items.length}</span>
            <span>条结果</span>
            <span className="mx-1">·</span>
            <span>关键词: "<strong className="text-gray-600">{query}</strong>"</span>
          </div>
          {items.map((item: Item) => (
            <Link
              key={item.key}
              to={`/items/${item.key}`}
              className="group block p-5 bg-white rounded-2xl border border-gray-100 hover:border-red-100 hover:shadow-lg hover:shadow-red-500/5 hover:-translate-y-0.5 transition-all duration-300"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1">
                  <h3 className="font-semibold text-gray-800 group-hover:text-red-600 transition-colors leading-relaxed">{item.title}</h3>
                  <div className="flex items-center gap-2 mt-2 text-xs text-gray-400">
                    <span>{formatAuthors(item.creators)}</span>
                    <span className="text-gray-200">·</span>
                    <span>{formatDate(item.date)}</span>
                    <span className="text-gray-200">·</span>
                    <span className="px-1.5 py-0.5 bg-gray-100 text-gray-500 rounded text-[10px]">{item.item_type}</span>
                  </div>
                  {item.full_text_preview && (
                    <p className="text-xs text-gray-400 mt-3 line-clamp-2 leading-relaxed bg-gray-50/80 rounded-lg px-3 py-2">{item.full_text_preview}</p>
                  )}
                </div>
                <ArrowRight className="w-4 h-4 text-gray-200 group-hover:text-red-400 shrink-0 mt-1 group-hover:translate-x-0.5 transition-all" />
              </div>
            </Link>
          ))}
        </div>
      ) : (
        <EmptyState icon={FileText} message="开始搜索" description="输入关键词以搜索文献库中的内容" />
      )}
    </div>
  )
}
