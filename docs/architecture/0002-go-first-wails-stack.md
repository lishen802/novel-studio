# ADR-0002：采用 Go-first 与 Wails v2，Rust 按需作为 Sidecar

- 状态：已接受
- 日期：2026-07-15
- 取代：ADR-0001

## 背景

产品由个人主导开发，开发者当前主要使用 Go，对 Rust 不熟悉。项目需要长期迭代多智能体编排、模型流式调用、本地数据、导入导出、版本恢复和 Windows/macOS 桌面能力。技术选择必须同时考虑商业产品质量与实际交付速度。

Wails v2 当前提供稳定的 Go + Web 技术桌面开发路径，支持 React/TypeScript、Windows、macOS、Go 到 TypeScript 类型生成和生产打包。Wails v3 在决策时仍为 Alpha，因此不作为首版商业基线。

## 决策

- Wails v2 作为 Windows 与 macOS 桌面框架。
- Go 作为领域、应用、持久化、多智能体运行时、模型适配器和平台适配器的主要语言。
- React、TypeScript 与 Vite 作为前端技术栈。
- SQLite 作为本地数据库，通过 Go 接口隔离具体驱动。
- Goroutine、Context 和持久化任务记录承载模型流式调用、并发、取消、重试和恢复。
- Windows Credential Manager 与 macOS Keychain 通过平台适配器保存模型凭据。
- Rust 不进入首版必需依赖；只有满足明确准入条件时才加入。

## Rust 准入条件

满足以下至少一项，并经过独立 ADR 批准，才能新增 Rust 模块：

1. Go 缺少可靠、可商业分发的实现。
2. 可复现的性能测试证明 Go 实现是用户可感知瓶颈。
3. 必须接入主要由 Rust 生态维护的原生组件，且替代方案明显更差。
4. 该模块使用 Rust 能显著降低安全或跨平台维护风险。

SQLite、HTTP、模型编排、普通文本处理、业务规则和凭据加密不得仅因偏好而改用 Rust。

## Go 与 Rust 的边界

首选独立 Sidecar 进程，并使用版本化 JSON Lines 协议通过 stdin/stdout 通信。需要长驻双向通信时，可以升级为 Unix Domain Socket 与 Windows Named Pipe。CGO/FFI 是最后选择，只有进程边界无法满足明确性能要求时才评估。

Sidecar 必须可以独立启动、健康检查、超时、取消和崩溃恢复。Go 端只依赖接口和协议，不直接依赖 Rust 内部类型。

## 理由

1. 主力语言熟练度能显著提高个人项目的持续交付速度和可维护性。
2. Go 的并发模型适合模型流式请求、任务队列、取消、超时和多供应商编排。
3. Wails v2 能复用现有 React/Vite 原型经验，并生成 Go/TypeScript 绑定。
4. Go-first 避免从第一天承担双语言构建、调试、签名和发布成本。
5. Sidecar 边界保留了以后使用 Rust 的能力，同时避免早期 FFI 和 ABI 风险。

## 影响

- 自动更新、系统凭据和部分商业分发能力需要在 Go/Wails 生态内单独设计和验证。
- Wails API 只能出现在桌面入口和平台适配层，核心业务不得绑定 Wails。
- Wails v3 稳定后通过新的 ADR 评估迁移，不在开发过程中追随 Alpha API。
- Windows 与 macOS 必须在原生 CI Runner 上分别验证构建、签名、安装、升级和系统集成。

## 参考

- Wails v2 Introduction: https://wails.io/docs/introduction/
- Wails v2 Project Structure: https://wails.io/docs/gettingstarted/firstproject/
- Wails v3 Status: https://v3.wails.io/status/

