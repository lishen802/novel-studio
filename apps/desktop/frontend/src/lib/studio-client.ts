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

export type WorldRegionKind =
  | "world"
  | "continent"
  | "territory"
  | "settlement"
  | "terrain"
  | "secret_realm"
  | "resource";
export type RelativeDirection =
  | "north"
  | "north_east"
  | "east"
  | "south_east"
  | "south"
  | "south_west"
  | "west"
  | "north_west"
  | "center"
  | "surrounding";
export type WorldRegionView = {
  id: string;
  parentId: string;
  kind: WorldRegionKind;
  name: string;
  direction: RelativeDirection;
  summary: string;
  terrain: string;
  climate: string;
};
export type WorldRegionInput = Omit<WorldRegionView, "id">;
export type RegionAdjacencyView = {
  firstRegionId: string;
  secondRegionId: string;
  travelNote: string;
};
export type RegionAdjacencyInput = RegionAdjacencyView;
export type LocationKind =
  "city" | "settlement" | "building" | "landmark" | "wilderness" | "other";
export type LocationStatus = "active" | "inactive" | "destroyed";
export type LocationView = {
  id: string;
  regionId: string;
  kind: LocationKind;
  name: string;
  summary: string;
  status: LocationStatus;
};
export type LocationInput = Omit<LocationView, "id">;
export type LocationTargetType = "chapter" | "scene" | "character" | "faction";
export type LocationLinkView = {
  locationId: string;
  targetType: LocationTargetType;
  targetId: string;
  role: string;
};
export type LocationLinkInput = LocationLinkView;
export type ItemStatus = "active" | "lost" | "destroyed";
export type HolderType = "character" | "faction" | "location";
export type ItemView = {
  id: string;
  name: string;
  summary: string;
  status: ItemStatus;
  holderType: HolderType | "";
  holderId: string;
};
export type ItemInput = Pick<ItemView, "name" | "summary" | "status">;
export type ItemTransferView = {
  id: string;
  itemId: string;
  fromType: HolderType | "";
  fromId: string;
  toType: HolderType;
  toId: string;
  chapterId: string;
  note: string;
};
export type ItemTransferInput = {
  itemId: string;
  holderType: HolderType;
  holderId: string;
  chapterId: string;
  note: string;
};
export type RuleScopeType = "project" | "region" | "location";
export type WorldRuleView = {
  id: string;
  name: string;
  description: string;
  scopeType: RuleScopeType;
  scopeId: string;
  status: "active" | "inactive";
};
export type WorldRuleInput = Omit<WorldRuleView, "id">;
export type HookStatus = "planted" | "progressing" | "resolved" | "abandoned";
export type ForeshadowingHookView = {
  id: string;
  title: string;
  setup: string;
  plantedChapterId: string;
  dueChapterId: string;
  resolvedChapterId: string;
  status: HookStatus;
  notes: string;
};
export type ForeshadowingHookInput = Omit<ForeshadowingHookView, "id">;
export type WorldAssetsView = {
  regions: WorldRegionView[];
  regionAdjacencies: RegionAdjacencyView[];
  locations: LocationView[];
  locationLinks: LocationLinkView[];
  items: ItemView[];
  itemTransfers: ItemTransferView[];
  rules: WorldRuleView[];
  hooks: ForeshadowingHookView[];
  consistencyIssues: ConsistencyIssueView[];
};

export type EventStatus = "planned" | "occurred" | "uncertain";
export type EventTargetType =
  "chapter" | "character" | "faction" | "location" | "item";
export type StoryEventView = {
  id: string;
  title: string;
  summary: string;
  cause: string;
  result: string;
  storyTime: string;
  storyOrder: number;
  narrativeOrder: number;
  status: EventStatus;
};
export type StoryEventInput = Omit<StoryEventView, "id">;
export type StoryEventLinkView = {
  eventId: string;
  targetType: EventTargetType;
  targetId: string;
  role: string;
};
export type StoryEventLinkInput = StoryEventLinkView;
export type FactOrigin = "author" | "text" | "model" | "rule";
export type FactStatus =
  | "author_confirmed"
  | "text_evidenced"
  | "ai_inferred"
  | "conflicted"
  | "rejected";
export type FactView = {
  id: string;
  text: string;
  category: string;
  origin: FactOrigin;
  status: FactStatus;
  confidence: number;
  validFromEventId: string;
  validUntilEventId: string;
  createdBy: string;
  confirmedBy: string;
};
export type FactInput = Pick<
  FactView,
  | "text"
  | "category"
  | "origin"
  | "status"
  | "confidence"
  | "validFromEventId"
  | "validUntilEventId"
>;
export type FactEvidenceView = {
  id: string;
  factId: string;
  chapterId: string;
  quote: string;
  startOffset: number;
  endOffset: number;
  chapterRevision: number;
};
export type FactEvidenceInput = Omit<FactEvidenceView, "id">;
export type FactScopeTargetType =
  "character" | "faction" | "location" | "item" | "region" | "rule" | "event";
export type FactScopeView = {
  id: string;
  factId: string;
  targetType: FactScopeTargetType;
  targetId: string;
};
export type FactScopeInput = Omit<FactScopeView, "id">;
export type FactConflictView = {
  id: string;
  factId: string;
  otherFactId: string;
  reason: string;
  status: "open" | "resolved";
};
export type FactConflictInput = Pick<
  FactConflictView,
  "factId" | "otherFactId" | "reason"
>;
export type ContinuityView = {
  events: StoryEventView[];
  eventLinks: StoryEventLinkView[];
  facts: FactView[];
  evidence: FactEvidenceView[];
  scopes: FactScopeView[];
  conflicts: FactConflictView[];
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
  worldAssets(): Promise<WorldAssetsView>;
  createWorldRegion(input: WorldRegionInput): Promise<WorldRegionView>;
  updateWorldRegion(
    id: string,
    input: WorldRegionInput,
  ): Promise<WorldRegionView>;
  archiveWorldRegion(id: string): Promise<void>;
  upsertRegionAdjacency(
    input: RegionAdjacencyInput,
  ): Promise<RegionAdjacencyView>;
  deleteRegionAdjacency(firstRegionId: string, secondRegionId: string): Promise<void>;
  createLocation(input: LocationInput): Promise<LocationView>;
  updateLocation(id: string, input: LocationInput): Promise<LocationView>;
  archiveLocation(id: string): Promise<void>;
  linkLocation(input: LocationLinkInput): Promise<LocationLinkView>;
  unlinkLocation(
    locationId: string,
    targetType: LocationTargetType,
    targetId: string,
  ): Promise<void>;
  createItem(input: ItemInput): Promise<ItemView>;
  updateItem(id: string, input: ItemInput): Promise<ItemView>;
  transferItem(input: ItemTransferInput): Promise<ItemTransferView>;
  archiveItem(id: string): Promise<void>;
  createWorldRule(input: WorldRuleInput): Promise<WorldRuleView>;
  updateWorldRule(id: string, input: WorldRuleInput): Promise<WorldRuleView>;
  archiveWorldRule(id: string): Promise<void>;
  createForeshadowingHook(
    input: ForeshadowingHookInput,
  ): Promise<ForeshadowingHookView>;
  updateForeshadowingHook(
    id: string,
    input: ForeshadowingHookInput,
  ): Promise<ForeshadowingHookView>;
  archiveForeshadowingHook(id: string): Promise<void>;
  continuity(): Promise<ContinuityView>;
  createStoryEvent(input: StoryEventInput): Promise<StoryEventView>;
  updateStoryEvent(id: string, input: StoryEventInput): Promise<StoryEventView>;
  archiveStoryEvent(id: string): Promise<void>;
  linkStoryEvent(input: StoryEventLinkInput): Promise<StoryEventLinkView>;
  unlinkStoryEvent(
    eventId: string,
    targetType: EventTargetType,
    targetId: string,
  ): Promise<void>;
  createFact(input: FactInput): Promise<FactView>;
  updateFact(id: string, input: FactInput): Promise<FactView>;
  confirmFact(id: string): Promise<FactView>;
  archiveFact(id: string): Promise<void>;
  addFactEvidence(input: FactEvidenceInput): Promise<FactEvidenceView>;
  removeFactEvidence(id: string): Promise<void>;
  addFactScope(input: FactScopeInput): Promise<FactScopeView>;
  removeFactScope(id: string): Promise<void>;
  createFactConflict(input: FactConflictInput): Promise<FactConflictView>;
  resolveFactConflict(id: string): Promise<FactConflictView>;
  deleteFactConflict(id: string): Promise<void>;
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
