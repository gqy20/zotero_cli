import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BookOpen } from 'lucide-react'
import StatCard from './StatCard'

const defaultStyle = {
  bg: 'from-blue-500 to-indigo-600',
  shadow: 'shadow-blue-500/20',
  iconBg: 'bg-blue-500/10',
  iconColor: 'text-blue-600',
}

describe('StatCard', () => {
  it('renders title and value', () => {
    render(<StatCard title="文献数" value={42} Icon={BookOpen} style={defaultStyle} />)
    expect(screen.getByText('文献数')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('formats number value with locale string', () => {
    render(<StatCard title="Count" value={1000} Icon={BookOpen} style={defaultStyle} />)
    expect(screen.getByText('1,000')).toBeInTheDocument()
  })

  it('renders string value as-is', () => {
    render(<StatCard title="Label" value="-" Icon={BookOpen} style={defaultStyle} />)
    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('renders the Icon component in icon container', () => {
    const { container } = render(
      <StatCard title="Test" value={1} Icon={BookOpen} style={defaultStyle} />,
    )
    const iconContainer = container.querySelector('.rounded-xl')
    expect(iconContainer).toBeTruthy()
    expect(iconContainer?.querySelector('svg')).toBeTruthy()
  })
})
