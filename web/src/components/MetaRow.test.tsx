import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { User } from 'lucide-react'
import MetaRow from './MetaRow'

describe('MetaRow', () => {
  it('renders label and value text', () => {
    render(<MetaRow icon={<User className="w-4 h-4" />} label="作者" value="Smith et al." />)
    expect(screen.getByText('作者')).toBeInTheDocument()
    expect(screen.getByText('Smith et al.')).toBeInTheDocument()
  })

  it('renders value as ReactNode (link)', () => {
    render(
      <MetaRow
        icon={<User className="w-4 h-4" />}
        label="DOI"
        value={<a href="https://doi.org/10.123/test">10.123/test</a>}
      />,
    )
    const link = screen.getByRole('link', { name: /10\.123\/test/i })
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute('href', 'https://doi.org/10.123/test')
  })

  it('renders icon element', () => {
    const { container } = render(
      <MetaRow icon={<User className="w-4 h-4" />} label="Type" value="journal" />,
    )
    expect(container.querySelector('svg')).toBeTruthy()
  })
})
