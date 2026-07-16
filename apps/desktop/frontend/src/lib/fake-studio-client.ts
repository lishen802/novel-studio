import type {
  AppearanceInput,
  ChapterView,
  CharacterInput,
  CharacterView,
  CreateProjectInput,
  CreateOutlineNodeInput,
  CreateSceneCardInput,
  CreateStoryLineInput,
  FactionInput,
  FactionView,
  LinkStoryLineInput,
  MembershipInput,
  MembershipView,
  OutlineNodeView,
  ProjectView,
  RelationshipInput,
  RelationshipView,
  SceneCardView,
  StoryLineLinkView,
  StoryLineView,
  StoryStructureView,
  StoryWorldView,
  StudioClient,
} from "./studio-client";

type ChapterWriteCall = {
  chapterId: string;
  body: string;
  baseRevision: number;
};

export class FakeStudioClient implements StudioClient {
  projectParent: string | null = null;
  projectToOpen: string | null = null;
  createProjectResult: ProjectView = {
    id: "project-1",
    title: "长夜织境",
    genre: "东方奇幻",
    summary: "",
    path: "/books/长夜织境.zjproject",
  };
  openProjectResult: ProjectView = this.createProjectResult;
  chaptersResult: ChapterView[] = [];
  createChapterResult: ChapterView = {
    id: "chapter-1",
    projectId: "project-1",
    title: "第一章",
    position: 1,
    body: "",
    revision: 1,
    updatedAt: "2026-07-15T08:00:00Z",
    hasRecoveryDraft: false,
  };
  openChapterResult: ChapterView = this.createChapterResult;
  commitChapterResult: ChapterView = {
    ...this.createChapterResult,
    revision: 2,
  };
  storyStructureResult: StoryStructureView = {
    outlineNodes: [],
    storyLines: [],
    storyLineLinks: [],
    sceneCards: [],
  };
  storyWorldResult: StoryWorldView = {
    characters: [],
    factions: [],
    memberships: [],
    relationships: [],
    chapterAppearances: [],
    sceneAppearances: [],
    consistencyIssues: [],
  };

  createProjectError: Error | null = null;
  openProjectError: Error | null = null;
  createChapterError: Error | null = null;
  openChapterError: Error | null = null;
  saveDraftError: Error | null = null;
  discardDraftError: Error | null = null;
  commitChapterError: Error | null = null;
  storyWorldError: Error | null = null;
  worldMutationError: Error | null = null;

  createProjectCalls: CreateProjectInput[] = [];
  openProjectCalls: string[] = [];
  createChapterCalls: string[] = [];
  openChapterCalls: string[] = [];
  saveDraftCalls: ChapterWriteCall[] = [];
  discardDraftCalls: Array<{ chapterId: string; baseRevision: number }> = [];
  commitChapterCalls: ChapterWriteCall[] = [];
  createOutlineNodeCalls: CreateOutlineNodeInput[] = [];
  createStoryLineCalls: CreateStoryLineInput[] = [];
  linkStoryLineCalls: LinkStoryLineInput[] = [];
  createSceneCardCalls: CreateSceneCardInput[] = [];
  archiveOutlineNodeCalls: string[] = [];
  moveOutlineNodeCalls: Array<{ id: string; direction: "up" | "down" }> = [];
  createCharacterCalls: CharacterInput[] = [];
  updateCharacterCalls: Array<{ id: string; input: CharacterInput }> = [];
  archiveCharacterCalls: string[] = [];
  createFactionCalls: FactionInput[] = [];
  updateFactionCalls: Array<{ id: string; input: FactionInput }> = [];
  archiveFactionCalls: string[] = [];
  upsertMembershipCalls: MembershipInput[] = [];
  deleteMembershipCalls: Array<{ characterId: string; factionId: string }> = [];
  createRelationshipCalls: RelationshipInput[] = [];
  archiveRelationshipCalls: string[] = [];
  linkChapterCalls: AppearanceInput[] = [];
  unlinkChapterCalls: Array<{ characterId: string; chapterId: string }> = [];
  linkSceneCalls: AppearanceInput[] = [];
  unlinkSceneCalls: Array<{ characterId: string; sceneCardId: string }> = [];
  completeCloseCalls = 0;
  beforeCloseHandler: (() => Promise<void>) | null = null;

  static withReadyProject(): FakeStudioClient {
    const client = new FakeStudioClient();
    client.chaptersResult = [];
    return client;
  }

  async chooseProjectParent(): Promise<string | null> {
    return this.projectParent;
  }

  async chooseProjectToOpen(): Promise<string | null> {
    return this.projectToOpen;
  }

  async createProject(input: CreateProjectInput): Promise<ProjectView> {
    this.createProjectCalls.push(input);
    if (this.createProjectError) throw this.createProjectError;
    return this.createProjectResult;
  }

  async openProject(path: string): Promise<ProjectView> {
    this.openProjectCalls.push(path);
    if (this.openProjectError) throw this.openProjectError;
    return this.openProjectResult;
  }

  async listChapters(): Promise<ChapterView[]> {
    return this.chaptersResult;
  }

  async createChapter(title: string): Promise<ChapterView> {
    this.createChapterCalls.push(title);
    if (this.createChapterError) throw this.createChapterError;
    const created = { ...this.createChapterResult, title };
    this.openChapterResult = created;
    this.commitChapterResult = { ...this.commitChapterResult, title };
    return created;
  }

  async openChapter(chapterId: string): Promise<ChapterView> {
    this.openChapterCalls.push(chapterId);
    if (this.openChapterError) throw this.openChapterError;
    const listed = this.chaptersResult.find(
      (chapter) => chapter.id === chapterId,
    );
    return listed ?? this.openChapterResult;
  }

  async saveDraft(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<void> {
    this.saveDraftCalls.push({ chapterId, body, baseRevision });
    if (this.saveDraftError) throw this.saveDraftError;
  }

  async discardDraft(chapterId: string, baseRevision: number): Promise<void> {
    this.discardDraftCalls.push({ chapterId, baseRevision });
    if (this.discardDraftError) throw this.discardDraftError;
  }

  async commitChapter(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<ChapterView> {
    this.commitChapterCalls.push({ chapterId, body, baseRevision });
    if (this.commitChapterError) throw this.commitChapterError;
    return { ...this.commitChapterResult, id: chapterId, body };
  }

  async storyStructure(): Promise<StoryStructureView> {
    return this.storyStructureResult;
  }

  async createOutlineNode(
    input: CreateOutlineNodeInput,
  ): Promise<OutlineNodeView> {
    this.createOutlineNodeCalls.push(input);
    const node: OutlineNodeView = {
      ...input,
      id: `outline-${this.storyStructureResult.outlineNodes.length + 1}`,
      position:
        this.storyStructureResult.outlineNodes.filter(
          (item) => item.parentId === input.parentId,
        ).length + 1,
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      outlineNodes: [...this.storyStructureResult.outlineNodes, node],
    };
    return node;
  }

  async createStoryLine(input: CreateStoryLineInput): Promise<StoryLineView> {
    this.createStoryLineCalls.push(input);
    const line = {
      ...input,
      id: `line-${this.storyStructureResult.storyLines.length + 1}`,
      status: "planned",
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      storyLines: [...this.storyStructureResult.storyLines, line],
    };
    return line;
  }

  async linkStoryLine(input: LinkStoryLineInput): Promise<StoryLineLinkView> {
    this.linkStoryLineCalls.push(input);
    const link = {
      ...input,
      position:
        this.storyStructureResult.storyLineLinks.filter(
          (item) => item.storyLineId === input.storyLineId,
        ).length + 1,
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      storyLineLinks: [...this.storyStructureResult.storyLineLinks, link],
    };
    return link;
  }

  async createSceneCard(input: CreateSceneCardInput): Promise<SceneCardView> {
    this.createSceneCardCalls.push(input);
    const card = {
      ...input,
      id: `card-${this.storyStructureResult.sceneCards.length + 1}`,
      position:
        this.storyStructureResult.sceneCards.filter(
          (item) => item.chapterId === input.chapterId,
        ).length + 1,
      status: "planned",
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      sceneCards: [...this.storyStructureResult.sceneCards, card],
    };
    return card;
  }

  async archiveOutlineNode(id: string): Promise<void> {
    this.archiveOutlineNodeCalls.push(id);
    this.storyStructureResult = {
      ...this.storyStructureResult,
      outlineNodes: this.storyStructureResult.outlineNodes.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async moveOutlineNode(id: string, direction: "up" | "down"): Promise<void> {
    this.moveOutlineNodeCalls.push({ id, direction });
  }

  async storyWorld(): Promise<StoryWorldView> {
    if (this.storyWorldError) throw this.storyWorldError;
    return this.storyWorldResult;
  }

  async createCharacter(input: CharacterInput): Promise<CharacterView> {
    this.createCharacterCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const character: CharacterView = {
      ...input,
      id: `character-${this.storyWorldResult.characters.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: [...this.storyWorldResult.characters, character],
    };
    return character;
  }

  async updateCharacter(
    id: string,
    input: CharacterInput,
  ): Promise<CharacterView> {
    this.updateCharacterCalls.push({ id, input });
    if (this.worldMutationError) throw this.worldMutationError;
    const character = { ...input, id };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: this.storyWorldResult.characters.map((item) =>
        item.id === id ? character : item,
      ),
    };
    return character;
  }

  async archiveCharacter(id: string): Promise<void> {
    this.archiveCharacterCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: this.storyWorldResult.characters.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async createFaction(input: FactionInput): Promise<FactionView> {
    this.createFactionCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const faction: FactionView = {
      ...input,
      id: `faction-${this.storyWorldResult.factions.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: [...this.storyWorldResult.factions, faction],
    };
    return faction;
  }

  async updateFaction(id: string, input: FactionInput): Promise<FactionView> {
    this.updateFactionCalls.push({ id, input });
    if (this.worldMutationError) throw this.worldMutationError;
    const faction = { ...input, id };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: this.storyWorldResult.factions.map((item) =>
        item.id === id ? faction : item,
      ),
    };
    return faction;
  }

  async archiveFaction(id: string): Promise<void> {
    this.archiveFactionCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: this.storyWorldResult.factions.filter((item) => item.id !== id),
    };
  }

  async upsertMembership(input: MembershipInput): Promise<MembershipView> {
    this.upsertMembershipCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const others = this.storyWorldResult.memberships.filter(
      (item) =>
        item.characterId !== input.characterId ||
        item.factionId !== input.factionId,
    );
    this.storyWorldResult = {
      ...this.storyWorldResult,
      memberships: [...others, input],
    };
    return input;
  }

  async deleteMembership(
    characterId: string,
    factionId: string,
  ): Promise<void> {
    this.deleteMembershipCalls.push({ characterId, factionId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      memberships: this.storyWorldResult.memberships.filter(
        (item) =>
          item.characterId !== characterId || item.factionId !== factionId,
      ),
    };
  }

  async createRelationship(
    input: RelationshipInput,
  ): Promise<RelationshipView> {
    this.createRelationshipCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const relationship = {
      ...input,
      id: `relationship-${this.storyWorldResult.relationships.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      relationships: [...this.storyWorldResult.relationships, relationship],
    };
    return relationship;
  }

  async archiveRelationship(id: string): Promise<void> {
    this.archiveRelationshipCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      relationships: this.storyWorldResult.relationships.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async linkCharacterToChapter(input: AppearanceInput): Promise<void> {
    this.linkChapterCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      chapterAppearances: [
        ...this.storyWorldResult.chapterAppearances.filter(
          (item) =>
            item.characterId !== input.characterId ||
            item.chapterId !== input.targetId,
        ),
        {
          characterId: input.characterId,
          chapterId: input.targetId,
          role: input.role,
        },
      ],
    };
  }

  async unlinkCharacterFromChapter(
    characterId: string,
    chapterId: string,
  ): Promise<void> {
    this.unlinkChapterCalls.push({ characterId, chapterId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      chapterAppearances: this.storyWorldResult.chapterAppearances.filter(
        (item) =>
          item.characterId !== characterId || item.chapterId !== chapterId,
      ),
    };
  }

  async linkCharacterToScene(input: AppearanceInput): Promise<void> {
    this.linkSceneCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      sceneAppearances: [
        ...this.storyWorldResult.sceneAppearances.filter(
          (item) =>
            item.characterId !== input.characterId ||
            item.sceneCardId !== input.targetId,
        ),
        {
          characterId: input.characterId,
          sceneCardId: input.targetId,
          role: input.role,
        },
      ],
    };
  }

  async unlinkCharacterFromScene(
    characterId: string,
    sceneCardId: string,
  ): Promise<void> {
    this.unlinkSceneCalls.push({ characterId, sceneCardId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      sceneAppearances: this.storyWorldResult.sceneAppearances.filter(
        (item) =>
          item.characterId !== characterId || item.sceneCardId !== sceneCardId,
      ),
    };
  }

  onBeforeClose(handler: () => Promise<void>): () => void {
    this.beforeCloseHandler = handler;
    return () => {
      if (this.beforeCloseHandler === handler) this.beforeCloseHandler = null;
    };
  }

  async completeClose(): Promise<void> {
    this.completeCloseCalls++;
  }

  async requestNativeClose(): Promise<void> {
    await this.beforeCloseHandler?.();
  }
}
