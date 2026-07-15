import type { StateStorage } from 'zustand/middleware'

export const STORAGE_KEY = 'novel-studio-data-v1'

export function migrateStudioState(value: unknown): unknown {
  if (!value || typeof value !== 'object') return value
  const state = value as Record<string, unknown>
  if (!Array.isArray(state.projects)) return value
  return {
    ...state,
    projects: state.projects.map((project) => {
      if (!project || typeof project !== 'object') return project
      const record = project as Record<string, unknown>
      return {
        ...record,
        worldRegions: Array.isArray(record.worldRegions) ? record.worldRegions : [],
        worldMapState: record.worldMapState && typeof record.worldMapState === 'object'
          ? record.worldMapState
          : { mode: 'map', focusRegionId: null },
      }
    }),
  }
}

export function createSafeStorage(storage: Storage): StateStorage {
  return {
    getItem(name) {
      const raw = storage.getItem(name)
      if (!raw) return null
      try {
        JSON.parse(raw)
        return raw
      } catch {
        try { storage.setItem(`${name}.corrupt.${Date.now()}`, raw) } catch { /* recovery backup is best-effort */ }
        storage.removeItem(name)
        return null
      }
    },
    setItem(name, value) {
      const temporary = `${name}.temporary`
      storage.setItem(temporary, value)
      storage.setItem(name, value)
      storage.removeItem(temporary)
    },
    removeItem(name) { storage.removeItem(name) },
  }
}
