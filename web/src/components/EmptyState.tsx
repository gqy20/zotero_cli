import type { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  icon: LucideIcon
  message: string
  description?: string
  className?: string
}

export default function EmptyState({ icon: Icon, message, description, className = '' }: EmptyStateProps) {
  return (
    <div className={`text-center py-16 ${className}`}>
      <Icon className="w-12 h-12 text-gray-200 mx-auto mb-4" />
      <p className="text-gray-500 font-medium">{message}</p>
      {description && (
        <p className="text-sm text-gray-400 mt-1">{description}</p>
      )}
    </div>
  )
}
