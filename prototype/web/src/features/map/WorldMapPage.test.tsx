import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
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

test('updates a region and keeps the change in project state', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.clear(screen.getByLabelText('区域简介'))
  await user.type(screen.getByLabelText('区域简介'), '北方大陆，常年受寒潮影响。')

  expect(useStudioStore.getState().projects[0].worldRegions.find((item) => item.id === 'region-north-land')?.summary).toBe('北方大陆，常年受寒潮影响。')
})

test('replaces an empty region name before leaving the field', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.clear(screen.getByLabelText('区域名称'))
  await user.tab()

  expect(screen.getByLabelText('区域名称')).toHaveValue('未命名区域')
  expect(screen.getByRole('status')).toHaveTextContent('区域名称不能为空')
})

test('blocks deletion when a region still has children', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.click(screen.getByRole('button', { name: '删除区域' }))

  expect(screen.getByRole('status')).toHaveTextContent('请先移动或删除下级区域')
})

test('creates a child region under the current map focus', async () => {
  vi.spyOn(window, 'prompt').mockReturnValue('东陆')
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '新增区域' }))

  expect(useStudioStore.getState().projects[0].worldRegions.some((item) => item.name === '东陆' && item.parentId === 'region-world')).toBe(true)
})

test('creates a world root for a project without map data', async () => {
  useStudioStore.getState().updateProject({ worldRegions: [], worldMapState: { mode: 'map', focusRegionId: null } })
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '创建世界根节点' }))

  expect(useStudioStore.getState().projects[0].worldRegions[0]).toMatchObject({ name: '新世界', type: 'world', parentId: null })
})
