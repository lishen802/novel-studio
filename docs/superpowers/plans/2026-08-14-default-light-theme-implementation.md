# 默认浅色主题 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将全部已实现的桌面应用界面收敛到原型的默认浅色文学工作台，并固定浅色外观。

**Architecture:** 在现有单一 CSS 文件顶部定义原型色彩令牌，再按应用壳、作品工作台、设置/弹窗和语义提示四个边界替换深色专用规则。`theme` 的 API 与历史数据保持兼容，但前端不再提供会影响外观的主题选择。

**Tech Stack:** React 19、TypeScript、Vitest、Testing Library、Vite、现有 Wails StudioClient。

---

## 文件职责

- `apps/desktop/frontend/src/app/styles.css`：全局令牌及首页、工作台、对话框、表单、提示的浅色表现。
- `apps/desktop/frontend/src/app/App.tsx`：为正常页、加载页和错误页提供统一的浅色应用壳。
- `apps/desktop/frontend/src/app/App.test.tsx`：验证全窗口浅色壳和应用级状态不回归。
- `apps/desktop/frontend/src/features/settings/ApplicationSettings.tsx`：将主题选择改为固定浅色说明，保留其余设置能力。
- `apps/desktop/frontend/src/features/settings/ApplicationSettings.test.tsx`：验证设置不再暴露深色/系统切换，同时兼容旧主题值。
- `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`、`apps/desktop/frontend/src/features/workbench/AIPanel.test.tsx`：覆盖工作台、AI、只读和风险提示的浅色语义容器。

### Task 1: 固定浅色应用壳和设置行为

**Files:**

- Modify: `apps/desktop/frontend/src/app/App.tsx`
- Modify: `apps/desktop/frontend/src/app/App.test.tsx`
- Modify: `apps/desktop/frontend/src/features/settings/ApplicationSettings.tsx`
- Modify: `apps/desktop/frontend/src/features/settings/ApplicationSettings.test.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`

- [ ] **Step 1: 写失败测试**

在 `App.test.tsx` 添加“正常首页和加载页均使用 `light-app-shell`，且没有 `app-shell__glow`”断言；在 `ApplicationSettings.test.tsx` 添加“旧 bootstrap 值为 `dark` 时仅显示‘浅色文学工作台’说明，查询不到‘深色’和‘跟随系统’按钮”的用例。

```tsx
expect(document.querySelector("main.light-app-shell")).toBeInTheDocument();
expect(screen.queryByRole("button", { name: "深色" })).not.toBeInTheDocument();
expect(screen.getByText("当前应用固定使用浅色文学工作台。")).toBeVisible();
```

- [ ] **Step 2: 运行 RED**

Run: `npm test -- App.test.tsx ApplicationSettings.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 新断言失败，现有加载页仍渲染深色 `app-shell`，设置页仍可点击三种主题。

- [ ] **Step 3: 实现最小应用壳与设置改动**

在 `App.tsx` 将首页、加载和 bootstrap 错误都收敛到 `light-app-shell`，只保留系统原生标题栏；删除这些路径上的 glow 元素。在 `ApplicationSettings.tsx` 移除 `submitTheme` 调用入口及主题按钮，改为无交互说明；不改 `ApplicationPreferencesInput`、StudioClient 或 Go 枚举。为 `styles.css` 的 `:root` 添加 `color-scheme: light` 和 `--color-*` 原型令牌，并令壳、welcome card、通用按钮和表单使用浅色令牌。

```css
:root {
  color-scheme: light;
  --color-bg: #f4f6f8;
  --color-surface: #ffffff;
  --color-border: #e8eaed;
  --color-text: #1f2937;
  --color-primary: #1a1f2e;
}
.light-app-shell { min-height: 100vh; background: var(--color-bg); }
```

- [ ] **Step 4: 运行 GREEN**

Run: `npm test -- App.test.tsx ApplicationSettings.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 两个测试文件通过，旧 `dark` 值不会导致设置渲染失败。

- [ ] **Step 5: 提交应用壳切片**

```bash
git add apps/desktop/frontend/src/app/App.tsx apps/desktop/frontend/src/app/App.test.tsx apps/desktop/frontend/src/app/styles.css apps/desktop/frontend/src/features/settings/ApplicationSettings.tsx apps/desktop/frontend/src/features/settings/ApplicationSettings.test.tsx
git commit -m "feat: fix default light appearance"
```

### Task 2: 将作品工作台迁移为原型浅色侧栏与面板

**Files:**

- Modify: `apps/desktop/frontend/src/app/styles.css`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workbench/AIPanel.test.tsx`

- [ ] **Step 1: 写失败测试**

为项目工作台根元素、左侧导航、AI 面板、只读提示和文件系统风险提示增加稳定的浅色语义类断言。保留现有 `Cmd/Ctrl+J`、只读禁用和风险操作测试。

```tsx
expect(screen.getByLabelText("项目工作台")).toHaveClass("workspace-shell--light");
expect(screen.getByText(/只读模式/).closest("p")).toHaveClass("notice--warning");
expect(screen.getByText(/文件系统风险/).closest("section")).toHaveClass("notice--info");
```

- [ ] **Step 2: 运行 RED**

Run: `npm test -- Workspace.test.tsx AIPanel.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 测试失败，因为当前工作台仍只有深色侧栏专用样式和非统一提示类。

- [ ] **Step 3: 实现最小工作台样式迁移**

在 `Workspace.tsx` 与相关提示组件添加不承载业务含义的稳定样式类；在 `styles.css` 将 `.chapter-sidebar`、`.workspace-brand`、`.workspace-sections`、`.chapter-list`、`.workspace-main`、`.ai-panel` 和二级标签改为白色/浅灰表面。活动导航使用 `--color-primary-subtle` 与主色文字；AI、只读、风险提示分别使用 `.notice--warning`、`.notice--info`、`.notice--danger` 的浅色边框/背景。不得更改只读 fieldset、路由、三栏宽度或 AI 草稿状态。

```css
.workspace-shell--light { color: var(--color-text); background: var(--color-bg); }
.chapter-sidebar, .ai-panel { background: var(--color-surface); border-color: var(--color-border); }
.notice--warning { color: #b45309; background: #fff7ed; border-color: #fed7aa; }
.notice--info { color: #1d4ed8; background: #eff6ff; border-color: #bfdbfe; }
```

- [ ] **Step 4: 运行 GREEN**

Run: `npm test -- Workspace.test.tsx AIPanel.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 两个测试文件通过，键盘呼出、关闭草稿保留和只读写入保护仍可用。

- [ ] **Step 5: 提交工作台切片**

```bash
git add apps/desktop/frontend/src/app/styles.css apps/desktop/frontend/src/features/workspace/Workspace.tsx apps/desktop/frontend/src/features/workspace/Workspace.test.tsx apps/desktop/frontend/src/features/workbench/AIPanel.test.tsx
git commit -m "feat: apply light workspace theme"
```

### Task 3: 统一弹窗、表单和真实状态提示

**Files:**

- Modify: `apps/desktop/frontend/src/app/styles.css`
- Modify: `apps/desktop/frontend/src/app/App.test.tsx`
- Modify: `apps/desktop/frontend/src/features/import/ImportWorkbench.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`

- [ ] **Step 1: 写失败测试**

分别触发应用错误、项目打开错误和导入错误，断言对话框/提示使用浅色面板类，并验证错误文案与关闭、重试、取消行为不变。

```tsx
expect(await screen.findByRole("alertdialog", { name: "项目操作错误" })).toHaveClass("project-dialog--light");
expect(screen.getByRole("alert")).toHaveClass("notice--danger");
```

- [ ] **Step 2: 运行 RED**

Run: `npm test -- App.test.tsx ImportWorkbench.test.tsx Workspace.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 断言失败，因为现有 `project-dialog`、输入框和部分警告面板仍使用深色背景规则。

- [ ] **Step 3: 实现最小提示和表单样式迁移**

在 `styles.css` 将 `.application-settings`、`.project-dialog`、`.dialog-backdrop`、通用输入/textarea/select、导入/恢复/未就绪提示和成功/错误标签迁移到白色表面与浅色语义容器。每个 DOM 状态继续沿用现有真实错误码、只读或未就绪文案；只添加 `notice--*` 或 `project-dialog--light` 等表示样式的类。

```css
.project-dialog--light, .application-settings { background: var(--color-surface); color: var(--color-text); }
.notice--danger { color: #b91c1c; background: #fef2f2; border-color: #fecaca; }
.dialog-backdrop { background: rgba(0, 0, 0, 0.55); }
```

- [ ] **Step 4: 运行 GREEN**

Run: `npm test -- App.test.tsx ImportWorkbench.test.tsx Workspace.test.tsx --pool=threads --maxWorkers=1 --fileParallelism=false`

Expected: 错误、取消、重试、导入和恢复测试均通过，无假成功状态。

- [ ] **Step 5: 提交提示与弹窗切片**

```bash
git add apps/desktop/frontend/src/app/styles.css apps/desktop/frontend/src/app/App.test.tsx apps/desktop/frontend/src/features/import/ImportWorkbench.test.tsx apps/desktop/frontend/src/features/workspace/Workspace.test.tsx
git commit -m "feat: unify light status surfaces"
```

### Task 4: 完整验证和计划证据

**Files:**

- Modify: `docs/superpowers/plans/2026-08-14-default-light-theme-implementation.md`

- [ ] **Step 1: 运行全量前端验证**

Run: `npm test -- --pool=threads --maxWorkers=1 --fileParallelism=false && npm run typecheck && npm run build && git diff --check`

Expected: 全部 Vitest 文件、类型检查、Vite 生产构建和差异检查成功。

- [ ] **Step 2: 做视觉验收**

启动现有桌面开发入口，在首页、打开项目后的正文/档案/版本页、应用设置和一类错误弹窗中检查：系统标题栏保持原生，画布/卡片/侧栏全部为原型浅色层级，状态提示使用对应浅色语义色，键盘焦点清晰可见。

- [ ] **Step 3: 更新证据并提交**

将实际命令与结果追加到本计划的“完成证据”章节，然后运行：

```bash
git add docs/superpowers/plans/2026-08-14-default-light-theme-implementation.md
git commit -m "docs: record light theme verification"
```
