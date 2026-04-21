import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { BookOpen, FolderOpen, Search, BarChart3, ArrowUpRight, Clock } from 'lucide-react'
import type { LibraryStats, Item } from '@/types/item'

const statStyles = [
  { bg: 'from-blue-500 to-indigo-600', shadow: 'shadow-blue-500/20', iconBg: 'bg-blue-500/10', iconColor: 'text-blue-600' },
  { bg: 'from-emerald-500 to-teal-600', shadow: 'shadow-emerald-500/20', iconBg: 'bg-emerald-500/10', iconColor: 'text-emerald-600' },
  { bg: 'from-violet-500 to-purple-600', shadow: 'shadow-violet-500/20', iconBg: 'bg-violet-500/10', iconColor: 'text-violet-600' },
  { bg: 'from-amber-500 to-orange-600', shadow: 'shadow-amber-500/20', iconBg: 'bg-amber-500/10', iconColor: 'text-amber-600' },
]

function StatCard({ title, value, Icon, style }: { title: string; value: number | string; Icon: React.ComponentType<{ className?: string }>; style: typeof statStyles[0] }) {
  return (
    <div className="group relative bg-white rounded-2xl border border-gray-100 p-5 hover:shadow-lg hover:shadow-gray-200/50 hover:-translate-y-0.5 transition-all duration-300">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-gray-400 uppercase tracking-wider">{title}</p>
          <p className="text-3xl font-bold mt-1.5 text-gray-900 tracking-tight">{value.toLocaleString()}</p>
        </div>
        <div className={`w-12 h-12 rounded-xl ${style.iconBg} flex items-center justify-center`}>
          <Icon className={`w-6 h-6 ${style.iconColor}`} />
        </div>
      </div>
      <div className={`absolute inset-x-0 top-0 h-1 rounded-t-2xl bg-gradient-to-r ${style.bg} opacity-0 group-hover:opacity-100 transition-opacity duration-300`} />
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
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 tracking-tight">总览</h1>
          <p className="text-sm text-gray-400 mt-1">文献库概览与最近动态</p>
        </div>
        <div className="flex items-center gap-2 text-xs text-gray-400">
          <Clock className="w-3.5 h-3.5" />
          <span>实时更新</span>
        </div>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-4 gap-5">
        <StatCard title="文献数" value={stats.total_items} Icon={BookOpen} style={statStyles[0]} />
        <StatCard title="分类数" value={stats.total_collections} Icon={FolderOpen} style={statStyles[1]} />
        <StatCard title="标签数" value="-" Icon={BarChart3} style={statStyles[2]} />
        <StatCard title="搜索数" value={stats.total_searches} Icon={Search} style={statStyles[3]} />
      </div>

      {/* Recent items */}
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
          <h2 className="font-semibold text-sm text-gray-900">最近添加</h2>
          <span className="text-xs text-gray-400">{recentItems.length} 条记录</span>
        </div>
        <div className="divide-y divide-gray-50">
          {recentItems.length === 0 ? (
            <div className="px-6 py-12 text-center">
              <BookOpen className="w-8 h-8 text-gray-200 mx-auto mb-2" />
              <p className="text-sm text-gray-400">暂无文献</p>
            </div>
          ) : (
            recentItems.map((item: Item) => (
              <a
                key={item.key}
                href={`/items/${item.key}`}
                className="group px-6 py-4 flex items-center justify-between hover:bg-gray-50/80 transition-colors"
              >
                <div className="min-w-0 flex-1 mr-4">
                  <p className="text-sm font-medium text-gray-800 group-hover:text-red-600 transition-colors truncate">{item.title}</p>
                  <p className="text-xs text-gray-400 mt-1">
                    {[...new Set(item.creators.map(c => c.name))].slice(0, 2).join(', ')}
                    {item.creators.length > 2 && ' et al.'}
                  </p>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  {(item.tags ?? []).length > 0 && (
                    <span className="hidden sm:inline-flex px-2 py-0.5 text-[10px] bg-gray-100 text-gray-500 rounded-full">
                      {(item.tags ?? []).length} tags
                    </span>
                  )}
                  <span className="text-xs text-gray-300 whitespace-nowrap">{item.date?.slice(0, 4) || '-'}</span>
                  <ArrowUpRight className="w-3.5 h-3.5 text-gray-300 group-hover:text-red-400 transition-colors" />
                </div>
              </a>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
