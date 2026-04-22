import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import LoadingSpinner from './LoadingSpinner'

describe('LoadingSpinner', () => {
  it('renders spinner animation by default', () => {
    render(<LoadingSpinner />)
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeTruthy()
  })

  it('renders default text label "加载中..."', () => {
    render(<LoadingSpinner />)
    expect(screen.getByText('加载中...')).toBeInTheDocument()
  })

  it('renders custom message when provided', () => {
    render(<LoadingSpinner message="搜索中..." />)
    expect(screen.getByText('搜索中...')).toBeInTheDocument()
    expect(screen.queryByText('加载中...')).not.toBeInTheDocument()
  })

  it('applies custom className', () => {
    const { container } = render(<LoadingSpinner className="py-12" />)
    expect((container.firstChild as Element)?.classList.contains('py-12')).toBe(true)
  })
})
