import { createDemoProject } from '../domain/demoData'
import { useStudioStore } from './useStudioStore'

beforeEach(() => useStudioStore.getState().resetDemo())

test('creates, duplicates, selects, and deletes a project', () => {
  const createdId = useStudioStore.getState().createProject({
    title: '星海遗民',
    genre: '科幻',
    summary: '寻找失落母星',
  })
  expect(useStudioStore.getState().selectedProjectId).toBe(createdId)
  const copiedId = useStudioStore.getState().duplicateProject(createdId)
  expect(useStudioStore.getState().projects.find((project) => project.id === copiedId)?.title).toContain('副本')
  useStudioStore.getState().deleteProject(copiedId)
  expect(useStudioStore.getState().projects.some((project) => project.id === copiedId)).toBe(false)
})

test('restoring a snapshot preserves a pre-restore snapshot', () => {
  const project = createDemoProject()
  useStudioStore.setState({ projects: [project], selectedProjectId: project.id })
  const snapshotId = useStudioStore.getState().createSnapshot('基线')
  useStudioStore.getState().updateProject({ title: '修改后的标题' })
  useStudioStore.getState().restoreSnapshot(snapshotId)
  const restored = useStudioStore.getState().projects[0]
  expect(restored.title).toBe(project.title)
  expect(restored.snapshots.some((snapshot) => snapshot.kind === 'pre-restore')).toBe(true)
})
