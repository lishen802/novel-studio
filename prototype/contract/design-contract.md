# Design Contract — 织境 Story Studio

## Style Tier & Aesthetic Direction
- style: brand-themed (literary-light default, literary-dark optional)
- aesthetic: refined literary workstation / quiet premium desktop writing studio
- tone keywords: calm, restrained, literary, authoritative, warm

## Design Tokens

### Color
- color.bg:             #f4f6f8
- color.surface:        #ffffff
- color.surface-hover:  #f8f9fa
- color.surface-elevated: #ffffff
- color.surface-popup:  #ffffff
- color.border:         #e8eaed
- color.border-strong:  #d1d5db
- color.border-subtle:  #f0f1f3
- color.text:           #1f2937
- color.text-sub:       #6b7280
- color.text-muted:     #9ca3af
- color.primary:        #1a1f2e
- color.primary-hover:  #2d3548
- color.primary-active: #11151f
- color.primary-subtle: rgba(26, 31, 46, 0.08)
- color.success:        #059669
- color.warning:        #d97706
- color.danger:         #dc2626
- color.info:           #2563eb

默认采用浅色文学工作台，以保证长篇正文、差异和证据的长时间可读性；深色主题是应用外观设置中的可选主题，不能另行改变信息层级或交互语义。

### Typography
- font.display: "Source Han Serif SC", "Noto Serif SC", "STSong", "SimSun", serif
- font.ui: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif
- font.mono: "SF Mono", "Menlo", "Consolas", monospace
- scale: 12 / 13 / 14 / 16 / 18 / 20 / 24 / 32

### Spacing
- unit: 4px
- scale: 4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64

### Radius
- sm: 6px
- md: 10px
- lg: 12px

### Shadow
- sm: 0 1px 2px rgba(0,0,0,0.20)
- md: 0 4px 12px rgba(0,0,0,0.25)
- lg: 0 8px 28px rgba(0,0,0,0.35)

### Layout
- titlebar-h: 40px
- sidebar-w: 220px
- right-panel-w: 360px
- content-max-w: none (fluid inside main area)
- breakpoints: compact < 1440, standard 1440×900, wide > 1600；1280×720 为必验窗口，更小窗口提供单栏阅读

## Component Spec

### Button
- default: surface-elevated bg, 1px border-subtle, text color, px-16 py-8, radius-md
- hover: surface-hover bg, border-strong
- active: primary-subtle bg, primary border
- primary: primary bg, #1e1b16 text, no border
- primary-hover: primary-hover bg
- ghost: transparent bg, text-sub
- disabled: opacity 0.4, cursor not-allowed

### Input / Textarea
- bg surface, border-subtle, radius-md
- focus: border-primary, subtle primary glow
- placeholder: text-muted

### Card
- bg surface-elevated, border-subtle, radius-lg, shadow-sm
- hover: border-strong

### Nav Sidebar
- fixed left, width sidebar-w, bg surface
- nav item: height 40, px-16, text-sub, radius-md, gap-12
- active: primary-subtle bg, primary text, left accent bar 3px
- hover: surface-hover

### Modal / Drawer
- overlay: rgba(0,0,0,0.55)
- panel: surface-popup, radius-lg, shadow-lg

### Tag
- small pill, bg surface-hover, border-subtle, text-sub, radius-sm
- active/warning variants use subtle colors

### Empty State
- centered, icon 48, muted text, action button

### Loading
- skeleton: surface-hover pulse
- spinner: primary, stroke 1.5

## App Shell + Canonical Nav

### Shell (React Layout)
- `Layout` component renders the title bar, app menu, sidebar, main outlet, and optional right panel. 首页与应用设置属于应用工作区，不渲染当前作品侧栏或项目级 AI 面板。
- Title bar contains macOS traffic lights (red/yellow/green) and one application-name menu on the left；不重复展示第二个品牌下拉。
- 展开侧栏宽 220px；主区域随真实侧栏宽度布局，折叠后不得保留固定 220px 空白。
- Right AI panel is a slide-over `position: fixed` overlay when toggled in writing/review phase. 创意驾驶舱使用自己的方向讨论工作面，避免出现两个通用聊天入口。

### Sidebar Nav Items (frozen order)
1. 创作驾驶舱   icon: Compass    path: /cockpit
2. 正文与章节   icon: BookOpen   path: /chapters
3. 审阅与候选   icon: GitPullRequest  path: /review
4. 导入与续写   icon: Upload     path: /import
5. 作品档案     icon: Library    path: /archive/outline
6. 版本与交付   icon: GitBranch  path: /versions

### Active Rule
- Use React Router `NavLink` with `isActive` callback.
- Active item gets `bg-primary-subtle text-primary` and a 3px left accent bar.

### Settings Access
- Application settings is reachable from the top model-status button or app menu at `/settings`.
- Settings uses a Codex-style two-column layout: a fixed 260px category sidebar with “返回应用”, search, and vertical categories; the right pane contains the active category, scrollable content, and an in-pane save footer.
- “返回应用” returns to the current project manuscript when a project is open, otherwise to Home.
- Project settings is reachable only from File → 项目设置 (`/project-settings`).

## Page List

| #  | Page            | Responsibility                                                         | Key Components                          | Navigates To                          |
|----|-----------------|------------------------------------------------------------------------|-----------------------------------------|---------------------------------------|
| 0  | Home            | Launch entry: new idea, import, recent projects                        | Hero, RecentList, EmptyState            | /cockpit, /import                     |
| 1  | Cockpit         | Creative phase: project overview, AI team chat, direction cards        | ChatPanel, DirectionCards, StatusBar    | /chapters, /review                    |
| 2  | Chapters        | Writing phase: chapter nav + rich editor, AI side panel toggle         | ChapterSidebar, Editor, AIPanel         | /review                               |
| 3  | Review          | Review phase: candidate text, annotations, diff, decisions             | CandidateList, DiffView, AnnotationBar  | /chapters                             |
| 4  | Import          | Import & continue: upload / paste, processing, error states            | DropZone, ProgressPanel, ErrorBanner    | /chapters                             |
| 5  | Archive         | Project archive with secondary nav                                     | SubNav, ArchiveContent                  | /archive/:tab                         |
| 5a | 作品方案        | Project proposal / pitch                                               | MarkdownPreview, MetaCards              | —                                     |
| 5b | 故事架构        | Story structure / arcs / beats                                         | TreeView, BeatCards                     | —                                     |
| 5c | 人物与势力      | Characters & factions                                                  | CharacterGrid, FactionList              | —                                     |
| 5d | 世界设定        | World-building entries                                                 | CategoryNav, LoreCards                  | —                                     |
| 5e | 时间线与事实    | Timeline & facts                                                       | Timeline, FactTable                     | —                                     |
| 5f | 伏笔与关系      | Foreshadowing & relationship graph                                     | ForeshadowList, RelationMap (simplified)| —                                     |
| 6  | Versions        | Versions & delivery: snapshots, export, manuscript                     | VersionTimeline, ExportActions          | —                                     |
| 7  | Settings        | Application settings: model, plan, templates, appearance, advanced     | SettingsSidebar, Search, FormGroups     | /chapters or /                        |
| 8  | ProjectSettings | Project-level settings only                                            | FormGroups                              | —                                     |
| 9  | 智能体实验室    | 原创测试集、费用确认、运行观察、匿名盲评和回归报告                     | RunConfig, Observer, BlindRating, Report | /                                    |

智能体实验室从“设置 → 高级 → 智能体实验室”进入，不与首页创作入口并列；属于应用工作区，不渲染作品侧栏。候选在整组评分提交前只显示稳定匿名槽位，提交成功后才揭示来源和冻结绑定；所有状态都持续显示“不会写入作品正文或正式资产”。

### Stateful Views (embedded inside pages as needed)
- NoModelConfig: settings prompt when no model is configured
- ImportProcessing: progress during import
- CandidatePending: candidate text awaiting author confirmation
- ErrorState: generic error fallback

## Mock Schema

```ts
Project {
  id, title, subtitle, author, wordCount, targetWordCount,
  status: 'creative' | 'writing' | 'review' | 'finished',
  phase: string, updatedAt, coverColor, synopsis
}

Chapter {
  id, projectId, title, order, wordCount,
  status: 'draft' | 'review' | 'approved' | 'empty',
  content, summary, aiNotes
}

Character {
  id, projectId, name, role: 'protagonist' | 'supporting' | 'antagonist' | 'minor',
  factionId, tags[], summary, appearance, motivation
}

Faction {
  id, projectId, name, type: 'family' | 'organization' | 'nation' | 'sect',
  description, goals, conflicts
}

WorldEntry {
  id, projectId, category: 'geography' | 'history' | 'culture' | 'magic' | 'technology',
  title, content, importance
}

TimelineEvent {
  id, projectId, date, event, importance: 'major' | 'minor',
  linkedCharacters[], linkedChapters[]
}

Foreshadowing {
  id, projectId, title, description, status: 'planted' | 'growing' | 'resolved' | 'dropped',
  linkedChapters[], payoffChapterId?
}

Candidate {
  id, projectId, chapterId, type: 'rewrite' | 'continuation' | 'insert',
  content, diffFrom: string, status: 'pending' | 'accepted' | 'rejected',
  aiReason, createdAt
}

Version {
  id, projectId, label, wordCount, createdAt, note, isTagged
}

AiMessage {
  id, projectId, role: 'author' | 'ai',
  agent?: 'editor' | 'plot' | 'world' | 'style',
  content, timestamp
}
```

## Icon Set (Lucide)
Frequently used: Compass, BookOpen, GitPullRequest, Upload, Library, GitBranch, Settings, Plus, Search, Bell, ChevronRight, ChevronDown, MoreHorizontal, Trash2, Edit3, Check, X, RotateCcw, Sparkles, MessageSquare, Clock, AlertCircle, CheckCircle2, FileText, Download, Eye, Maximize2, Minimize2, PanelRight, User, Users, Map, Calendar, Link2, GitCommit.

## Motion
- Page load: 40ms staggered fade-up (opacity 0→1, translateY 6px→0), duration 260ms, ease-out.
- Hover transitions: 150ms ease.
- Panel slide: 220ms cubic-bezier(0.16, 1, 0.3, 1).
- Background: subtle radial gradient from top-left primary-subtle glow, plus noise texture overlay at 3% opacity.

## 多起点与阅读交互验收契约

本节是后续原型与生产界面的目标合同，尚未实现的入口明确标为未就绪。沿用现有 Token 与组件，不把文档更新视为页面已完成。

| 场景 | 必须呈现的交互 | 实施阶段 |
|---|---|---|
| 首页 | 从想法开始、我已有方案、导入小说并续写、打开及最近项目；实验室移至设置高级分类 | R4 |
| 灵感引导 | 每轮 1–3 个关键问题，可选“你来建议”；方向后可选试写，不先强制审完世界与全书路线图 | R4 交接，R5 试写 |
| 试写准备 | 方向卡提供“选定并准备试写”；可保存视角、语气、约束和字数，显示已保存/来源变化；生成按钮明确未就绪。原型仅本地演示，生产端绑定持久化命令 | R4 |
| 试写 | 明示非正式候选、范围与费用；喜欢后完善方案、调整语气/人物/节奏或放弃；不计定稿字数 | R5 |
| 试写转正 | 档案确认后按新约束复检，展示差异与风险并由作者采纳；不能一键绕过正式检查 | R5 |
| 已有方案 | 原文、来源、缺口与适用阶段；方向显示“不需要此步骤”，已有内容可验证复用，不伪造已选卡 | R4 |
| 已有正文 | 导入后立即阅读编辑；档案后台进度与续写限制清晰，不能误入灵感流程 | 原入口保留，R8 完整档案 |
| 驾驶舱 | 显示当前进展、待决定与一个主要下一步；已有正文、待审阅、运行、记忆待恢复各有投影，回看不回退作品阶段 | R4 基础，R5/R6 后续状态 |
| 方案操作 | 接受、锁定、入档分离；明确范围批量接受，未查看不自动接受；锁定只保护值 | R4 |
| 正文/审阅 | 1280×720 和 1440×900 下中央阅读区目标至少 560 CSS px；先收一级导航，再用结论抽屉，更窄时单栏；标签不拆字换行、确认按钮可达 | R5 |
| 批次摘要 | 优先待处理问题，重复发现归组但保留章节证据与状态；点击定位、返回原队列；逐章定稿独立 | R6 |

抽屉支持 Escape、焦点圈定与关闭后焦点返回；布局或路由变化保留未保存正文、未发送 AI 输入和队列位置。原型可用明确标识的本地状态演示这些流程，生产端必须使用权威投影，不能以示例伪造后端完成。

原型回归样例必须覆盖：零经验用户方向后试写；专业作者已有完整/不完整方案；已有正文项目回看策划；接受不锁定、锁定不接受；1280×720 双侧栏挤压；归组问题的多章证据与状态。新手和专业作者均使用同一组件体系，不增加强制模式切换。

### 创作进度面板

构思和正文生成使用同一进度组件：中文当前阶段、一句工作说明、按真实步骤更新的完成记录、等待时长与取消入口。动态流程使用移动的不定进度条，不虚构百分比或剩余时间；终态停止动画并保留结果。模型和用量收在折叠详情，普通运行面板不展示内部阶段 ID、节点名或 Action。原型演示必须标识未调用模型，不能作为真实运行状态。

### 候选文本编辑与锁

四类候选提供可折叠的文本编辑面板，按中文实体分组和字段名称选择内容，显示候选版本及未保存状态。保存生成待确认的新候选，锁定与接受保持独立；已锁字段先解锁才能改。身份、引用、数量和结构增删不在本入口开放。未保存时阻止重新生成和阶段推进；候选刷新或保存冲突保留输入与原基线，不静默覆盖。原型明确标识仅保存本地演示状态，生产端绑定应用层权威字段投影和版本校验。
