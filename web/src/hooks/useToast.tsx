import { createContext, useContext, useReducer, useCallback, type ReactNode } from 'react'

export type ToastVariant = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
  id: string
  title?: string
  description?: string
  variant?: ToastVariant
}

type ToastAction =
  | { type: 'ADD'; toast: Toast }
  | { type: 'REMOVE'; id: string }

interface ToastState {
  toasts: Toast[]
}

const AUTO_DISMISS_MS = 4000

let counter = 0
function genId() {
  counter++
  return `toast-${counter}-${Date.now()}`
}

function toastReducer(state: ToastState, action: ToastAction): ToastState {
  switch (action.type) {
    case 'ADD':
      return { toasts: [...state.toasts, action.toast] }
    case 'REMOVE':
      return { toasts: state.toasts.filter(t => t.id !== action.id) }
    default:
      return state
  }
}

interface ToastContextValue {
  toasts: Toast[]
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(toastReducer, { toasts: [] })

  const addToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = genId()
    dispatch({ type: 'ADD', toast: { ...toast, id, variant: toast.variant || 'info' } })
    setTimeout(() => dispatch({ type: 'REMOVE', id }), AUTO_DISMISS_MS)
  }, [])

  const removeToast = useCallback((id: string) => {
    dispatch({ type: 'REMOVE', id })
  }, [])

  return (
    <ToastContext.Provider value={{ toasts: state.toasts, addToast, removeToast }}>
      {children}
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within <ToastProvider>')
  return {
    ...ctx,
    success: (description: string) => ctx.addToast({ description, variant: 'success' }),
    error: (description: string) => ctx.addToast({ description, variant: 'error' }),
    warning: (description: string) => ctx.addToast({ description, variant: 'warning' }),
    info: (description: string) => ctx.addToast({ description, variant: 'info' }),
    toast: (toast: Omit<Toast, 'id'>) => ctx.addToast(toast),
  }
}
