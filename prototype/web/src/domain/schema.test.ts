import { createDemoProject, createDemoStudioData } from './demoData'
import { parseProject, StudioDataSchema } from './schema'

describe('StudioDataSchema', () => {
  test('accepts the complete demo workspace', () => {
    const result = StudioDataSchema.safeParse(createDemoStudioData())
    expect(result.success).toBe(true)
  })

  test('rejects a project without a title', () => {
    const data = createDemoStudioData()
    const result = StudioDataSchema.safeParse({
      ...data,
      projects: [{ ...data.projects[0], title: '' }],
    })
    expect(result.success).toBe(false)
  })

  test('adds empty world map fields to legacy projects', () => {
    const project = structuredClone(createDemoProject()) as unknown as Record<string, unknown>
    delete project.worldRegions
    delete project.worldMapState

    const parsed = parseProject(project)

    expect(parsed.worldRegions).toEqual([])
    expect(parsed.worldMapState).toEqual({ mode: 'map', focusRegionId: null })
  })
})
