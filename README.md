# 织境 Story Studio

面向中文长篇小说创作的商业级桌面软件项目，目标平台为 Windows 与 macOS。产品围绕正文、大纲、角色、势力、世界、地图、时间线、关系拓扑、版本管理和异构多智能体协作构建。

## 当前状态

- `prototype/web`：已经通过验收的完整交互原型，可独立运行。
- `docs`：产品基线、技术选型、规格和新任务接手资料。
- `apps/desktop`：正式 Tauri 2 桌面应用的预留目录。
- `crates`：Rust 领域层、应用层、持久化、多智能体运行时和模型适配器的预留目录。
- `packages`：前端共享 Schema 与 UI 组件的预留目录。

正式产品尚未初始化 Tauri 业务代码。下一阶段先完成产品规格、领域模型、架构边界和版本路线图，再按实施计划开发。

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

## 推荐技术栈

- 桌面：Tauri 2
- 后端：Rust + Tokio
- 前端：React + TypeScript + Vite
- 本地数据库：SQLite + SQLx
- 密钥：系统钥匙串或 Tauri Stronghold
- 自动更新：Tauri Updater
- 模型接入：独立 Provider Adapter 层

从新任务开始工作时，先阅读 [接手说明](docs/handoff/START-HERE.md)。

