import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { BookOpen, FolderOpen, Search, BarChart3 } from 'lucide-react'
import type { LibraryStats, Item } from '@/types/item'

function StatCard({ title, value, Icon }: { title: string; value: number | string; Icon: React.ComponentType<{ className?: string }> }) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-500">{title}</p>
          <p className="text-2xl font-semibold mt-1">{value}</p>
        </div>
        <Icon className="w-8 h-8 text-red-100 text-red-600" />
      </div>
    </div>
  )
}

export default function Dashboard() {
  const { data, isLoading } = useQuery({ queryKey: ['overview'], queryFn: () => api.overview() })

  if (isLoading) return <div className="p-6">Loading...</div>
  if (!data?.ok) return <div className="p-6 text-red-500">{data?.error || 'Failed to load'}</div>

  const stats = data.data.stats
  const recentItems = data.data.recent_items

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold">总览</h1>

      {/* Stats grid */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard title="文献数" value={stats.total_items} Icon={BookOpen} />
        <StatCard title="分类数" value={stats.total_collections} Icon={FolderOpen} />
        <StatCard title="标签数" value="- " Icon={TagsPlaceholder} />
        <StatCard title="搜索数" value={stats.total_searches} Icon={Search} />
      </div>

      {/* Recent items */}
      <div className="bg-white rounded-lg border border-gray-200">
        <div className="px-5 py-4 border-b border-gray-200">
          <h2 className="font-medium text-sm">最近添加</h2>
        </div>
        <div className="divide-y divide-gray-100">
          {recentItems.length === 0 ? (
            <div className="px-5 py-8 text-center text-sm text-gray-400">暂无文献</div>
          ) : (
            recentItems.map((item: Item) => (
              <div key={item.key} className="px-5 py-3 flex items-center justify-between hover:bg-gray-50 transition-colors">
                <div className="min-w-0 flex-1 mr-4">
                  <p className="text-sm font-medium truncate">{item.title}</p>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {[...new Set(item.creators.map(c => c.name))].slice(0, 2).join(', ')}
                    {item.creators.length > 2 && ' et al.'}
                  </p>
                </div>
                <span className="text-xs text-gray-400 whitespace-nowrap">{item.date || '-'}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

function TagsPlaceholder({ className }: { className?: string }) {
  return <BarChart3 className={className ?? 'w-8 h-8'} />
}
