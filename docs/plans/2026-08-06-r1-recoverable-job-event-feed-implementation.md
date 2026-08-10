# R1 可恢复 JobEvent Feed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Agent 与抽取作业建立 append-only 的 `JobEvent` 事件源和可重建的 `JobSnapshot` 快照，使作业状态变化与事件在同一短事务提交；长作业在持久化后立即返回 `job_id` 并在后台执行，Wails 只发送 `job_id + latest_sequence` 唤醒通知；React 按 cursor 增量补拉、检测缺口并幂等更新，覆盖重挂载、重复通知、漏推、应用重启和事件清理后的 snapshot 重建。

**Architecture:** 新增 `job_events`（append-only，`(job_id, sequence_number)` 唯一）和 `job_snapshots`（权威投影）表。`ExecuteAgentRun`/`ExecuteExtractionRun` 改为异步：在短锁内持久化 running 状态与首个 `job_started` 事件后立即返回，Provider 流水线在受 lease/generation 保护的 goroutine 中执行；每个状态/步骤变化在短事务内同时写入 run/step 行、append JobEvent、刷新 JobSnapshot，随后通过 `JobEventNotifier` 发出轻量 Wails 通知。Go 暴露 `GetJobSnapshot`、`GetJobEvents(afterSequence, limit)`（返回 `next_cursor`/`has_more`/`snapshot_required`）与 `ListRecoverableJobs`。React 维护 per-job cursor，收到通知或重挂载时增量补拉，reducer 按 `(job_id, sequence_number)` 幂等应用；cursor 早于事件下界时先用 snapshot 重建。

**Tech Stack:** Go 1.25、SQLite（WAL、外键、事务）、Goose SQL 迁移、Wails v2 `runtime.EventsEmit`、React、TypeScript、Vite。复用 R1.1 的 `leaseRegistry` 与 `storeGeneration`，不改变其公开行为。

---

- 状态：`completed`
- 路线图阶段：[R1：长任务与项目可靠性](../roadmap/development-roadmap.md#6-r1长任务与项目可靠性)
- 上游依据：[产品需求 7.1](../product/requirements.md)、[产品与交互设计：运行画布与恢复](../product/design.md)、[系统设计 4.3/12](../architecture/system-design.md)、[ADR-0002](../architecture/0002-go-first-wails-stack.md)
- 前置计划：[R1 Agent job lease 与工作区锁隔离](2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md)（`completed`）
- 同阶段后续：`R1 React JobEvent replay`（若本计划过大可拆分）、`R1 项目 OS lease 与只读 store`、`R1 Windows 文件锁与路径边界`

## 1. 当前基线与已知缺口

当前代码已经具备：

- `agent_runs`/`agent_run_steps`/`agent_artifacts` 与 `extraction_tasks`/`extraction_steps`/`extraction_artifacts` 表，记录作业、步骤和工件；
- R1.1 的 `leaseRegistry`（drain/stale）与 `storeGeneration`（安全网），`ExecuteAgentRun`/`ExecuteExtractionRun` 已三阶段重构，Provider 调用不持有工作区锁；
- `RecoverInterruptedAgentRuns`/`RecoverInterruptedTasks` 在项目打开时把 running 标记为 recoverable；
- Wails `DesktopAPI` 持有 `ctx`，`runtime.EventsEmit` 已用于 `beforeClose` 事件；
- 前端 `StudioClient` 接口有 `executeAgentRun`/`executeExtractionRun`（同步 await）、`agentRuns`/`agentRun`/`extractionTasks`/`extractionTask`（轮询读）。

当前缺口（对照系统设计 4.3/12 与需求 7.1）：

1. 没有 `JobEvent` 表与单调 sequence。作业状态变化只持久化到 `agent_runs`/`extraction_tasks` 行，没有 append-only 事件源；前端无法增量补拉，只能全量轮询。
2. 没有 `JobSnapshot`。前端没有权威快照可用于首次挂载或事件清理后重建。
3. `ExecuteAgentRun`/`ExecuteExtractionRun` 是同步阻塞调用。Wails binding `ExecuteAgentRun(id)` 阻塞到 Provider 流水线完成；若页面卸载或 Bridge 重挂载，调用可能被取消或孤立，违反“后台执行不依赖调用页面继续存活”。
4. 没有 Wails Event 通知。前端只能轮询 `agentRun(id)`，无法在状态变化时及时刷新。
5. 前端没有 cursor、gap 检测或幂等 reducer。重复通知或漏推会导致状态不一致。
6. 没有 `ListRecoverableJobs`。项目打开时只能分别查询 agent/extraction 列表，没有统一的“非终态及待确认作业”入口。

## 2. 设计选择

### 2.1 统一 JobEvent 模型

Agent run 与 extraction task 都是“作业”。`job_events` 表以 `job_id` + `job_kind`（`'agent'` | `'extraction'`）标识归属；`job_id` 引用 `agent_runs.id` 或 `extraction_tasks.id`（不强制外键，因为两种表）。`sequence_number` 是 per-job 单调递增整数（从 1 开始），`(job_id, sequence_number)` 唯一。sequence 分配在短事务内完成，保证单调。

### 2.2 JobSnapshot 投影

`job_snapshots` 表存储从作业、步骤和累计用量投影出的权威快照：`status`、`max_sequence`、`earliest_replayable_sequence`、累计 token 用量、待确认动作和终态信息。每次 append JobEvent 时在同一事务内 upsert JobSnapshot。`earliest_replayable_sequence` 初始等于 1；事件清理时递增，并把被清理的终态折叠进 snapshot。R1.2 实现投影和 `earliest_replayable_sequence` 字段，但实际清理策略只做最小实现（保留全部事件，标记下界），自动清理留到后续迭代。

### 2.3 同事务不变量

作业状态/步骤变化与对应 JobEvent 必须在同一短事务提交。实现方式：在 `AgentStore`/`ExtractionStore` 接口新增 `AppendJobEvent(ctx, tx-like, event)` 与 `UpsertJobSnapshot(ctx, snapshot)`，或在现有 `UpdateAgentRun`/`UpdateAgentRunStep` 内部以同事务追加事件。选择后者以减少事务嵌套：`UpdateAgentRunStep` 接受可选 `jobEvents []JobEvent` 参数，在同一 `BeginTx` 内写入 step 行和事件行。`UpdateAgentRun` 同理。

### 2.4 异步执行

`ExecuteAgentRun`/`ExecuteExtractionRun` 改为异步：

1. 短锁内：捕获 store/generation、acquire lease、`run.Transition(Running)`、`store.UpdateAgentRun`（同事务 append `job_started` 事件 + upsert snapshot）、释放锁；
2. 立即返回 `AgentRunView`（status=running）给调用方；
3. 在受 lease/generation 保护的 goroutine 中执行 Provider 流水线，每个步骤变化在短锁内持久化并 append 事件；
4. goroutine 退出时 release lease（复用 R1.1 的 `endAgentJob`）。

goroutine 使用 `Service` 持有的 `context.Context`（不绑定 Wails 请求 ctx），在 `Close`/`replaceActive` drain 时被取消。`CancelAgentRun` 通过 `agentJobs` cancel func 取消 goroutine。

### 2.5 JobEventNotifier 端口

新增 `ports.JobEventNotifier` 接口：

```go
type JobEventNotifier interface {
    NotifyJobUpdated(jobID string, jobKind JobKind, latestSequence uint64)
}
```

`Service` 持有 `notifier JobEventNotifier`。每次 commit JobEvent 后调用 `notifier.NotifyJobUpdated`。Wails 层实现 `wailsJobEventNotifier{ctx, emit func}`，用 `runtime.EventsEmit(ctx, "job:updated", {jobID, jobKind, latestSequence})`。测试用 fake notifier 或 noop。通知允许丢失/合并/重复——前端以 cursor 补拉为准。

### 2.6 Cursor API

Go 暴露三个查询：

- `GetJobSnapshot(ctx, jobID) (JobSnapshotView, error)`：返回权威状态、步骤摘要、累计用量、待确认动作、max_sequence、earliest_replayable_sequence。
- `GetJobEvents(ctx, jobID, afterSequence, limit) (JobEventsPage, error)`：按 sequence 升序返回事件、`next_cursor`（最后一条 sequence）、`has_more`、`snapshot_required`（当 afterSequence < earliest_replayable_sequence 时为 true，events 为空）。
- `ListRecoverableJobs(ctx) ([]JobSummaryView, error)`：返回非终态（running/queued/recoverable）及待确认作业摘要。

### 2.7 React cursor reducer

前端维护 per-job `cursor`（最后已应用 sequence）。收到 Wails `job:updated` 事件、组件重 Mount、HMR 或窗口恢复时，调用 `getJobEvents(jobID, cursor, limit)`：

- 若 `snapshot_required`：先 `getJobSnapshot`，用快照重建状态，`cursor = snapshot.max_sequence`，结束本轮；
- 否则：按 sequence 升序应用事件，reducer 按 `(job_id, sequence_number)` 幂等——已应用的事件跳过；遇到 `sequence > cursor + 1` 的缺口时停止应用后续事件，用缺口前一条作为 cursor 再次补拉；
- `has_more` 为 true 时继续翻页，直到追上 `max_sequence`。

不得根据事件到达次数累计 token、费用或章节数；累计值只来自 snapshot 或事件 payload 中的权威字段。

### 2.8 不引入的

- 不实现事件自动清理、保留期策略或 blob 回收（只实现 `earliest_replayable_sequence` 字段与 `snapshot_required` 语义，保留全部事件）；
- 不改变 R1.1 的 lease/drain/generation 行为；
- 不改变 `CreateChapterAgentRun`/`StartExtractionRun` 签名（仍同步返回 queued run）；
- 不实现 `needs_confirmation` 暂停态（留给 R4 批次检查点）；
- 不改变 Provider 接口或流式 delta 机制；
- 不引入项目级 OS 锁或只读 store（R1.3）。

## 3. 明确不包含

- 不自动清理或删除 JobEvent 行（只定义 `earliest_replayable_sequence` 和 `snapshot_required` 语义）；
- 不改变数据库连接策略或 PRAGMA；
- 不实现 `BatchCheckpoint`/`GenerationAttempt`/`needs_confirmation`（R4）；
- 不改变 `AgentRun`/`AgentRunStep`/`ExtractionTask`/`ExtractionStep` 域模型的核心字段（只在 store 层追加事件写入）；
- 不实现 Windows 文件锁或长路径边界（R1.5）；
- 不改变快照/分支恢复的 journal+stage+backup 模式。

## 4. 文件级改动

| 文件 | 职责 |
| --- | --- |
| `internal/persistence/sqlite/migrations/00015_job_events.sql` | `job_events` 与 `job_snapshots` 表、索引、CHECK 约束。 |
| `internal/domain/agent/job_event.go` | `JobEvent`、`JobSnapshot`、`JobKind`、`EventType` 域类型与构造函数。 |
| `internal/application/ports/agent_runtime.go` | `AgentStore` 新增 `AppendJobEvent`、`UpsertJobSnapshot`、`JobEvents`、`JobSnapshot`、`ListRecoverableJobs`；`UpdateAgentRun`/`UpdateAgentRunStep` 接受可选事件。 |
| `internal/application/ports/job_notifier.go` | `JobEventNotifier` 接口与 `JobKind`/`JobUpdated` 通知载荷。 |
| `internal/persistence/sqlite/job_event_store.go` | SQLite 实现 JobEvent/JobSnapshot 读写。 |
| `internal/persistence/sqlite/agent_store.go` | `UpdateAgentRun`/`UpdateAgentRunStep` 在同事务 append 事件与 upsert snapshot。 |
| `internal/persistence/sqlite/extraction_store.go` | 同上，extraction 侧。 |
| `internal/application/workspace/agent.go` | `ExecuteAgentRun` 改异步：短锁持久化 + 返回 + goroutine 执行 + 事件 append + 通知。 |
| `internal/application/workspace/extraction.go` | `ExecuteExtractionRun` 同步改异步。 |
| `internal/application/workspace/service.go` | `Service` 注入 `JobEventNotifier`；新增 `GetJobSnapshot`/`GetJobEvents`/`ListRecoverableJobs`；`RecoverInterruptedAgentRuns`/`RecoverInterruptedTasks` 在恢复时 append `job_recovered` 事件。 |
| `internal/application/workspace/job_event_test.go` | JobEvent 同事务、sequence 单调、cursor 补拉、snapshot 重建测试。 |
| `internal/application/workspace/agent_test.go` | 异步执行返回 running、goroutine 完成后事件齐全测试。 |
| `apps/desktop/api.go` | `DesktopAPI` 新增 `GetJobSnapshot`/`GetJobEvents`/`ListRecoverableJobs`；`ExecuteAgentRun`/`ExecuteExtractionRun` 返回 running view。 |
| `apps/desktop/job_notifier.go` | `wailsJobEventNotifier` 实现，`runtime.EventsEmit(ctx, "job:updated", payload)`。 |
| `apps/desktop/main.go` | 注入 `wailsJobEventNotifier` 到 `Service`。 |
| `apps/desktop/frontend/src/lib/studio-client.ts` | 新增 `JobEventView`/`JobSnapshotView`/`JobEventsPage` 类型与 `getJobSnapshot`/`getJobEvents`/`listRecoverableJobs`/`onJobUpdated` 方法。 |
| `apps/desktop/frontend/src/lib/wails-studio-client.ts` | 实现 Wails 绑定 + `EventsOn("job:updated")` 订阅。 |
| `apps/desktop/frontend/src/lib/fake-studio-client.ts` | fake 实现，支持事件序列回放测试。 |
| `apps/desktop/frontend/src/features/agents/jobEventReducer.ts` | 幂等 reducer、gap 检测、snapshot 重建。 |
| `apps/desktop/frontend/src/features/agents/AgentWorkbench.tsx` | 改用 cursor 补拉替代同步 await。 |
| `docs/testing/automated-test-baseline.md` | 把 JobEvent cursor 补拉与 snapshot 重建纳入基线。 |
| `docs/operations/project-recovery.md` | 补充 JobEvent 恢复与 cursor 语义说明。 |
| `docs/roadmap/development-roadmap.md` | 跟踪本计划状态。 |

## 5. 核心内部接口

```go
// internal/domain/agent/job_event.go
type JobKind string
const (
    JobKindAgent      JobKind = "agent"
    JobKindExtraction JobKind = "extraction"
)

type JobEventType string
const (
    EventJobStarted   JobEventType = "job_started"
    EventStepStarted  JobEventType = "step_started"
    EventStepDelta    JobEventType = "step_delta"    // 轻量增量指针，大文本进 artifact
    EventStepCompleted JobEventType = "step_completed"
    EventStepFailed   JobEventType = "step_failed"
    EventJobCompleted JobEventType = "job_completed"
    EventJobFailed    JobEventType = "job_failed"
    EventJobCancelled JobEventType = "job_cancelled"
    EventJobRecovered JobEventType = "job_recovered"
)

type JobEvent struct {
    JobID          string
    JobKind        JobKind
    SequenceNumber uint64
    EventType      JobEventType
    StepID         string // 可空
    Payload        string // 轻量 JSON；大文本引用 artifact，不复制正文
    CreatedAt      time.Time
}

type JobSnapshot struct {
    JobID                      string
    JobKind                    JobKind
    ProjectID                  string
    Status                     string
    MaxSequence                uint64
    EarliestReplayableSequence uint64
    InputTokens                int
    OutputTokens               int
    PendingAction              string // 可空
    TerminalInfo               string // JSON，终态时填
    UpdatedAt                  time.Time
}
```

```go
// ports/agent_runtime.go 扩展
type AgentStore interface {
    // ... 现有方法不变 ...
    AppendJobEvent(ctx context.Context, event agentdomain.JobEvent) error
    UpsertJobSnapshot(ctx context.Context, snapshot agentdomain.JobSnapshot) error
    JobEvents(ctx context.Context, jobID string, afterSequence uint64, limit int) ([]agentdomain.JobEvent, error)
    JobSnapshot(ctx context.Context, jobID string) (agentdomain.JobSnapshot, error)
    ListRecoverableJobs(ctx context.Context, projectID string) ([]agentdomain.JobSummary, error)
}
```

```go
// ports/job_notifier.go
type JobEventNotifier interface {
    NotifyJobUpdated(jobID string, jobKind agentdomain.JobKind, latestSequence uint64)
}
```

```go
// workspace Service 新增
func (s *Service) GetJobSnapshot(ctx context.Context, jobID string) (JobSnapshotView, error)
func (s *Service) GetJobEvents(ctx context.Context, jobID string, afterSequence uint64, limit int) (JobEventsPageView, error)
func (s *Service) ListRecoverableJobs(ctx context.Context) ([]JobSummaryView, error)
```

新增/复用错误：

```go
var ErrJobSnapshotRequired = errors.New("job events cursor is earlier than replayable lower bound; rebuild from snapshot")
```

## 6. TDD 实施步骤

### Task 0: 启动计划

**Files:**
- Modify: `docs/plans/2026-08-06-r1-recoverable-job-event-feed-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 创建隔离工作区**

从最新 `main` 创建 `codex/r1-recoverable-job-event-feed` worktree。不得引入主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist`。

- [ ] **Step 2: 标记计划开始**

```bash
git add docs/plans/2026-08-06-r1-recoverable-job-event-feed-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: start R1 recoverable job event feed plan"
```

### Task 1: JobEvent/JobSnapshot 域模型与 schema

**Files:**
- Create: `internal/domain/agent/job_event.go`
- Create: `internal/persistence/sqlite/migrations/00015_job_events.sql`
- Create: `internal/domain/agent/job_event_test.go`

- [ ] **Step 1: 先写失败测试**

`job_event_test.go` 断言 `JobEvent` 构造函数校验 jobID/kind/sequence/eventType 非空，sequence >= 1；`JobSnapshot` 构造函数校验 maxSequence >= earliestReplayableSequence >= 1。

- [ ] **Step 2: 实现域类型**

实现 `JobEvent`、`JobSnapshot`、`JobKind`、`JobEventType` 与构造函数。`NewJobEvent` 接受 jobID/kind/sequence/eventType/stepID/payload/now 并 trim/校验。

- [ ] **Step 3: 写迁移**

`00015_job_events.sql`：
- `job_events`：`job_id TEXT NOT NULL`、`job_kind TEXT NOT NULL CHECK IN ('agent','extraction')`、`sequence_number INTEGER NOT NULL CHECK >= 1`、`event_type TEXT NOT NULL`、`step_id TEXT NOT NULL DEFAULT ''`、`payload TEXT NOT NULL DEFAULT ''`、`created_at TEXT NOT NULL`，`UNIQUE(job_id, sequence_number)`，`INDEX(job_id, sequence_number)`；
- `job_snapshots`：`job_id TEXT PRIMARY KEY`、`job_kind`、`project_id`、`status`、`max_sequence`、`earliest_replayable_sequence`、`input_tokens`、`output_tokens`、`pending_action`、`terminal_info`、`updated_at`；
- Down 反向。

- [ ] **Step 4: 验证并提交**

```bash
go test -count=1 -race ./internal/domain/agent
go test -count=1 ./internal/persistence/sqlite
git add internal/domain/agent/job_event.go internal/domain/agent/job_event_test.go internal/persistence/sqlite/migrations/00015_job_events.sql
git commit -m "feat: add job event and snapshot domain model"
```

### Task 2: Store 层 JobEvent/JobSnapshot 读写

**Files:**
- Create: `internal/persistence/sqlite/job_event_store.go`
- Create: `internal/persistence/sqlite/job_event_store_test.go`
- Modify: `internal/application/ports/agent_runtime.go`
- Modify: `internal/application/ports/extraction_store.go`

- [ ] **Step 1: 先写失败测试**

`job_event_store_test.go` 断言：
1. `AppendJobEvent` 写入后 `JobEvents(jobID, 0, 10)` 返回该事件，sequence 升序；
2. 多次 append sequence 递增，`(job_id, sequence)` 唯一约束阻止重复；
3. `UpsertJobSnapshot` 后 `JobSnapshot(jobID)` 返回投影；
4. `JobEvents(afterSequence=N)` 只返回 sequence > N 的事件；
5. `ListRecoverableJobs` 返回 status 为 running/queued/recoverable 的作业（agent + extraction 合并）；
6. `earliest_replayable_sequence` > afterSequence 时 `JobEvents` 返回空且 `snapshot_required`（此语义在 Service 层实现，store 只返回 earliest）。

- [ ] **Step 2: 实现 store**

`job_event_store.go`：`AppendJobEvent`（INSERT，UNIQUE 冲突返回错误）、`UpsertJobSnapshot`（INSERT ON CONFLICT DO UPDATE）、`JobEvents`（SELECT WHERE job_id=? AND sequence_number > ? ORDER BY sequence_number LIMIT ?）、`JobSnapshot`（SELECT）、`ListRecoverableJobs`（UNION agent_runs + extraction_tasks WHERE status IN non-terminal）。

- [ ] **Step 3: 扩展 ports 接口**

`AgentStore` 新增 `AppendJobEvent`/`UpsertJobSnapshot`/`JobEvents`/`JobSnapshot`/`ListRecoverableJobs`。`ExtractionStore` 不重复定义，共享 `AgentStore` 的 JobEvent 方法（或抽到 `JobEventStore` 接口，两者嵌入）。

- [ ] **Step 4: 验证并提交**

```bash
go test -count=1 -race ./internal/persistence/sqlite
git add internal/persistence/sqlite internal/application/ports
git commit -m "feat: persist and query job events and snapshots"
```

### Task 3: 同事务事件 append 到 UpdateAgentRun/UpdateAgentRunStep

**Files:**
- Modify: `internal/persistence/sqlite/agent_store.go`
- Modify: `internal/persistence/sqlite/extraction_store.go`
- Modify: `internal/application/workspace/agent.go`
- Modify: `internal/application/workspace/extraction.go`

- [ ] **Step 1: 先写失败测试**

`job_event_test.go` 断言：`ExecuteAgentRun` 完成后 `JobEvents(runID, 0, 100)` 返回 `job_started`、`step_started(planner)`、`step_completed(planner)`、…、`job_completed` 事件，sequence 连续单调；snapshot 的 max_sequence 等于最后一条事件。Extraction 侧镜像。

- [ ] **Step 2: 在 UpdateAgentRun/UpdateAgentRunStep 同事务 append**

修改 store：`UpdateAgentRun(ctx, run, events []JobEvent)` 与 `UpdateAgentRunStep(ctx, step, events []JobEvent)` 在同一 `BeginTx` 内写入 run/step 行和事件行，并 upsert snapshot。Sequence 分配：在事务内 `SELECT COALESCE(MAX(sequence_number),0)+1 FROM job_events WHERE job_id=?`。

- [ ] **Step 3: 在 ExecuteAgentRun 各阶段构造事件**

Phase 1 短锁：`job_started`；每步开始：`step_started`；每步完成：`step_completed`（payload 含 artifact 摘要，不复制正文）；终态：`job_completed`/`job_failed`/`job_cancelled`。stale 分支：不写事件（store 已关闭），由 `RecoverInterruptedAgentRuns` 在重开时 append `job_recovered`。

- [ ] **Step 4: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/persistence/sqlite internal/application/workspace
git commit -m "feat: append job events in same transaction as run state"
```

### Task 4: 异步执行与 JobEventNotifier

**Files:**
- Create: `internal/application/ports/job_notifier.go`
- Modify: `internal/application/workspace/service.go`
- Modify: `internal/application/workspace/agent.go`
- Modify: `internal/application/workspace/extraction.go`
- Modify: `internal/application/workspace/agent_test.go`

- [ ] **Step 1: 先写失败测试**

`agent_test.go` 断言：
1. `ExecuteAgentRun` 立即返回 status=running 的 view，不阻塞等待 Provider；
2. Provider 完成后 `JobEvents` 含完整序列；
3. `ExecuteAgentRun` 返回后调用 `CancelAgentRun` 能取消后台 goroutine并 append `job_cancelled`；
4. notifier 收到 `NotifyJobUpdated` 调用，载荷含 jobID/kind/latestSequence。

- [ ] **Step 2: 实现 JobEventNotifier 端口与 noop**

`ports/job_notifier.go` 定义接口；`workspace` 包提供 `noopJobEventNotifier{}` 默认实现，`NewService` 默认注入 noop。

- [ ] **Step 3: ExecuteAgentRun 改异步**

Phase 1 短锁：持久化 running + `job_started` + snapshot + acquire lease + notify → 释放锁 → 返回 running view。Phase 2-3 移到 `go func()`，使用 `Service` 持有的 long-lived context（不绑定 Wails 请求 ctx）。goroutine 内每个短锁持久化后调用 `notifier.NotifyJobUpdated`。

- [ ] **Step 4: ExecuteExtractionRun 同步改异步**

镜像 Task 3 Step 3。

- [ ] **Step 5: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/ports internal/application/workspace
git commit -m "feat: run agent and extraction jobs asynchronously with event feed"
```

### Task 5: Cursor 查询 API 与 ListRecoverableJobs

**Files:**
- Modify: `internal/application/workspace/service.go`
- Modify: `internal/application/workspace/job_event_test.go`

- [ ] **Step 1: 先写失败测试**

断言：
1. `GetJobEvents(jobID, 0, 2)` 返回前 2 条 + `hasMore=true` + `nextCursor=2`；
2. `GetJobEvents(jobID, 2, 100)` 返回剩余 + `hasMore=false`；
3. `earliest_replayable_sequence=5` 时 `GetJobEvents(jobID, 3, 10)` 返回 `snapshotRequired=true` + 空 events；
4. `ListRecoverableJobs` 返回非终态作业摘要（agent + extraction）。

- [ ] **Step 2: 实现 Service 查询方法**

`GetJobSnapshot`、`GetJobEvents`（调用 store，计算 `snapshotRequired` = afterSequence < earliest）、`ListRecoverableJobs`。在短锁内读 store 引用，锁外执行查询。

- [ ] **Step 3: 验证并提交**

```bash
go test -count=1 -race ./internal/application/workspace
git add internal/application/workspace
git commit -m "feat: expose cursor-based job event and snapshot queries"
```

### Task 6: Wails 绑定与事件通知

**Files:**
- Create: `apps/desktop/job_notifier.go`
- Modify: `apps/desktop/api.go`
- Modify: `apps/desktop/main.go`

- [ ] **Step 1: 实现 wailsJobEventNotifier**

`job_notifier.go`：`wailsJobEventNotifier{ctx, emit func(ctx, name string, data ...any)}`，`NotifyJobUpdated` 调用 `emit(ctx, "job:updated", {jobID, jobKind, latestSequence})`。`emit` 默认为 `runtime.EventsEmit`，测试可注入 fake。

- [ ] **Step 2: 扩展 DesktopAPI 与 WorkspaceUseCases**

`WorkspaceUseCases` 新增 `GetJobSnapshot`/`GetJobEvents`/`ListRecoverableJobs`。`DesktopAPI` 暴露同名方法。`main.go` 把 `wailsJobEventNotifier` 注入 `Service`（通过 `Service.SetJobEventNotifier` 或 `NewService` 选项）。

- [ ] **Step 3: 验证并提交**

```bash
go test -count=1 ./apps/desktop
go vet ./apps/desktop
git add apps/desktop
git commit -m "feat: emit wails job updated events and expose cursor bindings"
```

### Task 7: React cursor reducer 与 workbench 改造

**Files:**
- Modify: `apps/desktop/frontend/src/lib/studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/wails-studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/fake-studio-client.ts`
- Create: `apps/desktop/frontend/src/features/agents/jobEventReducer.ts`
- Create: `apps/desktop/frontend/src/features/agents/jobEventReducer.test.ts`
- Modify: `apps/desktop/frontend/src/features/agents/AgentWorkbench.tsx`
- Modify: `apps/desktop/frontend/src/features/extraction/ExtractionWorkbench.tsx`

- [ ] **Step 1: 先写 reducer 单元测试**

`jobEventReducer.test.ts` 覆盖：
1. 按 sequence 升序应用事件，幂等（重复应用同 sequence 跳过）；
2. 缺口检测：`cursor=2` 收到 sequence=4 时停止应用，返回 needsPull=true + gapCursor=2；
3. `snapshotRequired` 时用 snapshot 重建，cursor 重置为 max_sequence；
4. `hasMore` 翻页直到追上。

- [ ] **Step 2: 实现 reducer**

`jobEventReducer.ts`：纯函数 `applyEvents(state, events) -> {state, cursor, needsPull, snapshotRequired}`。state 包含 status、steps、累计 token。

- [ ] **Step 3: 扩展 StudioClient 类型与方法**

`studio-client.ts` 新增 `JobEventView`/`JobSnapshotView`/`JobEventsPage`/`JobSummaryView` 类型与 `getJobSnapshot`/`getJobEvents`/`listRecoverableJobs`/`onJobUpdated(handler)` 方法。`wails-studio-client.ts` 实现：`EventsOn("job:updated")` → 调用 handler；`getJobEvents` 调用 `GetJobEvents` binding。

- [ ] **Step 4: 改造 AgentWorkbench/ExtractionWorkbench**

`executeAgentRun` 不再 await 完成——只拿 running view，随后通过 `onJobUpdated` 触发 `getJobEvents` 补拉，reducer 更新状态。组件卸载/重挂载时从 `listRecoverableJobs` 恢复 cursor。

- [ ] **Step 5: 验证并提交**

```bash
cd apps/desktop/frontend && npm test -- --run && npm run typecheck && npm run build && cd ../../..
git add apps/desktop/frontend
git commit -m "feat: replay job events via cursor in react workbench"
```

### Task 8: 恢复与边界测试

**Files:**
- Modify: `internal/application/workspace/job_event_test.go`
- Modify: `apps/desktop/frontend/src/features/agents/jobEventReducer.test.ts`

- [ ] **Step 1: 漏推/重复/乱序防护测试**

Go 侧：Provider 完成但 notifier 丢失（noop）→ 前端 `GetJobEvents` 仍能补拉全量；同一事件重复 notify → 幂等。React 侧：reducer 重复应用同 sequence 不累计 token。

- [ ] **Step 2: Bridge 重挂载与应用重启测试**

Go 侧：作业 running 时模拟 store 重开 → `RecoverInterruptedAgentRuns` append `job_recovered` → `ListRecoverableJobs` 返回该作业 → `GetJobSnapshot` 返回 recoverable 状态。React 侧：组件重 Mount → `listRecoverableJobs` → `getJobSnapshot` → 重建 cursor。

- [ ] **Step 3: cursor 早于清理下界测试**

设 `earliest_replayable_sequence=5`，`GetJobEvents(afterSequence=3)` 返回 `snapshotRequired=true`；前端用 snapshot 重建后 cursor=5，继续补拉。

- [ ] **Step 4: 异步 goroutine 在项目切换时安全退出**

复用 R1.1 的 sticky provider 测试：drain 超时后 goroutine 进入 stale 分支，不写事件到旧 store；重开项目后 `job_recovered` 事件写入新 store。

- [ ] **Step 5: 验证并提交**

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./internal/application/workspace
cd apps/desktop/frontend && npm test -- --run && cd ../../..
git add internal/application/workspace apps/desktop/frontend
git commit -m "test: cover job event cursor replay and recovery edge cases"
```

### Task 9: 更新基线与恢复文档

**Files:**
- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/operations/project-recovery.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 把 JobEvent 契约纳入基线**

在 `automated-test-baseline.md` 增加“JobEvent cursor 补拉与 snapshot 重建”门禁，引用对应 Go 与 React 测试。

- [ ] **Step 2: 补充恢复说明**

在 `project-recovery.md` 说明：作业状态变化持久化为 JobEvent；应用重启或 Bridge 重挂载时按 cursor 补拉，cursor 早于事件下界时用 JobSnapshot 重建。

- [ ] **Step 3: 提交**

```bash
git add docs
git commit -m "docs: document R1 job event feed and cursor recovery"
```

### Task 10: 全量验证、远端证据与合入 main

**Files:**
- Modify: `docs/plans/2026-08-06-r1-recoverable-job-event-feed-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 运行策略和正式前端**

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
cd apps/desktop/frontend && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../../..
```

- [ ] **Step 2: 运行 Go 全量门禁**

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
```

- [ ] **Step 3: 验证原型**

```bash
cd prototype/web && npm ci && npm test -- --run && npm run typecheck && npm run build && cd ../..
```

- [ ] **Step 4: 验证工作树**

- 不跟踪数据库、项目包或真实稿件；
- 改动只含本计划列出的 Go、迁移与前端；
- 主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist` 不在分支。

- [ ] **Step 5: 推送并检查 Foundation CI**

```bash
git push -u origin codex/r1-recoverable-job-event-feed
gh run watch <run-id> --exit-status
```

Expected: Ubuntu race、macOS、Windows 全部通过。

- [ ] **Step 6: 记录完成证据并合入 main**

```bash
git add docs/plans/2026-08-06-r1-recoverable-job-event-feed-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: complete R1 recoverable job event feed plan"
git push origin main
```

- [ ] **Step 7: 标记完成**

只有分支与 main CI 都成功后：

- 本计划状态改为 `completed`；
- 路线图 R1.2 标记完成；
- 在 R1 段记录最终 CI 证据。

## 7. 验收矩阵

| 能力 | 自动证据 |
| --- | --- |
| JobEvent 同事务持久化 | `job_event_test.go`：`ExecuteAgentRun` 后 events 连续单调 |
| 异步执行立即返回 running | `agent_test.go`：`ExecuteAgentRun` 不阻塞 Provider |
| cursor 增量补拉 | `GetJobEvents` 分页 + `hasMore`/`nextCursor` 测试 |
| cursor 早于下界用 snapshot 重建 | `GetJobEvents` `snapshotRequired` 测试 + reducer 测试 |
| 重复/漏推幂等 | reducer 重复 sequence 跳过 + Go 侧 noop notifier 补拉 |
| Bridge 重挂载恢复 | `ListRecoverableJobs` + `job_recovered` 事件测试 |
| Wails 通知只发 jobID+sequence | `wailsJobEventNotifier` 测试 |
| 不写已关闭 store | R1.1 stale 测试 + `job_recovered` 在新 store 写入 |
| 无 race | `go test -race` 通过 |

## 8. Windows/macOS 注意事项

- 异步 goroutine 使用的 long-lived context 不绑定 Wails 请求 ctx，避免页面卸载取消作业；
- `runtime.EventsEmit` 在 Windows/macOS 行为一致，通知允许丢失；
- JobEvent 写入复用 R1.1 的短锁 + generation 安全网，不在 drain 期间向旧 store 写事件；
- 测试用 fake notifier 与 gated provider 控制时序，不依赖 `time.Sleep`。

## 9. 回滚与支持策略

- 回滚本计划恢复同步 `ExecuteAgentRun` + 无 JobEvent，但不破坏已持久化的 run/step 数据（schema 只新增表）；
- 已写入的 JobEvent 留在 `job_events` 表，不影响 canonical 正文或已确认资产；
- 异步 goroutine 退出只写 run/step/event 状态，不写章节正文；
- 若 cursor 补拉在生产中出现不一致，作者可重开项目触发 `ListRecoverableJobs` + snapshot 重建。

## 10. 完成定义

本计划只有在以下条件全部满足时才能标记 `completed`：

1. `job_events` 与 `job_snapshots` 表存在且迁移可回滚；
2. `ExecuteAgentRun`/`ExecuteExtractionRun` 异步返回 running，Provider 在后台 goroutine 执行；
3. 每个状态/步骤变化在同事务 append JobEvent + upsert JobSnapshot；
4. `GetJobSnapshot`/`GetJobEvents`/`ListRecoverableJobs` 可用且 cursor 语义正确；
5. Wails `job:updated` 通知只发送 jobID + latestSequence；
6. React reducer 按 `(job_id, sequence_number)` 幂等应用，检测缺口并补拉，`snapshotRequired` 时重建；
7. 漏推、重复、Bridge 重挂载、应用重启、cursor 早于下界均有测试覆盖；
8. 全部并发测试在 `-race -shuffle=on` 下通过；
9. Ubuntu/macOS/Windows Foundation CI 通过；
10. 测试与恢复文档已更新；
11. main 分支 Foundation CI 通过；
12. 未破坏 R1.1 的 lease/drain/generation 行为。
