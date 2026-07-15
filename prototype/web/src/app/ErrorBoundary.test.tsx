import { render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

test('shows recovery actions after a render failure', () => {
  const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
  function Broken(): never { throw new Error('render failed') }
  render(<ErrorBoundary><Broken /></ErrorBoundary>)
  expect(screen.getByRole('heading', { name: '工作区遇到错误' })).toBeVisible()
  expect(screen.getByRole('button', { name: '重试渲染' })).toBeVisible()
  expect(screen.getByRole('button', { name: '重置演示数据' })).toBeVisible()
  spy.mockRestore()
})
