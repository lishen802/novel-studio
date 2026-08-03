import { describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChapterView,
  SnapshotRestoreView,
} from "../../lib/studio-client";
import { createWorkspaceStore } from "./workspace-store";

describe("workspace store", () => {
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
});
