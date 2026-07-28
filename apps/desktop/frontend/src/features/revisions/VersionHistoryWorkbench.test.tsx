import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChapterRevisionView,
  ChapterView,
  SnapshotView,
} from "../../lib/studio-client";
import { VersionHistoryWorkbench } from "./VersionHistoryWorkbench";

const chapter: ChapterView = {
  id: "chapter-1",
  projectId: "project-1",
  title: "第一章",
  position: 1,
  body: "当前正文",
  revision: 3,
  updatedAt: "2026-07-28T05:00:00Z",
  hasRecoveryDraft: false,
};

const history: ChapterRevisionView[] = [
  {
    chapterId: chapter.id,
    revision: 3,
    title: chapter.title,
    body: chapter.body,
    source: "author",
    summary: "",
    createdAt: "2026-07-28T05:00:00Z",
    isCurrent: true,
  },
  {
    chapterId: chapter.id,
    revision: 1,
    title: chapter.title,
    body: "初稿正文",
    source: "initial",
    summary: "",
    createdAt: "2026-07-28T04:00:00Z",
    isCurrent: false,
  },
];

const snapshot: SnapshotView = {
  id: "snapshot-1",
  name: "第一幕定稿",
  kind: "named",
  createdAt: "2026-07-28T05:10:00Z",
};

function readyClient() {
  const client = FakeStudioClient.withReadyProject();
  client.chaptersResult = [chapter];
  client.openChapterResult = chapter;
  client.chapterHistoryResult = history;
  client.snapshotsResult = [snapshot];
  return client;
}

describe("VersionHistoryWorkbench", () => {
  it("compares an old revision and restores it only after confirmation", async () => {
    const client = readyClient();
    const onChapterUpdated = vi.fn();
    const user = userEvent.setup();
    render(
      <VersionHistoryWorkbench
        client={client}
        project={client.openProjectResult}
        chapters={[chapter]}
        onChapterUpdated={onChapterUpdated}
        onWorkspaceRestored={vi.fn()}
        onBeforeVersionOperation={vi.fn()}
      />,
    );

    await user.click(
      await screen.findByRole("button", { name: /Revision 1/ }),
    );
    const compare = screen.getByLabelText("章节版本对比");
    expect(within(compare).getByText("初稿正文")).toBeInTheDocument();
    expect(within(compare).getByText("当前正文")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "恢复此版本" }));
    const dialog = screen.getByRole("dialog", { name: "确认恢复章节版本" });
    await user.click(within(dialog).getByRole("button", { name: "确认恢复" }));

    await waitFor(() =>
      expect(client.restoreChapterRevisionCalls).toEqual([
        {
          chapterId: chapter.id,
          targetRevision: 1,
          expectedRevision: 3,
        },
      ]),
    );
    expect(onChapterUpdated).toHaveBeenCalledWith(
      expect.objectContaining({ body: "初稿正文", revision: 4 }),
    );
  });

  it("creates a named whole-project snapshot", async () => {
    const client = readyClient();
    let finishPendingSave: (() => void) | undefined;
    const onBeforeVersionOperation = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finishPendingSave = resolve;
        }),
    );
    const user = userEvent.setup();
    render(
      <VersionHistoryWorkbench
        client={client}
        project={client.openProjectResult}
        chapters={[chapter]}
        onChapterUpdated={vi.fn()}
        onWorkspaceRestored={vi.fn()}
        onBeforeVersionOperation={onBeforeVersionOperation}
      />,
    );

    await user.type(
      screen.getByRole("textbox", { name: "快照名称" }),
      "第二幕完成",
    );
    await user.click(
      screen.getByRole("button", { name: "创建命名快照" }),
    );

    expect(onBeforeVersionOperation).toHaveBeenCalledOnce();
    expect(client.createSnapshotCalls).toEqual([]);
    finishPendingSave?.();
    await waitFor(() =>
      expect(client.createSnapshotCalls).toEqual(["第二幕完成"]),
    );
    expect(await screen.findByText("第二幕完成")).toBeInTheDocument();
  });

  it("restores a whole-project snapshot after destructive-action confirmation", async () => {
    const client = readyClient();
    client.snapshotRestoreResult = {
      snapshotId: snapshot.id,
      project: client.openProjectResult,
      chapters: [{ ...chapter, body: "快照正文", revision: 2 }],
    };
    const onWorkspaceRestored = vi.fn();
    const user = userEvent.setup();
    render(
      <VersionHistoryWorkbench
        client={client}
        project={client.openProjectResult}
        chapters={[chapter]}
        onChapterUpdated={vi.fn()}
        onWorkspaceRestored={onWorkspaceRestored}
        onBeforeVersionOperation={vi.fn()}
      />,
    );

    const snapshotCard = await screen.findByRole("article", {
      name: "快照 第一幕定稿",
    });
    await user.click(
      within(snapshotCard).getByRole("button", { name: "恢复整个项目" }),
    );
    const dialog = screen.getByRole("dialog", { name: "确认恢复项目快照" });
    expect(dialog).toHaveTextContent("自动创建恢复前快照");
    await user.click(
      within(dialog).getByRole("button", { name: "确认恢复整个项目" }),
    );

    await waitFor(() =>
      expect(client.restoreSnapshotCalls).toEqual([snapshot.id]),
    );
    expect(onWorkspaceRestored).toHaveBeenCalledWith(
      client.snapshotRestoreResult,
    );
  });
});
