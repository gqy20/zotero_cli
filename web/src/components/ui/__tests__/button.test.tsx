import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Button } from '../button'

describe('Button', () => {
  it('renders children as text', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: /click me/i })).toBeInTheDocument()
  })

  it('applies variant classes', () => {
    const { container } = render(<Button variant="destructive">Delete</Button>)
    const btn = container.querySelector('button')
    expect(btn).toBeTruthy()
    expect(btn?.className).toContain('bg-red-600')
  })

  it('accepts className prop', () => {
    const { container } = render(<Button className="custom-class">Test</Button>)
    expect(container.querySelector('.custom-class')).toBeTruthy()
  })

  it('renders as child of <a> when asChild is used', () => {
    render(
      <Button asChild>
        <a href="/test">Link</a>
      </Button>,
    )
    expect(screen.getByRole('link', { name: /link/i })).toBeInTheDocument()
  })

  it('is disabled when disabled prop is set', () => {
    render(<Button disabled>Disabled</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
  })
})
