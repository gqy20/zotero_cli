import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Item, Collection } from '@/types/item'
import { formatAuthors, formatDate } from '@/lib/utils'
import { Search, ChevronLeft, ChevronRight, FolderOpen, BookOpen } from 'lucide-react'
import SearchInput from '@/components/SearchInput'
import EmptyState from '@/components/EmptyState'
import ErrorFallback from '@/components/ErrorFallback'
import { LibrarySkeleton } from '@/components/PageSkeletons'

export default function Library() {
  const [page, setPage] = useState(0)
  const [selectedCollection, setSelectedCollection] = useState<string | null>(null)
  const limit = 25

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['items', page, limit, selectedCollection],
    queryFn: () => api.items({ start: page * limit, limit, ...(selectedCollection ? { collection: selectedCollection } : {}) }),
    retry: 1,
  })

  const { data: collectionsData } = useQuery({
    queryKey: ['collections'],
    queryFn: () => api.collections(),
  })

  const items = data?.ok ? data.data : []
  const collections = collectionsData?.ok ? collectionsData.data : []

  const handleCollectionClick = useCallback((key: string | null) => {
    setSelectedCollection(key)
    setPage(0)
  }, [])

  const handleCollectionKeyDown = useCallback((e: React.KeyboardEvent, key: string | null) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      handleCollectionClick(key)
    }
  }, [handleCollectionClick])

  return (
    <div className="flex h-full">
      {/* Collection tree sidebar - hidden on mobile */}
      <aside className="hidden md:flex w-56 flex-col border-r border-gray-200/80 bg-white overflow-y-auto" aria-label="文献分类">
        <div className="px-4 py-4 border-b border-gray-100">
          <div className="flex items-center gap-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">
            <FolderOpen className="w-3.5 h-3.5" aria-hidden="true" />
            分类
          </div>
        </div>
        <nav aria-label="文献集合">
          <div
            role="button"
            tabIndex={0}
            aria-selected={selectedCollection === null}
            aria-label={`全部文献，共 ${data?.meta?.total ?? items.length} 条`}
            onClick={() => handleCollectionClick(null)}
            onKeyDown={(e) => handleCollectionKeyDown(e, null)}
            className={`mx-2 px-3 py-2 text-sm rounded-xl cursor-pointer font-medium flex items-center gap-2 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-red-400 focus-visible:ring-offset-1 ${
              selectedCollection === null
                ? 'bg-gradient-to-r from-red-50 to-rose-50 text-red-700'
                : 'text-gray-500 hover:bg-gray-50'
            }`}
          >
            <BookOpen className="w-4 h-4" aria-hidden="true" />
            全部文献
            <span className="ml-auto text-xs bg-red-100 text-red-600 px-1.5 py-0.5 rounded-full" aria-label={`${data?.meta?.total ?? items.length} 条文献`}>{data?.meta?.total ?? items.length}</span>
          </div>
          {collections.map((col: Collection) => (
            <div
              key={col.key}
              role="button"
              tabIndex={0}
              aria-selected={selectedCollection === col.key}
              aria-label={`${col.name}${col.num_items ? `，${col.num_items} 条` : ''}`}
              onClick={() => handleCollectionClick(col.key)}
              onKeyDown={(e) => handleCollectionKeyDown(e, col.key)}
              className={`mx-2 px-3 py-2 text-sm rounded-xl cursor-pointer transition-colors flex items-center gap-2 group outline-none focus-visible:ring-2 focus-visible:ring-red-400 focus-visible:ring-offset-1 ${
                selectedCollection === col.key
                  ? 'bg-gradient-to-r from-red-50 to-rose-50 text-red-700 font-medium'
                  : 'text-gray-500 hover:bg-gray-50'
              }`}
            >
              <FolderOpen className={`w-3.5 h-3.5 ${selectedCollection === col.key ? 'text-red-500' : 'text-gray-300 group-hover:text-gray-400'}`} aria-hidden="true" />
              <span className="truncate">{col.name}</span>
              <span className={`ml-auto text-[10px] ${selectedCollection === col.key ? 'text-red-400' : 'text-gray-300 group-hover:text-gray-400'}`} aria-label={`${col.num_items || 0} 条文献`}>{col.num_items || ''}</span>
            </div>
          ))}
        </nav>
      </aside>

      {/* Items list */}
      <div className="flex-1 flex flex-col">
        {/* Toolbar */}
        <div className="px-4 md:px-8 py-4 md:py-5 border-b border-gray-200/80 bg-white/60 backdrop-blur-sm sticky top-0 z-10">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 md:gap-0">
            <SearchInput placeholder="搜索文献..." aria-label="搜索文献" className="w-full md:max-w-md" />
            <span className="text-xs text-gray-400 tabular-nums" aria-live="polite">
              共 <strong className="text-gray-600">{data?.meta?.total ?? items.length}</strong> 条
            </span>
          </div>
        </div>

        {/* Table */}
        <div className="flex-1 overflow-auto" role="region" aria-label="文献列表">
          {isLoading ? (
            <LibrarySkeleton />
          ) : isError ? (
            <ErrorFallback message={error instanceof Error ? error.message : undefined} onRetry={() => refetch()} />
          ) : items.length === 0 ? (
            <EmptyState icon={BookOpen} message="暂无文献" className="p-12" />
          ) : (
            <table className="w-full text-sm" aria-label="文献数据表">
              <thead className="bg-gray-50/80 sticky top-0">
                <tr>
                  <th scope="col" className="px-4 md:px-8 py-3 md:py-3.5 text-left text-[10px] md:text-[11px] font-semibold text-gray-400 uppercase tracking-wider">标题</th>
                  <th scope="col" className="hidden sm:table-cell px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">作者</th>
                  <th scope="col" className="hidden md:table-cell px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">期刊 / 容器</th>
                  <th scope="col" className="px-4 md:px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider text-right">年份</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {items.map((item: Item) => (
                  <tr key={item.key} className="group hover:bg-red-50/30 transition-colors">
                    <td className="px-4 md:px-8 py-3 md:py-3.5">
                      <Link to={`/items/${item.key}`} className="font-medium text-gray-800 group-hover:text-red-600 transition-colors leading-relaxed line-clamp-2">
                        {item.title}
                      </Link>
                      {(item.tags ?? []).length > 0 && (
                        <div className="flex gap-1 mt-1.5 flex-wrap" aria-label={`${(item.tags ?? []).length} 个标签`}>
                          {(item.tags ?? []).slice(0, 3).map(tag => (
                            <span key={tag} className="inline-block px-1.5 py-0.5 text-[10px] bg-gray-100 text-gray-400 rounded-md">{tag}</span>
                          ))}
                          {(item.tags ?? []).length > 3 && (
                            <span className="inline-block px-1.5 py-0.5 text-[10px] text-gray-300">+{(item.tags ?? []).length - 3}</span>
                          )}
                        </div>
                      )}
                    </td>
                    <td className="hidden sm:table-cell px-6 py-3.5 text-gray-500 whitespace-nowrap text-xs">{formatAuthors(item.creators)}</td>
                    <td className="hidden md:table-cell px-6 py-3.5 text-gray-400 text-xs max-w-[200px] truncate">{item.container || '-'}</td>
                    <td className="px-4 md:px-6 py-3.5 text-gray-400 whitespace-nowrap text-xs tabular-nums text-right">{formatDate(item.date)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Pagination */}
        {items.length > 0 && (
          <nav className="px-4 md:px-8 py-3 md:py-4 border-t border-gray-100 bg-white/60 backdrop-blur-sm flex items-center justify-between gap-2" aria-label="分页导航">
            <button
              disabled={page === 0}
              onClick={() => setPage(p => p - 1)}
              aria-label="上一页"
              className="flex items-center gap-1.5 px-3 md:px-3.5 py-2 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all min-h-[38px]"
            >
              <ChevronLeft className="w-3.5 h-3.5" aria-hidden="true" />
              <span className="hidden sm:inline">上一页</span>
            </button>
            <div className="flex items-center gap-2" aria-current="page">
              <span className="text-xs text-gray-500">第</span>
              <span className="text-xs font-semibold text-gray-800 px-2 py-0.5 bg-gray-100 rounded-md min-w-[24px] text-center">{page + 1}</span>
              <span className="text-xs text-gray-500">页</span>
            </div>
            <button
              onClick={() => setPage(p => p + 1)}
              aria-label="下一页"
              className="flex items-center gap-1.5 px-3 md:px-3.5 py-2 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all min-h-[38px]"
            >
              <span className="hidden sm:inline">下一页</span>
              <ChevronRight className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </nav>
        )}
      </div>
    </div>
  )
}
