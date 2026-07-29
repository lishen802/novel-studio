import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { FakeStudioClient } from "../../lib/fake-studio-client";
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
