# 织境 Story Studio

面向中文长篇小说创作的商业级桌面软件项目，目标平台为 Windows 与 macOS。产品围绕正文、大纲、角色、势力、世界、地图、时间线、关系拓扑、版本管理和异构多智能体协作构建。

## 当前状态

- `prototype/web`：已经通过验收的完整交互原型，可独立运行。
- `docs`：产品基线、技术选型、规格和新任务接手资料。
- `apps/desktop`：正式 Wails v2 桌面应用的预留目录。
- `crates`：早期 Rust-first 方案留下的空目录；后续仅在确有需要时存放隔离的 Rust Sidecar。
- `packages`：前端共享 Schema 与 UI 组件的预留目录。

正式产品尚未初始化 Wails 或 Go 业务代码。下一阶段先完成产品规格、领域模型、架构边界和版本路线图，再按实施计划开发。

## 原型运行

```bash
cd prototype/web
npm install
npm run dev
```

测试命令：

```bash
npm run typecheck
npm test -- --run
npm run build
npx playwright test
```

## 已批准技术栈

- 桌面：Wails v2
- 后端：Go
- 前端：React + TypeScript + Vite
- 本地数据库：SQLite，通过 Go 数据访问适配层使用
- 密钥：通过 Go 平台适配层访问 Windows Credential Manager 和 macOS Keychain
- 异步任务：Goroutine、Context 和持久化任务状态
- 模型接入：独立 Provider Adapter 层
- 可选扩展：只有 Go 明确不适合时才使用独立 Rust Sidecar

当前有效架构决策见 [ADR-0002](docs/architecture/0002-go-first-wails-stack.md)。从新任务开始工作时，先阅读 [接手说明](docs/handoff/START-HERE.md)。
