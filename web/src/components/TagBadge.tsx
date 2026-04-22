import { Tag } from 'lucide-react'

interface TagBadgeProps {
  tags: string[]
  maxVisible?: number
  variant?: 'default' | 'styled' | 'count'
}

export default function TagBadge({ tags, maxVisible = 3, variant = 'default' }: TagBadgeProps) {
  if (!tags.length) return null

  if (variant === 'count') {
    return (
      <span className="hidden sm:inline-flex px-2 py-0.5 text-[10px] bg-gray-100 text-gray-500 rounded-full">
        {tags.length} tags
      </span>
    )
  }

  if (variant === 'styled') {
    return (
      <div className="flex items-center gap-2 flex-wrap">
        <Tag className="w-3.5 h-3.5 text-gray-300" />
        {tags.map(tag => (
          <span key={tag} className="px-2.5 py-1 text-xs bg-gradient-to-r from-red-50 to-rose-50 text-red-600 rounded-lg border border-red-100 font-medium">
            {tag}
          </span>
        ))}
      </div>
    )
  }

  const visible = tags.slice(0, maxVisible)
  const overflow = tags.length - maxVisible

  return (
    <div className="flex gap-1 mt-1.5 flex-wrap">
      {visible.map(tag => (
        <span key={tag} className="inline-block px-1.5 py-0.5 text-[10px] bg-gray-100 text-gray-400 rounded-md">{tag}</span>
      ))}
      {overflow > 0 && (
        <span className="inline-block px-1.5 py-0.5 text-[10px] text-gray-300">+{overflow}</span>
      )}
    </div>
  )
}
