# R0 双平台 Wails 构建矩阵 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 macOS/Windows Wails production build 强化为固定原生 Runner、结构化校验、可下载检查且能明确定位失败阶段的 R0 双平台构建矩阵。

**Architecture:** 保留 `.github/workflows/ci.yml` 中独立的 macOS 与 Windows Job，不合并成隐藏平台差异的单一 matrix Job。新增无第三方依赖的 Node.js 产物检查器，统一验证 Wails 输出结构、平台二进制格式和脱敏构建元数据；随后使用平台原生工具把 macOS `.app` 打成保留权限的 `tar.gz`、把 Windows `.exe` 打成 zip，并作为短期 GitHub Actions artifact 上传。

**Tech Stack:** GitHub Actions、`actions/upload-artifact@v7`、Node.js 22 `node:test`、Go 1.25.12、Wails v2.13.0、macOS 15 arm64 Runner、Windows Server 2025 x64 Runner、SQLite/React 现有生产构建。

---

- 状态：`in_progress`
- 路线图阶段：[R0：固定基线与 CI](../roadmap/development-roadmap.md#5-r0固定基线与-ci)
- 上游依据：[产品需求 7.1/7.5](../product/requirements.md#71-平台与性能)、[系统设计 14](../architecture/system-design.md#14-测试与质量门槛)、[ADR-0002](../architecture/0002-go-first-wails-stack.md)、[ADR-0003](../architecture/0003-wails-project-layout.md)
- 前置计划：[R0 CI 与测试基线](2026-08-06-r0-ci-test-baseline-implementation.md)（`completed`）
- 后续独立计划：`R0 旧项目迁移与恢复 fixture`

## 1. 当前基线与缺口

当前 `.github/workflows/ci.yml` 已经具备：

- `Ubuntu tests`：敏感工件策略、桌面前端测试/typecheck/build、Go race/vet、原型测试/typecheck/build；
- `macOS production build`：`macos-latest` 上安装 Wails v2.13.0、执行 `wails doctor` 与 `wails build -clean -m`；
- `Windows production build`：`windows-latest` 上执行相同 Wails build，并运行 Windows Go tests；
- main 分支 Foundation CI 运行 `31069519793` 的三个 Job 均通过。

当前缺口：

1. `*-latest` 会随 GitHub Runner 映射变化，失败时难以确认实际 OS/架构基线；
2. CI 只以 `wails build` 退出码判断成功，没有验证 `.app`/`.exe` 的路径、入口和二进制格式；
3. build Job 结束后不保留可下载产物，无法复核特定提交的生产构建；
4. 没有随产物保存 commit、Runner、架构、Go/Node/Wails 版本与入口哈希；
5. macOS `.app` 若直接交给 `upload-artifact`，下载后不能依赖 GitHub artifact 保留原始 Unix 权限，因此需要先用 tar 封装；
6. 构建、产物校验、平台打包和上传还不是四个独立失败阶段。

## 2. 设计选择

### 2.1 保留独立 Job

不把 macOS/Windows 合并为一个 `strategy.matrix` Job。两个 Job 的依赖、诊断命令、Wails 输出、打包格式和专项 smoke 不同；独立 Job 名能直接定位平台，并允许后续在 R8 分别加入签名、公证或安装器。

### 2.2 固定当前稳定原生 Runner

- macOS：`macos-15`，当前 GitHub-hosted 标准 Runner 为 Apple Silicon/arm64；
- Windows：`windows-2025`，当前 GitHub-hosted 标准 Runner 为 x64。

本计划只建立各一个原生基线。macOS Intel/universal binary、Windows ARM64 和支持版本扩展属于 R8 发布矩阵，不在 R0 提前承诺。

### 2.3 产物契约

每个平台在 `apps/desktop/build/bin/build-manifest.json` 写入：

- schema 与产品名；
- 平台、产物类型和相对路径；
- 入口相对路径、字节数和 SHA-256；
- macOS bundle 文件数/总字节数或 Windows exe 文件数/总字节数；
- source commit、GitHub run ID/attempt；
- Runner label、OS、架构；
- Go、Node 和 Wails 实际版本。

Manifest 只从显式白名单字段构造，不遍历环境变量，不记录工作区绝对路径、用户目录、正文、Prompt、API Key、SQL、数据库或签名材料。

### 2.4 CI artifact 与 Release artifact 分层

R0 只上传 7 天保留的调试/检查 artifact：

- macOS：`NovelStudio-macos-arm64.tar.gz` + `build-manifest.json`；
- Windows：`NovelStudio-windows-x64.zip` + `build-manifest.json`。

它们是未公证/未正式签名的 CI production build，不是安装包或商业发布资产。Developer ID、Gatekeeper、公证、Authenticode、安装器、升级与 Release 发布属于 R8。

## 3. 明确不包含

- 不修改产品功能、Wails binding、React 页面或数据库 Schema；
- 不升级 Go、Wails、React、Vite 或 SQLite；
- 不引入 Rust、CGO/FFI、Docker 或第三方打包框架；
- 不创建 DMG、PKG、MSI、MSIX、NSIS 安装器；
- 不配置 Developer ID、Apple 公证、Authenticode 或任何签名密钥；
- 不把 CI artifact 发布到 GitHub Release；
- 不在 CI 启动 GUI、操作 WebView 或替代双平台人工验收；
- 不扩展到 macOS Intel/universal、Windows ARM64 或多 OS 版本矩阵；
- 不处理旧项目迁移 fixture；该能力由下一份 R0 独立计划完成。

## 4. 文件结构

| 文件 | 职责 |
| --- | --- |
| `scripts/ci/wails-build-artifact.mjs` | 跨平台验证 Wails 输出，计算入口哈希并写入白名单 build manifest。 |
| `scripts/ci/wails-build-artifact.test.mjs` | 使用临时合成 `.app`/`.exe` 验证成功、缺失、错误格式和脱敏字段。 |
| `.github/workflows/ci.yml` | 固定 Runner，执行检查/平台打包并上传短期 artifact。 |
| `.gitignore` | 忽略本地和 CI 生成的 `apps/desktop/build/artifacts/`。 |
| `docs/testing/wails-build-matrix.md` | 记录 Runner、产物契约、下载检查、失败分类和 R8 边界。 |
| `docs/testing/automated-test-baseline.md` | 将产物检查器和双平台 artifact 纳入自动化基线。 |
| `docs/README.md` | 链接双平台构建矩阵文档。 |
| `docs/roadmap/development-roadmap.md` | 反映第一份 R0 计划已完成及本计划已建立。 |

## 5. 数据、兼容与恢复

- 本计划不修改任何应用数据库或项目包，不需要 Goose migration；
- 所有测试工件生成在 `node:test` 临时目录或 CI 的 `apps/desktop/build/bin|artifacts`，均已被构建忽略规则覆盖；
- 失败时保留 Git 工作树和已生成 build 目录供当前 Job 诊断，但不得上传数据库、日志目录或整个工作区；
- artifact 上传路径采用显式白名单，只包含平台压缩包和 manifest；
- CI 取消或上传失败不影响源码、项目数据或 canonical 内容；重新运行 workflow 会进入新的 `run_attempt` 并使用不同的不可变 artifact 名称；
- 回滚方式是还原 `.github/workflows/ci.yml` 的产物步骤并删除两个 Node 脚本/测试和对应文档，不涉及用户数据恢复。

### Task 0: 启动已批准计划

**Files:**

- Modify: `docs/plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 创建隔离工作区**

调用 `superpowers:using-git-worktrees`，从最新 `main` 创建 `codex/r0-dual-platform-wails-build-matrix` 隔离 worktree。确认未跟踪的 `apps/desktop/build/darwin/Info.dev.plist` 没有被复制、暂存或提交。

- [ ] **Step 2: 标记计划开始执行**

将本文件顶部：

```markdown
- 状态：`draft`
```

改为：

```markdown
- 状态：`in_progress`
```

同时将 `docs/roadmap/development-roadmap.md` 中本计划的状态从 `draft` 改为 `in_progress`。

- [ ] **Step 3: 提交计划状态**

```bash
git add docs/plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: start R0 Wails build matrix plan"
```

### Task 1: 为 Wails 产物契约编写失败测试

**Files:**

- Create: `scripts/ci/wails-build-artifact.test.mjs`
- Required by this test and created in Task 2: `scripts/ci/wails-build-artifact.mjs`

- [ ] **Step 1: 创建产物契约测试**

创建 `scripts/ci/wails-build-artifact.test.mjs`：

```js
import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  inspectWailsBuild,
  writeBuildManifest,
} from "./wails-build-artifact.mjs";

const roots = [];

test.afterEach(() => {
  for (const root of roots.splice(0)) {
    rmSync(root, { force: true, recursive: true });
  }
});

function temporaryBuildDir() {
  const root = mkdtempSync(join(tmpdir(), "novel-studio-wails-"));
  roots.push(root);
  return root;
}

function metadata(overrides = {}) {
  return {
    commit: "0123456789abcdef",
    runId: "101",
    runAttempt: "2",
    runnerLabel: "fixture-runner",
    runnerOS: "fixture-os",
    runnerArch: "fixture-arch",
    goVersion: "go version go1.25.12 fixture",
    nodeVersion: "v22.22.0",
    wailsVersion: "v2.13.0",
    ...overrides,
  };
}

test("inspects a macOS app bundle without persisting absolute paths", () => {
  const buildDir = temporaryBuildDir();
  const app = join(buildDir, "NovelStudio.app", "Contents");
  mkdirSync(join(app, "MacOS"), { recursive: true });
  writeFileSync(join(app, "Info.plist"), "<plist><dict/></plist>");
  writeFileSync(
    join(app, "MacOS", "NovelStudio"),
    Buffer.from([0xcf, 0xfa, 0xed, 0xfe, 0x00]),
  );

  const manifest = inspectWailsBuild({
    platform: "darwin",
    buildDir,
    metadata: metadata({
      runnerLabel: "macos-15",
      runnerOS: "macOS",
      runnerArch: "ARM64",
    }),
  });

  assert.equal(manifest.platform, "darwin");
  assert.equal(manifest.artifact.kind, "app_bundle");
  assert.equal(manifest.artifact.path, "NovelStudio.app");
  assert.equal(
    manifest.artifact.entrypoint,
    "NovelStudio.app/Contents/MacOS/NovelStudio",
  );
  assert.equal(manifest.runner.label, "macos-15");
  assert.match(manifest.artifact.entrypoint_sha256, /^[a-f0-9]{64}$/);
  assert.doesNotMatch(JSON.stringify(manifest), new RegExp(buildDir));
});

test("inspects a Windows PE executable and records only allowlisted metadata", () => {
  const buildDir = temporaryBuildDir();
  writeFileSync(
    join(buildDir, "NovelStudio.exe"),
    Buffer.from([0x4d, 0x5a, 0x00, 0x00]),
  );

  const manifest = inspectWailsBuild({
    platform: "windows",
    buildDir,
    metadata: metadata({
      runnerLabel: "windows-2025",
      runnerOS: "Windows",
      runnerArch: "X64",
      ignoredSecret: "must-not-be-persisted",
    }),
  });

  assert.equal(manifest.artifact.kind, "pe_executable");
  assert.equal(manifest.artifact.path, "NovelStudio.exe");
  assert.equal(manifest.artifact.entrypoint, "NovelStudio.exe");
  assert.equal(manifest.runner.label, "windows-2025");
  assert.doesNotMatch(JSON.stringify(manifest), /must-not-be-persisted/);
});

test("rejects missing or wrong-format Wails output", () => {
  const missingDir = temporaryBuildDir();
  assert.throws(
    () =>
      inspectWailsBuild({
        platform: "windows",
        buildDir: missingDir,
        metadata: metadata(),
      }),
    /missing Windows executable/i,
  );

  const invalidDir = temporaryBuildDir();
  writeFileSync(join(invalidDir, "NovelStudio.exe"), "not-a-pe-file");
  assert.throws(
    () =>
      inspectWailsBuild({
        platform: "windows",
        buildDir: invalidDir,
        metadata: metadata(),
      }),
    /PE\/MZ/i,
  );
});

test("writes a stable JSON manifest next to the build output", () => {
  const buildDir = temporaryBuildDir();
  writeFileSync(
    join(buildDir, "NovelStudio.exe"),
    Buffer.from([0x4d, 0x5a, 0x00, 0x00]),
  );
  const output = join(buildDir, "build-manifest.json");

  const written = writeBuildManifest({
    platform: "windows",
    buildDir,
    output,
    metadata: metadata(),
  });
  const parsed = JSON.parse(readFileSync(output, "utf8"));

  assert.deepEqual(parsed, written);
  assert.equal(parsed.schema_version, 1);
  assert.equal(parsed.product, "NovelStudio");
  assert.equal(readFileSync(output, "utf8").endsWith("\n"), true);
});
```

- [ ] **Step 2: 运行测试并确认因实现文件不存在而失败**

Run:

```bash
node --test scripts/ci/wails-build-artifact.test.mjs
```

Expected: FAIL，错误包含 `ERR_MODULE_NOT_FOUND` 和 `wails-build-artifact.mjs`。

- [ ] **Step 3: 提交失败测试**

```bash
git add scripts/ci/wails-build-artifact.test.mjs
git commit -m "test(ci): define Wails build artifact contract"
```

### Task 2: 实现跨平台产物检查器

**Files:**

- Create: `scripts/ci/wails-build-artifact.mjs`
- Test: `scripts/ci/wails-build-artifact.test.mjs`

- [ ] **Step 1: 创建最小检查器**

创建 `scripts/ci/wails-build-artifact.mjs`：

```js
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const PRODUCT = "NovelStudio";
const MACH_O_MAGICS = new Set([
  "cafebabe",
  "bebafeca",
  "cefaedfe",
  "cffaedfe",
  "feedface",
  "feedfacf",
]);

const platformSpecs = {
  darwin: {
    artifactName: "NovelStudio.app",
    entrypoint: ["Contents", "MacOS", "NovelStudio"],
    requiredFiles: [
      ["Contents", "Info.plist"],
      ["Contents", "MacOS", "NovelStudio"],
    ],
    kind: "app_bundle",
  },
  windows: {
    artifactName: "NovelStudio.exe",
    entrypoint: [],
    requiredFiles: [[]],
    kind: "pe_executable",
  },
};

function slashPath(value) {
  return value.split(sep).join("/");
}

function requireString(value, name) {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`missing build metadata: ${name}`);
  }
  return value.trim();
}

function digest(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function directoryMetrics(root) {
  let fileCount = 0;
  let sizeBytes = 0;

  function visit(current) {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const fullPath = join(current, entry.name);
      if (entry.isDirectory()) {
        visit(fullPath);
        continue;
      }
      if (entry.isSymbolicLink()) {
        lstatSync(fullPath);
        continue;
      }
      if (entry.isFile()) {
        fileCount += 1;
        sizeBytes += statSync(fullPath).size;
      }
    }
  }

  visit(root);
  return { fileCount, sizeBytes };
}

function assertExecutableFormat(platform, entrypointPath) {
  const bytes = readFileSync(entrypointPath);
  if (platform === "windows") {
    if (bytes.length < 2 || bytes[0] !== 0x4d || bytes[1] !== 0x5a) {
      throw new Error("Windows build entrypoint is not a PE/MZ executable");
    }
    return;
  }

  const magic = bytes.subarray(0, 4).toString("hex");
  if (!MACH_O_MAGICS.has(magic)) {
    throw new Error("macOS build entrypoint is not a Mach-O executable");
  }
}

function allowlistedMetadata(metadata) {
  return {
    source: {
      commit: requireString(metadata.commit, "commit"),
    },
    ci: {
      run_id: requireString(metadata.runId, "runId"),
      run_attempt: requireString(metadata.runAttempt, "runAttempt"),
    },
    runner: {
      label: requireString(metadata.runnerLabel, "runnerLabel"),
      os: requireString(metadata.runnerOS, "runnerOS"),
      arch: requireString(metadata.runnerArch, "runnerArch"),
    },
    toolchain: {
      go: requireString(metadata.goVersion, "goVersion"),
      node: requireString(metadata.nodeVersion, "nodeVersion"),
      wails: requireString(metadata.wailsVersion, "wailsVersion"),
    },
  };
}

export function inspectWailsBuild({ platform, buildDir, metadata }) {
  const spec = platformSpecs[platform];
  if (!spec) {
    throw new Error(`unsupported Wails build platform: ${platform}`);
  }

  const absoluteBuildDir = resolve(buildDir);
  const artifactPath = join(absoluteBuildDir, spec.artifactName);
  let artifactStat;
  try {
    artifactStat = statSync(artifactPath);
  } catch {
    const label = platform === "darwin" ? "macOS app bundle" : "Windows executable";
    throw new Error(`missing ${label}: ${spec.artifactName}`);
  }

  if (platform === "darwin" && !artifactStat.isDirectory()) {
    throw new Error("macOS Wails artifact must be an app bundle directory");
  }
  if (platform === "windows" && !artifactStat.isFile()) {
    throw new Error("Windows Wails artifact must be an executable file");
  }

  for (const parts of spec.requiredFiles) {
    const requiredPath = join(artifactPath, ...parts);
    let requiredStat;
    try {
      requiredStat = statSync(requiredPath);
    } catch {
      throw new Error(
        `missing required Wails output: ${slashPath(relative(absoluteBuildDir, requiredPath))}`,
      );
    }
    if (!requiredStat.isFile() || requiredStat.size === 0) {
      throw new Error(
        `empty required Wails output: ${slashPath(relative(absoluteBuildDir, requiredPath))}`,
      );
    }
  }

  const entrypointPath =
    platform === "darwin"
      ? join(artifactPath, ...spec.entrypoint)
      : artifactPath;
  assertExecutableFormat(platform, entrypointPath);

  const metrics =
    platform === "darwin"
      ? directoryMetrics(artifactPath)
      : { fileCount: 1, sizeBytes: artifactStat.size };
  const allowlisted = allowlistedMetadata(metadata);

  return {
    schema_version: 1,
    product: PRODUCT,
    platform,
    artifact: {
      kind: spec.kind,
      path: slashPath(relative(absoluteBuildDir, artifactPath)),
      entrypoint: slashPath(relative(absoluteBuildDir, entrypointPath)),
      entrypoint_size_bytes: statSync(entrypointPath).size,
      entrypoint_sha256: digest(entrypointPath),
      file_count: metrics.fileCount,
      size_bytes: metrics.sizeBytes,
    },
    ...allowlisted,
  };
}

export function writeBuildManifest({
  platform,
  buildDir,
  output,
  metadata,
}) {
  const manifest = inspectWailsBuild({ platform, buildDir, metadata });
  mkdirSync(dirname(resolve(output)), { recursive: true });
  writeFileSync(resolve(output), `${JSON.stringify(manifest, null, 2)}\n`);
  return manifest;
}

function argument(name) {
  const index = process.argv.indexOf(`--${name}`);
  if (index === -1 || index + 1 >= process.argv.length) {
    throw new Error(`missing required argument: --${name}`);
  }
  return process.argv[index + 1];
}

function commandVersion(command, args) {
  return execFileSync(command, args, { encoding: "utf8" }).trim();
}

function metadataFromEnvironment() {
  return {
    commit: process.env.GITHUB_SHA || "local",
    runId: process.env.GITHUB_RUN_ID || "local",
    runAttempt: process.env.GITHUB_RUN_ATTEMPT || "1",
    runnerLabel: process.env.BUILD_RUNNER_LABEL || "local",
    runnerOS: process.env.RUNNER_OS || process.platform,
    runnerArch: process.env.RUNNER_ARCH || process.arch,
    goVersion: commandVersion("go", ["version"]),
    nodeVersion: process.version,
    wailsVersion: commandVersion("wails", ["version"]),
  };
}

function run() {
  const manifest = writeBuildManifest({
    platform: argument("platform"),
    buildDir: argument("build-dir"),
    output: argument("output"),
    metadata: metadataFromEnvironment(),
  });
  console.log(
    `Validated ${manifest.platform} ${manifest.artifact.path} ` +
      `(${manifest.artifact.size_bytes} bytes, ` +
      `${manifest.artifact.entrypoint_sha256})`,
  );
}

const currentFile = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === currentFile) {
  try {
    run();
  } catch (error) {
    console.error(`Wails build artifact validation failed: ${error.message}`);
    process.exitCode = 1;
  }
}
```

- [ ] **Step 2: 运行产物契约测试**

Run:

```bash
node --test scripts/ci/wails-build-artifact.test.mjs
```

Expected: PASS，4 项测试全部通过。

- [ ] **Step 3: 运行第一阶段既有策略门禁**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
```

Expected: 两条命令均 PASS；新脚本不包含真实格式凭据或私钥。

- [ ] **Step 4: 提交检查器**

```bash
git add scripts/ci/wails-build-artifact.mjs
git commit -m "ci: inspect Wails production artifacts"
```

### Task 3: 固定并强化 macOS 原生构建

**Files:**

- Modify: `.github/workflows/ci.yml`
- Modify: `.gitignore`
- Test: `scripts/ci/wails-build-artifact.test.mjs`

- [ ] **Step 1: 固定 macOS Runner**

在 `macos-build` Job 中将：

```yaml
    runs-on: macos-latest
```

替换为：

```yaml
    runs-on: macos-15
```

该 label 在本计划创建时对应稳定的 arm64 GitHub-hosted Runner。不要使用 beta/public-preview label。

- [ ] **Step 2: 在 Wails build 后验证 `.app` 与 plist**

在 `Build macOS application` 后加入：

```yaml
      - name: Validate macOS application bundle
        env:
          BUILD_RUNNER_LABEL: macos-15
        run: |
          node scripts/ci/wails-build-artifact.mjs \
            --platform darwin \
            --build-dir apps/desktop/build/bin \
            --output apps/desktop/build/bin/build-manifest.json
          plutil -lint apps/desktop/build/bin/NovelStudio.app/Contents/Info.plist
```

Node 检查器负责 `.app` 结构、Mach-O magic、入口 SHA-256 和 manifest；`plutil` 负责平台原生 plist 语法检查。

- [ ] **Step 3: 忽略平台检查包输出**

在 `.gitignore` 的 Wails build output 规则旁加入：

```gitignore
apps/desktop/build/artifacts/
```

Run:

```bash
git check-ignore -v apps/desktop/build/artifacts/NovelStudio-macos-arm64.tar.gz
```

Expected: 输出命中新增的 `apps/desktop/build/artifacts/` 规则。

- [ ] **Step 4: 用 tar 封装 `.app` 并复核包内容**

继续加入：

```yaml
      - name: Package macOS inspection artifact
        run: |
          mkdir -p apps/desktop/build/artifacts
          tar -czf apps/desktop/build/artifacts/NovelStudio-macos-arm64.tar.gz \
            -C apps/desktop/build/bin \
            NovelStudio.app \
            build-manifest.json
          tar -tzf apps/desktop/build/artifacts/NovelStudio-macos-arm64.tar.gz
```

必须先 tar 再上传，以保留 `.app/Contents/MacOS/NovelStudio` 的 Unix 可执行权限。此 tar 不是签名或公证发布包。

- [ ] **Step 5: 上传短期 macOS artifact**

继续加入：

```yaml
      - name: Upload macOS inspection artifact
        uses: actions/upload-artifact@v7
        with:
          name: novel-studio-macos-arm64-${{ github.sha }}-${{ github.run_attempt }}
          path: |
            apps/desktop/build/artifacts/NovelStudio-macos-arm64.tar.gz
            apps/desktop/build/bin/build-manifest.json
          if-no-files-found: error
          retention-days: 7
          compression-level: 0
```

不上传整个 `apps/desktop/build`、工作区、日志、数据库或测试临时目录。

- [ ] **Step 6: 静态检查 macOS Job**

Run:

```bash
rg -n -C 3 "macos-15|Validate macOS application bundle|plutil|NovelStudio-macos-arm64|upload-artifact@v7|retention-days" .github/workflows/ci.yml
```

Expected: 能看到固定 Runner、验证、打包、上传和 7 天保留；没有签名、公证或 secrets 引用。

- [ ] **Step 7: 提交 macOS 构建强化**

```bash
git add .github/workflows/ci.yml .gitignore
git commit -m "ci: retain inspectable macOS Wails build"
```

### Task 4: 固定并强化 Windows 原生构建

**Files:**

- Modify: `.github/workflows/ci.yml`
- Test: `scripts/ci/wails-build-artifact.test.mjs`

- [ ] **Step 1: 固定 Windows Runner**

在 `windows-build` Job 中将：

```yaml
    runs-on: windows-latest
```

替换为：

```yaml
    runs-on: windows-2025
```

- [ ] **Step 2: 在 Windows 也执行 Wails 诊断与产物验证**

在 `Install Wails CLI` 后加入：

```yaml
      - name: Diagnose Wails toolchain
        run: wails doctor
```

在 `Build Windows application` 后加入：

```yaml
      - name: Validate Windows executable
        env:
          BUILD_RUNNER_LABEL: windows-2025
        run: >
          node scripts/ci/wails-build-artifact.mjs
          --platform windows
          --build-dir apps/desktop/build/bin
          --output apps/desktop/build/bin/build-manifest.json
```

检查器必须在 `Run Go tests on Windows` 之前运行，使 Wails 打包问题与 Go 测试问题分别显示。

- [ ] **Step 3: 用 PowerShell 打包 `.exe` 与 manifest**

在 Windows Go tests 之后加入：

```yaml
      - name: Package Windows inspection artifact
        shell: pwsh
        run: |
          New-Item -ItemType Directory -Force -Path "apps/desktop/build/artifacts" | Out-Null
          Compress-Archive `
            -Path "apps/desktop/build/bin/NovelStudio.exe","apps/desktop/build/bin/build-manifest.json" `
            -DestinationPath "apps/desktop/build/artifacts/NovelStudio-windows-x64.zip"
          $archive = Get-Item "apps/desktop/build/artifacts/NovelStudio-windows-x64.zip"
          if ($archive.Length -le 0) {
            throw "Windows inspection artifact is empty"
          }
          Write-Host "Created $($archive.FullName) ($($archive.Length) bytes)"
```

- [ ] **Step 4: 上传短期 Windows artifact**

继续加入：

```yaml
      - name: Upload Windows inspection artifact
        uses: actions/upload-artifact@v7
        with:
          name: novel-studio-windows-x64-${{ github.sha }}-${{ github.run_attempt }}
          path: |
            apps/desktop/build/artifacts/NovelStudio-windows-x64.zip
            apps/desktop/build/bin/build-manifest.json
          if-no-files-found: error
          retention-days: 7
          compression-level: 0
```

- [ ] **Step 5: 检查 workflow 权限、凭据和包范围**

Run:

```bash
rg -n "runs-on:|permissions:|contents: read|secrets\\.|OPENAI|ANTHROPIC|build/artifacts|build/bin/build-manifest|upload-artifact@v7" .github/workflows/ci.yml
```

Expected:

- macOS 为 `macos-15`，Windows 为 `windows-2025`；
- 权限仍只有 `contents: read`；
- 不包含 `secrets.`、`OPENAI`、`ANTHROPIC`；
- 上传路径只包含两个显式压缩包和各自 manifest；
- 不上传 `apps/desktop/build/bin/` 整个目录。

- [ ] **Step 6: 提交 Windows 构建强化**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: retain inspectable Windows Wails build"
```

### Task 5: 固定双平台构建与 artifact 文档

**Files:**

- Create: `docs/testing/wails-build-matrix.md`
- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/README.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 创建双平台构建矩阵说明**

创建 `docs/testing/wails-build-matrix.md`：

```markdown
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

## R8 边界

以下能力留给 R8：macOS Intel/universal、Windows ARM64、正式版本号与渠道、DMG/PKG/MSI/MSIX/NSIS、Developer ID、Apple 公证、Gatekeeper、Authenticode、安装/升级/卸载、Release artifact 和长期供应链证明。
```

- [ ] **Step 2: 更新自动化测试基线**

在 `docs/testing/automated-test-baseline.md` 的环境说明后加入：

```markdown
双平台 Runner、产物契约、短期 artifact 与失败分类见 [Wails 双平台构建矩阵](wails-build-matrix.md)。
```

将 Foundation CI 表更新为：

```markdown
| Job | 职责 | 不负责 |
| --- | --- | --- |
| Ubuntu tests | 策略门禁、产物契约单元测试、正式前端、Go race/vet、原型 | Wails 原生安装与签名 |
| macOS production build | 固定 arm64 Runner 上构建、结构/plist 校验、manifest、短期 tar artifact | Developer ID、正式签名、公证、安装和升级 |
| Windows production build | 固定 x64 Runner 上构建、PE 校验、manifest、Windows Go tests、短期 zip artifact | Authenticode、安装器和升级 |
```

在本地策略门禁命令后加入：

```bash
node --test scripts/ci/wails-build-artifact.test.mjs
```

将隐私规则中的：

```markdown
- 测试输出、GitHub artifact 和失败日志不得包含正文、Prompt、凭据、SQL 或本地数据库；
```

替换为：

```markdown
- 测试输出、GitHub artifact 和失败日志不得包含用户正文、Prompt、凭据、本地数据库、SQL dump、调试查询或失败现场数据；生产二进制正常包含的版本化迁移资源不属于额外泄露工件；
```

- [ ] **Step 3: 更新文档索引**

在 `docs/README.md` 的测试/运维阅读项中同时链接：

```markdown
[自动化测试基线](testing/automated-test-baseline.md)、[Wails 双平台构建矩阵](testing/wails-build-matrix.md)、[项目恢复](operations/project-recovery.md)与[基础手工验收](testing/foundation-acceptance.md)
```

- [ ] **Step 4: 核对路线图计划状态**

确认 `docs/roadmap/development-roadmap.md` 的 R0 计划列表为：

```markdown
1. [R0 CI 与测试基线](../plans/2026-08-06-r0-ci-test-baseline-implementation.md)（`completed`）
2. [R0 双平台 Wails 构建矩阵](../plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md)（`in_progress`）
3. `R0 旧项目迁移与恢复 fixture`
```

若链接或状态不一致，在提交文档前修正。计划执行完成时，再把第二项状态同步改为 `completed`。

- [ ] **Step 5: 检查新增 Markdown 链接**

Run:

```bash
node -e 'const fs=require("fs"),path=require("path"); const files=["docs/README.md","docs/testing/automated-test-baseline.md","docs/testing/wails-build-matrix.md","docs/roadmap/development-roadmap.md"]; let bad=[]; for(const f of files){const s=fs.readFileSync(f,"utf8"); for(const m of s.matchAll(/\[[^\]]+\]\(([^)]+)\)/g)){const t=m[1].split("#")[0]; if(!t||/^[a-z]+:/i.test(t)) continue; if(!fs.existsSync(path.resolve(path.dirname(f),t))) bad.push(`${f}: ${m[1]}`)}} if(bad.length){console.error(bad.join("\n")); process.exit(1)} console.log("all Wails matrix links resolve")'
```

Expected: PASS，输出 `all Wails matrix links resolve`。

- [ ] **Step 6: 提交文档**

```bash
git add docs/README.md docs/testing/automated-test-baseline.md docs/testing/wails-build-matrix.md docs/roadmap/development-roadmap.md
git commit -m "docs: define Wails build artifact matrix"
```

### Task 6: 执行 R0 双平台构建矩阵验收

**Files:**

- Verify: `.github/workflows/ci.yml`
- Verify: `scripts/ci/wails-build-artifact.mjs`
- Verify: `scripts/ci/wails-build-artifact.test.mjs`
- Verify: `docs/testing/wails-build-matrix.md`
- Modify after evidence: `docs/plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md`
- Modify after evidence: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 运行两个 CI 策略测试**

Run:

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
node --test scripts/ci/wails-build-artifact.test.mjs
```

Expected: tracked artifact 4 项与 Wails artifact 4 项全部 PASS，仓库扫描 PASS。

- [ ] **Step 2: 运行现有本地 Foundation 门禁**

Run:

```bash
cd apps/desktop/frontend
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../../..
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
cd prototype/web
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../..
```

Expected: 两套前端测试/typecheck/build、Go race tests 与 vet 全部退出码为 0。

- [ ] **Step 3: 本地 macOS production build smoke**

在 macOS 实施环境安装固定 Wails CLI 后运行：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
cd apps/desktop
wails doctor
wails build -clean -m
cd ../..
BUILD_RUNNER_LABEL=local-macos \
  node scripts/ci/wails-build-artifact.mjs \
  --platform darwin \
  --build-dir apps/desktop/build/bin \
  --output apps/desktop/build/bin/build-manifest.json
plutil -lint apps/desktop/build/bin/NovelStudio.app/Contents/Info.plist
```

Expected: Wails build、Mach-O/bundle 检查和 plist 语法检查均 PASS。此步骤只验证当前 macOS 开发机，不替代 `macos-15` CI。

- [ ] **Step 4: 检查差异、忽略目录与敏感文件**

Run:

```bash
git diff --check
git status --short
git diff --name-only
git check-ignore -v apps/desktop/build/bin/build-manifest.json
git check-ignore -v apps/desktop/build/artifacts/NovelStudio-macos-arm64.tar.gz
```

Expected:

- `git diff --check` 无输出；
- 变更只包含本计划列出的 CI、脚本、测试和文档；
- build output/manifest/package 均被忽略；
- `apps/desktop/build/darwin/Info.dev.plist` 仍未跟踪且未暂存；
- 不出现数据库、项目包、凭据、证书或真实正文。

- [ ] **Step 5: 推送分支并记录 GitHub Foundation CI**

按正常流程推送 `codex/r0-dual-platform-wails-build-matrix`，等待：

- `Ubuntu tests`；
- `macOS production build`；
- `Windows production build`

全部完成。只有三个 Job 均为 `success`，且 macOS/Windows 各自存在一个可下载 artifact，才能继续。

- [ ] **Step 6: 下载并复核两个 artifact**

直接下载该分支最新一次 Foundation CI run：

```bash
WAILS_RUN_ID="$(gh run list --workflow ci.yml --branch codex/r0-dual-platform-wails-build-matrix --limit 1 --json databaseId --jq '.[0].databaseId')"
WAILS_ARTIFACT_ROOT="$(mktemp -d /tmp/novel-studio-wails-artifacts.XXXXXX)"
gh run download "$WAILS_RUN_ID" --dir "$WAILS_ARTIFACT_ROOT"
find "$WAILS_ARTIFACT_ROOT" -maxdepth 3 -type f -print | sort

WAILS_MAC_DIR="$(find "$WAILS_ARTIFACT_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'novel-studio-macos-arm64-*' -print -quit)"
WAILS_WINDOWS_DIR="$(find "$WAILS_ARTIFACT_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'novel-studio-windows-x64-*' -print -quit)"
mkdir -p "$WAILS_MAC_DIR/unpacked" "$WAILS_WINDOWS_DIR/unpacked"
tar -xzf "$WAILS_MAC_DIR/NovelStudio-macos-arm64.tar.gz" -C "$WAILS_MAC_DIR/unpacked"
unzip -q "$WAILS_WINDOWS_DIR/NovelStudio-windows-x64.zip" -d "$WAILS_WINDOWS_DIR/unpacked"

WAILS_HEAD_SHA="$(git rev-parse HEAD)"
node -e 'const fs=require("fs"),crypto=require("crypto"); const [outerPath,innerPath,entryPath,platform,label,arch,commit]=process.argv.slice(1); const outerText=fs.readFileSync(outerPath,"utf8"), innerText=fs.readFileSync(innerPath,"utf8"); if(outerText!==innerText) throw new Error(`${platform} inner and outer manifests differ`); const m=JSON.parse(outerText); const hash=crypto.createHash("sha256").update(fs.readFileSync(entryPath)).digest("hex"); if(m.platform!==platform||m.runner.label!==label||m.runner.arch!==arch||m.source.commit!==commit||m.artifact.entrypoint_sha256!==hash) throw new Error(`${platform} manifest verification failed`); console.log(`${platform} artifact verified: ${hash}`)' \
  "$WAILS_MAC_DIR/build-manifest.json" \
  "$WAILS_MAC_DIR/unpacked/build-manifest.json" \
  "$WAILS_MAC_DIR/unpacked/NovelStudio.app/Contents/MacOS/NovelStudio" \
  darwin macos-15 ARM64 "$WAILS_HEAD_SHA"
node -e 'const fs=require("fs"),crypto=require("crypto"); const [outerPath,innerPath,entryPath,platform,label,arch,commit]=process.argv.slice(1); const outerText=fs.readFileSync(outerPath,"utf8"), innerText=fs.readFileSync(innerPath,"utf8"); if(outerText!==innerText) throw new Error(`${platform} inner and outer manifests differ`); const m=JSON.parse(outerText); const hash=crypto.createHash("sha256").update(fs.readFileSync(entryPath)).digest("hex"); if(m.platform!==platform||m.runner.label!==label||m.runner.arch!==arch||m.source.commit!==commit||m.artifact.entrypoint_sha256!==hash) throw new Error(`${platform} manifest verification failed`); console.log(`${platform} artifact verified: ${hash}`)' \
  "$WAILS_WINDOWS_DIR/build-manifest.json" \
  "$WAILS_WINDOWS_DIR/unpacked/build-manifest.json" \
  "$WAILS_WINDOWS_DIR/unpacked/NovelStudio.exe" \
  windows windows-2025 X64 "$WAILS_HEAD_SHA"
```

Expected:

- macOS artifact 含 `NovelStudio-macos-arm64.tar.gz` 与 `build-manifest.json`；
- Windows artifact 含 `NovelStudio-windows-x64.zip` 与 `build-manifest.json`；
- 两个 manifest 的 commit 都等于该次 run 的 head SHA；
- Runner 分别为 `macos-15/ARM64` 与 `windows-2025/X64`；
- 解包后只包含平台 production build 和 manifest；
- manifest 入口 SHA-256 与解包后的入口一致。

完成后可删除 `$WAILS_ARTIFACT_ROOT`；它是本次验收的临时下载目录，不属于仓库或产品数据。

- [ ] **Step 7: 记录分支验收证据**

在本计划末尾新增“完成证据（分支）”，记录：

- 分支 Foundation CI run URL/ID；
- 三个 Job 结论；
- 两个 artifact 名称、Runner/架构、manifest commit 和入口 SHA-256；
- 本地测试数量与 production build 结果；
- 任何 Runner image 或 Wails 诊断警告。

计划和路线图继续保持 `in_progress`。提交分支证据：

```bash
git add docs/plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md
git commit -m "docs: record R0 Wails build matrix evidence"
git push origin codex/r0-dual-platform-wails-build-matrix
```

- [ ] **Step 8: 合入 main 并验证合并提交**

按照项目工作流自动将已验证分支 fast-forward 合入 `main`，推送后等待 `main` 最新 Foundation CI：

```bash
git switch main
git merge --ff-only codex/r0-dual-platform-wails-build-matrix
git push origin main
WAILS_MAIN_RUN_ID="$(gh run list --workflow ci.yml --branch main --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$WAILS_MAIN_RUN_ID" --exit-status
gh run view "$WAILS_MAIN_RUN_ID" --json conclusion,jobs,url
```

Expected: main 最新 run 的 `Ubuntu tests`、`macOS production build` 和 `Windows production build` 均为 `success`，且两个平台 artifact 仍存在。

- [ ] **Step 9: 标记计划完成**

只有 Step 1–8 全部通过后，才在 main 工作区：

1. 把本计划状态从 `in_progress` 改为 `completed`；
2. 把 `docs/roadmap/development-roadmap.md` 中第二项改为 `completed`；
3. 在“完成证据”中补充 main run URL/ID 和三个 Job 结论。

不得在任一平台失败、artifact 缺失或 manifest 不匹配时标记完成。

- [ ] **Step 10: 提交完成状态并复核最终 main CI**

```bash
git add docs/plans/2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: complete R0 Wails build matrix plan"
git push origin main
WAILS_FINAL_RUN_ID="$(gh run list --workflow ci.yml --branch main --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$WAILS_FINAL_RUN_ID" --exit-status
```

Expected: 完成状态提交触发的最终 main Foundation CI 仍为 success。若失败，计划恢复为 `in_progress` 并记录失败 Job，不能宣告完成。

## 6. 阶段完成后

本计划完成后，R0 仍缺少 `R0 旧项目迁移与恢复 fixture`。下一份计划应覆盖：

1. 可提交的最小旧 Schema 合成 fixture 或确定性生成器；
2. 旧项目打开、Goose 迁移、升级前备份和未来 Schema 拒绝；
3. 迁移失败、完整性失败和中断时不覆盖原项目；
4. Windows/macOS 可重复回归；
5. R0 全阶段退出标准复核。

只有该计划也完成，且 R0 退出标准全部有证据，才进入 R1 长任务与项目可靠性。
