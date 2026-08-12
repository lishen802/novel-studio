import { createStore, type StoreApi } from "zustand/vanilla";
import {
  StudioClientError,
  type ApplicationBootstrapView,
  type ProjectView,
  type StudioClient,
} from "../lib/studio-client";

export type AppShellStatus =
  | { kind: "loading" }
  | { kind: "home"; bootstrap: ApplicationBootstrapView }
  | { kind: "settings"; bootstrap: ApplicationBootstrapView; project?: ProjectView }
  | { kind: "project"; project: ProjectView }
  | { kind: "error"; message: string; recovery?: AppRecoveryAction };

export type AppRecoveryAction = {
  label: string;
  action: () => Promise<void>;
};

export type AppShellState = {
  status: AppShellStatus;
  bootstrap: ApplicationBootstrapView | null;
  busy: boolean;
  load(): Promise<void>;
  openSettings(): void;
  closeSettings(): void;
  openRecentProject(projectId: string): Promise<void>;
  enterProject(project: ProjectView): void;
  closeProject(): Promise<void>;
  reloadBootstrap(): Promise<void>;
  dismissError(): void;
  subscribeMenuCommands(
    client: StudioClient,
    options?: {
      onBeforeProjectClose?: () => Promise<void>;
    },
  ): () => void;
};

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "操作失败，请重试";
}

const RELOCATE_ERROR_CODE = "PROJECT_RELOCATE_REQUIRED";

function isRelocateError(error: unknown): boolean {
  if (error instanceof StudioClientError) {
    return error.code === RELOCATE_ERROR_CODE;
  }
  if (error instanceof Error) {
    return /PROJECT_RELOCATE_REQUIRED|项目已移动|路径不可用/u.test(
      error.message,
    );
  }
  return false;
}

export function createAppShellStore(
  client: StudioClient,
): StoreApi<AppShellState> {
  let lifecycleRequest = 0;
  return createStore<AppShellState>((set, get) => ({
    status: { kind: "loading" },
    bootstrap: null,
    busy: false,

    async load() {
      const request = ++lifecycleRequest;
      set({ status: { kind: "loading" } });
      try {
        const bootstrap = await client.applicationBootstrap();
        if (request !== lifecycleRequest) return;
        set({ status: { kind: "home", bootstrap }, bootstrap });
      } catch (error) {
        if (request === lifecycleRequest) {
          set({
            status: { kind: "error", message: errorMessage(error) },
          });
        }
      }
    },

    openSettings() {
      const current = get().status;
      const bootstrap = get().bootstrap;
      if (!bootstrap) return;
      // 允许从首页或项目模式打开设置；从项目模式打开时保留项目状态
      if (current.kind === "project") {
        set({ status: { kind: "settings", bootstrap, project: current.project } });
      } else if (current.kind === "home") {
        set({ status: { kind: "settings", bootstrap } });
      }
    },

    closeSettings() {
      const current = get().status;
      if (current.kind !== "settings") return;
      // 如果设置是从项目模式打开的，关闭后返回项目模式
      if (current.project) {
        set({ status: { kind: "project", project: current.project } });
      } else {
        set({ status: { kind: "home", bootstrap: current.bootstrap } });
      }
    },

    async openRecentProject(projectId) {
      if (get().busy) return;
      const request = ++lifecycleRequest;
      set({ busy: true });
      try {
        const result = await client.openRecentProject(projectId);
        if (request !== lifecycleRequest) return;
        if (result.outcome === "lease_conflict") {
          set({
            status: {
              kind: "error",
              message:
                "项目正在由另一个实例编辑，请稍后重试或只读打开。",
            },
          });
          return;
        }
        set({ status: { kind: "project", project: result.project } });
      } catch (error) {
        if (request !== lifecycleRequest) return;
        if (isRelocateError(error)) {
          set({
            status: {
              kind: "error",
              message: "项目已移动或路径不可用，请重新定位或移除该最近项目。",
              recovery: {
                label: "重新定位项目",
                action: async () => {
                  await get().reloadBootstrap();
                },
              },
            },
          });
          return;
        }
        set({
          status: { kind: "error", message: errorMessage(error) },
        });
      } finally {
        if (request === lifecycleRequest) set({ busy: false });
      }
    },

    enterProject(project) {
      set({ status: { kind: "project", project } });
    },

    async closeProject() {
      if (get().busy) return;
      const request = ++lifecycleRequest;
      set({ busy: true });
      try {
        await client.closeProject();
        if (request !== lifecycleRequest) return;
        await get().reloadBootstrap();
      } catch (error) {
        if (request !== lifecycleRequest) return;
        set({
          status: { kind: "error", message: errorMessage(error) },
        });
      } finally {
        if (request === lifecycleRequest) set({ busy: false });
      }
    },

    async reloadBootstrap() {
      const request = ++lifecycleRequest;
      try {
        const bootstrap = await client.applicationBootstrap();
        if (request !== lifecycleRequest) return;
        set({ status: { kind: "home", bootstrap }, bootstrap });
      } catch (error) {
        if (request === lifecycleRequest) {
          set({
            status: { kind: "error", message: errorMessage(error) },
          });
        }
      }
    },

    dismissError() {
      const current = get().status;
      if (current.kind !== "error") return;
      const bootstrap = get().bootstrap;
      if (bootstrap) {
        set({ status: { kind: "home", bootstrap } });
      } else {
        void get().load();
      }
    },

    subscribeMenuCommands(target, options) {
      return target.onMenuCommand((command) => {
        if (command.type === "close_project") {
          void (async () => {
            try {
              await options?.onBeforeProjectClose?.();
            } catch {
              // Flush errors should not block closing the project.
            }
            await get().closeProject();
          })();
        } else if (command.type === "quit_application") {
          void (async () => {
            await options?.onBeforeProjectClose?.();
            await target.completeClose();
          })().catch(() => {
            // 草稿刷新失败时保留应用窗口，避免未保存文本丢失。
          });
        } else if (command.type === "app_settings") {
          get().openSettings();
        }
      });
    },
  }));
}
