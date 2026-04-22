import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SearchInput from './SearchInput'

describe('SearchInput', () => {
  it('renders input with placeholder', () => {
    render(<SearchInput placeholder="搜索文献..." />)
    expect(screen.getByPlaceholderText('搜索文献...')).toBeInTheDocument()
  })

  it('calls onChange when user types', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<SearchInput placeholder="Search" value="" onChange={onChange} />)
    await user.type(screen.getByRole('searchbox'), 'hello')
    expect(onChange).toHaveBeenCalled()
  })

  it('displays controlled value', () => {
    render(<SearchInput value="existing query" onChange={() => {}} />)
    const input = screen.getByRole('searchbox') as HTMLInputElement
    expect(input.value).toBe('existing query')
  })

  it('renders search icon', () => {
    const { container } = render(<SearchInput placeholder="Search" />)
    expect(container.querySelector('svg')).toBeTruthy()
  })

  it('applies prominent variant styling with rounded-2xl', () => {
    const { container } = render(
      <SearchInput placeholder="Search" variant="prominent" value="test" onChange={() => {}} />,
    )
    const input = container.querySelector('input')
    expect(input?.className).toContain('rounded-2xl')
  })
})
