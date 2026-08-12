import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type { AgentRunView, ChapterView } from "../../lib/studio-client";
import { AgentRunPanel } from "./AgentRunPanel";

function chapter(overrides: Partial<ChapterView> = {}): ChapterView {
  return {
    id: "chapter-1",
    projectId: "project-1",
    title: "第一章",
    position: 1,
    body: "",
    revision: 1,
    updatedAt: "2026-07-29T10:00:00Z",
    hasRecoveryDraft: false,
    ...overrides,
  };
}

function runBase(
  overrides: Partial<AgentRunView> = {},
): Omit<AgentRunView, "id" | "status" | "errorCode" | "retryOf"> {
  return {
    chapterId: "chapter-1",
    prompt: "续写",
    errorMessage: "",
    changeSetId: "",
    tokenBudget: 30000,
    contextCharBudget: 120000,
    inputTokens: 1200,
    outputTokens: 800,
    contextChars: 4200,
    contextTruncated: false,
    steps: [],
    createdAt: "2026-07-30T10:00:00Z",
    updatedAt: "2026-07-30T10:00:00Z",
    ...overrides,
  };
}

describe("AgentRunPanel", () => {
  it("shows the current run budget and task creation form", async () => {
    const client = new FakeStudioClient();
    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    expect(await screen.findByText("章节智能创作")).toBeVisible();
    expect(await screen.findByLabelText("单次任务令牌预算")).toHaveValue(30000);
    expect(screen.getByLabelText("上下文字符预算")).toHaveValue(120000);
    expect(screen.getByLabelText("目标章节")).toBeInTheDocument();
    expect(screen.getByLabelText("创作要求")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "开始智能创作" }),
    ).toBeDisabled();
  });

  it("starts a chapter agent run when the form is submitted", async () => {
    const client = new FakeStudioClient();
    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    await screen.findByText("章节智能创作");
    fireEvent.change(screen.getByLabelText("创作要求"), {
      target: { value: "续写第一章" },
    });
    fireEvent.click(screen.getByRole("button", { name: "开始智能创作" }));

    await waitFor(() =>
      expect(client.createChapterAgentRunCalls).toHaveLength(1),
    );
    expect(client.executeAgentRunCalls).toEqual(["run-1"]);
  });

  it("shows running, completed, and recoverable runs with appropriate actions", async () => {
    const client = new FakeStudioClient();
    client.agentRunsResult = [
      {
        ...runBase(),
        id: "run-running",
        status: "running",
        errorCode: "",
        retryOf: "",
      },
      {
        ...runBase(),
        id: "run-completed",
        status: "completed",
        errorCode: "",
        retryOf: "",
        changeSetId: "change-set-1",
      },
      {
        ...runBase({
          errorCode: "provider_error",
          errorMessage: "模型连接超时",
        }),
        id: "run-recoverable",
        status: "recoverable",
        retryOf: "",
      },
    ];

    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    // 运行中的任务可取消
    expect(await screen.findByRole("button", { name: /取消任务/ })).toBeVisible();
    // 已完成任务可审阅变更
    expect(
      screen.getByRole("button", { name: "审阅变更" }),
    ).toBeVisible();
    // 可恢复任务可重试
    expect(
      screen.getByRole("button", { name: /重试任务/ }),
    ).toBeVisible();
    expect(
      screen.getByText("模型调用失败，请检查连接配置后重试。"),
    ).toBeVisible();
  });

  it("saves the run budget and announces it for new tasks", async () => {
    const client = new FakeStudioClient();
    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    fireEvent.change(await screen.findByLabelText("单次任务令牌预算"), {
      target: { value: "24000" },
    });
    fireEvent.change(screen.getByLabelText("上下文字符预算"), {
      target: { value: "90000" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存运行预算" }));

    await waitFor(() =>
      expect(client.saveAgentRuntimeSettingsCalls).toEqual([
        { maxRunTokens: 24000, maxContextChars: 90000 },
      ]),
    );
    expect(
      screen.getByText("运行预算已保存，新任务将使用此配置"),
    ).toBeVisible();
  });

  it("does not render connection or API key form fields", async () => {
    const client = new FakeStudioClient();
    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    await screen.findByText("章节智能创作");
    expect(screen.queryByLabelText("API Key")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("连接名称")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("接口地址")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "安全保存连接" }),
    ).not.toBeInTheDocument();
  });

  it("cancels a running task and retries a failed task", async () => {
    const client = new FakeStudioClient();
    client.agentRunsResult = [
      {
        ...runBase(),
        id: "run-running",
        status: "running",
        errorCode: "",
        retryOf: "",
      },
      {
        ...runBase({ errorCode: "provider_error", errorMessage: "连接超时" }),
        id: "run-failed",
        status: "failed",
        retryOf: "",
      },
    ];

    render(
      <AgentRunPanel
        client={client}
        chapters={[chapter()]}
        onOpenChanges={() => undefined}
      />,
    );

    fireEvent.click(await screen.findByRole("button", { name: /取消任务/ }));
    await waitFor(() =>
      expect(client.cancelAgentRunCalls).toEqual(["run-running"]),
    );

    // 取消后 run-running 变为 cancelled，也会显示重试按钮，所以使用 getAllByRole
    const retryButtons = screen.getAllByRole("button", { name: /重试任务/ });
    expect(retryButtons.length).toBeGreaterThanOrEqual(1);
    fireEvent.click(retryButtons[retryButtons.length - 1]);
    await waitFor(() =>
      expect(client.retryAgentRunCalls).toEqual(["run-failed"]),
    );
  });
});
