import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import sharp from 'sharp'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(scriptDir, '../../..')
const sourceDir = path.join(repoRoot, 'assets/branding/source')
const generatedDir = path.join(repoRoot, 'assets/branding/generated')

export const sourceForSize = (size) => size <= 64 ? 'app-icon-small.svg' : 'app-icon.svg'

export function encodeIco(images) {
  const header = Buffer.alloc(6 + images.length * 16)
  header.writeUInt16LE(0, 0)
  header.writeUInt16LE(1, 2)
  header.writeUInt16LE(images.length, 4)
  let offset = header.length
  images.forEach(({ size, png }, index) => {
    const entry = 6 + index * 16
    header[entry] = size === 256 ? 0 : size
    header[entry + 1] = size === 256 ? 0 : size
    header.writeUInt16LE(1, entry + 4)
    header.writeUInt16LE(32, entry + 6)
    header.writeUInt32LE(png.length, entry + 8)
    header.writeUInt32LE(offset, entry + 12)
    offset += png.length
  })
  return Buffer.concat([header, ...images.map(({ png }) => png)])
}

export function encodeIcns(images) {
  const chunkTypes = new Map([[16, 'icp4'], [32, 'icp5'], [64, 'icp6'], [128, 'ic07'], [256, 'ic08'], [512, 'ic09'], [1024, 'ic10']])
  const chunks = images.map(({ size, png }) => {
    const type = chunkTypes.get(size)
    if (!type) throw new Error(`Unsupported ICNS size: ${size}`)
    const header = Buffer.alloc(8)
    header.write(type, 0, 4, 'ascii')
    header.writeUInt32BE(png.length + 8, 4)
    return Buffer.concat([header, png])
  })
  const header = Buffer.alloc(8)
  header.write('icns', 0, 4, 'ascii')
  header.writeUInt32BE(8 + chunks.reduce((total, chunk) => total + chunk.length, 0), 4)
  return Buffer.concat([header, ...chunks])
}

export async function renderIcon(size) {
  const source = await readFile(path.join(sourceDir, sourceForSize(size)))
  return sharp(source).resize(size, size).png().toBuffer()
}

export async function composePreview(rendered, background, foreground) {
  const sizes = [1024, 512, 256, 128, 64, 32, 24, 16]
  const composites = []
  for (const [index, size] of sizes.entries()) {
    const column = index % 4
    const row = Math.floor(index / 4)
    const displaySize = Math.min(size, 220)
    const centerX = 200 + column * 400
    const centerY = 190 + row * 420
    const icon = await sharp(rendered.get(size)).resize(displaySize, displaySize).png().toBuffer()
    composites.push({ input: icon, left: Math.round(centerX - displaySize / 2), top: Math.round(centerY - displaySize / 2) })
    const label = Buffer.from(`<svg width="400" height="48"><text x="200" y="32" text-anchor="middle" font-family="Arial,sans-serif" font-size="22" fill="${foreground}">${size}px</text></svg>`)
    composites.push({ input: label, left: column * 400, top: centerY + 128 })
  }
  return sharp({ create: { width: 1600, height: 900, channels: 4, background } })
    .composite(composites)
    .png()
    .toBuffer()
}

async function writePreview(rendered, background, foreground, fileName) {
  const previewDir = path.join(generatedDir, 'previews')
  await mkdir(previewDir, { recursive: true })
  await writeFile(path.join(previewDir, fileName), await composePreview(rendered, background, foreground))
}

export async function buildIcons() {
  const sizes = [16, 24, 32, 48, 64, 128, 180, 256, 512, 1024]
  const pngDir = path.join(generatedDir, 'png')
  const webDir = path.join(repoRoot, 'prototype/web/public/brand')
  await Promise.all([mkdir(pngDir, { recursive: true }), mkdir(webDir, { recursive: true })])
  const rendered = new Map()
  for (const size of sizes) {
    const png = await renderIcon(size)
    rendered.set(size, png)
    await writeFile(path.join(pngDir, `app-icon-${size}.png`), png)
  }
  for (const size of [32, 64, 180, 512]) {
    await copyFile(path.join(pngDir, `app-icon-${size}.png`), path.join(webDir, `app-icon-${size}.png`))
  }
  const icoImages = [16, 24, 32, 48, 64, 128, 256].map((size) => ({ size, png: rendered.get(size) }))
  const windowsDir = path.join(generatedDir, 'windows')
  await mkdir(windowsDir, { recursive: true })
  await writeFile(path.join(windowsDir, 'app.ico'), encodeIco(icoImages))

  if (process.platform === 'darwin') {
    const macosDir = path.join(generatedDir, 'macos')
    const iconset = path.join(macosDir, 'app.iconset')
    await mkdir(iconset, { recursive: true })
    const pairs = [
      [16, 'icon_16x16.png'], [32, 'icon_16x16@2x.png'],
      [32, 'icon_32x32.png'], [64, 'icon_32x32@2x.png'],
      [128, 'icon_128x128.png'], [256, 'icon_128x128@2x.png'],
      [256, 'icon_256x256.png'], [512, 'icon_256x256@2x.png'],
      [512, 'icon_512x512.png'], [1024, 'icon_512x512@2x.png'],
    ]
    for (const [size, name] of pairs) await writeFile(path.join(iconset, name), rendered.get(size))
    const icnsImages = [16, 32, 64, 128, 256, 512, 1024].map((size) => ({ size, png: rendered.get(size) }))
    await writeFile(path.join(macosDir, 'app.icns'), encodeIcns(icnsImages))
  }
  await writePreview(rendered, '#FFFAF0', '#111827', 'icon-sizes-light.png')
  await writePreview(rendered, '#111827', '#FFFAF0', 'icon-sizes-dark.png')
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await buildIcons()
