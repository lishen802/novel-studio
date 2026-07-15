import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { App } from '../../app/App'
import { useStudioStore } from '../../store/useStudioStore'

beforeEach(() => useStudioStore.getState().resetDemo())

test('edits and persists the current chapter body', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '正文与场景' }))
  const editor = screen.getByLabelText('章节正文')
  await user.clear(editor)
  await user.type(editor, '潮水越过警戒线。')
  expect(useStudioStore.getState().projects[0].chapters.find((item) => item.id === 'demo-chapter-38')?.content).toBe('潮水越过警戒线。')
})

test('accepts an evidence-based review issue', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: /^审稿中心/ }))
  await user.click(screen.getAllByRole('button', { name: '接受建议' })[0])
  expect(useStudioStore.getState().projects[0].reviews[0].disposition).toBe('accepted')
})

test('changes the model assigned to an agent', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '模型与智能体' }))
  await user.selectOptions(screen.getByLabelText('剧情规划师模型'), 'gpt-5.3')
  expect(useStudioStore.getState().projects[0].agents[0].model).toBe('gpt-5.3')
})

test('activates the branch it just created', async () => {
  vi.spyOn(window, 'prompt').mockReturnValue('替代剧情线')
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '版本历史' }))
  await user.click(screen.getByRole('button', { name: '创建分支' }))
  const project = useStudioStore.getState().projects[0]
  const created = project.branches.at(-1)
  expect(created?.name).toBe('替代剧情线')
  expect(project.activeBranchId).toBe(created?.id)
})
