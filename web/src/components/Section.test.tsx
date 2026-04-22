import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Paperclip } from 'lucide-react'
import Section from './Section'

describe('Section', () => {
  it('renders title', () => {
    render(<Section title="附件">content</Section>)
    expect(screen.getByText('附件')).toBeInTheDocument()
  })

  it('renders children content', () => {
    render(<Section title="Notes"><p data-testid="child">Hello</p></Section>)
    expect(screen.getByTestId('child')).toBeInTheDocument()
  })

  it('renders count badge when count is provided', () => {
    render(<Section title="附件" count={3}>content</Section>)
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('does not render count badge when count is omitted', () => {
    const { container } = render(<Section title="附件">content</Section>)
    const badges = container.querySelectorAll('.bg-gray-100.rounded-full')
    expect(badges.length).toBe(0)
  })

  it('renders icon when provided', () => {
    const { container } = render(
      <Section title="附件" icon={<Paperclip className="w-4 h-4" />}>content</Section>,
    )
    expect(container.querySelector('svg')).toBeTruthy()
  })
})
