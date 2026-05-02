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

describe('Layout Mobile Responsiveness', () => {
  it('renders mobile menu button for small screens', () => {
    const { container } = renderWithRouter(<Layout />)
    const menuBtn = container.querySelector('button[aria-label*="菜单"]')
    expect(menuBtn).toBeTruthy()
    expect(menuBtn?.className).toContain('md:hidden')
  })

  it('sidebar has responsive positioning', () => {
    const { container } = renderWithRouter(<Layout />)
    const aside = container.querySelector('aside')
    expect(aside).toBeTruthy()
    // Should be hidden on mobile, visible on md+
    expect(aside?.className).toContain('hidden')
    expect(aside?.className).toContain('md:flex')
  })

  it('main content area takes full width on mobile', () => {
    const { container } = renderWithRouter(<Layout />)
    const main = container.querySelector('main')
    expect(main).toBeTruthy()
    expect(main?.className).toContain('flex-1')
  })

  it('nav links have accessible names', () => {
    renderWithRouter(<Layout />)
    // Nav should be in the document (inside sidebar)
    const nav = screen.getByRole('navigation', { name: /主导航/ })
    expect(nav).toBeInTheDocument()
    const links = screen.getAllByRole('link')
    expect(links.length).toBeGreaterThanOrEqual(5)
  })
})
