import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { StoryArchitecture } from "./StoryArchitecture";

describe("StoryArchitecture", () => {
  it("renders the outline and opens a linked chapter", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyStructureResult = {
      outlineNodes: [
        { id: "book", parentId: "", level: "book", title: "全书大纲", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" },
        { id: "chapter-node", parentId: "book", level: "chapter", title: "雨夜来客", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "chapter-1" },
      ],
      storyLines: [{ id: "line", name: "主线", summary: "寻找导师", status: "planned" }],
      storyLineLinks: [{ storyLineId: "line", outlineNodeId: "chapter-node", position: 1 }],
      sceneCards: [],
    };
    const openChapter = vi.fn();
    const user = userEvent.setup();

    render(<StoryArchitecture client={client} chapters={[client.createChapterResult]} onOpenChapter={openChapter} />);

    expect(await screen.findByRole("heading", { name: "故事架构" })).toBeInTheDocument();
    expect(screen.getByText("主线")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "打开章节：雨夜来客" }));
    expect(openChapter).toHaveBeenCalledWith("chapter-1");
  });

  it("creates the root outline and refreshes the structure", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<StoryArchitecture client={client} chapters={[]} onOpenChapter={() => undefined} />);

    await screen.findByRole("heading", { name: "故事架构" });
    await user.click(screen.getByRole("button", { name: "新建大纲节点" }));
    await user.type(screen.getByLabelText("节点标题"), "全书大纲");
    await user.click(screen.getByRole("button", { name: "保存节点" }));

    expect(client.createOutlineNodeCalls).toEqual([
      { parentId: "", level: "book", title: "全书大纲", summary: "", goal: "", conflict: "", outcome: "", chapterId: "" },
    ]);
    expect(await screen.findByText("全书大纲")).toBeInTheDocument();
  });

  it("links a story line to an outline node", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyStructureResult = {
      outlineNodes: [{ id: "book", parentId: "", level: "book", title: "全书大纲", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" }],
      storyLines: [{ id: "line", name: "主线", summary: "寻找导师", status: "planned" }],
      storyLineLinks: [],
      sceneCards: [],
    };
    const user = userEvent.setup();
    render(<StoryArchitecture client={client} chapters={[]} onOpenChapter={() => undefined} />);

    await user.selectOptions(await screen.findByLabelText("主线关联节点"), "book");
    await user.click(screen.getByRole("button", { name: "关联主线" }));

    expect(client.linkStoryLineCalls).toEqual([{ storyLineId: "line", outlineNodeId: "book" }]);
    expect(await screen.findAllByText("全书大纲")).toHaveLength(2);
  });

  it("moves and archives outline nodes through explicit controls", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyStructureResult = {
      outlineNodes: [
        { id: "book", parentId: "", level: "book", title: "全书大纲", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" },
        { id: "volume", parentId: "book", level: "volume", title: "第一卷", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" },
      ],
      storyLines: [], storyLineLinks: [], sceneCards: [],
    };
    const user = userEvent.setup();
    render(<StoryArchitecture client={client} chapters={[]} onOpenChapter={() => undefined} />);

    await user.click(await screen.findByRole("button", { name: "下移：第一卷" }));
    expect(client.moveOutlineNodeCalls).toEqual([{ id: "volume", direction: "down" }]);
    await user.click(screen.getByRole("button", { name: "归档：第一卷" }));
    const dialog = screen.getByRole("dialog", { name: "归档大纲节点" });
    expect(client.archiveOutlineNodeCalls).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "取消" }));
    expect(client.archiveOutlineNodeCalls).toEqual([]);

    await user.click(screen.getByRole("button", { name: "归档：第一卷" }));
    await user.click(within(screen.getByRole("dialog", { name: "归档大纲节点" })).getByRole("button", { name: "确认归档" }));
    expect(client.archiveOutlineNodeCalls).toEqual(["volume"]);
    expect(screen.queryByText("第一卷")).not.toBeInTheDocument();
  });

  it("offers only scene nodes belonging to the selected chapter", async () => {
    const client = FakeStudioClient.withReadyProject();
    const secondChapter = { ...client.createChapterResult, id: "chapter-2", title: "第二章", position: 2 };
    client.storyStructureResult = {
      outlineNodes: [
        { id: "chapter-node-1", parentId: "arc", level: "chapter", title: "第一章规划", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "chapter-1" },
        { id: "scene-1", parentId: "chapter-node-1", level: "scene", title: "雨夜来客", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" },
        { id: "chapter-node-2", parentId: "arc", level: "chapter", title: "第二章规划", summary: "", goal: "", conflict: "", outcome: "", position: 2, chapterId: "chapter-2" },
        { id: "scene-2", parentId: "chapter-node-2", level: "scene", title: "清晨追踪", summary: "", goal: "", conflict: "", outcome: "", position: 1, chapterId: "" },
      ],
      storyLines: [], storyLineLinks: [], sceneCards: [],
    };
    const user = userEvent.setup();
    render(<StoryArchitecture client={client} chapters={[client.createChapterResult, secondChapter]} onOpenChapter={() => undefined} />);

    await user.click(await screen.findByRole("button", { name: "新建场景卡" }));
    await user.selectOptions(screen.getByLabelText("所属章节"), "chapter-1");
    const scenes = screen.getByLabelText("场景节点");
    expect(within(scenes).getByRole("option", { name: "雨夜来客" })).toBeInTheDocument();
    expect(within(scenes).queryByRole("option", { name: "清晨追踪" })).not.toBeInTheDocument();
  });
});
