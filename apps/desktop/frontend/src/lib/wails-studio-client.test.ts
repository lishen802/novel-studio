import { afterEach, describe, expect, it, vi } from "vitest";
import {
  RevisionConflictError,
  StudioClientError,
  type AppearanceInput,
  type CharacterInput,
  type FactionInput,
  type ItemTransferInput,
  type MembershipInput,
  type RelationshipInput,
  type WorldRegionInput,
} from "./studio-client";
import { wailsStudioClient } from "./wails-studio-client";

const project = {
  id: "project-1",
  title: "长夜织境",
  genre: "东方奇幻",
  summary: "",
  path: "/books/长夜织境.zjproject",
};

function installNative(overrides: Record<string, unknown> = {}) {
  const api = {
    ChooseProjectParent: vi.fn(async () => ""),
    ChooseProjectToOpen: vi.fn(async () => ""),
    CreateProject: vi.fn(async () => project),
    OpenProject: vi.fn(async () => project),
    ListChapters: vi.fn(async () => []),
    CreateChapter: vi.fn(),
    OpenChapter: vi.fn(),
    SaveDraft: vi.fn(async () => undefined),
    DiscardDraft: vi.fn(async () => undefined),
    CommitChapter: vi.fn(),
    ChangeSets: vi.fn(async () => []),
    ProposeChapterChange: vi.fn(),
    ApplyChangeSet: vi.fn(),
    RejectChangeSet: vi.fn(async () => undefined),
    StoryStructure: vi.fn(async () => ({
      outlineNodes: [],
      storyLines: [],
      storyLineLinks: [],
      sceneCards: [],
    })),
    CreateOutlineNode: vi.fn(),
    CreateStoryLine: vi.fn(),
    LinkStoryLine: vi.fn(),
    CreateSceneCard: vi.fn(),
    ArchiveOutlineNode: vi.fn(async () => undefined),
    MoveOutlineNode: vi.fn(async () => undefined),
    StoryWorld: vi.fn(async () => ({
      characters: [],
      factions: [],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    })),
    CreateCharacter: vi.fn(),
    UpdateCharacter: vi.fn(),
    ArchiveCharacter: vi.fn(async () => undefined),
    CreateFaction: vi.fn(),
    UpdateFaction: vi.fn(),
    ArchiveFaction: vi.fn(async () => undefined),
    UpsertMembership: vi.fn(),
    DeleteMembership: vi.fn(async () => undefined),
    CreateRelationship: vi.fn(),
    ArchiveRelationship: vi.fn(async () => undefined),
    LinkCharacterToChapter: vi.fn(async () => undefined),
    UnlinkCharacterFromChapter: vi.fn(async () => undefined),
    LinkCharacterToScene: vi.fn(async () => undefined),
    UnlinkCharacterFromScene: vi.fn(async () => undefined),
    WorldAssets: vi.fn(async () => ({
      regions: [],
      regionAdjacencies: [],
      locations: [],
      locationLinks: [],
      items: [],
      itemTransfers: [],
      rules: [],
      hooks: [],
      consistencyIssues: [],
    })),
    CreateWorldRegion: vi.fn(),
    UpdateWorldRegion: vi.fn(),
    ArchiveWorldRegion: vi.fn(async () => undefined),
    UpsertRegionAdjacency: vi.fn(),
    DeleteRegionAdjacency: vi.fn(async () => undefined),
    CreateLocation: vi.fn(),
    UpdateLocation: vi.fn(),
    ArchiveLocation: vi.fn(async () => undefined),
    LinkLocation: vi.fn(),
    UnlinkLocation: vi.fn(async () => undefined),
    CreateItem: vi.fn(),
    UpdateItem: vi.fn(),
    TransferItem: vi.fn(),
    ArchiveItem: vi.fn(async () => undefined),
    CreateWorldRule: vi.fn(),
    UpdateWorldRule: vi.fn(),
    ArchiveWorldRule: vi.fn(async () => undefined),
    CreateForeshadowingHook: vi.fn(),
    UpdateForeshadowingHook: vi.fn(),
    ArchiveForeshadowingHook: vi.fn(async () => undefined),
    Continuity: vi.fn(async () => ({
      events: [],
      eventLinks: [],
      facts: [],
      evidence: [],
      scopes: [],
      conflicts: [],
      consistencyIssues: [],
    })),
    CreateStoryEvent: vi.fn(),
    UpdateStoryEvent: vi.fn(),
    ArchiveStoryEvent: vi.fn(async () => undefined),
    LinkStoryEvent: vi.fn(),
    UnlinkStoryEvent: vi.fn(async () => undefined),
    CreateFact: vi.fn(),
    UpdateFact: vi.fn(),
    ConfirmFact: vi.fn(),
    ArchiveFact: vi.fn(async () => undefined),
    CharacterKnowledge: vi.fn(async () => []),
    CreateCharacterKnowledge: vi.fn(),
    ArchiveCharacterKnowledge: vi.fn(async () => undefined),
    AddFactEvidence: vi.fn(),
    RemoveFactEvidence: vi.fn(async () => undefined),
    AddFactScope: vi.fn(),
    RemoveFactScope: vi.fn(async () => undefined),
    CreateFactConflict: vi.fn(),
    ResolveFactConflict: vi.fn(),
    DeleteFactConflict: vi.fn(async () => undefined),
    CompleteClose: vi.fn(async () => undefined),
    ...overrides,
  };
  window.go = { main: { DesktopAPI: api } } as typeof window.go;
  return api;
}

afterEach(() => {
  delete window.go;
  delete window.runtime;
});

describe("wailsStudioClient", () => {
  it("maps an empty native directory selection to cancellation", async () => {
    installNative();

    await expect(wailsStudioClient.chooseProjectParent()).resolves.toBeNull();
  });

  it("forwards create input and validates the returned project DTO", async () => {
    const api = installNative();
    const input = {
      parentDirectory: "/books",
      title: "长夜织境",
      genre: "东方奇幻",
      summary: "",
    };

    await expect(wailsStudioClient.createProject(input)).resolves.toEqual(
      project,
    );
    expect(api.CreateProject).toHaveBeenCalledWith(input);
  });

  it("rejects an invalid native DTO", async () => {
    installNative({
      CreateProject: vi.fn(async () => ({ title: "missing fields" })),
    });

    await expect(
      wailsStudioClient.createProject({
        parentDirectory: "/books",
        title: "长夜织境",
        genre: "",
        summary: "",
      }),
    ).rejects.toThrow();
  });

  it("maps a native revision error to a typed conflict", async () => {
    installNative({
      CommitChapter: vi.fn(async () => {
        throw new Error("commit chapter: revision conflict");
      }),
    });

    await expect(
      wailsStudioClient.commitChapter("chapter-1", "正文", 1),
    ).rejects.toBeInstanceOf(RevisionConflictError);
  });

  it("forwards explicit recovery draft deletion", async () => {
    const api = installNative();

    await wailsStudioClient.discardDraft("chapter-1", 4);

    expect(api.DiscardDraft).toHaveBeenCalledWith("chapter-1", 4);
  });

  it("validates the story world graph and forwards character creation", async () => {
    const character = {
      id: "character-1",
      name: "林默",
      aliases: ["阿默"],
      role: "protagonist",
      summary: "调查员",
      motivation: "寻找导师",
      fear: "失去同伴",
      secret: "",
      arc: "学会信任",
      status: "active",
    };
    const world = {
      characters: [character],
      factions: [],
      memberships: [],
      relationships: [],
      chapterAppearances: [],
      sceneAppearances: [],
      consistencyIssues: [],
    };
    const api = installNative({
      StoryWorld: vi.fn(async () => world),
      CreateCharacter: vi.fn(async () => character),
    });
    const input: CharacterInput = {
      name: "林默",
      aliases: ["阿默"],
      role: "protagonist",
      summary: "调查员",
      motivation: "寻找导师",
      fear: "失去同伴",
      secret: "",
      arc: "学会信任",
      status: "active",
    };

    await expect(wailsStudioClient.storyWorld()).resolves.toEqual(world);
    await expect(wailsStudioClient.createCharacter(input)).resolves.toEqual(
      character,
    );
    expect(api.CreateCharacter).toHaveBeenCalledWith(input);
  });

  it("forwards story world mutation commands", async () => {
    const api = installNative({
      UpdateCharacter: vi.fn(async () => ({
        id: "character",
        name: "林默",
        aliases: [],
        role: "protagonist",
        summary: "",
        motivation: "",
        fear: "",
        secret: "",
        arc: "",
        status: "active",
      })),
      ArchiveCharacter: vi.fn(async () => undefined),
      CreateFaction: vi.fn(async () => ({
        id: "faction",
        name: "守夜人",
        summary: "",
        goal: "",
        status: "active",
      })),
      UpdateFaction: vi.fn(async () => ({
        id: "faction",
        name: "守夜司",
        summary: "",
        goal: "",
        status: "inactive",
      })),
      ArchiveFaction: vi.fn(async () => undefined),
      UpsertMembership: vi.fn(async () => ({
        characterId: "character",
        factionId: "faction",
        role: "调查员",
        status: "active",
      })),
      DeleteMembership: vi.fn(async () => undefined),
      CreateRelationship: vi.fn(async () => ({
        id: "relationship",
        sourceCharacterId: "character",
        targetCharacterId: "other",
        kind: "ally",
        label: "战友",
        strength: 70,
        publicState: "",
        trueState: "",
      })),
      ArchiveRelationship: vi.fn(async () => undefined),
      LinkCharacterToChapter: vi.fn(async () => undefined),
      UnlinkCharacterFromChapter: vi.fn(async () => undefined),
      LinkCharacterToScene: vi.fn(async () => undefined),
      UnlinkCharacterFromScene: vi.fn(async () => undefined),
    });
    const character: CharacterInput = {
      name: "林默",
      aliases: [],
      role: "protagonist",
      summary: "",
      motivation: "",
      fear: "",
      secret: "",
      arc: "",
      status: "active",
    };
    const faction: FactionInput = {
      name: "守夜司",
      summary: "",
      goal: "",
      status: "inactive",
    };
    const membership: MembershipInput = {
      characterId: "character",
      factionId: "faction",
      role: "调查员",
      status: "active",
    };
    const relationship: RelationshipInput = {
      sourceCharacterId: "character",
      targetCharacterId: "other",
      kind: "ally",
      label: "战友",
      strength: 70,
      publicState: "",
      trueState: "",
    };
    const chapterAppearance: AppearanceInput = {
      characterId: "character",
      targetId: "chapter",
      role: "pov",
    };
    const sceneAppearance: AppearanceInput = {
      characterId: "character",
      targetId: "scene",
      role: "present",
    };

    await wailsStudioClient.updateCharacter("character", character);
    await wailsStudioClient.archiveCharacter("character");
    await wailsStudioClient.createFaction(faction);
    await wailsStudioClient.updateFaction("faction", faction);
    await wailsStudioClient.archiveFaction("faction");
    await wailsStudioClient.upsertMembership(membership);
    await wailsStudioClient.deleteMembership("character", "faction");
    await wailsStudioClient.createRelationship(relationship);
    await wailsStudioClient.archiveRelationship("relationship");
    await wailsStudioClient.linkCharacterToChapter(chapterAppearance);
    await wailsStudioClient.unlinkCharacterFromChapter("character", "chapter");
    await wailsStudioClient.linkCharacterToScene(sceneAppearance);
    await wailsStudioClient.unlinkCharacterFromScene("character", "scene");

    expect(api.UpdateCharacter).toHaveBeenCalledWith("character", character);
    expect(api.DeleteMembership).toHaveBeenCalledWith("character", "faction");
    expect(api.LinkCharacterToScene).toHaveBeenCalledWith(sceneAppearance);
  });

  it("validates world assets and forwards structured mutations", async () => {
    const region = {
      id: "region-root",
      parentId: "",
      kind: "world",
      name: "镜海世界",
      direction: "center",
      summary: "",
      terrain: "",
      climate: "",
    };
    const assets = {
      regions: [region],
      regionAdjacencies: [],
      locations: [],
      locationLinks: [],
      items: [],
      itemTransfers: [],
      rules: [],
      hooks: [],
      consistencyIssues: [],
    };
    const api = installNative({
      WorldAssets: vi.fn(async () => assets),
      CreateWorldRegion: vi.fn(async () => region),
      TransferItem: vi.fn(async () => ({
        id: "transfer-1",
        itemId: "item-1",
        fromType: "",
        fromId: "",
        toType: "location",
        toId: "location-1",
        chapterId: "chapter-1",
        note: "存放",
      })),
    });
    const regionInput: WorldRegionInput = {
      parentId: "",
      kind: "world",
      name: "镜海世界",
      direction: "center",
      summary: "",
      terrain: "",
      climate: "",
    };
    const transferInput: ItemTransferInput = {
      itemId: "item-1",
      holderType: "location",
      holderId: "location-1",
      chapterId: "chapter-1",
      note: "存放",
    };

    await expect(wailsStudioClient.worldAssets()).resolves.toEqual(assets);
    await wailsStudioClient.createWorldRegion(regionInput);
    await wailsStudioClient.transferItem(transferInput);

    expect(api.CreateWorldRegion).toHaveBeenCalledWith(regionInput);
    expect(api.TransferItem).toHaveBeenCalledWith(transferInput);
  });

  it("validates continuity data and forwards timeline and fact commands", async () => {
    const event = {
      id: "event-1",
      title: "雾港失火",
      summary: "",
      cause: "",
      result: "",
      storyTime: "初冬",
      storyOrder: 10,
      narrativeOrder: 1,
      status: "occurred",
    };
    const fact = {
      id: "fact-1",
      text: "雾港禁飞",
      category: "规则",
      origin: "model",
      status: "ai_inferred",
      confidence: 0.8,
      validFromEventId: "event-1",
      validUntilEventId: "",
      createdBy: "model",
      confirmedBy: "",
    };
    const continuity = {
      events: [event],
      eventLinks: [],
      facts: [fact],
      evidence: [],
      scopes: [],
      conflicts: [],
      consistencyIssues: [],
    };
    const api = installNative({
      Continuity: vi.fn(async () => continuity),
      CreateStoryEvent: vi.fn(async () => event),
      ConfirmFact: vi.fn(async () => ({
        ...fact,
        status: "author_confirmed",
        confirmedBy: "author",
      })),
    });
    const eventInput = {
      title: event.title,
      summary: "",
      cause: "",
      result: "",
      storyTime: event.storyTime,
      storyOrder: 10,
      narrativeOrder: 1,
      status: "occurred" as const,
    };

    await expect(wailsStudioClient.continuity()).resolves.toEqual(continuity);
    await wailsStudioClient.createStoryEvent(eventInput);
    await wailsStudioClient.confirmFact("fact-1");

    expect(api.CreateStoryEvent).toHaveBeenCalledWith(eventInput);
    expect(api.ConfirmFact).toHaveBeenCalledWith("fact-1");
  });

  it("validates reviewable change sets and forwards author decisions", async () => {
    const changeSet = {
      id: "change-set-1",
      title: "重写开场",
      summary: "加强冲突",
      createdBy: "agent:writer",
      status: "pending",
      operations: [
        {
          id: "operation-1",
          kind: "replace_chapter_body",
          targetId: "chapter-1",
          baseRevision: 1,
          beforeValue: "旧正文",
          afterValue: "新正文",
          reason: "更早引出危机",
          status: "pending",
        },
      ],
      createdAt: "2026-07-28T04:00:00Z",
      updatedAt: "2026-07-28T04:00:00Z",
    };
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "新正文",
      revision: 2,
      updatedAt: "2026-07-28T04:01:00Z",
      hasRecoveryDraft: false,
    };
    const api = installNative({
      ChangeSets: vi.fn(async () => [changeSet]),
      ApplyChangeSet: vi.fn(async () => chapter),
    });

    await expect(wailsStudioClient.changeSets()).resolves.toEqual([changeSet]);
    await expect(wailsStudioClient.applyChangeSet(changeSet.id)).resolves.toEqual(
      chapter,
    );
    await wailsStudioClient.rejectChangeSet(changeSet.id);

    expect(api.ApplyChangeSet).toHaveBeenCalledWith(changeSet.id);
    expect(api.RejectChangeSet).toHaveBeenCalledWith(changeSet.id);
  });

  it("registers the native close event and completes the close handshake", async () => {
    const api = installNative();
    let beforeClose: (() => void) | undefined;
    const cancel = vi.fn();
    window.runtime = {
      EventsOnMultiple: vi.fn((_event, callback) => {
        beforeClose = callback as () => void;
        return cancel;
      }),
    };
    const handler = vi.fn(async () => undefined);

    const unsubscribe = wailsStudioClient.onBeforeClose(handler);
    beforeClose?.();
    await Promise.resolve();
    await wailsStudioClient.completeClose();
    unsubscribe();

    expect(window.runtime.EventsOnMultiple).toHaveBeenCalledWith(
      "app:before-close",
      expect.any(Function),
      -1,
    );
    expect(handler).toHaveBeenCalledOnce();
    expect(api.CompleteClose).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("shows only the safe message from a coded desktop error", async () => {
    installNative({
      OpenProject: vi.fn(async () => {
        throw new Error("PROJECT_CORRUPT|项目格式无效或数据已损坏。");
      }),
    });

    const operation = wailsStudioClient.openProject("/books/broken.zjproject");
    await expect(operation).rejects.toBeInstanceOf(StudioClientError);
    await expect(operation).rejects.toMatchObject({
      code: "PROJECT_CORRUPT",
      message: "项目格式无效或数据已损坏。",
    });
  });
});
