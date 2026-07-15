# Desktop App

这里是正式的 Wails v2.13.0 桌面工程，与 `prototype/web` 原型相互独立。

## 边界

- `main.go` 和 `app.go` 只负责桌面生命周期与依赖组装。
- `frontend/` 保存正式 React/TypeScript UI。
- `build/` 保存 Windows 与 macOS 构建资源，`build/bin/` 仅保存本地产物。
- 领域、应用、持久化、智能体和平台适配代码位于仓库根目录的 `internal/`。
- Wails binding 必须保持轻薄，React 组件不得直接访问数据库、文件系统或模型供应商。

## 本地命令

Wails v2.13.0 要求 Go 1.25 或更高版本；当前持久化依赖把 module 下限提升到 Go 1.25.7。本项目通过 `go.mod` 的 toolchain 指令固定 Go 1.25.12。

```bash
cd apps/desktop/frontend
npm install
npm test
npm run build

cd ..
/Users/shenlee/go/bin/wails dev
/Users/shenlee/go/bin/wails build -clean
```

不要提交 API key、用户稿件、`.zjproject` 项目包、本地数据库或 `build/bin/`。
