import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import {
  RevisionConflictError,
  type ChangeSetView,
} from "../../lib/studio-client";
import { ChangeReviewWorkbench } from "./ChangeReviewWorkbench";

const pendingChange: ChangeSetView = {
  id: "change-set-1",
  title: "重写第一章开场",
  summary: "让冲突更早出现",
  createdBy: "agent:writer",
  status: "pending",
  operations: [
    {
      id: "operation-1",
      kind: "replace_chapter_body",
      targetId: "chapter-1",
      baseRevision: 1,
      beforeValue: "雨一直下。",
      afterValue: "警钟在雨幕中骤然响起。",
      reason: "提升开场张力",
      status: "pending",
    },
  ],
  createdAt: "2026-07-28T04:00:00Z",
  updatedAt: "2026-07-28T04:00:00Z",
};

describe("ChangeReviewWorkbench", () => {
  it("shows before and after content and applies the selected change", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.openChapterResult = {
      ...client.createChapterResult,
      body: "雨一直下。",
    };
    client.changeSetsResult = [pendingChange];
    const onChapterUpdated = vi.fn();
    const user = userEvent.setup();
    render(
      <ChangeReviewWorkbench
        client={client}
        chapters={[client.openChapterResult]}
        onChapterUpdated={onChapterUpdated}
      />,
    );

    expect(await screen.findAllByText("重写第一章开场")).toHaveLength(2);
    const review = screen.getByLabelText("正文变更对比");
    expect(within(review).getByText("雨一直下。")).toBeInTheDocument();
    expect(
      within(review).getByText("警钟在雨幕中骤然响起。"),
    ).toBeInTheDocument();
    expect(screen.getByText("提升开场张力")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "接受并应用" }));
    await waitFor(() =>
      expect(client.applyChangeSetCalls).toEqual([pendingChange.id]),
    );
    expect(onChapterUpdated).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "chapter-1",
        body: "警钟在雨幕中骤然响起。",
        revision: 2,
      }),
    );
    expect(await screen.findAllByText("已应用")).toHaveLength(2);
  });

  it("rejects a pending change without updating the chapter", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.changeSetsResult = [pendingChange];
    const onChapterUpdated = vi.fn();
    const user = userEvent.setup();
    render(
      <ChangeReviewWorkbench
        client={client}
        chapters={[client.createChapterResult]}
        onChapterUpdated={onChapterUpdated}
      />,
    );

    await user.click(
      await screen.findByRole("button", { name: "拒绝此变更" }),
    );
    await waitFor(() =>
      expect(client.rejectChangeSetCalls).toEqual([pendingChange.id]),
    );
    expect(onChapterUpdated).not.toHaveBeenCalled();
    expect(await screen.findAllByText("已拒绝")).toHaveLength(2);
  });

  it("keeps a stale candidate visible and explains the revision conflict", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.changeSetsResult = [pendingChange];
    client.applyChangeSet = vi.fn(async () => {
      client.changeSetsResult = [
        {
          ...pendingChange,
          status: "conflicted",
          operations: pendingChange.operations.map((item) => ({
            ...item,
            status: "conflicted",
          })),
        },
      ];
      throw new RevisionConflictError();
    });
    const user = userEvent.setup();
    render(
      <ChangeReviewWorkbench
        client={client}
        chapters={[client.createChapterResult]}
        onChapterUpdated={vi.fn()}
      />,
    );

    await user.click(
      await screen.findByRole("button", { name: "接受并应用" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "正文已在候选生成后发生变化",
    );
    expect(await screen.findAllByText("存在冲突")).toHaveLength(2);
    expect(
      screen.queryByRole("button", { name: "接受并应用" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "拒绝此变更" }),
    ).toBeInTheDocument();
  });
});
