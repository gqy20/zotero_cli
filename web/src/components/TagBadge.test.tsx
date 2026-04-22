import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import TagBadge from './TagBadge'

describe('TagBadge', () => {
  const tags = ['machine-learning', 'deep-learning', 'nlp', 'transformers', 'attention']

  it('renders all tags when within limit', () => {
    render(<TagBadge tags={['ml', 'dl']} maxVisible={3} />)
    expect(screen.getByText('ml')).toBeInTheDocument()
    expect(screen.getByText('dl')).toBeInTheDocument()
  })

  it('limits visible tags and shows overflow count', () => {
    render(<TagBadge tags={tags} maxVisible={3} />)
    expect(screen.getByText('machine-learning')).toBeInTheDocument()
    expect(screen.getByText('deep-learning')).toBeInTheDocument()
    expect(screen.getByText('nlp')).toBeInTheDocument()
    expect(screen.getByText('+2')).toBeInTheDocument()
    expect(screen.queryByText('transformers')).not.toBeInTheDocument()
  })

  it('renders nothing when tags array is empty', () => {
    const { container } = render(<TagBadge tags={[]} maxVisible={3} />)
    expect(container.innerHTML.trim()).toBe('')
  })

  it('renders compact count mode (no individual tag names)', () => {
    render(<TagBadge tags={tags} variant="count" />)
    expect(screen.getByText('5 tags')).toBeInTheDocument()
    expect(screen.queryByText('machine-learning')).not.toBeInTheDocument()
  })

  it('renders styled variant with gradient badges', () => {
    const { container } = render(<TagBadge tags={['important']} variant="styled" />)
    const badge = container.querySelector('.from-red-50')
    expect(badge).toBeTruthy()
    expect(screen.getByText('important')).toBeInTheDocument()
  })
})
