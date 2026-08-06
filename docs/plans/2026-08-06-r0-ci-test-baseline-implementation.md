# R0 CI 与测试基线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变产品行为的前提下，把当前已可运行的 Foundation CI 固定为可审计、可重复且能阻止敏感项目工件进入版本库的 R0 自动化测试基线。

**Architecture:** 保留现有 `.github/workflows/ci.yml` 的 Ubuntu 测试与 macOS/Windows Wails 构建 Job，不重建第二套流水线。新增一个无第三方依赖的 Node.js 策略检查器，对 Git 已跟踪路径和文本内容执行敏感工件门禁；CI 继续用明确的 Go package pattern、fake/Recorded Provider 和两个独立前端门禁，并通过测试基线文档固定本地与 CI 的同构命令。

**Tech Stack:** GitHub Actions、Node.js 22 `node:test`、Go 1.25.12、Wails v2.13.0、React/TypeScript/Vite/Vitest、SQLite/Goose。

---

- 状态：`in_progress`
- 路线图阶段：[R0：固定基线与 CI](../roadmap/development-roadmap.md#5-r0固定基线与-ci)
- 上游依据：[产品需求 7.5](../product/requirements.md#75-质量与可验证性)、[系统设计 14](../architecture/system-design.md#14-测试与质量门槛)、[ADR-0003](../architecture/0003-wails-project-layout.md)
- 后续独立计划：`R0 双平台 Wails 构建矩阵`、`R0 旧项目迁移与恢复 fixture`

## 1. 当前基线

计划编写时，仓库已经存在：

- `.github/workflows/ci.yml`
  - Ubuntu：桌面前端测试/类型检查/构建、Go race tests、Go vet、浏览器原型测试/类型检查/构建；
  - macOS：Wails production build；
  - Windows：Wails production build 与 Go tests；
- `apps/desktop/frontend/package-lock.json` 与 `prototype/web/package-lock.json`；
- 明确禁止 `go test ./...` 的 ADR 和 README 规则；
- Provider 测试使用注入的 HTTP transport、fake CredentialStore 和 fake Provider，不依赖真实 API Key；
- SQLite 迁移、完整性、revision、导入、抽取、快照和分支测试。

2026-08-06 本地基线证据：

- 桌面前端：17 个测试文件、145 项测试通过；
- 浏览器原型：13 个测试文件、43 项测试通过；
- 两套 TypeScript typecheck 和 production frontend build 通过；
- `go test -count=1 -race ./apps/desktop ./internal/...` 通过；
- `go vet ./apps/desktop ./internal/...` 通过。

本计划不把上述已存在能力重新实现。它只补充：

1. Git 已跟踪敏感项目工件和真实凭据特征的自动门禁；
2. Foundation CI 对策略门禁、随机测试顺序和测试超时的显式执行；
3. 本地命令、CI Job、失败分类和隐私边界的权威测试基线文档。

## 2. 明确不包含

- 不实现或修改产品功能；
- 不修改数据库 Schema；
- 不创建 500 章 Harness；
- 不新增 Playwright E2E 门禁；
- 不拆分或重写现有 macOS/Windows Wails build Job；
- 不处理签名、公证、安装器、升级或 Release artifact；
- 不创建旧项目迁移 fixture；
- 不引入第三方 secret scanner、SaaS 扫描器或需要云端账户的 CI 服务。

### 2.1 实施期补充修复（Windows CI 既有失败）

计划编写时本地基线在 macOS 上全绿，但 GitHub Foundation CI 的 `Windows production build` Job 早就有 6 个 Windows 专属失败（`internal/application/workspace` 3 个、`internal/platform/projectfs` 3 个），均源自测试中未关闭的 SQLite 连接或文件句柄：Windows 不允许删除/重命名/读取被另一句柄占用的文件，而 macOS/Linux 允许。这些失败不在本计划声明的文件范围内，但 Task 5 Step 6 要求 Ubuntu/macOS/Windows 全绿才能标记完成，因此经评审确认后在同一 worktree 内补修：

- `internal/application/workspace/branch_test.go`：3 个分支合并测试在 `t.TempDir()` 之后注册 `t.Cleanup(func(){ _ = service.Close() })`，确保活跃 SQLite store 在 TempDir RemoveAll 之前关闭（LIFO 顺序）；
- `internal/platform/projectfs/import_source_test.go`：`TestCommitImportArchiveNoReplaceRejectsExistingTarget` 与 `TestCommitImportArchiveUsesTheStillOpenTemporaryObject` 在读取断言前显式关闭临时文件与 imports 目录句柄；`TestImportArchiveOperationsStayBoundToVerifiedImportsDirectory` 在 Windows 上跳过，因为“句柄打开期间重命名目录”在 Windows 不被支持，该路径替换绑定语义在 macOS/Linux 仍被验证，Windows 文件锁硬化归 R8。

以上均为测试卫生改动，未改变生产行为或数据库 Schema；提交为 `140ca74`。

## 3. 文件结构

| 文件 | 职责 |
| --- | --- |
| `scripts/ci/tracked-artifact-policy.mjs` | 读取 `git ls-files`，拒绝项目数据库、项目包、私钥/签名文件和具有真实格式的模型密钥。 |
| `scripts/ci/tracked-artifact-policy.test.mjs` | 使用 `node:test` 对路径、内容和允许样例进行确定性测试。 |
| `.github/workflows/ci.yml` | 在安装依赖后执行策略测试和仓库扫描，并强化 Go race test 参数。 |
| `docs/testing/automated-test-baseline.md` | 记录本地/CI 同构命令、Job 职责、隐私约束和失败分类。 |
| `fixtures/README.md` | 说明 fixture 内容规则和本地策略检查命令。 |
| `README.md` | 链接自动化测试基线，保留快速验证入口。 |
| `docs/README.md` | 将自动化测试基线加入文档阅读入口。 |
| `docs/testing/foundation-acceptance.md` | 明确自动化基线与双平台手工验收的分工。 |

### Task 0: 启动已批准计划

**Files:**

- Modify: `docs/plans/2026-08-06-r0-ci-test-baseline-implementation.md`

- [ ] **Step 1: 创建隔离工作区**

调用 `superpowers:using-git-worktrees`，从最新 `main` 创建 `codex/r0-ci-test-baseline` 隔离 worktree。不要把当前未跟踪的 `apps/desktop/build/darwin/Info.dev.plist` 带入新分支。

- [ ] **Step 2: 标记计划开始执行**

将本文件顶部：

```markdown
- 状态：`draft`
```

改为：

```markdown
- 状态：`in_progress`
```

- [ ] **Step 3: 提交计划状态**

```bash
git add docs/plans/2026-08-06-r0-ci-test-baseline-implementation.md
git commit -m "docs: start R0 CI test baseline plan"
```

### Task 1: 为已跟踪工件策略编写失败测试

**Files:**

- Create: `scripts/ci/tracked-artifact-policy.test.mjs`
- Required by this test and created in Task 2: `scripts/ci/tracked-artifact-policy.mjs`

- [ ] **Step 1: 创建策略单元测试**

创建 `scripts/ci/tracked-artifact-policy.test.mjs`：

```js
import assert from "node:assert/strict";
import test from "node:test";

import {
  contentViolation,
  pathViolation,
} from "./tracked-artifact-policy.mjs";

test("rejects tracked project packages, databases, and private signing material", () => {
  assert.match(
    pathViolation("fixtures/demo.zjproject/project.db"),
    /project package/i,
  );
  assert.match(pathViolation("tmp/project.db"), /database/i);
  assert.match(pathViolation("tmp/project.db-wal"), /database/i);
  assert.match(pathViolation("signing/release.p12"), /private signing/i);
  assert.match(pathViolation("signing/notarization.key"), /private signing/i);
});

test("allows source, migration, documentation, and generated fixture specifications", () => {
  assert.equal(pathViolation("internal/persistence/sqlite/migrations/00014.sql"), "");
  assert.equal(pathViolation("fixtures/longform/novel_500ch/spec.json"), "");
  assert.equal(pathViolation("docs/testing/foundation-acceptance.md"), "");
});

test("rejects private keys and live-shaped provider credentials", () => {
  const privateKeyHeader = ["-----BEGIN", "PRIVATE KEY-----"].join(" ");
  const openAIKey = ["sk", "proj", "A".repeat(32)].join("-");
  const anthropicKey = ["sk", "ant", "B".repeat(32)].join("-");

  assert.match(contentViolation(privateKeyHeader), /private key/i);
  assert.match(contentViolation(openAIKey), /OpenAI/i);
  assert.match(contentViolation(anthropicKey), /Anthropic/i);
});

test("allows documentation labels and short fake secrets", () => {
  assert.equal(contentViolation("OPENAI_API_KEY"), "");
  assert.equal(contentViolation("sk-test"), "");
  assert.equal(contentViolation("fake-secret"), "");
});
```

- [ ] **Step 2: 运行测试并确认因实现文件不存在而失败**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
```

Expected: FAIL，错误包含 `ERR_MODULE_NOT_FOUND` 和 `tracked-artifact-policy.mjs`。

### Task 2: 实现无第三方依赖的策略检查器

**Files:**

- Create: `scripts/ci/tracked-artifact-policy.mjs`
- Test: `scripts/ci/tracked-artifact-policy.test.mjs`

- [ ] **Step 1: 创建策略实现**

创建 `scripts/ci/tracked-artifact-policy.mjs`：

```js
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { basename, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const forbiddenDatabaseSuffixes = [
  ".db",
  ".db-shm",
  ".db-wal",
  ".sqlite",
  ".sqlite3",
];

const forbiddenSigningSuffixes = [
  ".key",
  ".p12",
  ".pem",
  ".pfx",
];

const scannedTextExtensions = new Set([
  ".go",
  ".js",
  ".json",
  ".md",
  ".mjs",
  ".sql",
  ".toml",
  ".ts",
  ".tsx",
  ".txt",
  ".yaml",
  ".yml",
]);

export function pathViolation(filePath) {
  const normalized = filePath.replaceAll("\\", "/");
  const lower = normalized.toLowerCase();
  const segments = lower.split("/");

  if (segments.some((segment) => segment.endsWith(".zjproject"))) {
    return "tracked project package";
  }
  if (forbiddenDatabaseSuffixes.some((suffix) => lower.endsWith(suffix))) {
    return "tracked project database";
  }
  if (forbiddenSigningSuffixes.some((suffix) => lower.endsWith(suffix))) {
    return "tracked private signing material";
  }
  return "";
}

export function contentViolation(content) {
  if (/-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/.test(content)) {
    return "tracked private key";
  }
  if (/\bsk-proj-[A-Za-z0-9_-]{20,}\b/.test(content)) {
    return "tracked OpenAI-shaped credential";
  }
  if (/\bsk-ant-[A-Za-z0-9_-]{20,}\b/.test(content)) {
    return "tracked Anthropic-shaped credential";
  }
  return "";
}

function shouldScanText(filePath) {
  return (
    scannedTextExtensions.has(extname(filePath).toLowerCase()) ||
    basename(filePath) === ".env.example"
  );
}

export function inspectRepository(cwd) {
  const tracked = execFileSync("git", ["ls-files", "-z"], {
    cwd,
    encoding: "utf8",
  })
    .split("\0")
    .filter(Boolean);

  const violations = [];
  for (const filePath of tracked) {
    const pathReason = pathViolation(filePath);
    if (pathReason) {
      violations.push(`${filePath}: ${pathReason}`);
    }

    if (!shouldScanText(filePath)) {
      continue;
    }
    const contentReason = contentViolation(
      readFileSync(resolve(cwd, filePath), "utf8"),
    );
    if (contentReason) {
      violations.push(`${filePath}: ${contentReason}`);
    }
  }
  return { trackedCount: tracked.length, violations };
}

function run() {
  const { trackedCount, violations } = inspectRepository(process.cwd());
  if (violations.length > 0) {
    console.error("Tracked artifact policy failed:");
    for (const violation of violations) {
      console.error(`- ${violation}`);
    }
    process.exitCode = 1;
    return;
  }
  console.log(`Tracked artifact policy passed (${trackedCount} files checked).`);
}

const currentFile = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === currentFile) {
  run();
}
```

- [ ] **Step 2: 运行策略单元测试**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
```

Expected: PASS，4 项测试全部通过。

- [ ] **Step 3: 对当前 Git 已跟踪文件执行真实扫描**

Run:

```bash
node scripts/ci/tracked-artifact-policy.mjs
```

Expected: PASS，输出 `Tracked artifact policy passed (`，且不报告文件路径。

- [ ] **Step 4: 确认未跟踪的本地构建文件不被策略脚本读取**

Run:

```bash
git status --short
```

Expected: `apps/desktop/build/darwin/Info.dev.plist` 可以继续显示为未跟踪文件，但策略扫描仍通过；脚本只检查 `git ls-files` 的结果。

- [ ] **Step 5: 提交策略检查器和测试**

```bash
git add scripts/ci/tracked-artifact-policy.mjs scripts/ci/tracked-artifact-policy.test.mjs
git commit -m "test(ci): guard tracked project artifacts"
```

### Task 3: 将策略门禁接入 Foundation CI

**Files:**

- Modify: `.github/workflows/ci.yml`
- Test: `scripts/ci/tracked-artifact-policy.test.mjs`

- [ ] **Step 1: 固定 CI 环境标志**

在 `.github/workflows/ci.yml` 的顶层 `env` 中加入 `CI: "true"`，形成：

```yaml
env:
  CI: "true"
  WAILS_VERSION: v2.13.0
  NODE_VERSION: "22"
```

- [ ] **Step 2: 在 Ubuntu test Job 中加入策略门禁**

在 `Set up Node.js` 之后、安装任一 npm 依赖之前加入：

```yaml
      - name: Test tracked artifact policy
        run: node --test scripts/ci/tracked-artifact-policy.test.mjs

      - name: Enforce tracked artifact policy
        run: node scripts/ci/tracked-artifact-policy.mjs
```

这样策略测试只依赖 Node 22 和 Git checkout，不依赖 npm 包，也不会把安装后的 `node_modules` 纳入扫描。

- [ ] **Step 3: 为 Go race tests 增加顺序扰动和硬超时**

将：

```yaml
      - name: Run Go race tests
        run: go test -count=1 -race ./apps/desktop ./internal/...
```

替换为：

```yaml
      - name: Run Go race tests
        run: go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
```

不得改成 `go test ./...`。桌面前端 build 必须继续位于 Go test 之前，以满足 `go:embed`。

- [ ] **Step 4: 本地执行新增 CI 门禁**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
```

Expected: 两条命令均 PASS。

- [ ] **Step 5: 本地执行与新 CI 参数一致的 Go 测试**

Run:

```bash
cd apps/desktop/frontend
npm ci
npm run build
cd ../../..
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
```

Expected: 桌面前端 build 通过；所有列出的 Go package 均为 `ok` 或 `[no test files]`，进程退出码为 0。

- [ ] **Step 6: 检查 workflow 没有扩大权限或引用真实凭据**

Run:

```bash
rg -n "permissions:|contents: read|secrets\\.|OPENAI|ANTHROPIC|go test ./\\.\\.\\." .github/workflows/ci.yml
```

Expected:

- 包含 `permissions:` 与 `contents: read`；
- 不包含 `secrets.`、`OPENAI`、`ANTHROPIC`；
- 不包含 `go test ./...`。

- [ ] **Step 7: 提交 CI 改动**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: harden foundation test gate"
```

### Task 4: 建立自动化测试基线文档

**Files:**

- Create: `docs/testing/automated-test-baseline.md`
- Modify: `fixtures/README.md`
- Modify: `README.md`
- Modify: `docs/README.md`
- Modify: `docs/testing/foundation-acceptance.md`

- [ ] **Step 1: 创建自动化测试基线**

创建 `docs/testing/automated-test-baseline.md`：

```markdown
# 自动化测试基线

本基线定义普通提交必须通过的本地和 CI 自动验证。它不替代 Windows/macOS 人工验收、正式签名、公证、安装或升级验证。

## 环境

- Go：以根目录 `go.mod` 和 `toolchain` 为准；
- Node.js：22；
- npm：使用两个已提交的 `package-lock.json` 和 `npm ci`；
- Wails：与 `go.mod`、`apps/desktop/wails.json` 和 ADR-0003 一致。

CI 不配置真实模型密钥，不访问真实模型 API，不读取用户项目或系统凭据库。

## 本地验证顺序

### 1. 策略门禁

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
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
| Ubuntu tests | 策略门禁、正式前端、Go race/vet、原型 | Wails 原生安装与签名 |
| macOS production build | Wails 原生构建可用性 | Developer ID 签名、公证、安装和升级 |
| Windows production build | Wails 原生构建与 Windows Go tests | Authenticode 签名、安装器和升级 |

签名、公证、安装器和升级属于 R8；旧项目迁移 fixture 与平台矩阵的进一步硬化由后续 R0 独立实施计划定义。

## 隐私与 fixture 规则

- 不提交 `.zjproject`、SQLite 数据库、WAL/SHM、API Key、私钥或签名证书；
- fixture 必须是确定性合成内容，不得复制真实作者稿件或受版权保护小说；
- Provider 测试使用 fake/Recorded Provider 或注入的 HTTP transport；
- 测试输出、GitHub artifact 和失败日志不得包含正文、Prompt、凭据、SQL 或本地数据库；
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
```

- [ ] **Step 2: 补充 fixture 目录规则**

将 `fixtures/README.md` 扩展为：

```markdown
# Fixtures

保存无敏感信息、无版权风险的合成测试规格、导入文本和模型响应样本。

## 规则

- 不保存真实作者稿件、真实项目数据库或完整第三方小说；
- 不保存 `.zjproject`、`.db`、`.db-wal`、`.db-shm`、API Key、私钥或签名文件；
- 长篇测试优先保存固定种子、生成规格和黄金查询，在测试临时目录生成正文；
- Recorded Provider 响应必须脱敏，不包含请求凭据、用户正文或 Provider 私有身份信息；
- 每个 fixture 在对应测试或目录说明中写明用途和合成方式。

本地检查：

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
```
```

- [ ] **Step 3: 在根 README 链接自动化基线**

在根 `README.md` 的“可复制的验证与运行命令”标题后加入：

```markdown
完整命令顺序、CI Job 分工和 fixture 隐私规则见[自动化测试基线](docs/testing/automated-test-baseline.md)。
```

- [ ] **Step 4: 在文档索引加入自动化测试基线**

将 `docs/README.md` 阅读顺序的最后一项改为：

```markdown
8. [自动化测试基线](testing/automated-test-baseline.md)、[项目恢复](operations/project-recovery.md)与[基础手工验收](testing/foundation-acceptance.md)：自动门禁、运行保障和双平台人工验收。
```

- [ ] **Step 5: 明确自动化与人工验收的分工**

在 `docs/testing/foundation-acceptance.md` 第一段后加入：

```markdown
提交级测试命令和 CI Job 由[自动化测试基线](automated-test-baseline.md)定义。本清单只覆盖必须在真实 Windows/macOS 应用环境观察的安装、WebView、原生对话框、文件锁、恢复和跨平台行为。
```

- [ ] **Step 6: 检查新增 Markdown 链接**

Run:

```bash
node -e 'const fs=require("fs"),path=require("path"); const files=["README.md","docs/README.md","docs/testing/automated-test-baseline.md","docs/testing/foundation-acceptance.md","fixtures/README.md"]; let bad=[]; for(const f of files){const s=fs.readFileSync(f,"utf8"); for(const m of s.matchAll(/\[[^\]]+\]\(([^)]+)\)/g)){const t=m[1].split("#")[0]; if(!t||/^[a-z]+:/i.test(t)) continue; if(!fs.existsSync(path.resolve(path.dirname(f),t))) bad.push(`${f}: ${m[1]}`)}} if(bad.length){console.error(bad.join("\n")); process.exit(1)} console.log("all automated baseline links resolve")'
```

Expected: PASS，输出 `all automated baseline links resolve`。

- [ ] **Step 7: 提交测试基线文档**

```bash
git add README.md docs/README.md docs/testing/automated-test-baseline.md docs/testing/foundation-acceptance.md fixtures/README.md
git commit -m "docs: define automated test baseline"
```

### Task 5: 执行 R0 CI 与测试基线验收

**Files:**

- Verify: `.github/workflows/ci.yml`
- Verify: `scripts/ci/tracked-artifact-policy.mjs`
- Verify: `scripts/ci/tracked-artifact-policy.test.mjs`
- Verify: `docs/testing/automated-test-baseline.md`

- [ ] **Step 1: 运行策略门禁**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
```

Expected: 策略测试和仓库扫描均 PASS。

- [ ] **Step 2: 运行桌面前端门禁**

Run:

```bash
cd apps/desktop/frontend
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../../..
```

Expected: Vitest、TypeScript 和 Vite build 全部退出码为 0。

- [ ] **Step 3: 运行 Go 门禁**

Run:

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
```

Expected: 所有 Go package 通过 race tests 和 vet。

- [ ] **Step 4: 运行原型门禁**

Run:

```bash
cd prototype/web
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../..
```

Expected: 原型 Vitest、TypeScript 和 Vite build 全部退出码为 0。

- [ ] **Step 5: 检查差异和敏感文件范围**

Run:

```bash
git diff --check
git status --short
git diff --name-only
```

Expected:

- `git diff --check` 无输出；
- 变更文件只包含本计划列出的 CI、脚本、文档，以及 §2.1 记录的 Windows 测试卫生修复；
- `apps/desktop/build/darwin/Info.dev.plist` 仍保持未跟踪且未被加入提交；
- 不出现 `.db`、`.zjproject`、密钥、证书或真实正文。

- [ ] **Step 6: 记录 GitHub Foundation CI 结果**

按仓库正常推送/合并流程触发 `.github/workflows/ci.yml`。只有 `Ubuntu tests`、`macOS production build` 和 `Windows production build` 全部通过，才把本计划状态更新为 `completed`；失败时保留失败 Job 和非敏感日志，不得跳过失败 Job 宣告完成。

- [ ] **Step 7: 提交计划完成状态**

执行开始时先将本文件顶部状态从 `draft` 更新为 `in_progress`；只有 Step 1–6 全部取得证据后，才将状态更新为 `completed` 并提交：

```bash
git add docs/plans/2026-08-06-r0-ci-test-baseline-implementation.md
git commit -m "docs: complete R0 CI test baseline plan"
```

## 4. 计划完成后

本计划只完成 R0 的第一个独立能力。接下来按总体路线图分别编写和评审：

1. `R0 双平台 Wails 构建矩阵`：原生 Runner 版本、构建 artifact、平台专项 smoke 和失败诊断；
2. `R0 旧项目迁移与恢复 fixture`：旧 Schema 项目、升级前备份、失败恢复和跨版本兼容。

两份计划完成并满足 R0 阶段退出标准后，才进入 R1 长任务与项目可靠性。
