import { createDemoStudioData } from './demoData'
import { StudioDataSchema } from './schema'

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
})
