import { X, CheckCircle2, AlertCircle, AlertTriangle, Info } from 'lucide-react'
import { useToast } from '@/hooks/useToast'
import type { ToastVariant } from '@/hooks/useToast'

const variantConfig: Record<ToastVariant, { icon: typeof CheckCircle2; border: string; bg: string; iconColor: string }> = {
  success: { icon: CheckCircle2, border: 'border-emerald-200', bg: 'bg-emerald-50', iconColor: 'text-emerald-500' },
  error: { icon: AlertCircle, border: 'border-red-200', bg: 'bg-red-50', iconColor: 'text-red-500' },
  warning: { icon: AlertTriangle, border: 'border-amber-200', bg: 'bg-amber-50', iconColor: 'text-amber-500' },
  info: { icon: Info, border: 'border-blue-200', bg: 'bg-blue-50', iconColor: 'text-blue-500' },
}

function ToastItem({ id, title, description, variant = 'info' }: { id: string; title?: string; description?: string; variant?: ToastVariant }) {
  const { removeToast } = useToast()
  const config = variantConfig[variant]
  const Icon = config.icon

  return (
    <div
      role="status"
      className={`flex items-start gap-3 px-4 py-3 rounded-xl border ${config.border} ${config.bg} shadow-lg shadow-black/5 animate-in slide-in-from-right-full duration-300 max-w-sm`}
    >
      <Icon className={`w-5 h-5 shrink-0 mt-0.5 ${config.iconColor}`} />
      <div className="flex-1 min-w-0">
        {title && <p className="text-sm font-semibold text-gray-800">{title}</p>}
        {description && <p className="text-sm text-gray-600 mt-0.5">{description}</p>}
      </div>
      <button
        onClick={() => removeToast(id)}
        aria-label="关闭"
        className="shrink-0 w-5 h-5 rounded-md hover:bg-black/5 flex items-center justify-center text-gray-400 hover:text-gray-600 transition-colors"
      >
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
  )
}

export function Toaster() {
  const { toasts } = useToast()

  if (toasts.length === 0) return null

  return (
    <div className="fixed top-6 right-6 z-[100] flex flex-col gap-2">
      {toasts.map(t => (
        <ToastItem key={t.id} {...t} />
      ))}
    </div>
  )
}
