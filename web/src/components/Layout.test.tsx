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
})
