interface SectionProps {
  title: string
  icon?: React.ReactNode
  count?: number
  children: React.ReactNode
}

export default function Section({ title, icon, count, children }: SectionProps) {
  return (
    <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
      <div className="px-6 py-4 border-b border-gray-100 flex items-center gap-2.5">
        {icon}
        <h2 className="font-semibold text-sm text-gray-900">{title}</h2>
        {count != null && (
          <span className="ml-auto text-xs px-2 py-0.5 bg-gray-100 text-gray-500 rounded-full tabular-nums">{count}</span>
        )}
      </div>
      <div className="p-6">{children}</div>
    </div>
  )
}
