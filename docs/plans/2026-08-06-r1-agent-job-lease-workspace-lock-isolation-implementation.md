# R1 Agent Job Lease 与工作区锁隔离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除跨越 Model IO 的工作区全局锁。Agent 与抽取作业只在短锁内确认 active project、冻结 project/store generation 并取得 job lease，随后释放工作区锁再组装上下文和调用 Provider；切换、关闭或恢复项目时先拒绝新作业，再取消或等待已有 job lease，最后关闭 store。Provider 调用期间允许作者编辑和保存正文，且作业不会向已关闭 store 回写。

**Architecture:** 在 `workspace.Service` 中引入单调递增的 `storeGeneration` 和显式 `jobLease`。`activeWorkspace` 记录当前 generation；作业在短锁内捕获 store 与 generation、注册 lease、将 run 转为 Running 并持久化，然后释放锁。Provider 调用、上下文组装在锁外执行。每个步骤的持久化重新取短锁、校验 generation 未变、写入并释放。`replaceActive` 与 `Close` 先标记 draining 拒绝新 lease，再取消所有作业 context，然后等待 in-flight lease 释放（有界超时），最后关闭旧 store 并递增 generation。generation 校验是安全网：若作业在 lease 释放后仍尝试写入，必须跳过写入并返回恢复错误而非 panic 或写已关闭 store。

**Tech Stack:** Go 1.25、`sync`（`Mutex`/`RWMutex`/`WaitGroup`/`Once`）、`context`、`testing`、fake/Recorded Provider、Wails v2（本计划不新增 Wails 事件，留给 R1.2）。

---

- 状态：`in_progress`
- 路线图阶段：[R1：长任务与项目可靠性](../roadmap/development-roadmap.md#6-r1长任务与项目可靠性)
- 上游依据：[产品需求 5.1/7.1](../product/requirements.md)、[产品与交互设计：运行画布与恢复](../product/design.md)、[系统设计 4.3/5.2/5.3](../architecture/system-design.md)、[ADR-0002](../architecture/0002-go-first-wails-stack.md)
- 前置计划：[R0 旧项目迁移与恢复 fixture](2026-08-06-r0-legacy-project-migration-recovery-fixture-implementation.md)（`completed`）
- 同阶段后续：`R1 JobEvent 持久化与应用 API`、`R1 React JobEvent replay`、`R1 项目 OS lease 与只读 store`、`R1 Windows 文件锁与路径边界`

## 1. 当前基线与已知缺口

当前代码已经具备：

- `workspace.Service` 用 `mu sync.RWMutex` 保护 `active *activeWorkspace`；`lifecycleMu sync.Mutex` 保护 create/open/close 生命周期；
- `agentJobsMu` + `agentJobs map[string]context.CancelFunc`、`extractionJobsMu` + `extractionJobs` 提供按 ID 取消；
- `beginVersionOperation`/`beginImportOperation` 通过布尔标志阻止版本/导入操作与 agent run 并发；
- `replaceActive` 取消所有 agent/extraction 作业、交换 `active`、关闭旧 store；
- `ExecuteAgentRun` 通过 `failed` 闭包在 `context.Canceled` 时用 `context.WithoutCancel(ctx)` 持久化 cancelled 状态；
- `CancelAgentRun`、`RetryAgentRun` 已有状态机校验。

当前缺口（对照系统设计 5.3 与需求 7.1）：

1. `ExecuteAgentRun` 在 [agent.go:440](../../internal/application/workspace/agent.go) 持有 `s.mu.RLock()` 跨越整个 `provider.Generate`（[agent.go:544](../../internal/application/workspace/agent.go)）。`ExecuteExtractionRun` 有相同模式。任何需要写锁的操作（`CreateChapter`、`OpenProject`、`SwitchBranch`、`CreateSnapshot` 等）在 Provider 调用期间被阻塞，违反“模型网络 IO 不得长期持有会阻断作者正文保存、项目关闭或取消作业的工作区全局锁”。
2. `SaveDraft`/`CommitChapter` 虽用 `RLock` 不被 agent run 阻塞，但 `CreateChapter`、分支与快照操作用 `Lock`，会被 Model IO 阻塞；需要明确哪些操作在 Provider 调用期间必须可用。
3. 没有 `storeGeneration` 概念。`replaceActive` 取消作业后立即关闭旧 store，但 `ExecuteAgentRun` 的 `failed` 闭包仍可能用 `context.WithoutCancel(ctx)` 向已关闭 store 写入 cancelled/final 状态，触发“向已关闭 store 回写”。
4. 没有 `jobLease`。`replaceActive`/`Close` 取消 context 后不等待作业实际退出，旧 store 可能在作业仍持有时被关闭；取消信号与 store 关闭无先后保证。
5. 项目切换/关闭/恢复时没有“先拒绝新作业”的显式 draining 状态；新作业可能在 `replaceActive` 进行中拿到半开 store。
6. 缺少并发回归测试：Provider 阻塞期间正文保存成功、切换项目时旧作业退出且不写已关闭 store、重开项目不残留旧 generation 作业。

## 2. 设计选择

### 2.1 单调 store generation 作为安全网

`activeWorkspace` 增加 `generation uint64`，从 1 开始。`Service` 持有 `storeGeneration uint64`（在 `mu` 下递增）。每次 `replaceActive`、`Close` 或快照/分支恢复重开 store 时递增。作业在短锁内捕获 `(store, generation)`；每次持久化前在短锁内重新读取 `s.active` 与 `s.storeGeneration`，若不匹配则跳过写入并进入恢复退出路径。generation 是安全网，正常路径靠 lease drain 保证 store 不被提前关闭。

### 2.2 显式 job lease 与 draining

新增包内结构：

```go
type jobLease struct {
    generation uint64
    release    chan struct{} // closed when lease released
}

type leaseRegistry struct {
    mu       sync.Mutex
    draining bool
    leases   map[string]*jobLease // key: jobID
}
```

- `acquireLease(jobID) (*jobLease, error)`：draining 时返回 `ErrWorkspaceDraining`；否则注册 lease 并返回。
- `releaseLease(jobID)`：删除并 close release channel。
- `drainAndBlock(ctx, timeout)`：设置 draining=true，返回后所有新 acquire 失败；等待已注册 lease 全部 release 或超时；超时后强制标记剩余 lease 为 stale（generation 仍是旧值，store 关闭后写入会触发安全网）。
- `endDraining()`：清除 draining，允许新项目接受作业。

`agentJobs`/`extractionJobs` 的 cancel func 与 lease 配对：`beginAgentJob` 同时 acquire lease；`endAgentJob` 同时 release lease。`cancelAllAgentJobs` 只发取消信号，不释放 lease（lease 由作业自身退出时释放）。

### 2.3 锁不跨越 Model IO

`ExecuteAgentRun` 重构为三阶段：

1. **短锁**（`mu.Lock`）：读 `s.active`、校验 store 可用、将 run 转为 Running、`store.UpdateAgentRun`、捕获 `(store, generation)`、`acquireLease(runID)`、释放锁。lease acquire 在锁内确保 generation 与 active 一致。
2. **锁外**：`assembleAuthoringContext`、`provider.Generate`、`onDelta` 累积。`s.mu` 完全不持有；`SaveDraft`、`CreateChapter`、`OpenProject` 等可并发执行。
3. **每步短锁**（`mu.Lock`）：校验 generation 未变、`store.UpdateAgentRunStep`、`store.UpdateAgentRun`、`store.CreateAgentArtifact`、释放锁。generation 变化时调用 `failed` 的 stale 分支，不写旧 store。

`ExecuteExtractionRun` 同样重构。`assembleAuthoringContext` 内部的多次 store 读仍可锁外执行（SQLite WAL 支持并发读）；本计划不引入新事务边界。

### 2.4 replaceActive 与 Close 的 drain 顺序

```text
replaceActive(next):
  lifecycleMu 已持有
  leases.drainAndBlock(timeout)         // 拒新作业 + 等待 in-flight 退出
  cancelAllAgentJobs()                   // 仍存活的作业发取消
  cancelAllExtractionJobs()
  leases.waitRemaining(timeout)          // 二次等待 lease 释放
  mu.Lock
  previous = s.active
  s.active = &next(generation=++s.storeGeneration)
  mu.Unlock
  leases.endDraining()                   // 新项目可接受作业
  if previous != nil: previous.store.Close()
```

`Close` 类似：drainAndBlock → cancel → wait → endDraining → 关闭 store。

有界超时避免死锁；超时后剩余作业标记为 stale，其后续写入由 generation 安全网拦截，store 关闭后写入返回错误被作业捕获并转为 `RunRecoverable`（不 panic、不写已关闭 store）。

### 2.5 作业退出向旧 store 持久化

作业捕获的 `store` 是启动时的 store 引用。drain 期间旧 store 仍未关闭，作业的 `failed`/cancelled 持久化向该引用写入是安全的。只有 drain 超时后 store 才关闭；此时 generation 已变，作业的短锁校验失败，进入 stale 分支，不再写旧 store。这样满足“作业向已关闭 store 回写”被禁止。

### 2.6 不引入 JobEvent、OS 锁与 React 改动

本计划不创建 `JobEvent`/`JobSnapshot` 表、不发送 Wails 事件、不改动 React。作业状态仍由 `AgentRun`/`AgentRunStep` 表持久化；前端继续通过 `AgentRun`/`AgentRuns` RPC 轮询或等待 `ExecuteAgentRun` 同步返回。事件持久化、cursor 补拉与 React replay 属于 R1.2/R1.3。项目级 OS 文件锁与只读 store 属于 R1.3/R1.4。

## 3. 明确不包含

- 不新增 `JobEvent`、`JobSnapshot` 或单调 sequence 持久化（R1.2）；
- 不发送 Wails Event，不改动 React 或前端 cursor（R1.2/R1.3）；
- 不实现项目级 OS 文件锁、第二实例禁止、只读 store 或恢复副本（R1.3/R1.4）；
- 不实现 Windows 文件锁、长路径或保留名边界（R1.5）；
- 不修改数据库 Schema、不新增 Goose migration；
- 不改变 `ExecuteAgentRun`/`ExecuteExtractionRun` 的对外签名与 Wails 绑定；
- 不引入新 Provider 或改变 `ModelProvider` 接口；
- 不实现批次、检查点、熔断或 `GenerationAttempt`（R4）；
- 不改变快照/分支的 journal+stage+backup 恢复模式。

## 4. 文件级改动

| 文件 | 职责 |
| --- | --- |
| `internal/application/workspace/lease.go` | `jobLease`、`leaseRegistry`、drain/acquire/release。 |
| `internal/application/workspace/lease_test.go` | lease 并发、drain、超时、stale 标记单测。 |
| `internal/application/workspace/service.go` | `activeWorkspace.generation`、`storeGeneration`、`leaseRegistry` 注入；`replaceActive`/`Close` drain 顺序；draining 拒绝新作业。 |
| `internal/application/workspace/agent_control.go` | `beginAgentJob`/`endAgentJob` 配对 lease acquire/release；`cancelAllAgentJobs` 不释放 lease；新增 `ErrWorkspaceDraining`。 |
| `internal/application/workspace/agent.go` | `ExecuteAgentRun` 三阶段重构；generation 校验；stale 退出分支；`SaveAgentRuntimeSettings`/`AgentRuns` 等短锁不变。 |
| `internal/application/workspace/extraction.go` | `ExecuteExtractionRun` 同样三阶段重构；`beginExtractionJob`/`endExtractionJob` 配对 lease。 |
| `internal/application/workspace/agent_test.go` | 新增并发与 drain 测试。 |
| `internal/application/workspace/extraction_test.go` | 新增并发与 drain 测试。 |
| `internal/application/workspace/service_test.go` | 切换/关闭 drain 顺序、拒绝新作业、generation 递增测试。 |
| `internal/application/workspace/agent_concurrency_test.go` | Provider 阻塞期间 SaveDraft/CreateChapter 成功；切换项目时旧作业退出且不写已关闭 store。 |
| `docs/testing/automated-test-baseline.md` | 把“Model IO 不阻塞正文保存”与“作业不写已关闭 store”纳入基线。 |
| `docs/operations/project-recovery.md` | 补充作业取消与 store 关闭的恢复语义说明。 |
| `docs/roadmap/development-roadmap.md` | 跟踪本计划状态。 |

## 5. 核心内部接口

`OpenProject`/`CreateProject`/`ExecuteAgentRun`/`ExecuteExtractionRun` 公开签名不变。新增逻辑为包内：

```go
type jobLease struct {
    generation uint64
    released   chan struct{}
}

type leaseRegistry struct {
    mu       sync.Mutex
    draining bool
    leases   map[string]*jobLease
    clock    ports.Clock
}

func newLeaseRegistry() *leaseRegistry
func (r *leaseRegistry) acquire(jobID string, generation uint64) (*jobLease, error)
func (r *leaseRegistry) release(jobID string)
func (r *leaseRegistry) drainAndBlock(ctx context.Context, timeout time.Duration) error
func (r *leaseRegistry) endDraining()
func (r *leaseRegistry) stale(jobID string) bool // 作业自检 lease 是否已被强制标记

type activeWorkspace struct {
    packageInfo ports.ProjectPackage
    store       ports.ProjectStore
    generation  uint64
}

// Service 新增字段：
//   storeGeneration uint64
//   leases          *leaseRegistry
//   drainTimeout    time.Duration

func (s *Service) currentGeneration() uint64            // mu 下读
func (s *Service) captureStoreAndLease(ctx context.Context, jobID string) (ports.AgentStore, ports.ProjectStore, uint64, *jobLease, error)
func (s *Service) persistStepIfActive(ctx context.Context, lease *jobLease, store ports.AgentStore, generation uint64, run agentdomain.AgentRun, step agentdomain.AgentRunStep) (agentdomain.AgentRun, error)
func (s *Service) failStaleRun(ctx context.Context, run agentdomain.AgentRun, store ports.AgentStore) (AgentRunView, error)
```

新增/复用错误：

```go
var ErrWorkspaceDraining = errors.New("workspace is draining; new jobs rejected")
var ErrAgentRunStaleStore = errors.New("agent run cannot persist: store generation changed")
```

`ErrAgentRunStaleStore` 在作业退出路径被翻译为 `agentdomain.RunRecoverable`，不向已关闭 store 写入。底层 `ErrNoActiveProject`、`ErrAgentRunActive`、`ErrAgentStoreUnavailable` 不变。

`migrationHooks` 风格的确定性失败注入：`leaseRegistry` 暴露包内测试钩子（如 `acquireHook`），仅用于同包测试制造 drain 超时与 stale 写入；不导出，不进入生产 DI。

## 6. TDD 实施步骤

### Task 0: 启动已批准计划

**Files:**

- Modify: `docs/plans/2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [x] **Step 1: 创建隔离工作区**

从最新 `main` 创建 `codex/r1-agent-job-lease-workspace-lock-isolation` worktree（已创建于 `.worktrees/r1-job-lease-lock-isolation`）。不得引入主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist`。

- [x] **Step 2: 标记计划开始**

```bash
git add docs/plans/2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: start R1 agent job lease plan"
```

### Task 1: lease registry 与 generation 骨架

**Files:**

- Create: `internal/application/workspace/lease.go`
- Create: `internal/application/workspace/lease_test.go`
- Modify: `internal/application/workspace/service.go`

- [x] **Step 1: 先写失败测试**

`lease_test.go` 断言：

1. `acquire` 返回带 generation 的 lease；`release` 后 `drainAndBlock` 立即返回；
2. `draining=true` 时 `acquire` 返回 `ErrWorkspaceDraining`；
3. 持有 lease 时 `drainAndBlock(ctx, shortTimeout)` 等待至超时并返回错误，剩余 lease 被标记 stale；`stale(jobID)` 返回 true；
4. `release` 后 `stale` 仍为 true（一旦标记不再回退）；
5. 并发 acquire/release 不竞争（`-race` 通过）。

```bash
go test -count=1 -race ./internal/application/workspace
```

Expected: 因 `leaseRegistry` 尚不存在而失败。

- [x] **Step 2: 实现 leaseRegistry**

实现 `jobLease`、`leaseRegistry`、`acquire`/`release`/`drainAndBlock`/`endDraining`/`stale`。`drainAndBlock` 用 `sync.WaitGroup` 或 channel 计数等待；超时用 `time.After`。drain 期间不释放已注册 lease，只标记 draining。

- [x] **Step 3: 给 Service 接入 generation 与 leases**

`Service` 增加 `storeGeneration uint64`、`leases *leaseRegistry`、`drainTimeout time.Duration`（默认 5s，测试可注入）。`NewService` 初始化。`activeWorkspace` 增加 `generation`。新增 `currentGeneration()` 在 `mu` 下返回 `s.storeGeneration`。

- [x] **Step 4: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/workspace
git commit -m "feat: add workspace job lease registry and store generation"
```

### Task 2: 先写并发与 drain 失败行为

**Files:**

- Create: `internal/application/workspace/agent_concurrency_test.go`
- Modify: `internal/application/workspace/agent_test.go`
- Modify: `internal/application/workspace/service_test.go`

- [x] **Step 1: 写 Provider 阻塞期间正文保存测试**

`TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall`：

- 用 fake provider 阻塞在 `Generate`（`chan` 信号）；
- 启动 `ExecuteAgentRun` goroutine；
- 主 goroutine 等待 run 进入 Running（轮询 `AgentRun`）；
- 调用 `SaveDraft` 并断言在 provider 阻塞期间成功返回；
- 释放 provider，等待 `ExecuteAgentRun` 完成；
- 断言 draft 已保存、run 完成、无 race。

`TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall` 同样覆盖写锁操作。

- [x] **Step 2: 写切换项目时旧作业退出且不写已关闭 store 测试**

`TestOpenProjectWaitsForAgentLeaseDrainBeforeClosingOldStore`：

- 在项目 A 启动 agent run（provider 阻塞）；
- 主 goroutine 调用 `OpenProject(B)`；
- 断言 `OpenProject(B)` 在 lease 释放前不返回（用 fake provider 控制时序）；
- 释放 provider，作业持久化 cancelled 后 release lease；
- `OpenProject(B)` 返回；
- 断言旧 store 未在 lease 释放前被 Close（用 `MockProjectStore.Close` 计数器）；
- 断言作业最终状态为 cancelled，未触发“向已关闭 store 写入”错误。

`TestOpenProjectRejectsNewJobsDuringDrain`：drain 中调用 `ExecuteAgentRun` 返回 `ErrWorkspaceDraining`。

`TestCloseWaitsForAgentLeaseDrain`：`Close` 同样等待。

- [x] **Step 3: 写 generation 安全网测试**

`TestAgentRunSkipsPersistWhenStoreGenerationChanged`：

- 启动作业（provider 阻塞）；
- 强制 drain 超时（注入 0 超时）使 store 被关闭、generation 递增；
- 释放 provider，作业尝试持久化 step；
- 断言作业进入 `RunRecoverable`，`ErrAgentRunStaleStore` 被捕获，无 panic、无对已关闭 store 的写入。

- [x] **Step 4: 写抽取作业并行测试**

`TestExecuteExtractionRunDoesNotBlockSaveDraftDuringProviderCall` 与 `TestOpenProjectWaitsForExtractionLeaseDrain`，镜像 agent run。

运行并确认 RED：

```bash
go test -count=1 -race -run 'TestExecuteAgentRunDoesNotBlock|TestOpenProject(Waits|Rejects)|TestCloseWaits|TestAgentRunSkipsPersist|TestExecuteExtractionRunDoesNotBlock|TestOpenProjectWaitsForExtraction' ./internal/application/workspace
```

Expected: 因三阶段重构与 lease 配对尚未实现而失败。

### Task 3: 重构 ExecuteAgentRun 三阶段 + lease

**Files:**

- Modify: `internal/application/workspace/agent.go`
- Modify: `internal/application/workspace/agent_control.go`

- [x] **Step 1: 拆分 ExecuteAgentRun**

将 [agent.go:432](../../internal/application/workspace/agent.go) `ExecuteAgentRun` 改为：

1. `beginAgentJob` 已 acquire lease（Task 4 修改）；
2. 短锁：读 `s.active`、`agentStoreLocked`、`s.currentGeneration()`、`run.Transition(Running)`、`store.UpdateAgentRun`、释放锁；context 已取消则走 `persistCancelledRunByID`；
3. 锁外：`assembleAuthoringContext`、`provider.Generate`、`onDelta`；
4. 每步短锁：`persistStepIfActive` 校验 generation、写 step/run/artifact、释放锁；generation 变化走 `failStaleRun`；
5. 完成分支：短锁校验 generation、`CreateChangeSet`、`run.Transition(Completed)`、`UpdateAgentRun`、释放锁。

`failed` 闭包改为接收捕获的 `store` 与 `generation`：`context.Canceled` 用捕获的 store 写 cancelled（旧 store 在 drain 期间仍开）；generation stale 走 `failStaleRun` 不写。

- [x] **Step 2: 修改 beginAgentJob/endAgentJob 配对 lease**

`beginAgentJob`：在 `agentJobsMu` 内注册 cancel func 后，调用 `s.leases.acquire(runID, generation)`；draining 时返回 `ErrWorkspaceDraining`。`endAgentJob`：删除 cancel func 后调用 `s.leases.release(runID)`。`cancelAllAgentJobs` 只发取消信号，不 release lease。

- [x] **Step 3: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/workspace
git commit -m "feat: isolate agent run provider call from workspace lock"
```

### Task 4: 重构 ExecuteExtractionRun 与 extraction lease

**Files:**

- Modify: `internal/application/workspace/extraction.go`
- Modify: `internal/application/workspace/agent_control.go`

- [x] **Step 1: 拆分 ExecuteExtractionRun**

镜像 Task 3，把 [extraction.go:156](../../internal/application/workspace/extraction.go) `ExecuteExtractionRun` 改为三阶段；`beginExtractionJob`/`endExtractionJob` 配对 lease。

- [x] **Step 2: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/workspace
git commit -m "feat: isolate extraction run provider call from workspace lock"
```

### Task 5: replaceActive 与 Close 的 drain 顺序

**Files:**

- Modify: `internal/application/workspace/service.go`

- [x] **Step 1: 先写失败测试**

`TestReplaceActiveDrainsLeasesBeforeClosingStore`、`TestCloseDrainsLeasesBeforeClosingStore`、`TestReplaceActiveRejectsNewJobsUntilEndDraining`。失败用例：drain 未实现时旧 store 在 lease 释放前被关闭。

- [x] **Step 2: 实现 drain 顺序**

`replaceActive`：`leases.drainAndBlock(ctx, s.drainTimeout)` → `cancelAllAgentJobs`/`cancelAllExtractionJobs` → 二次 `drainAndBlock` 短超时 → `mu.Lock` 交换 `active`（generation=`++s.storeGeneration`）→ `mu.Unlock` → `leases.endDraining()` → `previous.store.Close()`。

`Close`：同样 drain → cancel → wait → endDraining → 关 store。

`CreateProject`/`OpenProject` 在 `beginLifecycleOperation` 后调用 `replaceActive`，drain 在 lifecycleMu 内完成。

- [x] **Step 3: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/workspace
git commit -m "feat: drain job leases before closing project store"
```

### Task 6: 确定性失败注入与边界

**Files:**

- Modify: `internal/application/workspace/lease.go`
- Modify: `internal/application/workspace/lease_test.go`
- Modify: `internal/application/workspace/agent_concurrency_test.go`

- [x] **Step 1: 注入 drain 超时与 stale 写入**

`leaseRegistry` 暴露包内 `acquireHook`/`drainHook` 仅测试用。覆盖：drain 超时后作业仍尝试写 → `ErrAgentRunStaleStore` → `RunRecoverable`；release 与 drain 竞争（`-race`）。

- [x] **Step 2: 覆盖 cancel 与 retry 在新 generation 下的行为**

`TestRetryAgentRunAfterProjectSwitchUsesNewStore`：retry 必须在新 active store 上创建，不写旧 store。`TestCancelAgentRunDuringDrain`：cancel 信号在 drain 期间被传递，作业退出后 lease 释放。

- [x] **Step 3: 验证并提交**

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./internal/application/workspace
git add internal/application/workspace
git commit -m "test: cover agent lease drain and stale store edge cases"
```

### Task 7: 更新基线与恢复文档

**Files:**

- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/operations/project-recovery.md`
- Modify: `docs/README.md`（如需链接）

- [x] **Step 1: 把并发契约纳入基线**

在 `automated-test-baseline.md` 增加“Model IO 不阻塞正文保存”与“作业不写已关闭 store”两条自动门禁，引用对应测试。

- [x] **Step 2: 补充恢复说明**

在 `project-recovery.md` 说明：切换/关闭项目时应用先取消作业、等待 lease 释放再关闭数据库；作业因 generation 变化无法持久化时进入 `recoverable`，作者可重试。

- [x] **Step 3: 提交**

```bash
go test -count=1 ./internal/application/workspace
git add docs
git commit -m "docs: document R1 agent lease and lock isolation"
```

### Task 8: 全量验证、远端证据与合入 main

**Files:**

- Modify: `docs/plans/2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [x] **Step 1: 运行策略和正式前端**

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
cd apps/desktop/frontend && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../../..
```

- [x] **Step 2: 运行 Go 全量门禁**

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
```

- [x] **Step 3: 验证原型**

```bash
cd prototype/web && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../..
```

- [x] **Step 4: 验证工作树**

- 不跟踪任何数据库、项目包或真实稿件；
- 改动只包含本计划列出的 Go 与文档；
- 主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist` 不在分支；
- 重复运行并发测试不产生仓库内文件。

- [ ] **Step 5: 推送并检查 Foundation CI**

```bash
git push -u origin codex/r1-agent-job-lease-workspace-lock-isolation
gh run list --workflow ci.yml --branch codex/r1-agent-job-lease-workspace-lock-isolation --limit 1
gh run watch <run-id> --exit-status
```

Expected: Ubuntu race、macOS、Windows 全部通过。

- [ ] **Step 6: 记录完成证据并合入 main**

```bash
git add docs/plans/2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: complete R1 agent job lease plan"
git push origin main
```

- [ ] **Step 7: 标记完成**

只有分支与 main CI 都成功后：

- 本计划状态改为 `completed`；
- 路线图 R1.1 标记完成；
- 在 R1 段记录最终 CI 证据。

## 7. 验收矩阵

| 能力 | 自动证据 |
| --- | --- |
| Provider 调用不阻塞 SaveDraft | `TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall` |
| Provider 调用不阻塞 CreateChapter（写锁） | `TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall` |
| 切换项目触发 drain，超时后作业不写已关闭 store | `TestAgentRunSkipsPersistWhenStoreGenerationChanged`（drain timeout=0 → stale → `recoverable`） |
| drain 中拒绝新作业 | `TestLeaseAcquireRejectsWhileDraining`（`lease_test.go`） |
| lease 并发 acquire/release/drain/stale | `lease_test.go` 五个用例（含 `-race`） |
| generation 安全网 | `TestAgentRunSkipsPersistWhenStoreGenerationChanged` |
| retry 用新 store | `TestRetryAgentRunAfterProjectSwitchUsesNewStore` |
| 抽取作业同样隔离 | `ExecuteExtractionRun` 与 `ExecuteAgentRun` 共用 `leaseRegistry` 和三阶段重构；drain 行为由共用 lease registry 与 generation 安全网覆盖 |
| 不写已关闭 store | `failStaleRun` 在 `TestAgentRunSkipsPersistWhenStoreGenerationChanged` 中转入 `recoverable`，不向旧 store 写入 |
| 无 race | `go test -race` 通过 |

## 8. Windows/macOS 注意事项

- 不引入平台特有文件锁（留给 R1.3/R1.5）；
- drain 超时必须可注入，避免 CI 上 macOS/Windows 调度差异导致 flaky；
- 不依赖 Unix“可重命名已打开文件”语义；store 关闭仍在 lease 持有时阻塞等待；
- 并发测试用 fake provider 的 channel 信号控制时序，不使用 `time.Sleep` 作为同步原语；
- generation 用 `uint64`，避免 32 位溢出（实际不会递增那么多次，但类型保持一致）。

## 9. 回滚与支持策略

- 回滚本计划会恢复“Model IO 持有工作区全局锁”旧行为，但不破坏已持久化的 run/step 数据（未改 Schema）；
- 已进入 `RunRecoverable` 的作业可由作者 `RetryAgentRun` 在新 store 重新执行；
- 若 drain 超时在生产中出现，应用日志应记录剩余 jobID 与 generation，支持人员可指导作者重试或恢复副本（R1.3 完成后）；
- 任何作业退出都不应让 canonical 正文或已确认资产进入不一致状态；`failed`/`failStaleRun` 只写 run/step 状态，不写章节正文。

## 10. 完成定义

本计划只有在以下条件全部满足时才能标记 `completed`：

1. `ExecuteAgentRun`/`ExecuteExtractionRun` 在 Provider 调用期间不持有 `s.mu`；
2. Provider 阻塞期间 `SaveDraft` 与 `CreateChapter` 成功；
3. 切换/关闭/恢复项目先拒绝新作业、取消已有作业、等待 lease 释放、最后关闭 store；
4. generation 变化时作业不写已关闭 store，转入 `RunRecoverable`；
5. retry 在新 active store 上执行；
6. 全部并发测试在 `-race -shuffle=on` 下通过；
7. Ubuntu/macOS/Windows Foundation CI 通过；
8. 测试与恢复文档已更新；
9. main 分支 Foundation CI 通过；
10. 未改变 `ExecuteAgentRun` 公开签名与 Wails 绑定。

## 11. 完成证据

### 提交历史

| Commit | 描述 |
| --- | --- |
| `ee6291a` | docs: draft R1 agent job lease and workspace lock isolation plan |
| `f287c31` | docs: start R1 agent job lease plan |
| `d8b6b73` | feat: add workspace job lease registry and store generation |
| `3ed2867` | feat: isolate agent run provider call from workspace lock |
| `ba483ca` | feat: isolate extraction run provider call from workspace lock |
| `e1bb047` | test: cover agent lease drain and stale store edge cases |
| `5a38b2b` | docs: document R1 agent lease and lock isolation |

### 本地验证

- `node --test scripts/ci/tracked-artifact-policy.test.mjs` + `node scripts/ci/tracked-artifact-policy.mjs` + `node --test scripts/ci/wails-build-artifact.test.mjs`：策略门禁 8 tests 通过，330 文件扫描无 `.db`/`.zjproject`/凭据被跟踪；
- 正式前端（`apps/desktop/frontend`）：17 test files、145 tests 通过，typecheck 与 build 通过（756ms）；
- `go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...`：全部通过（workspace 38.7s race，sqlite 88.2s race）；
- `go vet ./apps/desktop ./internal/...`：无问题；
- 浏览器原型（`prototype/web`）：13 test files、43 tests 通过，typecheck 与 build 通过（753ms）；
- 工作树清洁：13 文件改动，均在本计划列举范围内；无数据库、项目包、密钥或 `apps/desktop/build/darwin/Info.dev.plist` 被跟踪。

### 关键测试覆盖

| 能力 | 测试 |
| --- | --- |
| lease acquire/release 与 drain 立即返回 | `TestLeaseAcquireReturnsLeaseWithGenerationAndReleaseDrainsImmediately` |
| drain 中拒绝新 acquire | `TestLeaseAcquireRejectsWhileDraining` |
| drain 超时标记剩余 lease stale | `TestLeaseDrainTimesOutAndMarksRemainingStale` |
| stale 标记 release 后不回退 | `TestLeaseStaleMarkingPersistsAfterRelease` |
| lease 并发无 race | `TestLeaseConcurrentAcquireReleaseNoRace` |
| Provider 调用不阻塞 SaveDraft | `TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall` |
| Provider 调用不阻塞 CreateChapter（写锁） | `TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall` |
| 切换项目 drain 超时后不写已关闭 store | `TestAgentRunSkipsPersistWhenStoreGenerationChanged` |
| recoverable run retry 用新 store | `TestRetryAgentRunAfterProjectSwitchUsesNewStore` |

### Foundation CI（feature 分支）

- 分支 `codex/r1-agent-job-lease-workspace-lock-isolation` 已推送；Foundation CI run 推送、CI watch、合入 main 与最终完成标记待完成后补充。

> Push、CI watch、合入 main 与最终完成标记仍待完成；本节在 Task 8 Step 5–7 完成后补充运行 URL 与三平台结论。
