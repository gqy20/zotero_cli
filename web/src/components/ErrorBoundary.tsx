import { Component, type ReactNode, type ErrorInfo } from 'react'
import { logger } from '@/lib/logger'

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
        <div className="flex flex-col items-center justify-center min-h-[200px] p-6 text-center">
          <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
            <span className="text-xl">!</span>
          </div>
          <h2 className="text-lg font-semibold text-gray-900 mb-1">Something went wrong</h2>
          <p className="text-sm text-gray-500 mb-4 max-w-md">
            An unexpected error occurred while rendering this component.
          </p>
          {err && typeof (import.meta as any)?.env?.DEV !== 'undefined' && (
            <pre className="text-xs text-left bg-gray-900 text-red-300 p-3 rounded-md mb-4 max-w-lg overflow-auto">
              {err.message}
              {err.stack}
            </pre>
          )}
          <button
            onClick={this.handleReset}
            className="px-4 py-2 text-sm bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors"
          >
            Try again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
