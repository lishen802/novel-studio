# 织境 Story Studio

面向 Windows 与 macOS 的本地优先长篇小说多智能体创作桌面软件。正式产品采用 Go、Wails v2.13.0、React、TypeScript、Vite 与 SQLite；Rust 只在有测量证据的性能或原生能力缺口出现后，以隔离 Sidecar 方式按需引入。

## 当前状态

仓库已进入正式产品研发，P0 桌面基础纵向切片包含：创建/打开本地项目、新建章节、正文编辑、乐观 revision、恢复草稿、异常打开保护和退出前落盘。`prototype/web` 是已验收的 UX 参考，继续独立运行，不是正式产品架构。

接下来的 P1 将实现故事工程与结构化模型；多智能体创作、小说导入重建、完整版本/快照、模型供应商、发布签名和商业授权尚未在当前切片交付。

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
└── snapshots/
```

当前正文、章节、revision 和恢复草稿存放在 `project.db`。备份、迁移或跨平台传递时必须复制整个 `.zjproject` 目录，不能只复制正在使用的数据库文件。异常处理见[项目恢复与数据保护](docs/operations/project-recovery.md)。

## 隐私与安全规则

- 正文和核心资产默认保存在作者选择的本地项目包中。
- 不提交或上传 API 密钥、模型凭据、签名证书、作者稿件、本地数据库和构建产物。
- CI 不注入真实模型密钥，也不访问真实模型 API。
- UI 只显示稳定、安全的错误摘要，不显示 SQL、数据库内部信息或正文。
- 未经作者明确授权，不把项目包、正文或诊断数据发送到任何外部服务。

## 已知限制

- 当前是 P0 研发基础，不是可对外发布的完整 V1。
- 尚未实现大纲、角色、世界、时间线、关系图、多智能体写作和小说导入重建。
- V1 使用默认放行的 `LicenseService`；30 天试用、激活和月度续期在后续版本启用。
- 当前构建未完成正式 Windows 签名、macOS Developer ID 签名/公证、安装器与自动升级。
- Rust、CGO/FFI、云端必需依赖和 Wails v3 均不在当前基线内。

开始新任务前先阅读[接手说明](docs/handoff/START-HERE.md)。双平台人工验收使用[P0 基础验收清单](docs/testing/foundation-acceptance.md)。
