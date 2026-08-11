# R1.3 项目生命周期保护 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让同一 `.zjproject` 在任意时刻最多只有一个可编辑进程；争锁时提供可理解的重试、真正只读打开和安全恢复副本路径，并保证只读模式不会迁移、恢复、checkpoint、重建索引、启动后台任务或修改 canonical/业务项目数据。

**Architecture:** 在 `projectfs` 提供纯只读包检查、固定短名 `project.lease` 的跨平台 OS lease 和恢复副本装配；在 `sqlite` 增加 `mode=ro` 的当前 Schema 只读入口；`workspace.activeWorkspace` 同时持有 access mode、store generation 与可选项目 lease，并以应用层写屏障和 SQLite 只读连接双重阻止写入。可编辑打开必须先取得 lease，再执行恢复/迁移及其他可变准备；只读打开不取得 lease，且只做只读预检。Wails 通过结构化打开结果呈现 lease conflict，React 用对话卡提供重试、只读、创建恢复副本和取消。

**Tech Stack:** Go 1.25、Wails v2、React 19、TypeScript、Zustand、Zod、SQLite（modernc）、`golang.org/x/sys/unix`、`golang.org/x/sys/windows`、Vitest、Go `testing` 子进程 fixture。

---

- 状态：`completed`
- 路线图阶段：[R1：长任务与项目可靠性](../roadmap/development-roadmap.md#6-r1长任务与项目可靠性)
- 上游依据：[产品需求 5.1.7–5.1.8、7.5、9.1.13](../product/requirements.md)、[产品设计 8/10.15](../product/design.md)、[系统设计 3、5、13、14](../architecture/system-design.md)、[ADR-0002](../architecture/0002-go-first-wails-stack.md)、[ADR-0003](../architecture/0003-wails-project-layout.md)
- 前置计划：[R1.1 Agent job lease 与工作区锁隔离](2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md)、[R1.2 可恢复 JobEvent feed](2026-08-06-r1-recoverable-job-event-feed-implementation.md)（均 `completed`）
- 同阶段后续：`R1.4 Windows 文件锁、写入探针与路径边界`

## 1. 当前基线与已知缺口

当前代码已经具备：

- `workspace.Service` 的 `lifecycleMu`、`storeGeneration` 和 job lease drain，项目切换/关闭会先停止作业再关闭 store；
- `.zjproject/project.db`、固定短目录、WAL、`busy_timeout=5000`、安全迁移、快照/分支中断恢复；
- Wails 安全错误码、React/Zustand 工作区入口及项目打开流程；
- Windows/macOS Foundation CI 和生产构建。

当前缺口：

1. `activeWorkspace` 只持有 package/store/generation，没有跨进程 lease；两个进程可同时执行 `OpenExistingDatabase` 并写同一项目。
2. `projectfs.OpenLayout` 会创建 `imports/`、`recovery/` 并清理临时文件，不能用于真正只读打开。
3. `sqlite.Factory.Open` 会恢复迁移、执行迁移并配置 WAL；没有 `mode=ro`、不迁移的 store。
4. 只读状态没有进入 `ProjectView`，应用层写用例与 UI 也没有统一写屏障。
5. lease conflict 目前只能被压成普通字符串错误，无法安全显示占用主机、PID、版本和开始时间，也没有救局动作。
6. 没有从活跃 WAL 数据库生成一致数据库快照、校验整包稳定性后原子发布的恢复副本流程。
7. `Close`/`replaceActive` 关闭 store 后还不需要释放 OS lease；实现后必须严格保证顺序为 `drain jobs → close DB → release lease`。

## 2. 本计划范围

### 2.1 包含

- 固定短名 `project.lease`，Darwin/Linux 使用非阻塞 `flock`，Windows 使用非阻塞 `LockFileEx`；
- lease 元数据：PID、主机、应用版本、开始时间、可选 project ID；有效性只看 OS lock；
- 纯只读 `Inspect` 与可编辑 `PrepareEditable` 分离；
- SQLite 当前 Schema 的真正只读 store，能读取活跃 WAL 的已提交内容；
- 只读模式不执行迁移、恢复、checkpoint、索引重建、临时清理、分支初始化、作业恢复或模型任务；
- `ProjectView.accessMode`、应用层统一写屏障和 UI 只读提示/禁用；
- lease conflict 的结构化结果，以及重试、只读打开、创建恢复副本、取消；
- 恢复副本的 SQLite 一致快照、非数据库文件稳定性校验、临时目录装配、完整性校验和原子发布；
- Darwin/Windows 原生第二进程争锁、残留元数据、只读、WAL 与恢复副本测试。

### 2.2 不包含

- 不实现“强制解锁”，不按 PID、主机或时间戳删除锁文件；
- 不把 SQLite `database is locked` 当作项目生命周期锁；
- 不在只读模式偷偷创建目录、清理临时文件或自动升级旧 Schema；
- 不实现同步盘供应商检测、运行期 sharing violation 救援、完整 WAL/SHM/checkpoint/同目录原子替换写入探针；
- 不完成 Windows 默认 MAX_PATH 的全部路径预算、长路径前缀、保留名和附件深层路径验收；
- 不修改正文、资产、导入或 JobEvent Schema。

最后两项进入 R1.4；R1 阶段退出必须等 R1.4 完成，不把本计划完成误报为整个 R1 完成。

## 3. 关键设计决策

### 3.1 lease 先于任何可变打开动作

```text
可编辑打开：Inspect（纯读）→ Acquire OS lease → PrepareEditable
            → 恢复/迁移 → Open writable store → 恢复 jobs/branch catalog
            → 激活 workspace

只读打开：Inspect（纯读）→ CheckRecoveryRequired（纯读）
          → OpenReadOnly store → 读取 project/chapters → 激活 workspace
```

可编辑候选在任何步骤失败，都先关闭已打开 store，再释放候选 lease；原活动工作区保持不变。创建新项目时，目录创建成功后立即取得 lease，再创建数据库。

### 3.2 锁文件与元数据

- 路径固定为 `<root>/project.lease`，释放时只 unlock/close，不删除文件；残留 JSON 是诊断信息，不是锁真实性来源。
- 元数据写入只发生在成功取得 OS lock 后，采用 `truncate → JSON encode → fsync`；读取失败只让 holder 变为 unknown，不得把有效争锁降级为可编辑打开。
- `ProjectLeaseConflict` 包含 package path、可选 holder、`retryable=true`；日志和 UI 不显示用户名目录以外的敏感进程参数。
- 单元测试必须用 helper subprocess 争锁，不能用同进程两次 `flock` 代替跨进程语义。

### 3.3 access mode 与双重写保护

```go
type ProjectAccessMode string

const (
    ProjectAccessEditable ProjectAccessMode = "editable"
    ProjectAccessReadOnly ProjectAccessMode = "read_only"
)
```

`activeWorkspace` 保存 `mode` 和可选 `ports.ProjectLease`。所有 workspace 写用例在持有 `s.mu` 后先调用统一 `requireEditableLocked()`；遗漏应用层检查时，SQLite `mode=ro`/`query_only` 仍必须拒绝写入。读用例继续可用。只读状态禁止：正文/草稿保存与定稿、资产/ChangeSet 写入、导入/抽取、Agent 生成、快照/分支写操作、恢复、瘦身和后台作业恢复。

### 3.4 只读不是“可写连接但 UI 禁用”

`OpenReadOnlyDatabase` 使用 SQLite URI `mode=ro`，单连接，设置 `busy_timeout`、`foreign_keys` 和 `query_only=ON`，但不设置 `journal_mode`、不 checkpoint、不迁移。它必须：

- 读取现有 `-wal/-shm` 中已提交状态；
- 验证 SQLite header、`quick_check`、唯一 project 和 Schema 恰好为当前版本；
- 旧 Schema 返回稳定的 `ErrDatabaseMigrationRequired`，未来 Schema沿用 `ErrDatabaseTooNew`；
- 检测数据库、快照或分支的待恢复 journal 时返回 `ErrProjectRecoveryRequired`，仅报告，不执行恢复。

SQLite 为读取活跃 WAL 可能更新 `project.db-shm`，并在 WAL 协调文件不存在时创建空的 `-wal/-shm`。这些 SQLite 读取协调工件不属于 canonical 或业务数据，允许发生该受限变化；只读验证仍必须证明 `project.db`、已有 `project.db-wal` 的内容、`recovery/` 与其他项目文件集合和哈希不变。不得使用 `immutable=1` 规避该行为，因为它会忽略活跃 WAL。

### 3.5 恢复副本是新项目包，不覆盖源项目

恢复副本写入作者选择的父目录，目标名使用截断的 portable basename、UTC 时间和短随机 ID，始终 `O_EXCL`/不覆盖。适配器按以下顺序执行：

1. 纯读检查源包，建立除 `project.db*`、`project.lease`、`recovery/` 和临时文件外的文件清单与哈希；
2. 在目标父目录创建同文件系统临时包；
3. 从源数据库只读连接执行 SQLite 一致快照到临时 `project.db`，不能裸拷贝活跃 `project.db`；
4. 复制清单中的附件、导入、快照、分支和其他持久文件，目标使用 `O_CREATE|O_EXCL`，逐文件 fsync；
5. 再次扫描源文件清单与哈希；文件新增、删除或变化则放弃临时包并返回可重试错误；
6. 对副本执行 `quick_check`、Schema、唯一 project、快照/分支目录只读校验；
7. fsync 临时目录后原子 rename 为最终 `.zjproject`；失败不留下对用户可见的半成品；
8. 正常可编辑打开新副本并取得它自己的 lease。

源项目始终只读。失败清理仅限本次生成且带随机 ID 的临时目录；不得删除源文件或已存在目标。

### 3.6 Wails/React 契约

`OpenProject` 改为结构化结果，而不是用错误字符串承载 holder：

```ts
type ProjectOpenResult =
  | { outcome: "opened"; project: ProjectView }
  | { outcome: "lease_conflict"; conflict: ProjectLeaseConflictView };
```

`openProject(path, "editable" | "read_only")` 只有 lease contention 返回 `lease_conflict`；损坏、权限、迁移需求等仍走稳定错误码。冲突卡保存所选 path，`重试` 再发 editable open；`只读打开` 发 read-only open；`创建恢复副本` 先选父目录再创建并打开；`取消` 回到原工作区/欢迎页。

## 4. 文件级改动范围

| 模块 | 主要文件 | 职责 |
|---|---|---|
| Ports | `internal/application/ports/project_package.go`、新建 `project_lease.go`、`project_store.go`、snapshot/branch archive ports | access mode、lease、纯读检查、只读 store、恢复需求探针与恢复副本契约 |
| Project FS | `internal/platform/projectfs/layout.go`、`manager.go`、新建 `lease*.go`、`recovery_copy.go` | 纯读布局、跨平台锁、元数据、稳定整包复制 |
| SQLite | `internal/persistence/sqlite/database.go`、`project_store.go`、新建 `readonly_test.go` | `mode=ro`、Schema/恢复需求分类、一致数据库副本 |
| Snapshot/branch | `internal/platform/snapshotfs/archive.go`、`internal/platform/branchfs/archive.go` | 只读判断是否存在待恢复 journal |
| Application | `internal/application/workspace/service.go`、`errors.go`、`view.go` 及所有写用例文件 | lease 生命周期、access mode、写屏障、恢复副本用例 |
| Desktop | `apps/desktop/api.go`、`errors.go`、`app.go` 及测试 | 结构化 open result、稳定错误码、实例元数据注入 |
| React | `studio-client.ts`、`wails-studio-client.ts`、workspace store、`App.tsx`、`Workspace.tsx`、相关 workbench/tests | 冲突救局、只读 banner、禁用写动作与客户端 schema |
| CI/docs | `.github/workflows/ci.yml`、testing/operations 文档、路线图 | 原生锁测试、人工验收与恢复说明 |

## 5. TDD 实施任务

### Task 1：冻结 access mode、包检查和错误契约

**Files:**

- Modify: `internal/application/ports/project_package.go`
- Modify: `internal/application/ports/project_store.go`
- Create: `internal/application/ports/project_lease.go`
- Modify: `internal/platform/projectfs/layout.go`
- Modify: `internal/platform/projectfs/manager.go`
- Modify: `internal/platform/projectfs/layout_test.go`
- Modify: `internal/platform/projectfs/manager_test.go`
- Modify: `internal/application/workspace/errors.go`
- Modify: `internal/application/workspace/view.go`

- [ ] 先写失败测试：`Inspect` 对合法项目零写入，缺少 `imports/recovery` 时也不创建；`PrepareEditable` 才补齐兼容目录并清理孤儿临时文件。
- [ ] 给 `ProjectPackage` 增加 `Recovery`、`Lease`；将 `Open` 拆为 `Inspect(packagePath)` 与 `PrepareEditable(packageInfo)`，所有路径仍由 `layoutForRoot` 产生。
- [ ] 定义 `ProjectAccessMode`、`ProjectLeaseMetadata`、`ProjectLeaseConflict`、`ProjectLease/ProjectLeaseManager`、`ErrProjectLeaseHeld`、`ErrWorkspaceReadOnly`、`ErrProjectRecoveryRequired`、`ErrRecoveryCopySourceChanged`。
- [ ] `ProjectView` 增加 `accessMode`；新建项目固定为 `editable`。
- [ ] 更新 fake ports 和编译测试，不在本任务实现真实锁。
- [ ] 运行：

```bash
go test -count=1 ./internal/platform/projectfs ./internal/application/workspace
```

- [ ] Commit：`git commit -m "refactor: separate project inspection from editable preparation"`

### Task 2：实现 Darwin/Linux/Windows 跨进程 OS lease

**Files:**

- Create: `internal/platform/projectfs/lease.go`
- Create: `internal/platform/projectfs/lease_unix.go`
- Create: `internal/platform/projectfs/lease_windows.go`
- Create: `internal/platform/projectfs/lease_test.go`
- Create: `internal/platform/projectfs/lease_process_test.go`

- [ ] 先写 helper subprocess 测试：第一进程持锁时第二进程得到 typed conflict；释放后立即可取得；残留合法/损坏元数据都不阻止重新取得 OS lock；未持锁者绝不 truncate 元数据。
- [ ] Unix 用 `unix.Flock(fd, LOCK_EX|LOCK_NB)`；Windows 用 `windows.LockFileEx(...LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY...)`，错误只在明确 lock/sharing contention 时映射 `ErrProjectLeaseHeld`。
- [ ] 成功持锁后写/flush JSON；`Release` 幂等且先 unlock 后 close。不得 unlink lease 文件。
- [ ] 用 build tags 保证目标平台只编译对应实现，并让 Linux race 测试、macOS/Windows 原生测试都运行同一契约。
- [ ] 运行：

```bash
go test -count=20 -run 'TestProjectLease' ./internal/platform/projectfs
GOOS=windows GOARCH=amd64 go test -c -o /tmp/novel-studio-projectfs-windows.test.exe ./internal/platform/projectfs
```

- [ ] Commit：`git commit -m "feat: add cross-process project lease"`

### Task 3：实现不迁移、不恢复的 SQLite 只读 store

**Files:**

- Modify: `internal/persistence/sqlite/database.go`
- Modify: `internal/persistence/sqlite/integrity.go`
- Modify: `internal/persistence/sqlite/project_store.go`
- Create: `internal/persistence/sqlite/readonly_test.go`
- Modify: `internal/persistence/sqlite/migration_test.go`

- [ ] 先写失败测试：当前 Schema 可读；写方法返回 readonly；只读打开前后 `project.db`、已有 WAL 内容、`recovery/` 与其他业务项目文件集合和哈希不变；允许 SQLite 为活跃 WAL 读取更新 `-shm` 或创建空的 WAL 协调文件；旧 Schema 返回 migration-required；未来 Schema不变；活跃 WAL 中已提交章节可见。
- [ ] 用正确转义的 SQLite file URI 加 `mode=ro`；不要使用 `immutable=1`（它会忽略活跃 WAL 语义）。
- [ ] 配置只读 PRAGMA，不调用 `configureRuntimeDatabase`、Goose、migration recovery 或 checkpoint；验证当前 Schema、`quick_check` 和唯一项目。
- [ ] `Factory.OpenReadOnly` 返回同一 `Store` 读接口，SQLite 权限作为应用层写屏障后的第二道防线。
- [ ] 验证 `Store.Backup` 能从只读源生成一致副本；若 `VACUUM INTO` 在 `mode=ro/query_only` 下不受支持，使用 modernc SQLite backup API 的隔离适配器，禁止临时打开可写源连接。
- [ ] 运行：

```bash
go test -count=1 -run 'Test(OpenReadOnly|ReadOnly|RecoveryCopyDatabase)' ./internal/persistence/sqlite
go test -count=1 -race ./internal/persistence/sqlite
```

- [ ] Commit：`git commit -m "feat: add true read-only sqlite project store"`

### Task 4：只读预检与 workspace lease 生命周期

**Files:**

- Modify: `internal/application/ports/snapshot_archive.go`
- Modify: `internal/application/ports/branch_archive.go`
- Modify: `internal/platform/snapshotfs/archive.go`
- Modify: `internal/platform/branchfs/archive.go`
- Modify: corresponding `*_test.go`
- Modify: `internal/application/workspace/service.go`
- Modify: `internal/application/workspace/service_test.go`
- Modify: `internal/application/workspace/recovery_test.go`

- [ ] 先写失败测试：只读预检发现 snapshot/branch/database 待恢复状态但不改文件；可编辑打开先拿 lease 后恢复；只读打开跳过 `recoverInterruptedJobs` 和 `ensureBranchCatalog`。
- [ ] 为 snapshot/branch archive 增加纯读 `RecoveryRequired(...) (bool, error)`；不得复用会写盘的 recover 方法冒充检查。
- [ ] `activeWorkspace` 增加 `mode` 和 `projectLease`；Service 注入 lease manager 与进程元数据，测试使用 fake lease manager，不把 OS 行为混入应用单测。
- [ ] 重构 create/editable-open/read-only-open 的候选构建与失败清理；候选失败不替换旧 active。
- [ ] 重构 close/switch 顺序：job lease drain 完成后关闭 DB，再释放 OS lease；store close 失败也必须尝试 release，并用 `errors.Join` 保留两类错误。
- [ ] 更新 lease 元数据中的 project ID 只能在锁已持有且 project 已读出后进行。
- [ ] 运行：

```bash
go test -count=1 -race -run 'Test(CreateProject|OpenProject|OpenProjectReadOnly|Close|ReplaceActive|RecoveryRequired)' ./internal/application/workspace ./internal/platform/snapshotfs ./internal/platform/branchfs
```

- [ ] Commit：`git commit -m "feat: bind project lease to workspace lifecycle"`

### Task 5：建立统一应用层写屏障

**Files:**

- Modify: `internal/application/workspace/service.go`
- Modify: `agent.go`, `extraction.go`, `import.go`, `revision.go`, `version_history.go`, `branch.go`, `story.go`, `world.go`, `world_assets.go`, `continuity.go`（均位于 `internal/application/workspace/`，以实际存在文件为准）
- Create: `internal/application/workspace/read_only_test.go`

- [ ] 先写表驱动失败测试，至少覆盖草稿/正文、章节、ChangeSet、故事资产、世界资产、事实、Agent、抽取、导入应用、snapshot restore 与 branch switch；均返回 `ErrWorkspaceReadOnly` 且数据库/项目文件哈希不变。
- [ ] 增加 `requireEditableLocked()`，并在每个写入口取得 `s.mu` 后、任何 store/archive/provider 调用前执行；读入口不受影响。
- [ ] `beginVersionOperation`、`beginImportOperation` 和作业启动也必须检查 mode，避免先改变内存状态再被数据库拒绝。
- [ ] 用 `rg` 审计所有 `Create/Save/Update/Delete/Archive/Apply/Restore/Switch/Execute/Retry/Cancel` 公共方法；Cancel 对当前内存中运行任务只用于可编辑 workspace，只读打开不会恢复或启动任务。
- [ ] 运行：

```bash
go test -count=1 -race -run 'TestReadOnlyWorkspace' ./internal/application/workspace
rg -n 'func \(s \*Service\) (Create|Save|Update|Delete|Archive|Apply|Restore|Switch|Execute|Retry)' internal/application/workspace
```

- [ ] Commit：`git commit -m "feat: enforce read-only workspace write barrier"`

### Task 6：实现非破坏性恢复副本

**Files:**

- Modify: `internal/application/ports/project_package.go`
- Create: `internal/platform/projectfs/recovery_copy.go`
- Create: `internal/platform/projectfs/recovery_copy_test.go`
- Create: `internal/application/workspace/recovery_copy.go`
- Create: `internal/application/workspace/recovery_copy_test.go`

- [ ] 先写失败测试：活跃 WAL 副本含最新已提交 revision；源 lease/`-wal`/`-shm`/迁移临时物不进入副本；目标已存在不覆盖；源附件在复制中变化则失败；注入 copy/fsync/rename/validation 失败不会发布半成品或删除源。
- [ ] 实现两次稳定清单、SQLite 一致快照、临时包、目标校验、目录 fsync 与原子发布；复制循环必须响应 context cancel。
- [ ] 应用用例 `CreateProjectRecoveryCopy(ctx, sourcePath, parentDirectory)` 只从源读取，创建完成后按普通 editable-open 取得新 lease；若最终打开失败，保留已经验证的副本并在结果中返回路径，不删除唯一有效副本。
- [ ] 多次点击使用不同短 ID，不能幂等覆盖前一次成功副本。
- [ ] 运行：

```bash
go test -count=1 -race -run 'Test(CreateRecoveryCopy|RecoveryCopy)' ./internal/platform/projectfs ./internal/persistence/sqlite ./internal/application/workspace
```

- [ ] Commit：`git commit -m "feat: create validated project recovery copies"`

### Task 7：暴露结构化 Wails 契约与稳定错误

**Files:**

- Modify: `apps/desktop/api.go`
- Modify: `apps/desktop/errors.go`
- Modify: `apps/desktop/app.go`
- Modify: `apps/desktop/api_test.go`
- Modify: `apps/desktop/frontend/src/lib/studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/wails-studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/wails-studio-client.test.ts`
- Regenerate: `apps/desktop/frontend/wailsjs/go/main/DesktopAPI.{js,d.ts}` and affected models through the repository Wails binding workflow

- [ ] 先写 Go/TS 失败测试：editable success、lease conflict（holder 有/无）、read-only success、migration/recovery required、recovery copy success/partial success；错误消息不泄露 SQL/正文/内部路径。
- [ ] 增加 `ProjectOpenInput{Path, Mode}`、`ProjectOpenResultView`、`ProjectLeaseConflictView`、`RecoveryCopyInput/ResultView`；expected contention 不走字符串错误。
- [ ] `safeDesktopError` 增加 `PROJECT_READ_ONLY`、`PROJECT_MIGRATION_REQUIRED`、`PROJECT_RECOVERY_REQUIRED`、`RECOVERY_COPY_SOURCE_CHANGED`、`RECOVERY_COPY_FAILED`；同步目录 sharing violation 分类留给 R1.4。
- [ ] `ProjectView.accessMode` 和所有 Zod schema 严格校验枚举；拒绝未知 mode/outcome。
- [ ] 在 `newApp` 注入 PID、hostname、启动 UTC、可获取的应用版本；版本不可用时显示 `dev/unknown`，不读取其他进程命令行或认证文件。
- [ ] 运行：

```bash
go test -count=1 ./apps/desktop
cd apps/desktop/frontend && npm test -- --run src/lib/wails-studio-client.test.ts && npm run typecheck
```

- [ ] Commit：`git commit -m "feat: expose project open conflict recovery contract"`

### Task 8：实现 React 冲突救局与只读体验

**Files:**

- Modify: `apps/desktop/frontend/src/features/workspace/workspace-store.ts`
- Modify: `apps/desktop/frontend/src/features/workspace/workspace-store.test.ts`
- Modify: `apps/desktop/frontend/src/app/App.tsx`
- Modify: `apps/desktop/frontend/src/app/App.test.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/ProjectLeaseConflictCard.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/ProjectLeaseConflictCard.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Modify: all project-mutating workbench entry components and tests
- Modify: `apps/desktop/frontend/src/app/styles.css`

- [ ] 先写交互失败测试：冲突卡四个动作；重试保留 path；只读成功显示常驻 banner；恢复副本先选目录；取消回到原状态；动作进行中防重复点击；holder 缺失时显示可理解文案。
- [ ] workspace store 增加 `lease_conflict` 状态，保存 path/conflict 和原状态；不要把预期争锁变成通用 error 页。
- [ ] `Workspace` 根据 `project.accessMode` 向编辑器、资产、导入、Agent、版本/分支等传递 capability；隐藏/禁用所有写按钮并保留阅读、导航、复制和导出只读内容。
- [ ] ChapterEditor 只读时不注册 debounce save/commit，不在卸载或关闭时 flush 写请求；文本选择/复制仍可用。
- [ ] 即使 UI 漏禁一个按钮，客户端收到 `PROJECT_READ_ONLY` 后也显示一致说明，不假装保存成功。
- [ ] 运行：

```bash
cd apps/desktop/frontend && npm test -- --run && npm run typecheck && npm run build
```

- [ ] Commit：`git commit -m "feat: add read-only and lease conflict workspace flows"`

### Task 9：原生平台门禁、恢复文档与阶段证据

**Files:**

- Modify: `.github/workflows/ci.yml`
- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/testing/foundation-acceptance.md`
- Modify: `docs/operations/project-recovery.md`
- Modify: `docs/handoff/START-HERE.md`
- Modify: `docs/roadmap/development-roadmap.md`
- Modify: this plan

- [x] macOS/Windows 原生 job 显式运行 projectfs lease helper-process、SQLite read-only/WAL、workspace conflict/recovery-copy tests；Ubuntu race 继续覆盖应用并发，不能用跳过代替平台修复。[run 31451705510](https://github.com/lishen802/novel-studio/actions/runs/31451705510) 的 Ubuntu、macOS 与 Windows job 均通过。
- [x] 手工验收记录：第二实例冲突、占用来源、重试、真正只读、从只读退出、恢复副本、应用退出释放锁、残留元数据、活跃 WAL。Windows 与 macOS 的 F-12 至 F-14 于 2026-08-11 通过，记录见 [`foundation-acceptance.md`](../testing/foundation-acceptance.md)。
- [x] 更新恢复手册，明确不能手删有效 lease、只读不会升级旧项目、恢复副本是新包且原件不变。
- [x] 将计划状态改为 `completed` 之前记录 commit、CI run URL/ID、macOS/Windows 结果和遗留 R1.4 风险。提交 `ee008c36d703b56c7adf9310f854347c7ec81afc` 已推送；[run 31451705510](https://github.com/lishen802/novel-studio/actions/runs/31451705510) 与两平台手工验收均通过，因此计划状态为 `completed`。
- [x] Commit：`ee008c3 fix: harden Windows project lifecycle tests`

## 6. 最终验证

按顺序执行，任一失败都不能把计划标记完成：

```bash
cd apps/desktop/frontend && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../../..
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
cd prototype/web && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../..
git diff --check
git status --short
```

不要使用 `go test ./...`，避免根 module pattern 遍历 `node_modules`。

通过标准：

| 证据 | 门槛 |
|---|---|
| 跨进程互斥 | 第二进程在 macOS/Windows 都不能取得 editable lease |
| 残留元数据 | OS lock 释放后可以重新打开；不依赖删除 lease 文件 |
| 关闭顺序 | jobs 已 drain，DB 已 close，最后 release project lease |
| 真正只读 | `project.db`、已有 WAL 内容、`recovery/` 与业务项目文件不变，无迁移/恢复/后台事件；仅允许 SQLite 的 `-shm`/空 WAL 协调文件变化 |
| 双重写保护 | UI 无写入口；绕过 UI 的应用写用例返回 `ErrWorkspaceReadOnly`；SQLite 仍拒写 |
| WAL 一致性 | 只读能看到已提交 WAL；恢复副本通过 quick_check 且含该 revision |
| 恢复副本 | 不覆盖源/目标；源变化或故障不发布半成品 |
| UX | 冲突卡支持重试、只读、恢复副本、取消，holder 不可读时仍可操作 |
| 回归 | Go race/vet、React、prototype、macOS/Windows build 全部通过 |

## 7. 回滚与故障边界

- 本计划不含数据库迁移；回滚代码不能改写任何项目 Schema。
- 若 lease 实现异常，回滚到上一版本前必须先正常关闭应用；不得用删除 `project.lease` 作为自动修复，因为另一个进程可能仍持有 OS lock。
- 候选 workspace 打开失败必须保留当前 active workspace；候选 store/lease 只关闭自身。
- 恢复副本失败只清理本次随机临时目录；已经校验并原子发布的副本不自动删除。
- 只读打开遇到旧 Schema 或待恢复 journal 时停止并报告；不能为提高成功率绕过完整性检查或调用 writable open。
- R1.4 实现前，同步目录、sharing violation、完整写入探针和 Windows 长路径仍是明确遗留项，必须保留现有通用安全错误和人工恢复指引。

## 8. 实施记录与交付门槛

- 实现提交：`411a098`、`9c67a2c`、`206ecce`、`61ce14a`、`5d3da6a`、`a70175b`、`435fde9`、`b2c3b85221822e2e8415621130716739afb8f4d3`、`00791e4b9e267108fa295761d2cf7c56bde0ece1`、`ee008c36d703b56c7adf9310f854347c7ec81afc`；
- 本地证据：React 测试、typecheck、production build；聚焦 SQLite、workspace、lease 与恢复副本 Go 测试；完整 Go race/vet 与 prototype 验证见最终验证记录；
- CI 证据：提交 `b2c3b85221822e2e8415621130716739afb8f4d3` 的 [run 31370477847](https://github.com/lishen802/novel-studio/actions/runs/31370477847) 因缺少 `frontend/dist` 失败；[run 31450873891](https://github.com/lishen802/novel-studio/actions/runs/31450873891) 在修正构建顺序后通过 Ubuntu 与 macOS，但暴露 Windows 文件语义问题。提交 `ee008c36d703b56c7adf9310f854347c7ec81afc` 的 [run 31451705510](https://github.com/lishen802/novel-studio/actions/runs/31451705510) 在 Ubuntu、macOS 和 Windows 均通过，关闭本计划的 CI 证据完备。
- 手工证据：`foundation-acceptance.md` 的 F-12 至 F-14 已记录为 Windows/macOS 通过（2026-08-11）。
- 遗留风险：R1.4 仍负责同步盘/运行期 sharing violation、完整 WAL/SHM/checkpoint 写入探针与 Windows 默认 MAX_PATH 路径边界。

CI 与人工证据均已完成，本计划状态为 `completed`。R1.4 的同步目录、运行期 sharing violation、完整 WAL/SHM/checkpoint 写入探针和 Windows 默认 MAX_PATH 路径边界仍是独立遗留项，不得据此宣告 R1 阶段整体退出。
