import { createStore, type StoreApi } from "zustand/vanilla";
import type {
  ChapterView,
  CreateProjectInput,
  ProjectAccessMode,
  ProjectLeaseConflictView,
  ProjectView,
  SnapshotRestoreView,
  StudioClient,
} from "../../lib/studio-client";

type WorkspaceReplacement = Pick<SnapshotRestoreView, "project" | "chapters">;

export type WorkspaceStatus =
  | { kind: "welcome" }
  | { kind: "opening" }
  | {
      kind: "ready";
      project: ProjectView;
      chapters: ChapterView[];
      activeChapterId: string | null;
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
  beginCreate(): Promise<void>;
  cancelCreate(): void;
  createProject(input: Omit<CreateProjectInput, "parentDirectory">): Promise<void>;
  openProject(): Promise<void>;
  retryProjectOpen(): Promise<void>;
  openProjectReadOnly(): Promise<void>;
  createRecoveryCopy(): Promise<void>;
  cancelProjectOpen(): void;
  loadProject(project: ProjectView): Promise<void>;
  createChapter(title: string): Promise<boolean>;
  selectChapter(id: string): Promise<void>;
  updateChapter(chapter: ChapterView): void;
  replaceChapters(chapters: ChapterView[]): void;
  restoreWorkspace(result: WorkspaceReplacement): void;
  clearWorkspaceError(): void;
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

    return {
    status: { kind: "welcome" },
    pendingParentDirectory: null,
    workspaceError: null,

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
        });
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
      const sourcePath =
        current.kind === "lease_conflict"
          ? current.conflict.path
          : current.kind === "ready" && current.project.accessMode === "editable"
            ? current.project.path
            : null;
      if (!sourcePath || (current.kind === "lease_conflict" && current.busy)) return;
      try {
        if (current.kind === "lease_conflict") {
          set({ status: { ...current, busy: true } });
        }
        const parentDirectory = await client.chooseProjectParent();
        if (!parentDirectory) {
          if (current.kind === "lease_conflict") {
            set({ status: { ...current, busy: false } });
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
        });
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
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
        };
      });
    },

    clearWorkspaceError() {
      set({ workspaceError: null });
    },
  };
  });
}
