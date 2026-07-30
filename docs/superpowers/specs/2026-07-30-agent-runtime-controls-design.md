# P3 智能体运行控制与上下文预算设计

- 日期：2026-07-30
- 状态：已批准实施
- 前置规格：`2026-07-29-provider-agent-authoring-design.md`

## 1. 目标

补齐第一条真实智能创作闭环尚未完成的运行治理能力：作者可以取消、重试任务，看到各步骤和整次运行的 Token 用量，并通过项目级预算限制单次任务规模；上下文装配必须在明确字符预算内确定性筛选和裁剪，且能解释包含、遗漏了哪些数据。

本切片不引入后台常驻队列、自动价格目录、向量数据库或语义检索，也不开始小说导入。

## 2. 运行控制

- `ExecuteAgentRun` 为每个运行建立独立 `context.WithCancel` 并登记在进程内运行注册表。
- `CancelAgentRun`：
  - queued 运行直接落为 cancelled；
  - running 运行调用 cancel，Provider HTTP 请求继承取消信号；
  - 当前步骤和未开始步骤都落为 cancelled；
  - 已完成步骤与 Artifact 保留。
- `RetryAgentRun` 只接受 failed、cancelled、recoverable 状态。它使用原章节、原作者目标和当前角色配置创建新 queued Run，并写入 `retry_of`；旧运行不可变。
- 项目关闭会先取消全部进程内运行，再等待运行释放项目读锁。
- 分支切换、确认合并和快照恢复在有运行时立即返回 `ErrAgentRunActive`，不等待、不关闭仍被使用的 SQLite。

## 3. Token 预算

新增项目级 `AgentRuntimeSettings`：

- `max_run_tokens`：单次运行允许的 Provider input + output Token 总量，默认 30,000；
- `max_context_chars`：装配上下文 JSON 的最大字符数，默认 120,000；
- `updated_at`。

每次创建 Run 时把设置快照写入 Run，保证后续修改设置不改变历史。创建前校验三个角色的 `max_output_tokens` 总和不超过 `max_run_tokens`。每个步骤完成后保存 Provider 返回的 input/output Token；若累计使用量已经达到预算，则不再发起下一步，运行以 `token_budget_exceeded` 失败，不生成 ChangeSet。

Token 为 Provider 报告的实际值，不自行估算费用。本切片不保存单价和金额。

## 4. 上下文裁剪

Context Assembler 输出一个稳定 JSON Manifest，分为：

1. 必选：项目、目标章节、关联大纲、目标章节场景卡、已确认事实；
2. 高优先级：章节出场人物、相关势力/关系、角色知识；
3. 中优先级：世界规则、未回收伏笔、故事事件；
4. 低优先级：其他人物、地点、区域和候选事实。

同级数据使用现有持久化排序和 ID 作为稳定次序。超过 `max_context_chars` 时，从低优先级数组尾部开始移除；必选数据不移除。若必选数据本身超过预算，运行在任何 Provider 请求前以 `context_budget_exceeded` 失败。

Manifest 的 `meta` 保存：

- 最终字符数与预算；
- 是否发生裁剪；
- 每个数据域的总数、包含数、遗漏数；
- 每个数据域原始内容 SHA-256；
- 装配版本 `context-v2`。

完整 Manifest 仍作为 Context Artifact 保存，因此历史运行可追溯。

## 5. 数据迁移

SQLite v12：

- 新建 `agent_runtime_settings`；
- `agent_runs` 增加 `token_budget`、`context_char_budget`、`input_tokens`、`output_tokens`、`context_chars`、`context_truncated`；
- `agent_run_steps` 增加 `provider_request_id`、`input_tokens`、`output_tokens`。

迁移为已有 Run 填充安全默认值，不改变已有状态或产物。

## 6. UI

智能创作工作区增加：

- 单次 Token 预算和上下文字符预算设置；
- 运行卡片显示 `已用 Token / 预算`、上下文大小和裁剪标识；
- running/queued 状态显示“取消”；
- failed/cancelled/recoverable 状态显示“重试”；
- 执行时先把 queued Run 加入列表，再异步等待执行结果，保证取消按钮可用；
- 预算、取消和上下文错误使用可理解中文提示。

## 7. 测试与验收

1. 取消信号能传入阻塞的假 Provider，运行与步骤最终为 cancelled。
2. 重试生成新 ID、保留原输入并正确关联旧 Run。
3. 运行期间分支切换、合并确认和快照恢复立即被拒绝。
4. Step 和 Run 的实际 Token 使用量持久化并可在重启后读取。
5. 创建时输出上限超过预算会被拒绝；运行中实际用量达到预算不会发起下一步骤。
6. 相同项目状态和预算生成字节一致的 Manifest。
7. 超预算时按优先级裁剪并记录计数；必选域超限时不调用 Provider。
8. 正式正文仍只通过待审 ChangeSet 更新。
9. Go 单元/集成测试、前端测试、macOS build 和 Windows amd64 交叉编译通过。
