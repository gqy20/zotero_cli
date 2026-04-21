import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Item, Collection } from '@/types/item'
import { formatAuthors, formatDate } from '@/lib/utils'

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
      <aside className="w-56 border-r border-gray-200 bg-white overflow-y-auto">
        <div className="px-4 py-3 border-b border-gray-200">
          <h3 className="text-xs font-semibold text-gray-500 uppercase">分类</h3>
        </div>
        <div className="py-2">
          <div className="px-4 py-1.5 text-sm bg-red-50 text-red-700 rounded-md mx-2 cursor-pointer font-medium">
            全部文献
          </div>
          {collections.map((col: Collection) => (
            <div key={col.key} className="px-4 py-1.5 text-sm text-gray-600 hover:bg-gray-100 rounded-md mx-2 cursor-pointer transition-colors">
              {col.name}
            </div>
          ))}
        </div>
      </aside>

      {/* Items list */}
      <div className="flex-1 flex flex-col">
        {/* Toolbar */}
        <div className="px-6 py-4 border-b border-gray-200 bg-white flex items-center gap-3">
          <input
            type="search"
            placeholder="搜索文献..."
            className="flex-1 max-w-sm px-3 py-1.5 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-red-500/20"
          />
          <span className="text-xs text-gray-400">
            共 {data?.meta?.total ?? items.length} 条
          </span>
        </div>

        {/* Table */}
        <div className="flex-1 overflow-auto">
          {isLoading ? (
            <div className="p-6 text-sm text-gray-400">Loading...</div>
          ) : items.length === 0 ? (
            <div className="p-6 text-sm text-gray-400 text-center">暂无文献</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-gray-50 sticky top-0">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">标题</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">作者</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">期刊/容器</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">年份</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {items.map((item: Item) => (
                  <tr key={item.key} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-3">
                      <Link to={`/items/${item.key}`} className="font-medium text-red-700 hover:text-red-800 hover:underline">
                        {item.title}
                      </Link>
                      {item.tags.length > 0 && (
                        <div className="flex gap-1 mt-1 flex-wrap">
                          {item.tags.slice(0, 3).map(tag => (
                            <span key={tag} className="inline-block px-1.5 py-0.5 text-[10px] bg-gray-100 text-gray-600 rounded">{tag}</span>
                          ))}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-3 text-gray-600 whitespace-nowrap">{formatAuthors(item.creators)}</td>
                    <td className="px-6 py-3 text-gray-500">{item.container || '-'}</td>
                    <td className="px-6 py-3 text-gray-500 whitespace-nowrap">{formatDate(item.date)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Pagination */}
        {items.length > 0 && (
          <div className="px-6 py-3 border-t border-gray-200 bg-white flex items-center justify-between">
            <button
              disabled={page === 0}
              onClick={() => setPage(p => p - 1)}
              className="px-3 py-1 text-sm border rounded hover:bg-gray-50 disabled:opacity-40"
            >
              上一页
            </button>
            <span className="text-xs text-gray-500">第 {page + 1} 页</span>
            <button
              onClick={() => setPage(p => p + 1)}
              className="px-3 py-1 text-sm border rounded hover:bg-gray-50 disabled:opacity-40"
            >
              下一页
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
