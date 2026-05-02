import { RefreshCw, AlertTriangle } from 'lucide-react'

interface ErrorFallbackProps {
  message?: string
  onRetry?: () => void
  className?: string
}

const friendlyMessages: Record<string, string> = {
  'Failed to fetch': '无法连接到服务器，请检查网络连接',
  'Network error': '网络连接失败，请检查网络后重试',
  'API error': '服务暂时不可用，请稍后重试',
}

function getFriendlyMessage(error: string): string {
  for (const [key, value] of Object.entries(friendlyMessages)) {
    if (error.toLowerCase().includes(key.toLowerCase())) return value
  }
  return '数据加载失败，请稍后重试'
}

export default function ErrorFallback({ message, onRetry, className = '' }: ErrorFallbackProps) {
  const displayMessage = message ? getFriendlyMessage(message) : '数据加载失败，请稍后重试'

  return (
    <div className={`flex flex-col items-center justify-center py-12 ${className}`} role="alert">
      <div className="w-12 h-12 rounded-full bg-amber-50 flex items-center justify-center mb-4">
        <AlertTriangle className="w-6 h-6 text-amber-500" aria-hidden="true" />
      </div>
      <p className="text-sm text-gray-600 font-medium mb-4">{displayMessage}</p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-white border border-gray-200 rounded-lg text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition-all"
        >
          <RefreshCw className="w-3.5 h-3.5" aria-hidden="true" />
          重试
        </button>
      )}
    </div>
  )
}
