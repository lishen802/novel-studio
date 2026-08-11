# Wails 双平台构建矩阵

本矩阵定义普通 Foundation CI 对 Windows/macOS 原生 production build 的可审计基线。它生成短期检查产物，不代表签名、公证、安装或商业发布完成。

## 固定矩阵

| Job | Runner | 架构 | Wails 输出 | CI 检查包 |
| --- | --- | --- | --- | --- |
| macOS production build | `macos-15` | arm64 | `NovelStudio.app` | `NovelStudio-macos-arm64.tar.gz` |
| Windows production build | `windows-2025` | x64 | `NovelStudio.exe` | `NovelStudio-windows-x64.zip` |

Go 版本来自根 `go.mod`/`toolchain`，Node 固定为 22，Wails 固定为 v2.13.0。Runner label 发生弃用或产品支持矩阵扩大时，必须通过独立变更更新本表和 CI，不依赖 `*-latest` 静默迁移。

## 产物契约

`scripts/ci/wails-build-artifact.mjs` 在原生 Runner 上验证：

- 预期输出路径存在且非空；
- macOS app 含有效 `Info.plist` 和 Mach-O 入口；
- Windows 输出具有 PE/MZ 入口；
- manifest 记录相对路径、大小、SHA-256、commit、run、Runner 和工具链版本；
- manifest 不复制环境变量、工作区绝对路径、正文、Prompt、凭据、数据库或签名材料。

`build-manifest.json` 的 `entrypoint_sha256` 用于确认下载检查包对应本次构建，不是代码签名或来源证明。

## CI artifact

两个 Job 仅上传显式白名单文件：

- 平台压缩包；
- 同一次检查生成的 `build-manifest.json`。

保留期为 7 天。macOS 先使用 tar 封装以保留 app 内入口权限；Windows 使用 PowerShell zip。CI artifact 不发布到 GitHub Release，不包含用户项目、测试数据库、日志目录、API Key 或签名证书。

生产二进制可以包含应用运行所必需的已编译代码、静态资源和版本化数据库迁移；“不上传 SQL”是指不得附带用户数据库、SQL dump、调试查询或失败现场数据，而不是从已编译应用中移除正常迁移资源。

## 下载检查

1. 打开对应 Foundation CI run；
2. 下载与平台、架构和 commit SHA 对应的 artifact；
3. 比较外层 `build-manifest.json` 与压缩包内 manifest；
4. 解包后确认只有 `NovelStudio.app`/`NovelStudio.exe` 与 manifest；
5. 需要启动、文件对话框、WebView 或项目读写验证时，转入双平台人工验收，不以 CI 结构检查代替。

## 失败分类

- `Build ... application`：Wails/Go/前端 production build 失败；
- `Validate ...`：输出缺失、入口格式错误、plist 错误或 manifest 生成失败；
- `Package ...`：tar/zip 创建失败、路径或磁盘空间问题；
- `Upload ...`：GitHub artifact 服务、上传路径或保留策略失败；
- `Run Go tests on Windows`：Windows 专属 Go 行为失败，与打包失败分开处理。

任何失败都不得通过 `continue-on-error`、缩小上传范围到空文件或删除平台 Job 规避。

## R9 边界

以下能力留给 R9：macOS Intel/universal、Windows ARM64、正式版本号与渠道、DMG/PKG/MSI/MSIX/NSIS、Developer ID、Apple 公证、Gatekeeper、Authenticode、安装/升级/卸载、Release artifact 和长期供应链证明。
