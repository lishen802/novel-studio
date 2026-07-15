import { createSafeStorage, migrateStudioState, STORAGE_KEY } from './persistence'

test('backs up corrupt JSON and returns null', () => {
  localStorage.setItem(STORAGE_KEY, '{broken')
  const storage = createSafeStorage(localStorage)
  expect(storage.getItem(STORAGE_KEY)).toBeNull()
  expect(Object.keys(localStorage).some((key) => key.startsWith(`${STORAGE_KEY}.corrupt.`))).toBe(true)
})

test('writes valid state through the temporary key', () => {
  const storage = createSafeStorage(localStorage)
  storage.setItem(STORAGE_KEY, '{"state":{"projects":[]},"version":1}')
  expect(localStorage.getItem(STORAGE_KEY)).toContain('projects')
  expect(localStorage.getItem(`${STORAGE_KEY}.temporary`)).toBeNull()
})

test('adds world map defaults to projects saved by the previous prototype', () => {
  const migrated = migrateStudioState({ projects: [{ id: 'legacy-project', title: '旧作品' }] }) as { projects: Array<Record<string, unknown>> }

  expect(migrated.projects[0].worldRegions).toEqual([])
  expect(migrated.projects[0].worldMapState).toEqual({ mode: 'map', focusRegionId: null })
})
