import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, act, fireEvent, waitFor } from '@testing-library/react'
import { Toaster } from '@/components/Toaster'
import { useToast, ToastProvider } from '@/hooks/useToast'

function TestComponent() {
  const toast = useToast()
  return (
    <div>
      <button data-testid="success" onClick={() => toast.success('操作成功')} />
      <button data-testid="error" onClick={() => toast.error('出错了')} />
      <button data-testid="warning" onClick={() => toast.warning('注意')} />
      <button data-testid="info" onClick={() => toast.toast({ description: '提示信息' })} />
      <button data-testid="custom" onClick={() => toast.toast({ title: '自定义', description: '详情', variant: 'info' })} />
    </div>
  )
}

function renderWithProvider() {
  return render(
    <ToastProvider>
      <Toaster />
      <TestComponent />
    </ToastProvider>,
  )
}

describe('useToast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('renders nothing initially', () => {
    renderWithProvider()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows success toast on success() call', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('success'))
    expect(screen.getByText('操作成功')).toBeInTheDocument()
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })

  it('shows error toast on error() call', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('error'))
    expect(screen.getByText('出错了')).toBeInTheDocument()
  })

  it('shows warning toast on warning() call', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('warning'))
    expect(screen.getByText('注意')).toBeInTheDocument()
  })

  it('shows custom toast via toast()', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('custom'))
    expect(screen.getByText('自定义')).toBeInTheDocument()
    expect(screen.getByText('详情')).toBeInTheDocument()
  })

  it('stacks multiple toasts', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('success'))
    fireEvent.click(screen.getByTestId('error'))
    expect(screen.getAllByRole('status')).toHaveLength(2)
  })

  it('auto-dismisses after duration', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('success'))
    expect(screen.getByText('操作成功')).toBeInTheDocument()

    act(() => { vi.advanceTimersByTime(5000) })

    expect(screen.queryByText('操作成功')).not.toBeInTheDocument()
  })

  it('dismisses on close button click', () => {
    renderWithProvider()
    fireEvent.click(screen.getByTestId('success'))
    expect(screen.getByText('操作成功')).toBeInTheDocument()

    const closeBtn = screen.getByLabelText('关闭')
    fireEvent.click(closeBtn)

    expect(screen.queryByText('操作成功')).not.toBeInTheDocument()
  })

  it('applies variant-specific styling', () => {
    renderWithProvider()

    fireEvent.click(screen.getByTestId('success'))
    const successToast = screen.getByText('操作成功').closest('[role="status"]')
    expect(successToast?.className).toContain('border-emerald')

    fireEvent.click(screen.getByTestId('error'))
    const errorToast = screen.getByText('出错了').closest('[role="status"]')
    expect(errorToast?.className).toContain('border-red')
  })
})
