import { createStore, type StoreApi } from "zustand/vanilla";
import type {
  ChapterView,
  CreateProjectInput,
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
  | { kind: "error"; message: string };

export type WorkspaceState = {
  status: WorkspaceStatus;
  pendingParentDirectory: string | null;
  workspaceError: string | null;
  beginCreate(): Promise<void>;
  cancelCreate(): void;
  createProject(input: Omit<CreateProjectInput, "parentDirectory">): Promise<void>;
  openProject(): Promise<void>;
  loadProject(project: ProjectView): Promise<void>;
  createChapter(title: string): Promise<boolean>;
  selectChapter(id: string): Promise<void>;
  updateChapter(chapter: ChapterView): void;
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
  return createStore<WorkspaceState>((set, get) => ({
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
      selectionRequest++;
      set({ status: { kind: "opening" } });
      try {
        const path = await client.chooseProjectToOpen();
        if (!path) {
          set({ status: { kind: "welcome" } });
          return;
        }
        const project = await client.openProject(path);
        await get().loadProject(project);
      } catch (error) {
        set({ status: { kind: "error", message: errorMessage(error) } });
      }
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
      if (current.kind !== "ready") return false;
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
        if (latest.status.kind !== "ready") return {};
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

    restoreWorkspace(result) {
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
  }));
}
