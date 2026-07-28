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
      baseFingerprint: "",
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

    expect(await screen.findByText("重写第一章开场")).toBeInTheDocument();
    const review = screen.getByLabelText("正文变更对比");
    expect(within(review).getByText("雨一直下。")).toBeInTheDocument();
    expect(
      within(review).getByText("警钟在雨幕中骤然响起。"),
    ).toBeInTheDocument();
    expect(screen.getByText("提升开场张力")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "接受剩余并应用" }),
    );
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
    expect((await screen.findAllByText("已应用")).length).toBeGreaterThanOrEqual(2);
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
      await screen.findByRole("button", { name: "拒绝整组" }),
    );
    await waitFor(() =>
      expect(client.rejectChangeSetCalls).toEqual([pendingChange.id]),
    );
    expect(onChapterUpdated).not.toHaveBeenCalled();
    expect((await screen.findAllByText("已拒绝")).length).toBeGreaterThanOrEqual(2);
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
      await screen.findByRole("button", { name: "接受剩余并应用" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "目标内容已在候选生成后发生变化",
    );
    expect((await screen.findAllByText("存在冲突")).length).toBeGreaterThanOrEqual(2);
    expect(
      screen.queryByRole("button", { name: "接受剩余并应用" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "拒绝整组" }),
    ).toBeInTheDocument();
  });

  it("reviews multiple structured operations and lets the author edit and apply one item", async () => {
    const client = FakeStudioClient.withReadyProject();
    const structured: ChangeSetView = {
      ...pendingChange,
      id: "change-set-structured",
      title: "统一人物与事实",
      operations: [
        {
          id: "operation-character",
          kind: "update_character",
          targetId: "character-1",
          baseRevision: 0,
          baseFingerprint: "character-fingerprint",
          beforeValue: JSON.stringify({
            name: "林默",
            aliases: [],
            role: "protagonist",
            summary: "",
            motivation: "",
            fear: "",
            secret: "",
            arc: "",
            status: "active",
          }),
          afterValue: JSON.stringify({
            name: "林默",
            aliases: ["阿默"],
            role: "protagonist",
            summary: "模型摘要",
            motivation: "",
            fear: "",
            secret: "",
            arc: "",
            status: "active",
          }),
          reason: "补足人物动机",
          status: "pending",
        },
        {
          id: "operation-fact",
          kind: "update_fact",
          targetId: "fact-1",
          baseRevision: 0,
          baseFingerprint: "fact-fingerprint",
          beforeValue: JSON.stringify({
            text: "钟楼在城西",
            category: "location",
            status: "text_evidenced",
            confidence: 0.8,
            validFromEventId: "",
            validUntilEventId: "",
          }),
          afterValue: JSON.stringify({
            text: "钟楼在城北",
            category: "location",
            status: "text_evidenced",
            confidence: 0.95,
            validFromEventId: "",
            validUntilEventId: "",
          }),
          reason: "与第三章保持一致",
          status: "pending",
        },
      ],
    };
    client.changeSetsResult = [structured];
    const user = userEvent.setup();
    render(
      <ChangeReviewWorkbench
        client={client}
        chapters={[client.createChapterResult]}
        onChapterUpdated={vi.fn()}
      />,
    );

    expect(
      await screen.findByRole("button", { name: /1\. 角色档案/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /2\. 故事事实/ }),
    ).toBeInTheDocument();
    const comparison = screen.getByLabelText("结构化字段对比");
    expect(within(comparison).getByText("模型摘要")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "修改候选" }));
    const editor = screen.getByLabelText("修改候选内容");
    const summary = within(editor).getByRole("textbox", { name: "摘要" });
    await user.clear(summary);
    await user.type(summary, "作者修订摘要");
    await user.click(within(editor).getByRole("button", { name: "保存候选" }));
    await waitFor(() =>
      expect(client.editChangeOperationCalls[0]).toEqual(
        expect.objectContaining({
          changeSetId: structured.id,
          operationId: "operation-character",
          character: expect.objectContaining({ summary: "作者修订摘要" }),
        }),
      ),
    );

    await user.click(await screen.findByRole("button", { name: "接受此项" }));
    await waitFor(() =>
      expect(client.applyChangeOperationCalls).toEqual([
        {
          changeSetId: structured.id,
          operationId: "operation-character",
        },
      ]),
    );
    expect(
      (await screen.findAllByText("部分已应用")).length,
    ).toBeGreaterThanOrEqual(1);

    await user.click(screen.getByRole("button", { name: /2\. 故事事实/ }));
    await user.click(screen.getByRole("button", { name: "拒绝此项" }));
    await waitFor(() =>
      expect(client.rejectChangeOperationCalls).toEqual([
        { changeSetId: structured.id, operationId: "operation-fact" },
      ]),
    );
    expect(await screen.findAllByText("已应用")).not.toHaveLength(0);
  });
});
