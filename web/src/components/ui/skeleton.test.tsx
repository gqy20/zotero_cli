import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Skeleton } from './skeleton'

describe('Skeleton', () => {
  it('renders with default className', () => {
    const { container } = render(<Skeleton className="h-4 w-full" />)
    const el = container.querySelector('.animate-pulse')
    expect(el).toBeTruthy()
    expect(el?.className).toContain('bg-gray-100')
  })

  it('renders circle variant', () => {
    const { container } = render(<Skeleton className="h-10 w-10 rounded-full" />)
    expect(container.querySelector('.rounded-full')).toBeTruthy()
  })

  it('renders multiple skeletons in a row', () => {
    const { container } = render(
      <div>
        <Skeleton className="h-4 w-[200px]" />
        <Skeleton className="h-4 w-[150px]" />
        <Skeleton className="h-4 w-[100px] ml-auto" />
      </div>,
    )
    expect(container.querySelectorAll('.animate-pulse')).toHaveLength(3)
  })
})
