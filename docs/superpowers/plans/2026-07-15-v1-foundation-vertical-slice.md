# Novel Studio V1 Foundation Vertical Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建成正式产品的第一个可运行纵向切片：用户可在 Windows/macOS 创建 `.zjproject` 项目、新建章节、编辑正文、自动保存，并在异常退出后恢复未提交草稿。

**Architecture:** 使用根 Go module 与 `apps/desktop` 内的标准 Wails v2 项目。领域、应用、端口、SQLite 和平台实现位于 `internal/`；桌面入口只组装依赖，Wails API 只转发应用用例；React 通过一个可替换的 `StudioClient` 调用生成绑定。每个项目是独立目录包和 SQLite 数据库，保存采用乐观并发，短周期草稿与正式章节提交分离。V1 的授权实现为默认放行，但创建和编辑仍经过能力门禁。

**Tech Stack:** Go 1.25+、Wails v2.13.0、React 19、TypeScript 5.7、Vite、SQLite（`modernc.org/sqlite`）、Go `testing`、Vitest、Testing Library、GitHub Actions。

---

## 实施约束

- 在仓库根目录执行 Go 命令；在 `apps/desktop/frontend` 执行 npm 命令；在 `apps/desktop` 执行 Wails 命令。
- 不修改 `prototype/web`，仅对照其布局、主题和交互。
- 不提交 `.devcrew/`、`docs/devcrew/`、API key、稿件、数据库、构建产物或签名材料。
- 每个任务必须按“失败测试 → 最小实现 → 全量回归 → 小提交”执行。
- 下文中的提交命令只暂存任务列出的文件，禁止使用 `git add .`。

## Task 1：冻结正式目录、工具链和 Wails 工程

**Files:**

- Create: `docs/architecture/0003-wails-project-layout.md`
- Create: `go.mod`
- Create: `apps/desktop/main.go`
- Create: `apps/desktop/app.go`
- Create: `apps/desktop/wails.json`
- Create: `apps/desktop/build/README.md`
- Modify: `.gitignore`
- Modify: `apps/desktop/README.md`

- [x] **Step 1: 记录当前工具链并验证前置条件**

Run:

```bash
go version
node --version
npm --version
"$(go env GOPATH)/bin/wails" version
```

Expected: Go 自动选择的有效工具链 ≥ 1.25、Node ≥ 20；首次执行时 `wails` 可以不存在，这正是安装步骤的前置证据。

- [x] **Step 2: 安装固定的 Wails v2 CLI 并运行环境诊断**

Run:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
"$(go env GOPATH)/bin/wails" doctor
```

Expected: CLI 报告 v2.13.0；macOS 本机诊断通过。Windows 构建环境由 CI 在 Task 10 验证。

- [x] **Step 3: 写 ADR，固定 Wails 工程位于 `apps/desktop`**

`docs/architecture/0003-wails-project-layout.md` 必须写明：

1. Wails v2 的资源嵌入和 CLI 工作目录要求入口、`wails.json`、`build` 与 `frontend` 形成一个工程单元。
2. 因此正式入口采用 `apps/desktop/main.go`，替代早期设计中仅为建议的 `cmd/novel-studio`。
3. 业务代码仍全部位于 `internal/`，该调整不改变模块化单体边界。
4. 若未来恢复 `cmd/`，必须通过新 ADR 解决嵌入资源和跨平台构建，不使用符号链接。

- [x] **Step 4: 初始化根 Go module 并固定依赖**

Run:

```bash
go mod init github.com/lishen802/novel-studio
go get github.com/wailsapp/wails/v2@v2.13.0
```

Expected: 根目录生成 `go.mod`/`go.sum`，不存在第二个嵌套 Go module。此处暂不执行 `go mod tidy`，因为正式前端构建产物尚未生成，而 `main.go` 的 `go:embed` 会在 Task 2 前故意保持不可编译状态。

- [x] **Step 5: 建立最小 Wails 入口和配置**

`apps/desktop/main.go`：

```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := newApp()
	err := wails.Run(&options.App{
		Title:  "织境 Story Studio",
		Width:  1440,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`apps/desktop/app.go` 在本任务只负责生命周期，不添加业务方法：

```go
package main

import "context"

type App struct {
	ctx context.Context
}

func newApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
```

`apps/desktop/wails.json`：

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "NovelStudio",
  "outputfilename": "NovelStudio",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "Novel Studio",
    "email": ""
  }
}
```

- [x] **Step 6: 忽略本地产物并完成冒烟检查**

`.gitignore` 至少新增：

```gitignore
apps/desktop/build/bin/
apps/desktop/frontend/dist/
apps/desktop/frontend/node_modules/
apps/desktop/frontend/src/bindings/
*.db
*.db-shm
*.db-wal
*.zjproject/
```

Run:

```bash
go mod verify
git status --short
```

Expected: module checksum 验证通过；本任务不声称桌面入口已经可编译，第一次完整编译必须在 Task 2 生成真实 `frontend/dist` 后完成。

- [x] **Step 7: 提交本任务**

```bash
git add .gitignore go.mod go.sum apps/desktop docs/architecture/0003-wails-project-layout.md
git commit -m "build: establish Wails v2 desktop foundation"
```

## Task 2：搭建正式 React 壳与可替换客户端边界

**Files:**

- Create: `apps/desktop/frontend/package.json`
- Create: `apps/desktop/frontend/index.html`
- Create: `apps/desktop/frontend/tsconfig.json`
- Create: `apps/desktop/frontend/vite.config.ts`
- Create: `apps/desktop/frontend/src/main.tsx`
- Create: `apps/desktop/frontend/src/app/App.tsx`
- Create: `apps/desktop/frontend/src/app/App.test.tsx`
- Create: `apps/desktop/frontend/src/app/styles.css`
- Create: `apps/desktop/frontend/src/test/setup.ts`
- Create: `apps/desktop/frontend/src/lib/studio-client.ts`

- [x] **Step 1: 创建前端包并安装固定依赖**

`package.json` 使用已在原型验证的 React/测试依赖版本，脚本至少包含：

```json
{
  "name": "novel-studio-desktop",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "typecheck": "tsc -b --pretty false",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "lucide-react": "^0.468.0",
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "zod": "^3.24.1",
    "zustand": "^5.0.3"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.1.0",
    "@testing-library/user-event": "^14.5.2",
    "@types/react": "^19.0.3",
    "@types/react-dom": "^19.0.2",
    "@vitejs/plugin-react": "^6.0.3",
    "jsdom": "^25.0.1",
    "typescript": "~5.7.2",
    "vite": "^8.1.4",
    "vitest": "^4.1.10"
  }
}
```

Run: `cd apps/desktop/frontend && npm install`

Expected: 生成 `package-lock.json`，安装成功。

- [x] **Step 2: 先写应用壳失败测试**

`src/app/App.test.tsx`：

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("shows the product identity and project entry actions", () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: "织境 Story Studio" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "创建项目" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "打开项目" })).toBeEnabled();
  });
});
```

Run: `npm test -- App.test.tsx`

Expected: FAIL because `App` does not exist.

- [x] **Step 3: 实现最小正式壳**

`src/app/App.tsx`：

```tsx
export function App() {
  return (
    <main className="app-shell">
      <section className="welcome-card" aria-labelledby="product-title">
        <p className="eyebrow">长篇小说智能创作工作台</p>
        <h1 id="product-title">织境 Story Studio</h1>
        <p>从故事工程开始，让智能体参与规划、写作与审阅，同时由作者掌握最终决定。</p>
        <div className="welcome-actions">
          <button type="button">创建项目</button>
          <button type="button" className="secondary">打开项目</button>
        </div>
      </section>
    </main>
  );
}
```

建立 `main.tsx`、测试 setup、Vite/TS 配置和从原型提炼的基础设计 token；不要复制原型状态管理或假数据。

- [x] **Step 4: 定义 UI 唯一后端边界**

`src/lib/studio-client.ts`：

```ts
export type ProjectView = {
  id: string;
  title: string;
  genre: string;
  summary: string;
  path: string;
};

export type ChapterView = {
  id: string;
  projectId: string;
  title: string;
  position: number;
  body: string;
  revision: number;
};

export type CreateProjectInput = {
  parentDirectory: string;
  title: string;
  genre: string;
  summary: string;
};

export interface StudioClient {
  chooseProjectParent(): Promise<string | null>;
  chooseProjectToOpen(): Promise<string | null>;
  createProject(input: CreateProjectInput): Promise<ProjectView>;
  openProject(path: string): Promise<ProjectView>;
  listChapters(): Promise<ChapterView[]>;
  createChapter(title: string): Promise<ChapterView>;
  saveDraft(chapterId: string, body: string, baseRevision: number): Promise<void>;
  commitChapter(chapterId: string, body: string, baseRevision: number): Promise<ChapterView>;
}
```

React feature code只能依赖该接口，不能直接导入生成的 Wails bindings。

- [x] **Step 5: 验证和提交**

Run:

```bash
cd apps/desktop/frontend
npm test
npm run typecheck
npm run build
cd ../../..
go test ./apps/desktop
go vet ./apps/desktop
cd apps/desktop
"$(go env GOPATH)/bin/wails" build -clean -m
```

Expected: 全部 PASS，生成本地应用包；产物不进入 Git。

```bash
git add apps/desktop/frontend apps/desktop/main.go apps/desktop/wails.json
git commit -m "feat: add production desktop shell"
```

## Task 3：以测试驱动建立项目和章节领域模型

**Files:**

- Create: `internal/domain/project/project.go`
- Create: `internal/domain/project/project_test.go`
- Create: `internal/domain/project/chapter.go`
- Create: `internal/domain/project/chapter_test.go`

- [x] **Step 1: 写领域不变量失败测试**

测试必须覆盖：

```go
func TestNewProjectRejectsBlankTitle(t *testing.T)
func TestNewProjectNormalizesTextAndKeepsStableIdentity(t *testing.T)
func TestNewChapterStartsAtRevisionOne(t *testing.T)
func TestChapterCommitRequiresExpectedRevision(t *testing.T)
func TestNewChapterRejectsBlankTitle(t *testing.T)
```

关键并发测试：

```go
func TestChapterCommitRequiresExpectedRevision(t *testing.T) {
	chapter, err := NewChapter("chapter-1", "project-1", "第一章", 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := chapter.Commit("第一版正文", 1, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision = %d, want 2", updated.Revision)
	}

	_, err = updated.Commit("过期窗口覆盖", 1, time.Unix(102, 0))
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}
}
```

Run: `go test ./internal/domain/project -run Test -v`

Expected: FAIL because constructors and errors do not exist.

- [x] **Step 2: 实现纯领域对象**

需要导出的最小 API：

```go
var (
	ErrBlankTitle       = errors.New("title must not be blank")
	ErrRevisionConflict = errors.New("chapter revision conflict")
)

type Project struct {
	ID        string
	Title     string
	Genre     string
	Summary   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Chapter struct {
	ID        string
	ProjectID string
	Title     string
	Position  int
	Body      string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProject(id, title, genre, summary string, now time.Time) (Project, error)
func NewChapter(id, projectID, title string, position int, now time.Time) (Chapter, error)
func (c Chapter) Commit(body string, expectedRevision int64, now time.Time) (Chapter, error)
```

规则：对标题、类型和摘要执行 `strings.TrimSpace`；ID 不能为空；position 从 1 开始；新章节 revision 为 1；`Commit` 只改变正文、更新时间和 revision，不执行 I/O。

- [x] **Step 3: 回归和提交**

```bash
go test ./internal/domain/project -race
go test ./apps/desktop ./internal/...
git add internal/domain/project
git commit -m "feat: define project and chapter domain"
```

## Task 4：建立项目包、授权门禁和应用端口

**Files:**

- Create: `internal/application/ports/clock.go`
- Create: `internal/application/ports/id_generator.go`
- Create: `internal/application/ports/project_store.go`
- Create: `internal/application/license/service.go`
- Create: `internal/application/license/noop.go`
- Create: `internal/application/license/noop_test.go`
- Create: `internal/platform/projectfs/layout.go`
- Create: `internal/platform/projectfs/layout_test.go`

- [ ] **Step 1: 写项目包和门禁失败测试**

项目包测试使用 `t.TempDir()`，验证：

- 标题 `My / Novel` 生成安全目录名且后缀为 `.zjproject`。
- 建立 `project.db`、`attachments/`、`exports/`、`snapshots/` 的路径约定。
- 已存在目录不会被静默覆盖。
- 路径生成不依赖 `/` 或 `\\` 字符串拼接。

授权测试：

```go
func TestNoopServiceAllowsEveryV1Capability(t *testing.T) {
	service := license.NoopService{}
	for _, capability := range []license.Capability{
		license.CreateProject,
		license.EditManuscript,
		license.ImportManuscript,
		license.RunAI,
	} {
		if err := service.Require(context.Background(), capability); err != nil {
			t.Fatalf("Require(%q) = %v", capability, err)
		}
	}
}
```

- [ ] **Step 2: 定义端口，避免应用层依赖 SQLite/Wails**

`project_store.go`：

```go
package ports

import (
	"context"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type ProjectStore interface {
	Project(ctx context.Context) (projectdomain.Project, error)
	CreateChapter(ctx context.Context, chapter projectdomain.Chapter) error
	ListChapters(ctx context.Context) ([]projectdomain.Chapter, error)
	Chapter(ctx context.Context, id string) (projectdomain.Chapter, error)
	SaveDraft(ctx context.Context, chapterID, body string, baseRevision int64) error
	Draft(ctx context.Context, chapterID string) (body string, baseRevision int64, found bool, err error)
	CommitChapter(ctx context.Context, chapter projectdomain.Chapter, previous projectdomain.Chapter) error
	Close() error
}

type ProjectStoreFactory interface {
	Create(ctx context.Context, packagePath string, project projectdomain.Project) (ProjectStore, error)
	Open(ctx context.Context, packagePath string) (ProjectStore, error)
}
```

同时定义可注入的 `Clock`、`IDGenerator`，测试中禁止依赖真实时间和随机 ID。

- [ ] **Step 3: 实现 `projectfs.Layout`**

公开 API：

```go
type Layout struct {
	Root        string
	Database    string
	Attachments string
	Exports     string
	Snapshots   string
}

func NewLayout(parentDirectory, projectTitle string) (Layout, error)
func (l Layout) Create() error
func OpenLayout(packagePath string) (Layout, error)
```

使用 `filepath.Join` 和 `os.Mkdir`/`os.MkdirAll`；`Create` 对根目录使用不覆盖语义。不要在项目包里保存许可证或模型密钥。

- [ ] **Step 4: 验证和提交**

```bash
go test ./internal/application/license ./internal/platform/projectfs -race
go test ./apps/desktop ./internal/...
git add internal/application/ports internal/application/license internal/platform/projectfs
git commit -m "feat: add project package and capability boundaries"
```

## Task 5：建立 SQLite schema、迁移和事务仓储

**Files:**

- Create: `internal/persistence/sqlite/migrations/00001_foundation.sql`
- Create: `internal/persistence/sqlite/database.go`
- Create: `internal/persistence/sqlite/database_test.go`
- Create: `internal/persistence/sqlite/project_store.go`
- Create: `internal/persistence/sqlite/project_store_test.go`

- [ ] **Step 0: 添加本任务实际使用的持久化依赖**

Run:

```bash
go get modernc.org/sqlite
go get github.com/pressly/goose/v3
```

Expected: `go.mod`/`go.sum` 固定当前选定版本；后续测试直接编译到实际 SQLite 和 migration 库。

- [ ] **Step 1: 写空库迁移失败测试**

测试在 `t.TempDir()` 创建数据库，执行两次 `OpenDatabase`，断言：

- 第二次迁移是幂等的。
- `PRAGMA foreign_keys` 为 1。
- `PRAGMA journal_mode` 为 `wal`。
- 必需表和唯一索引存在。
- 损坏数据库返回分类错误，不重新创建并覆盖原文件。

- [ ] **Step 2: 写第一版完整 migration**

`00001_foundation.sql`：

```sql
-- +goose Up
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    genre TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE chapters (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 1),
    body TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, position)
);

CREATE TABLE chapter_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(chapter_id, revision)
);

CREATE TABLE drafts (
    chapter_id TEXT PRIMARY KEY REFERENCES chapters(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    base_revision INTEGER NOT NULL CHECK (base_revision >= 1),
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_chapters_project_position
    ON chapters(project_id, position);

-- +goose Down
DROP TABLE drafts;
DROP TABLE chapter_revisions;
DROP TABLE chapters;
DROP TABLE projects;
```

- [ ] **Step 3: 实现数据库打开策略**

`OpenDatabase(path string)` 必须：

1. 使用 modernc SQLite driver。
2. 设置 `foreign_keys=ON`、`journal_mode=WAL`、`busy_timeout=5000`。
3. 限制每个项目数据库 `SetMaxOpenConns(1)`，首版以确定性写入优先。
4. 使用 `go:embed migrations/*.sql` 与 goose 迁移。
5. 迁移失败时关闭连接并原样返回错误。

- [ ] **Step 4: 先写仓储失败测试**

至少包含：

```go
func TestStoreCreatesAndListsChaptersInPositionOrder(t *testing.T)
func TestSaveDraftUpsertsWithoutChangingCommittedChapter(t *testing.T)
func TestCommitChapterArchivesPreviousRevisionAndClearsDraft(t *testing.T)
func TestCommitChapterRollsBackAllWritesOnRevisionConflict(t *testing.T)
func TestOpenRejectsPackageWithoutExactlyOneProject(t *testing.T)
```

事务断言：提交章节时必须在同一个 transaction 中完成“校验旧 revision → 写入 revision 历史 → 条件更新章节 → 删除 draft”。条件更新使用：

```sql
UPDATE chapters
SET title = ?, body = ?, revision = ?, updated_at = ?
WHERE id = ? AND revision = ?;
```

受影响行数不是 1 时回滚并返回 `project.ErrRevisionConflict`。

- [ ] **Step 5: 实现 `ports.ProjectStore` 和 factory**

所有时间按 UTC RFC3339Nano 存储；查询后恢复为 `time.Time`。SQL 只存在于 persistence 包。错误使用 `%w` 保留分类，不把 SQL 文本或正文写入日志。

- [ ] **Step 6: 验证和提交**

```bash
go test ./internal/persistence/sqlite -race -count=1
go test ./apps/desktop ./internal/... -race
git add internal/persistence/sqlite go.mod go.sum
git commit -m "feat: persist projects chapters and recovery drafts"
```

## Task 6：实现状态化工作区应用服务

**Files:**

- Create: `internal/application/workspace/service.go`
- Create: `internal/application/workspace/service_test.go`
- Create: `internal/application/workspace/view.go`
- Create: `internal/application/workspace/errors.go`

- [ ] **Step 1: 用 fake ports 写失败测试**

测试以下用户用例：

```go
func TestCreateProjectChecksLicenseBeforeCreatingPackage(t *testing.T)
func TestCreateProjectLeavesNoActiveWorkspaceWhenFactoryFails(t *testing.T)
func TestOpenProjectClosesPreviousStoreAfterNewStoreIsReady(t *testing.T)
func TestCreateChapterAssignsNextPosition(t *testing.T)
func TestSaveDraftDoesNotAdvanceRevision(t *testing.T)
func TestCommitChapterReturnsConflictForStaleEditor(t *testing.T)
func TestOpenChapterReturnsRecoverableDraft(t *testing.T)
```

每个测试使用固定 clock 和顺序 ID generator，断言能力门禁先于文件/数据库写入发生。

- [ ] **Step 2: 定义面向 UI 的稳定 DTO**

`view.go`：

```go
type ProjectView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Genre   string `json:"genre"`
	Summary string `json:"summary"`
	Path    string `json:"path"`
}

type ChapterView struct {
	ID               string `json:"id"`
	ProjectID        string `json:"projectId"`
	Title            string `json:"title"`
	Position         int    `json:"position"`
	Body             string `json:"body"`
	Revision         int64  `json:"revision"`
	RecoveryBody     string `json:"recoveryBody,omitempty"`
	HasRecoveryDraft bool   `json:"hasRecoveryDraft"`
}
```

DTO 不暴露 `sql.Null*`、数据库路径细节或供应商类型。

- [ ] **Step 3: 实现服务和并发保护**

`Service` 持有 factory、license、clock、IDs、`sync.RWMutex` 和当前 active store/path。公开方法：

```go
func (s *Service) CreateProject(ctx context.Context, parent, title, genre, summary string) (ProjectView, error)
func (s *Service) OpenProject(ctx context.Context, packagePath string) (ProjectView, error)
func (s *Service) ListChapters(ctx context.Context) ([]ChapterView, error)
func (s *Service) CreateChapter(ctx context.Context, title string) (ChapterView, error)
func (s *Service) OpenChapter(ctx context.Context, chapterID string) (ChapterView, error)
func (s *Service) SaveDraft(ctx context.Context, chapterID, body string, baseRevision int64) error
func (s *Service) CommitChapter(ctx context.Context, chapterID, body string, baseRevision int64) (ChapterView, error)
func (s *Service) Close() error
```

创建和编辑分别调用 `license.CreateProject`、`license.EditManuscript`。切换项目时先成功打开新 store，再原子替换并关闭旧 store；失败时旧项目保持可用。

- [ ] **Step 4: 验证和提交**

```bash
go test ./internal/application/workspace -race
go test ./apps/desktop ./internal/... -race
git add internal/application/workspace
git commit -m "feat: add workspace application use cases"
```

## Task 7：实现薄 Wails API 和依赖组装

**Files:**

- Create: `internal/platform/ids/uuid.go`
- Create: `internal/platform/timeutil/system_clock.go`
- Create: `apps/desktop/api.go`
- Create: `apps/desktop/api_test.go`
- Modify: `apps/desktop/app.go`
- Modify: `apps/desktop/main.go`

- [ ] **Step 0: 添加 UUID 实现依赖**

Run:

```bash
go get github.com/google/uuid
```

Expected: UUID 依赖直到平台 ID adapter 实际实现时才进入根 Go module。

- [ ] **Step 1: 写 API 转发失败测试**

使用 mock workspace，验证 API：

- 不修改输入内容、不生成 ID、不执行业务校验。
- 取消目录选择返回空值而不是错误。
- 应用服务错误原样进入统一的安全错误映射。
- shutdown 总是调用 workspace `Close`。

- [ ] **Step 2: 定义薄 API 方法**

```go
type DesktopAPI struct {
	ctx       context.Context
	workspace WorkspaceUseCases
}

func (a *DesktopAPI) ChooseProjectParent() (string, error)
func (a *DesktopAPI) ChooseProjectToOpen() (string, error)
func (a *DesktopAPI) CreateProject(input CreateProjectInput) (workspace.ProjectView, error)
func (a *DesktopAPI) OpenProject(path string) (workspace.ProjectView, error)
func (a *DesktopAPI) ListChapters() ([]workspace.ChapterView, error)
func (a *DesktopAPI) CreateChapter(title string) (workspace.ChapterView, error)
func (a *DesktopAPI) OpenChapter(id string) (workspace.ChapterView, error)
func (a *DesktopAPI) SaveDraft(id, body string, baseRevision int64) error
func (a *DesktopAPI) CommitChapter(id, body string, baseRevision int64) (workspace.ChapterView, error)
```

目录对话框只允许选择目录；打开项目后由应用服务验证 `.zjproject`、数据库完整性和项目记录。

- [ ] **Step 3: 在 `newApp` 组装生产依赖**

依赖方向固定为：

```text
Wails lifecycle
  -> DesktopAPI
  -> workspace.Service
  -> license.NoopService + sqlite.Factory + projectfs.Layout
  -> modernc SQLite / operating system
```

`main.go` 的 `Bind` 只绑定 `DesktopAPI`，不绑定仓储或领域对象。`OnShutdown` 调用关闭服务。

- [ ] **Step 4: 生成绑定并验证**

Run:

```bash
go test ./apps/desktop ./internal/... -race
cd apps/desktop
"$(go env GOPATH)/bin/wails" build -clean -m
```

Expected: `wails build` 自动生成 TypeScript bindings 并完成桌面构建；若生成位置与 `StudioClient` adapter 不一致，只调整 adapter，不让 feature 组件导入生成代码。

```bash
git add apps/desktop internal/platform
git commit -m "feat: expose workspace through thin desktop API"
```

## Task 8：实现创建/打开项目与章节编辑器

**Files:**

- Create: `apps/desktop/frontend/src/lib/wails-studio-client.ts`
- Create: `apps/desktop/frontend/src/lib/fake-studio-client.ts`
- Create: `apps/desktop/frontend/src/features/workspace/Workspace.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/Workspace.test.tsx`
- Create: `apps/desktop/frontend/src/features/workspace/workspace-store.ts`
- Create: `apps/desktop/frontend/src/features/editor/ChapterEditor.tsx`
- Create: `apps/desktop/frontend/src/features/editor/ChapterEditor.test.tsx`
- Modify: `apps/desktop/frontend/src/app/App.tsx`

- [ ] **Step 1: 写项目入口失败测试**

测试用户事件：

1. 点击“创建项目”。
2. native client 返回父目录。
3. 填写书名、类型、简介并确认。
4. `createProject` 只调用一次。
5. 成功后进入空章节工作区；取消选择时仍停留欢迎页。

- [ ] **Step 2: 写编辑器失败测试**

测试必须覆盖：

- 新建章节后自动选中并聚焦正文。
- 输入后显示“草稿保存中/已保存”状态。
- `saveDraft` 使用当前 revision，但不更新 revision。
- `commitChapter` 成功后用返回 revision 替换本地 revision。
- revision conflict 时保留本地文本，显示比较/重新加载提示，不静默覆盖。
- 打开含 recovery draft 的章节时先提示恢复或丢弃。

使用 Vitest fake timers 验证 200ms 草稿保存和 750ms 正式提交，不使用真实等待。

- [ ] **Step 3: 实现客户端 adapter**

`wails-studio-client.ts` 是唯一可导入 Wails 生成绑定的文件。它把生成 DTO 转换为 `studio-client.ts` 的显式类型，并用 Zod 在开发环境校验返回值。测试和 Storybook/浏览器预览使用 `FakeStudioClient`。

- [ ] **Step 4: 实现工作区状态机**

状态至少包含：

```ts
type WorkspaceStatus =
  | { kind: "welcome" }
  | { kind: "opening" }
  | { kind: "ready"; project: ProjectView; chapters: ChapterView[]; activeChapterId: string | null }
  | { kind: "error"; message: string };
```

把异步调用放在 store actions；组件只渲染状态并发送用户意图。不要在 store 中保存 native 路径以外的敏感信息。

- [ ] **Step 5: 实现编辑器保存控制器**

每次输入：标记 dirty；200ms 后调用 `saveDraft`；用户停止输入 750ms 后调用 `commitChapter`。同一章节同时最多一个提交请求；请求期间的新输入排入下一次提交。切换章节/关闭窗口前 flush 当前 draft；不得通过阻塞 UI 等待网络，因为本阶段全为本地调用。

- [ ] **Step 6: 验证和提交**

```bash
cd apps/desktop/frontend
npm test
npm run typecheck
npm run build
cd ..
"$(go env GOPATH)/bin/wails" build -clean -m
git add frontend
git commit -m "feat: create projects and edit chapters"
```

## Task 9：完成崩溃恢复、数据完整性和错误体验

**Files:**

- Create: `internal/application/workspace/recovery_test.go`
- Create: `internal/persistence/sqlite/integrity.go`
- Create: `internal/persistence/sqlite/integrity_test.go`
- Create: `apps/desktop/frontend/src/features/editor/RecoveryDialog.tsx`
- Create: `apps/desktop/frontend/src/features/editor/RecoveryDialog.test.tsx`
- Create: `docs/operations/project-recovery.md`

- [ ] **Step 1: 写端到端式 Go 恢复测试**

使用真实临时 SQLite 数据库，不使用 fake store：

1. 创建项目和章节，提交正文 A。
2. 保存基于 revision 2 的草稿 B，不提交。
3. 关闭并重新创建整个 workspace service。
4. 打开同一项目和章节。
5. 断言 committed body 是 A，recovery body 是 B，用户选择前两者都未被改写。
6. 接受恢复后提交 B，revision 递增且 draft 清除。

- [ ] **Step 2: 实现打开时完整性检查**

项目打开顺序：路径/后缀验证 → 只读探测 → SQLite header/open → migration → `PRAGMA quick_check` → 验证唯一 project row → 激活 workspace。任一步失败都不替换当前已打开项目。

错误至少分类为：项目不存在、无权限、数据库损坏、版本过新、revision 冲突、磁盘写入失败。UI 只显示安全摘要和恢复动作，不显示 SQL 或正文。

- [ ] **Step 3: 实现恢复对话框**

对话框显示已提交版和恢复草稿的时间/字数摘要，提供：

- 恢复草稿：把草稿放入编辑器，随后正常提交。
- 保留已提交版：明确确认后删除 draft。
- 稍后决定：保持 draft，不阻止只读查看。

- [ ] **Step 4: 编写操作文档并验证异常路径**

`project-recovery.md` 说明备份项目包、只读复制、恢复草稿和损坏库处理。测试磁盘只读、目录被删除、数据库锁等待超时和旧窗口 revision 冲突。

```bash
go test ./internal/... -race -count=1
cd apps/desktop/frontend && npm test && npm run build
git add internal apps/desktop/frontend docs/operations/project-recovery.md
git commit -m "feat: recover manuscript drafts after interruption"
```

## Task 10：建立双平台 CI、验收脚本和阶段门禁

**Files:**

- Create: `.github/workflows/ci.yml`
- Create: `docs/testing/foundation-acceptance.md`
- Modify: `README.md`
- Modify: `docs/handoff/START-HERE.md`

- [ ] **Step 1: 建立 Go/前端测试矩阵**

CI 至少包含：

- Ubuntu：`go test ./apps/desktop ./internal/... -race`、`go vet ./apps/desktop ./internal/...`、前端 test/typecheck/build。
- macOS：安装 Wails v2.13.0，执行 `wails doctor` 与 production build。
- Windows：安装 Wails v2.13.0，执行 production build 和 Go tests。
- npm 使用 `npm ci`，Go/NPM cache key 包含 lockfile hash。
- CI 不访问真实模型 API，不注入任何用户密钥。

- [ ] **Step 2: 编写手工验收清单**

`foundation-acceptance.md` 按 Windows/macOS 分栏记录：

1. 干净启动。
2. 创建含中英文和空格名称的项目。
3. 新建三章并验证顺序。
4. 连续输入、快速切换章节并重启。
5. 模拟未提交草稿并恢复。
6. 同一章节双窗口 revision conflict。
7. 只读目录、磁盘空间不足和项目路径丢失。
8. 项目包在两个系统间往返打开。

- [ ] **Step 3: 更新入口文档**

README 必须给出可复制的开发命令、目录边界、数据位置、隐私规则和已知限制。`START-HERE` 指向批准规格、总体路线图、本计划和 ADR-0003。

- [ ] **Step 4: 运行最终验证**

Run:

```bash
go test ./apps/desktop ./internal/... -race -count=1
go vet ./apps/desktop ./internal/...
cd apps/desktop/frontend
npm ci
npm test
npm run typecheck
npm run build
cd ..
"$(go env GOPATH)/bin/wails" build -clean -m
git status --short
```

Expected: 所有命令退出码为 0；Git 状态只包含当前任务有意修改；构建产物、数据库和 bindings 未被跟踪。

- [ ] **Step 5: 独立审查并提交**

在提交前使用 `superpowers:requesting-code-review` 检查架构边界、数据安全、Windows/macOS 差异和计划验收标准。处理反馈后重新运行 Step 4。

```bash
git add .github/workflows/ci.yml README.md docs/handoff/START-HERE.md docs/testing/foundation-acceptance.md
git commit -m "ci: verify foundation on Windows and macOS"
```

## 完成定义

只有同时满足以下条件，本纵向切片才算完成：

- [ ] macOS 与 Windows CI 生产构建通过。
- [ ] 创建/打开项目、新建章节、自动保存、冲突处理和草稿恢复全部有自动测试。
- [ ] 项目数据库可从空库迁移，异常打开不会覆盖原数据或当前工作区。
- [ ] React 只依赖 `StudioClient`，Wails API 只调用应用服务，领域层无 Wails/SQLite 依赖。
- [ ] 创建和编辑已经过 `LicenseService`，但 V1 `NoopService` 不限制用户。
- [ ] 原型仍能独立启动，未被正式代码改写。
- [ ] README、ADR、恢复文档和验收记录与实现一致。

完成后进入总体路线图 P1“故事工程与结构化模型”，并在编码前另写文件级实施计划。
