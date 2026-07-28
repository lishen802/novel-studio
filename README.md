# 织境 Story Studio

面向 Windows 与 macOS 的本地优先长篇小说多智能体创作桌面软件。正式产品采用 Go、Wails v2.13.0、React、TypeScript、Vite 与 SQLite；Rust 只在有测量证据的性能或原生能力缺口出现后，以隔离 Sidecar 方式按需引入。

## 当前状态

仓库已进入正式产品研发。P0 桌面基础纵向切片包含：创建/打开本地项目、新建章节、正文编辑、乐观 revision、恢复草稿、异常打开保护和退出前落盘。P1 已交付五级大纲、故事线与场景卡；角色、势力、知识边界和有向关系拓扑；区域层级、相邻关系、层级地图、地点及叙事关联、物品流转、分范围世界规则和伏笔；双顺序时间线、可确认事实、精确正文证据、实体作用域、显式冲突与基础一致性提醒。P2 已交付正文、大纲节点、角色档案和故事事实的统一可审阅变更集，支持字段级对比、编辑候选、逐项接受/拒绝、整组原子应用、基线冲突保护和应用前恢复点；同时具备显式章节版本历史、命名项目快照、安全恢复，以及隔离整部作品状态的剧情分支。剧情分支支持旧项目懒初始化、新建、切换、完整数据域比较、严格快进合并、冲突阻止和安全删除；合并前会自动建立恢复点。`prototype/web` 是已验收的 UX 参考，继续独立运行，不是正式产品架构。

下一阶段进入 Provider 与多智能体运行时，再建设小说导入重建和完整创作闭环；字段级分支三方合并、发布签名和商业授权尚未在当前切片交付。

## 开发环境

- Go：根目录 `go.mod` 声明最低 Go 1.25.7，并固定 `toolchain go1.25.12`。
- Wails CLI：v2.13.0。
- Node.js：22，配套 npm 10 或兼容版本。
- macOS：Xcode Command Line Tools。
- Windows：标准 Go 工具链与 Microsoft WebView2 Runtime；按 Wails v2 Windows 环境要求准备构建机。

安装 Wails CLI：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
wails doctor
```

## 可复制的验证与运行命令

首次安装正式前端依赖并构建嵌入资源：

```bash
cd apps/desktop/frontend
npm ci
npm run build
cd ../../..
```

运行 Go、正式前端与原型门禁：

```bash
go test -count=1 -race ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...

cd apps/desktop/frontend
npm test -- --run
npm run typecheck
npm run build

cd ../../../prototype/web
npm ci
npm test -- --run
npm run typecheck
npm run build
```

启动和打包正式桌面应用：

```bash
cd apps/desktop
wails dev
wails build -clean -m
```

单独启动已验收网页原型：

```bash
cd prototype/web
npm ci
npm run dev
```

不要在仓库根目录使用 `go test ./...`：安装前端依赖后，该 pattern 可能遍历 `node_modules`。应始终使用 `./apps/desktop ./internal/...`。Wails 构建使用 `-m`，依赖更新通过明确的 `go get` 和干净检出下的 `go mod tidy` 完成。

## 目录与依赖边界

| 路径 | 职责 |
| --- | --- |
| `apps/desktop` | Wails 入口、薄绑定、原生对话框和 React 正式 UI。 |
| `internal/domain` | 不依赖 Wails、SQLite 或 UI 的领域模型与规则。 |
| `internal/application` | 用例、端口、授权能力门禁和工作区编排。 |
| `internal/persistence` | SQLite、迁移、revision 与恢复草稿实现。 |
| `internal/platform` | 文件系统、ID、时钟等平台适配。 |
| `prototype/web` | 独立的已验收浏览器原型，只作为 UX 参考。 |
| `docs` | 产品基线、批准规格、ADR、路线图、运维和验收资料。 |

React 只能调用 `StudioClient`；Wails API 只能转发应用服务；持久化与模型供应商不能从组件直接访问。不同智能体使用不同供应商和模型的能力必须保留，不能把运行时收缩为单模型流水线。

## 项目数据位置

作者在创建项目时选择保存目录。每部作品是完整的 `书名.zjproject` 目录包：

```text
书名.zjproject/
├── project.db
├── attachments/
├── exports/
├── branches/
└── snapshots/
```

当前正文、章节、revision、恢复草稿和已实现的结构化故事资产存放在 `project.db`；剧情分支注册表和完整分支头存放在 `branches/`；命名快照及恢复前自动快照存放在 `snapshots/`。备份、迁移或跨平台传递时必须复制整个 `.zjproject` 目录，不能只复制正在使用的数据库文件。异常处理见[项目恢复与数据保护](docs/operations/project-recovery.md)。

## 隐私与安全规则

- 正文和核心资产默认保存在作者选择的本地项目包中。
- 不提交或上传 API 密钥、模型凭据、签名证书、作者稿件、本地数据库和构建产物。
- CI 不注入真实模型密钥，也不访问真实模型 API。
- UI 只显示稳定、安全的错误摘要，不显示 SQL、数据库内部信息或正文。
- 未经作者明确授权，不把项目包、正文或诊断数据发送到任何外部服务。

## 已知限制

- 当前是 P0/P1 完成、P2 持续建设中的研发版本，不是可对外发布的完整 V1。
- 当前候选变更覆盖正文、大纲节点、角色档案和故事事实；更多资产类型仍需逐步纳入统一变更协议。
- 剧情分支第一版只支持子分支向直接来源分支严格快进合并；双方都发生变化时会阻止合并，字段级三方冲突解决尚未实现。
- 多智能体写作、模型供应商、小说导入重建、伏笔证据扩展、阶段事件和世界规则的时期范围尚未实现。
- V1 使用默认放行的 `LicenseService`；30 天试用、激活和月度续期在后续版本启用。
- 当前构建未完成正式 Windows 签名、macOS Developer ID 签名/公证、安装器与自动升级。
- Rust、CGO/FFI、云端必需依赖和 Wails v3 均不在当前基线内。

开始新任务前先阅读[接手说明](docs/handoff/START-HERE.md)。双平台人工验收使用[P0 基础验收清单](docs/testing/foundation-acceptance.md)。
