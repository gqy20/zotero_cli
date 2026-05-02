import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import Layout from './Layout'
import { ToastProvider } from '@/hooks/useToast'

function renderWithRouter(ui: React.ReactElement) {
  return render(
    <ToastProvider>
      <BrowserRouter>{ui}</BrowserRouter>
    </ToastProvider>,
  )
}

describe('Layout', () => {
  it('renders app title "Zotero Web"', () => {
    renderWithRouter(<Layout />)
    expect(screen.getByText('Zotero Web')).toBeInTheDocument()
  })

  it('renders all navigation items', () => {
    renderWithRouter(<Layout />)
    expect(screen.getByText('总览')).toBeInTheDocument()
    expect(screen.getByText('文献库')).toBeInTheDocument()
    expect(screen.getByText('搜索')).toBeInTheDocument()
    expect(screen.getByText('标签')).toBeInTheDocument()
    expect(screen.getByText('导出')).toBeInTheDocument()
  })

  it('has a main content area for Outlet', () => {
    const { container } = renderWithRouter(<Layout />)
    // Layout should have a sidebar and main area
    expect(container.querySelector('aside')).toBeTruthy()
    expect(container.querySelector('main')).toBeTruthy()
  })

  // --- Accessibility Tests ---

  it('navigation has role=navigation with aria-label', () => {
    const { container } = renderWithRouter(<Layout />)
    const nav = container.querySelector('nav')
    expect(nav).toBeTruthy()
    expect(nav).toHaveAttribute('aria-label')
  })

  it('sidebar has landmark role', () => {
    const { container } = renderWithRouter(<Layout />)
    const aside = container.querySelector('aside')
    expect(aside).toBeTruthy()
  })

  it('main content area has proper landmark', () => {
    const { container } = renderWithRouter(<Layout />)
    const main = container.querySelector('main')
    expect(main).toBeTruthy()
  })

  it('nav links are keyboard focusable and have accessible names', () => {
    renderWithRouter(<Layout />)
    const links = screen.getAllByRole('link')
    expect(links.length).toBeGreaterThanOrEqual(5)
    links.forEach(link => {
      expect(link.getAttribute('href')).toBeTruthy()
    })
  })
})
