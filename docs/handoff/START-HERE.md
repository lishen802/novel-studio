# 新任务接手说明

## 从这里开始

这是织境 Story Studio 的正式产品仓库。上一阶段已经完成并验收网页交互原型，本阶段将把产品规划转化为可商业发布的 Windows/macOS 桌面软件。

开始新任务时按顺序阅读：

1. `docs/product/product-baseline.md`
2. `docs/architecture/0001-desktop-stack.md`
3. `docs/specifications/2026-07-15-product-repository-design.md`
4. `prototype/web/README.md`

## 已确定事项

- 正式技术栈：Rust + Tauri 2 + React + TypeScript + Vite。
- 数据层：SQLite + SQLx，本地优先。
- 智能体必须支持不同角色使用不同供应商和模型。
- 现有原型是 UX 和功能基线，正式代码需要重新划分领域、应用、基础设施和 UI 边界。
- 产品目标平台是 Windows 与 macOS，并按未来商业发布标准设计。

## 下一任务建议

先完成“商业桌面产品总体设计规格”，建议依次确定：

1. V1、V1.5、V2 的版本范围与验收标准。
2. 小说项目文件格式、SQLite Schema、资产与备份策略。
3. 领域模型和跨模块接口。
4. 多智能体任务图、上下文组装、证据与仲裁协议。
5. 模型供应商能力矩阵、凭据、安全、预算和限流策略。
6. 桌面生命周期、自动升级、许可证和崩溃恢复。
7. Windows/macOS CI、签名、公证与发布流程。

完成规格审查后，再创建详细实施计划和首个 Tauri/Rust 骨架。

