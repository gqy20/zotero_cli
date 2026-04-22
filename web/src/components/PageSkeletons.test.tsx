import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DashboardSkeleton } from './PageSkeletons'
import { LibrarySkeleton } from './PageSkeletons'
import { ItemDetailSkeleton } from './PageSkeletons'
import { SearchSkeleton } from './PageSkeletons'
import { TagsSkeleton } from './PageSkeletons'
import { ExportSkeleton } from './PageSkeletons'

describe('PageSkeletons', () => {
  it('DashboardSkeleton renders 4 stat cards', () => {
    const { container } = render(<DashboardSkeleton />)
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThanOrEqual(8)
  })

  it('LibrarySkeleton renders sidebar and table rows', () => {
    const { container } = render(<LibrarySkeleton />)
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThanOrEqual(6)
  })

  it('ItemDetailSkeleton renders title and metadata grid', () => {
    const { container } = render(<ItemDetailSkeleton />)
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThanOrEqual(6)
  })

  it('SearchSkeleton renders search bar area', () => {
    const { container } = render(<SearchSkeleton />)
    expect(container.querySelector('.animate-pulse')).toBeTruthy()
  })

  it('TagsSkeleton renders tag areas', () => {
    const { container } = render(<TagsSkeleton />)
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThanOrEqual(3)
  })

  it('ExportSkeleton renders format cards and list', () => {
    const { container } = render(<ExportSkeleton />)
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThanOrEqual(5)
  })
})
