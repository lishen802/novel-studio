# ADR-0001：桌面技术栈选择 Rust 与 Tauri 2

- 状态：已接受
- 日期：2026-07-15

## 背景

产品需要同时运行在 Windows 和 macOS，复用现有 React 交互原型，并长期承载本地小说数据库、模型密钥、多模型流式请求、后台任务、自动更新、版本迁移和商业分发。

候选方案是 Go + Wails 与 Rust + Tauri。

## 决策

采用以下正式技术栈：

- Tauri 2 作为桌面壳与系统能力桥接层。
- Rust 作为桌面后端和核心业务实现语言。
- React、TypeScript 与 Vite 作为前端技术栈。
- Tokio 负责异步任务和模型流式调用。
- SQLite 与 SQLx 负责本地持久化和迁移。
- 系统钥匙串或 Tauri Stronghold 保存模型凭据。
- Tauri Updater 负责应用升级。

## 理由

1. Tauri 2 已是正式稳定代际，并提供文件、SQL、Stronghold、Updater、单实例等官方插件。
2. 可以复用当前 React/Vite 原型的界面经验和部分前端资产。
3. Rust 的类型系统和内存安全适合维护本地数据、迁移、并发任务和权限边界。
4. Tauri 的能力权限机制更适合包含用户手稿与 API Key 的商业桌面应用。
5. Windows 和 macOS 的安装包、签名、公证与商店分发路径清晰。

## 未选择 Go + Wails 的原因

Go 的开发效率、编译速度和并发模型具有明显优势，Wails 2 也足以构建跨平台桌面应用。不过，Wails 3 在决策时仍处于 Alpha，Wails 在官方插件、密钥存储、权限控制和自动升级的一体化程度上弱于 Tauri。若未来需要独立的高吞吐后台服务，Go 仍可作为可选的服务端语言，但不进入桌面端首版基线。

## 影响

- 团队需要承担 Rust 学习和较慢编译速度的成本。
- 核心领域逻辑不得直接写在 Tauri command 或 React 组件中。
- 模型供应商必须通过统一适配接口接入，避免上层绑定具体 SDK。
- Windows 和 macOS 应分别在原生 CI Runner 上完成构建、签名和端到端验证。

