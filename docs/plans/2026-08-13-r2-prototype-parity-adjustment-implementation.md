# R2 原型视觉与交互回归 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变 Wails binding、Go 应用服务或未实现 AI 业务的前提下，使已实现的首页、项目导航、正文工作区和 AI 面板的结构与原型一致。

**Architecture:** 保留 `StudioClient`、`WorkspaceStore` 和现有功能组件，前端仅重新编排容器、导航与状态呈现。原型中的模拟项目、候选和档案结果不进入生产 UI；状态只读取现有 `ApplicationBootstrapView`、`ProjectView` 和 `WorkbenchProjection`。

**Tech Stack:** React 19、TypeScript、Zustand、Vitest、Testing Library、现有 CSS。

---

- 状态：`completed`
- 所属阶段：[R2 产品壳、应用设置与交互基础](../roadmap/development-roadmap.md#7-r2产品壳应用设置与交互基础)
- 上游文档：[产品需求](../product/requirements.md)、[产品与交互设计](../product/design.md)、[系统技术设计](../architecture/system-design.md)
- 不包含：新模型调用、候选生成、整书档案生成、数据库迁移、Go binding 改动。

## 当前基线与验收口径

- 首页已有“新建、导入、打开、最近项目”，但最近项目卡缺少原型的封面、最近位置与明确继续路径的视觉层级。
- 项目导航已有六个一级能力，但档案和交付入口以侧栏折叠子项表达，偏离原型的一级直达、内容区二级标签结构。
- `Workspace` 已有章节编辑器、只读保护、风险提示和 `Cmd/Ctrl+J` AI 面板；本计划只将它们放回原型的三栏工作区布局。
- 不能显示原型的假候选、模型对话或档案结果。没有来自后端的字段时，展示真实的最近位置、最近打开时间、未生成或未就绪状态。

### Task 1: 首页最近作品卡回归

**Files:**
- Modify: `apps/desktop/frontend/src/features/home/HomeScreen.tsx`
- Modify: `apps/desktop/frontend/src/features/home/HomeScreen.test.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`

- [x] **Step 1: 写失败测试**

在 `HomeScreen.test.tsx` 断言：有最近项目时，侧栏项目卡有封面占位、最近打开时间和最近工作区；主卡显示“最近继续”、最近位置和“继续《标题》”；缺失路径仍禁用继续按钮。

- [x] **Step 2: 运行 RED**

Run: `npm test -- HomeScreen.test.tsx`

Expected: 新断言因封面占位与主卡状态标签尚不存在而失败。

- [x] **Step 3: 实现最小 UI 调整**

使用 `lastWorkspace` 和 `lastOpenedAt` 构造本地展示文案；为侧栏卡加入无数据依赖的封面色块；为主卡加入状态标签和“去审阅”样式的次级入口。入口仍调用 `onOpenRecent`，不新增网络或数据请求。

- [x] **Step 4: 运行 GREEN**

Run: `npm test -- HomeScreen.test.tsx`

Expected: 首页测试全部通过。

### Task 2: 六个一级入口与内容区二级标签回归

**Files:**
- Modify: `apps/desktop/frontend/src/features/workspace/ProjectNavigation.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/ProjectNavigation.test.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/WorkspaceSectionTabs.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/WorkspaceSectionTabs.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/WorkbenchRouter.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`

- [x] **Step 1: 写失败测试**

断言作品档案与版本与交付都是点击即跳转的一级入口；档案和交付的已实现子页面在内容区以标签导航出现，选择标签仍回调既有 `ProjectPage`。

- [x] **Step 2: 运行 RED**

Run: `npm test -- ProjectNavigation.test.tsx WorkspaceSectionTabs.test.tsx`

Expected: 现有测试仍将档案与交付视为折叠分组，新增二级标签组件尚不存在。

- [x] **Step 3: 实现最小导航调整**

将侧栏固定为原型的六个直接入口：驾驶舱、正文、审阅、导入、档案和交付。档案默认进入已实现的故事架构，交付默认进入版本历史；`WorkbenchRouter` 在对应内容页上方渲染二级标签，保留人物、世界、连续性、分支和未就绪页面的既有可达性。

- [x] **Step 4: 运行 GREEN**

Run: `npm test -- ProjectNavigation.test.tsx WorkspaceSectionTabs.test.tsx`

Expected: 六入口与二级标签测试全部通过。

### Task 3: 正文工作区与 AI 面板回归

**Files:**
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workbench/AIPanel.tsx`
- Modify: `apps/desktop/frontend/src/features/workbench/AIPanel.test.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`

- [x] **Step 1: 写失败测试**

断言正文工作区在已有章节时呈现项目摘要、章节列表、正文内容区和展开后的“AI 创作”三栏；`Cmd/Ctrl+J` 打开 AI 面板不丢失编辑器或面板草稿；未实现选区提交仍仅清除选区。

- [x] **Step 2: 运行 RED**

Run: `npm test -- Workspace.test.tsx AIPanel.test.tsx App.test.tsx`

Expected: 新的可访问区域和 AI 面板标题／操作控制尚不存在。

- [x] **Step 3: 实现最小工作区调整**

为工作区增加原型式顶部上下文栏与明确的内容区语义；保留当前章节、风险、只读、保存与恢复逻辑。为 AI 面板加入标题、关闭控制、说明输入和真实未就绪提示，不渲染任何模型结果或候选。

- [x] **Step 4: 运行 GREEN**

Run: `npm test -- Workspace.test.tsx AIPanel.test.tsx App.test.tsx`

Expected: 相关 UI 测试通过且已有草稿保留测试不回归。

### Task 4: 类型检查、全量验证与原型对照

**Files:**
- Modify: `docs/plans/2026-08-13-r2-prototype-parity-adjustment-implementation.md`（更新完成证据）

- [x] **Step 1: 运行静态验证**

Run: `npm run typecheck && npm test && npm run build`

Expected: TypeScript、31+ 个前端测试文件和 Vite 生产构建全部通过。

- [x] **Step 2: 浏览器视觉对照**

打开 `prototype` 的首页、正文、审阅、档案与版本页面，并在带 `FakeStudioClient` 的组件测试结果中确认生产组件保留对应布局：顶部产品区、六项侧栏、内容区标签、章节列表和 AI 侧栏。

- [x] **Step 3: 更新计划证据**

记录精确验证命令、结果与提交 SHA；状态改为 `completed`。不将 `.superpowers/`、`node_modules/`、本地数据库、真实作品或凭据加入 Git。

## 完成证据

- 2026-08-14：`npm test -- --pool=threads --maxWorkers=1 --fileParallelism=false` 通过，32 个测试文件、288 项测试全部通过。默认 fork worker 模式偶发启动超时；单线程 threads 模式消除了环境并发资源波动。
- 2026-08-14：`npm run typecheck` 与 `npm run build` 通过；Vite 生产构建成功。
- 2026-08-14：`git diff --check` 通过。
- 2026-08-14：浏览器验证了空首页、正文工作区与 `Cmd/Ctrl+J` 展开的 AI 三栏。首页、六项一级导航、档案/交付内容标签与 AI 侧栏均只读取真实本地状态；未实现 AI 生成、选区提交与整书档案继续显示未就绪状态。
