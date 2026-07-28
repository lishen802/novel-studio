import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type { BranchView } from "../../lib/studio-client";
import { BranchWorkbench } from "./BranchWorkbench";

const main: BranchView = {
  id: "main",
  name: "主分支",
  description: "",
  parentId: "",
  active: true,
  main: true,
  generation: 2,
  createdAt: "2026-07-28T06:00:00Z",
  updatedAt: "2026-07-28T06:10:00Z",
};

const child: BranchView = {
  ...main,
  id: "child",
  name: "黑化线",
  description: "主角接受魔契",
  parentId: main.id,
  active: false,
  main: false,
  generation: 3,
};

describe("BranchWorkbench", () => {
  it("creates, switches, compares, merges and deletes complete project branches with confirmation", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.branchesResult = [main, child];
    const beforeOperation = vi.fn(async () => undefined);
    const workspaceChanged = vi.fn();
    render(
      <BranchWorkbench
        client={client}
        onBeforeVersionOperation={beforeOperation}
        onWorkspaceChanged={workspaceChanged}
      />,
    );

    expect(await screen.findByText("黑化线")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "新建剧情分支" }));
    fireEvent.change(screen.getByLabelText("分支名称"), {
      target: { value: "远行线" },
    });
    fireEvent.change(screen.getByLabelText("分支说明"), {
      target: { value: "主角离开宗门" },
    });
    fireEvent.click(screen.getByRole("button", { name: "创建分支" }));
    await waitFor(() =>
      expect(client.createBranchCalls).toEqual([
        { name: "远行线", description: "主角离开宗门" },
      ]),
    );

    fireEvent.click(screen.getByRole("button", { name: "切换到黑化线" }));
    expect(
      await screen.findByRole("dialog", { name: "确认切换剧情分支" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "确认切换" }));
    await waitFor(() => expect(client.switchBranchCalls).toEqual([child.id]));
    expect(beforeOperation).toHaveBeenCalled();
    expect(workspaceChanged).toHaveBeenCalled();
  });

  it("shows a complete-project merge preview before confirmation", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.branchesResult = [main, child];
    const workspaceChanged = vi.fn();
    render(
      <BranchWorkbench
        client={client}
        onBeforeVersionOperation={async () => undefined}
        onWorkspaceChanged={workspaceChanged}
      />,
    );

    expect(await screen.findByText("黑化线")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "预览合并黑化线" }));
    expect(
      await screen.findByRole("dialog", { name: "确认合并剧情分支" }),
    ).toBeInTheDocument();
    expect(screen.getByText("完整项目内容")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "确认合并" }));

    await waitFor(() =>
      expect(client.confirmBranchMergeCalls).toEqual([
        "fake-branch-merge-token",
      ]),
    );
    expect(workspaceChanged).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "删除黑化线" }));
    expect(
      await screen.findByRole("dialog", { name: "确认删除剧情分支" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(client.deleteBranchCalls).toEqual([child.id]));
  });
});
