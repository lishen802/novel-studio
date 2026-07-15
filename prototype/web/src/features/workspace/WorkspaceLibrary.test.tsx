import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App } from '../../app/App'
import { useStudioStore } from '../../store/useStudioStore'

beforeEach(() => useStudioStore.getState().resetDemo())

test('creates a project and opens its dashboard', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '作品库' }))
  await user.click(screen.getByRole('button', { name: '新建作品' }))
  await user.type(screen.getByLabelText('书名'), '星海遗民')
  await user.type(screen.getByLabelText('题材'), '科幻')
  await user.click(screen.getByRole('button', { name: '创建并进入' }))
  expect(screen.getByRole('heading', { name: '星海遗民' })).toBeVisible()
  expect(screen.getByRole('button', { name: '创作驾驶舱' })).toBeVisible()
})
