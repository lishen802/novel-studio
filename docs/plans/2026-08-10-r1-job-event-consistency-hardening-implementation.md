# R1 JobEvent 一致性加固实施计划

**状态：** `completed`
**关联阶段：** R1 长任务与项目可靠性
**基线：** `334a860`（R1.2 可恢复 JobEvent feed）

## 目标

修复 R1.2 中 JobEvent / JobSnapshot 在作业创建、事件补拉、步骤用量投影与抽取启动失败路径上的一致性缺口。修复后的 `queued`、`running`、终态作业都必须有可重建快照；通知仅为唤醒信号，不能因正在补拉而永久丢失；快照必须包含当时的步骤状态与累计 Token；抽取启动的任一后续持久化失败都不能遗留无执行者的 `running` 作业。

## 不变量

1. 每个新建的 Agent run / extraction task 在同一事务中写入 `job_queued` 事件和 sequence=1 的 `JobSnapshot`。
2. 作业行、步骤行、事件、快照始终在同一个短 SQLite 事务内更新；模型 I/O 不进入事务。
3. `JobSnapshot` 是完整重建点：包含状态、累计 Token、序列边界和所有步骤的 `{id,status}`。
4. React 收到通知时记录目标 sequence；若已有补拉在进行，当前或紧随其后的补拉必须追至该目标，不能因去重锁而丢失唤醒。
5. `ExecuteExtractionRun` 在任务转 `running` 后，若登记运行上下文或更新导入任务失败，必须持久化为可解释的失败终态并发出 `job_failed`。
6. 新迁移对既有 `queued` 作业回填初始事件与快照，不覆盖或伪造已有事件历史。

## 实施顺序（测试先行）

### 1. 持久化契约与迁移

- 新增 `job_queued` 事件类型及其域测试。
- 新建版本化迁移：`job_snapshots.step_states`；为没有快照的既有 queued Agent / extraction 作业插入 sequence=1 的 queued 事件与快照。
- 修改 `CreateAgentRun` / `CreateTask`：创建作业、步骤、初始事件及快照在同一事务提交。
- 扩展快照领域、查询 DTO 与 Wails JSON 映射，保存并读取步骤状态。
- 为每次步骤更新在同一事务中重算父作业累计 Token，并用当前步骤状态刷新快照。

### 2. 应用层失败补偿

- 写入测试：抽取任务已切换到 running、但运行登记或导入任务更新失败时，最终为 `failed`，事件序列包括 `job_failed`，且不启动后台流水线。
- 实现集中失败补偿函数；原始错误与补偿错误使用 `errors.Join` 保留。
- 将步骤持久化错误从静默忽略改为能导致可恢复的明确失败路径。

### 3. 前端重放可靠性

- 写入 hook 测试：第一次 `getJobEvents` 未结束时收到更高 sequence 的通知，结束后无需第三次通知即可追上新事件。
- 以每个 `job_id` 的 pending target 取代单纯 `pulling` 短路；结束时检查是否有尚未追上的目标并继续补拉。
- `rebuildFromSnapshot` 采用快照的步骤状态，保持旧 mock 可省略该字段。

### 4. 验证与文档

- Go：相关 domain/sqlite/workspace 测试先独立运行，再运行 `go test -race -shuffle=on ./apps/desktop ./internal/...` 与 `go vet`。
- 前端：`npm test -- --run`、`npm run typecheck`、`npm run build`。
- 检查迁移升级路径、格式化、`git diff --check`；更新自动化测试基线中 JobEvent 快照重建断言。

## 明确不包含

- 不调整事件保留期、压缩策略或 Context Manifest。
- 不改变 R1.1 lease/generation 语义，不引入 OS 文件锁。
- 不增加新模型 Provider、批次检查点或剧情分支功能。

## 验收标准

- 新建 queued 作业可立即调用 `GetJobSnapshot` / `GetJobEvents`，均不返回 `ErrJobSnapshotMissing`。
- 快照的 Token 与步骤状态与数据库作业/步骤行一致，重建 reducer 后不丢失运行中的步骤。
- 并发通知测试证明 cursor 最终达到通知中最大的 sequence。
- 任一抽取启动后置写入失败不会留下孤立 running task。
- 全部 Go 与 React 检查通过，且原始主工作区未提交文件不进入该修复提交。
