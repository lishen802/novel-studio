import type { WorldRegion } from '../../domain/types'
import { canDeleteRegion, canMoveRegion, getRegionChildren, getRegionPath } from './worldRegionModel'

const base = {
  direction: 'center', order: 1, summary: '', terrain: '', climate: '', rules: '', canonStatus: 'author-confirmed',
  adjacentRegionIds: [], travelNotes: '', locationIds: [], factionIds: [], characterIds: [], eventIds: [], chapterIds: [], factIds: [],
  layout: { x: 0, y: 0, width: 100, height: 100 },
} satisfies Omit<WorldRegion, 'id' | 'parentId' | 'name' | 'type'>

const regions: WorldRegion[] = [
  { ...base, id: 'world', parentId: null, name: '镜海世界', type: 'world' },
  { ...base, id: 'north-land', parentId: 'world', name: '北陆', type: 'continent', order: 2 },
  { ...base, id: 'central-sea', parentId: 'world', name: '中央雾海', type: 'sea', order: 1 },
  { ...base, id: 'fog-state', parentId: 'north-land', name: '雾港州', type: 'state' },
  { ...base, id: 'fog-city', parentId: 'fog-state', name: '雾港城', type: 'city' },
]

test('returns children in display order', () => {
  expect(getRegionChildren(regions, 'world').map((item) => item.name)).toEqual(['中央雾海', '北陆'])
})

test('builds an ordered breadcrumb path', () => {
  expect(getRegionPath(regions, 'fog-city').map((item) => item.name)).toEqual(['镜海世界', '北陆', '雾港州', '雾港城'])
})

test('rejects a descendant as a new parent', () => {
  expect(canMoveRegion(regions, 'north-land', 'fog-city')).toBe(false)
  expect(canMoveRegion(regions, 'fog-city', 'central-sea')).toBe(true)
})

test('only allows leaf deletion', () => {
  expect(canDeleteRegion(regions, 'north-land')).toBe(false)
  expect(canDeleteRegion(regions, 'fog-city')).toBe(true)
})
