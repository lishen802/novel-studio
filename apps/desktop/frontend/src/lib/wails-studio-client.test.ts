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

function nativeImportJob(overrides: Record<string, unknown> = {}) {
  return {
    id: "import-1",
    status: "prepared",
    sourceName: "长夜.md",
    sourceFormat: "markdown",
    sourceEncoding: "UTF-8",
    sourceHashDigest: "0123456789ab",
    sourceBytes: 2048,
    archivedRelativePath: "imports/import-1.md",
    chapterCount: 1,
    selectedChapterCount: 0,
    errorCode: "",
    errorMessage: "",
    createdAt: "2026-08-03T08:00:00Z",
    updatedAt: "2026-08-03T08:00:00Z",
    appliedAt: "",
    chapters: [
      {
        id: "candidate-1",
        volumeTitle: "第一卷",
        title: "第一章 雾港",
        wordCount: 9,
        bodyPreview: "潮声越过防波堤。",
        position: 1,
        sourceStartLine: 3,
        sourceEndLine: 5,
        contentHashSummary: "abcdef012345",
        selected: false,
        createdChapterId: "",
      },
    ],
    ...overrides,
  };
}

function installNative(overrides: Record<string, unknown> = {}) {
  const api = {
    ChooseProjectParent: vi.fn(async () => ""),
    ChooseProjectToOpen: vi.fn(async () => ""),
    ChooseManuscriptToImport: vi.fn(async () => ""),
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
    ProposeStructuredChange: vi.fn(),
    EditChangeOperation: vi.fn(),
    ApplyChangeSet: vi.fn(),
    ApplyChangeOperation: vi.fn(),
    RejectChangeSet: vi.fn(async () => undefined),
    RejectChangeOperation: vi.fn(async () => undefined),
    ChapterHistory: vi.fn(async () => []),
    RestoreChapterRevision: vi.fn(),
    Snapshots: vi.fn(async () => []),
    CreateSnapshot: vi.fn(),
    RestoreSnapshot: vi.fn(),
    Branches: vi.fn(async () => []),
    CreateBranch: vi.fn(),
    SwitchBranch: vi.fn(),
    CompareBranches: vi.fn(),
    PreviewBranchMerge: vi.fn(),
    ConfirmBranchMerge: vi.fn(),
    DeleteBranch: vi.fn(async () => undefined),
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
    ProviderConnections: vi.fn(async () => []),
    SaveProviderConnection: vi.fn(),
    DeleteProviderConnection: vi.fn(async () => undefined),
    TestProviderConnection: vi.fn(async () => undefined),
    AgentProfiles: vi.fn(async () => []),
    SaveAgentProfile: vi.fn(),
    AgentRuntimeSettings: vi.fn(async () => ({
      maxRunTokens: 30000,
      maxContextChars: 120000,
      updatedAt: "2026-07-30T10:00:00Z",
    })),
    SaveAgentRuntimeSettings: vi.fn(),
    CreateChapterAgentRun: vi.fn(),
    ExecuteAgentRun: vi.fn(),
    CancelAgentRun: vi.fn(async () => undefined),
    RetryAgentRun: vi.fn(),
    AgentRuns: vi.fn(async () => []),
    AgentRun: vi.fn(),
    GetJobSnapshot: vi.fn(),
    GetJobEvents: vi.fn(async () => ({
      events: [],
      nextCursor: 0,
      hasMore: false,
      snapshotRequired: false,
    })),
    ListRecoverableJobs: vi.fn(async () => []),
    PrepareManuscriptImport: vi.fn(),
    ImportJobs: vi.fn(async () => []),
    ImportJob: vi.fn(),
    DeleteImportJob: vi.fn(async () => undefined),
    ApplyImportJob: vi.fn(),
    ExtractionTasks: vi.fn(async () => []),
    ExtractionTask: vi.fn(),
    StartExtractionRun: vi.fn(),
    ExecuteExtractionRun: vi.fn(),
    CancelExtractionRun: vi.fn(async () => undefined),
    RetryExtractionRun: vi.fn(),
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

  it("validates and forwards manuscript import operations without exposing archive paths", async () => {
    const job = nativeImportJob();
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章 雾港",
      position: 1,
      body: "潮声越过防波堤。",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const api = installNative({
      ChooseManuscriptToImport: vi.fn(async () => "/Users/author/长夜.md"),
      PrepareManuscriptImport: vi.fn(async () => ({ job })),
      ImportJobs: vi.fn(async () => [job]),
      ImportJob: vi.fn(async () => job),
      ApplyImportJob: vi.fn(async () => ({
        job: {
          ...job,
          status: "applied",
          selectedChapterCount: 1,
          appliedAt: "2026-08-03T08:01:00Z",
          chapters: job.chapters.map((candidate) => ({
            ...candidate,
            selected: true,
            createdChapterId: chapter.id,
          })),
        },
        chapters: [chapter],
      })),
    });

    await expect(wailsStudioClient.chooseManuscriptToImport()).resolves.toBe(
      "/Users/author/长夜.md",
    );
    const prepared = await wailsStudioClient.prepareManuscriptImport(
      "/Users/author/长夜.md",
    );
    await expect(wailsStudioClient.importJobs()).resolves.toHaveLength(1);
    await expect(wailsStudioClient.importJob(job.id)).resolves.toMatchObject({
      id: job.id,
      chapters: [
        {
          sourceStartLine: 3,
          sourceEndLine: 5,
          wordCount: 9,
          bodyPreview: "潮声越过防波堤。",
        },
      ],
    });
    await wailsStudioClient.deleteImportJob(job.id);
    await expect(
      wailsStudioClient.applyImportJob(job.id, ["candidate-1"]),
    ).resolves.toMatchObject({ chapters: [chapter] });

    expect(prepared.job).not.toHaveProperty("archivedRelativePath");
    expect(api.PrepareManuscriptImport).toHaveBeenCalledWith(
      "/Users/author/长夜.md",
    );
    expect(api.ApplyImportJob).toHaveBeenCalledWith(job.id, ["candidate-1"]);
    expect(api.DeleteImportJob).toHaveBeenCalledWith(job.id);
  });

  it("rejects malformed manuscript hashes from the native boundary", async () => {
    installNative({
      ImportJobs: vi.fn(async () => [
        {
          id: "import-1",
          status: "prepared",
          sourceName: "长夜.md",
          sourceFormat: "markdown",
          sourceEncoding: "UTF-8",
          sourceHashDigest: "/Users/author/private.md",
          sourceBytes: 2048,
          archivedRelativePath: "imports/import-1.md",
          chapterCount: 0,
          selectedChapterCount: 0,
          errorCode: "",
          errorMessage: "",
          createdAt: "2026-08-03T08:00:00Z",
          updatedAt: "2026-08-03T08:00:00Z",
          appliedAt: "",
          chapters: [],
        },
      ]),
    });

    await expect(wailsStudioClient.importJobs()).rejects.toThrow();
  });

  it("accepts lightweight prepared and failed task summaries without candidates", async () => {
    installNative({
      ImportJobs: vi.fn(async () => [
        nativeImportJob({ chapters: [] }),
        nativeImportJob({
          id: "import-failed",
          status: "failed",
          chapterCount: 0,
          selectedChapterCount: 0,
          errorCode: "IMPORT_PARSE_EMPTY",
          errorMessage: "No importable chapters were found.",
          chapters: [],
        }),
      ]),
    });

    await expect(wailsStudioClient.importJobs()).resolves.toMatchObject([
      { id: "import-1", status: "prepared", chapters: [] },
      { id: "import-failed", status: "failed", chapters: [] },
    ]);
  });

  it.each([
    ["source path", { sourceName: "/Users/author/长夜.md" }],
    [
      "reversed source lines",
      {
        chapters: [
          {
            ...nativeImportJob().chapters[0],
            sourceStartLine: 5,
            sourceEndLine: 3,
          },
        ],
      },
    ],
    [
      "duplicate candidate IDs",
      {
        chapterCount: 2,
        chapters: [
          nativeImportJob().chapters[0],
          {
            ...nativeImportJob().chapters[0],
            position: 2,
          },
        ],
      },
    ],
    [
      "duplicate candidate positions",
      {
        chapterCount: 2,
        chapters: [
          nativeImportJob().chapters[0],
          {
            ...nativeImportJob().chapters[0],
            id: "candidate-2",
          },
        ],
      },
    ],
    ["chapter count mismatch", { chapterCount: 2 }],
    ["prepared selection count", { selectedChapterCount: 1 }],
    [
      "full body crossing the boundary",
      {
        chapters: [
          {
            ...nativeImportJob().chapters[0],
            body: "不应跨过 Wails 边界的完整正文",
          },
        ],
      },
    ],
    [
      "oversized body preview",
      {
        chapters: [
          {
            ...nativeImportJob().chapters[0],
            bodyPreview: "长".repeat(601),
          },
        ],
      },
    ],
  ])("rejects malformed import job invariant: %s", async (_name, override) => {
    installNative({
      ImportJob: vi.fn(async () => nativeImportJob(override)),
    });

    await expect(wailsStudioClient.importJob("import-1")).rejects.toThrow();
  });

  it.each([
    ["empty chapter result", []],
    [
      "duplicate chapter IDs",
      [
        {
          id: "chapter-1",
          projectId: "project-1",
          title: "第一章",
          position: 1,
          body: "正文",
          revision: 1,
          updatedAt: "2026-08-03T08:01:00Z",
          hasRecoveryDraft: false,
        },
        {
          id: "chapter-1",
          projectId: "project-1",
          title: "第二章",
          position: 2,
          body: "正文",
          revision: 1,
          updatedAt: "2026-08-03T08:01:00Z",
          hasRecoveryDraft: false,
        },
      ],
    ],
    [
      "mixed project IDs",
      [
        {
          id: "chapter-1",
          projectId: "project-1",
          title: "第一章",
          position: 1,
          body: "正文",
          revision: 1,
          updatedAt: "2026-08-03T08:01:00Z",
          hasRecoveryDraft: false,
        },
        {
          id: "chapter-2",
          projectId: "project-2",
          title: "第二章",
          position: 2,
          body: "正文",
          revision: 1,
          updatedAt: "2026-08-03T08:01:00Z",
          hasRecoveryDraft: false,
        },
      ],
    ],
  ])("rejects malformed import apply invariant: %s", async (_name, chapters) => {
    const candidate = {
      ...nativeImportJob().chapters[0],
      selected: true,
      createdChapterId: "chapter-1",
    };
    installNative({
      ApplyImportJob: vi.fn(async () => ({
        job: nativeImportJob({
          status: "applied",
          selectedChapterCount: 1,
          appliedAt: "2026-08-03T08:01:00Z",
          chapters: [candidate],
        }),
        chapters,
      })),
    });

    await expect(
      wailsStudioClient.applyImportJob("import-1", ["candidate-1"]),
    ).rejects.toThrow();
  });

  it("rejects a prepared job returned from apply", async () => {
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "正文",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    installNative({
      ApplyImportJob: vi.fn(async () => ({
        job: nativeImportJob(),
        chapters: [chapter],
      })),
    });

    await expect(
      wailsStudioClient.applyImportJob("import-1", ["candidate-1"]),
    ).rejects.toThrow();
  });

  it("rejects an applied job whose ID does not match the request", async () => {
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "正文",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const candidate = {
      ...nativeImportJob().chapters[0],
      selected: true,
      createdChapterId: chapter.id,
    };
    installNative({
      ApplyImportJob: vi.fn(async () => ({
        job: nativeImportJob({
          id: "import-other",
          status: "applied",
          selectedChapterCount: 1,
          appliedAt: "2026-08-03T08:01:00Z",
          chapters: [candidate],
        }),
        chapters: [chapter],
      })),
    });

    await expect(
      wailsStudioClient.applyImportJob("import-1", ["candidate-1"]),
    ).rejects.toThrow();
  });

  it("rejects duplicate created chapter mappings from apply", async () => {
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "正文",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const first = {
      ...nativeImportJob().chapters[0],
      selected: true,
      createdChapterId: chapter.id,
    };
    const second = {
      ...first,
      id: "candidate-2",
      position: 2,
    };
    installNative({
      ApplyImportJob: vi.fn(async () => ({
        job: nativeImportJob({
          status: "applied",
          chapterCount: 2,
          selectedChapterCount: 2,
          appliedAt: "2026-08-03T08:01:00Z",
          chapters: [first, second],
        }),
        chapters: [chapter],
      })),
    });

    await expect(
      wailsStudioClient.applyImportJob(
        "import-1",
        ["candidate-1", "candidate-2"],
      ),
    ).rejects.toThrow();
  });

  it("rejects applied candidates that do not match the request", async () => {
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "正文",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const candidate = {
      ...nativeImportJob().chapters[0],
      id: "candidate-other",
      selected: true,
      createdChapterId: chapter.id,
    };
    installNative({
      ApplyImportJob: vi.fn(async () => ({
        job: nativeImportJob({
          status: "applied",
          selectedChapterCount: 1,
          appliedAt: "2026-08-03T08:01:00Z",
          chapters: [candidate],
        }),
        chapters: [chapter],
      })),
    });

    await expect(
      wailsStudioClient.applyImportJob("import-1", ["candidate-1"]),
    ).rejects.toThrow();
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
          baseFingerprint: "",
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
      ApplyChangeSet: vi.fn(async () => ({
        changeSet: { ...changeSet, status: "applied" },
        chapters: [chapter],
      })),
    });

    await expect(wailsStudioClient.changeSets()).resolves.toEqual([changeSet]);
    await expect(wailsStudioClient.applyChangeSet(changeSet.id)).resolves.toEqual(
      {
        changeSet: { ...changeSet, status: "applied" },
        chapters: [chapter],
      },
    );
    await wailsStudioClient.rejectChangeSet(changeSet.id);

    expect(api.ApplyChangeSet).toHaveBeenCalledWith(changeSet.id);
    expect(api.RejectChangeSet).toHaveBeenCalledWith(changeSet.id);
  });

  it("validates version history and forwards snapshot recovery operations", async () => {
    const history = [
      {
        chapterId: "chapter-1",
        revision: 2,
        title: "第一章",
        body: "当前正文",
        source: "author",
        summary: "",
        createdAt: "2026-07-28T05:00:00Z",
        isCurrent: true,
      },
      {
        chapterId: "chapter-1",
        revision: 1,
        title: "第一章",
        body: "初稿",
        source: "initial",
        summary: "",
        createdAt: "2026-07-28T04:00:00Z",
        isCurrent: false,
      },
    ];
    const snapshot = {
      id: "snapshot-1",
      name: "第一幕定稿",
      kind: "named",
      createdAt: "2026-07-28T05:10:00Z",
    };
    const chapter = {
      id: "chapter-1",
      projectId: "project-1",
      title: "第一章",
      position: 1,
      body: "初稿",
      revision: 3,
      updatedAt: "2026-07-28T05:20:00Z",
      hasRecoveryDraft: false,
    };
    const restore = {
      snapshotId: snapshot.id,
      project,
      chapters: [chapter],
    };
    const api = installNative({
      ChapterHistory: vi.fn(async () => history),
      RestoreChapterRevision: vi.fn(async () => chapter),
      Snapshots: vi.fn(async () => [snapshot]),
      CreateSnapshot: vi.fn(async () => snapshot),
      RestoreSnapshot: vi.fn(async () => restore),
    });

    await expect(wailsStudioClient.chapterHistory(chapter.id)).resolves.toEqual(
      history,
    );
    await expect(
      wailsStudioClient.restoreChapterRevision(chapter.id, 1, 2),
    ).resolves.toEqual(chapter);
    await expect(wailsStudioClient.snapshots()).resolves.toEqual([snapshot]);
    await expect(
      wailsStudioClient.createSnapshot(snapshot.name),
    ).resolves.toEqual(snapshot);
    await expect(
      wailsStudioClient.restoreSnapshot(snapshot.id),
    ).resolves.toEqual(restore);

    expect(api.RestoreChapterRevision).toHaveBeenCalledWith(chapter.id, 1, 2);
    expect(api.CreateSnapshot).toHaveBeenCalledWith(snapshot.name);
    expect(api.RestoreSnapshot).toHaveBeenCalledWith(snapshot.id);
  });

  it("validates and forwards complete project branch operations", async () => {
    const main = {
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
    const child = {
      ...main,
      id: "child",
      name: "黑化线",
      description: "主角接受魔契",
      parentId: main.id,
      active: false,
      main: false,
    };
    const switched = {
      activeBranchId: child.id,
      project,
      chapters: [],
    };
    const comparison = {
      source: child,
      target: main,
      addedChapters: 0,
      modifiedChapters: 1,
      removedChapters: 0,
      chapterChanges: [
        { chapterId: "chapter-1", title: "第一章", kind: "modified" },
      ],
      domains: [
        {
          key: "characters",
          label: "人物与势力",
          sourceCount: 2,
          targetCount: 1,
          changed: true,
        },
      ],
    };
    const preview = {
      token: "merge-token",
      sourceId: child.id,
      targetId: main.id,
      comparison,
    };
    const api = installNative({
      Branches: vi.fn(async () => [main, child]),
      CreateBranch: vi.fn(async () => child),
      SwitchBranch: vi.fn(async () => switched),
      CompareBranches: vi.fn(async () => comparison),
      PreviewBranchMerge: vi.fn(async () => preview),
      ConfirmBranchMerge: vi.fn(async () => switched),
    });

    await expect(wailsStudioClient.branches()).resolves.toEqual([main, child]);
    await expect(
      wailsStudioClient.createBranch({
        name: child.name,
        description: child.description,
      }),
    ).resolves.toEqual(child);
    await expect(wailsStudioClient.switchBranch(child.id)).resolves.toEqual(
      switched,
    );
    await expect(
      wailsStudioClient.compareBranches(child.id, main.id),
    ).resolves.toEqual(comparison);
    await expect(
      wailsStudioClient.previewBranchMerge(child.id),
    ).resolves.toEqual(preview);
    await expect(
      wailsStudioClient.confirmBranchMerge(preview.token),
    ).resolves.toEqual(switched);
    await wailsStudioClient.deleteBranch(child.id);

    expect(api.CreateBranch).toHaveBeenCalledWith({
      name: child.name,
      description: child.description,
    });
    expect(api.SwitchBranch).toHaveBeenCalledWith(child.id);
    expect(api.CompareBranches).toHaveBeenCalledWith(child.id, main.id);
    expect(api.ConfirmBranchMerge).toHaveBeenCalledWith(preview.token);
    expect(api.DeleteBranch).toHaveBeenCalledWith(child.id);
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

  it("forwards extraction run operations and validates task DTOs", async () => {
    const step = {
      id: "step-1",
      importChapterId: "candidate-1",
      createdChapterId: "chapter-1",
      chapterTitle: "第一章 潮声",
      position: 1,
      status: "completed",
      output: "",
      errorCode: "",
      errorMessage: "",
      providerRequestId: "req-1",
      inputTokens: 1200,
      outputTokens: 800,
    };
    const task = {
      id: "extraction-1",
      importJobId: "import-1",
      connectionId: "conn-1",
      model: "gpt-4o",
      prompt: "抽取设定",
      status: "completed",
      errorCode: "",
      errorMessage: "",
      changeSetId: "changeset-1",
      retryOf: "",
      tokenBudget: 30000,
      contextCharBudget: 120000,
      inputTokens: 1200,
      outputTokens: 800,
      steps: [step],
      createdAt: "2026-08-04T08:00:00Z",
      updatedAt: "2026-08-04T09:00:00Z",
    };
    const changeSet = {
      id: "changeset-1",
      title: "设定抽取",
      summary: "已合并",
      createdBy: "extraction",
      status: "pending",
      operations: [
        {
          id: "op-1",
          kind: "create_character",
          targetId: "",
          baseRevision: 1,
          baseFingerprint: "",
          beforeValue: "{}",
          afterValue: '{"name":"苏沉"}',
          reason: "主角",
          status: "pending",
          evidence: [
            {
              chapterId: "chapter-1",
              chapterTitle: "第一章 潮声",
              startLine: 3,
              endLine: 12,
              excerpt: "苏沉推开灯塔木门。",
            },
          ],
        },
      ],
      createdAt: "2026-08-04T09:00:00Z",
      updatedAt: "2026-08-04T09:00:00Z",
    };
    const run = { task, changeSet };
    const api = installNative({
      ExtractionTasks: vi.fn(async () => [task]),
      ExtractionTask: vi.fn(async () => run),
      StartExtractionRun: vi.fn(async () => run),
      ExecuteExtractionRun: vi.fn(async () => run),
      CancelExtractionRun: vi.fn(async () => undefined),
      RetryExtractionRun: vi.fn(async () => run),
    });

    await expect(wailsStudioClient.extractionTasks()).resolves.toEqual([task]);
    await expect(
      wailsStudioClient.extractionTask(task.id),
    ).resolves.toEqual(run);
    await expect(
      wailsStudioClient.startExtractionRun({
        importJobId: "import-1",
        connectionId: "conn-1",
        model: "gpt-4o",
        instruction: "",
      }),
    ).resolves.toEqual(run);
    await expect(
      wailsStudioClient.executeExtractionRun(task.id),
    ).resolves.toEqual(run);
    await expect(
      wailsStudioClient.cancelExtractionRun(task.id),
    ).resolves.toBeUndefined();
    await expect(
      wailsStudioClient.retryExtractionRun(task.id),
    ).resolves.toEqual(run);

    expect(api.ExtractionTasks).toHaveBeenCalledOnce();
    expect(api.ExtractionTask).toHaveBeenCalledWith(task.id);
    expect(api.StartExtractionRun).toHaveBeenCalledWith({
      importJobId: "import-1",
      connectionId: "conn-1",
      model: "gpt-4o",
      instruction: "",
    });
    expect(api.ExecuteExtractionRun).toHaveBeenCalledWith(task.id);
    expect(api.CancelExtractionRun).toHaveBeenCalledWith(task.id);
    expect(api.RetryExtractionRun).toHaveBeenCalledWith(task.id);
  });

  it("rejects extraction tasks with invalid status values", async () => {
    installNative({
      ExtractionTasks: vi.fn(async () => [
        {
          id: "extraction-bad",
          importJobId: "import-1",
          connectionId: "conn-1",
          model: "gpt-4o",
          prompt: "",
          status: "unknown",
          errorCode: "",
          errorMessage: "",
          changeSetId: "",
          retryOf: "",
          tokenBudget: 0,
          contextCharBudget: 0,
          inputTokens: 0,
          outputTokens: 0,
          steps: [],
          createdAt: "2026-08-04T08:00:00Z",
          updatedAt: "2026-08-04T08:00:00Z",
        },
      ]),
    });

    await expect(wailsStudioClient.extractionTasks()).rejects.toThrow();
  });

  it("rejects import jobs that report extraction status on non-applied tasks", async () => {
    installNative({
      ImportJobs: vi.fn(async () => [
        {
          id: "import-prepared",
          status: "prepared",
          sourceName: "稿.md",
          sourceFormat: "markdown",
          sourceEncoding: "UTF-8",
          sourceHashDigest: "0123456789ab",
          sourceBytes: 1024,
          archivedRelativePath: "archive/import-prepared",
          chapterCount: 1,
          selectedChapterCount: 0,
          extractionStatus: "running",
          extractionRunId: "run-1",
          createdAt: "2026-08-03T08:00:00Z",
          updatedAt: "2026-08-03T08:00:00Z",
          chapters: [
            {
              id: "candidate-1",
              volumeTitle: "",
              title: "第一章",
              wordCount: 100,
              bodyPreview: "正文",
              position: 1,
              sourceStartLine: 1,
              sourceEndLine: 2,
              contentHashSummary: "abcdef012345",
              selected: false,
            },
          ],
        },
      ]),
    });

    await expect(wailsStudioClient.importJobs()).rejects.toThrow();
  });
});
