import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../lib/fake-studio-client";
import type {
  ApplicationBootstrapView,
  ProjectOpenResultView,
} from "../lib/studio-client";
import { StudioClientError } from "../lib/studio-client";
import { createAppShellStore } from "./app-shell-store";

const bootstrap: ApplicationBootstrapView = {
  preferences: {
    theme: "system",
    defaultCreativeProfileId: "",
    updatedAt: "2026-08-12T10:00:00Z",
  },
  recentProjects: [
    {
      projectId: "recent-1",
      title: "长夜织境",
      path: "/books/长夜织境.zjproject",
      lastWorkspace: "manuscript",
      lastOpenedAt: "2026-08-10T08:00:00Z",
      missing: false,
    },
  ],
  providers: [],
  profiles: [],
};

describe("app shell store", () => {
  it("loads bootstrap and surfaces home status", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);

    await store.getState().load();

    expect(store.getState().status).toMatchObject({
      kind: "home",
      bootstrap,
    });
    expect(store.getState().bootstrap).toEqual(bootstrap);
  });

  it("opens a recent project by id and transitions to project status", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();

    await store.getState().openRecentProject("recent-1");

    expect(client.openRecentProjectCalls).toEqual(["recent-1"]);
    expect(store.getState().status).toMatchObject({
      kind: "project",
      project: { id: "project-1" },
    });
  });

  it("offers a recovery action when openRecentProject fails with PROJECT_RELOCATE_REQUIRED", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    client.openRecentProjectError = {
      code: "PROJECT_RELOCATE_REQUIRED",
      message: "项目已移动或路径不可用。",
    };
    const store = createAppShellStore(client);
    await store.getState().load();

    await store.getState().openRecentProject("recent-1");

    const status = store.getState().status;
    expect(status).toMatchObject({
      kind: "error",
      message: "项目已移动或路径不可用，请重新定位或移除该最近项目。",
    });
    if (status.kind === "error") {
      expect(status.recovery).toMatchObject({ label: "重新定位项目" });
    }
  });

  it("surfaces a generic error when bootstrap fails", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapError = new Error("无法读取配置");
    const store = createAppShellStore(client);

    await store.getState().load();

    expect(store.getState().status).toMatchObject({
      kind: "error",
      message: "无法读取配置",
    });
  });

  it("retries initial bootstrap after dismissing the startup error", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapError = new Error("无法读取配置");
    const store = createAppShellStore(client);
    await store.getState().load();

    client.applicationBootstrapError = null;
    store.getState().dismissError();

    await vi.waitFor(() => {
      expect(store.getState().status.kind).toBe("home");
    });
    expect(client.applicationBootstrapCalls).toBe(2);
  });

  it("opens and closes settings while keeping the project mounted", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();

    store.getState().openSettings();
    expect(store.getState().status.kind).toBe("settings");

    // Settings does not unmount the bootstrap, so the user can return home.
    store.getState().closeSettings();
    expect(store.getState().status.kind).toBe("home");
  });

  it("opens settings from project mode and returns to project on close", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();
    await store.getState().openRecentProject("recent-1");
    expect(store.getState().status.kind).toBe("project");

    store.getState().openSettings();
    const settingsStatus = store.getState().status;
    expect(settingsStatus.kind).toBe("settings");
    if (settingsStatus.kind === "settings") {
      expect(settingsStatus.project).toBeDefined();
    }

    store.getState().closeSettings();
    expect(store.getState().status.kind).toBe("project");
  });

  it("ensures only one lifecycle operation runs at a time", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    let resolveOpen: ((result: ProjectOpenResultView) => void) | undefined;
    client.openRecentProject = vi.fn(
      () =>
        new Promise<ProjectOpenResultView>((resolve) => {
          resolveOpen = resolve;
        }),
    );
    const store = createAppShellStore(client);
    await store.getState().load();

    const firstCall = store.getState().openRecentProject("recent-1");
    // Second call should be ignored because the first is in-flight.
    await store.getState().openRecentProject("recent-2");
    expect(store.getState().busy).toBe(true);

    resolveOpen?.({
      outcome: "opened",
      project: {
        id: "project-1",
        title: "长夜织境",
        genre: "",
        summary: "",
        path: "/books/长夜织境.zjproject",
        accessMode: "editable",
        filesystemHealth: { level: "normal", category: "", actions: [] },
      },
    });
    await firstCall;

    expect(store.getState().status.kind).toBe("project");
    expect(client.openRecentProject).toHaveBeenCalledTimes(1);
  });

  it("handles a lease conflict by surfacing an error without entering project mode", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    client.projectOpenResponse = {
      outcome: "lease_conflict",
      conflict: { path: "/books/长夜织境.zjproject", retryable: true },
    };
    const store = createAppShellStore(client);
    await store.getState().load();

    await store.getState().openRecentProject("recent-1");

    expect(store.getState().status).toMatchObject({
      kind: "error",
      message: expect.stringContaining("另一个实例编辑"),
    });
  });

  it("reloads bootstrap when closeProject is invoked", async () => {
    const client = new FakeStudioClient();
    const updated: ApplicationBootstrapView = {
      ...bootstrap,
      recentProjects: [],
    };
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();
    await store.getState().openRecentProject("recent-1");
    expect(store.getState().status.kind).toBe("project");

    client.applicationBootstrapResult = updated;
    await store.getState().closeProject();

    expect(client.closeProjectCalls).toBe(1);
    expect(store.getState().status).toMatchObject({
      kind: "home",
      bootstrap: updated,
    });
  });

  it("subscribes to menu commands and invokes closeProject on close_project", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();
    await store.getState().openRecentProject("recent-1");

    const beforeClose = vi.fn(async () => undefined);
    const unsubscribe = store.getState().subscribeMenuCommands(client, {
      onBeforeProjectClose: beforeClose,
    });

    client.emitMenuCommand({ type: "close_project" });
    // Wait for the async handler chain to complete.
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(beforeClose).toHaveBeenCalledOnce();
    expect(client.closeProjectCalls).toBe(1);

    unsubscribe();
  });

  it("flushes pending edits and completes application close on quit_application", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();
    const beforeClose = vi.fn(async () => undefined);
    const unsubscribe = store.getState().subscribeMenuCommands(client, {
      onBeforeProjectClose: beforeClose,
    });

    client.emitMenuCommand({ type: "quit_application" });
    await vi.waitFor(() => {
      expect(client.completeCloseCalls).toBe(1);
    });

    expect(beforeClose).toHaveBeenCalledOnce();
    expect(client.closeProjectCalls).toBe(0);
    unsubscribe();
  });

  it("keeps the application open when pending edits fail on quit_application", async () => {
    const client = new FakeStudioClient();
    const store = createAppShellStore(client);
    const unsubscribe = store.getState().subscribeMenuCommands(client, {
      onBeforeProjectClose: vi.fn(async () => {
        throw new Error("草稿保存失败");
      }),
    });

    client.emitMenuCommand({ type: "quit_application" });
    await Promise.resolve();
    await Promise.resolve();

    expect(client.completeCloseCalls).toBe(0);
    unsubscribe();
  });

  it("opens settings on app_settings menu command", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    const store = createAppShellStore(client);
    await store.getState().load();
    expect(store.getState().status.kind).toBe("home");

    const unsubscribe = store.getState().subscribeMenuCommands(client);
    client.emitMenuCommand({ type: "app_settings" });
    expect(store.getState().status.kind).toBe("settings");

    unsubscribe();
  });

  it("rejects PROJECT_RELOCATE_REQUIRED as a typed StudioClientError", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = bootstrap;
    client.openRecentProject = vi.fn(async () => {
      throw new StudioClientError(
        "PROJECT_RELOCATE_REQUIRED",
        "项目已移动或路径不可用。",
      );
    });
    const store = createAppShellStore(client);
    await store.getState().load();

    await store.getState().openRecentProject("recent-1");

    const status = store.getState().status;
    expect(status.kind).toBe("error");
    if (status.kind === "error") {
      expect(status.recovery).toBeDefined();
    }
  });
});
