import type { LucideIcon } from 'lucide-react'

interface StatStyle {
  bg: string
  shadow: string
  iconBg: string
  iconColor: string
}

interface StatCardProps {
  title: string
  value: number | string
  Icon: LucideIcon
  style: StatStyle
}

export default function StatCard({ title, value, Icon, style }: StatCardProps) {
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
