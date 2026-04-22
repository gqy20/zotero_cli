import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Tag } from '@/types/item'
import { Tag as TagIcon, TrendingUp, Hash } from 'lucide-react'
import LoadingSpinner from '@/components/LoadingSpinner'
import EmptyState from '@/components/EmptyState'

export default function Tags() {
  const { data, isLoading } = useQuery({
    queryKey: ['tags'],
    queryFn: () => api.tags(),
  })

  const tags = data?.ok ? (data.data as Tag[]) : []
  const sorted = [...tags].sort((a, b) => (b.num_items || 0) - (a.num_items || 0))
  const maxCount = sorted[0]?.num_items || 1

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 tracking-tight">标签管理</h1>
          <p className="text-sm text-gray-400 mt-1">按使用频率浏览所有标签</p>
        </div>
        {tags.length > 0 && (
          <div className="flex items-center gap-2 px-3 py-1.5 bg-emerald-50 rounded-full text-xs text-emerald-600 font-medium">
            <TrendingUp className="w-3.5 h-3.5" />
            {tags.length} 个标签
          </div>
        )}
      </div>

      {isLoading ? (
        <LoadingSpinner />
      ) : tags.length === 0 ? (
        <EmptyState icon={TagIcon} message="暂无标签" description="文献库中还没有任何标签" />
      ) : (
        <>
          {/* Top tags highlight */}
          {sorted.length >= 3 && (
            <div className="grid grid-cols-3 gap-4">
              {sorted.slice(0, 3).map((tag, i) => (
                <div key={tag.name} className="bg-white rounded-2xl border border-gray-100 p-5 hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300">
                  <div className="flex items-center gap-3 mb-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                      i === 0 ? 'bg-gradient-to-br from-red-100 to-rose-100' :
                      i === 1 ? 'bg-gradient-to-br from-blue-100 to-indigo-100' :
                      'bg-gradient-to-br from-violet-100 to-purple-100'
                    }`}>
                      <Hash className={`w-5 h-5 ${
                        i === 0 ? 'text-red-600' : i === 1 ? 'text-blue-600' : 'text-violet-600'
                      }`} />
                    </div>
                    <div>
                      <span className="text-xs text-gray-400">#{i + 1}</span>
                    </div>
                  </div>
                  <p className="font-semibold text-gray-800 truncate">{tag.name}</p>
                  <p className="text-2xl font-bold mt-1 tabular-nums">{tag.num_items?.toLocaleString()}</p>
                  <p className="text-[10px] text-gray-400 uppercase tracking-wider mt-0.5">篇文献</p>
                </div>
              ))}
            </div>
          )}

          {/* Full tag list */}
          <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
            <div className="px-6 py-4 border-b border-gray-100">
              <h2 className="font-semibold text-sm text-gray-900">全部标签</h2>
            </div>
            <div className="divide-y divide-gray-50">
              {sorted.map(tag => {
                const pct = ((tag.num_items || 0) / maxCount) * 100
                return (
                  <div
                    key={tag.name}
                    className="group px-6 py-3.5 flex items-center gap-4 hover:bg-gray-50/80 transition-colors cursor-pointer"
                  >
                    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg border ${
                      pct > 60 ? 'bg-red-50 text-red-700 border-red-200' :
                      pct > 30 ? 'bg-blue-50 text-blue-700 border-blue-200' :
                      'bg-gray-50 text-gray-600 border-gray-200'
                    }`}>
                      <Hash className="w-3 h-3 opacity-50" />
                      {tag.name}
                    </span>
                    <div className="flex-1 h-1.5 bg-gray-100 rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all duration-300 group-hover:opacity-80 ${
                          pct > 60 ? 'bg-gradient-to-r from-red-400 to-rose-400' :
                          pct > 30 ? 'bg-gradient-to-r from-blue-400 to-indigo-400' :
                          'bg-gradient-to-r from-gray-300 to-gray-300'
                        }`}
                        style={{ width: `${Math.max(pct, 6)}%` }}
                      />
                    </div>
                    <span className="text-xs text-gray-400 tabular-nums w-12 text-right shrink-0">{tag.num_items ?? '-'}</span>
                  </div>
                )
              })}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
