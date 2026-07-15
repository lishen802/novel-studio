# 织境品牌图标资产

“折页成境”以两张书页、深蓝世界入口、金色叙事主线和一颗灵感星构成。生成式母稿只用于确认纸张质感、层次与整体气质；正式发布资产以 `source/` 中的确定性 SVG 为准。

## 色板

- 境夜蓝：`#111827`
- 叙事金：`#C9983B`
- 宣纸白：`#FFFAF0`

## 母稿提示词

```text
Use case: logo-brand
Asset type: premium commercial desktop application icon concept for macOS and Windows
Primary request: Create the “织境 Story Studio” app icon concept named “折页成境”. Inside a restrained midnight-navy circular world portal, exactly two broad ivory paper-page ribbons unfold from the lower-left and form a clear valley/path leading toward the upper-right. A single warm-gold narrative line begins at the page junction and continues into the world. One small four-point inspiration star sits in the upper-right. The symbol must feel professional first and subtly East-Asian literary second.
Input images: the first image is a mood reference for pages opening into a fictional world; the second image is a palette and elegant line-weight reference only. Do not copy their detailed composition.
Scene/backdrop: centered icon on a warm ivory rounded-square app tile
Style/medium: premium vector-friendly brand mark with extremely subtle dimensional paper relief, clean geometry, crisp edges
Composition/framing: centered, generous safe area, strong silhouette, readable at 32px
Color palette: midnight navy #111827, narrative gold #C9983B, ivory #FFFAF0
Constraints: exactly two page shapes, one gold path, one small star; no text, no letters, no Chinese characters, no realistic landscape details, no pen nib
Avoid: starscape, moon, mountains, many particles, braided DNA shape, robot, brain, circuit board, neon gradients, excessive 3D, heavy shadow, watermark
```

## 概念稿

- `concepts/folded-pages-world-v1.png`：视觉母稿，不直接用于产品发布。

## 构建

在仓库根目录执行：

```bash
cd prototype/web
npm run build:icons
```

构建器会从 SVG 主源重新生成 PNG、macOS ICNS、Windows ICO、网页图标和尺寸验收图。输出是确定性的，不依赖概念稿，也不需要手工导出。

## 唯一几何主源

- `source/app-icon.svg`：128px 及以上的完整图标，保留金环与灵感星。
- `source/app-icon-small.svg`：64px 及以下的简化图标，移除小尺寸易糊细节并加粗主线。

修改品牌图标时只编辑这两份 SVG，然后重新运行构建命令；不要直接修改 `generated/` 或网页目录中的 PNG。

## 平台交付物

- macOS：`generated/macos/app.icns` 与 `generated/macos/app.iconset/`
- Windows：`generated/windows/app.ico`
- 通用 PNG：`generated/png/`
- 浅色/深色验收图：`generated/previews/`
- 网页原型：`prototype/web/public/brand/`

ICNS 与 ICO 都由 Node 构建器直接打包，便于在 macOS、Windows 和 CI 环境中复现。
