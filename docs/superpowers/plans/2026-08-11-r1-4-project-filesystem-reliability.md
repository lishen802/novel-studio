# R1.4 Project Filesystem Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Protect editable projects on sync/network-prone filesystems with lossless preflight probes, typed runtime failures, persistent risk UI, and Windows-compatible path budgets.

**Architecture:** `projectfs` owns temporary probes, portable path validation, risk hints, and OS failure classification. `workspace.Service` runs the preflight after acquiring the OS lease, retains process-local filesystem health, and degrades safely after typed write failures. Wails/React present only safe categories and retain editor text whenever persistence fails.

**Tech Stack:** Go 1.25, modernc SQLite, Wails v2, React, TypeScript, Vitest, Windows/macOS native CI.

---

## File map

- Create `internal/application/ports/project_filesystem.go`: stable health, action, failure and guard contracts.
- Create `internal/platform/projectfs/reliability.go`, `reliability_windows.go`, `reliability_unix.go`: probe, path budget and OS mapping.
- Modify `internal/platform/projectfs/layout.go`, `manager.go`, `recovery_copy.go`: validate all project-package paths before writes.
- Modify `internal/persistence/sqlite/integrity.go`: preserve SQLite BUSY, READONLY, IOERR and FULL categories.
- Modify `internal/application/workspace/service.go`, `view.go`: lifecycle preflight and project health.
- Create `internal/application/workspace/filesystem_health_test.go`: lifecycle and canonical-invariance tests.
- Modify desktop API/client types and add `ProjectFilesystemRiskBanner.tsx`: warning/degraded UI and editor recovery behavior.

## Task 1: Establish the shared contract

**Files:**
- Create: `internal/application/ports/project_filesystem.go`
- Create: `internal/platform/projectfs/reliability_test.go`

- [ ] **Step 1: Write failing port tests**

```go
func TestNormalProjectFilesystemHealthHasNoActions(t *testing.T) {
	got := ports.NormalProjectFilesystemHealth()
	if got.Level != ports.ProjectFilesystemNormal || len(got.Actions) != 0 {
		t.Fatalf("health = %#v", got)
	}
}

func TestProjectFilesystemFailureMatchesStableSentinel(t *testing.T) {
	err := &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("error = %v", err)
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/platform/projectfs -run 'Test(NormalProjectFilesystemHealth|ProjectFilesystemFailure)' -count=1`

Expected: FAIL because the port definitions do not exist.

- [ ] **Step 3: Add minimal contract**

```go
package ports

import "errors"

var ErrProjectFilesystemFailure = errors.New("project filesystem operation failed")

type ProjectFilesystemLevel string
const (
	ProjectFilesystemNormal ProjectFilesystemLevel = "normal"
	ProjectFilesystemWarning ProjectFilesystemLevel = "warning"
	ProjectFilesystemDegraded ProjectFilesystemLevel = "degraded"
)
type ProjectFilesystemCategory string
const (
	ProjectFilesystemSyncRisk ProjectFilesystemCategory = "sync_risk"
	ProjectFilesystemPathTooLong ProjectFilesystemCategory = "path_too_long"
	ProjectFilesystemSharingViolation ProjectFilesystemCategory = "sharing_violation"
	ProjectFilesystemBusy ProjectFilesystemCategory = "busy"
	ProjectFilesystemReadOnly ProjectFilesystemCategory = "read_only"
	ProjectFilesystemDiskFull ProjectFilesystemCategory = "disk_full"
	ProjectFilesystemIO ProjectFilesystemCategory = "io"
	ProjectFilesystemCheckpoint ProjectFilesystemCategory = "checkpoint"
	ProjectFilesystemAtomicReplace ProjectFilesystemCategory = "atomic_replace"
	ProjectFilesystemPreflight ProjectFilesystemCategory = "preflight"
)
type ProjectFilesystemAction string
const (
	ProjectFilesystemRetry ProjectFilesystemAction = "retry"
	ProjectFilesystemCopyText ProjectFilesystemAction = "copy_text"
	ProjectFilesystemOpenReadOnly ProjectFilesystemAction = "open_read_only"
	ProjectFilesystemCreateRecoveryCopy ProjectFilesystemAction = "create_recovery_copy"
)
type ProjectFilesystemHealth struct {
	Level ProjectFilesystemLevel `json:"level"`
	Category ProjectFilesystemCategory `json:"category,omitempty"`
	Actions []ProjectFilesystemAction `json:"actions,omitempty"`
}
func NormalProjectFilesystemHealth() ProjectFilesystemHealth {
	return ProjectFilesystemHealth{Level: ProjectFilesystemNormal}
}
type ProjectFilesystemFailure struct { Category ProjectFilesystemCategory }
func (e *ProjectFilesystemFailure) Error() string { return ErrProjectFilesystemFailure.Error() }
func (e *ProjectFilesystemFailure) Is(target error) bool { return target == ErrProjectFilesystemFailure }
type ProjectFilesystemGuard interface {
	PreflightEditable(ProjectPackage) (ProjectFilesystemHealth, error)
	ValidateCreatePath(string, string) error
	ValidateRecoveryCopyPath(ProjectPackage, string) error
	ClassifyRuntimeError(error) *ProjectFilesystemFailure
}
```

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/platform/projectfs -run 'Test(NormalProjectFilesystemHealth|ProjectFilesystemFailure)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/ports/project_filesystem.go internal/platform/projectfs/reliability_test.go
git commit -m "feat: define project filesystem reliability contract"
```

## Task 2: Enforce fixed Windows-compatible path budgets

**Files:**
- Modify: `internal/platform/projectfs/layout.go`, `manager.go`, `recovery_copy.go`
- Create: `internal/platform/projectfs/reliability.go`
- Test: `internal/platform/projectfs/layout_test.go`, `reliability_test.go`, `reliability_windows_test.go`

- [ ] **Step 1: Write failing validation tests**

```go
func TestValidateCreatePathRejectsLongWindowsCompatibleRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), strings.Repeat("segment-", 35))
	err := (Manager{}).ValidateCreatePath(parent, "作品")
	failure := (Manager{}).ClassifyRuntimeError(err)
	if failure == nil || failure.Category != ports.ProjectFilesystemPathTooLong {
		t.Fatalf("error = %v, failure = %#v", err, failure)
	}
}

func TestPortableProjectNameTrimsTrailingWindowsComponents(t *testing.T) {
	got, err := portableProjectName(" 长夜.  ")
	if err != nil || got != "长夜" { t.Fatalf("name = %q, error = %v", got, err) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/platform/projectfs -run 'Test(ValidateCreatePath|PortableProjectName)' -count=1`

Expected: FAIL because validation is missing or the long path is accepted.

- [ ] **Step 3: Implement path validation before every filesystem write**

```go
const windowsCompatiblePathLimit = 240
const longestProjectRelativePath = "recovery/.r1-4-probe-0123456789abcdef/replace-target-0123456789abcdef.sqlite"

func validateProjectPath(root string) error {
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil { return err }
	if len([]rune(filepath.Join(absolute, longestProjectRelativePath))) > windowsCompatiblePathLimit {
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPathTooLong}
	}
	return nil
}
func (Manager) ValidateCreatePath(parent, title string) error {
	name, err := portableProjectName(title)
	if err != nil { return err }
	return validateProjectPath(filepath.Join(parent, name+packageExtension))
}
```

Call `ValidateCreatePath` before `os.Mkdir` in `Manager.Create`; call `validateProjectPath` from `InspectLayout`; call `ValidateRecoveryCopyPath` after recovery-copy names are generated but before either target exists. Keep internal filenames fixed and short. Preserve the existing reserved-name suffix behavior.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/platform/projectfs -run 'Test(ValidateCreatePath|PortableProjectName|ManagerCreates|CreateRecoveryCopy)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/projectfs/layout.go internal/platform/projectfs/manager.go internal/platform/projectfs/recovery_copy.go internal/platform/projectfs/reliability.go internal/platform/projectfs/layout_test.go internal/platform/projectfs/reliability_test.go internal/platform/projectfs/reliability_windows_test.go
git commit -m "feat: enforce portable project path budgets"
```

## Task 3: Probe editable project filesystems without touching canonical paths

**Files:**
- Modify: `internal/platform/projectfs/reliability.go`, `manager.go`
- Test: `internal/platform/projectfs/reliability_test.go`

- [ ] **Step 1: Write failing lossless-probe tests**

```go
func TestPrepareEditableProbeLeavesCanonicalFilesUnchanged(t *testing.T) {
	pkg := createProjectPackage(t)
	before := collectFileHashes(t, pkg.Root)
	health, err := (Manager{}).PreflightEditable(pkg)
	if err != nil || health.Level == "" { t.Fatalf("health = %#v, error = %v", health, err) }
	if after := collectFileHashes(t, pkg.Root); !sameFileHashes(before, after) {
		t.Fatalf("probe changed files: %#v", after)
	}
}

func TestPrepareEditableProbeCleansUpAfterCheckpointFailure(t *testing.T) {
	manager := Manager{reliabilityHooks: &reliabilityHooks{checkpoint: func(*sql.DB) error { return errors.New("blocked") }}}
	_, err := manager.PreflightEditable(createProjectPackage(t))
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) { t.Fatalf("error = %v", err) }
	if probeDirectoryExists(t, manager) { t.Fatal("probe directory remains") }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/platform/projectfs -run TestPrepareEditableProbe -count=1`

Expected: FAIL because the new `PreflightEditable` method does not exist.

- [ ] **Step 3: Implement temporary-only stages**

```go
func (m Manager) PreflightEditable(pkg ports.ProjectPackage) (ports.ProjectFilesystemHealth, error) {
	if err := validateProjectPath(pkg.Root); err != nil { return ports.NormalProjectFilesystemHealth(), err }
	probeRoot, err := os.MkdirTemp(pkg.Root, ".r1-4-probe-")
	if err != nil { return ports.NormalProjectFilesystemHealth(), preflightFailure(err) }
	defer os.RemoveAll(probeRoot)
	if err := probeDirectory(probeRoot); err != nil { return ports.NormalProjectFilesystemHealth(), preflightFailure(err) }
	if err := probeSQLite(filepath.Join(probeRoot, "probe.sqlite")); err != nil { return ports.NormalProjectFilesystemHealth(), preflightFailure(err) }
	if err := probeAtomicReplacement(probeRoot); err != nil { return ports.NormalProjectFilesystemHealth(), preflightFailure(err) }
	return filesystemRiskHint(pkg.Root), nil
}
func preflightFailure(err error) error {
	return errors.Join(&ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPreflight}, err)
}
```

`probeSQLite` must create a temporary database, set WAL, commit one row, roll back a second row, verify exactly one row, execute `PRAGMA wal_checkpoint(TRUNCATE)`, then close. `probeAtomicReplacement` must write/sync two temporary files, rename source over target within the probe directory, reread the source body, and sync where supported. Test hooks may fail individual stages; production paths must never point at `project.db`, snapshots, branches, imports, lease, or recovery artifacts.

- [ ] **Step 4: Verify GREEN**

Run: `go test -race ./internal/platform/projectfs -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/projectfs/reliability.go internal/platform/projectfs/manager.go internal/platform/projectfs/reliability_test.go
git commit -m "feat: probe editable project filesystems safely"
```

## Task 4: Classify SQLite and platform write failures

**Files:**
- Modify: `internal/persistence/sqlite/integrity.go`
- Modify: `internal/platform/projectfs/reliability.go`, `reliability_windows.go`, `reliability_unix.go`
- Test: `internal/persistence/sqlite/integrity_test.go`, `internal/platform/projectfs/reliability_test.go`, `reliability_windows_test.go`

- [ ] **Step 1: Write failing mapping tests**

```go
func TestClassifyFilesystemErrorMapsSQLiteBusyAndFull(t *testing.T) {
	for _, test := range []struct { err error; want ports.ProjectFilesystemCategory }{
		{sqliteError(5, "database is locked"), ports.ProjectFilesystemBusy},
		{sqliteError(13, "database or disk is full"), ports.ProjectFilesystemDiskFull},
	} {
		got := projectfs.ClassifyFilesystemError(test.err)
		if got == nil || got.Category != test.want { t.Fatalf("%v => %#v", test.err, got) }
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/persistence/sqlite ./internal/platform/projectfs -run TestClassifyFilesystemErrorMapsSQLiteBusyAndFull -count=1`

Expected: FAIL because only generic database-write errors are emitted.

- [ ] **Step 3: Add primary-code and OS mapping**

```go
func ClassifyFilesystemError(err error) *ports.ProjectFilesystemFailure {
	if err == nil { return nil }
	if isSharingViolation(err) { return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation} }
	if isBusy(err) { return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemBusy} }
	if errors.Is(err, os.ErrPermission) || isReadOnly(err) { return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemReadOnly} }
	if isDiskFull(err) { return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemDiskFull} }
	if isIOError(err) { return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemIO} }
	return nil
}
```

In SQLite error classification map primary codes BUSY (5), READONLY (8), IOERR (10), and FULL (13) by joining this shared failure with the existing SQLite sentinel. Windows `isSharingViolation` must use `windows.ERROR_SHARING_VIOLATION` and `windows.ERROR_LOCK_VIOLATION`; Unix must not use a broad message match. Preserve existing corrupt/schema behavior.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/persistence/sqlite ./internal/platform/projectfs -run 'Test(Classify|Open.*Write|ProjectLease)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/persistence/sqlite/integrity.go internal/persistence/sqlite/integrity_test.go internal/platform/projectfs/reliability.go internal/platform/projectfs/reliability_windows.go internal/platform/projectfs/reliability_unix.go internal/platform/projectfs/reliability_test.go internal/platform/projectfs/reliability_windows_test.go
git commit -m "feat: classify project filesystem failures"
```

## Task 5: Wire preflight and health into workspace lifecycle

**Files:**
- Modify: `internal/application/workspace/service.go`, `view.go`
- Create: `internal/application/workspace/filesystem_health_test.go`
- Modify: `internal/application/ports/project_package.go`, `internal/application/workspace/recovery_copy.go`

- [ ] **Step 1: Write failing workspace invariance tests**

```go
func TestOpenProjectPreflightFailureKeepsCurrentWorkspace(t *testing.T) {
	service, current := serviceWithActiveProject(t)
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{prepareErr: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPreflight}})
	_, err := service.OpenProject(context.Background(), createPackage(t).Root)
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) { t.Fatalf("open error = %v", err) }
	if got := activePath(t, service); got != current.Path { t.Fatalf("active = %q", got) }
}

func TestCommitFilesystemFailureDegradesWithoutChangingRevision(t *testing.T) {
	service, chapter := editableServiceWithChapter(t)
	setCommitError(t, service, sharingViolationError())
	_, err := service.CommitChapter(context.Background(), chapter.ID, "未保存文本", chapter.Revision)
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) { t.Fatalf("error = %v", err) }
	if got := activeHealth(t, service); got.Level != ports.ProjectFilesystemDegraded { t.Fatalf("health = %#v", got) }
	if got := storedRevision(t, service, chapter.ID); got != chapter.Revision { t.Fatalf("revision = %d", got) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/workspace -run 'Test(OpenProjectPreflightFailure|CommitFilesystemFailure)' -count=1`

Expected: FAIL because no filesystem guard or health exists.

- [ ] **Step 3: Implement lifecycle sequencing and health transitions**

Add `filesystem ports.ProjectFilesystemGuard` to `Service`, `filesystemHealth ports.ProjectFilesystemHealth` to `activeWorkspace`, and `ConfigureProjectFilesystemGuard`. In editable create/open, acquire OS lease, call `filesystem.PreflightEditable`, then call the existing compatibility preparation, migration recovery, branch recovery and writable store open. If preflight fails, release only the new lease and do not replace the active workspace. `OpenProjectReadOnly` must not call it.

Add `FilesystemHealth ports.ProjectFilesystemHealth\x60json:"filesystemHealth"\x60` to `ProjectView`. Route classified failed store/archive writes through a private method that changes only the active in-memory health to:

```go
ports.ProjectFilesystemHealth{
	Level: ports.ProjectFilesystemDegraded,
	Category: failure.Category,
	Actions: []ports.ProjectFilesystemAction{
		ports.ProjectFilesystemRetry,
		ports.ProjectFilesystemCopyText,
		ports.ProjectFilesystemOpenReadOnly,
		ports.ProjectFilesystemCreateRecoveryCopy,
	},
}
```

Do not write this state to SQLite. On a successful explicit health recheck, reset to warning when a risk hint remains or normal otherwise.

- [ ] **Step 4: Verify GREEN**

Run: `go test -race -shuffle=on ./internal/application/workspace -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/ports/project_package.go internal/application/workspace/service.go internal/application/workspace/view.go internal/application/workspace/recovery_copy.go internal/application/workspace/filesystem_health_test.go
git commit -m "feat: guard editable workspace filesystem health"
```

## Task 6: Expose safe desktop data and keep editor text recoverable

**Files:**
- Modify: `apps/desktop/app.go`, `api.go`, `errors.go`, `api_test.go`
- Modify: `apps/desktop/frontend/src/lib/studio-client.ts`, `wails-studio-client.ts`, `fake-studio-client.ts`
- Create: `apps/desktop/frontend/src/features/workspace/ProjectFilesystemRiskBanner.tsx`, `ProjectFilesystemRiskBanner.test.tsx`
- Modify: `Workspace.tsx`, `Workspace.test.tsx`, `ChapterEditor.tsx`, `ChapterEditor.test.tsx`, `styles.css`

- [ ] **Step 1: Write failing API and React tests**

```go
func TestDesktopAPIMapsSharingViolationToSafeCode(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{commitErr: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}}, &fakeDirectoryPicker{})
	api.startup(context.Background())
	_, err := api.CommitChapter("chapter-1", "正文", 1)
	var got *desktopError
	if !errors.As(err, &got) || got.Code != errorProjectSharingViolation { t.Fatalf("error = %#v", err) }
}
```

```tsx
it("keeps editable workspace enabled for sync risk", () => {
	render(<Workspace {...props} workspace={readyWorkspace({ filesystemHealth: { level: "warning", category: "sync_risk", actions: [] } })} />);
	expect(screen.getByText("项目文件系统存在风险")).toBeInTheDocument();
	expect(screen.getByRole("textbox", { name: "章节正文" })).not.toHaveAttribute("readonly");
});
```

- [ ] **Step 2: Verify RED**

Run: `go test ./apps/desktop -run TestDesktopAPIMapsSharingViolationToSafeCode -count=1 && cd apps/desktop/frontend && npm test -- --run src/features/workspace/ProjectFilesystemRiskBanner.test.tsx src/features/editor/ChapterEditor.test.tsx`

Expected: FAIL because no safe code or risk component exists.

- [ ] **Step 3: Implement presentation and recovery behavior**

Map shared categories ahead of generic write errors to `PROJECT_FILESYSTEM_PREFLIGHT_FAILED`, `PROJECT_PATH_TOO_LONG`, `PROJECT_SHARING_VIOLATION`, `PROJECT_CHECKPOINT_FAILED`, and `PROJECT_ATOMIC_REPLACE_FAILED`; never expose OS paths, SQL, or manuscript text.

Implement:

```tsx
export function ProjectFilesystemRiskBanner({ health }: { health: ProjectFilesystemHealth }) {
	if (health.level === "normal") return null;
	const degraded = health.level === "degraded";
	return <aside className={`project-filesystem-risk project-filesystem-risk--${health.level}`} role="status">
		<strong>{degraded ? "项目写入遇到风险" : "项目文件系统存在风险"}</strong>
		<span>{degraded ? "未保存文本仍保留在当前编辑器中。请重试或创建恢复副本。" : "此位置可能干预 SQLite 写入；请保持同步客户端空闲并定期备份。"}</span>
	</aside>;
}
```

Render it under the existing read-only notice. When `commitChapter` fails, `ChapterEditor` must retain `bodyRef` and `dirtyRef`, make one `saveDraft` attempt with the same body/revision, and show either “恢复草稿已安全保存” or “无法保存恢复草稿；请复制文本”。 The retry action repeats commit; copy uses `navigator.clipboard.writeText(bodyRef.current)`; recovery-copy action is routed through the existing workspace-store flow. A warning never disables the editable workspace; read-only still does.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./apps/desktop -count=1 && cd apps/desktop/frontend && npm test -- --run && npm run typecheck && npm run build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop apps/desktop/frontend/src/lib apps/desktop/frontend/src/features/workspace apps/desktop/frontend/src/features/editor/ChapterEditor.tsx apps/desktop/frontend/src/features/editor/ChapterEditor.test.tsx apps/desktop/frontend/src/app/styles.css
git commit -m "feat: present project filesystem risks safely"
```

## Task 7: Record native acceptance and close only with evidence

**Files:**
- Modify: `docs/testing/automated-test-baseline.md`, `docs/testing/foundation-acceptance.md`, `docs/operations/project-recovery.md`, `docs/roadmap/development-roadmap.md`, `docs/handoff/START-HERE.md`

- [ ] **Step 1: Add F-15 through F-18 only after their automated tests exist**

F-15 sync-client or network-share locking while editing; F-16 failed probe leaves a disposable package unchanged; F-17 full/readonly/sharing failure preserves editor text and gives recovery actions; F-18 Windows long root and recovery-copy path. State that successful preflight is not a promise of future safety.

- [ ] **Step 2: Run complete regression evidence**

Run: `go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...`

Run: `cd apps/desktop/frontend && npm test -- --run && npm run typecheck && npm run build`

Run: `cd prototype/web && npm test -- --run && npm run typecheck && npm run build`

Expected: all commands PASS.

- [ ] **Step 3: Run native CI and manual acceptance**

Run focused projectfs/workspace reliability tests in the Windows Foundation job, then verify Windows and macOS production builds. Execute F-15–F-18 only on disposable copies and record commit, OS version, date, result, and non-sensitive evidence path.

- [ ] **Step 4: Commit documentation and close the stage only after evidence succeeds**

```bash
git add docs/testing docs/operations/project-recovery.md docs/roadmap/development-roadmap.md docs/handoff/START-HERE.md
git commit -m "docs: close R1.4 filesystem reliability"
```

- [ ] **Step 5: Final completion checks**

Run: `git diff --check && git status --short && go test -count=1 -race -shuffle=on -timeout=5m ./apps/desktop ./internal/...`

Expected: empty status, no sensitive artifacts, and PASS. Merge the isolated feature branch into `main` only after the recorded Windows/macOS evidence is successful.
