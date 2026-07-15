# 正式产品仓库设计

- 日期：2026-07-15
- 状态：已批准

## 目标

建立一个独立、可长期演进的正式产品仓库，将已经验收的网页原型保留为 UX 基准，并为 Windows/macOS 桌面实现、Go 核心模块、可选 Rust Sidecar、共享前端包、产品文档和跨层测试划定清晰边界。

## 目录设计

```text
novel-studio/
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── specifications/
│   ├── plans/
│   └── handoff/
├── prototype/web/
├── apps/desktop/
├── cmd/novel-studio/
├── internal/
│   ├── domain/
│   ├── application/
│   ├── persistence/
│   ├── agents/
│   ├── providers/
│   └── platform/
├── rust/sidecars/
├── packages/
│   ├── story-schema/
│   └── ui/
├── fixtures/
└── tests/
```

## 边界说明

- `prototype/web` 保存当前可运行原型，继续用于需求验证，不直接承担正式数据迁移和模型调用职责。
- `apps/desktop` 保存 Wails v2 配置、桌面构建资源和正式 React 前端，不承载核心领域规则。
- `cmd/novel-studio` 是 Go 桌面进程入口，只负责组装依赖和启动应用。
- `internal/domain` 定义小说、章节、场景、角色、势力、地点、事件、事实、关系和版本等领域对象及不变量。
- `internal/application` 编排创作、分析、审稿、修订、恢复和导出用例。
- `internal/persistence` 负责 SQLite、文件资产、事务、迁移、备份和恢复。
- `internal/agents` 负责多智能体任务图、暂停恢复、重试、预算、证据和运行记录。
- `internal/providers` 统一不同云模型和本地模型的鉴权、能力、流式响应、错误和成本信息。
- `internal/platform` 隔离 Wails、系统钥匙串、自动升级、窗口和文件关联等平台能力。
- `rust/sidecars` 只保存经过 ADR 批准的隔离式 Rust 工具；默认保持为空。
- `packages/story-schema` 保存前后端契约与生成类型。
- `packages/ui` 保存正式应用的可复用设计系统和故事可视化组件。
- `fixtures` 保存无版权风险的测试小说和导入样本。
- `tests` 保存跨模块、迁移、恢复和桌面端到端测试。

## 数据流原则

React 界面只通过 Wails 生成的类型化绑定访问 Go 应用服务。应用层调用领域逻辑，并通过接口访问持久化、模型供应商和系统能力。长任务先写入持久化任务记录，再执行模型步骤；任何阶段崩溃后都能根据任务状态恢复或安全重跑。

确需 Rust 时，Go 通过版本化 Sidecar 协议调用独立进程。首选 JSON Lines 与 stdin/stdout；只有长期双向吞吐需要时才使用 Unix Domain Socket 或 Windows Named Pipe，不默认采用 CGO/FFI。

## 错误处理原则

- 领域错误、输入错误、供应商错误、网络错误、配额错误和内部错误使用可区分的错误类型。
- 模型步骤记录供应商、模型、提示版本、输入摘要、Token、费用、耗时和重试原因。
- 写入正文和结构化资产必须经过事务或可回滚变更集。
- 密钥不得进入日志、项目导出、崩溃报告或测试夹具。

## 测试原则

- 领域不变量、应用服务和数据迁移使用 Go 单元测试与集成测试。
- 可选 Rust Sidecar 必须拥有独立 Cargo 测试和 Go 侧协议契约测试。
- Provider Adapter 使用契约测试和录制的无敏感信息响应。
- React 使用组件测试；关键作者流程使用 Playwright。
- Windows 与 macOS 分别验证安装、升级、文件关联、密钥存储和崩溃恢复。

## 本次交付范围

本次只创建正式仓库结构、复制已验收原型、记录产品基线与技术决策。Wails 工程、Go module、SQLite Schema 和正式业务模块留到下一阶段规格与实施计划获批后创建。
