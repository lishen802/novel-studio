import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type { AgentRunView } from "../../lib/studio-client";
import { AgentWorkbench } from "./AgentWorkbench";

test("configures providers and starts the three-role chapter workflow", async () => {
  const client = new FakeStudioClient();
  client.providerConnectionsResult = [
    {
      id: "provider-1",
      name: "OpenAI",
      kind: "openai",
      baseUrl: "https://api.openai.com/v1",
      hasCredential: true,
      updatedAt: "2026-07-29T10:00:00Z",
    },
  ];
  client.agentProfilesResult = [
    {
      id: "profile-1",
      role: "planner",
      connectionId: "provider-1",
      model: "gpt-5",
      maxOutputTokens: 1000,
      temperature: 0.2,
      enabled: true,
      updatedAt: "2026-07-29T10:00:00Z",
    },
  ];

  render(
    <AgentWorkbench
      client={client}
      chapters={[
        {
          id: "chapter-1",
          projectId: "project-1",
          title: "第一章",
          position: 1,
          body: "",
          revision: 1,
          updatedAt: "2026-07-29T10:00:00Z",
          hasRecoveryDraft: false,
        },
      ]}
      onOpenChanges={() => undefined}
    />,
  );

  expect(await screen.findByText("模型与智能体")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("创作要求"), {
    target: { value: "续写第一章" },
  });
  fireEvent.click(screen.getByRole("button", { name: "开始智能创作" }));

  await waitFor(() => expect(client.createChapterAgentRunCalls).toHaveLength(1));
  expect(client.executeAgentRunCalls).toEqual(["run-1"]);
  expect(await screen.findByText("草稿已进入候选变更")).toBeInTheDocument();
});

test("updates runtime budgets and exposes cancel and retry controls", async () => {
  const client = new FakeStudioClient();
  const runBase: Omit<
    AgentRunView,
    "id" | "status" | "errorCode" | "retryOf"
  > = {
    chapterId: "chapter-1",
    prompt: "续写",
    errorMessage: "",
    changeSetId: "",
    tokenBudget: 30000,
    contextCharBudget: 120000,
    inputTokens: 1200,
    outputTokens: 800,
    contextChars: 4200,
    contextTruncated: true,
    steps: [],
    createdAt: "2026-07-30T10:00:00Z",
    updatedAt: "2026-07-30T10:00:00Z",
  };
  client.agentRunsResult = [
    {
      ...runBase,
      id: "run-running",
      status: "running",
      errorCode: "",
      retryOf: "",
    },
    {
      ...runBase,
      id: "run-failed",
      status: "failed",
      errorCode: "provider_error",
      errorMessage: "模型连接超时",
      retryOf: "",
    },
  ];

  render(
    <AgentWorkbench
      client={client}
      chapters={[
        {
          id: "chapter-1",
          projectId: "project-1",
          title: "第一章",
          position: 1,
          body: "",
          revision: 1,
          updatedAt: "2026-07-30T10:00:00Z",
          hasRecoveryDraft: false,
        },
      ]}
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

  fireEvent.click(screen.getByRole("button", { name: "取消任务" }));
  await waitFor(() =>
    expect(client.cancelAgentRunCalls).toEqual(["run-running"]),
  );

  fireEvent.click(screen.getAllByRole("button", { name: "重试任务" })[1]);
  await waitFor(() => expect(client.retryAgentRunCalls).toEqual(["run-failed"]));
  expect(screen.getAllByText(/上下文已裁剪/)).not.toHaveLength(0);
  expect(
    screen.getByText("模型调用失败，请检查连接配置后重试。"),
  ).toBeInTheDocument();
});
