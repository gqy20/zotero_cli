import { useState } from 'react'
import { Search, Sparkles } from 'lucide-react'

interface SearchInputProps {
  placeholder?: string
  value?: string
  onChange?: (value: string) => void
  variant?: 'default' | 'prominent'
  autoFocus?: boolean
  className?: string
}

export default function SearchInput({
  placeholder = '搜索文献...',
  value: controlledValue,
  onChange,
  variant = 'default',
  autoFocus = false,
  className = '',
}: SearchInputProps) {
  const [internalValue, setInternalValue] = useState('')
  const value = controlledValue !== undefined ? controlledValue : internalValue
  const hasValue = value.length > 0

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value
    if (controlledValue === undefined) setInternalValue(newValue)
    onChange?.(newValue)
  }

  if (variant === 'prominent') {
    return (
      <div className="relative">
        <div className="absolute left-5 top-1/2 -translate-y-1/2 flex items-center gap-2 pointer-events-none">
          {hasValue && <Sparkles className="w-3.5 h-3.5 text-red-400" />}
          <Search className={`w-5 h-5 ${hasValue ? 'text-red-400' : 'text-gray-300'} transition-colors`} />
        </div>
        <input
          type="search"
          value={value}
          onChange={handleChange}
          placeholder={placeholder}
          autoFocus={autoFocus}
          className={`w-full pl-13 pr-5 py-4 text-sm bg-white border rounded-2xl focus:outline-none transition-all duration-200 shadow-sm ${
            hasValue
              ? 'border-red-200 ring-4 ring-red-500/5 focus:border-red-400 focus:ring-4 focus:ring-red-500/10'
              : 'border-gray-200 focus:ring-2 focus:ring-red-500/20 focus:border-red-300'
          } ${className}`}
        />
      </div>
    )
  }

  return (
    <div className={`relative max-w-md flex-1 ${className}`}>
      <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-300" />
      <input
        type="search"
        value={value}
        onChange={handleChange}
        placeholder={placeholder}
        className="w-full pl-10 pr-4 py-2.5 text-sm bg-gray-50 border-0 rounded-xl focus:outline-none focus:ring-2 focus:ring-red-500/20 focus:bg-white transition-all placeholder:text-gray-300"
      />
    </div>
  )
}
