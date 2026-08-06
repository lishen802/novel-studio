# R0 旧项目迁移与恢复 Fixture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为旧版 `.zjproject` 建立确定性、无版权风险的自动化 fixture，并把现有“直接在原库上执行 Goose migration”改为“升级前备份 → 临时副本迁移与校验 → 可恢复原子替换”，使旧项目打开、迁移失败和替换中断都不会损坏原数据库。

**Architecture:** fixture 不提交二进制 SQLite 或真实稿件，而是提交合成 seed/expected 规格，测试时在临时目录按固定旧 Schema 版本生成完整 `.zjproject`。`projectfs` 为新旧项目保证短路径 `recovery/` 目录存在；`sqlite.OpenExistingDatabase` 先恢复未完成的迁移替换，再判断 Schema。只有旧 Schema 才创建一致性备份和 stage 数据库，在 stage 上执行 Goose、`quick_check`、Schema 和唯一项目校验，全部通过后才用带 journal 的短时文件替换激活；失败时保留原库并回滚。

**Tech Stack:** Go 1.25、`testing`、`modernc.org/sqlite`、Goose v3、SQLite `VACUUM INTO`/WAL、JSON migration journal、GitHub Actions Ubuntu/macOS/Windows Runner。

---

- 状态：`draft`
- 路线图阶段：[R0：固定基线与 CI](../roadmap/development-roadmap.md#5-r0固定基线与-ci)
- 上游依据：[产品需求 5.1/7.1/7.3/9.1](../product/requirements.md)、[产品设计：项目打开与恢复](../product/design.md)、[系统设计 3.1/4.2/14](../architecture/system-design.md)、[ADR-0002](../architecture/0002-go-first-wails-stack.md)、[ADR-0003](../architecture/0003-wails-project-layout.md)
- 前置计划：[R0 CI 与测试基线](2026-08-06-r0-ci-test-baseline-implementation.md)（`completed`）、[R0 双平台 Wails 构建矩阵](2026-08-06-r0-dual-platform-wails-build-matrix-implementation.md)（`completed`）
- 后续阶段：[R1 长任务与项目可靠性](../roadmap/development-roadmap.md#6-r1长任务与项目可靠性)

## 1. 当前基线与已知缺口

当前代码已经具备：

- `projectfs.OpenLayout` 拒绝非 `.zjproject` 和非目录路径，并可为旧项目补建 `imports/`；
- `OpenExistingDatabase` 在迁移前检查文件存在、SQLite header、未来 Schema 和 `PRAGMA quick_check`；
- Goose 迁移 1–14、WAL、`busy_timeout=5000` 与单连接 SQLite 基线；
- 单项迁移测试已覆盖 v9 候选、v11 Agent、v12 导入和 v13 抽取数据；
- `workspace.Service.OpenProject` 只有在包、快照、分支、数据库和项目身份全部打开成功后才替换当前工作区；
- 快照和剧情分支已经证明 journal + stage + backup + rename + reopen validation 的恢复模式可行。

当前缺口：

1. `OpenExistingDatabase` 直接对作者的 `project.db` 执行 `provider.Up(ctx)`；迁移 SQL、磁盘写入或进程中断可能留下部分升级状态。
2. 打开旧项目之前没有一致性升级备份，也没有迁移临时副本。
3. 没有数据库迁移专用 journal；无法区分“尚未替换”“已替换待验证”“验证后清理中断”。
4. 迁移后的校验只有 `quick_check`，没有验证当前 Schema、唯一项目记录和旧数据哨兵。
5. 现有迁移测试以单表或单版本为主，没有完整旧 `.zjproject` 从打开到工作区激活的端到端 fixture。
6. 不能提交 `.db`/`.zjproject`，但尚未建立文本 fixture 生成协议。
7. macOS build Job 只做 Wails build；Windows 会运行 Go tests，Ubuntu 会运行 race tests，旧项目兼容测试还没有显式的平台门禁名称。

## 2. 设计选择

### 2.1 Fixture 使用文本规格，测试时生成数据库

不提交二进制 `.db`、`.zjproject`、WAL 或 SHM。新增 `fixtures/migrations/v09/`：

- `seed.sql`：只包含固定 ID、固定时间和少量合成中文故事数据；
- `expected.json`：描述升级后必须保留的项目、章节、revision、恢复草稿、正式资产和待审候选哨兵；
- `README.md`：记录来源、旧 Schema 版本、生成方法和覆盖的迁移链。

测试 helper 使用仓库内已版本化的 migration 1–9 建库，再执行 `seed.sql`。同时断言生成库确实停在 v9 且拥有预期的 v9 Schema 指纹。这样既不违反工件策略，又能对 v9→v14 做完整回归。

本 fixture 只使用原创合成内容，例如“雾港”“林澈”“旧铜钥匙”，不得来自真实作者项目或已出版/网络小说。

### 2.2 只迁移旧库，不改变新建库路径

- `OpenDatabase` 用于创建新项目，仍可在新空库上直接执行全部 migration；
- `OpenExistingDatabase` 用于旧项目，必须走安全升级协调器；
- Schema 已是当前版本时，不创建备份或 stage，只完成运行时 PRAGMA 与完整性检查；
- 未来版本、损坏库和无权限库在任何 migration 工件创建前失败。

### 2.3 项目包增加内部 `recovery/` 目录

项目包使用短、固定、与章节标题无关的路径：

```text
作品.zjproject/
  project.db
  recovery/
    migration.json
    migration-stage.sqlite
    migration-original.sqlite
    migration-v09-v14.sqlite
```

- `migration-v09-v14.sqlite` 是升级前一致性备份，成功后仍保留；
- `migration-stage.sqlite` 是执行 migration 的临时副本；
- `migration-original.sqlite` 只在原子替换窗口内暂存原数据库；
- `migration.json` 是不含绝对路径和正文的外部事务 journal。

`recovery/` 是内部恢复目录，R0 不把它加入 UI，也不让 React/Wails 直接操作。`projectfs` 在创建项目和打开缺少该目录的旧包时补建它。运行时文件继续受 `.gitignore` 和 tracked artifact policy 保护。

### 2.4 迁移状态机

Journal 只保存：

```go
type migrationJournal struct {
    Schema      int    `json:"schema"`
    FromVersion int64  `json:"from_version"`
    ToVersion   int64  `json:"to_version"`
    Phase       string `json:"phase"`
}
```

合法阶段：

```text
prepared
  原库仍在 project.db；stage 已迁移并验证；持久备份已完成

activated
  原库在 migration-original.sqlite；stage 已成为 project.db；等待重新打开验证

committed
  新 project.db 已验证；仅等待删除 operation 工件
```

恢复规则：

| 现场 | 动作 |
| --- | --- |
| `prepared`，原库仍在 | 删除 stage/journal，保留升级前备份，重新规划迁移 |
| 存在 `migration-original.sqlite` | 优先恢复 original 为 `project.db`，删除未确认的新库和 stage |
| `committed` 且 original 已删除 | 校验当前 `project.db` 是目标版本；通过后只清 journal |
| journal 损坏或文件组合无法判定 | 返回专用恢复错误，不删除任何可能是唯一副本的数据库 |

“无法判定时停止”优先于猜测和覆盖。

### 2.5 备份与 stage 的一致性

源库在短连接中读取当前版本并先执行 `quick_check`。升级前备份使用 SQLite `VACUUM INTO`，让 WAL 中已提交内容进入单文件一致性副本。stage 从已校验的备份复制并 `fsync`，不是在活动 WAL 文件旁直接复制 `project.db`。

若相同 `v09→v14` 备份已存在，只在以下条件全部成立时复用：

- SQLite header 和 `quick_check` 通过；
- Schema 是 v9；
- 唯一 project ID 与源库相同。

否则停止并返回恢复错误，不覆盖既有备份。

### 2.6 原子替换与验证

模型 IO 不参与本计划，也不得出现在数据库事务中。文件激活顺序：

1. 原子写入 `migration.json=prepared` 并同步目录；
2. 关闭源库，移除已关闭库的 `-wal/-shm` 残留；
3. rename `project.db` → `migration-original.sqlite`；
4. rename `migration-stage.sqlite` → `project.db`；
5. 写入 `phase=activated` 并同步目录；
6. 重新打开新 `project.db`，验证目标 Schema、`quick_check`、唯一 project ID；
7. 写入 `phase=committed`；
8. 删除 operation original/journal，保留 `migration-v09-v14.sqlite`。

任一步在 `migration-original.sqlite` 删除前失败，都恢复原库。Windows rename/file lock 行为由原生 Runner 测试验证；不引入 CGO 或平台特有第三方库。

## 3. 明确不包含

- 不增加新的产品数据库业务表或 Goose migration 15；
- 不实现数据库降级或回写旧 Schema；
- 不迁移订阅运行时、Provider 凭据或系统凭证库；
- 不修改、删除或自动重写历史定稿正文；
- 不实现项目级 OS lease、第二实例保护或真正只读 store，这些属于 R1；
- 不实现磁盘空间预估 UI、恢复中心 UI 或让作者手动选择 migration 文件；
- 不做安装器跨版本升级、签名、公证、OneDrive/杀毒软件完整矩阵，这些属于 R8；
- 不提交二进制 fixture、完整 Prompt、真实用户日志或真实小说；
- 不把 migration 备份混入用户命名快照列表；
- 不清理成功升级后的持久备份；备份保留策略在 R8 单独设计。

## 4. 文件级改动

| 文件 | 职责 |
| --- | --- |
| `fixtures/migrations/v09/seed.sql` | v9 合成旧项目数据。 |
| `fixtures/migrations/v09/expected.json` | 升级后的数据哨兵。 |
| `fixtures/migrations/v09/README.md` | fixture 版本、来源和生成规则。 |
| `internal/testsupport/legacyproject/fixture.go` | 在 `t.TempDir()` 生成旧 `.zjproject`，不写仓库。 |
| `internal/testsupport/legacyproject/fixture_test.go` | 校验 fixture 版本、Schema 指纹和确定性。 |
| `internal/platform/projectfs/layout.go` | 创建/打开时保证内部 `recovery/` 目录。 |
| `internal/platform/projectfs/layout_test.go` | 新项目和缺目录旧项目的兼容测试。 |
| `internal/persistence/sqlite/integrity.go` | 返回 Schema version，增加迁移/恢复分类错误。 |
| `internal/persistence/sqlite/database.go` | 分离新库直接迁移与现有库安全打开。 |
| `internal/persistence/sqlite/migration.go` | stage、备份、journal、激活、回滚与中断恢复。 |
| `internal/persistence/sqlite/migration_test.go` | 成功、失败、备份、恢复、不可判定状态测试。 |
| `internal/persistence/sqlite/sync_directory_unix.go` | macOS/Linux 目录同步。 |
| `internal/persistence/sqlite/sync_directory_windows.go` | Windows 安全目录同步边界。 |
| `internal/application/workspace/recovery_test.go` | 从 v9 `.zjproject` 打开并保留当前工作区失败安全。 |
| `.github/workflows/ci.yml` | 给 macOS Job 增加命名明确的迁移兼容测试。 |
| `docs/testing/legacy-project-migration.md` | fixture、状态机、平台门禁和故障诊断。 |
| `docs/testing/automated-test-baseline.md` | 将旧项目迁移纳入基线。 |
| `docs/operations/project-recovery.md` | 向支持/用户说明自动备份和失败行为。 |
| `docs/README.md` | 链接新测试文档。 |
| `docs/roadmap/development-roadmap.md` | 跟踪本计划状态和 R0 完成证据。 |

## 5. 核心内部接口

实现时保持 `OpenExistingDatabase(ctx, path)` 的公开签名不变，避免扩散到所有 Store 调用方。新增逻辑采用包内接口/函数：

```go
type migrationPaths struct {
    database string
    recovery string
    journal  string
    stage    string
    original string
    backup   string
}

type migrationHooks struct {
    afterStageValidated func() error
    afterActivated      func() error
}

func schemaVersion(ctx context.Context, db *sql.DB) (int64, error)
func openRuntimeDatabase(ctx context.Context, path string) (*sql.DB, error)
func migrateDatabase(ctx context.Context, db *sql.DB) error
func openExistingDatabase(ctx context.Context, path string, hooks migrationHooks) (*sql.DB, error)
func recoverInterruptedMigration(ctx context.Context, paths migrationPaths) error
func prepareMigrationBackup(ctx context.Context, source *sql.DB, paths migrationPaths, from, to int64) error
func migrateAndValidateStage(ctx context.Context, paths migrationPaths, projectID string) error
func activateMigratedDatabase(ctx context.Context, paths migrationPaths, journal migrationJournal, projectID string, hooks migrationHooks) (*sql.DB, error)
```

`migrationHooks` 只用于同包测试制造确定性失败，不导出，不进入生产依赖注入。禁止使用 `time.Sleep`、磁盘填满或杀进程作为普通单测的失败注入方式。

新增分类错误：

```go
var (
    ErrDatabaseMigration         = errors.New("database migration failed")
    ErrDatabaseMigrationRecovery = errors.New("database migration recovery required")
)
```

底层 `ErrDatabaseCorrupt`、`ErrDatabasePermission`、`ErrDatabaseWrite` 和 `ErrDatabaseTooNew` 必须继续可通过 `errors.Is` 识别；外层迁移错误只增加上下文，不吞掉原分类。

## 6. TDD 实施步骤

### Task 0: 启动已批准计划

**Files:**

- Modify: `docs/plans/2026-08-06-r0-legacy-project-migration-recovery-fixture-implementation.md`
- Modify: `docs/roadmap/development-roadmap.md`

- [ ] **Step 1: 创建隔离工作区**

调用 `superpowers:using-git-worktrees`，从最新 `main` 创建 `codex/r0-legacy-project-migration-recovery-fixture` worktree。不得复制或提交主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist`。

- [ ] **Step 2: 标记计划开始**

将本计划和路线图状态从 `draft` 改为 `in_progress`，单独提交：

```bash
git add docs/plans/2026-08-06-r0-legacy-project-migration-recovery-fixture-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: start R0 legacy migration plan"
```

### Task 1: 建立确定性 v9 旧项目 fixture

**Files:**

- Create: `fixtures/migrations/v09/seed.sql`
- Create: `fixtures/migrations/v09/expected.json`
- Create: `fixtures/migrations/v09/README.md`
- Create: `internal/testsupport/legacyproject/fixture.go`
- Create: `internal/testsupport/legacyproject/fixture_test.go`
- Modify: `fixtures/README.md`

- [ ] **Step 1: 先写失败测试**

测试必须断言：

1. fixture 在测试临时目录生成 `legacy-v09.zjproject/project.db`；
2. 不预建 `imports/` 与 `recovery/`，用于覆盖旧包兼容；
3. `goose_db_version` 最大 applied version 等于 9；
4. v9 Schema 至少包含 `projects`、`chapters`、`chapter_revisions`、`drafts`、`characters`、`world_rules`、`canon_facts`、`change_sets`、`change_operations`，且不包含 v11 的 `agent_runs`；
5. 两次生成得到相同业务行与 `expected.json`，但不比较 SQLite 文件字节；
6. 所有正文、名称和 ID 都来自合成 seed。

运行并确认 RED：

```bash
go test -count=1 ./internal/testsupport/legacyproject
```

Expected: 因 fixture helper/文件尚不存在而失败。

- [ ] **Step 2: 实现 fixture builder**

`legacyproject.Build(t, parent)` 返回：

```go
type Fixture struct {
    PackagePath  string
    DatabasePath string
    Expected     Expected
}
```

Builder 通过 `runtime.Caller` 定位仓库根，读取 `internal/persistence/sqlite/migrations`，用 Goose `UpTo(ctx, 9)` 创建旧 Schema，执行 `seed.sql` 后关闭数据库。固定时间使用 UTC RFC3339，固定 ID 不使用随机数。

`expected.json` 至少记录：

- project ID/title/genre/summary；
- 1 个已提交章节的 ID/body/revision；
- 1 个历史 revision；
- 1 个恢复草稿；
- 1 个人物、1 条世界规则、1 条 canonical fact；
- 1 个 v9 pending change set 和 operation。

- [ ] **Step 3: 验证并提交**

```bash
go test -count=1 ./internal/testsupport/legacyproject
node scripts/ci/tracked-artifact-policy.mjs
git add fixtures internal/testsupport/legacyproject
git commit -m "test: add deterministic v9 project fixture"
```

### Task 2: 为项目包补充内部 recovery 目录

**Files:**

- Modify: `internal/platform/projectfs/layout.go`
- Modify: `internal/platform/projectfs/layout_test.go`

- [ ] **Step 1: 先写失败测试**

新增测试：

- `TestLayoutCreateIncludesRecoveryDirectory`
- `TestOpenLayoutCreatesMissingRecoveryDirectoryForLegacyPackage`
- `TestOpenLayoutRecoveryDirectoryFailureDoesNotTouchDatabase`

失败用例通过把包目录设为不可写或让 `recovery` 被普通文件占用，断言 `project.db` 字节不变。

```bash
go test -count=1 ./internal/platform/projectfs
```

Expected: 前两个用例因目录不存在失败。

- [ ] **Step 2: 最小实现**

给 `Layout` 增加 `Recovery string`，固定为 `filepath.Join(root, "recovery")`。`Create` 创建它；`OpenLayout` 与 `imports/` 一样只补建目录，不创建数据库，不删除冲突文件。

- [ ] **Step 3: 验证并提交**

```bash
go test -count=1 ./internal/platform/projectfs
git add internal/platform/projectfs
git commit -m "feat: add project recovery directory"
```

### Task 3: 先定义安全迁移失败行为

**Files:**

- Create: `internal/persistence/sqlite/migration_test.go`
- Modify: `internal/persistence/sqlite/integrity_test.go`

- [ ] **Step 1: 写成功迁移测试**

`TestOpenExistingDatabaseMigratesLegacyFixtureThroughValidatedStage` 必须断言：

- v9 fixture 打开后变为 v14；
- `PRAGMA quick_check` 为 `ok`，WAL、foreign keys、busy timeout 保持现有契约；
- `expected.json` 中全部哨兵保留；
- v10 结构化 change operation 转换正确；
- v11–v14 新表存在；
- `recovery/migration-v09-v14.sqlite` 存在、可独立打开、仍为 v9、project ID 相同；
- stage、original、journal 均已清理。

- [ ] **Step 2: 写迁移失败测试**

通过 `migrationHooks.afterStageValidated` 和包内 migration runner 注入失败，覆盖：

- stage migration 失败：原 `project.db` 文件身份、Schema 和业务哨兵不变；
- stage `quick_check`/项目数量失败：不进入替换；
- 激活后验证失败：original 被恢复；
- backup 创建失败：不创建 journal、不改原库；
- 当前 v14 库：不创建任何 migration backup；
- future/corrupt/missing：维持现有错误分类且不创建 `recovery` 中的迁移文件。

文件身份比较使用源库关闭后的 SHA-256，不能在 WAL 活动时用裸文件字节推导逻辑一致性。

- [ ] **Step 3: 写中断恢复表驱动测试**

至少覆盖：

| Case | 初始文件 | 期望 |
| --- | --- | --- |
| prepared | db + stage + journal | 原 db 保留，stage/journal 清理 |
| activated | db(new) + original(old) + journal | old 恢复为 db |
| committed cleanup | db(new) + journal，无 original | new 校验后保留，journal 清理 |
| damaged journal | db + 无法解析 journal | 返回 `ErrDatabaseMigrationRecovery`，不删除文件 |
| ambiguous | db + original，无有效 journal | 返回恢复错误或按明确的“original 优先”规则恢复；实现与文档必须一致 |

运行并确认 RED：

```bash
go test -count=1 -run 'Test(OpenExistingDatabaseMigratesLegacy|Migration)' ./internal/persistence/sqlite
```

Expected: 因安全迁移协调器尚不存在而失败。

### Task 4: 实现 stage、备份、journal 与原子激活

**Files:**

- Create: `internal/persistence/sqlite/migration.go`
- Create: `internal/persistence/sqlite/sync_directory_unix.go`
- Create: `internal/persistence/sqlite/sync_directory_windows.go`
- Modify: `internal/persistence/sqlite/database.go`
- Modify: `internal/persistence/sqlite/integrity.go`

- [ ] **Step 1: 拆分数据库打开职责**

把当前 `openDatabase` 拆成：

- `openSQLite`：`sql.Open`、单连接、Ping；
- `configureRuntimeDatabase`：foreign keys、busy timeout、WAL；
- `migrateDatabase`：只包装 embedded Goose；
- `validateDatabase`：Schema version、quick check、可选唯一 project ID；
- `openRuntimeDatabase`：只打开已是当前版本的数据库，不执行 migration。

`OpenDatabase` 对新库执行 migrate；`OpenExistingDatabase` 先 probe 和恢复，再决定直接打开或安全迁移。

- [ ] **Step 2: 实现安全复制**

- 源库先 `quickCheck`、读取 from version 和唯一 project ID；
- 使用参数化/安全转义的 `VACUUM INTO` 生成备份；
- `VACUUM INTO` 期间不包裹额外长事务；
- stage 从备份复制，目标以 `O_CREATE|O_EXCL` 创建，写完 `Sync`；
- 禁止复制 `-wal/-shm`；
- 所有路径由数据库父目录和固定短文件名推导，不接受 journal 中的任意路径。

- [ ] **Step 3: 实现原子 journal**

先写同目录临时 JSON，`Sync` 文件后 rename 为 `migration.json`，再同步目录。JSON 只允许 schema 1 和三个已知 phase；未知字段可忽略，未知 schema/phase 必须停止。

- [ ] **Step 4: 实现激活与回滚**

严格按 2.6 顺序 rename。任一错误都通过 `errors.Join` 保留主错误和恢复错误。回滚成功也应向调用者返回原始 migration 错误，不得把旧库悄悄作为“成功打开”继续运行。

- [ ] **Step 5: 实现平台目录同步**

- Unix/macOS：打开父目录并 `Sync()`；
- Windows：提供明确实现；若 Go/Windows 对目录句柄同步不支持，只允许把特定“不支持”错误视作 capability limitation，rename 和文件 `Sync` 仍必须成功，其他错误继续上抛；
- build tags 必须让 Linux、macOS、Windows 都能编译。

- [ ] **Step 6: 运行测试并提交**

```bash
go test -count=1 ./internal/persistence/sqlite
go test -count=20 -run 'Test(Migration|OpenExistingDatabaseMigratesLegacy)' ./internal/persistence/sqlite
go test -count=1 -race ./internal/persistence/sqlite
git add internal/persistence/sqlite
git commit -m "feat: migrate legacy databases through recoverable staging"
```

### Task 5: 增加完整 `.zjproject` 打开回归

**Files:**

- Modify: `internal/application/workspace/recovery_test.go`
- Modify: `internal/persistence/sqlite/project_store_test.go`

- [ ] **Step 1: 写 Workspace 端到端测试**

用真实 `projectfs.Manager`、`sqlite.Factory` 和 v9 fixture：

- 打开旧包成功；
- `workspace.CurrentProject` 返回 fixture 项目；
- 章节正文/revision 与正式资产可从现有 store API 读取；
- `imports/`、`recovery/` 被兼容创建；
- 关闭重开不会再次生成备份或再次 migration。

- [ ] **Step 2: 写工作区失败安全测试**

先打开一个当前项目，再尝试打开：

- stage migration 失败的旧项目；
- journal 损坏的旧项目；
- future Schema 项目。

断言原当前项目和 store 仍然激活，失败目标原库未变化。

- [ ] **Step 3: 验证并提交**

```bash
go test -count=1 ./internal/application/workspace ./internal/persistence/sqlite ./internal/platform/projectfs
git add internal/application/workspace internal/persistence/sqlite/project_store_test.go
git commit -m "test: cover legacy project open and recovery"
```

### Task 6: 把迁移兼容性加入三平台门禁

**Files:**

- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: 在 macOS build Job 增加明确步骤**

在 Wails build 前增加：

```yaml
- name: Test legacy project migration on macOS
  run: go test -count=1 ./internal/testsupport/legacyproject ./internal/platform/projectfs ./internal/persistence/sqlite ./internal/application/workspace
```

Ubuntu 的 race Job 和 Windows 的完整 Go tests 已覆盖相同 package，不复制第三套脚本。Job 日志应能直接看到旧项目迁移失败阶段。

- [ ] **Step 2: 本地校验 workflow 文本和测试**

```bash
rg -n "Test legacy project migration on macOS|Run Go tests on Windows|Run Go race tests" .github/workflows/ci.yml
go test -count=1 ./internal/testsupport/legacyproject ./internal/platform/projectfs ./internal/persistence/sqlite ./internal/application/workspace
```

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: gate legacy project migration on macOS"
```

### Task 7: 更新测试与恢复文档

**Files:**

- Create: `docs/testing/legacy-project-migration.md`
- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/operations/project-recovery.md`
- Modify: `docs/README.md`
- Modify: `fixtures/README.md`

- [ ] **Step 1: 编写工程测试文档**

记录：

- fixture 为什么是文本 seed 而非二进制数据库；
- v9→v14 覆盖范围和增加新 fixture 的规则；
- migration 三阶段文件状态和恢复矩阵；
- 三平台 CI 入口；
- 失败日志不得打印 seed 正文、SQL dump、绝对路径或数据库文件；
- 如何在临时目录人工生成 fixture 进行诊断。

- [ ] **Step 2: 更新用户/支持恢复说明**

补充：

- 旧项目升级前会在 `recovery/` 留下一致性备份；
- migration 在临时副本完成，失败不替换原 `project.db`；
- 中断后应用优先自动恢复原库；
- 若提示“迁移现场无法判定”，保留整个项目包并在副本上排查，不手工删除 recovery 文件；
- `recovery/` 不等同于用户命名快照，跨设备仍复制完整 `.zjproject`。

- [ ] **Step 3: 提交**

```bash
git add docs fixtures/README.md
git commit -m "docs: document legacy migration recovery"
```

### Task 8: 全量验证、远端证据与合入

- [ ] **Step 1: 运行策略和正式前端**

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
node --test scripts/ci/wails-build-artifact.test.mjs

cd apps/desktop/frontend
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../../..
```

- [ ] **Step 2: 运行 Go 全量门禁**

```bash
go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...
go vet ./apps/desktop ./internal/...
```

- [ ] **Step 3: 验证原型**

```bash
cd prototype/web
npm ci
npm test -- --run
npm run typecheck
npm run build
cd ../..
```

- [ ] **Step 4: 验证工作树**

```bash
git status --short
git diff --check
git diff --name-only main...HEAD
git ls-files '*.db' '*.sqlite' '*.db-wal' '*.db-shm' '*.zjproject'
```

Expected:

- 不跟踪任何数据库、项目包或真实稿件；
- 改动只包含本计划列出的 fixture 文本、Go、CI 和文档；
- 主工作区未跟踪的 `apps/desktop/build/darwin/Info.dev.plist` 不在分支；
- 重复运行迁移测试不会产生仓库内文件。

- [ ] **Step 5: 推送并检查 Foundation CI**

```bash
git push -u origin codex/r0-legacy-project-migration-recovery-fixture
gh run list --workflow ci.yml --branch codex/r0-legacy-project-migration-recovery-fixture --limit 1
gh run watch <run-id> --exit-status
gh run view <run-id> --json conclusion,jobs,url
```

Expected:

- Ubuntu tests 成功；
- macOS 日志存在并通过 `Test legacy project migration on macOS`；
- Windows 完整 Go tests 和 production build 成功；
- 双平台 Wails inspection artifact 仍按前置计划生成。

- [ ] **Step 6: 记录完成证据并合入 main**

在本计划末尾记录 commit、CI run、关键测试和平台结果。完成验证后按项目工作流 fast-forward 合入 `main` 并等待 main Foundation CI。

- [ ] **Step 7: 标记完成**

只有分支与 main CI 都成功后：

- 本计划状态改为 `completed`；
- 路线图第 3 项改为 `completed`；
- 在 R0 段记录三项计划和最终 CI 证据；
- 确认 R0 退出标准全部满足，再开始 R1。

提交：

```bash
git add docs/plans/2026-08-06-r0-legacy-project-migration-recovery-fixture-implementation.md docs/roadmap/development-roadmap.md
git commit -m "docs: complete R0 legacy migration plan"
git push origin main
```

## 7. 验收矩阵

| 能力 | 自动证据 |
| --- | --- |
| 旧项目 fixture | v9 版本、Schema 指纹、固定业务哨兵、无二进制工件 |
| 正常升级 | v9→v14，业务哨兵保留，当前表存在 |
| 升级前备份 | 独立可打开、v9、相同 project ID |
| stage 隔离 | 注入 migration/validation 失败时原库 hash 与数据不变 |
| 激活回滚 | 注入 reopen 失败时 original 恢复 |
| 中断恢复 | prepared/activated/committed/damaged journal 表驱动测试 |
| 幂等 | 已升级项目重开不再备份、不再迁移 |
| Workspace 安全 | 失败打开不替换当前项目 |
| 隐私 | policy 扫描通过，无 `.db/.zjproject` 被跟踪 |
| 跨平台 | Ubuntu race、macOS targeted、Windows full Go tests |

## 8. Windows/macOS 注意事项

- 文件名全部固定且短，不使用作品名、章节名或绝对路径写入 journal；
- 所有 SQLite 连接必须在 rename 前关闭，不能依赖 Unix 可重命名已打开文件的行为；
- Windows 上 Defender/OneDrive 短时占用应返回可识别写入/恢复错误，不能删除 original；重试策略和同步盘完整矩阵留到 R8；
- macOS/Linux 对目录执行 `fsync`，确保 rename 顺序落盘；
- 不用 `os.Rename` 覆盖未知目标；每个固定 operation path 在使用前验证其角色；
- 不复制活动的裸 `project.db`，一致性备份必须来自 SQLite backup/VACUUM 机制；
- 路径和错误日志不输出用户标题或正文，测试只使用临时路径。

## 9. 回滚与支持策略

- 代码回滚不会自动降级已升级为 v14 的数据库；
- 已成功升级项目仍可继续由当前版本打开，`recovery/migration-v09-v14.sqlite` 用于支持恢复；
- 若新迁移实现需要撤回，先停止发布并保留 safe opener；不得恢复“直接在原库 migration”的旧行为；
- migration 失败时应用返回错误，原项目不被激活，当前工作区保持不变；
- 恢复错误现场必须整体复制 `.zjproject`，不得只上传或替换 `project.db`；
- 任何需要读取用户正文的支持操作都必须另行取得作者明确授权。

## 10. 完成定义

本计划只有在以下条件全部满足时才能标记 `completed`：

1. v9 文本 fixture 可在临时目录确定性生成，仓库不含二进制项目；
2. 旧项目仅在 stage 上迁移，原库在迁移与验证阶段不被修改；
3. 升级前备份可独立打开并保留；
4. stage 失败、激活失败和三类中断现场均有自动恢复测试；
5. v9 业务哨兵经 v14 迁移和 Workspace 打开后完整保留；
6. 已是 v14、future、corrupt、missing 的现有行为不退化；
7. Ubuntu/macOS/Windows CI 全部通过；
8. 测试与恢复文档已更新；
9. main 分支 Foundation CI 通过；
10. R0 三项计划的退出证据完整。
