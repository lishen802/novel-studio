# 织境应用图标 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 生成并交付“折页成境”商业应用图标，建立可重复构建的多尺寸资产流水线，并把最终图标接入现有网页原型。

**Architecture:** 生成式图像只用于确认材质、光影与整体气质；正式发布资产以两份确定性 SVG（完整版、64px 以下简化版）为唯一几何源。现有 React/Vite 工具链增加一个隔离的 Node 资产构建脚本，使用 Sharp 渲染 PNG、用确定性 ICO 编码器打包 Windows 图标，并在 macOS 上调用 `iconutil` 生成 ICNS；原型只消费构建产物，不直接引用概念稿。

**Tech Stack:** OpenAI built-in image generation, SVG, Node.js 20+, Sharp, Vitest, React 19, Vite, Playwright, macOS `iconutil`.

---

## 文件结构

- `assets/branding/source/app-icon.svg`：完整几何源，适用于 128px 及以上。
- `assets/branding/source/app-icon-small.svg`：简化几何源，适用于 16–64px。
- `assets/branding/concepts/folded-pages-world-v1.png`：经确认的生成式视觉母稿，仅作材质参考。
- `assets/branding/generated/png/`：各标准尺寸 PNG。
- `assets/branding/generated/macos/app.iconset/`：macOS iconset 输入。
- `assets/branding/generated/macos/app.icns`：macOS 应用图标。
- `assets/branding/generated/windows/app.ico`：Windows 多图层图标。
- `assets/branding/README.md`：色值、提示词、构建方式和资产用途。
- `prototype/web/scripts/build-icons.mjs`：确定性资产构建器。
- `prototype/web/scripts/build-icons.test.ts`：尺寸选择、SVG 校验与 ICO 容器测试。
- `prototype/web/public/brand/`：网页原型消费的 32、64、180、512px 产物。
- `prototype/web/src/app/App.tsx`：把文字占位品牌标替换为图标。
- `prototype/web/src/styles/global.css`：图标在全局导航和侧栏中的显示规则。
- `prototype/web/src/app/App.test.tsx`：品牌图标可访问性与渲染回归测试。
- `prototype/web/index.html`：真实 favicon 和 Apple touch icon。

### Task 1: 生成并确认视觉母稿

**Files:**
- Create: `assets/branding/concepts/folded-pages-world-v1.png`
- Create: `assets/branding/README.md`

- [ ] **Step 1: 使用已批准规格生成一张母稿**

使用 built-in `image_gen`，把两张用户初稿作为风格参考而非编辑目标。使用以下提示词：

```text
Use case: logo-brand
Asset type: premium commercial desktop application icon concept for macOS and Windows
Primary request: Create the “织境 Story Studio” app icon concept named “折页成境”. Inside a restrained midnight-navy circular world portal, two broad ivory paper-page ribbons unfold from the lower-left and form a clear valley/path leading toward the upper-right. A single warm-gold narrative line begins at the page junction and continues into the world. One small four-point inspiration star sits in the upper-right. The symbol must feel professional first and subtly East-Asian literary second.
Input images: Image 1 is a mood reference for “pages opening into a fictional world”; Image 2 is a reference only for the existing navy, warm-gold, ivory palette. Do not copy their detailed composition.
Scene/backdrop: centered icon on a warm ivory rounded-square app tile
Style/medium: premium vector-friendly brand mark with extremely subtle dimensional paper relief; clean geometry; crisp edges
Composition/framing: centered, generous safe area, strong silhouette, readable at 32px
Color palette: midnight navy #111827, narrative gold #C9983B, ivory #FFFAF0
Constraints: exactly two page shapes, one gold path, one small star; no text; no letters; no Chinese characters; no realistic landscape details; no pen nib
Avoid: starscape, moon, mountains, many particles, braided DNA shape, robot, brain, circuit board, neon gradients, excessive 3D, heavy shadow, watermark
```

- [ ] **Step 2: 检查母稿**

用 `view_image` 检查：主体是否正中、书页是否只有两张、32px 轮廓是否仍可辨、是否严格使用三色、是否混入山川/月亮/笔尖/文字。若只差一个明确问题，使用一次针对性编辑；不得在同一轮同时改变构图和色彩。

- [ ] **Step 3: 保存母稿与提示词**

把最终选中图像复制到 `assets/branding/concepts/folded-pages-world-v1.png`。在 `assets/branding/README.md` 写入：最终提示词、三色色值、母稿仅作材质参考、正式发布资产来源于 SVG。

- [ ] **Step 4: 提交母稿**

```bash
git add assets/branding/concepts/folded-pages-world-v1.png assets/branding/README.md
git commit -m "art: add folded-pages icon concept"
```

### Task 2: 建立确定性的 SVG 主源

**Files:**
- Create: `assets/branding/source/app-icon.svg`
- Create: `assets/branding/source/app-icon-small.svg`
- Create: `prototype/web/scripts/build-icons.test.ts`

- [ ] **Step 1: 写 SVG 约束测试**

创建 `prototype/web/scripts/build-icons.test.ts`：

```ts
import { readFile } from 'node:fs/promises'
import { describe, expect, test } from 'vitest'

const root = new URL('../../../', import.meta.url)

describe('brand icon sources', () => {
  test.each(['app-icon.svg', 'app-icon-small.svg'])('%s uses the approved viewBox and palette', async (name) => {
    const svg = await readFile(new URL(`assets/branding/source/${name}`, root), 'utf8')
    expect(svg).toContain('viewBox="0 0 1024 1024"')
    expect(svg).toContain('#111827')
    expect(svg).toContain('#C9983B')
    expect(svg).toContain('#FFFAF0')
    expect(svg).not.toMatch(/<text|filter=|data:image/i)
  })
})
```

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
cd prototype/web
npm test -- --run scripts/build-icons.test.ts
```

Expected: FAIL with `ENOENT` for `assets/branding/source/app-icon.svg`.

- [ ] **Step 3: 创建完整 SVG**

创建 `assets/branding/source/app-icon.svg`，只使用确定性矢量路径：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
  <rect width="1024" height="1024" rx="224" fill="#FFFAF0"/>
  <circle cx="512" cy="492" r="338" fill="#111827"/>
  <circle cx="512" cy="492" r="296" fill="none" stroke="#C9983B" stroke-width="18"/>
  <path d="M232 594C364 396 496 500 522 512C650 574 700 472 790 330" fill="none" stroke="#FFFAF0" stroke-width="94" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M232 594C374 518 470 610 522 674C640 816 716 724 790 702" fill="none" stroke="#FFFAF0" stroke-width="94" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M522 512C500 604 502 696 526 798" fill="none" stroke="#C9983B" stroke-width="24" stroke-linecap="round"/>
  <path d="M714 244v96M666 292h96" stroke="#FFFAF0" stroke-width="13" stroke-linecap="round"/>
  <circle cx="714" cy="292" r="15" fill="#C9983B"/>
</svg>
```

- [ ] **Step 4: 创建小尺寸 SVG**

创建 `assets/branding/source/app-icon-small.svg`；移除金环和星形并加粗关键路径：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
  <rect width="1024" height="1024" rx="224" fill="#FFFAF0"/>
  <circle cx="512" cy="492" r="350" fill="#111827"/>
  <path d="M220 600C360 390 494 500 522 516C654 582 704 468 804 316" fill="none" stroke="#FFFAF0" stroke-width="112" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M220 600C374 514 470 612 522 680C644 828 724 730 804 704" fill="none" stroke="#FFFAF0" stroke-width="112" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M522 516C500 612 502 704 528 808" fill="none" stroke="#C9983B" stroke-width="36" stroke-linecap="round"/>
</svg>
```

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `npm test -- --run scripts/build-icons.test.ts`

Expected: 2 tests pass.

- [ ] **Step 6: 提交 SVG 主源**

```bash
git add assets/branding/source prototype/web/scripts/build-icons.test.ts
git commit -m "feat: add canonical app icon sources"
```

### Task 3: 实现跨平台图标资产构建器

**Files:**
- Modify: `prototype/web/package.json`
- Modify: `prototype/web/package-lock.json`
- Create: `prototype/web/scripts/build-icons.mjs`
- Modify: `prototype/web/scripts/build-icons.test.ts`

- [ ] **Step 1: 安装 Sharp 并增加构建命令**

Run:

```bash
cd prototype/web
npm install --save-dev sharp
```

在 `package.json` 的 `scripts` 中增加：

```json
"build:icons": "node scripts/build-icons.mjs"
```

- [ ] **Step 2: 先写 ICO 和尺寸选择测试**

把以下测试追加到 `scripts/build-icons.test.ts`：

```ts
import { encodeIco, sourceForSize } from './build-icons.mjs'

test('uses the simplified source at 64px and below', () => {
  expect(sourceForSize(64)).toBe('app-icon-small.svg')
  expect(sourceForSize(128)).toBe('app-icon.svg')
})

test('encodes multiple PNG images in a Windows ICO container', () => {
  const ico = encodeIco([
    { size: 32, png: Buffer.from([1, 2, 3]) },
    { size: 256, png: Buffer.from([4, 5]) },
  ])
  expect(ico.readUInt16LE(0)).toBe(0)
  expect(ico.readUInt16LE(2)).toBe(1)
  expect(ico.readUInt16LE(4)).toBe(2)
  expect(ico[6]).toBe(32)
  expect(ico[22]).toBe(0)
})
```

- [ ] **Step 3: 运行测试确认 RED**

Run: `npm test -- --run scripts/build-icons.test.ts`

Expected: FAIL because `build-icons.mjs` does not exist.

- [ ] **Step 4: 实现构建器**

创建 `prototype/web/scripts/build-icons.mjs`：

```js
import { execFileSync } from 'node:child_process'
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

async function render(size) {
  const source = await readFile(path.join(sourceDir, sourceForSize(size)))
  return sharp(source, { density: 1200 }).resize(size, size).png().toBuffer()
}

export async function buildIcons() {
  const sizes = [16, 24, 32, 48, 64, 128, 180, 256, 512, 1024]
  const pngDir = path.join(generatedDir, 'png')
  const webDir = path.join(repoRoot, 'prototype/web/public/brand')
  await Promise.all([mkdir(pngDir, { recursive: true }), mkdir(webDir, { recursive: true })])
  const rendered = new Map()
  for (const size of sizes) {
    const png = await render(size)
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
    const iconset = path.join(generatedDir, 'macos/app.iconset')
    await mkdir(iconset, { recursive: true })
    const pairs = [[16, 'icon_16x16.png'], [32, 'icon_16x16@2x.png'], [32, 'icon_32x32.png'], [64, 'icon_32x32@2x.png'], [128, 'icon_128x128.png'], [256, 'icon_128x128@2x.png'], [256, 'icon_256x256.png'], [512, 'icon_256x256@2x.png'], [512, 'icon_512x512.png'], [1024, 'icon_512x512@2x.png']]
    for (const [size, name] of pairs) await writeFile(path.join(iconset, name), rendered.get(size))
    execFileSync('iconutil', ['-c', 'icns', iconset, '-o', path.join(generatedDir, 'macos/app.icns')])
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await buildIcons()
```

- [ ] **Step 5: 运行单元测试和资产构建**

Run:

```bash
npm test -- --run scripts/build-icons.test.ts
npm run build:icons
```

Expected: tests pass; PNG、ICO、ICNS 和 `public/brand` 文件生成。

- [ ] **Step 6: 验证生成文件结构**

Run:

```bash
file ../../assets/branding/generated/windows/app.ico
file ../../assets/branding/generated/macos/app.icns
sips -g pixelWidth -g pixelHeight public/brand/app-icon-512.png
```

Expected: Windows icon, macOS icon, and a 512×512 PNG are reported.

- [ ] **Step 7: 提交构建器与资产**

```bash
git add prototype/web/package.json prototype/web/package-lock.json prototype/web/scripts prototype/web/public/brand assets/branding/generated
git commit -m "build: generate cross-platform app icons"
```

### Task 4: 把正式图标接入网页原型

**Files:**
- Modify: `prototype/web/src/app/App.test.tsx`
- Modify: `prototype/web/src/app/App.tsx`
- Modify: `prototype/web/src/styles/global.css`
- Modify: `prototype/web/index.html`

- [ ] **Step 1: 写失败的品牌渲染测试**

在 `App.test.tsx` 增加：

```tsx
test('renders the approved brand icon', () => {
  render(<App />)
  expect(screen.getByAltText('织境')).toHaveAttribute('src', '/brand/app-icon-64.png')
})
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `npm test -- --run src/app/App.test.tsx`

Expected: FAIL because no image with alt text `织境` exists.

- [ ] **Step 3: 替换全局品牌占位符**

在 `App.tsx` 中把：

```tsx
<div className="brand-mark">织</div>
```

替换为：

```tsx
<div className="brand-mark"><img src="/brand/app-icon-64.png" alt="织境" /></div>
```

在 `global.css` 中调整：

```css
.brand-mark { width: 40px; height: 40px; border: 0; overflow: hidden; border-radius: 12px; margin-bottom: 28px; }
.brand-mark img { display: block; width: 100%; height: 100%; object-fit: cover; }
```

- [ ] **Step 4: 配置浏览器图标**

在 `index.html` 中用以下内容替换 `data:,` favicon：

```html
<link rel="icon" type="image/png" sizes="32x32" href="/brand/app-icon-32.png" />
<link rel="apple-touch-icon" sizes="180x180" href="/brand/app-icon-180.png" />
```

- [ ] **Step 5: 运行测试与构建**

Run:

```bash
npm test -- --run src/app/App.test.tsx
npm run typecheck
npm run build
```

Expected: App test passes; typecheck and production build exit 0.

- [ ] **Step 6: 提交原型接入**

```bash
git add prototype/web/src/app/App.tsx prototype/web/src/app/App.test.tsx prototype/web/src/styles/global.css prototype/web/index.html
git commit -m "feat: apply zhi jing brand icon"
```

### Task 5: 完整视觉验收与交付说明

**Files:**
- Modify: `prototype/web/scripts/build-icons.mjs`
- Modify: `assets/branding/README.md`
- Modify: `prototype/web/README.md`

- [ ] **Step 1: 生成尺寸总览图**

把以下函数加入 `build-icons.mjs`，使用 Sharp 将 1024、512、256、128、64、32、24、16px 产物放在深色和浅色两张 1600×900 画布上：

```js
async function writePreview(rendered, background, foreground, fileName) {
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
  const previewDir = path.join(generatedDir, 'previews')
  await mkdir(previewDir, { recursive: true })
  await sharp({ create: { width: 1600, height: 900, channels: 4, background } })
    .composite(composites)
    .png()
    .toFile(path.join(previewDir, fileName))
}
```

在 `buildIcons()` 末尾、macOS 条件分支之后调用：

```js
await writePreview(rendered, '#FFFAF0', '#111827', 'icon-sizes-light.png')
await writePreview(rendered, '#111827', '#FFFAF0', 'icon-sizes-dark.png')
```

输出：

```text
assets/branding/generated/previews/icon-sizes-light.png
assets/branding/generated/previews/icon-sizes-dark.png
```

总览图必须标明尺寸，但标注只能存在于预览图，不能进入正式图标。

- [ ] **Step 2: 浏览器视觉 QA**

启动原型并用 Chrome 检查 1440×900 与 390×844：全局左栏图标不模糊、不被裁切，导航折叠时仍可辨，favicon 请求无 404，控制台无应用错误。

- [ ] **Step 3: 平台资产 QA**

使用 `view_image` 查看 512、64、32、16px PNG 和两张总览图；确认 32px 仍能读出双书页，16px 不粘连为实心圆。使用 Quick Look 或 Finder 检查 `app.icns`，使用 `file` 检查 `app.ico`。

- [ ] **Step 4: 补充交付说明**

在 `assets/branding/README.md` 记录：

```markdown
## Build

Run `cd prototype/web && npm run build:icons`.

## Canonical sources

- `source/app-icon.svg`: 128px and above
- `source/app-icon-small.svg`: 64px and below

## Platform outputs

- macOS: `generated/macos/app.icns`
- Windows: `generated/windows/app.ico`
- Web: `prototype/web/public/brand/`
```

在 `prototype/web/README.md` 的运行说明中增加 `npm run build:icons`。

- [ ] **Step 5: 执行完整验证**

Run:

```bash
cd prototype/web
npm run build:icons
npm run typecheck
npm test -- --run
npm run build
npm run test:e2e
```

Expected: icon build succeeds; all Vitest and Playwright tests pass; TypeScript and Vite builds exit 0.

- [ ] **Step 6: 提交最终交付**

```bash
git add assets/branding prototype/web/README.md prototype/web/public/brand
git commit -m "docs: complete app icon asset delivery"
```
