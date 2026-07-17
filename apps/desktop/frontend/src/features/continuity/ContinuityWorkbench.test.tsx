import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { ContinuityWorkbench } from "./ContinuityWorkbench";

function readyClient() {
  const client = FakeStudioClient.withReadyProject();
  client.chaptersResult = [
    {
      ...client.createChapterResult,
      body: "夜色中，雾港禁止御剑。",
      revision: 3,
    },
  ];
  client.continuityResult = {
    events: [
      {
        id: "event-1",
        title: "雾港失火",
        summary: "港口陷入混乱",
        cause: "旧案",
        result: "城门封锁",
        storyTime: "初冬子夜",
        storyOrder: 10,
        narrativeOrder: 1,
        status: "occurred",
      },
    ],
    eventLinks: [],
    facts: [
      {
        id: "fact-1",
        text: "雾港禁止御剑",
        category: "世界规则",
        origin: "model",
        status: "ai_inferred",
        confidence: 0.82,
        validFromEventId: "event-1",
        validUntilEventId: "",
        createdBy: "model",
        confirmedBy: "",
      },
    ],
    evidence: [],
    scopes: [],
    conflicts: [],
    consistencyIssues: [
      {
        code: "MISSING_EVIDENCE",
        severity: "warning",
        message: "事实缺少正文证据。",
        entityIds: ["fact-1"],
      },
    ],
  };
  return client;
}

describe("ContinuityWorkbench", () => {
  let client: FakeStudioClient;
  beforeEach(() => {
    client = readyClient();
  });

  it("shows the dual-order timeline and consistency warnings", async () => {
    render(
      <ContinuityWorkbench client={client} chapters={client.chaptersResult} />,
    );
    expect(
      await screen.findByRole("heading", { name: "时间线与事实" }),
    ).toBeInTheDocument();
    expect(screen.getByText("雾港失火")).toBeInTheDocument();
    expect(screen.getByText(/故事顺序 10/)).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("事实缺少正文证据");
  });

  it("creates a story event with story and narrative order", async () => {
    const user = userEvent.setup();
    render(
      <ContinuityWorkbench client={client} chapters={client.chaptersResult} />,
    );
    await user.click(await screen.findByRole("button", { name: "新建事件" }));
    const dialog = screen.getByRole("dialog", { name: "新建事件" });
    await user.type(within(dialog).getByLabelText("事件标题"), "主角抵达雾港");
    await user.clear(within(dialog).getByLabelText("故事顺序"));
    await user.type(within(dialog).getByLabelText("故事顺序"), "20");
    await user.clear(within(dialog).getByLabelText("叙事顺序"));
    await user.type(within(dialog).getByLabelText("叙事顺序"), "2");
    await user.click(within(dialog).getByRole("button", { name: "保存事件" }));
    await waitFor(() =>
      expect(client.createStoryEventCalls[0]).toMatchObject({
        title: "主角抵达雾港",
        storyOrder: 20,
        narrativeOrder: 2,
      }),
    );
    expect(await screen.findByText("主角抵达雾港")).toBeInTheDocument();
  });

  it("confirms an inferred fact and anchors it to an exact chapter quote", async () => {
    const user = userEvent.setup();
    render(
      <ContinuityWorkbench client={client} chapters={client.chaptersResult} />,
    );
    await user.click(await screen.findByRole("tab", { name: "事实库" }));
    await user.click(
      screen.getByRole("button", { name: "作者确认雾港禁止御剑" }),
    );
    await waitFor(() => expect(client.confirmFactCalls).toEqual(["fact-1"]));
    await user.click(
      screen.getByRole("button", { name: "添加证据雾港禁止御剑" }),
    );
    const dialog = screen.getByRole("dialog", { name: "添加正文证据" });
    await user.selectOptions(
      within(dialog).getByLabelText("章节"),
      "chapter-1",
    );
    await user.type(within(dialog).getByLabelText("原文摘录"), "雾港禁止御剑");
    await user.click(within(dialog).getByRole("button", { name: "保存证据" }));
    await waitFor(() =>
      expect(client.addFactEvidenceCalls[0]).toMatchObject({
        factId: "fact-1",
        chapterId: "chapter-1",
        quote: "雾港禁止御剑",
        startOffset: 4,
        endOffset: 10,
        chapterRevision: 3,
      }),
    );
  });

  it("records and resolves an explicit fact conflict", async () => {
    client.continuityResult.facts.push({
      ...client.continuityResult.facts[0],
      id: "fact-2",
      text: "雾港允许御剑",
      origin: "author",
      status: "author_confirmed",
    });
    const user = userEvent.setup();
    render(
      <ContinuityWorkbench client={client} chapters={client.chaptersResult} />,
    );
    await user.click(await screen.findByRole("tab", { name: "事实库" }));
    await user.click(
      screen.getByRole("button", { name: "标记冲突雾港禁止御剑" }),
    );
    const dialog = screen.getByRole("dialog", { name: "标记事实冲突" });
    await user.selectOptions(
      within(dialog).getByLabelText("冲突事实"),
      "fact-2",
    );
    await user.type(within(dialog).getByLabelText("冲突原因"), "御剑规则互斥");
    await user.click(within(dialog).getByRole("button", { name: "保存冲突" }));
    expect(await screen.findByText("御剑规则互斥")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "解决冲突御剑规则互斥" }),
    );
    await waitFor(() =>
      expect(client.resolveFactConflictCalls).toEqual(["conflict-1"]),
    );
  });
});
