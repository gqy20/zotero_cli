import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Tag } from '@/types/item'

export default function Tags() {
  const { data, isLoading } = useQuery({
    queryKey: ['tags'],
    queryFn: () => api.tags(),
  })

  const tags = data?.ok ? (data.data as Tag[]) : []

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold">标签管理</h1>

      {isLoading ? (
        <div className="text-sm text-gray-400">Loading...</div>
      ) : tags.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500 text-sm">暂无标签</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-5 py-3 text-left text-xs font-medium text-gray-500 uppercase">标签名</th>
                <th className="px-5 py-3 text-right text-xs font-medium text-gray-500 uppercase">文献数</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {[...tags]
                .sort((a, b) => (b.num_items || 0) - (a.num_items || 0))
                .map(tag => (
                  <tr key={tag.name} className="hover:bg-gray-50 transition-colors cursor-pointer">
                    <td className="px-5 py-2.5">
                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 bg-red-50 text-red-700 rounded-full text-xs font-medium border border-red-200">
                        {tag.name}
                      </span>
                    </td>
                    <td className="px-5 py-2.5 text-right text-gray-500 tabular-nums">{tag.num_items ?? '-'}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
