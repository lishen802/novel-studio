import { createDemoProject } from '../domain/demoData'
import { buildMarkdownExport, buildProjectJson, parseProjectJson } from './exportService'

test('exports chapters in order as markdown', () => {
  const project = createDemoProject()
  const output = buildMarkdownExport(project)
  expect(output.indexOf('# 第36章')).toBeLessThan(output.indexOf('# 第37章'))
  expect(output).toContain(project.chapters[0].content)
})

test('round-trips a project through validated JSON', () => {
  const original = createDemoProject()
  const parsed = parseProjectJson(buildProjectJson(original))
  expect(parsed.success).toBe(true)
  if (parsed.success) expect(parsed.data.chapters).toHaveLength(original.chapters.length)
})
