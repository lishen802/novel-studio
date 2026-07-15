import type { WorldRegion } from '../../domain/types'

export function getRegionChildren(regions: WorldRegion[], parentId: string | null): WorldRegion[] {
  return regions.filter((item) => item.parentId === parentId).sort((a, b) => a.order - b.order)
}

export function getRegionPath(regions: WorldRegion[], id: string | null): WorldRegion[] {
  const path: WorldRegion[] = []
  const visited = new Set<string>()
  let current = id ? regions.find((item) => item.id === id) : undefined

  while (current && !visited.has(current.id)) {
    visited.add(current.id)
    path.unshift(current)
    current = current.parentId ? regions.find((item) => item.id === current?.parentId) : undefined
  }

  return path
}

export function isDescendant(regions: WorldRegion[], ancestorId: string, candidateId: string): boolean {
  return getRegionPath(regions, candidateId).some((item) => item.id === ancestorId)
}

export function canMoveRegion(regions: WorldRegion[], id: string, parentId: string | null): boolean {
  return parentId !== id && !(parentId && isDescendant(regions, id, parentId))
}

export function canDeleteRegion(regions: WorldRegion[], id: string): boolean {
  return !regions.some((item) => item.parentId === id)
}
