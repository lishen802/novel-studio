import type { StateStorage } from 'zustand/middleware'

export const STORAGE_KEY = 'novel-studio-data-v1'

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
