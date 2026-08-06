# v09 Legacy Project Migration Fixture

## 用途

为旧版 `.zjproject` 数据库迁移（v9 → v14）提供确定性、无版权风险的合成测试规格。`internal/testsupport/legacyproject` 在测试临时目录读取本目录的 `seed.sql` 与 `expected.json`，生成完整的 v9 `.zjproject` 包，用于验证：

- 升级前一致性备份；
- stage 迁移与原子替换；
- 迁移失败和中断后的恢复；
- v9 业务哨兵经 v14 迁移和 Workspace 打开后完整保留。

## 文件

| 文件 | 作用 |
| --- | --- |
| `seed.sql` | 在 Goose `UpTo(ctx, 9)` 后执行的合成数据：1 个项目、1 章已提交正文（revision=2）、1 条历史 revision、1 条恢复草稿、1 个人物、1 条世界规则、1 条 canonical fact、1 个 pending change set 和 operation。 |
| `expected.json` | 升级后必须保留的业务哨兵；`legacyproject` 测试用它和数据库实际行做严格比较。 |

## 合成规则

- 所有 ID、标题、正文均为原创合成内容（雾港、林澈、旧铜钥匙），不来自真实作者项目或已出版/网络小说；
- 时间戳固定为 UTC RFC3339Nano（`2024-01-01T00:00:00Z` / `2024-01-02T00:00:00Z`），不使用 `CURRENT_TIMESTAMP`、随机数或平台特定时钟；
- 不提交二进制 `.db` / `.zjproject` / `-wal` / `-shm`；测试运行时由 `legacyproject.Build` 在 `t.TempDir()` 生成数据库；
- 不包含绝对路径、API Key、真实凭据或用户正文。

## 覆盖的迁移链

v9 fixture 覆盖以下迁移路径：

| 迁移 | 覆盖点 |
| --- | --- |
| v10 结构化 change_sets/change_operations | `legacy-change-set-1` / `legacy-change-op-1`（`replace_chapter_body`，base_revision=2） |
| v11 agent_runtime_settings | 不需要 v9 哨兵，验证表新增不破坏现有行 |
| v12 manuscript_import | 不需要 v9 哨兵，验证表新增不破坏现有行 |
| v13 extraction_runtime | 不需要 v9 哨兵，验证 `change_operations.evidence` 默认值和 `import_jobs` 新列 |
| v14 extraction_runtime（保留） | 同上 |

## 增加新 fixture

当需要覆盖 v10/v11/v12 起点的迁移时，新增 `fixtures/migrations/v10/` 等目录，遵循相同的 seed/expected/README 三文件结构，并在 `internal/testsupport/legacyproject` 增加对应 builder。不要在本目录混合多个版本。
