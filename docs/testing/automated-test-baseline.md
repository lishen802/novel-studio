# 自动化测试基线

本基线定义普通提交必须通过的本地和 CI 自动验证。它不替代 Windows/macOS 人工验收、正式签名、公证、安装或升级验证。

## 环境

- Go：以根目录 `go.mod` 和 `toolchain` 为准；
- Node.js：22；
- npm：使用两个已提交的 `package-lock.json` 和 `npm ci`；
- Wails：与 `go.mod`、`apps/desktop/wails.json` 和 ADR-0003 一致。

CI 不配置真实模型密钥，不访问真实模型 API，不读取用户项目或系统凭据库。

双平台 Runner、产物契约、短期 artifact 与失败分类见 [Wails 双平台构建矩阵](wails-build-matrix.md)。

## 本地验证顺序

### 1. 策略门禁

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
node --test scripts/ci/wails-build-artifact.test.mjs
```

### 2. 正式桌面前端

```bash
cd apps/desktop/frontend
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../../..
```

桌面前端必须先 build，根 Go module 的 Wails 入口才能嵌入 `frontend/dist`。

### 3. Go

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
```

不得使用 `go test ./...`，因为根 module 下的前端依赖目录可能被递归 pattern 遍历。

### 4. 已验收浏览器原型

```bash
cd prototype/web
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../..
```

原型只作为 UX 参考，不能导入正式产品模块；保持可构建用于防止设计参考失真。

## Foundation CI Job

| Job | 职责 | 不负责 |
| --- | --- | --- |
| Ubuntu tests | 策略门禁、产物契约单元测试、正式前端、Go race/vet、原型 | Wails 原生安装与签名 |
| macOS production build | 固定 arm64 Runner 上构建、结构/plist 校验、manifest、短期 tar artifact | Developer ID、正式签名、公证、安装和升级 |
| Windows production build | 固定 x64 Runner 上构建、PE 校验、manifest、Windows Go tests、短期 zip artifact | Authenticode、安装器和升级 |

签名、公证、安装器和升级属于 R8；旧项目迁移 fixture 与平台矩阵的进一步硬化由后续 R0 独立实施计划定义。

## 隐私与 fixture 规则

- 不提交 `.zjproject`、SQLite 数据库、WAL/SHM、API Key、私钥或签名证书；
- fixture 必须是确定性合成内容，不得复制真实作者稿件或受版权保护小说；
- Provider 测试使用 fake/Recorded Provider 或注入的 HTTP transport；
- 测试输出、GitHub artifact 和失败日志不得包含用户正文、Prompt、凭据、本地数据库、SQL dump、调试查询或失败现场数据；生产二进制正常包含的版本化迁移资源不属于额外泄露工件；
- tracked artifact policy 是最小自动门禁，不替代提交者对 fixture 来源和版权的审查。

## 失败分类

- 策略门禁失败：先移除敏感工件或真实凭据，不能添加忽略例外绕过；
- 前端测试失败：定位到具体组件或状态用例；
- typecheck/build 失败：修复 TypeScript 或资源构建后再运行 Go；
- Go race 失败：按并发缺陷处理，不得只关闭 `-race`；
- Go vet 失败：修复静态问题，不得从 package pattern 排除；
- Wails build 失败：记录平台、Go/Wails/Node 版本和完整非敏感日志，交由双平台构建计划处理。

## 完成判定

普通提交的自动化基线通过，需要：

1. 策略测试和仓库扫描通过；
2. 正式前端测试、typecheck 和 build 通过；
3. Go race tests 和 vet 通过；
4. 原型测试、typecheck 和 build 通过；
5. GitHub Foundation CI 的相关 Job 通过。
