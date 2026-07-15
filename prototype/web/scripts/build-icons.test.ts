import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const root = path.resolve(process.cwd(), '../..')

describe('brand icon sources', () => {
  test.each(['app-icon.svg', 'app-icon-small.svg'])('%s uses the approved viewBox and palette', async (name) => {
    const svg = await readFile(path.join(root, 'assets/branding/source', name), 'utf8')
    expect(svg).toContain('viewBox="0 0 1024 1024"')
    expect(svg).toContain('#111827')
    expect(svg).toContain('#C9983B')
    expect(svg).toContain('#FFFAF0')
    expect(svg).not.toMatch(/<text|filter=|data:image/i)
  })
})
