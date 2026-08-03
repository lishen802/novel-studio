# 新任务接手说明

## 当前阶段

织境 Story Studio 已从网页原型阶段进入正式 Windows/macOS 产品研发。P0 桌面基础纵向切片已经实现本地项目、章节编辑、revision、恢复草稿、SQLite 完整性保护和 Wails v2.13.0 桌面壳；P1 已实现结构化故事世界与连续性基础；P2 已实现统一可审阅变更集、版本历史、项目快照和完整剧情分支。P3 已实现 OpenAI Responses 与 Anthropic Messages 流式 Provider、macOS Keychain/Windows Credential Manager 凭据隔离、规划/写作/连续性审阅三角色异构编排、持久化运行记录、中断恢复、预算与 `context-v2` 上下文裁剪。P4 第一切片已实现 TXT/Markdown 小说的安全归档、编码识别、SHA 重复检测、确定性卷章切分、持久化导入任务、作者预览/选择和正式章节原子应用；UI 仅接收有界正文预览，不传递整本原稿。托管 Windows/macOS CI 与双平台人工验收仍以对应工作流结果和验收记录为准。

当前下一组产品能力应建立在 P4 导入任务和证据行范围之上，分章抽取故事线、人物关系、大纲、世界设定和时间线，再做跨章合并、冲突保留与作者逐项确认。模型结果仍必须作为候选，不得直接污染正式项目数据。向量检索、并行任务队列和动态价格目录仍是后续运行时增强，不应抢先扩大当前模块边界。分支字段级三方冲突解决属于后续增强，不应阻塞第一版严格快进分支。大功能按 `AGENTS.md` 在对话中给出实施计划并使用隔离工作树，不把原型状态或组件直接搬入正式架构。

## 必读顺序

开始任何产品或架构任务前，依次完整阅读：

1. [`AGENTS.md`](../../AGENTS.md) 与仓库根目录 [`README.md`](../../README.md)
2. [产品基线](../product/product-baseline.md)
3. [批准的商业产品设计](../superpowers/specs/2026-07-15-commercial-product-design.md)
4. [Go-first 桌面技术规格](../specifications/2026-07-15-go-first-desktop-stack-design.md)
5. [产品仓库设计](../specifications/2026-07-15-product-repository-design.md)
6. [ADR-0002：Go-first Wails 技术栈](../architecture/0002-go-first-wails-stack.md)
7. [ADR-0003：Wails 工程布局](../architecture/0003-wails-project-layout.md)
8. [V1 总体实施路线图](../superpowers/plans/2026-07-15-v1-implementation-roadmap.md)
9. [Provider 与智能体创作纵向切片规格](../superpowers/specs/2026-07-29-provider-agent-authoring-design.md)
10. [智能体运行控制与上下文预算设计](../superpowers/specs/2026-07-30-agent-runtime-controls-design.md)
11. [小说导入基础设计](../superpowers/specs/2026-07-30-manuscript-import-foundation-design.md)
12. [P0 文件级实施计划](../superpowers/plans/2026-07-15-v1-foundation-vertical-slice.md)
13. [`prototype/web/README.md`](../../prototype/web/README.md) 与当前原型，只作为 UX 参考

涉及项目损坏、恢复或跨平台验收时，还要阅读[恢复说明](../operations/project-recovery.md)和[P0 基础验收清单](../testing/foundation-acceptance.md)。

## 已批准且不得无决策改写的基线

- 正式技术栈：Go + Wails v2.13.0 + React + TypeScript + Vite + SQLite。
- 架构：模块化单体；Go 领域、应用、持久化、智能体运行时、Provider、平台适配和 UI 边界明确。
- 正式 Wails 工程位于 `apps/desktop`；根目录保持单一 Go module。
- Rust 只为经过测量的性能瓶颈、可靠 Go 库缺口或更安全的原生能力按需引入，优先隔离 Sidecar；CGO/FFI 需要新 ADR。
- Wails v3 Alpha 不是生产基线；稳定发布后也必须通过新 ADR 重新评估。
- React 不直接访问 SQLite 或模型 Provider；Wails binding 保持薄层。
- 多智能体复核必须支持不同角色选择不同供应商和模型。
- V1 授权能力门禁默认放行，但接口必须允许未来加入 30 天试用、激活、月度续期和离线宽限。
- 原型保持可运行，正式实现不反向依赖原型。

## P4 后续切片门禁

1. 当前分支的 Ubuntu 自动测试与 macOS/Windows production build 工作流通过。
2. [P0 基础验收清单](../testing/foundation-acceptance.md)在两个系统均有记录，失败项已关联缺陷。
3. 工作区无作者稿件、数据库、密钥、签名材料或构建产物被跟踪。
4. 在对话中给出包含领域 Schema、迁移、用例、UI 适配和双平台测试的实施计划；除非用户明确要求，不创建实施计划文件。
5. 对新的跨模块能力完成产品、架构和数据安全检查后，再开始生产编码。
