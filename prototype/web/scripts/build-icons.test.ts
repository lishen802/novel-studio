import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const root = path.resolve(process.cwd(), '../..')
const loadBuilder = () => {
  const modulePath = './build-icons.mjs'
  return import(/* @vite-ignore */ modulePath).catch(() => ({}))
}

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

test('uses the simplified source at 64px and below', async () => {
  const module = await loadBuilder()
  expect('sourceForSize' in module ? module.sourceForSize(64) : undefined).toBe('app-icon-small.svg')
  expect('sourceForSize' in module ? module.sourceForSize(128) : undefined).toBe('app-icon.svg')
})

test('encodes multiple PNG images in a Windows ICO container', async () => {
  const module = await loadBuilder()
  const ico = 'encodeIco' in module ? module.encodeIco([
    { size: 32, png: Buffer.from([1, 2, 3]) },
    { size: 256, png: Buffer.from([4, 5]) },
  ]) : Buffer.alloc(0)
  expect(ico.readUInt16LE(0)).toBe(0)
  expect(ico.readUInt16LE(2)).toBe(1)
  expect(ico.readUInt16LE(4)).toBe(2)
  expect(ico[6]).toBe(32)
  expect(ico[22]).toBe(0)
})

test('renders the 1024px source without exceeding image safety limits', async () => {
  const module = await loadBuilder()
  const png = 'renderIcon' in module ? await module.renderIcon(1024) : Buffer.alloc(0)
  expect(png.length).toBeGreaterThan(10_000)
})

test('encodes PNG images in a modern macOS ICNS container', async () => {
  const module = await loadBuilder()
  const icns = 'encodeIcns' in module ? module.encodeIcns([
    { size: 16, png: Buffer.from([1, 2, 3]) },
    { size: 512, png: Buffer.from([4, 5]) },
  ]) : Buffer.alloc(0)
  expect(icns.subarray(0, 4).toString('ascii')).toBe('icns')
  expect(icns.readUInt32BE(4)).toBe(icns.length)
  expect(icns.subarray(8, 12).toString('ascii')).toBe('icp4')
  expect(icns.subarray(19, 23).toString('ascii')).toBe('ic09')
})
