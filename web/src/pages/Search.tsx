import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Item } from '@/types/item'
import { formatAuthors, formatDate } from '@/lib/utils'
import { Search as SearchIcon } from 'lucide-react'

export default function Search() {
  const [query, setQuery] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['search', query],
    queryFn: () => api.items({ q: query, limit: 50 }),
    enabled: query.length > 0,
  })

  const items = data?.ok ? data.data : []

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold">全文搜索</h1>

      {/* Search bar */}
      <div className="relative max-w-xl">
        <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
        <input
          type="search"
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="输入关键词搜索文献..."
          className="w-full pl-10 pr-4 py-2.5 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-red-500/20"
          autoFocus
        />
      </div>

      {/* Results */}
      {isLoading ? (
        <div className="text-sm text-gray-400">Searching...</div>
      ) : query && items.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500 text-sm">未找到匹配的文献</p>
        </div>
      ) : items.length > 0 ? (
        <div className="space-y-1">
          <p className="text-xs text-gray-400 mb-3">找到 {items.length} 条结果</p>
          {items.map((item: Item) => (
            <Link
              key={item.key}
              to={`/items/${item.key}`}
              className="block p-4 bg-white rounded-lg border border-gray-200 hover:border-red-300 hover:shadow-sm transition-all"
            >
              <h3 className="font-medium text-red-700 hover:underline">{item.title}</h3>
              <p className="text-xs text-gray-500 mt-1">{formatAuthors(item.creators)} &middot; {formatDate(item.date)}</p>
              {item.full_text_preview && (
                <p className="text-xs text-gray-400 mt-2 line-clamp-2">{item.full_text_preview}</p>
              )}
            </Link>
          ))}
        </div>
      ) : null}
    </div>
  )
}
