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
| macOS production build | 固定 arm64 Runner 上构建、结构/plist 校验、manifest、短期 tar artifact、旧项目迁移命名测试 | Developer ID、正式签名、公证、安装和升级 |
| Windows production build | 固定 x64 Runner 上构建、PE 校验、manifest、Windows Go tests、短期 zip artifact | Authenticode、安装器和升级 |

签名、公证、安装器和升级属于 R8。旧项目迁移的 fixture 规格、三阶段状态机、恢复矩阵和平台门禁入口见[旧项目迁移测试基线](legacy-project-migration.md)。

## 并发与作业恢复契约

R1 引入“Model IO 不持有工作区全局锁”与“作业不向已关闭 store 回写”两条自动门禁。它们不是可选优化，而是产品需求 7.1 与系统设计 5.3 要求的数据保护底线，普通提交必须保持。

| 契约 | 自动证据 |
| --- | --- |
| Agent Provider 调用期间作者可保存恢复草稿 | `TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall` |
| Agent Provider 调用期间作者可新建章节（写锁操作） | `TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall` |
| 切换项目 drain 超时后，作业不写已关闭 store 而转入 `recoverable` | `TestAgentRunSkipsPersistWhenStoreGenerationChanged` |
| 旧 generation 的 `recoverable` run 重试在新 active store 上执行 | `TestRetryAgentRunAfterProjectSwitchUsesNewStore` |
| lease registry 并发 acquire/release、drain 超时与 stale 标记 | `internal/application/workspace/lease_test.go` |

这些测试位于 `internal/application/workspace`，由本地 `go test -race -shuffle=on` 与 Ubuntu Foundation CI Job 覆盖；失败按并发缺陷处理，不得关闭 `-race` 或从 package pattern 排除。契约背后由 `workspace.Service` 的单调 `storeGeneration` 与 `leaseRegistry` 实现，详见 [R1 实施计划](../plans/2026-08-06-r1-agent-job-lease-workspace-lock-isolation-implementation.md)。

## JobEvent cursor 补拉与 snapshot 重建

R1.2 引入 append-only `JobEvent` 事件源与权威 `JobSnapshot` 投影：作业状态变化与事件在同一短事务提交，Wails 只发送 `job_id + latest_sequence` 轻量通知，React 按 cursor 增量补拉、检测缺口并幂等更新。以下门禁保证漏推、重复通知、Bridge 重挂载、应用重启和事件清理后的状态一致性，普通提交必须保持。

| 契约 | 自动证据 |
| --- | --- |
| 新建 queued 作业立即拥有 `job_queued` 事件和可重建 snapshot | `TestCreatedQueuedAgentRunHasReplayableSnapshot` |
| 作业生命周期 emit 连续单调的 JobEvent 且 snapshot 同事务刷新 | `TestExecuteAgentRunAppendsContiguousJobEvents`、`TestExecuteExtractionRunAppendsContiguousJobEvents` |
| snapshot 同时投影累计 Token 与步骤状态 | `TestUpdateAgentRunAppendsEventsInSameTransaction`、`jobEventReducer.test.ts`（snapshot steps） |
| Provider 失败 append `job_failed` 而非停在 `job_started` | `TestExecuteAgentRunFailureAppendsJobFailedEvent`、`TestExecuteExtractionRunFailureAppendsJobFailedEvent` |
| cursor 分页补拉返回 `has_more`/`next_cursor` | `TestGetJobEventsPaginatesByCursor` |
| cursor 早于 `earliest_replayable_sequence` 时返回 `snapshot_required` | `TestGetJobEventsSignalsSnapshotRequired` |
| `ListRecoverableJobs` 返回非终态作业（agent + extraction 合并） | `TestListRecoverableJobsReturnsNonTerminalJobs` |
| 项目重开时恢复 running 作业并 append `job_recovered` 事件 | `TestRecoverInterruptedAgentRunsAppendsJobRecoveredEvent`、`TestRecoverInterruptedTasksAppendsJobRecoveredEvent` |
| 恢复不向已终态作业追加 `job_recovered` | `TestRecoverInterruptedAgentRunsLeavesTerminalJobsUntouched` |
| reducer 按 sequence 幂等应用、检测缺口并停止 | `apps/desktop/frontend/src/features/agents/jobEventReducer.test.ts` |
| snapshot 重建后重投递事件不累计 token | `apps/desktop/frontend/src/features/agents/jobEventReducer.test.ts`（snapshot rebuild + 幂等） |
| `job:updated` 触发 cursor 补拉、snapshot 重建与分页 | `apps/desktop/frontend/src/features/agents/useJobEventReplay.test.tsx` |
| 漏推通知后下一次唤醒能补齐缺失事件 | `apps/desktop/frontend/src/features/agents/useJobEventReplay.test.tsx`（lost notification） |
| 通知在补拉进行中抵达时仍追至最新 sequence | `apps/desktop/frontend/src/features/agents/useJobEventReplay.test.tsx`（in-flight update） |
| Bridge 重挂载后 `pullJobEvents` 恢复 `recoverable` 状态且不触发终态回调 | `apps/desktop/frontend/src/features/agents/useJobEventReplay.test.tsx`（remount recovery） |
| 抽取启动后的导入状态写入失败不会遗留孤立 running task | `TestFailExtractionStartCompensatesRunningTask` |

Go 侧测试位于 `internal/application/workspace`（`job_event_test.go`），React 侧测试位于 `apps/desktop/frontend/src/features/agents`。前者由本地 `go test -race -shuffle=on` 与 Ubuntu Foundation CI Job 覆盖，后者由本地 `npm test -- --run` 与 Ubuntu Foundation CI Job 覆盖；失败按数据一致性缺陷处理，不得关闭 `-race` 或跳过 reducer 用例。实现详见 [R1.2 实施计划](../plans/2026-08-06-r1-recoverable-job-event-feed-implementation.md)。

## R1.3 项目生命周期保护

项目 lease、真正只读打开和恢复副本是作者数据保护门禁。Ubuntu 的 race 套件覆盖应用并发；macOS 与 Windows 原生 Runner 还显式运行以下测试，不能用交叉编译或跳过替代原生锁语义。

| 契约 | 自动证据 |
| --- | --- |
| 第二进程不能取得 editable lease，释放后可重试；残留或损坏元数据不代表锁 | `TestProjectLease*`（helper subprocess） |
| 只读可看到活跃 WAL 的已提交内容，但不迁移、恢复或写 canonical/业务文件 | `TestOpenReadOnly*`、`TestReadOnly*` |
| 只读工作区的正文、资产、导入、Agent、快照和分支写用例均被拒绝 | `TestReadOnlyWorkspace*` |
| 恢复副本使用一致数据库快照，源项目不被覆盖，发布后可作为新项目打开 | `TestCreateRecoveryCopy*` |

本地完整 Go 命令仍使用 `go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...`。平台 job 另外运行上述聚焦命令，是为了在失败时明确标出生命周期保护边界。只读读取活跃 WAL 时 SQLite 可能维护 `project.db-shm` 或建立空协调文件；该受限协调工件不等同于业务写入，测试会分别校验 `project.db`、已有 WAL 内容、`recovery/` 与其他业务文件保持不变。

## R1.4 文件系统运行期可靠性

R1.4 的自动测试验证可移植路径预算、无损可编辑预检、运行期错误分类，以及桌面端保留未保存文本的恢复界面。预检成功只说明当时的临时探针可完成；同步客户端、网络共享、杀毒或存储状态仍可能在编辑期间改变，因此不能把它当作后续写入必然安全的承诺。

| 验收关联 | 自动证据 |
| --- | --- |
| F-15 同步目录与网络共享位置显示风险提示，运行期 busy/共享冲突进入可恢复的 degraded 状态 | `TestPreflightEditableWarnsForKnownSyncRoot`、`TestFilesystemRiskHintUsesNetworkVolumeCheck`、`TestFilesystemRiskHintWarnsForRawUNCPath`、`TestClassifyFilesystemErrorMapsRuntimeWriteFailures`、`TestCommitFilesystemFailureDegradesWithoutChangingRevision` |
| F-16 预检只在临时目录执行，不能改动项目包中的 canonical 文件 | `TestPreflightEditableLeavesCanonicalFilesUnchanged` |
| F-17 只读、磁盘满和 sharing failure 不提交失败写入；编辑器保留文本，并提供恢复草稿、复制和重试路径 | `TestReadOnlyWorkspaceRejectsEveryWriteWithoutChangingProjectFiles`、`TestCommitFilesystemFailureDegradesWithoutChangingRevision`、`TestCreateChapterFilesystemFailureDegradesWithoutCreatingChapter`、`ChapterEditor` 的 `safely saves one recovery draft when a commit fails`、`tells the author to copy text when draft recovery also fails` 与 `retries the failed commit and copies the retained text` |
| F-18 Windows 兼容路径预算拒绝过长项目根；恢复副本在发布前也验证目标路径 | `TestValidateCreatePathRejectsLongWindowsCompatibleRoot`、`TestCreateRecoveryCopyRejectsFilesystemPathFailureBeforeCopying`、`TestRecoveryCopyFixturesFitPortablePathBudgetWithLongGoTemporaryTestNames` |

这些自动证据由本地完整 Go/前端命令和 Foundation CI 的相关 Go 测试覆盖，但不模拟真实同步客户端、共享服务、配额、Windows 长路径策略或原生文件锁时序。F-15 至 F-18 仍须在 Windows 与 macOS production build 上按[基础手工验收](foundation-acceptance.md)使用一次性副本执行并记录证据；在该证据完成前，R1.4 不得关闭或合并。

## R2 产品壳与交互基础

R2 建立独立于 `.zjproject` 的应用设置数据库、最近项目首页、原生菜单、分级作品导航、权威工作台投影和阶段自适应 AI 面板。以下自动证据保证应用设置隔离、旧配置迁移幂等、权威投影不可被本地猜测覆盖，以及未实现能力不伪造已完成效果。

| 契约 | 自动证据 |
| --- | --- |
| `settings.db` 位于应用数据目录，不在任一 `.zjproject` 内；打开时创建版本化 schema | `TestResolveCreatesNoDirectoryAndUsesStableSettingsPath`、`TestOpenCreatesVersionedApplicationSchema` |
| 应用设置仓储事务、并发安全与迁移幂等 | `internal/persistence/appsqlite/repository_test.go` |
| 旧项目 provider/profile 配置无密钥导出，source digest 去重，失败可回滚 | `TestLegacyExport*`、`TestMigrateLegacySettings*` |
| 首页导入项目幂等：重复提交复用已准备 import job 和已创建章节 | `TestCreateImportedProjectRetryReusesPreparedJob` |
| 首页导入 apply 失败保留可恢复项目与 import job，不删除源文件 | `TestCreateImportedProjectApplyFailureReturnsRecoverableProject` |
| 整书档案在 R2 返回 `FEATURE_NOT_AVAILABLE`，不创建抽取任务 | `TestStartWholeBookArchiveReturnsFeatureUnavailableWithoutCreatingExtractionTask` |
| `WorkbenchProjection` 由 Go 应用层权威投影，React 不按路由猜测 canvas | `WorkbenchRouter.test.tsx`（routes from authoritative canvas） |
| AI 面板默认隐藏，`Cmd/Ctrl+J` 呼出，切换画布/设置不丢输入与正文草稿 | `Workspace.test.tsx`（Cmd+J、面板切换保留状态） |
| 选区审阅携带 base revision，正文变化后提交返回 `STALE_SELECTION` | `ChapterEditor.test.tsx`（selection review） |
| 整书档案卡片在 `not_generated` 状态显示按钮但不伪造结论 | `WholeBookArchiveCard.test.tsx` |

Go 侧测试位于 `internal/application/appshell`、`internal/application/appsettings`、`internal/persistence/appsqlite` 和 `apps/desktop`，React 侧测试位于 `apps/desktop/frontend/src/features/workbench`、`features/workspace`、`features/agents` 和 `features/imports`。前者由本地 `go test -race -shuffle=on` 覆盖，后者由本地 `npm test -- --run` 与 `npm run typecheck` 覆盖。实现详见 [R2 实施计划](../plans/2026-08-11-r2-product-shell-interaction-foundation-implementation.md)。

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
5. 并发与作业恢复契约测试通过（见上文表格）；
6. JobEvent cursor 补拉与 snapshot 重建契约测试通过（见上文表格）；
7. GitHub Foundation CI 的相关 Job 通过。

这仅是提交级自动化完成判定；涉及 R1.4 时，F-15 至 F-18 的双平台原生手工证据仍是阶段关闭和合并的额外前提。
