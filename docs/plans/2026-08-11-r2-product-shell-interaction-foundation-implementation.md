# R2 产品壳与交互基础 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不实现原创生成、批次生成和整书档案聚合业务的前提下，建立应用级设置、最近项目首页、原生菜单、分级作品导航、权威工作台投影和阶段自适应 AI 面板，为 R3–R10 提供稳定的桌面产品壳。

**Architecture:** 新增独立于 `.zjproject` 的应用设置数据库与应用服务，项目数据库只保存所选创作方案版本、项目覆盖和运行冻结快照；凭据继续留在 OS 凭据库。Wails binding 只投影应用服务与工作区服务的 DTO，React 通过一个应用壳 store 和现有 workspace store 消费权威状态，不读取 SQLite、文件系统或 Provider。

**Tech Stack:** Go 1.25、Wails v2.13、React 19、TypeScript、Vite、Zustand、Zod、SQLite/modernc、Goose、Vitest、Testing Library

---

- 状态：`in_progress`
- 所属阶段：[R2 产品壳、应用设置与交互基础](../roadmap/development-roadmap.md#7-r2产品壳应用设置与交互基础)
- 上游文档：[产品需求](../product/requirements.md)、[产品与交互设计](../product/design.md)、[系统技术设计](../architecture/system-design.md)
- 架构约束：[Go-first](../architecture/0002-go-first-wails-stack.md)、[Wails 目录](../architecture/0003-wails-project-layout.md)

## 范围与边界

本计划交付可运行、可测试的 R2 纵切：应用设置、模型连接/创作方案版本、最近项目、原生菜单、首页、项目壳、工作台投影、AI 面板状态和导入后的正文入口。它明确不生成三套方向、作品方案、路线图、批次正文或整书档案；这些入口必须返回结构化的 `not_available`/`not_generated` 状态，不能用假数据制造已完成效果。

应用设置数据库位于操作系统应用数据目录，不能放进任一 `.zjproject`。项目数据库继续保留现有 `provider_connections`、`agent_profiles` 和运行记录，以便旧任务可解释；迁移后它们只作为旧配置来源和运行冻结快照，不再作为应用默认配置的权威来源。

## 文件职责图

```text
internal/domain/appsettings/          应用设置、方案版本、最近项目的纯领域类型
internal/application/appsettings/     应用设置用例、版本发布、最近项目和旧配置导入
internal/application/ports/           应用设置仓储、凭据和项目配置迁移端口
internal/persistence/appsqlite/        settings.db、Goose 迁移和 repository
internal/platform/appdirs/             Windows/macOS 应用数据目录解析
internal/application/workspace/        项目方案 pin、项目覆盖、WorkbenchProjection
apps/desktop/                           组合根、原生菜单、薄 binding 和安全错误映射
apps/desktop/frontend/src/app/         首页、应用设置抽屉、菜单事件和应用壳 store
apps/desktop/frontend/src/features/    分级项目导航、阶段画布、AI 面板和导入入口
```

### Task 1: 建立应用数据目录与 `settings.db`

**Files:**
- Create: `internal/platform/appdirs/appdirs.go`
- Create: `internal/platform/appdirs/appdirs_test.go`
- Create: `internal/domain/appsettings/model.go`
- Create: `internal/domain/appsettings/model_test.go`
- Create: `internal/persistence/appsqlite/database.go`
- Create: `internal/persistence/appsqlite/database_test.go`
- Create: `internal/persistence/appsqlite/migrations/00001_application_settings.sql`
- Create: `internal/persistence/appsqlite/repository.go`
- Create: `internal/persistence/appsqlite/repository_test.go`

- [ ] **Step 1: 写应用目录与 schema 的失败测试**

```go
func TestResolveCreatesNoDirectoryAndUsesStableSettingsPath(t *testing.T) {
	root := t.TempDir()
	paths, err := appdirs.Resolve(func() (string, error) { return root, nil })
	if err != nil { t.Fatal(err) }
	if got, want := paths.Database, filepath.Join(root, "StoryStudio", "settings.db"); got != want {
		t.Fatalf("Database = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Dir(paths.Database)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Resolve mutated filesystem: %v", err)
	}
}

func TestOpenCreatesVersionedApplicationSchema(t *testing.T) {
	db, err := appsqlite.Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	for _, table := range []string{"app_preferences", "provider_connections", "creative_profile_versions", "creative_profile_roles", "recent_projects", "legacy_imports"} {
		assertTableExists(t, db, table)
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/platform/appdirs ./internal/domain/appsettings ./internal/persistence/appsqlite -count=1`

Expected: FAIL，提示三个新 package 或 `Resolve`/`Open` 尚不存在。

- [ ] **Step 3: 实现最小领域类型、目录解析和迁移入口**

`appdirs.Resolve` 只计算路径；`appsqlite.Open` 才创建父目录、设置 `foreign_keys=ON`、`busy_timeout=5000`、`journal_mode=WAL`，运行内嵌 Goose 迁移并执行 `quick_check`。首版领域类型固定如下，后续任务复用同名字段：

```go
type Preferences struct {
	Theme                   Theme
	DefaultCreativeProfileID string
	UpdatedAt               time.Time
}

type ProviderConnection struct {
	ID, Name, Kind, BaseURL, CredentialRef string
	CreatedAt, UpdatedAt time.Time
}

type CreativeProfileVersion struct {
	ProfileID string
	Version int64
	Name string
	Roles []RoleBinding
	MaxRunTokens, MaxContextChars int
	CreatedAt time.Time
}

type RecentProject struct {
	ProjectID, Title, Path, LastWorkspace string
	LastOpenedAt time.Time
	Missing bool
}
```

迁移使用不可变方案版本：`creative_profile_versions` 主键为 `(profile_id, version)`，角色主键为 `(profile_id, version, role)`；`recent_projects.path` 唯一；`legacy_imports` 以项目 ID 和来源摘要保证幂等。任何表都不得保存 API Key、正文或 Prompt。

- [ ] **Step 4: 实现 repository 并覆盖事务与并发**

```go
type Repository interface {
	Preferences(context.Context) (appsettings.Preferences, error)
	SavePreferences(context.Context, appsettings.Preferences) error
	UpsertProvider(context.Context, appsettings.ProviderConnection) error
	Providers(context.Context) ([]appsettings.ProviderConnection, error)
	PublishProfileVersion(context.Context, appsettings.CreativeProfileVersion) error
	ProfileVersion(context.Context, string, int64) (appsettings.CreativeProfileVersion, error)
	RecordRecentProject(context.Context, appsettings.RecentProject) error
	RecentProjects(context.Context, int) ([]appsettings.RecentProject, error)
	MarkProjectMissing(context.Context, string, bool) error
}
```

Repository 测试必须证明：版本只增不改、一次事务写入三个角色、同一路径只保留一项最近项目、并发 upsert 不产生重复行、关闭重开后数据一致。

- [ ] **Step 5: 运行 GREEN 和 race 测试**

Run: `go test -race -shuffle=on ./internal/platform/appdirs ./internal/domain/appsettings ./internal/persistence/appsqlite -count=1`

Expected: PASS，且无 race 或 SQLite lock 泄漏。

- [ ] **Step 6: 提交**

```bash
git add internal/platform/appdirs internal/domain/appsettings internal/persistence/appsqlite
git commit -m "feat: add application settings database"
```

### Task 2: 建立应用设置服务、OS 凭据引用与方案版本

**Files:**
- Create: `internal/application/ports/app_settings.go`
- Create: `internal/application/appsettings/service.go`
- Create: `internal/application/appsettings/service_test.go`
- Modify: `internal/platform/credentials/store.go`
- Modify: `internal/platform/credentials/store_test.go`

- [ ] **Step 1: 写应用设置用例的失败测试**

```go
func TestPublishDefaultProfileCreatesNewVersionWithoutChangingOldVersion(t *testing.T) {
	service := newServiceHarness(t)
	v1 := mustPublish(t, service, profileInput("默认方案", "writer-model-v1"))
	v2 := mustPublish(t, service, profileInput("默认方案", "writer-model-v2"))
	if v1.Version != 1 || v2.Version != 2 { t.Fatalf("versions = %d, %d", v1.Version, v2.Version) }
	if got := mustVersion(t, service, v1.ProfileID, 1).Roles[1].Model; got != "writer-model-v1" {
		t.Fatalf("v1 was mutated: %q", got)
	}
}

func TestSaveProviderStoresOnlyCredentialReferenceInDatabase(t *testing.T) {
	service, repo, secrets := newServiceHarnessWithSpies(t)
	provider, err := service.SaveProvider(context.Background(), ProviderInput{Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "secret"})
	if err != nil { t.Fatal(err) }
	if strings.Contains(fmt.Sprint(repo.savedProvider), "secret") { t.Fatal("secret reached repository") }
	if secrets.savedRef != provider.CredentialRef { t.Fatal("credential reference mismatch") }
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/application/appsettings ./internal/platform/credentials -count=1`

Expected: FAIL，提示 `appsettings.Service`、应用设置 ports 或凭据命名方法尚不存在。

- [ ] **Step 3: 实现应用设置 ports 和 service**

```go
type CredentialStore interface {
	Save(context.Context, string, string) error
	Get(context.Context, string) (string, error)
	Delete(context.Context, string) error
}

type Service struct {
	repo ports.AppSettingsRepository
	credentials ports.CredentialStore
	clock ports.Clock
	ids ports.IDGenerator
}
```

`SaveProvider` 生成 `app-provider:<uuid>` 引用后先写 OS 凭据，再写非敏感 metadata；metadata 失败时删除刚创建的凭据，更新已有 provider 时保留原引用。`PublishCreativeProfile` 校验 planner/writer/reviewer 均存在，且写作与连续性审阅不能使用同一连接和同一模型；发布新版本后才可更新默认指针。全局默认变化只改变 `Preferences.DefaultCreativeProfileID`，不接触项目数据库。

- [ ] **Step 4: 为删除、凭据失败和版本事务补齐测试**

```go
func TestSaveProviderRollsBackNewCredentialWhenMetadataWriteFails(t *testing.T) {
	service, _, secrets := newServiceHarnessWithRepositoryError(t, errors.New("write failed"))
	_, err := service.SaveProvider(context.Background(), ProviderInput{Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "secret"})
	if err == nil || len(secrets.deletedRefs) != 1 || secrets.deletedRefs[0] != secrets.savedRef { t.Fatalf("credential rollback = %#v, err = %v", secrets, err) }
}

func TestDeleteProviderRejectsProfileReferences(t *testing.T) {
	service := newServiceHarness(t)
	provider := mustSaveProvider(t, service)
	mustPublish(t, service, profileUsing(provider.ID))
	if err := service.DeleteProvider(context.Background(), provider.ID); !errors.Is(err, ErrProviderInUse) { t.Fatalf("DeleteProvider() error = %v", err) }
}

func TestPublishProfileRejectsSameWriterAndReviewerModel(t *testing.T) {
	service := newServiceHarness(t)
	_, err := service.PublishCreativeProfile(context.Background(), sameWriterReviewerInput())
	if !errors.Is(err, agent.ErrReviewerNotIndependent) { t.Fatalf("PublishCreativeProfile() error = %v", err) }
}

func TestPreferencesDefaultChangeDoesNotMutatePublishedVersions(t *testing.T) {
	service := newServiceHarness(t)
	v1 := mustPublish(t, service, profileInput("方案 A", "writer-a"))
	v2 := mustPublish(t, service, profileInput("方案 B", "writer-b"))
	mustSetDefault(t, service, v2.ProfileID)
	if got := mustVersion(t, service, v1.ProfileID, v1.Version).Roles[1].Model; got != "writer-a" { t.Fatalf("old version model = %q", got) }
}
```

每个测试写出完整 fake 行为，不用真实系统钥匙串或网络。

- [ ] **Step 5: 运行 GREEN**

Run: `go test -race -shuffle=on ./internal/application/appsettings ./internal/platform/credentials -count=1`

Expected: PASS，凭据值不会出现在错误、日志或 repository spy 中。

- [ ] **Step 6: 提交**

```bash
git add internal/application/ports/app_settings.go internal/application/appsettings internal/platform/credentials
git commit -m "feat: add versioned creative profiles"
```

### Task 3: 为项目保存方案 pin、覆盖和权威工作台投影

**Files:**
- Create: `internal/persistence/sqlite/migrations/00017_project_workbench_profile.sql`
- Modify: `internal/persistence/sqlite/migration_test.go`
- Create: `internal/domain/project/workbench.go`
- Create: `internal/domain/project/workbench_test.go`
- Modify: `internal/application/ports/project_store.go`
- Modify: `internal/persistence/sqlite/project_store.go`
- Modify: `internal/persistence/sqlite/project_store_test.go`
- Create: `internal/application/workspace/workbench.go`
- Create: `internal/application/workspace/workbench_test.go`
- Modify: `internal/application/workspace/service.go`
- Modify: `internal/application/workspace/service_test.go`
- Modify: `internal/application/workspace/view.go`

- [ ] **Step 1: 写迁移和新项目 pin 的失败测试**

```go
func TestCreateProjectPinsCurrentDefaultProfileVersion(t *testing.T) {
	defaults := &fakeProjectDefaults{selection: ports.ProjectProfileSelection{ProfileID: "profile-1", Version: 3}}
	service := newServiceWithProjectDefaults(t, defaults)
	view, err := service.CreateProject(context.Background(), t.TempDir(), "新书", "奇幻", "")
	if err != nil { t.Fatal(err) }
	if view.ProfileSelection.ProfileID != "profile-1" || view.ProfileSelection.Version != 3 {
		t.Fatalf("selection = %#v", view.ProfileSelection)
	}
	defaults.selection.Version = 4
	if got := mustProjectView(t, service).ProfileSelection.Version; got != 3 {
		t.Fatalf("existing project moved to default v%d", got)
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/domain/project ./internal/persistence/sqlite ./internal/application/workspace -run 'Profile|Workbench' -count=1`

Expected: FAIL，提示 profile selection、迁移表或 `WorkbenchProjection` 尚不存在。

- [ ] **Step 3: 增加项目 schema 与领域状态**

迁移创建单行 `project_creative_profile` 与 `project_workbench_state`。前者保存 `profile_id`、`profile_version`、`role_overrides_json` 和更新时间；后者保存受控 stage、最近画布、AI 面板状态和更新时间。`role_overrides_json` 只允许连接 ID、模型、预算与启用状态，不允许 API Key。

```go
type WorkbenchStage string
const (
	StageIdea WorkbenchStage = "idea"
	StageProposal WorkbenchStage = "proposal"
	StageRoadmap WorkbenchStage = "roadmap"
	StageBatchRunning WorkbenchStage = "batch_running"
	StageReview WorkbenchStage = "review"
	StageWriting WorkbenchStage = "writing"
)

type WorkbenchProjection struct {
	Stage WorkbenchStage `json:"stage"`
	Canvas string `json:"canvas"`
	AIPanel string `json:"aiPanel"`
	PendingReviewCount int `json:"pendingReviewCount"`
	RecoverableJobCount int `json:"recoverableJobCount"`
	ArchiveStatus string `json:"archiveStatus"`
}
```

Stage 由持久化项目状态、导入状态、运行任务和候选数量投影；React 不可提交任意 stage。R2 中尚未实现的原创状态返回 `idea` + `idea_not_available` 画布，导入已应用项目返回 `writing` + `manuscript`，整书档案状态固定为 `not_generated`。

- [ ] **Step 4: 注入只读默认方案端口并原子保存 pin**

```go
type ProjectDefaults interface {
	DefaultProfileSelection(context.Context) (ProjectProfileSelection, error)
}

func (s *Service) ConfigureProjectDefaults(defaults ports.ProjectDefaults) {
	s.projectDefaults = defaults
}
```

`CreateProject` 在新项目 store 创建后、安装 active workspace 前读取默认方案并写入 pin；写入失败要关闭 store、释放 lease 并返回错误，不能留下已打开的半配置项目。`OpenProject` 不读取当前默认覆盖已有 pin。

- [ ] **Step 5: 覆盖只读、重挂载和故障回滚**

```go
func TestWorkbenchProjectionSurvivesServiceRestart(t *testing.T) {
	path, before := createProjectWithWorkbench(t, StageWriting, "manuscript")
	after := reopenAndReadWorkbench(t, path)
	if diff := cmp.Diff(before, after); diff != "" { t.Fatalf("projection mismatch (-before +after):\n%s", diff) }
}

func TestOpenProjectNeverUpgradesPinnedProfileImplicitly(t *testing.T) {
	path := createProjectPinnedTo(t, 3)
	service := openWithDefaultVersion(t, path, 4)
	if got := mustProjectView(t, service).ProfileSelection.Version; got != 3 { t.Fatalf("profile version = %d", got) }
}

func TestCreateProjectProfilePinFailureReleasesLease(t *testing.T) {
	h := newLifecycleHarness(t, &fakeProjectDefaults{err: errors.New("settings unavailable")})
	_, err := h.service.CreateProject(context.Background(), t.TempDir(), "新书", "", "")
	if err == nil { t.Fatal("CreateProject() succeeded") }
	if diff := cmp.Diff([]string{"store.close", "lease.release"}, h.calls); diff != "" { t.Fatalf("cleanup order mismatch: %s", diff) }
}

func TestReadOnlyProjectReturnsProjectionWithoutWritingState(t *testing.T) {
	path := createProjectPinnedTo(t, 3)
	before := fileSHA256(t, projectDatabasePath(path))
	service := openReadOnlyService(t, path)
	_ = mustWorkbench(t, service)
	if after := fileSHA256(t, projectDatabasePath(path)); after != before { t.Fatalf("read-only checksum changed: %s -> %s", before, after) }
}
```

- [ ] **Step 6: 运行完整受影响测试**

Run: `go test -race -shuffle=on ./internal/domain/project ./internal/persistence/sqlite ./internal/application/workspace -count=1`

Expected: PASS；旧 fixture 自动迁移到 v17，未来 schema 仍被拒绝，只读打开不执行迁移。

- [ ] **Step 7: 提交**

```bash
git add internal/domain/project internal/application/ports/project_store.go internal/application/workspace internal/persistence/sqlite
git commit -m "feat: pin project profiles and workbench state"
```

### Task 4: 幂等迁移旧项目模型配置到应用设置

**Files:**
- Create: `internal/application/ports/legacy_agent_settings.go`
- Create: `internal/application/appsettings/legacy_import.go`
- Create: `internal/application/appsettings/legacy_import_test.go`
- Modify: `internal/application/workspace/agent.go`
- Modify: `internal/application/workspace/agent_test.go`
- Modify: `internal/persistence/sqlite/agent_store.go`
- Modify: `internal/persistence/sqlite/agent_store_test.go`

- [ ] **Step 1: 写旧项目导入失败测试**

```go
func TestImportLegacyProjectCreatesOneProfileAndPreservesCredentialRefs(t *testing.T) {
	legacy := fakeLegacySource{
		projectID: "project-1",
		connections: []ports.LegacyProvider{{ID: "p1", Name: "旧连接", Kind: "openai", BaseURL: "https://api.openai.com/v1", CredentialRef: "provider:p1"}},
		profiles: completeLegacyRoles("p1"),
	}
	service := newMigrationHarness(t)
	first, err := service.ImportLegacyProject(context.Background(), legacy)
	if err != nil { t.Fatal(err) }
	second, err := service.ImportLegacyProject(context.Background(), legacy)
	if err != nil { t.Fatal(err) }
	if first.ProfileVersion != second.ProfileVersion { t.Fatalf("migration not idempotent: %#v %#v", first, second) }
	if got := mustProvider(t, service, first.ProviderIDs[0]).CredentialRef; got != "provider:p1" {
		t.Fatalf("credential ref = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/application/appsettings ./internal/application/workspace ./internal/persistence/sqlite -run Legacy -count=1`

Expected: FAIL，提示 legacy export/import contract 尚不存在。

- [ ] **Step 3: 实现无密钥的 legacy export 与 source digest**

```go
type LegacyAgentSettingsSnapshot struct {
	ProjectID string
	Connections []LegacyProvider
	Profiles []LegacyRoleBinding
	MaxRunTokens int
	MaxContextChars int
	SourceDigest string
}
```

Workspace 只导出非敏感字段和既有 `CredentialRef`；`SourceDigest` 对排序后的 metadata 做 SHA-256，不读取凭据值。应用设置服务在一个事务中完成：检查 `legacy_imports(project_id, source_digest)`、按 `(kind, base_url, credential_ref)` 去重 provider、发布命名为“从《作品名》迁移”的方案版本、回写 import record。旧项目 provider/profile 表不删除，既有 run/step 外键继续有效。

- [ ] **Step 4: 规定迁移触发和失败语义**

首次可编辑打开旧项目且尚无 `project_creative_profile` 时执行迁移并写 pin；只读打开只报告 `migration_required`，不写项目或 settings.db。应用设置写成功但项目 pin 失败时，下次打开复用同一 `legacy_imports` 结果重试 pin，不新建第二份 profile。缺失 OS 凭据时 provider 保留 `hasCredential=false`，UI 要求重新输入，不把项目判为损坏。

```go
func TestLegacyImportSettingsSuccessPinFailureRetriesWithoutDuplicateVersion(t *testing.T) {
	h := newLegacyOpenHarness(t, failFirstProjectPin())
	if _, err := h.openEditable(); err == nil { t.Fatal("first open succeeded") }
	if _, err := h.openEditable(); err != nil { t.Fatal(err) }
	if got := h.settings.profileVersionCount(); got != 1 { t.Fatalf("profile versions = %d", got) }
}

func TestLegacyReadOnlyOpenDoesNotWriteEitherDatabase(t *testing.T) {
	h := newLegacyOpenHarness(t)
	projectBefore, settingsBefore := h.checksums()
	if _, err := h.openReadOnly(); err != nil { t.Fatal(err) }
	projectAfter, settingsAfter := h.checksums()
	if projectAfter != projectBefore || settingsAfter != settingsBefore { t.Fatal("read-only open mutated storage") }
}

func TestLegacyMissingCredentialImportsSafeMetadata(t *testing.T) {
	h := newLegacyMigrationHarness(t, credentials.ErrCredentialNotFound)
	result, err := h.importProject()
	if err != nil { t.Fatal(err) }
	if result.Providers[0].HasCredential { t.Fatal("missing credential reported as present") }
}

func TestLegacySourceChangeCreatesExplicitNewVersion(t *testing.T) {
	h := newLegacyMigrationHarness(t, nil)
	first, err := h.importProject(); if err != nil { t.Fatal(err) }
	h.source.Profiles[0].Model = "changed-model"
	second, err := h.importProject(); if err != nil { t.Fatal(err) }
	if second.ProfileVersion != first.ProfileVersion+1 { t.Fatalf("versions = %d, %d", first.ProfileVersion, second.ProfileVersion) }
}
```

- [ ] **Step 5: 运行迁移、恢复和 race 测试**

Run: `go test -race -shuffle=on ./internal/application/appsettings ./internal/application/workspace ./internal/persistence/sqlite ./internal/testsupport/legacyproject -count=1`

Expected: PASS；旧项目正文与运行记录 checksum 不变，迁移失败可重试且不复制凭据。

- [ ] **Step 6: 提交**

```bash
git add internal/application/ports/legacy_agent_settings.go internal/application/appsettings internal/application/workspace internal/persistence/sqlite
git commit -m "feat: migrate legacy model settings safely"
```

### Task 5: 组合应用壳服务、最近项目与原生菜单

**Files:**
- Create: `internal/application/appshell/service.go`
- Create: `internal/application/appshell/service_test.go`
- Modify: `apps/desktop/app.go`
- Modify: `apps/desktop/main.go`
- Create: `apps/desktop/menu.go`
- Create: `apps/desktop/menu_test.go`
- Modify: `apps/desktop/api.go`
- Modify: `apps/desktop/api_test.go`
- Modify: `apps/desktop/errors.go`

- [ ] **Step 1: 写最近项目和菜单命令的失败测试**

```go
func TestOpenProjectRecordsRecentOnlyAfterSuccessfulOpen(t *testing.T) {
	workspace := &fakeWorkspace{openResult: workspace.ProjectView{ID: "p1", Title: "长夜", Path: "/books/长夜.zjproject"}}
	settings := &fakeAppSettings{}
	shell := appshell.New(workspace, settings)
	if _, err := shell.OpenProject(context.Background(), "/books/长夜.zjproject", ports.ProjectAccessEditable); err != nil { t.Fatal(err) }
	if got := settings.recorded.Path; got != "/books/长夜.zjproject" { t.Fatalf("recent path = %q", got) }
}

func TestApplicationMenuContainsOnlyApplicationCommands(t *testing.T) {
	model := buildMenuModel()
	assertMenuItems(t, model.Application, "关于织境", "设置…", "检查更新…", "隐藏织境", "退出织境")
	assertMenuOmits(t, model.Application, "新建原创作品…", "打开项目…", "项目设置…")
	assertMenuContains(t, model.File, "新建原创作品…", "导入小说并续写…", "打开项目…", "打开最近项目", "关闭当前项目", "项目设置…", "导出…", "项目维护…")
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/application/appshell ./apps/desktop -run 'Recent|Menu|ApplicationSettings' -count=1`

Expected: FAIL，提示 appshell、菜单模型和应用设置 DesktopAPI 尚不存在。

- [ ] **Step 3: 实现 appshell 生命周期协调**

```go
type WorkspaceLifecycle interface {
	CreateProject(context.Context, string, string, string, string) (workspace.ProjectView, error)
	OpenProject(context.Context, string) (workspace.ProjectView, error)
	OpenProjectReadOnly(context.Context, string) (workspace.ProjectView, error)
	Close() error
}

type Settings interface {
	RecentProjects(context.Context, int) ([]appsettings.RecentProject, error)
	RecordRecentProject(context.Context, appsettings.RecentProject) error
	MarkRecentMissing(context.Context, string, bool) error
}
```

appshell 在 create/open 成功后记录最近项目；记录失败返回安全 warning 字段但不得关闭刚打开项目。最近项目点击前先走现有项目路径校验和 lease/preflight；路径不存在时标为 missing 并返回 `PROJECT_RELOCATE_REQUIRED`，不能直接从首页删除记录。

- [ ] **Step 4: 用 Wails v2 menu package 构建原生菜单**

`main.go` 将 `options.App.Menu` 设为 `app.menu()`；`menu.go` 使用 `github.com/wailsapp/wails/v2/pkg/menu`。菜单回调只发送稳定命令事件，不直接操作 React 状态或 SQLite：

```go
const menuCommandEvent = "app:menu-command"

type MenuCommand struct { Command string `json:"command"` }

func (a *App) emitMenuCommand(command string) {
	runtime.EventsEmit(a.api.ctx, menuCommandEvent, MenuCommand{Command: command})
}
```

稳定命令值固定为 `new_original`、`import_novel`、`open_project`、`open_recent:<project-id>`、`close_project`、`app_settings`、`project_settings`、`toggle_navigation`、`toggle_ai_panel`、`focus_ai`。macOS 使用 `menu.AppMenu()` 角色补齐系统项；Windows 菜单使用相同业务命令，快捷键以 Wails `keys` 定义。

- [ ] **Step 5: 扩展 DesktopAPI 的应用设置投影**

```go
type ApplicationBootstrapView struct {
	Preferences appsettings.PreferencesView `json:"preferences"`
	RecentProjects []appsettings.RecentProjectView `json:"recentProjects"`
	Providers []appsettings.ProviderView `json:"providers"`
	Profiles []appsettings.CreativeProfileVersionView `json:"profiles"`
}
```

增加 `ApplicationBootstrap`、`SaveApplicationPreferences`、`SaveApplicationProvider`、`DeleteApplicationProvider`、`TestApplicationProvider`、`PublishCreativeProfile`、`SelectProjectCreativeProfile`、`ProjectWorkbench` 和 `CloseProject`。所有错误继续经 `safeDesktopError` 映射；API Key、绝对内部恢复路径、SQL 和 Provider 原始响应不得进入 JSON。

- [ ] **Step 6: 覆盖 close/菜单/最近项目错误路径**

```go
func TestRecentOpenLeaseConflictDoesNotAdvanceLastOpenedAt(t *testing.T) {
	settings := &fakeAppSettings{}
	shell := appshell.New(&fakeWorkspace{openErr: &ports.ProjectLeaseConflict{}}, settings)
	_, err := shell.OpenProject(context.Background(), "/books/a.zjproject", ports.ProjectAccessEditable)
	if err == nil || settings.recordCount != 0 { t.Fatalf("err = %v, recent writes = %d", err, settings.recordCount) }
}

func TestMissingRecentProjectReturnsRelocateAction(t *testing.T) {
	api := newDesktopAPIWithShell(&fakeShell{openErr: fs.ErrNotExist})
	_, err := api.OpenRecentProject("project-1")
	assertDesktopError(t, err, "PROJECT_RELOCATE_REQUIRED")
}

func TestCloseMenuWaitsForBeforeCloseFlushProtocol(t *testing.T) {
	app := newMenuTestApp()
	app.dispatchMenuCommand("close_project")
	if diff := cmp.Diff([]string{"emit:app:before-project-close"}, app.calls); diff != "" { t.Fatalf("calls mismatch: %s", diff) }
}

func TestBootstrapNeverSerializesCredentialValue(t *testing.T) {
	api := newDesktopAPIWithSettings(fakeSettingsContainingSecret("do-not-serialize"))
	view, err := api.ApplicationBootstrap(); if err != nil { t.Fatal(err) }
	encoded, err := json.Marshal(view); if err != nil { t.Fatal(err) }
	if bytes.Contains(encoded, []byte("do-not-serialize")) { t.Fatalf("secret serialized: %s", encoded) }
}
```

- [ ] **Step 7: 运行 Go 验证**

Run: `go test -race -shuffle=on ./internal/application/appshell ./internal/application/appsettings ./apps/desktop -count=1`

Expected: PASS；菜单单元测试不要求启动 GUI。

- [ ] **Step 8: 提交**

```bash
git add internal/application/appshell apps/desktop/app.go apps/desktop/main.go apps/desktop/menu.go apps/desktop/menu_test.go apps/desktop/api.go apps/desktop/api_test.go apps/desktop/errors.go
git commit -m "feat: add native application shell commands"
```

### Task 6: 实现最近项目首页与应用设置界面

**Files:**
- Create: `apps/desktop/frontend/src/app/app-shell-store.ts`
- Create: `apps/desktop/frontend/src/app/app-shell-store.test.ts`
- Create: `apps/desktop/frontend/src/features/home/HomeScreen.tsx`
- Create: `apps/desktop/frontend/src/features/home/HomeScreen.test.tsx`
- Create: `apps/desktop/frontend/src/features/settings/ApplicationSettings.tsx`
- Create: `apps/desktop/frontend/src/features/settings/ApplicationSettings.test.tsx`
- Modify: `apps/desktop/frontend/src/app/App.tsx`
- Modify: `apps/desktop/frontend/src/app/App.test.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`
- Modify: `apps/desktop/frontend/src/lib/studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/wails-studio-client.ts`
- Modify: `apps/desktop/frontend/src/lib/wails-studio-client.test.ts`
- Modify: `apps/desktop/frontend/src/lib/fake-studio-client.ts`

- [ ] **Step 1: 写首页与设置的失败测试**

```tsx
it("shows welcome actions without a redundant brand dropdown when recent list is empty", async () => {
  client.applicationBootstrapResult.recentProjects = [];
  render(<App client={client} />);
  expect(await screen.findByRole("button", { name: "从一个想法开始" })).toBeVisible();
  expect(screen.getByRole("button", { name: "导入小说并续写" })).toBeVisible();
  expect(screen.queryByRole("button", { name: "织境" })).not.toBeInTheDocument();
});

it("opens a recent project from its card", async () => {
  client.applicationBootstrapResult.recentProjects = [recentProject("project-1", "雾港来信")];
  render(<App client={client} />);
  await user.click(await screen.findByRole("button", { name: /继续《雾港来信》/ }));
  expect(client.openProjectCalls).toEqual([{ path: "/books/雾港来信.zjproject", mode: "editable" }]);
});
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd apps/desktop/frontend && npm test -- --run src/app/App.test.tsx src/features/home/HomeScreen.test.tsx src/features/settings/ApplicationSettings.test.tsx`

Expected: FAIL，提示 bootstrap DTO、HomeScreen 或 ApplicationSettings 尚不存在。

- [ ] **Step 3: 增加严格 Zod DTO 与应用壳 store**

```ts
export type AppShellStatus =
  | { kind: "loading" }
  | { kind: "home"; bootstrap: ApplicationBootstrapView }
  | { kind: "settings"; bootstrap: ApplicationBootstrapView }
  | { kind: "project"; project: ProjectView }
  | { kind: "error"; message: string; recovery?: AppRecoveryAction };
```

Zod schema 对 `recentProjects`、profile version、provider `hasCredential` 和 theme 使用严格枚举；原生返回省略可选字段时给空数组/安全默认，不为缺失业务状态生成假对象。store 保证同一时间只有一个 lifecycle 操作，最近项目打开失败仍保留首页列表和可恢复动作。

- [ ] **Step 4: 实现首页与应用设置**

HomeScreen 有三类入口：原创、导入、打开；最近项目存在时显示最近卡和上次位置，不存在时显示欢迎引导。路径只在辅助信息中显示，不把应用标题做成第二个下拉按钮。

ApplicationSettings 分为“模型连接”“默认创作方案”“外观”“高级”。模型连接表单提交 API Key 后立即清空输入；已保存连接只显示 `已保存凭据`/`需要凭据`。发布方案明确产生新版本；设置为默认时提示“只影响以后新建的作品”。

- [ ] **Step 5: 接入菜单事件且保持未保存状态**

```ts
client.onMenuCommand((command) => {
  switch (command.type) {
    case "app_settings": shell.openSettings(); break;
    case "open_project": void shell.chooseAndOpenProject(); break;
    case "import_novel": void shell.startImportEntry(); break;
    case "focus_ai": shell.requestAIFocus(); break;
  }
});
```

订阅在卸载时清理；设置抽屉关闭后恢复原首页/项目；菜单打开设置不能卸载 workspace store。`close_project` 先调用现有章节 flush，再调用 DesktopAPI `CloseProject`，成功后刷新 recent bootstrap。

- [ ] **Step 6: 覆盖错误、键盘和可访问性**

```tsx
it("offers relocate or remove when recent path is missing", async () => {
  client.openProjectError = desktopError("PROJECT_RELOCATE_REQUIRED");
  renderHomeWithRecent(client);
  await user.click(screen.getByRole("button", { name: /继续《雾港来信》/ }));
  expect(await screen.findByRole("button", { name: "重新定位" })).toBeVisible();
  expect(screen.getByRole("button", { name: "从最近项目移除" })).toBeVisible();
});

it("keeps project mounted while application settings is open", async () => {
  render(<App client={client} initialProject={project()} />);
  await user.type(await screen.findByRole("textbox", { name: "章节正文" }), "未保存内容");
  client.emitMenuCommand({ type: "app_settings" });
  await user.click(await screen.findByRole("button", { name: "关闭设置" }));
  expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveValue(expect.stringContaining("未保存内容"));
});

it("clears API key field after save and never renders the secret", async () => {
  render(<ApplicationSettings client={client} bootstrap={bootstrap()} />);
  const key = screen.getByLabelText("API Key");
  await user.type(key, "secret-value");
  await user.click(screen.getByRole("button", { name: "保存连接" }));
  await waitFor(() => expect(key).toHaveValue(""));
  expect(screen.queryByText("secret-value")).not.toBeInTheDocument();
});

it("announces profile version publication and future-project scope", async () => {
  render(<ApplicationSettings client={client} bootstrap={bootstrap()} />);
  await user.click(screen.getByRole("button", { name: "发布新版本" }));
  expect(await screen.findByRole("status")).toHaveTextContent("只影响以后新建的作品");
});
```

- [ ] **Step 7: 运行前端 GREEN**

Run: `cd apps/desktop/frontend && npm test -- --run src/app src/features/home src/features/settings src/lib/wails-studio-client.test.ts`

Expected: PASS；控制台无 act warning，键盘焦点返回触发按钮。

- [ ] **Step 8: 提交**

```bash
git add apps/desktop/frontend/src/app apps/desktop/frontend/src/features/home apps/desktop/frontend/src/features/settings apps/desktop/frontend/src/lib
git commit -m "feat: add recent projects home and app settings"
```

### Task 7: 替换项目一级导航并拆分模型设置与任务运行

**Files:**
- Create: `apps/desktop/frontend/src/features/workspace/ProjectNavigation.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/ProjectNavigation.test.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/WorkbenchRouter.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/WorkbenchRouter.test.tsx`
- Create: `apps/desktop/frontend/src/features/agents/ProjectCreativeProfile.tsx`
- Create: `apps/desktop/frontend/src/features/agents/ProjectCreativeProfile.test.tsx`
- Create: `apps/desktop/frontend/src/features/agents/AgentRunPanel.tsx`
- Create: `apps/desktop/frontend/src/features/agents/AgentRunPanel.test.tsx`
- Modify: `apps/desktop/frontend/src/features/agents/AgentWorkbench.tsx`
- Modify: `apps/desktop/frontend/src/features/agents/AgentWorkbench.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/workspace-store.ts`
- Modify: `apps/desktop/frontend/src/features/workspace/workspace-store.test.ts`
- Modify: `apps/desktop/frontend/src/lib/studio-client.ts`

- [x] **Step 1: 写导航层级和权威投影的失败测试**

```tsx
it("groups archive pages below a secondary section", async () => {
  renderWorkspace({ workbench: writingProjection() });
  expect(screen.getByRole("button", { name: "正文与章节" })).toBeVisible();
  expect(screen.getByRole("button", { name: "作品档案" })).toBeVisible();
  expect(screen.queryByRole("button", { name: "人物与势力" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "作品档案" }));
  expect(screen.getByRole("button", { name: "人物与势力" })).toBeVisible();
});

it("routes from the authoritative workbench canvas instead of local mode", () => {
  render(<WorkbenchRouter projection={{ ...writingProjection(), canvas: "manuscript" }} />);
  expect(screen.getByRole("heading", { name: "正文与章节" })).toBeVisible();
  expect(screen.queryByRole("tab", { name: "构建模式" })).not.toBeInTheDocument();
});
```

- [x] **Step 2: 运行测试并确认 RED**

Run: `cd apps/desktop/frontend && npm test -- --run src/features/workspace src/features/agents`

Expected: FAIL，旧 `Workspace` 仍把智能创作、故事架构、人物、世界等全部放在同一级。

- [x] **Step 3: 实现固定一级分组和可恢复页面 ID**

```ts
export type ProjectSection =
  | "dashboard"
  | "manuscript"
  | "review"
  | "import"
  | "archive"
  | "delivery";

export type ProjectPage =
  | "dashboard"
  | "manuscript"
  | "changes"
  | "imports"
  | "proposal"
  | "story"
  | "characters"
  | "world"
  | "continuity"
  | "foreshadowing"
  | "versions"
  | "branches"
  | "export"
  | "maintenance";
```

一级只渲染“创作驾驶舱、正文与章节、审阅与候选、导入与续写、作品档案、版本与交付”。二级展开现有页面；尚无独立页面的 proposal/foreshadowing/export/maintenance 显示诚实的 `尚未在当前阶段提供`，不显示假内容。当前 page 存 workspace store，但 stage/canvas/counts 始终从 `client.projectWorkbench()` 刷新。

- [x] **Step 4: 将 `AgentWorkbench` 拆成三个边界**

`ApplicationSettings` 已承载连接和全局 profile；`ProjectCreativeProfile` 只选择已发布 profile version 和角色覆盖；`AgentRunPanel` 只显示当前项目运行预算、任务创建、运行进度与恢复。保留 `AgentWorkbench` 作为临时组合组件直到所有调用者迁移，然后删除其中连接/API Key 表单。

```ts
export type ProjectProfileSelectionInput = {
  profileId: string;
  version: number;
  roleOverrides: Partial<Record<AgentRole, ProjectRoleOverride>>;
};
```

项目选择页面必须显示 `默认方案 vN`、覆盖角色和“不会随全局默认自动变化”。切换 profile 不能修改已存在 run 的冻结 profile_id/model。

- [x] **Step 5: 保留现有页面可达性和读写保护**

为 story/world/assets/continuity/imports/changes/versions/branches 建立一一对应路由测试；只读项目隐藏写操作但保留查看；文件系统风险 banner 继续位于项目壳顶部，不因二级导航折叠消失。

```tsx
it.each([
  ["故事架构", "故事架构"], ["人物与势力", "人物与势力"],
  ["世界设定", "世界设定"], ["时间线与事实", "时间线与事实"],
  ["版本与恢复", "版本历史"],
])("opens %s without losing existing workbench", async (item, heading) => {
  renderWorkspace({ workbench: writingProjection() });
  await user.click(screen.getByRole("button", { name: "作品档案" }));
  await user.click(screen.getByRole("button", { name: item }));
  expect(screen.getByRole("heading", { name: heading })).toBeVisible();
  expect(screen.getByRole("button", { name: "正文与章节" })).toBeVisible();
});
```

- [x] **Step 6: 运行 GREEN**

Run: `cd apps/desktop/frontend && npm test -- --run src/features/workspace src/features/agents src/features/story src/features/world src/features/assets src/features/continuity src/features/revisions`

Expected: PASS；所有原有能力仍可到达，首页动作不出现在项目侧栏。

- [x] **Step 7: 提交**

```bash
git add apps/desktop/frontend/src/features/workspace apps/desktop/frontend/src/features/agents apps/desktop/frontend/src/lib/studio-client.ts
git commit -m "feat: add staged project navigation"
```

### Task 8: 实现阶段画布、AI 面板状态与导入后正文入口

**Files:**
- Create: `apps/desktop/frontend/src/features/workbench/StageCanvas.tsx`
- Create: `apps/desktop/frontend/src/features/workbench/StageCanvas.test.tsx`
- Create: `apps/desktop/frontend/src/features/workbench/AIPanel.tsx`
- Create: `apps/desktop/frontend/src/features/workbench/AIPanel.test.tsx`
- Create: `apps/desktop/frontend/src/features/imports/WholeBookArchiveCard.tsx`
- Create: `apps/desktop/frontend/src/features/imports/WholeBookArchiveCard.test.tsx`
- Modify: `apps/desktop/frontend/src/features/imports/ImportWorkbench.tsx`
- Modify: `apps/desktop/frontend/src/features/imports/ImportWorkbench.test.tsx`
- Modify: `apps/desktop/frontend/src/features/editor/ChapterEditor.tsx`
- Modify: `apps/desktop/frontend/src/features/editor/ChapterEditor.test.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Modify: `apps/desktop/frontend/src/features/workspace/workspace-store.ts`
- Modify: `apps/desktop/frontend/src/app/App.tsx`
- Modify: `apps/desktop/frontend/src/app/styles.css`
- Modify: `apps/desktop/api.go`
- Modify: `apps/desktop/api_test.go`
- Modify: `internal/application/appshell/service.go`
- Modify: `internal/application/appshell/service_test.go`

- [ ] **Step 1: 写面板状态和直接导入入口的失败测试**

```tsx
it("keeps the AI panel hidden during normal writing until Cmd+J", async () => {
  renderWorkspace({ workbench: writingProjection({ aiPanel: "hidden" }) });
  expect(screen.queryByRole("complementary", { name: "AI 创作" })).not.toBeInTheDocument();
  await user.keyboard("{Meta>}j{/Meta}");
  expect(screen.getByRole("complementary", { name: "AI 创作" })).toBeVisible();
});

it("opens imported manuscript immediately and shows whole-book archive action", async () => {
  render(<App client={client} />);
  await user.click(await screen.findByRole("button", { name: "导入小说并续写" }));
  await completeImportWizard(user, client);
  expect(await screen.findByRole("heading", { name: "正文与章节" })).toBeVisible();
  expect(screen.getByRole("button", { name: "AI 生成整书档案" })).toBeVisible();
});
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd apps/desktop/frontend && npm test -- --run src/features/workbench src/features/imports src/features/editor src/app/App.test.tsx`

Expected: FAIL，阶段画布、AI 面板和无项目导入生命周期尚不存在。

- [ ] **Step 3: 实现受控 AI 面板状态机**

```ts
export type AIPanelMode = "expanded" | "collapsed" | "hidden" | "selection_review";

type AIPanelDraft = {
  input: string;
  selection?: { chapterId: string; baseRevision: number; start: number; end: number; text: string };
};
```

默认值由 WorkbenchProjection 给出：idea 展开，proposal 保持作者选择，roadmap 可折叠，batch/writing 隐藏，review 以 selection_review 打开。workspace store 按 project ID 保存未发送输入；切换画布或打开应用设置不能清除。章节选区包含 base revision；正文变化后提交选区意见必须返回 `STALE_SELECTION` 并要求重新选择。

- [ ] **Step 4: 实现首页导入的应用生命周期**

新增 `CreateImportedProjectInput`：源文件、目标父目录、标题、类型和版权确认。appshell 的 `CreateImportedProject` 顺序固定为：校验源文件 → 创建导入型项目并 pin profile → `PrepareManuscriptImport` → 应用全部确定性章节 → 返回 `ProjectView + chapters + WorkbenchProjection`。任一步失败都保留可恢复项目与 import job，不删除作者唯一源文件；重复提交使用 source hash + project ID 幂等。

```go
type CreateImportedProjectResult struct {
	Project workspace.ProjectView `json:"project"`
	Chapters []workspace.ChapterView `json:"chapters"`
	Workbench workspace.WorkbenchProjection `json:"workbench"`
}
```

R2 不启动抽取任务。`StartWholeBookArchive` binding 返回安全的 `FEATURE_NOT_AVAILABLE`，同时 `ProjectWorkbench.ArchiveStatus` 为 `not_generated`；前端按钮显示“整书档案生成将在后续能力启用”，不能显示完成进度或写入档案。

- [ ] **Step 5: 实现导入后的正文卡与手工写作**

导入成功后 active page 强制为 manuscript，选中第一章；作者可直接编辑、保存、新建章节。正文顶部的 `WholeBookArchiveCard` 显示“尚未生成整本小说档案”和按钮；按钮禁用仅由结构化 capability 状态决定，不因缺模型配置伪装成已生成。

- [ ] **Step 6: 覆盖不丢状态、幂等和失败恢复**

```tsx
it("preserves unsaved chapter and AI input while toggling panel and settings", async () => {
  renderWritingWorkspace();
  await user.type(screen.getByRole("textbox", { name: "章节正文" }), "正文草稿");
  await user.keyboard("{Meta>}j{/Meta}");
  await user.type(screen.getByRole("textbox", { name: "给 AI 的说明" }), "保持第三人称");
  await user.click(screen.getByRole("button", { name: "应用设置" }));
  await user.click(screen.getByRole("button", { name: "关闭设置" }));
  expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveValue(expect.stringContaining("正文草稿"));
  expect(screen.getByRole("textbox", { name: "给 AI 的说明" })).toHaveValue("保持第三人称");
});

it("opens selection review with exact revision and range", async () => {
  renderWritingWorkspace({ chapter: chapter({ body: "潮声盖过脚步。", revision: 7 }) });
  selectEditorRange(0, 2);
  await user.click(screen.getByRole("button", { name: "告诉 AI 这里有问题" }));
  expect(screen.getByText("Revision 7 · 字符 0–2")).toBeVisible();
  expect(screen.getByText("潮声")).toBeVisible();
});

it("does not show chapter-level archive conclusions", async () => {
  render(<WholeBookArchiveCard status="not_generated" onStart={vi.fn()} />);
  expect(screen.getByRole("button", { name: "AI 生成整书档案" })).toBeVisible();
  expect(screen.queryByRole("button", { name: /确认第.*章档案/ })).not.toBeInTheDocument();
});
```

```go
func TestCreateImportedProjectRetryReusesPreparedJob(t *testing.T) {
	h := newImportedProjectHarness(t)
	first, err := h.create(); if err != nil { t.Fatal(err) }
	second, err := h.create(); if err != nil { t.Fatal(err) }
	if first.Project.ID != second.Project.ID || h.importJobCount() != 1 || h.chapterCount() != len(first.Chapters) { t.Fatalf("retry duplicated import: %#v", h) }
}

func TestCreateImportedProjectApplyFailureReturnsRecoverableProject(t *testing.T) {
	h := newImportedProjectHarness(t, withApplyError(errors.New("write failed")))
	result, err := h.create()
	if err == nil || result.Project.ID == "" || h.recoverableImportJobs() != 1 { t.Fatalf("result = %#v, err = %v", result, err) }
}

func TestStartWholeBookArchiveReturnsFeatureUnavailableWithoutCreatingExtractionTask(t *testing.T) {
	h := newImportedProjectHarness(t)
	err := h.api.StartWholeBookArchive()
	assertDesktopError(t, err, "FEATURE_NOT_AVAILABLE")
	if got := h.extractionTaskCount(); got != 0 { t.Fatalf("extraction tasks = %d", got) }
}
```

- [ ] **Step 7: 运行受影响 Go 与前端测试**

Run: `go test -race -shuffle=on ./internal/application/appshell ./internal/application/workspace ./apps/desktop -count=1`

Expected: PASS。

Run: `cd apps/desktop/frontend && npm test -- --run src/app src/features/workbench src/features/imports src/features/editor src/features/workspace`

Expected: PASS，编辑器输入、AI 输入和 import job 状态均不因重挂载丢失。

- [ ] **Step 8: 提交**

```bash
git add internal/application/appshell apps/desktop/api.go apps/desktop/api_test.go apps/desktop/frontend/src
git commit -m "feat: add stage canvas and imported writing entry"
```

### Task 9: 完成 R2 全量验证、双平台冒烟与文档证据

**Files:**
- Modify: `docs/testing/automated-test-baseline.md`
- Modify: `docs/testing/foundation-acceptance.md`
- Modify: `docs/handoff/START-HERE.md`
- Modify: `docs/roadmap/development-roadmap.md`
- Modify: `docs/plans/2026-08-11-r2-product-shell-interaction-foundation-implementation.md`

- [ ] **Step 1: 运行格式与静态检查**

Run: `gofmt -w internal/domain/appsettings internal/application/appsettings internal/application/appshell internal/application/workspace internal/application/ports internal/persistence/appsqlite internal/persistence/sqlite internal/platform/appdirs apps/desktop`

Expected: 命令成功，无未格式化 Go 文件。

Run: `git diff --check`

Expected: 无输出，exit 0。

- [ ] **Step 2: 运行完整 Go 门禁**

Run: `go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...`

Expected: 全部 PASS，无 race、timeout、真实网络或 OS 钥匙串依赖。

- [ ] **Step 3: 运行生产前端门禁**

Run: `cd apps/desktop/frontend && npm test -- --run && npm run typecheck && npm run build`

Expected: Vitest、TypeScript 和 Vite build 全部 PASS。

- [ ] **Step 4: 运行原型回归门禁**

Run: `cd prototype/web && npm test -- --run && npm run typecheck && npm run build`

Expected: 参考原型继续可运行；生产 UI 改动不能破坏原型。

- [ ] **Step 5: 执行 Windows/macOS 冒烟清单**

在两个原生平台分别记录：首次启动无 recent、存在 recent、recent 路径失效、应用菜单与文件菜单分离、设置中保存凭据、发布 profile v2 后旧项目仍 pin v1、新项目 pin v2、打开旧项目迁移、只读打开不迁移、项目侧栏分组、`Cmd/Ctrl+J`、设置抽屉与 AI 面板不丢正文、首页导入后立即进入正文、整书档案按钮不会伪造成功。

Expected: 每项记录平台、构建 SHA、结果和证据路径；任何一项失败都不把计划标为 `completed`。

- [ ] **Step 6: 更新权威状态与证据**

把自动命令结果加入 automated baseline，把新冒烟项加入 foundation acceptance。仅在双平台和全部自动门禁通过后：将本计划状态改为 `completed`，把路线图 R2 标记为完成，并在 START-HERE 将首个未完成阶段改为 R3。未通过时保持 `in_progress` 并记录真实阻断，不修改退出结论。

- [ ] **Step 7: 请求代码审阅并修复阻断问题**

使用 `superpowers:requesting-code-review` 检查需求符合性、设置数据库/凭据安全、旧项目迁移、Wails/React 边界、菜单平台差异和编辑状态保留。对审阅意见使用 `superpowers:receiving-code-review` 验证后修复，再完整重跑 Step 1–5。

- [ ] **Step 8: 提交阶段证据**

```bash
git add docs/testing/automated-test-baseline.md docs/testing/foundation-acceptance.md docs/handoff/START-HERE.md docs/roadmap/development-roadmap.md docs/plans/2026-08-11-r2-product-shell-interaction-foundation-implementation.md
git commit -m "docs: record R2 product shell acceptance"
```

## 回滚与恢复策略

- `settings.db` 迁移失败时保留原文件并停止应用设置写入，项目数据库不能因此被迁移或关闭；修复后重开应用重试。
- 新应用设置不可用时不得回退为把 API Key 写入项目数据库；允许显示安全错误并继续只读打开项目。
- 旧项目导入以 `legacy_imports` 幂等；应用设置已导入而项目 pin 未写入时，重试只补写 pin。
- 项目 schema v17 使用现有 staged migration、备份和恢复协议；只读打开永不执行 v17 迁移。
- 首页 recent 只是索引，不是项目数据源；删除或重建 recent 不影响 `.zjproject`。
- React 菜单/面板状态失败时，正文仍由现有 draft/revision 机制保护；不能用清空 workspace store 作为恢复动作。

## 计划自检

- 权威设计覆盖：首页/最近项目、系统菜单分离、应用级模型配置、项目 profile pin/覆盖、二级作品档案导航、阶段画布、按需 AI 面板、导入后即时正文、整书档案入口均有对应任务。
- 非目标受控：R2 不生成方向、方案、路线图、批次正文或整书档案；未实现能力使用结构化 capability 状态。
- 类型一致：`ProjectProfileSelection`、`WorkbenchProjection`、`AIPanelMode`、`ApplicationBootstrapView` 和 `CreateImportedProjectResult` 在 Go/TypeScript 两侧一一映射。
- 安全边界：API Key 只进入 `CredentialStore`；settings.db、项目 DB、日志和 DTO 不含密钥、正文副本或 Provider 原始错误。
- 平台边界：Wails 菜单使用原生角色，Windows/macOS 都有功能等价验证；应用数据目录由平台适配器解析。
