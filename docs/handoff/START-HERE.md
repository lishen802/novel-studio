# 织境 Story Studio 交接指南

## 当前状态（2026-08-10）

本仓库正在从早期“写作资产管理/辅助创作”原型，转向 AI 主创作的长篇小说桌面产品。当前基础代码已具备 P0–P4 的部分纵切：正文编辑、revision 与恢复；结构化故事资产；变更集/快照/分支；OpenAI/Anthropic API 与 Agent Run；TXT/Markdown 导入和基础资产抽取。它们是实现基线，不代表完整新产品已完成。

生产基线为 Go + Wails v2 + React/TypeScript + Vite + SQLite。`prototype/web` 是 UX 参考，不得成为生产层间依赖。

R1.1 与 R1.2 已完成。R1.3 的实现已覆盖项目级 OS lease、真正只读 store、结构化争锁救局和恢复副本；提交/合并前仍需按其实施计划记录 macOS/Windows CI 与手工验收证据。R1.4（同步盘/运行期文件锁、写入探针与 Windows 路径边界）尚未开始，不能把 R1.3 的完成误报为整个 R1 退出。

## 必读文档

1. [文档索引](../README.md)；
2. [完整产品需求](../product/requirements.md)；
3. [产品与交互设计](../product/design.md)；
4. [系统技术设计](../architecture/system-design.md)；
5. [总体开发路线图](../roadmap/development-roadmap.md)；
6. [Go-first 架构决策](../architecture/0002-go-first-wails-stack.md) 与 [Wails 目录决策](../architecture/0003-wails-project-layout.md)；
7. 开始具体能力前阅读对应[实施计划](../plans/)，并在相关时阅读[基础验收](../testing/foundation-acceptance.md)和[项目恢复](../operations/project-recovery.md)。

## 不可突破的约束

- AI 负责规划与写作，作者确认方向、作品方案、章节路线图、影响方案和定稿；
- 已定稿内容及确认资产是 canonical 事实，AI 候选不得静默覆盖；
- 正文分批 1–50 章生成；未完成当前批次审阅定稿不得继续下一批；
- 长篇连续性依赖显式、可追溯的上下文与记忆层，不依赖无限聊天记录；
- 模型设置属于应用设置；API Key 存 OS 凭据库，订阅连接只使用官方本地运行时，不能复制 OAuth/Cookie；
- 数据库继续使用版本化 Goose 迁移，不采用 GORM 自动迁移；
- Wails binding 薄、React 不直连持久化和模型、生产能力以 Go 领域/应用/适配器边界实现；
- 不提交密钥、小说正文、项目数据库、签名凭据或用户资料。

## 工作方式

先以当前三份权威设计校验需求，再按总体开发路线图选择首个未完成阶段，并为当前具体能力建立可测试的实施计划。实施计划不能修改产品范围或绕过架构决策。大型能力必须在隔离 worktree 内实现；完成验证后再合入主线。旧需求、设计和计划已从工作树移除，仅可作为 Git 历史参考。
