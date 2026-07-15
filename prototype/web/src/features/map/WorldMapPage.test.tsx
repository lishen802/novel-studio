import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App } from '../../app/App'
import { useStudioStore } from '../../store/useStudioStore'

beforeEach(() => useStudioStore.getState().resetDemo())

test('switches between schematic map and hierarchy tree while keeping selection', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  expect(screen.getByRole('heading', { name: '北陆' })).toBeVisible()

  await user.click(screen.getByRole('button', { name: '层级结构' }))

  expect(screen.getByRole('treeitem', { name: /北陆/ })).toHaveAttribute('aria-selected', 'true')
})

test('drills into a region and returns through breadcrumbs', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.click(screen.getByRole('button', { name: '进入北陆' }))
  expect(screen.getByRole('button', { name: '雾港州' })).toBeVisible()

  await user.click(screen.getByRole('button', { name: '镜海世界' }))

  expect(screen.getByRole('button', { name: '南陆' })).toBeVisible()
})
