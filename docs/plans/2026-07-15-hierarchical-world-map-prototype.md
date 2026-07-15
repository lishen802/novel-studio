# Hierarchical World Map Prototype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the image-based map prototype with a persistent hierarchical world model that supports schematic spatial and text-tree views, shared selection, drill-down navigation, editing, and contextual story details.

**Architecture:** Add a backward-compatible `WorldRegion` aggregate to the existing `Project` model, keep hierarchy and validation logic in a focused domain helper, and move the map UI out of the large visualization page into a dedicated feature folder. Both visual modes read and mutate the same region collection through the existing project replacement callback, so localStorage and JSON export continue to work without a second state system.

**Tech Stack:** React 19, TypeScript, Zustand, Zod, Lucide React, Vitest, Testing Library, Playwright, CSS/SVG.

---

### Task 1: Add the world-region model and backward-compatible project parsing

**Files:**
- Modify: `prototype/web/src/domain/types.ts`
- Modify: `prototype/web/src/domain/schema.ts`
- Modify: `prototype/web/src/domain/schema.test.ts`

- [ ] **Step 1: Write failing schema compatibility tests**

Add tests that remove the new fields from a valid demo project and assert that parsing supplies safe defaults:

```ts
test('adds empty world map fields to legacy projects', () => {
  const project = structuredClone(createDemoProject()) as Record<string, unknown>
  delete project.worldRegions
  delete project.worldMapState

  const parsed = parseProject(project)

  expect(parsed.worldRegions).toEqual([])
  expect(parsed.worldMapState).toEqual({ mode: 'map', focusRegionId: null })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd prototype/web
npm test -- --run src/domain/schema.test.ts
```

Expected: FAIL because `worldRegions` and `worldMapState` do not exist.

- [ ] **Step 3: Define the new domain types**

Add these types to `types.ts` and fields to `Project`:

```ts
export type WorldRegionType = 'world' | 'continent' | 'sea' | 'state' | 'island' | 'city' | 'stronghold' | 'terrain' | 'secret-realm' | 'resource'
export type RelativeDirection = 'north' | 'north-east' | 'east' | 'south-east' | 'south' | 'south-west' | 'west' | 'north-west' | 'center' | 'surrounding'

export interface WorldRegion {
  id: string
  parentId: string | null
  name: string
  type: WorldRegionType
  direction: RelativeDirection
  order: number
  summary: string
  terrain: string
  climate: string
  rules: string
  canonStatus: Extract<CanonStatus, 'author-confirmed' | 'text-evidenced' | 'ai-inferred' | 'conflicted'>
  adjacentRegionIds: string[]
  travelNotes: string
  locationIds: string[]
  factionIds: string[]
  characterIds: string[]
  eventIds: string[]
  chapterIds: string[]
  factIds: string[]
  layout: { x: number; y: number; width: number; height: number }
}

// Project fields
worldRegions: WorldRegion[]
worldMapState: { mode: 'map' | 'tree'; focusRegionId: string | null }
```

- [ ] **Step 4: Add Zod defaults for legacy data**

Use focused schemas and defaults in `schema.ts`:

```ts
const WorldRegionSchema = z.object({
  id: z.string().min(1), parentId: z.string().nullable(), name: z.string().trim().min(1),
  type: z.enum(['world', 'continent', 'sea', 'state', 'island', 'city', 'stronghold', 'terrain', 'secret-realm', 'resource']),
  direction: z.enum(['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west', 'center', 'surrounding']),
  order: z.number(), summary: z.string(), terrain: z.string(), climate: z.string(), rules: z.string(),
  canonStatus: z.enum(['author-confirmed', 'text-evidenced', 'ai-inferred', 'conflicted']),
  adjacentRegionIds: z.array(z.string()), travelNotes: z.string(), locationIds: z.array(z.string()),
  factionIds: z.array(z.string()), characterIds: z.array(z.string()), eventIds: z.array(z.string()),
  chapterIds: z.array(z.string()), factIds: z.array(z.string()),
  layout: z.object({ x: z.number(), y: z.number(), width: z.number(), height: z.number() }),
})

worldRegions: z.array(WorldRegionSchema).default([]),
worldMapState: z.object({ mode: z.enum(['map', 'tree']), focusRegionId: z.string().nullable() })
  .default({ mode: 'map', focusRegionId: null }),
```

- [ ] **Step 5: Run schema tests and verify GREEN**

Run `npm test -- --run src/domain/schema.test.ts`.

Expected: all schema tests pass.

- [ ] **Step 6: Commit**

```bash
git add prototype/web/src/domain/types.ts prototype/web/src/domain/schema.ts prototype/web/src/domain/schema.test.ts
git commit -m "feat: add hierarchical world region model"
```

### Task 2: Add coherent demo world data and hierarchy utilities

**Files:**
- Modify: `prototype/web/src/domain/demoData.ts`
- Create: `prototype/web/src/features/map/worldRegionModel.ts`
- Create: `prototype/web/src/features/map/worldRegionModel.test.ts`

- [ ] **Step 1: Write failing hierarchy tests**

Cover ordered children, breadcrumbs, descendant detection, and deletion safety:

```ts
test('builds an ordered breadcrumb path', () => {
  expect(getRegionPath(regions, 'fog-city').map((item) => item.name))
    .toEqual(['镜海世界', '北陆', '雾港州', '雾港城'])
})

test('rejects a descendant as a new parent', () => {
  expect(canMoveRegion(regions, 'north-land', 'fog-city')).toBe(false)
})

test('only allows leaf deletion', () => {
  expect(canDeleteRegion(regions, 'north-land')).toBe(false)
  expect(canDeleteRegion(regions, 'fog-city')).toBe(true)
})
```

- [ ] **Step 2: Run the utility test and verify RED**

Run `npm test -- --run src/features/map/worldRegionModel.test.ts`.

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement focused hierarchy helpers**

Export these pure functions:

```ts
export function getRegionChildren(regions: WorldRegion[], parentId: string | null) {
  return regions.filter((item) => item.parentId === parentId).sort((a, b) => a.order - b.order)
}

export function getRegionPath(regions: WorldRegion[], id: string | null): WorldRegion[] {
  const path: WorldRegion[] = []
  let current = id ? regions.find((item) => item.id === id) : undefined
  const visited = new Set<string>()
  while (current && !visited.has(current.id)) {
    visited.add(current.id)
    path.unshift(current)
    current = current.parentId ? regions.find((item) => item.id === current!.parentId) : undefined
  }
  return path
}

export function isDescendant(regions: WorldRegion[], ancestorId: string, candidateId: string): boolean {
  return getRegionPath(regions, candidateId).some((item) => item.id === ancestorId)
}

export const canMoveRegion = (regions: WorldRegion[], id: string, parentId: string | null) =>
  parentId !== id && !(parentId && isDescendant(regions, id, parentId))

export const canDeleteRegion = (regions: WorldRegion[], id: string) =>
  !regions.some((item) => item.parentId === id)
```

- [ ] **Step 4: Add the “镜海世界” demo hierarchy**

Populate at least the approved root, two continents, central sea, four states, two islands, one forbidden zone, and the three 雾港州 child locations. Link existing factions, characters, events, chapters, and locations where matching data exists.

- [ ] **Step 5: Run utility and schema tests**

Run:

```bash
npm test -- --run src/features/map/worldRegionModel.test.ts src/domain/schema.test.ts
```

Expected: both test files pass.

- [ ] **Step 6: Commit**

```bash
git add prototype/web/src/domain/demoData.ts prototype/web/src/features/map/worldRegionModel.ts prototype/web/src/features/map/worldRegionModel.test.ts
git commit -m "feat: add demo world hierarchy utilities"
```

### Task 3: Build shared selection, tree view, and drill-down behavior

**Files:**
- Create: `prototype/web/src/features/map/WorldMapPage.tsx`
- Create: `prototype/web/src/features/map/WorldMapPage.test.tsx`
- Modify: `prototype/web/src/features/pages/StudioPageRouter.tsx`
- Modify: `prototype/web/src/features/pages/VisualizationPages.tsx`

- [ ] **Step 1: Write failing component tests**

Test view switching, shared selection, drill-down, and breadcrumb return:

```tsx
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
  expect(screen.getByText('雾港州')).toBeVisible()
  await user.click(screen.getByRole('button', { name: '镜海世界' }))
  expect(screen.getByRole('button', { name: '南陆' })).toBeVisible()
})
```

- [ ] **Step 2: Run component tests and verify RED**

Run `npm test -- --run src/features/map/WorldMapPage.test.tsx`.

Expected: FAIL because the new page and controls do not exist.

- [ ] **Step 3: Implement `WorldMapPage` state and toolbar**

Use project-backed mode/focus plus local selection:

```tsx
const root = project.worldRegions.find((item) => item.type === 'world')
const [selectedId, setSelectedId] = useState(project.worldMapState.focusRegionId ?? root?.id ?? '')
const focusId = project.worldMapState.focusRegionId ?? root?.id ?? null
const setMode = (mode: 'map' | 'tree') => onChange({
  ...project,
  worldMapState: { ...project.worldMapState, mode },
})
```

Render accessible `方位地图` and `层级结构` buttons, breadcrumbs from `getRegionPath`, the active view, and a shared inspector.

- [ ] **Step 4: Implement schematic region blocks and the text tree**

Map blocks use the stored layout only for rendering; their accessible button name is the region name. Tree nodes use `role="treeitem"`, `aria-level`, `aria-expanded`, and `aria-selected`. Both invoke the same `setSelectedId` callback.

- [ ] **Step 5: Implement drill-down and breadcrumb navigation**

Double-clicking a non-leaf block and clicking `进入{区域名}` set `worldMapState.focusRegionId`. Breadcrumb buttons set focus to their own region and selection to the same ID.

- [ ] **Step 6: Route the new page and remove the legacy `MapPage` export**

Import `WorldMapPage` from the feature folder in `StudioPageRouter.tsx`. Delete the old upload/image/marker implementation from `VisualizationPages.tsx` and remove now-unused icon imports.

- [ ] **Step 7: Run component tests and verify GREEN**

Run `npm test -- --run src/features/map/WorldMapPage.test.tsx src/features/pages/StudioPages.test.tsx`.

Expected: both files pass.

- [ ] **Step 8: Commit**

```bash
git add prototype/web/src/features/map prototype/web/src/features/pages/StudioPageRouter.tsx prototype/web/src/features/pages/VisualizationPages.tsx
git commit -m "feat: add hierarchical map and tree views"
```

### Task 4: Add contextual inspector, editing, creation, and safe deletion

**Files:**
- Modify: `prototype/web/src/features/map/WorldMapPage.tsx`
- Modify: `prototype/web/src/features/map/WorldMapPage.test.tsx`

- [ ] **Step 1: Write failing mutation tests**

Add tests for details, editing persistence, child creation, and blocked parent deletion:

```tsx
test('updates a region and keeps the change in project state', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.clear(screen.getByLabelText('区域简介'))
  await user.type(screen.getByLabelText('区域简介'), '北方大陆，常年受寒潮影响。')
  expect(useStudioStore.getState().projects[0].worldRegions.find((item) => item.id === 'region-north-land')?.summary)
    .toBe('北方大陆，常年受寒潮影响。')
})

test('blocks deletion when a region still has children', async () => {
  const user = userEvent.setup()
  render(<App />)
  await user.click(screen.getByRole('button', { name: '世界地图' }))
  await user.click(screen.getByRole('button', { name: '北陆' }))
  await user.click(screen.getByRole('button', { name: '删除区域' }))
  expect(screen.getByRole('status')).toHaveTextContent('请先移动或删除下级区域')
})
```

- [ ] **Step 2: Run mutation tests and verify RED**

Run `npm test -- --run src/features/map/WorldMapPage.test.tsx`.

Expected: FAIL on missing form controls and deletion behavior.

- [ ] **Step 3: Implement the contextual inspector**

Render editable name, type, direction, parent, summary, terrain, climate, rules, canon status, and travel notes. Render read-only grouped links for children, locations, factions, characters, events, chapters, and facts by resolving project IDs.

- [ ] **Step 4: Validate edits and parent changes**

Reject blank names. Disable the current node and its descendants in the parent selector. Show a warning when a sibling has the same trimmed name, but allow the value to persist.

- [ ] **Step 5: Implement child creation and leaf-only deletion**

`新增区域` creates a child of the focused region with a generated ID, safe default layout, and author-confirmed status. `删除区域` calls `canDeleteRegion`; if false, show `请先移动或删除下级区域`, otherwise confirm and remove the leaf.

- [ ] **Step 6: Run mutation tests and verify GREEN**

Run `npm test -- --run src/features/map/WorldMapPage.test.tsx`.

Expected: all map component tests pass.

- [ ] **Step 7: Commit**

```bash
git add prototype/web/src/features/map/WorldMapPage.tsx prototype/web/src/features/map/WorldMapPage.test.tsx
git commit -m "feat: add world region inspector editing"
```

### Task 5: Replace legacy map styling with responsive schematic styling

**Files:**
- Modify: `prototype/web/src/styles/global.css`
- Modify: `prototype/web/src/features/map/WorldMapPage.test.tsx`

- [ ] **Step 1: Add a failing structural assertion for mobile-safe controls**

Assert the mode switch, breadcrumb navigation, map/tree surface, and inspector have stable accessible labels and no image upload control exists.

- [ ] **Step 2: Run the test and verify RED**

Run `npm test -- --run src/features/map/WorldMapPage.test.tsx`.

Expected: FAIL until the final semantic labels and upload removal are present.

- [ ] **Step 3: Add schematic map styles**

Replace `.map-canvas`, `.map-marker`, and `.map-path` rules with focused classes for:

```css
.world-map-shell { display:grid; grid-template-columns:minmax(0,1fr) 340px; gap:14px; }
.world-map-surface { position:relative; min-height:620px; overflow:hidden; background:linear-gradient(145deg,#dbe9eb,#b9d2d7); }
.world-region-block { position:absolute; border:1px solid rgba(68,78,82,.3); border-radius:28% 42% 34% 38%; }
.world-region-block.selected { outline:3px solid rgba(181,138,61,.35); border-color:#b58a3d; }
.world-tree { display:grid; gap:4px; padding:18px; }
.world-map-breadcrumbs { display:flex; overflow-x:auto; white-space:nowrap; }
```

Use distinct modifier classes for continent, sea, state, settlement, stronghold, terrain, secret realm, and resource nodes.

- [ ] **Step 4: Add responsive rules**

At `max-width: 980px`, stack the inspector below the main surface. At `max-width: 600px`, reduce the map minimum height, allow the toolbar to wrap, keep breadcrumbs scrollable, and preserve 44px touch targets.

- [ ] **Step 5: Run component tests and typecheck**

Run:

```bash
npm test -- --run src/features/map/WorldMapPage.test.tsx
npm run typecheck
```

Expected: tests and typecheck pass.

- [ ] **Step 6: Commit**

```bash
git add prototype/web/src/styles/global.css prototype/web/src/features/map/WorldMapPage.test.tsx
git commit -m "style: refine hierarchical world map"
```

### Task 6: Verify persistence, export compatibility, and end-to-end flows

**Files:**
- Modify: `prototype/web/e2e/authoring-flow.spec.ts`
- Modify: `prototype/web/src/services/exportService.test.ts`
- Modify: `prototype/web/README.md`

- [ ] **Step 1: Write failing export and end-to-end tests**

Export test:

```ts
test('includes hierarchical world regions in project JSON', () => {
  const payload = JSON.parse(buildProjectJson(createDemoProject()))
  expect(payload.worldRegions.some((item: { name: string }) => item.name === '镜海世界')).toBe(true)
})
```

Playwright flow:

```ts
test('switches views, drills down, and persists a region edit', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: '世界地图' }).click()
  await page.getByRole('button', { name: '北陆' }).click()
  await page.getByRole('button', { name: '进入北陆' }).click()
  await expect(page.getByRole('button', { name: '雾港州' })).toBeVisible()
  await page.getByRole('button', { name: '层级结构' }).click()
  await page.getByRole('treeitem', { name: /雾港州/ }).click()
  await page.getByLabel('区域简介').fill('北陆东部的重要海港州。')
  await page.reload()
  await page.getByRole('button', { name: '世界地图' }).click()
  await page.getByRole('button', { name: '层级结构' }).click()
  await page.getByRole('treeitem', { name: /雾港州/ }).click()
  await expect(page.getByLabel('区域简介')).toHaveValue('北陆东部的重要海港州。')
})
```

- [ ] **Step 2: Run targeted tests and verify RED where behavior is missing**

Run `npm test -- --run src/services/exportService.test.ts` and `npx playwright test e2e/authoring-flow.spec.ts`.

- [ ] **Step 3: Complete JSON compatibility and README description**

Keep `buildProjectJson` serializing the full project object and document the new dual-view hierarchical map in the prototype capability list. Remove README wording that describes uploading a background map.

- [ ] **Step 4: Run the full verification suite**

Run:

```bash
npm run typecheck
npm test -- --run
npm run build
npx playwright test
```

Expected: typecheck exit 0, all Vitest files pass, production build succeeds, and all Playwright tests pass.

- [ ] **Step 5: Perform browser visual QA**

Open the prototype at desktop and 390px widths. Verify world overview, 北陆 drill-down, tree mode, inspector groups, toolbar wrapping, and console errors. Confirm there is no image upload control.

- [ ] **Step 6: Commit**

```bash
git add prototype/web/e2e/authoring-flow.spec.ts prototype/web/src/services/exportService.test.ts prototype/web/README.md
git commit -m "test: cover hierarchical world map flows"
```

