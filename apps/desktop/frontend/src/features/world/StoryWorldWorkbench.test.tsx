import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { StoryWorldWorkbench } from "./StoryWorldWorkbench";

const linMo = {
  id: "character-1",
  name: "林默",
  aliases: ["阿默"],
  role: "protagonist" as const,
  summary: "守夜人调查员",
  motivation: "寻找失踪导师",
  fear: "失去同伴",
  secret: "曾为敌方传信",
  arc: "从孤立到信任",
  status: "active" as const,
};
const guQing = {
  ...linMo,
  id: "character-2",
  name: "顾青",
  aliases: [],
  role: "supporting" as const,
  summary: "港城医生",
  motivation: "保护难民",
  fear: "真相暴露",
  secret: "",
  arc: "",
};
const faction = {
  id: "faction-1",
  name: "守夜人",
  summary: "秘密组织",
  goal: "守护港城",
  status: "active" as const,
};

describe("StoryWorldWorkbench", () => {
  it("renders character profiles, graph edges and consistency warnings", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo, guQing],
      factions: [faction],
      memberships: [
        {
          characterId: linMo.id,
          factionId: faction.id,
          role: "调查员",
          status: "active",
        },
      ],
      relationships: [
        {
          id: "relationship",
          sourceCharacterId: linMo.id,
          targetCharacterId: guQing.id,
          kind: "ally",
          label: "战友",
          strength: 70,
          publicState: "公开合作",
          trueState: "互相信任",
        },
      ],
      chapterAppearances: [
        { characterId: linMo.id, chapterId: "chapter-1", role: "pov" },
      ],
      sceneAppearances: [],
      consistencyIssues: [
        {
          code: "duplicate_character_name",
          severity: "warning",
          message: "存在同名角色",
          entityIds: [linMo.id, guQing.id],
        },
      ],
    };

    render(
      <StoryWorldWorkbench
        client={client}
        chapters={[client.createChapterResult]}
      />,
    );

    expect(
      await screen.findByRole("heading", { name: "人物与势力" }),
    ).toBeInTheDocument();
    expect(screen.getByText("寻找失踪导师")).toBeInTheDocument();
    expect(screen.getByText("战友")).toBeInTheDocument();
    expect(screen.getByText("存在同名角色")).toBeInTheDocument();
    expect(screen.getByText("调查员 · 守夜人")).toBeInTheDocument();
  });

  it("lets the author navigate character profiles from the relationship topology", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      ...client.storyWorldResult,
      characters: [linMo, guQing],
      relationships: [
        {
          id: "relationship",
          sourceCharacterId: linMo.id,
          targetCharacterId: guQing.id,
          kind: "ally",
          label: "战友",
          strength: 70,
          publicState: "",
          trueState: "",
        },
      ],
    };
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    expect(await screen.findByLabelText("人物关系拓扑")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "查看角色：顾青" }));
    expect(screen.getByRole("heading", { name: "顾青" })).toBeInTheDocument();
  });

  it("creates and edits a character", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.click(await screen.findByRole("button", { name: "新建角色" }));
    const createDialog = screen.getByRole("dialog", { name: "新建角色" });
    await user.type(within(createDialog).getByLabelText("角色姓名"), "林默");
    await user.selectOptions(
      within(createDialog).getByLabelText("角色定位"),
      "protagonist",
    );
    await user.type(within(createDialog).getByLabelText("动机"), "寻找导师");
    await user.click(
      within(createDialog).getByRole("button", { name: "保存角色" }),
    );

    expect(client.createCharacterCalls).toEqual([
      expect.objectContaining({
        name: "林默",
        role: "protagonist",
        motivation: "寻找导师",
        status: "active",
      }),
    ]);
    await user.click(await screen.findByRole("button", { name: "编辑：林默" }));
    const editDialog = screen.getByRole("dialog", { name: "编辑角色" });
    const name = within(editDialog).getByLabelText("角色姓名");
    await user.clear(name);
    await user.type(name, "林默之");
    await user.click(
      within(editDialog).getByRole("button", { name: "保存角色" }),
    );
    expect(client.updateCharacterCalls).toEqual([
      expect.objectContaining({
        id: "character-1",
        input: expect.objectContaining({ name: "林默之" }),
      }),
    ]);
  });

  it("creates and edits a faction", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.click(await screen.findByRole("button", { name: "新建势力" }));
    const createDialog = screen.getByRole("dialog", { name: "新建势力" });
    await user.type(within(createDialog).getByLabelText("势力名称"), "守夜人");
    await user.type(
      within(createDialog).getByLabelText("势力目标"),
      "守护港城",
    );
    await user.click(
      within(createDialog).getByRole("button", { name: "保存势力" }),
    );

    expect(client.createFactionCalls).toEqual([
      expect.objectContaining({
        name: "守夜人",
        goal: "守护港城",
        status: "active",
      }),
    ]);
    await user.click(
      await screen.findByRole("button", { name: "编辑势力：守夜人" }),
    );
    const editDialog = screen.getByRole("dialog", { name: "编辑势力" });
    const name = within(editDialog).getByLabelText("势力名称");
    await user.clear(name);
    await user.type(name, "守夜司");
    await user.click(
      within(editDialog).getByRole("button", { name: "保存势力" }),
    );
    expect(client.updateFactionCalls).toEqual([
      expect.objectContaining({
        id: "faction-1",
        input: expect.objectContaining({ name: "守夜司" }),
      }),
    ]);
  });

  it("links a character to a faction, chapter and scene", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo],
      factions: [faction],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    client.storyStructureResult = {
      outlineNodes: [],
      storyLines: [],
      storyLineLinks: [],
      sceneCards: [
        {
          id: "scene-card",
          chapterId: "chapter-1",
          outlineNodeId: "scene-node",
          title: "雨夜来客",
          pov: "",
          location: "港口",
          goal: "",
          conflict: "",
          outcome: "",
          position: 1,
          status: "planned",
        },
      ],
    };
    const user = userEvent.setup();
    render(
      <StoryWorldWorkbench
        client={client}
        chapters={[client.createChapterResult]}
      />,
    );

    await screen.findByRole("heading", { name: "人物与势力" });
    await user.selectOptions(screen.getByLabelText("加入势力"), faction.id);
    await user.type(screen.getByLabelText("势力身份"), "调查员");
    await user.click(screen.getByRole("button", { name: "保存归属" }));
    await user.selectOptions(screen.getByLabelText("关联章节"), "chapter-1");
    await user.click(screen.getByRole("button", { name: "添加章节关联" }));
    await user.selectOptions(screen.getByLabelText("关联场景"), "scene-card");
    await user.click(screen.getByRole("button", { name: "添加场景关联" }));

    expect(client.upsertMembershipCalls).toEqual([
      {
        characterId: linMo.id,
        factionId: faction.id,
        role: "调查员",
        status: "active",
      },
    ]);
    expect(client.linkChapterCalls).toEqual([
      { characterId: linMo.id, targetId: "chapter-1", role: "present" },
    ]);
    expect(client.linkSceneCalls).toEqual([
      { characterId: linMo.id, targetId: "scene-card", role: "present" },
    ]);
  });

  it("requires and trims a faction membership role", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo],
      factions: [faction],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.selectOptions(await screen.findByLabelText("加入势力"), faction.id);
    const save = screen.getByRole("button", { name: "保存归属" });
    expect(save).toBeDisabled();
    expect(screen.getByText("请填写角色在该势力中的身份")).toBeInTheDocument();

    await user.type(screen.getByLabelText("势力身份"), "  调查员  ");
    expect(save).toBeEnabled();
    await user.click(save);
    expect(client.upsertMembershipCalls).toEqual([
      {
        characterId: linMo.id,
        factionId: faction.id,
        role: "调查员",
        status: "active",
      },
    ]);
  });

  it("creates and archives relationships through explicit controls", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo, guQing],
      factions: [],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.click(await screen.findByRole("button", { name: "新建关系" }));
    const dialog = screen.getByRole("dialog", { name: "新建人物关系" });
    await user.selectOptions(
      within(dialog).getByLabelText("关系起点"),
      linMo.id,
    );
    await user.selectOptions(
      within(dialog).getByLabelText("关系终点"),
      guQing.id,
    );
    await user.selectOptions(within(dialog).getByLabelText("关系类型"), "ally");
    await user.type(within(dialog).getByLabelText("关系名称"), "战友");
    await user.click(within(dialog).getByRole("button", { name: "保存关系" }));

    expect(client.createRelationshipCalls).toEqual([
      expect.objectContaining({
        sourceCharacterId: linMo.id,
        targetCharacterId: guQing.id,
        kind: "ally",
        label: "战友",
      }),
    ]);
    await user.click(
      await screen.findByRole("button", { name: "归档关系：林默 → 顾青" }),
    );
    const archiveDialog = screen.getByRole("dialog", { name: "归档人物关系" });
    expect(client.archiveRelationshipCalls).toEqual([]);
    await user.click(within(archiveDialog).getByRole("button", { name: "取消" }));
    expect(client.archiveRelationshipCalls).toEqual([]);
    await user.click(
      screen.getByRole("button", { name: "归档关系：林默 → 顾青" }),
    );
    await user.click(
      within(screen.getByRole("dialog", { name: "归档人物关系" })).getByRole(
        "button",
        { name: "确认归档" },
      ),
    );
    expect(client.archiveRelationshipCalls).toEqual(["relationship-1"]);
  });

  it("archives unreferenced characters and factions through explicit controls", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo],
      factions: [faction],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.click(
      await screen.findByRole("button", { name: "归档角色：林默" }),
    );
    const characterDialog = screen.getByRole("dialog", { name: "归档角色" });
    expect(within(characterDialog).getByText(/林默/)).toBeInTheDocument();
    expect(client.archiveCharacterCalls).toEqual([]);
    await user.click(
      within(characterDialog).getByRole("button", { name: "确认归档" }),
    );
    expect(client.archiveCharacterCalls).toEqual([linMo.id]);
    await user.click(
      await screen.findByRole("button", { name: "归档势力：守夜人" }),
    );
    const factionDialog = screen.getByRole("dialog", { name: "归档势力" });
    expect(within(factionDialog).getByText(/守夜人/)).toBeInTheDocument();
    expect(client.archiveFactionCalls).toEqual([]);
    await user.click(
      within(factionDialog).getByRole("button", { name: "确认归档" }),
    );
    expect(client.archiveFactionCalls).toEqual([faction.id]);
  });

  it("removes faction, chapter and scene links from a character", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo],
      factions: [faction],
      memberships: [
        {
          characterId: linMo.id,
          factionId: faction.id,
          role: "调查员",
          status: "active",
        },
      ],
      relationships: [],
      chapterAppearances: [
        { characterId: linMo.id, chapterId: "chapter-1", role: "present" },
      ],
      sceneAppearances: [
        { characterId: linMo.id, sceneCardId: "scene-card", role: "present" },
      ],
      consistencyIssues: [],
    };
    client.storyStructureResult = {
      outlineNodes: [],
      storyLines: [],
      storyLineLinks: [],
      sceneCards: [
        {
          id: "scene-card",
          chapterId: "chapter-1",
          outlineNodeId: "scene-node",
          title: "雨夜来客",
          pov: "",
          location: "港口",
          goal: "",
          conflict: "",
          outcome: "",
          position: 1,
          status: "planned",
        },
      ],
    };
    const user = userEvent.setup();
    render(
      <StoryWorldWorkbench
        client={client}
        chapters={[client.createChapterResult]}
      />,
    );

    await user.click(
      await screen.findByRole("button", { name: "移除归属：调查员" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "移除章节关联：第一章" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "移除场景关联：雨夜来客" }),
    );

    expect(client.deleteMembershipCalls).toEqual([
      { characterId: linMo.id, factionId: faction.id },
    ]);
    expect(client.unlinkChapterCalls).toEqual([
      { characterId: linMo.id, chapterId: "chapter-1" },
    ]);
    expect(client.unlinkSceneCalls).toEqual([
      { characterId: linMo.id, sceneCardId: "scene-card" },
    ]);
  });

  it("records confirmed knowledge and misinformation without changing facts", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.storyWorldResult = {
      characters: [linMo],
      factions: [],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    client.continuityResult = {
      events: [],
      eventLinks: [],
      facts: [
        {
          id: "fact-1",
          text: "导师已经离开港城",
          category: "人物",
          origin: "author",
          status: "author_confirmed",
          confidence: 1,
          validFromEventId: "",
          validUntilEventId: "",
          createdBy: "author",
          confirmedBy: "author",
        },
      ],
      evidence: [],
      scopes: [],
      conflicts: [],
      consistencyIssues: [],
    };
    const user = userEvent.setup();
    render(<StoryWorldWorkbench client={client} chapters={[]} />);

    await user.selectOptions(await screen.findByLabelText("知识事实"), "fact-1");
    await user.click(screen.getByRole("button", { name: "记录知识" }));
    expect(client.createCharacterKnowledgeCalls).toEqual([
      {
        characterId: linMo.id,
        factId: "fact-1",
        state: "known",
        beliefText: "",
        availableFromChapterId: "",
        notes: "",
      },
    ]);

    await user.selectOptions(screen.getByLabelText("知识状态"), "misinformed");
    await user.type(screen.getByLabelText("误信内容"), "导师仍在港城");
    await user.click(screen.getByRole("button", { name: "记录知识" }));
    expect(client.createCharacterKnowledgeCalls[1]).toEqual({
      characterId: linMo.id,
      factId: "",
      state: "misinformed",
      beliefText: "导师仍在港城",
      availableFromChapterId: "",
      notes: "",
    });
    expect(client.continuityResult.facts).toHaveLength(1);

    await user.click(
      await screen.findByRole("button", {
        name: "移除知识：导师已经离开港城",
      }),
    );
    expect(client.archiveCharacterKnowledgeCalls).toEqual(["knowledge-1"]);
  });
});
