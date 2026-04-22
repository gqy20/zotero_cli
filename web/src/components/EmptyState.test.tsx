import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BookOpen } from 'lucide-react'
import EmptyState from './EmptyState'

describe('EmptyState', () => {
  it('renders icon and primary message', () => {
    render(<EmptyState icon={BookOpen} message="暂无文献" />)
    expect(screen.getByText('暂无文献')).toBeInTheDocument()
    expect(document.querySelector('svg')).toBeTruthy()
  })

  it('renders secondary description when provided', () => {
    render(
      <EmptyState icon={BookOpen} message="未找到结果" description="尝试其他关键词" />,
    )
    expect(screen.getByText('未找到结果')).toBeInTheDocument()
    expect(screen.getByText('尝试其他关键词')).toBeInTheDocument()
  })

  it('does not render description when omitted', () => {
    const { container } = render(<EmptyState icon={BookOpen} message="空" />)
    const paragraphs = container.querySelectorAll('p')
    expect(paragraphs.length).toBe(1)
  })

  it('applies custom className', () => {
    const { container } = render(
      <EmptyState icon={BookOpen} message="test" className="py-16" />,
    )
    expect((container.firstChild as Element)?.classList.contains('py-16')).toBe(true)
  })
})
