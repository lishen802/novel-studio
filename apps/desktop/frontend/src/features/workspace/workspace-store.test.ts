import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChapterView,
  ProjectOpenResultView,
  SnapshotRestoreView,
  WorkbenchProjection,
} from "../../lib/studio-client";
import { createWorkspaceStore } from "./workspace-store";

describe("workspace store", () => {
  it("ignores a stale recent-project failure after a newer open succeeds", async () => {
    const client = new FakeStudioClient();
    let rejectFirst: ((error: Error) => void) | undefined;
    let resolveSecond: ((result: ProjectOpenResultView) => void) | undefined;
    client.openRecentProject = vi.fn((projectId: string) => {
      if (projectId === "recent-1") {
        return new Promise<ProjectOpenResultView>((_resolve, reject) => {
          rejectFirst = reject;
        });
      }
      return new Promise<ProjectOpenResultView>((resolve) => {
        resolveSecond = resolve;
      });
    });
    const store = createWorkspaceStore(client);

    const firstOpen = store.getState().openRecentProject("recent-1");
    const secondOpen = store.getState().openRecentProject("recent-2");
    resolveSecond?.({
      outcome: "opened",
      project: { ...client.openProjectResult, id: "project-2", title: "新作品" },
    });
    await secondOpen;
    rejectFirst?.(new Error("旧请求迟到失败"));
    await firstOpen;

    expect(store.getState().status).toMatchObject({
      kind: "ready",
      project: { id: "project-2", title: "新作品" },
    });
  });

  it("returns to the welcome state when a project-open error is dismissed", () => {
    const store = createWorkspaceStore(new FakeStudioClient());
    store.setState({
      status: { kind: "error", message: "项目数据无法读取" },
      workspaceError: "项目数据无法读取",
    });

    store.getState().clearWorkspaceError();

    expect(store.getState().status).toEqual({ kind: "welcome" });
    expect(store.getState().workspaceError).toBeNull();
  });

  it("preserves ready workspace updates when recovery-copy directory selection is cancelled", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = { ...client.createChapterResult, body: "原始正文" };
    client.chaptersResult = [chapter];
    let chooseParent: ((path: string | null) => void) | undefined;
    client.chooseProjectParent = vi.fn(
      () => new Promise<string | null>((resolve) => {
        chooseParent = resolve;
      }),
    );
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);

    const creatingCopy = store.getState().createRecoveryCopy();
    store.getState().updateChapter({ ...chapter, body: "选择目录期间的新正文", revision: 2 });
    chooseParent?.(null);
    await creatingCopy;

    const status = store.getState().status;
    expect(status).toMatchObject({ kind: "ready", recoveryCopying: false });
    if (status.kind === "ready") {
      expect(status.chapters).toContainEqual(
        expect.objectContaining({ body: "选择目录期间的新正文", revision: 2 }),
      );
    }
  });

  it("keeps a lease conflict actionable so the author can retry or open read-only", async () => {
    const client = new FakeStudioClient();
    client.projectToOpen = "/books/长夜织境.zjproject";
    client.projectOpenResponse = {
      outcome: "lease_conflict",
      conflict: {
        path: client.projectToOpen,
        retryable: true,
        holder: {
          pid: 42,
          hostname: "author-mac",
          applicationVersion: "0.1.0",
          startedAt: "2026-08-10T09:00:00Z",
        },
      },
    } satisfies ProjectOpenResultView;
    const store = createWorkspaceStore(client);

    await store.getState().openProject();

    expect(store.getState().status).toMatchObject({
      kind: "lease_conflict",
      conflict: { path: client.projectToOpen, holder: { hostname: "author-mac" } },
    });

    client.projectOpenResponse = {
      outcome: "opened",
      project: { ...client.openProjectResult, accessMode: "read_only" },
    };
    await store.getState().openProjectReadOnly();

    expect(store.getState().status).toMatchObject({
      kind: "ready",
      project: { accessMode: "read_only" },
    });
    expect(client.openProjectInputs).toEqual([
      { path: "/books/长夜织境.zjproject", mode: "editable" },
      { path: "/books/长夜织境.zjproject", mode: "read_only" },
    ]);
  });

  it("opens the published recovery copy and retains the published path on a partial result", async () => {
    const client = new FakeStudioClient();
    client.projectToOpen = "/books/长夜织境.zjproject";
    client.projectOpenResponse = {
      outcome: "lease_conflict",
      conflict: { path: client.projectToOpen, retryable: true },
    };
    client.projectParent = "/copies";
    client.recoveryCopyResponse = {
      outcome: "published",
      path: "/copies/长夜织境-恢复副本.zjproject",
    };
    const store = createWorkspaceStore(client);

    await store.getState().openProject();
    await store.getState().createRecoveryCopy();

    expect(client.createProjectRecoveryCopyCalls).toEqual([
      { sourcePath: "/books/长夜织境.zjproject", parentDirectory: "/copies" },
    ]);
    expect(store.getState().status).toMatchObject({
      kind: "error",
      publishedRecoveryPath: "/copies/长夜织境-恢复副本.zjproject",
    });
  });

  it("does not lose a concurrent chapter update when selection finishes", async () => {
    const client = FakeStudioClient.withReadyProject();
    const first = client.createChapterResult;
    const second: ChapterView = {
      ...first,
      id: "chapter-2",
      title: "第二章",
      position: 2,
    };
    client.chaptersResult = [first, second];
    let finishOpen: ((chapter: ChapterView) => void) | undefined;
    client.openChapter = async () =>
      new Promise((resolve) => {
        finishOpen = resolve;
      });
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);

    const selecting = store.getState().selectChapter(second.id);
    store.getState().updateChapter({ ...first, body: "已提交", revision: 2 });
    finishOpen?.(second);
    await selecting;

    const status = store.getState().status;
    expect(status.kind).toBe("ready");
    if (status.kind === "ready") {
      expect(status.chapters.find((chapter) => chapter.id === first.id)).toMatchObject({
        body: "已提交",
        revision: 2,
      });
    }
  });

  it("keeps the active workspace when creating a chapter fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.createChapterError = new Error("无法写入章节");
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);

    const created = await store.getState().createChapter("第一章");

    expect(created).toBe(false);
    expect(store.getState().status.kind).toBe("ready");
    expect(store.getState().workspaceError).toBe("无法写入章节");
  });

  it("keeps the latest chapter selection when opens finish out of order", async () => {
    const client = FakeStudioClient.withReadyProject();
    const first = client.createChapterResult;
    const second: ChapterView = {
      ...first,
      id: "chapter-2",
      title: "第二章",
      position: 2,
    };
    client.chaptersResult = [first, second];
    const finish = new Map<string, (chapter: ChapterView) => void>();
    client.openChapter = async (id) =>
      new Promise((resolve) => {
        finish.set(id, resolve);
      });
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);

    const openingFirst = store.getState().selectChapter(first.id);
    const openingSecond = store.getState().selectChapter(second.id);
    finish.get(second.id)?.(second);
    await openingSecond;
    finish.get(first.id)?.(first);
    await openingFirst;

    const status = store.getState().status;
    expect(status.kind).toBe("ready");
    if (status.kind === "ready") expect(status.activeChapterId).toBe(second.id);
  });

  it("replaces the open workspace with the restored project snapshot", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    await store.getState().selectChapter(client.createChapterResult.id);
    const restored: SnapshotRestoreView = {
      snapshotId: "snapshot-1",
      project: {
        ...client.openProjectResult,
        title: "恢复后的作品",
      },
      chapters: [
        {
          ...client.createChapterResult,
          body: "快照中的正文",
          revision: 2,
        },
      ],
    };

    store.getState().restoreWorkspace(restored);

    expect(store.getState().status).toMatchObject({
      kind: "ready",
      project: { title: "恢复后的作品" },
      chapters: [{ body: "快照中的正文", revision: 2 }],
      activeChapterId: client.createChapterResult.id,
    });
  });

  it("replaces imported chapters while preserving a surviving active chapter", async () => {
    const client = FakeStudioClient.withReadyProject();
    const first = client.createChapterResult;
    const second = { ...first, id: "chapter-2", title: "第二章", position: 2 };
    client.chaptersResult = [first, second];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    await store.getState().selectChapter(second.id);
    const imported = [
      first,
      second,
      { ...first, id: "chapter-3", title: "第三章", position: 3 },
    ];

    store.getState().replaceChapters(imported);

    expect(store.getState().status).toMatchObject({
      kind: "ready",
      chapters: imported,
      activeChapterId: second.id,
    });
  });

  it("focuses the first imported chapter when the previous selection disappeared", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    await store.getState().selectChapter(client.createChapterResult.id);
    const replacement = [
      {
        ...client.createChapterResult,
        id: "chapter-imported",
        title: "导入章节",
      },
    ];

    store.getState().replaceChapters(replacement);

    expect(store.getState().status).toMatchObject({
      activeChapterId: "chapter-imported",
      chapters: replacement,
    });
  });

  it("rejects imported chapters from another project without changing the workspace", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    const before = store.getState().status;

    store.getState().replaceChapters([
      {
        ...client.createChapterResult,
        id: "foreign-chapter",
        projectId: "project-2",
      },
    ]);

    expect(store.getState().status).toEqual(before);
    expect(store.getState().workspaceError).toBe(
      "导入章节数据无效，请重试。",
    );
  });

  it("rejects duplicate imported chapter IDs without changing the workspace", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    const before = store.getState().status;
    const duplicate = {
      ...client.createChapterResult,
      id: "duplicate-chapter",
    };

    store.getState().replaceChapters([duplicate, { ...duplicate, position: 2 }]);

    expect(store.getState().status).toEqual(before);
    expect(store.getState().workspaceError).toBe(
      "导入章节数据无效，请重试。",
    );
  });

  it("resets the current page to manuscript when a project loads", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);

    store.getState().setCurrentPage("story");
    expect(store.getState().currentPage).toBe("story");

    await store.getState().loadProject(client.openProjectResult);
    expect(store.getState().currentPage).toBe("manuscript");
  });

  it("fetches the authoritative workbench projection on project load", async () => {
    const client = FakeStudioClient.withReadyProject();
    const projection: WorkbenchProjection = {
      stage: "writing",
      canvas: "manuscript",
      aiPanel: "hidden",
      pendingReviewCount: 2,
      recoverableJobCount: 1,
      archiveStatus: "not_generated",
    };
    client.workbenchProjectionResult = projection;
    const store = createWorkspaceStore(client);

    await store.getState().loadProject(client.openProjectResult);
    // 等待异步的 refreshWorkbenchForProject 完成
    await vi.waitFor(() => {
      expect(store.getState().workbench).toEqual(projection);
    });
    expect(client.projectWorkbenchCalls).toBeGreaterThanOrEqual(1);
  });

  it("refreshWorkbench re-pulls the projection without reloading the project", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.workbenchProjectionResult = {
      stage: "writing",
      canvas: "manuscript",
      aiPanel: "hidden",
      pendingReviewCount: 0,
      recoverableJobCount: 0,
      archiveStatus: "not_generated",
    };
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    await vi.waitFor(() => {
      expect(store.getState().workbench).not.toBeNull();
    });
    const callsBefore = client.projectWorkbenchCalls;

    const updated: WorkbenchProjection = {
      stage: "writing",
      canvas: "manuscript",
      aiPanel: "hidden",
      pendingReviewCount: 3,
      recoverableJobCount: 0,
      archiveStatus: "not_generated",
    };
    client.workbenchProjectionResult = updated;

    await store.getState().refreshWorkbench();
    expect(client.projectWorkbenchCalls).toBe(callsBefore + 1);
    expect(store.getState().workbench).toEqual(updated);
  });

  it("setCurrentPage only updates local navigation state without contacting the client", async () => {
    const client = FakeStudioClient.withReadyProject();
    const store = createWorkspaceStore(client);
    await store.getState().loadProject(client.openProjectResult);
    const callsBefore = client.projectWorkbenchCalls;

    store.getState().setCurrentPage("versions");
    expect(store.getState().currentPage).toBe("versions");
    expect(client.projectWorkbenchCalls).toBe(callsBefore);
  });

  it("keeps workbench null when projection fetch fails without blocking project load", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.projectWorkbenchError = new Error("projection unavailable");
    const store = createWorkspaceStore(client);

    await store.getState().loadProject(client.openProjectResult);
    await vi.waitFor(() => {
      expect(client.projectWorkbenchCalls).toBeGreaterThanOrEqual(1);
    });

    expect(store.getState().status.kind).toBe("ready");
    expect(store.getState().workbench).toBeNull();
  });
});
