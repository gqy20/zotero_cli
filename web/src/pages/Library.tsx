import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Item, Collection } from '@/types/item'
import { formatAuthors, formatDate } from '@/lib/utils'
import { Search, ChevronLeft, ChevronRight, FolderOpen, BookOpen } from 'lucide-react'
import SearchInput from '@/components/SearchInput'
import EmptyState from '@/components/EmptyState'
import { LibrarySkeleton } from '@/components/PageSkeletons'

export default function Library() {
  const [page, setPage] = useState(0)
  const limit = 25

  const { data, isLoading } = useQuery({
    queryKey: ['items', page, limit],
    queryFn: () => api.items({ start: page * limit, limit }),
  })

  const { data: collectionsData } = useQuery({
    queryKey: ['collections'],
    queryFn: () => api.collections(),
  })

  const items = data?.ok ? data.data : []
  const collections = collectionsData?.ok ? collectionsData.data : []

  return (
    <div className="flex h-full">
      {/* Collection tree sidebar */}
      <aside className="w-56 border-r border-gray-200/80 bg-white overflow-y-auto">
        <div className="px-4 py-4 border-b border-gray-100">
          <div className="flex items-center gap-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">
            <FolderOpen className="w-3.5 h-3.5" />
            分类
          </div>
        </div>
        <div className="py-2">
          <div className="mx-2 px-3 py-2 text-sm bg-gradient-to-r from-red-50 to-rose-50 text-red-700 rounded-xl cursor-pointer font-medium flex items-center gap-2">
            <BookOpen className="w-4 h-4" />
            全部文献
            <span className="ml-auto text-xs bg-red-100 text-red-600 px-1.5 py-0.5 rounded-full">{data?.meta?.total ?? items.length}</span>
          </div>
          {collections.map((col: Collection) => (
            <div key={col.key} className="mx-2 px-3 py-2 text-sm text-gray-500 hover:bg-gray-50 rounded-xl cursor-pointer transition-colors flex items-center gap-2 group">
              <FolderOpen className="w-3.5 h-3.5 text-gray-300 group-hover:text-gray-400" />
              <span className="truncate">{col.name}</span>
              <span className="ml-auto text-[10px] text-gray-300 group-hover:text-gray-400">{col.num_items || ''}</span>
            </div>
          ))}
        </div>
      </aside>

      {/* Items list */}
      <div className="flex-1 flex flex-col">
        {/* Toolbar */}
        <div className="px-8 py-5 border-b border-gray-200/80 bg-white/60 backdrop-blur-sm sticky top-0 z-10">
          <div className="flex items-center justify-between">
            <SearchInput placeholder="搜索文献..." />
            <span className="text-xs text-gray-400 ml-6 tabular-nums">
              共 <strong className="text-gray-600">{data?.meta?.total ?? items.length}</strong> 条
            </span>
          </div>
        </div>

        {/* Table */}
        <div className="flex-1 overflow-auto">
          {isLoading ? (
            <LibrarySkeleton />
          ) : items.length === 0 ? (
            <EmptyState icon={BookOpen} message="暂无文献" className="p-12" />
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-gray-50/80 sticky top-0">
                <tr>
                  <th className="px-8 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">标题</th>
                  <th className="px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">作者</th>
                  <th className="px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">期刊 / 容器</th>
                  <th className="px-6 py-3.5 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">年份</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {items.map((item: Item) => (
                  <tr key={item.key} className="group hover:bg-red-50/30 transition-colors">
                    <td className="px-8 py-3.5">
                      <Link to={`/items/${item.key}`} className="font-medium text-gray-800 group-hover:text-red-600 transition-colors leading-relaxed line-clamp-1">
                        {item.title}
                      </Link>
                      {(item.tags ?? []).length > 0 && (
                        <div className="flex gap-1 mt-1.5 flex-wrap">
                          {(item.tags ?? []).slice(0, 3).map(tag => (
                            <span key={tag} className="inline-block px-1.5 py-0.5 text-[10px] bg-gray-100 text-gray-400 rounded-md">{tag}</span>
                          ))}
                          {(item.tags ?? []).length > 3 && (
                            <span className="inline-block px-1.5 py-0.5 text-[10px] text-gray-300">+{(item.tags ?? []).length - 3}</span>
                          )}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-3.5 text-gray-500 whitespace-nowrap text-xs">{formatAuthors(item.creators)}</td>
                    <td className="px-6 py-3.5 text-gray-400 text-xs max-w-[200px] truncate">{item.container || '-'}</td>
                    <td className="px-6 py-3.5 text-gray-400 whitespace-nowrap text-xs tabular-nums">{formatDate(item.date)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Pagination */}
        {items.length > 0 && (
          <div className="px-8 py-4 border-t border-gray-100 bg-white/60 backdrop-blur-sm flex items-center justify-between">
            <button
              disabled={page === 0}
              onClick={() => setPage(p => p - 1)}
              className="flex items-center gap-1.5 px-3.5 py-2 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              <ChevronLeft className="w-3.5 h-3.5" />
              上一页
            </button>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">第</span>
              <span className="text-xs font-semibold text-gray-800 px-2 py-0.5 bg-gray-100 rounded-md min-w-[24px] text-center">{page + 1}</span>
              <span className="text-xs text-gray-500">页</span>
            </div>
            <button
              onClick={() => setPage(p => p + 1)}
              className="flex items-center gap-1.5 px-3.5 py-2 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              下一页
              <ChevronRight className="w-3.5 h-3.5" />
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
