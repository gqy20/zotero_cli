interface MetaRowProps {
  icon: React.ReactNode
  label: string
  value: React.ReactNode
}

export default function MetaRow({ icon, label, value }: MetaRowProps) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-gray-300 mt-0.5">{icon}</span>
      <div className="min-w-0">
        <dt className="text-[11px] font-medium text-gray-400 uppercase tracking-wider">{label}</dt>
        <dd className="text-sm text-gray-800 mt-0.5">{value}</dd>
      </div>
    </div>
  )
}
