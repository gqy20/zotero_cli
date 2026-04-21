import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ErrorBoundary from './ErrorBoundary'

// Suppress console.error for expected errors
const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

function ThrowComponent(): React.ReactElement {
  throw new Error('test crash')
}

function NormalComponent() {
  return <div>I work fine</div>
}

describe('ErrorBoundary', () => {
  afterEach(() => {
    consoleSpy.mockClear()
  })

  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <NormalComponent />
      </ErrorBoundary>,
    )
    expect(screen.getByText('I work fine')).toBeInTheDocument()
  })

  it('catches rendering errors and shows fallback UI', () => {
    render(
      <ErrorBoundary>
        <ThrowComponent />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument()
  })

  it('shows error details in development', () => {
    render(
      <ErrorBoundary>
        <ThrowComponent />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/test crash/)).toBeInTheDocument()
  })

  it('provides a reset button to retry', async () => {
    const user = userEvent.setup()
    let shouldThrow = true

    function MaybeThrow() {
      if (shouldThrow) throw new Error('retryable')
      return <div>Recovered!</div>
    }

    render(
      <ErrorBoundary onReset={() => { shouldThrow = false }}>
        <MaybeThrow />
      </ErrorBoundary>,
    )

    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument()

    const resetBtn = screen.getByRole('button')
    await user.click(resetBtn)

    expect(screen.getByText('Recovered!')).toBeInTheDocument()
  })
})
