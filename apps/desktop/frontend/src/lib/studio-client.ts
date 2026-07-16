export type ProjectView = {
  id: string;
  title: string;
  genre: string;
  summary: string;
  path: string;
};

export type ChapterView = {
  id: string;
  projectId: string;
  title: string;
  position: number;
  body: string;
  revision: number;
  updatedAt: string;
  recoveryBody?: string;
  recoveryUpdatedAt?: string;
  hasRecoveryDraft: boolean;
};

export type CreateProjectInput = {
  parentDirectory: string;
  title: string;
  genre: string;
  summary: string;
};

export type OutlineLevel = "book" | "volume" | "arc" | "chapter" | "scene";

export type OutlineNodeView = {
  id: string;
  parentId: string;
  level: OutlineLevel;
  title: string;
  summary: string;
  goal: string;
  conflict: string;
  outcome: string;
  position: number;
  chapterId: string;
};

export type StoryLineView = {
  id: string;
  name: string;
  summary: string;
  status: string;
};
export type StoryLineLinkView = {
  storyLineId: string;
  outlineNodeId: string;
  position: number;
};
export type SceneCardView = {
  id: string;
  chapterId: string;
  outlineNodeId: string;
  title: string;
  pov: string;
  location: string;
  goal: string;
  conflict: string;
  outcome: string;
  position: number;
  status: string;
};
export type StoryStructureView = {
  outlineNodes: OutlineNodeView[];
  storyLines: StoryLineView[];
  storyLineLinks: StoryLineLinkView[];
  sceneCards: SceneCardView[];
};
export type CreateOutlineNodeInput = Omit<OutlineNodeView, "id" | "position">;
export type CreateStoryLineInput = { name: string; summary: string };
export type LinkStoryLineInput = { storyLineId: string; outlineNodeId: string };
export type CreateSceneCardInput = Omit<
  SceneCardView,
  "id" | "position" | "status"
>;

export type CharacterRole =
  "protagonist" | "supporting" | "antagonist" | "minor";
export type CharacterStatus = "active" | "inactive" | "deceased";
export type CharacterView = {
  id: string;
  name: string;
  aliases: string[];
  role: CharacterRole;
  summary: string;
  motivation: string;
  fear: string;
  secret: string;
  arc: string;
  status: CharacterStatus;
};
export type CharacterInput = Omit<CharacterView, "id">;
export type FactionStatus = "active" | "inactive" | "disbanded";
export type FactionView = {
  id: string;
  name: string;
  summary: string;
  goal: string;
  status: FactionStatus;
};
export type FactionInput = Omit<FactionView, "id">;
export type MembershipStatus = "active" | "former" | "secret";
export type MembershipView = {
  characterId: string;
  factionId: string;
  role: string;
  status: MembershipStatus;
};
export type MembershipInput = MembershipView;
export type RelationshipKind =
  "ally" | "family" | "romantic" | "rival" | "enemy" | "mentor" | "other";
export type RelationshipView = {
  id: string;
  sourceCharacterId: string;
  targetCharacterId: string;
  kind: RelationshipKind;
  label: string;
  strength: number;
  publicState: string;
  trueState: string;
};
export type RelationshipInput = Omit<RelationshipView, "id">;
export type AppearanceRole = "pov" | "present" | "mentioned";
export type AppearanceInput = {
  characterId: string;
  targetId: string;
  role: AppearanceRole;
};
export type ChapterAppearanceView = {
  characterId: string;
  chapterId: string;
  role: AppearanceRole;
};
export type SceneAppearanceView = {
  characterId: string;
  sceneCardId: string;
  role: AppearanceRole;
};
export type ConsistencyIssueView = {
  code: string;
  severity: string;
  message: string;
  entityIds: string[];
};
export type StoryWorldView = {
  characters: CharacterView[];
  factions: FactionView[];
  memberships: MembershipView[];
  relationships: RelationshipView[];
  chapterAppearances: ChapterAppearanceView[];
  sceneAppearances: SceneAppearanceView[];
  consistencyIssues: ConsistencyIssueView[];
};

export interface StudioClient {
  chooseProjectParent(): Promise<string | null>;
  chooseProjectToOpen(): Promise<string | null>;
  createProject(input: CreateProjectInput): Promise<ProjectView>;
  openProject(path: string): Promise<ProjectView>;
  listChapters(): Promise<ChapterView[]>;
  createChapter(title: string): Promise<ChapterView>;
  openChapter(chapterId: string): Promise<ChapterView>;
  saveDraft(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<void>;
  discardDraft(chapterId: string, baseRevision: number): Promise<void>;
  commitChapter(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<ChapterView>;
  storyStructure(): Promise<StoryStructureView>;
  createOutlineNode(input: CreateOutlineNodeInput): Promise<OutlineNodeView>;
  createStoryLine(input: CreateStoryLineInput): Promise<StoryLineView>;
  linkStoryLine(input: LinkStoryLineInput): Promise<StoryLineLinkView>;
  createSceneCard(input: CreateSceneCardInput): Promise<SceneCardView>;
  archiveOutlineNode(id: string): Promise<void>;
  moveOutlineNode(id: string, direction: "up" | "down"): Promise<void>;
  storyWorld(): Promise<StoryWorldView>;
  createCharacter(input: CharacterInput): Promise<CharacterView>;
  updateCharacter(id: string, input: CharacterInput): Promise<CharacterView>;
  archiveCharacter(id: string): Promise<void>;
  createFaction(input: FactionInput): Promise<FactionView>;
  updateFaction(id: string, input: FactionInput): Promise<FactionView>;
  archiveFaction(id: string): Promise<void>;
  upsertMembership(input: MembershipInput): Promise<MembershipView>;
  deleteMembership(characterId: string, factionId: string): Promise<void>;
  createRelationship(input: RelationshipInput): Promise<RelationshipView>;
  archiveRelationship(id: string): Promise<void>;
  linkCharacterToChapter(input: AppearanceInput): Promise<void>;
  unlinkCharacterFromChapter(
    characterId: string,
    chapterId: string,
  ): Promise<void>;
  linkCharacterToScene(input: AppearanceInput): Promise<void>;
  unlinkCharacterFromScene(
    characterId: string,
    sceneCardId: string,
  ): Promise<void>;
  onBeforeClose(handler: () => Promise<void>): () => void;
  completeClose(): Promise<void>;
}

export class RevisionConflictError extends Error {
  constructor() {
    super("章节版本冲突");
    this.name = "RevisionConflictError";
  }
}

export class StudioClientError extends Error {
  constructor(
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "StudioClientError";
  }
}
