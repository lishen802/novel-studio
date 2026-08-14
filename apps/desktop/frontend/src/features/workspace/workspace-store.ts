import { createStore, type StoreApi } from "zustand/vanilla";
import type {
  AIPanelSelection,
  ChapterView,
  CreateProjectInput,
  ProjectAccessMode,
  ProjectLeaseConflictView,
  ProjectView,
  SnapshotRestoreView,
  StudioClient,
  WorkbenchProjection,
} from "../../lib/studio-client";

type WorkspaceReplacement = Pick<SnapshotRestoreView, "project" | "chapters">;

// AIPanelDraft 保存某个项目下未发送的 AI 面板输入和章节选区。切换画布、
// 打开应用设置或刷新 workbench 都不能清除这些草稿；提交选区意见时若
// baseRevision 与当前章节不一致，后端返回 STALE_SELECTION 要求重选。
export type AIPanelDraft = {
  input: string;
  selection?: AIPanelSelection;
};

// 项目导航分组的二级页面 ID。一级分组固定，二级页面展开在其下；
// stage/canvas/counts 始终从 client.projectWorkbench() 投影，不在此处推断。
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

export type WorkspaceStatus =
  | { kind: "welcome" }
  | { kind: "opening" }
  | {
      kind: "ready";
      project: ProjectView;
      chapters: ChapterView[];
      activeChapterId: string | null;
      recoveryCopying?: boolean;
    }
  | {
      kind: "lease_conflict";
      conflict: ProjectLeaseConflictView;
      busy?: boolean;
    }
  | { kind: "error"; message: string; publishedRecoveryPath?: string };

export type WorkspaceState = {
  status: WorkspaceStatus;
  pendingParentDirectory: string | null;
  workspaceError: string | null;
  // 当前选中的二级页面。仅本地导航状态；权威的 stage/canvas 来自 workbench 投影。
  currentPage: ProjectPage;
  // 权威工作台投影。loadProject 时拉取，refreshWorkbench 可手动刷新。
  workbench: WorkbenchProjection | null;
  // AI 面板草稿：按 project ID 存储未发送的输入和章节选区。切换画布、
  // 打开应用设置或刷新 workbench 都不能清除这些草稿。
  aiPanelDraft: Record<string, AIPanelDraft>;
  // 当前是否有待提交的选区意见。selection_review 模式下由 ChapterEditor
  // 触发 onSelectionReview 写入；AIPanel 据此显示选区元数据和提交入口。
  pendingSelection: AIPanelSelection | null;
  beginCreate(): Promise<void>;
  cancelCreate(): void;
  createProject(input: Omit<CreateProjectInput, "parentDirectory">): Promise<void>;
  openProject(): Promise<void>;
  openRecentProject(projectId: string, targetPage?: ProjectPage): Promise<void>;
  retryProjectOpen(): Promise<void>;
  openProjectReadOnly(): Promise<void>;
  createRecoveryCopy(): Promise<void>;
  refreshFilesystemHealth(): Promise<void>;
  cancelProjectOpen(): void;
  loadProject(project: ProjectView): Promise<void>;
  loadImportedProject(result: {
    project: ProjectView;
    chapters: ChapterView[];
    workbench: WorkbenchProjection;
  }): Promise<void>;
  createChapter(title: string): Promise<boolean>;
  selectChapter(id: string): Promise<void>;
  updateChapter(chapter: ChapterView): void;
  replaceChapters(chapters: ChapterView[]): void;
  restoreWorkspace(result: WorkspaceReplacement): void;
  clearWorkspaceError(): void;
  setCurrentPage(page: ProjectPage): void;
  refreshWorkbench(): Promise<void>;
  setAIPanelInput(projectId: string, input: string): void;
  setAIPanelSelection(selection: AIPanelSelection): void;
  clearAIPanelSelection(): void;
  clearAIPanelDraft(projectId: string): void;
};

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "操作失败，请重试";
}

export function createWorkspaceStore(
  client: StudioClient,
): StoreApi<WorkspaceState> {
  let selectionRequest = 0;
  return createStore<WorkspaceState>((set, get) => {
    const openProjectAtPath = async (path: string, mode: ProjectAccessMode) => {
      const request = ++selectionRequest;
      set({ status: { kind: "opening" } });
      try {
        const result = await client.openProject(path, mode);
        if (request !== selectionRequest) return;
        if (result.outcome === "lease_conflict") {
          set({ status: { kind: "lease_conflict", conflict: result.conflict } });
          return;
        }
        await get().loadProject(result.project);
      } catch (error) {
        if (request === selectionRequest) {
          set({ status: { kind: "error", message: errorMessage(error) } });
        }
      }
    };

    const refreshWorkbenchForProject = async (projectId: string): Promise<void> => {
      try {
        const workbench = await client.projectWorkbench();
        set((latest) => {
          if (
            latest.status.kind !== "ready" ||
            latest.status.project.id !== projectId
          ) {
            return {};
          }
          return { workbench };
        });
      } catch {
        // 工作台投影失败不应阻断项目加载；UI 会显示占位状态。
      }
    };

    return {
    status: { kind: "welcome" },
    pendingParentDirectory: null,
    workspaceError: null,
    currentPage: "manuscript",
    workbench: null,
    aiPanelDraft: {},
    pendingSelection: null,

    async beginCreate() {
      set({ status: { kind: "opening" } });
      try {
        const parent = await client.chooseProjectParent();
        set({
          status: { kind: "welcome" },
          pendingParentDirectory: parent,
        });
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
    },

    cancelCreate() {
      set({ status: { kind: "welcome" }, pendingParentDirectory: null });
    },

    async createProject(input) {
      selectionRequest++;
      const parentDirectory = get().pendingParentDirectory;
      if (!parentDirectory) return;
      set({ status: { kind: "opening" } });
      try {
        const project = await client.createProject({
          parentDirectory,
          ...input,
        });
        const chapters = await client.listChapters();
        set({
          status: {
            kind: "ready",
            project,
            chapters,
            activeChapterId: null,
          },
          pendingParentDirectory: null,
          workspaceError: null,
          currentPage: "manuscript",
          workbench: null,
          pendingSelection: null,
        });
        void refreshWorkbenchForProject(project.id);
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
    },

    async openProject() {
      const request = ++selectionRequest;
      set({ status: { kind: "opening" } });
      try {
        const path = await client.chooseProjectToOpen();
        if (!path) {
          if (request === selectionRequest) set({ status: { kind: "welcome" } });
          return;
        }
        await openProjectAtPath(path, "editable");
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
    },

    // 最近项目打开使用请求序号隔离并发结果，迟到的旧请求不得覆盖新工作区。
    // targetPage 只决定前端初始画布，不改变后端项目打开契约。
    async openRecentProject(projectId, targetPage = "manuscript") {
      const request = ++selectionRequest;
      set({ status: { kind: "opening" } });
      try {
        const result = await client.openRecentProject(projectId);
        if (request !== selectionRequest) return;
        if (result.outcome === "lease_conflict") {
          set({ status: { kind: "lease_conflict", conflict: result.conflict } });
          return;
        }
        const chapters = await client.listChapters();
        if (request !== selectionRequest) return;
        set({
          status: {
            kind: "ready",
            project: result.project,
            chapters,
            activeChapterId: null,
          },
          workspaceError: null,
          currentPage: targetPage,
          workbench: null,
          pendingSelection: null,
        });
        void refreshWorkbenchForProject(result.project.id);
      } catch (error) {
        if (request === selectionRequest) {
          set({ status: { kind: "error", message: errorMessage(error) } });
        }
      }
    },

    async retryProjectOpen() {
      const current = get().status;
      if (current.kind !== "lease_conflict") return;
      await openProjectAtPath(current.conflict.path, "editable");
    },

    async openProjectReadOnly() {
      const current = get().status;
      if (current.kind !== "lease_conflict") return;
      await openProjectAtPath(current.conflict.path, "read_only");
    },

    async createRecoveryCopy() {
      const current = get().status;
      if (
        (current.kind === "lease_conflict" && current.busy) ||
        (current.kind === "ready" && current.recoveryCopying)
      ) {
        return;
      }
      const sourcePath =
        current.kind === "lease_conflict"
          ? current.conflict.path
          : current.kind === "ready" && current.project.accessMode === "editable"
            ? current.project.path
            : null;
      const readyProjectID =
        current.kind === "ready" ? current.project.id : null;
      if (!sourcePath) return;
      try {
        if (current.kind === "lease_conflict") {
          set({ status: { ...current, busy: true } });
        } else if (current.kind === "ready") {
          set({ status: { ...current, recoveryCopying: true } });
        }
        const parentDirectory = await client.chooseProjectParent();
        if (!parentDirectory) {
          if (current.kind === "lease_conflict") {
            set({ status: { ...current, busy: false } });
          } else if (current.kind === "ready") {
            set((latest) => {
              if (
                latest.status.kind !== "ready" ||
                latest.status.project.id !== readyProjectID
              ) {
                return {};
              }
              return {
                status: { ...latest.status, recoveryCopying: false },
              };
            });
          }
          return;
        }
        set({ status: { kind: "opening" } });
        const result = await client.createProjectRecoveryCopy({
          sourcePath,
          parentDirectory,
        });
        if (result.outcome === "opened") {
          await get().loadProject(result.project);
          return;
        }
        set({
          status: {
            kind: "error",
            message: "恢复副本已创建，但无法自动打开。请重新打开该副本。",
            publishedRecoveryPath: result.path,
          },
        });
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
    },

    async refreshFilesystemHealth() {
      const current = get().status;
      if (current.kind !== "ready") return;
      const projectID = current.project.id;
      try {
        const filesystemHealth = await client.filesystemHealth();
        set((latest) => {
          if (
            latest.status.kind !== "ready" ||
            latest.status.project.id !== projectID
          ) {
            return {};
          }
          return {
            status: {
              ...latest.status,
              project: { ...latest.status.project, filesystemHealth },
            },
          };
        });
      } catch {
        // The write failure and its local recovery controls remain visible even
        // when the health snapshot cannot be read.
      }
    },

    cancelProjectOpen() {
      selectionRequest++;
      set({ status: { kind: "welcome" } });
    },

    async loadProject(project) {
      selectionRequest++;
      set({ status: { kind: "opening" } });
      try {
        const chapters = await client.listChapters();
        set({
          status: {
            kind: "ready",
            project,
            chapters,
            activeChapterId: null,
          },
          workspaceError: null,
          currentPage: "manuscript",
          workbench: null,
          pendingSelection: null,
        });
        void refreshWorkbenchForProject(project.id);
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
    },

    // loadImportedProject 用于首页"导入小说并续写"入口：client 一次性返回
    // project + chapters + workbench，无需再额外调用 listChapters/projectWorkbench。
    // activeChapterId 强制为第一章，currentPage 强制为 manuscript。
    async loadImportedProject(result) {
      selectionRequest++;
      const activeChapterId = result.chapters[0]?.id ?? null;
      set({
        status: {
          kind: "ready",
          project: result.project,
          chapters: result.chapters,
          activeChapterId,
        },
        workspaceError: null,
        currentPage: "manuscript",
        workbench: result.workbench,
        pendingSelection: null,
      });
    },

    async createChapter(title) {
      const current = get().status;
      if (current.kind !== "ready" || current.project.accessMode !== "editable") {
        return false;
      }
      const projectID = current.project.id;
      selectionRequest++;
      set({ workspaceError: null });
      try {
        const chapter = await client.createChapter(title);
        let added = false;
        set((latest) => {
          if (
            latest.status.kind !== "ready" ||
            latest.status.project.id !== projectID
          ) {
            return {};
          }
          added = true;
          return {
            status: {
              ...latest.status,
              chapters: [...latest.status.chapters, chapter],
              activeChapterId: chapter.id,
            },
          };
        });
        return added;
      } catch (error) {
        set({ workspaceError: errorMessage(error) });
        return false;
      }
    },

    async selectChapter(id) {
      const current = get().status;
      if (current.kind !== "ready") return;
      const projectID = current.project.id;
      const request = ++selectionRequest;
      set({ workspaceError: null });
      try {
        const chapter = await client.openChapter(id);
        set((latest) => {
          if (
            request !== selectionRequest ||
            latest.status.kind !== "ready" ||
            latest.status.project.id !== projectID
          ) {
            return {};
          }
          return {
            status: {
              ...latest.status,
              chapters: latest.status.chapters.map((item) =>
                item.id === chapter.id ? chapter : item,
              ),
              activeChapterId: chapter.id,
            },
          };
        });
      } catch (error) {
        if (request === selectionRequest) {
          set({ workspaceError: errorMessage(error) });
        }
      }
    },

    updateChapter(chapter) {
      set((latest) => {
        if (
          latest.status.kind !== "ready" ||
          latest.status.project.accessMode !== "editable"
        ) {
          return {};
        }
        return {
          status: {
            ...latest.status,
            chapters: latest.status.chapters.map((item) =>
              item.id === chapter.id ? chapter : item,
            ),
          },
        };
      });
    },

    replaceChapters(chapters) {
      const current = get().status;
      if (current.kind !== "ready" || current.project.accessMode !== "editable") {
        return;
      }
      const ids = new Set<string>();
      const positions = new Set<number>();
      const invalid = chapters.some((chapter) => {
        const duplicate =
          ids.has(chapter.id) || positions.has(chapter.position);
        ids.add(chapter.id);
        positions.add(chapter.position);
        return (
          !chapter.id ||
          chapter.projectId !== current.project.id ||
          !Number.isInteger(chapter.position) ||
          chapter.position < 1 ||
          duplicate
        );
      });
      if (chapters.length === 0 || invalid) {
        set({ workspaceError: "导入章节数据无效，请重试。" });
        return;
      }
      const projectID = current.project.id;
      selectionRequest++;
      set((latest) => {
        if (
          latest.status.kind !== "ready" ||
          latest.status.project.id !== projectID
        ) {
          return {};
        }
        const currentWorkspace = latest.status;
        const activeChapterId = chapters.some(
          (chapter) => chapter.id === currentWorkspace.activeChapterId,
        )
          ? currentWorkspace.activeChapterId
          : (chapters[0]?.id ?? null);
        return {
          status: {
            ...currentWorkspace,
            chapters,
            activeChapterId,
          },
          workspaceError: null,
        };
      });
    },

    restoreWorkspace(result) {
      const current = get().status;
      if (current.kind === "ready" && current.project.accessMode !== "editable") {
        return;
      }
      selectionRequest++;
      set((latest) => {
        const previousChapterID =
          latest.status.kind === "ready"
            ? latest.status.activeChapterId
            : null;
        const activeChapterId =
          result.chapters.some(
            (chapter) => chapter.id === previousChapterID,
          )
            ? previousChapterID
            : (result.chapters[0]?.id ?? null);
        return {
          status: {
            kind: "ready",
            project: result.project,
            chapters: result.chapters,
            activeChapterId,
          },
          workspaceError: null,
          currentPage: "manuscript" as const,
          workbench: null,
          pendingSelection: null,
        };
      });
      void refreshWorkbenchForProject(result.project.id);
    },

    clearWorkspaceError() {
      set((latest) => ({
        workspaceError: null,
        ...(latest.status.kind === "error"
          ? {
              status: { kind: "welcome" } as const,
              pendingParentDirectory: null,
            }
          : {}),
      }));
    },

    setCurrentPage(page) {
      set({ currentPage: page });
    },

    async refreshWorkbench() {
      const current = get().status;
      if (current.kind !== "ready") return;
      await refreshWorkbenchForProject(current.project.id);
    },

    // setAIPanelInput 按 project ID 保存未发送的 AI 面板输入。切换画布、
    // 打开应用设置或刷新 workbench 都不能清除这些草稿。
    setAIPanelInput(projectId, input) {
      set((latest) => ({
        aiPanelDraft: {
          ...latest.aiPanelDraft,
          [projectId]: {
            ...latest.aiPanelDraft[projectId],
            input,
          },
        },
      }));
    },

    // setAIPanelSelection 设置当前待提交的选区意见。ChapterEditor 在作者
    // 选中文本并点击"告诉 AI 这里有问题"时调用；AIPanel 据此进入
    // selection_review 模式并显示选区元数据。
    setAIPanelSelection(selection) {
      set({ pendingSelection: selection });
    },

    // clearAIPanelSelection 清除当前选区意见，退出 selection_review 模式。
    clearAIPanelSelection() {
      set({ pendingSelection: null });
    },

    // clearAIPanelDraft 清除某个项目的 AI 面板草稿（输入和选区）。
    clearAIPanelDraft(projectId) {
      set((latest) => {
        const next = { ...latest.aiPanelDraft };
        delete next[projectId];
        return { aiPanelDraft: next, pendingSelection: null };
      });
    },
  };
  });
}
