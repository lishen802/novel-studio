# 旧项目迁移测试基线

本文档定义旧版 `.zjproject` 数据库安全升级的测试 fixture、状态机、恢复矩阵和三平台 CI 入口。适用于工程和支持人员；用户可见的恢复行为见[项目恢复与数据保护](../operations/project-recovery.md)。

## Fixture 为什么是文本规格而非二进制数据库

仓库不提交 `.db`、`.zjproject`、`.db-wal` 或 `.db-shm`，遵守 `fixtures/README.md` 和 tracked artifact policy 的隐私与版权约束。迁移测试使用合成文本规格：

- `fixtures/migrations/v09/seed.sql`：固定 ID、固定 UTC 时间、少量合成中文故事数据（如“雾港”“林澈”“旧铜钥匙”），不来自真实作者项目或已出版/网络小说。
- `fixtures/migrations/v09/expected.json`：升级后必须保留的业务哨兵。
- `fixtures/migrations/v09/README.md`：fixture 版本、来源和覆盖的迁移链。

`internal/testsupport/legacyproject.Build(t, parentDir)` 在测试临时目录读取这些规格，用仓库内已版本化的 Goose migration 1–9 建库，执行 `seed.sql` 后 checkpoint 并移除 WAL/SHM，生成自包含的 `legacy-v09.zjproject/project.db`。两次生成得到相同业务行和 `expected.json`，但不比较 SQLite 文件字节（VACUUM 和 WAL 不保证字节稳定性）。

## v9 → v14 覆盖范围

| 迁移 | 覆盖内容 |
| --- | --- |
| v10 | 结构化 change_operations：`position`、`base_fingerprint`、`evidence` 默认 `"[]"` |
| v11 | Agent runtime：`agent_runs`、`agent_run_steps`、`agent_artifacts` |
| v12 | 手稿导入：`import_jobs`、`import_chapters` |
| v13 | 抽取运行时：`extraction_tasks`、`extraction_steps`、`extraction_artifacts` |
| v14 | change_operations `evidence` 列默认 `"[]"` |

### 新增 fixture 规则

1. 新增 fixture 只覆盖新的迁移版本链（如 v14→v15），不重建已有版本；
2. `seed.sql` 必须使用固定 ID 和 UTC 时间，不使用随机数或 `time.Now()`；
3. `expected.json` 必须记录升级后保留的业务哨兵；
4. fixture 内容必须是原创合成，不得来自真实作者或已出版作品；
5. 通过 `tracked-artifact-policy.mjs` 校验不引入二进制数据库。

## 迁移三阶段文件状态

| 阶段 | `project.db` | `migration-original.sqlite` | `migration-stage.sqlite` | `migration.json` | `migration-v09-v14.sqlite` |
| --- | --- | --- | --- | --- | --- |
| prepared | 原库 | 不存在 | 已迁移校验 | phase=prepared | 已创建 |
| activated | 新库（原 stage） | 原库 | 不存在（已 rename） | phase=activated | 保留 |
| committed | 新库（已校验） | 不存在（已删除） | 不存在 | phase=committed | 保留 |
| 完成 | 新库 | 不存在 | 不存在 | 不存在 | 保留 |

## 恢复矩阵

`recoverInterruptedMigration` 按以下规则恢复中断的迁移现场。“无法判定时停止”优先于猜测和覆盖。

| 现场 | 动作 |
| --- | --- |
| 无 recovery 目录或无 journal、无 original | 无需恢复 |
| 无 journal，但 `migration-original.sqlite` 存在 | 按明确的“original 优先”恢复原库，删除新库、stage 和 journal |
| journal schema=1, phase=prepared | 原库仍在 `project.db`；删除 stage 和 journal，保留备份 |
| journal schema=1, phase=activated | 原库在 `migration-original.sqlite`；删除新库，rename original 回 `project.db`，删除 stage 和 journal |
| journal schema=1, phase=committed | 新库在 `project.db`，无 original；校验 Schema 和 quick_check 后只清 journal |
| journal 损坏（非 JSON） | 返回 `ErrDatabaseMigrationRecovery`，不删除任何文件 |
| journal schema 未知 | 返回 `ErrDatabaseMigrationRecovery`，不删除任何文件 |
| journal phase 未知 | 返回 `ErrDatabaseMigrationRecovery`，不删除任何文件 |

## 三平台 CI 入口

| 平台 | CI Job | 运行的测试 |
| --- | --- | --- |
| Ubuntu | `Ubuntu tests` → `Run Go race tests` | `go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...` |
| macOS | `macOS production build` → `Test legacy project migration on macOS` | `go test -count=1 ./internal/testsupport/legacyproject ./internal/platform/projectfs ./internal/persistence/sqlite ./internal/application/workspace` |
| Windows | `Windows production build` → `Run Go tests on Windows` | `go test -count=1 ./apps/desktop ./internal/...` |

Ubuntu race tests 和 Windows full Go tests 已覆盖全部 package；macOS 额外的命名步骤让旧项目迁移失败阶段直接在 Job 日志可见，无需检查 Wails build 输出。

## 失败日志约束

- 测试输出不得打印 seed 正文、SQL dump、绝对路径或数据库文件内容；
- 文件身份比较使用 SHA-256，不输出原始字节；
- 测试只使用临时路径（`t.TempDir()`），不写入仓库；
- 迁移 journal 只保存 schema 版本、from/to 版本和 phase，不保存路径、正文或凭据。

## 手工生成 fixture 诊断

在临时目录手工生成 v9 `.zjproject` 进行诊断：

```bash
go test -count=1 -run TestBuildsDeterministicV9Fixture -v ./internal/testsupport/legacyproject
```

或在 Go 测试中调用 `legacyproject.Build(t, t.TempDir())` 获取 `Fixture.PackagePath` 和 `Fixture.DatabasePath`，用 `sqlite3` 命令行或 DB Browser 检查生成的 `project.db`。

诊断完成后删除整个临时目录；不要将生成的 `.zjproject` 提交到仓库。
