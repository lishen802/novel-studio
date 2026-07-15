# ADR-0003：Wails 工程位于 apps/desktop

- 状态：已接受
- 日期：2026-07-15
- 补充：ADR-0002

## 背景

商业产品设计中的建议目录曾将 Wails 配置和 React UI 放在 `apps/desktop`，同时把 Go 进程入口放在 `cmd/novel-studio`。Wails v2 的标准工程把 Go 入口、`wails.json`、`build` 与 `frontend` 作为一个构建单元；Go 的 `embed` 规则也不允许入口通过 `..` 引用工程外的前端构建目录。

Wails v2.13.0 还把最低 Go 版本提升到 1.25。当前开发机的系统 Go 是 1.23.12，但 Go toolchain 自动选择机制可以按 module 声明下载并使用 Go 1.25 系列。

## 决策

- 正式 Wails 工程位于 `apps/desktop`。
- Go 入口使用 `apps/desktop/main.go`，Wails 配置使用 `apps/desktop/wails.json`。
- 正式 UI 位于 `apps/desktop/frontend`，平台构建资源位于 `apps/desktop/build`。
- 仓库只保留一个根 Go module；领域、应用、持久化、智能体运行时和平台适配仍位于根目录 `internal/`。
- 根 module 最低版本为 Go 1.25.7，并使用 `toolchain go1.25.12` 提供可复现的本地与 CI 工具链。Wails 本身要求 Go 1.25.0，当前 goose 迁移依赖把实际 module 下限提升到 1.25.7。
- Wails 固定为 v2.13.0；升级 Wails 或 Go 主/次版本时必须先运行双平台构建验证。

## 理由

1. 采用 Wails 官方工程形态，避免维护自定义资源复制脚本。
2. `main.go` 可以直接嵌入 `frontend/dist`，不使用在 Windows 上行为不一致的符号链接。
3. Wails CLI 可以在 `apps/desktop` 直接执行开发、绑定生成和生产构建。
4. 入口位置变化不改变模块化单体的依赖方向；业务代码仍不依赖 Wails。

## 影响

- 早期设计里的 `cmd/novel-studio` 不再创建；该目录只是建议而非已经实现的公共契约。
- `apps/desktop` 是唯一允许导入 Wails application/runtime API 的组装层之一；其他使用必须位于明确的平台适配包。
- 在没有 `frontend/dist` 时，包含 `go:embed` 的桌面入口不能单独编译，因此全量 Go 验证必须先执行前端构建。
- 根 module 同时覆盖正式前端和浏览器原型；安装 npm 依赖后，Go 的递归 `./...` pattern 会进入 `node_modules`。因此测试和 vet 必须显式使用 `./apps/desktop` 与 `./internal/...`，Wails 构建使用 `-m` 跳过其内部 `go mod tidy`。依赖更新由明确的 `go get` 完成，必要的 tidy 在没有 `node_modules` 的干净检出中执行。
- 如果未来恢复 `cmd/` 入口，必须通过新 ADR 解决资源嵌入、CLI 工作目录和双平台构建，不能依赖符号链接。

## 参考

- Wails v2.13.0 release: https://github.com/wailsapp/wails/releases/tag/v2.13.0
- Wails project structure: https://wails.io/docs/gettingstarted/firstproject/
- Go toolchains: https://go.dev/doc/toolchain
