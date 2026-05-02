import { Component, type ReactNode, type ErrorInfo } from 'react'
import { logger } from '@/lib/logger'
import { RefreshCw, AlertTriangle } from 'lucide-react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  onReset?: () => void
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    logger.error('React rendering error', {
      message: error.message,
      stack: error.stack,
      componentStack: info.componentStack,
    })
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
    this.props.onReset?.()
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      const err = this.state.error
      return (
        <div className="flex flex-col items-center justify-center min-h-[200px] p-6 text-center" role="alert">
          <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
            <AlertTriangle className="w-6 h-6 text-red-500" aria-hidden="true" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 mb-1">页面渲染出错</h2>
          <p className="text-sm text-gray-500 mb-4 max-w-md">
            页面加载时发生了意外错误，请尝试刷新或重试。
          </p>
          {err && typeof (import.meta as any)?.env?.DEV !== 'undefined' && (
            <pre className="text-xs text-left bg-gray-900 text-red-300 p-3 rounded-md mb-4 max-w-lg overflow-auto">
              {err.message}
              {err.stack}
            </pre>
          )}
          <button
            onClick={this.handleReset}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" aria-hidden="true" />
            重试
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
