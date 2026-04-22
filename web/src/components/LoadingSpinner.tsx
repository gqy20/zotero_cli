interface LoadingSpinnerProps {
  message?: string
  className?: string
}

export default function LoadingSpinner({ message = '加载中...', className = '' }: LoadingSpinnerProps) {
  return (
    <div className={`flex items-center gap-3 py-12 justify-center text-sm text-gray-400 ${className}`}>
      <div className="w-4 h-4 border-2 border-gray-200 border-t-red-500 rounded-full animate-spin" />
      {message}
    </div>
  )
}
