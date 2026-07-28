import { z } from "zod";
import {
  RevisionConflictError,
  StudioClientError,
  type ChapterView,
  type BranchComparisonView,
  type BranchMergePreviewView,
  type BranchSwitchView,
  type BranchView,
  type ChapterChangeInput,
  type ChapterRevisionView,
  type ChangeSetView,
  type ChangeOperationEditInput,
  type ChangeSetApplyView,
  type StructuredChangeInput,
  type AppearanceInput,
  type CharacterInput,
  type CharacterView,
  type CreateOutlineNodeInput,
  type CreateProjectInput,
  type CreateBranchInput,
  type CreateSceneCardInput,
  type CreateStoryLineInput,
  type FactionInput,
  type FactionView,
  type LinkStoryLineInput,
  type LocationInput,
  type LocationLinkInput,
  type LocationLinkView,
  type LocationTargetType,
  type LocationView,
  type MembershipInput,
  type MembershipView,
  type OutlineNodeView,
  type ProjectView,
  type RelationshipInput,
  type RelationshipView,
  type RegionAdjacencyInput,
  type RegionAdjacencyView,
  type SceneCardView,
  type SnapshotRestoreView,
  type SnapshotView,
  type StoryLineLinkView,
  type StoryLineView,
  type StoryStructureView,
  type StoryWorldView,
  type StudioClient,
  type ItemInput,
  type ItemTransferInput,
  type ItemTransferView,
  type ItemView,
  type WorldAssetsView,
  type WorldRegionInput,
  type WorldRegionView,
  type WorldRuleInput,
  type WorldRuleView,
  type ForeshadowingHookInput,
  type ForeshadowingHookView,
  type ContinuityView,
  type StoryEventInput,
  type StoryEventView,
  type StoryEventLinkInput,
  type StoryEventLinkView,
  type EventTargetType,
  type FactInput,
  type FactView,
  type FactEvidenceInput,
  type FactEvidenceView,
  type FactScopeInput,
  type FactScopeView,
  type FactConflictInput,
  type FactConflictView,
  type CharacterKnowledgeInput,
  type CharacterKnowledgeView,
} from "./studio-client";

const projectSchema = z.object({
  id: z.string(),
  title: z.string(),
  genre: z.string(),
  summary: z.string(),
  path: z.string(),
});

const chapterSchema = z.object({
  id: z.string(),
  projectId: z.string(),
  title: z.string(),
  position: z.number(),
  body: z.string(),
  revision: z.number(),
  updatedAt: z.string(),
  recoveryBody: z.string().optional(),
  recoveryUpdatedAt: z.string().optional(),
  hasRecoveryDraft: z.boolean(),
});
const changeSetStatusSchema = z.enum([
  "pending",
  "partially_applied",
  "applied",
  "rejected",
  "conflicted",
]);
const changeOperationSchema = z.object({
  id: z.string(),
  kind: z.enum([
    "replace_chapter_body",
    "update_outline_node",
    "update_character",
    "update_fact",
  ]),
  targetId: z.string(),
  baseRevision: z.number(),
  baseFingerprint: z.string(),
  beforeValue: z.string(),
  afterValue: z.string(),
  reason: z.string(),
  status: changeSetStatusSchema,
});
const changeSetSchema = z.object({
  id: z.string(),
  title: z.string(),
  summary: z.string(),
  createdBy: z.string(),
  status: changeSetStatusSchema,
  operations: z.array(changeOperationSchema),
  createdAt: z.string(),
  updatedAt: z.string(),
});
const changeSetApplySchema = z.object({
  changeSet: changeSetSchema,
  chapters: z.array(chapterSchema),
});
const revisionSourceSchema = z.enum([
  "initial",
  "author",
  "agent",
  "restore",
]);
const chapterRevisionSchema = z.object({
  chapterId: z.string(),
  revision: z.number(),
  title: z.string(),
  body: z.string(),
  source: revisionSourceSchema,
  summary: z.string(),
  createdAt: z.string(),
  isCurrent: z.boolean(),
});
const snapshotSchema = z.object({
  id: z.string(),
  name: z.string(),
  kind: z.enum(["named", "recovery"]),
  createdAt: z.string(),
});
const snapshotRestoreSchema = z.object({
  snapshotId: z.string(),
  project: projectSchema,
  chapters: z.array(chapterSchema),
});
const branchSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  parentId: z.string(),
  active: z.boolean(),
  main: z.boolean(),
  generation: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
});
const branchSwitchSchema = z.object({
  activeBranchId: z.string(),
  project: projectSchema,
  chapters: z.array(chapterSchema),
});
const branchComparisonSchema = z.object({
  source: branchSchema,
  target: branchSchema,
  addedChapters: z.number(),
  modifiedChapters: z.number(),
  removedChapters: z.number(),
  chapterChanges: z.array(
    z.object({
      chapterId: z.string(),
      title: z.string(),
      kind: z.enum(["added", "modified", "removed"]),
    }),
  ),
  domains: z.array(
    z.object({
      key: z.string(),
      label: z.string(),
      sourceCount: z.number(),
      targetCount: z.number(),
      changed: z.boolean(),
    }),
  ),
});
const branchMergePreviewSchema = z.object({
  token: z.string(),
  sourceId: z.string(),
  targetId: z.string(),
  comparison: branchComparisonSchema,
});

const outlineNodeSchema = z.object({
  id: z.string(),
  parentId: z.string(),
  level: z.enum(["book", "volume", "arc", "chapter", "scene"]),
  title: z.string(),
  summary: z.string(),
  goal: z.string(),
  conflict: z.string(),
  outcome: z.string(),
  position: z.number(),
  chapterId: z.string(),
});
const storyLineSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  status: z.string(),
});
const storyLineLinkSchema = z.object({
  storyLineId: z.string(),
  outlineNodeId: z.string(),
  position: z.number(),
});
const sceneCardSchema = z.object({
  id: z.string(),
  chapterId: z.string(),
  outlineNodeId: z.string(),
  title: z.string(),
  pov: z.string(),
  location: z.string(),
  goal: z.string(),
  conflict: z.string(),
  outcome: z.string(),
  position: z.number(),
  status: z.string(),
});
const storyStructureSchema = z.object({
  outlineNodes: z.array(outlineNodeSchema),
  storyLines: z.array(storyLineSchema),
  storyLineLinks: z.array(storyLineLinkSchema),
  sceneCards: z.array(sceneCardSchema),
});

const characterSchema = z.object({
  id: z.string(),
  name: z.string(),
  aliases: z.array(z.string()),
  role: z.enum(["protagonist", "supporting", "antagonist", "minor"]),
  summary: z.string(),
  motivation: z.string(),
  fear: z.string(),
  secret: z.string(),
  arc: z.string(),
  status: z.enum(["active", "inactive", "deceased"]),
});
const factionSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  goal: z.string(),
  status: z.enum(["active", "inactive", "disbanded"]),
});
const membershipSchema = z.object({
  characterId: z.string(),
  factionId: z.string(),
  role: z.string(),
  status: z.enum(["active", "former", "secret"]),
});
const relationshipSchema = z.object({
  id: z.string(),
  sourceCharacterId: z.string(),
  targetCharacterId: z.string(),
  kind: z.enum([
    "ally",
    "family",
    "romantic",
    "rival",
    "enemy",
    "mentor",
    "other",
  ]),
  label: z.string(),
  strength: z.number(),
  publicState: z.string(),
  trueState: z.string(),
});
const appearanceRoleSchema = z.enum(["pov", "present", "mentioned"]);
const storyWorldSchema = z.object({
  characters: z.array(characterSchema),
  factions: z.array(factionSchema),
  memberships: z.array(membershipSchema),
  relationships: z.array(relationshipSchema),
  chapterAppearances: z.array(
    z.object({
      characterId: z.string(),
      chapterId: z.string(),
      role: appearanceRoleSchema,
    }),
  ),
  sceneAppearances: z.array(
    z.object({
      characterId: z.string(),
      sceneCardId: z.string(),
      role: appearanceRoleSchema,
    }),
  ),
  consistencyIssues: z.array(
    z.object({
      code: z.string(),
      severity: z.string(),
      message: z.string(),
      entityIds: z.array(z.string()),
    }),
  ),
});

const worldRegionSchema = z.object({
  id: z.string(),
  parentId: z.string(),
  kind: z.enum([
    "world",
    "continent",
    "territory",
    "settlement",
    "terrain",
    "secret_realm",
    "resource",
  ]),
  name: z.string(),
  direction: z.enum([
    "north",
    "north_east",
    "east",
    "south_east",
    "south",
    "south_west",
    "west",
    "north_west",
    "center",
    "surrounding",
  ]),
  summary: z.string(),
  terrain: z.string(),
  climate: z.string(),
});
const locationSchema = z.object({
  id: z.string(),
  regionId: z.string(),
  kind: z.enum([
    "city",
    "settlement",
    "building",
    "landmark",
    "wilderness",
    "other",
  ]),
  name: z.string(),
  summary: z.string(),
  status: z.enum(["active", "inactive", "destroyed"]),
});
const locationLinkSchema = z.object({
  locationId: z.string(),
  targetType: z.enum(["chapter", "scene", "character", "faction"]),
  targetId: z.string(),
  role: z.string(),
});
const regionAdjacencySchema = z.object({
  firstRegionId: z.string(),
  secondRegionId: z.string(),
  travelNote: z.string(),
});
const itemSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  status: z.enum(["active", "lost", "destroyed"]),
  holderType: z.union([
    z.enum(["character", "faction", "location"]),
    z.literal(""),
  ]),
  holderId: z.string(),
});
const itemTransferSchema = z.object({
  id: z.string(),
  itemId: z.string(),
  fromType: z.union([
    z.enum(["character", "faction", "location"]),
    z.literal(""),
  ]),
  fromId: z.string(),
  toType: z.enum(["character", "faction", "location"]),
  toId: z.string(),
  chapterId: z.string(),
  note: z.string(),
});
const worldRuleSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  scopeType: z.enum(["project", "region", "location"]),
  scopeId: z.string(),
  status: z.enum(["active", "inactive"]),
});
const foreshadowingHookSchema = z.object({
  id: z.string(),
  title: z.string(),
  setup: z.string(),
  plantedChapterId: z.string(),
  dueChapterId: z.string(),
  resolvedChapterId: z.string(),
  status: z.enum(["planted", "progressing", "resolved", "abandoned"]),
  notes: z.string(),
});
const consistencyIssueSchema = z.object({
  code: z.string(),
  severity: z.string(),
  message: z.string(),
  entityIds: z.array(z.string()),
});
const worldAssetsSchema = z.object({
  regions: z.array(worldRegionSchema),
  regionAdjacencies: z.array(regionAdjacencySchema).default([]),
  locations: z.array(locationSchema),
  locationLinks: z.array(locationLinkSchema),
  items: z.array(itemSchema),
  itemTransfers: z.array(itemTransferSchema),
  rules: z.array(worldRuleSchema),
  hooks: z.array(foreshadowingHookSchema),
  consistencyIssues: z.array(consistencyIssueSchema),
});
const storyEventSchema = z.object({
  id: z.string(),
  title: z.string(),
  summary: z.string(),
  cause: z.string(),
  result: z.string(),
  storyTime: z.string(),
  storyOrder: z.number(),
  narrativeOrder: z.number(),
  status: z.enum(["planned", "occurred", "uncertain"]),
});
const storyEventLinkSchema = z.object({
  eventId: z.string(),
  targetType: z.enum(["chapter", "character", "faction", "location", "item"]),
  targetId: z.string(),
  role: z.string(),
});
const factSchema = z.object({
  id: z.string(),
  text: z.string(),
  category: z.string(),
  origin: z.enum(["author", "text", "model", "rule"]),
  status: z.enum([
    "author_confirmed",
    "text_evidenced",
    "ai_inferred",
    "conflicted",
    "rejected",
  ]),
  confidence: z.number(),
  validFromEventId: z.string(),
  validUntilEventId: z.string(),
  createdBy: z.string(),
  confirmedBy: z.string(),
});
const factEvidenceSchema = z.object({
  id: z.string(),
  factId: z.string(),
  chapterId: z.string(),
  quote: z.string(),
  startOffset: z.number(),
  endOffset: z.number(),
  chapterRevision: z.number(),
});
const factScopeSchema = z.object({
  id: z.string(),
  factId: z.string(),
  targetType: z.enum([
    "character",
    "faction",
    "location",
    "item",
    "region",
    "rule",
    "event",
  ]),
  targetId: z.string(),
});
const factConflictSchema = z.object({
  id: z.string(),
  factId: z.string(),
  otherFactId: z.string(),
  reason: z.string(),
  status: z.enum(["open", "resolved"]),
});
const characterKnowledgeSchema = z.object({
  id: z.string(),
  characterId: z.string(),
  factId: z.string(),
  state: z.enum(["known", "believed", "misinformed"]),
  beliefText: z.string(),
  availableFromChapterId: z.string(),
  notes: z.string(),
});
const continuitySchema = z.object({
  events: z.array(storyEventSchema),
  eventLinks: z.array(storyEventLinkSchema),
  facts: z.array(factSchema),
  evidence: z.array(factEvidenceSchema),
  scopes: z.array(factScopeSchema),
  conflicts: z.array(factConflictSchema),
  consistencyIssues: z.array(consistencyIssueSchema),
});

type NativeDesktopAPI = {
  ChooseProjectParent(): Promise<string>;
  ChooseProjectToOpen(): Promise<string>;
  CreateProject(input: CreateProjectInput): Promise<unknown>;
  OpenProject(path: string): Promise<unknown>;
  ListChapters(): Promise<unknown>;
  CreateChapter(title: string): Promise<unknown>;
  OpenChapter(id: string): Promise<unknown>;
  SaveDraft(id: string, body: string, revision: number): Promise<void>;
  DiscardDraft(id: string, revision: number): Promise<void>;
  CommitChapter(id: string, body: string, revision: number): Promise<unknown>;
  ChangeSets(): Promise<unknown>;
  ProposeChapterChange(input: ChapterChangeInput): Promise<unknown>;
  ProposeStructuredChange(input: StructuredChangeInput): Promise<unknown>;
  EditChangeOperation(input: ChangeOperationEditInput): Promise<unknown>;
  ApplyChangeSet(id: string): Promise<unknown>;
  ApplyChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<unknown>;
  RejectChangeSet(id: string): Promise<void>;
  RejectChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<void>;
  ChapterHistory(id: string): Promise<unknown>;
  RestoreChapterRevision(
    id: string,
    targetRevision: number,
    expectedRevision: number,
  ): Promise<unknown>;
  Snapshots(): Promise<unknown>;
  CreateSnapshot(name: string): Promise<unknown>;
  RestoreSnapshot(id: string): Promise<unknown>;
  Branches(): Promise<unknown>;
  CreateBranch(input: CreateBranchInput): Promise<unknown>;
  SwitchBranch(id: string): Promise<unknown>;
  CompareBranches(sourceId: string, targetId: string): Promise<unknown>;
  PreviewBranchMerge(sourceId: string): Promise<unknown>;
  ConfirmBranchMerge(token: string): Promise<unknown>;
  DeleteBranch(id: string): Promise<void>;
  StoryStructure(): Promise<unknown>;
  CreateOutlineNode(input: CreateOutlineNodeInput): Promise<unknown>;
  CreateStoryLine(input: CreateStoryLineInput): Promise<unknown>;
  LinkStoryLine(input: LinkStoryLineInput): Promise<unknown>;
  CreateSceneCard(input: CreateSceneCardInput): Promise<unknown>;
  ArchiveOutlineNode(id: string): Promise<void>;
  MoveOutlineNode(id: string, direction: string): Promise<void>;
  StoryWorld(): Promise<unknown>;
  CreateCharacter(input: CharacterInput): Promise<unknown>;
  UpdateCharacter(id: string, input: CharacterInput): Promise<unknown>;
  ArchiveCharacter(id: string): Promise<void>;
  CreateFaction(input: FactionInput): Promise<unknown>;
  UpdateFaction(id: string, input: FactionInput): Promise<unknown>;
  ArchiveFaction(id: string): Promise<void>;
  UpsertMembership(input: MembershipInput): Promise<unknown>;
  DeleteMembership(characterId: string, factionId: string): Promise<void>;
  CreateRelationship(input: RelationshipInput): Promise<unknown>;
  ArchiveRelationship(id: string): Promise<void>;
  LinkCharacterToChapter(input: AppearanceInput): Promise<void>;
  UnlinkCharacterFromChapter(
    characterId: string,
    chapterId: string,
  ): Promise<void>;
  LinkCharacterToScene(input: AppearanceInput): Promise<void>;
  UnlinkCharacterFromScene(
    characterId: string,
    sceneCardId: string,
  ): Promise<void>;
  WorldAssets(): Promise<unknown>;
  CreateWorldRegion(input: WorldRegionInput): Promise<unknown>;
  UpdateWorldRegion(id: string, input: WorldRegionInput): Promise<unknown>;
  ArchiveWorldRegion(id: string): Promise<void>;
  UpsertRegionAdjacency(input: RegionAdjacencyInput): Promise<unknown>;
  DeleteRegionAdjacency(firstRegionId: string, secondRegionId: string): Promise<void>;
  CreateLocation(input: LocationInput): Promise<unknown>;
  UpdateLocation(id: string, input: LocationInput): Promise<unknown>;
  ArchiveLocation(id: string): Promise<void>;
  LinkLocation(input: LocationLinkInput): Promise<unknown>;
  UnlinkLocation(
    locationId: string,
    targetType: LocationTargetType,
    targetId: string,
  ): Promise<void>;
  CreateItem(input: ItemInput): Promise<unknown>;
  UpdateItem(id: string, input: ItemInput): Promise<unknown>;
  TransferItem(input: ItemTransferInput): Promise<unknown>;
  ArchiveItem(id: string): Promise<void>;
  CreateWorldRule(input: WorldRuleInput): Promise<unknown>;
  UpdateWorldRule(id: string, input: WorldRuleInput): Promise<unknown>;
  ArchiveWorldRule(id: string): Promise<void>;
  CreateForeshadowingHook(input: ForeshadowingHookInput): Promise<unknown>;
  UpdateForeshadowingHook(
    id: string,
    input: ForeshadowingHookInput,
  ): Promise<unknown>;
  ArchiveForeshadowingHook(id: string): Promise<void>;
  Continuity(): Promise<unknown>;
  CreateStoryEvent(input: StoryEventInput): Promise<unknown>;
  UpdateStoryEvent(id: string, input: StoryEventInput): Promise<unknown>;
  ArchiveStoryEvent(id: string): Promise<void>;
  LinkStoryEvent(input: StoryEventLinkInput): Promise<unknown>;
  UnlinkStoryEvent(
    eventId: string,
    targetType: EventTargetType,
    targetId: string,
  ): Promise<void>;
  CreateFact(input: FactInput): Promise<unknown>;
  UpdateFact(id: string, input: FactInput): Promise<unknown>;
  ConfirmFact(id: string): Promise<unknown>;
  ArchiveFact(id: string): Promise<void>;
  CharacterKnowledge(): Promise<unknown>;
  CreateCharacterKnowledge(input: CharacterKnowledgeInput): Promise<unknown>;
  ArchiveCharacterKnowledge(id: string): Promise<void>;
  AddFactEvidence(input: FactEvidenceInput): Promise<unknown>;
  RemoveFactEvidence(id: string): Promise<void>;
  AddFactScope(input: FactScopeInput): Promise<unknown>;
  RemoveFactScope(id: string): Promise<void>;
  CreateFactConflict(input: FactConflictInput): Promise<unknown>;
  ResolveFactConflict(id: string): Promise<unknown>;
  DeleteFactConflict(id: string): Promise<void>;
  CompleteClose(): Promise<void>;
};

type NativeRuntime = {
  EventsOnMultiple(
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ): () => void;
};

declare global {
  interface Window {
    go?: { main?: { DesktopAPI?: NativeDesktopAPI } };
    runtime?: NativeRuntime;
  }
}

function nativeAPI(): NativeDesktopAPI {
  const api = window.go?.main?.DesktopAPI;
  if (!api) throw new Error("桌面服务尚未就绪");
  return api;
}

function projectView(value: unknown): ProjectView {
  return projectSchema.parse(value);
}

function chapterView(value: unknown): ChapterView {
  return chapterSchema.parse(value);
}

function chapterViews(value: unknown): ChapterView[] {
  return z.array(chapterSchema).parse(value);
}

const changeSetView = (value: unknown): ChangeSetView =>
  changeSetSchema.parse(value);
const changeSetViews = (value: unknown): ChangeSetView[] =>
  z.array(changeSetSchema).parse(value);
const changeSetApplyView = (value: unknown): ChangeSetApplyView =>
  changeSetApplySchema.parse(value);
const chapterRevisionViews = (value: unknown): ChapterRevisionView[] =>
  z.array(chapterRevisionSchema).parse(value);
const snapshotView = (value: unknown): SnapshotView =>
  snapshotSchema.parse(value);
const snapshotViews = (value: unknown): SnapshotView[] =>
  z.array(snapshotSchema).parse(value);
const snapshotRestoreView = (value: unknown): SnapshotRestoreView =>
  snapshotRestoreSchema.parse(value);
const branchView = (value: unknown): BranchView => branchSchema.parse(value);
const branchViews = (value: unknown): BranchView[] =>
  z.array(branchSchema).parse(value);
const branchSwitchView = (value: unknown): BranchSwitchView =>
  branchSwitchSchema.parse(value);
const branchComparisonView = (value: unknown): BranchComparisonView =>
  branchComparisonSchema.parse(value);
const branchMergePreviewView = (value: unknown): BranchMergePreviewView =>
  branchMergePreviewSchema.parse(value);

const storyStructureView = (value: unknown): StoryStructureView =>
  storyStructureSchema.parse(value);
const outlineNodeView = (value: unknown): OutlineNodeView =>
  outlineNodeSchema.parse(value);
const storyLineView = (value: unknown): StoryLineView =>
  storyLineSchema.parse(value);
const storyLineLinkView = (value: unknown): StoryLineLinkView =>
  storyLineLinkSchema.parse(value);
const sceneCardView = (value: unknown): SceneCardView =>
  sceneCardSchema.parse(value);
const storyWorldView = (value: unknown): StoryWorldView =>
  storyWorldSchema.parse(value);
const characterView = (value: unknown): CharacterView =>
  characterSchema.parse(value);
const factionView = (value: unknown): FactionView => factionSchema.parse(value);
const membershipView = (value: unknown): MembershipView =>
  membershipSchema.parse(value);
const relationshipView = (value: unknown): RelationshipView =>
  relationshipSchema.parse(value);
const worldAssetsView = (value: unknown): WorldAssetsView =>
  worldAssetsSchema.parse(value);
const worldRegionView = (value: unknown): WorldRegionView =>
  worldRegionSchema.parse(value);
const locationView = (value: unknown): LocationView =>
  locationSchema.parse(value);
const locationLinkView = (value: unknown): LocationLinkView =>
  locationLinkSchema.parse(value);
const regionAdjacencyView = (value: unknown): RegionAdjacencyView =>
  regionAdjacencySchema.parse(value);
const itemView = (value: unknown): ItemView => itemSchema.parse(value);
const itemTransferView = (value: unknown): ItemTransferView =>
  itemTransferSchema.parse(value);
const worldRuleView = (value: unknown): WorldRuleView =>
  worldRuleSchema.parse(value);
const foreshadowingHookView = (value: unknown): ForeshadowingHookView =>
  foreshadowingHookSchema.parse(value);
const continuityView = (value: unknown): ContinuityView =>
  continuitySchema.parse(value);
const storyEventView = (value: unknown): StoryEventView =>
  storyEventSchema.parse(value);
const storyEventLinkView = (value: unknown): StoryEventLinkView =>
  storyEventLinkSchema.parse(value);
const factView = (value: unknown): FactView => factSchema.parse(value);
const factEvidenceView = (value: unknown): FactEvidenceView =>
  factEvidenceSchema.parse(value);
const factScopeView = (value: unknown): FactScopeView =>
  factScopeSchema.parse(value);
const factConflictView = (value: unknown): FactConflictView =>
  factConflictSchema.parse(value);
const characterKnowledgeView = (value: unknown): CharacterKnowledgeView =>
  characterKnowledgeSchema.parse(value);
const characterKnowledgeViews = (value: unknown): CharacterKnowledgeView[] =>
  z.array(characterKnowledgeSchema).parse(value);

function safeCall<T>(operation: () => Promise<T>): Promise<T> {
  return operation().catch((error: unknown) => {
    const message = error instanceof Error ? error.message : String(error);
    const coded = message.match(/([A-Z][A-Z_]+)\|(.+)$/s);
    if (coded) {
      const [, code, safeMessage] = coded;
      if (code === "REVISION_CONFLICT") throw new RevisionConflictError();
      throw new StudioClientError(code, safeMessage);
    }
    if (/revision|版本冲突/i.test(message)) throw new RevisionConflictError();
    throw error instanceof Error ? error : new Error("桌面操作失败");
  });
}

export const wailsStudioClient: StudioClient = {
  async chooseProjectParent() {
    return (await safeCall(() => nativeAPI().ChooseProjectParent())) || null;
  },
  async chooseProjectToOpen() {
    return (await safeCall(() => nativeAPI().ChooseProjectToOpen())) || null;
  },
  async createProject(input) {
    return projectView(await safeCall(() => nativeAPI().CreateProject(input)));
  },
  async openProject(path) {
    return projectView(await safeCall(() => nativeAPI().OpenProject(path)));
  },
  async listChapters() {
    return chapterViews(await safeCall(() => nativeAPI().ListChapters()));
  },
  async createChapter(title) {
    return chapterView(await safeCall(() => nativeAPI().CreateChapter(title)));
  },
  async openChapter(id) {
    return chapterView(await safeCall(() => nativeAPI().OpenChapter(id)));
  },
  async saveDraft(id, body, revision) {
    await safeCall(() => nativeAPI().SaveDraft(id, body, revision));
  },
  async discardDraft(id, revision) {
    await safeCall(() => nativeAPI().DiscardDraft(id, revision));
  },
  async commitChapter(id, body, revision) {
    return chapterView(
      await safeCall(() => nativeAPI().CommitChapter(id, body, revision)),
    );
  },
  async changeSets() {
    return changeSetViews(await safeCall(() => nativeAPI().ChangeSets()));
  },
  async proposeChapterChange(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().ProposeChapterChange(input)),
    );
  },
  async proposeStructuredChange(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().ProposeStructuredChange(input)),
    );
  },
  async editChangeOperation(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().EditChangeOperation(input)),
    );
  },
  async applyChangeSet(id) {
    return changeSetApplyView(
      await safeCall(() => nativeAPI().ApplyChangeSet(id)),
    );
  },
  async applyChangeOperation(changeSetId, operationId) {
    return changeSetApplyView(
      await safeCall(() =>
        nativeAPI().ApplyChangeOperation(changeSetId, operationId),
      ),
    );
  },
  async rejectChangeSet(id) {
    await safeCall(() => nativeAPI().RejectChangeSet(id));
  },
  async rejectChangeOperation(changeSetId, operationId) {
    await safeCall(() =>
      nativeAPI().RejectChangeOperation(changeSetId, operationId),
    );
  },
  async chapterHistory(chapterId) {
    return chapterRevisionViews(
      await safeCall(() => nativeAPI().ChapterHistory(chapterId)),
    );
  },
  async restoreChapterRevision(
    chapterId,
    targetRevision,
    expectedRevision,
  ) {
    return chapterView(
      await safeCall(() =>
        nativeAPI().RestoreChapterRevision(
          chapterId,
          targetRevision,
          expectedRevision,
        ),
      ),
    );
  },
  async snapshots() {
    return snapshotViews(await safeCall(() => nativeAPI().Snapshots()));
  },
  async createSnapshot(name) {
    return snapshotView(
      await safeCall(() => nativeAPI().CreateSnapshot(name)),
    );
  },
  async restoreSnapshot(id) {
    return snapshotRestoreView(
      await safeCall(() => nativeAPI().RestoreSnapshot(id)),
    );
  },
  async branches() {
    return branchViews(await safeCall(() => nativeAPI().Branches()));
  },
  async createBranch(input) {
    return branchView(await safeCall(() => nativeAPI().CreateBranch(input)));
  },
  async switchBranch(id) {
    return branchSwitchView(
      await safeCall(() => nativeAPI().SwitchBranch(id)),
    );
  },
  async compareBranches(sourceId, targetId) {
    return branchComparisonView(
      await safeCall(() => nativeAPI().CompareBranches(sourceId, targetId)),
    );
  },
  async previewBranchMerge(sourceId) {
    return branchMergePreviewView(
      await safeCall(() => nativeAPI().PreviewBranchMerge(sourceId)),
    );
  },
  async confirmBranchMerge(token) {
    return branchSwitchView(
      await safeCall(() => nativeAPI().ConfirmBranchMerge(token)),
    );
  },
  async deleteBranch(id) {
    await safeCall(() => nativeAPI().DeleteBranch(id));
  },
  async storyStructure() {
    return storyStructureView(
      await safeCall(() => nativeAPI().StoryStructure()),
    );
  },
  async createOutlineNode(input) {
    return outlineNodeView(
      await safeCall(() => nativeAPI().CreateOutlineNode(input)),
    );
  },
  async createStoryLine(input) {
    return storyLineView(
      await safeCall(() => nativeAPI().CreateStoryLine(input)),
    );
  },
  async linkStoryLine(input) {
    return storyLineLinkView(
      await safeCall(() => nativeAPI().LinkStoryLine(input)),
    );
  },
  async createSceneCard(input) {
    return sceneCardView(
      await safeCall(() => nativeAPI().CreateSceneCard(input)),
    );
  },
  async archiveOutlineNode(id) {
    await safeCall(() => nativeAPI().ArchiveOutlineNode(id));
  },
  async moveOutlineNode(id, direction) {
    await safeCall(() => nativeAPI().MoveOutlineNode(id, direction));
  },
  async storyWorld() {
    return storyWorldView(await safeCall(() => nativeAPI().StoryWorld()));
  },
  async createCharacter(input) {
    return characterView(
      await safeCall(() => nativeAPI().CreateCharacter(input)),
    );
  },
  async updateCharacter(id, input) {
    return characterView(
      await safeCall(() => nativeAPI().UpdateCharacter(id, input)),
    );
  },
  async archiveCharacter(id) {
    await safeCall(() => nativeAPI().ArchiveCharacter(id));
  },
  async createFaction(input) {
    return factionView(await safeCall(() => nativeAPI().CreateFaction(input)));
  },
  async updateFaction(id, input) {
    return factionView(
      await safeCall(() => nativeAPI().UpdateFaction(id, input)),
    );
  },
  async archiveFaction(id) {
    await safeCall(() => nativeAPI().ArchiveFaction(id));
  },
  async upsertMembership(input) {
    return membershipView(
      await safeCall(() => nativeAPI().UpsertMembership(input)),
    );
  },
  async deleteMembership(characterId, factionId) {
    await safeCall(() => nativeAPI().DeleteMembership(characterId, factionId));
  },
  async createRelationship(input) {
    return relationshipView(
      await safeCall(() => nativeAPI().CreateRelationship(input)),
    );
  },
  async archiveRelationship(id) {
    await safeCall(() => nativeAPI().ArchiveRelationship(id));
  },
  async linkCharacterToChapter(input) {
    await safeCall(() => nativeAPI().LinkCharacterToChapter(input));
  },
  async unlinkCharacterFromChapter(characterId, chapterId) {
    await safeCall(() =>
      nativeAPI().UnlinkCharacterFromChapter(characterId, chapterId),
    );
  },
  async linkCharacterToScene(input) {
    await safeCall(() => nativeAPI().LinkCharacterToScene(input));
  },
  async unlinkCharacterFromScene(characterId, sceneCardId) {
    await safeCall(() =>
      nativeAPI().UnlinkCharacterFromScene(characterId, sceneCardId),
    );
  },
  async worldAssets() {
    return worldAssetsView(await safeCall(() => nativeAPI().WorldAssets()));
  },
  async createWorldRegion(input) {
    return worldRegionView(
      await safeCall(() => nativeAPI().CreateWorldRegion(input)),
    );
  },
  async updateWorldRegion(id, input) {
    return worldRegionView(
      await safeCall(() => nativeAPI().UpdateWorldRegion(id, input)),
    );
  },
  async archiveWorldRegion(id) {
    await safeCall(() => nativeAPI().ArchiveWorldRegion(id));
  },
  async upsertRegionAdjacency(input) {
    return regionAdjacencyView(
      await safeCall(() => nativeAPI().UpsertRegionAdjacency(input)),
    );
  },
  async deleteRegionAdjacency(firstRegionId, secondRegionId) {
    await safeCall(() =>
      nativeAPI().DeleteRegionAdjacency(firstRegionId, secondRegionId),
    );
  },
  async createLocation(input) {
    return locationView(
      await safeCall(() => nativeAPI().CreateLocation(input)),
    );
  },
  async updateLocation(id, input) {
    return locationView(
      await safeCall(() => nativeAPI().UpdateLocation(id, input)),
    );
  },
  async archiveLocation(id) {
    await safeCall(() => nativeAPI().ArchiveLocation(id));
  },
  async linkLocation(input) {
    return locationLinkView(
      await safeCall(() => nativeAPI().LinkLocation(input)),
    );
  },
  async unlinkLocation(locationId, targetType, targetId) {
    await safeCall(() =>
      nativeAPI().UnlinkLocation(locationId, targetType, targetId),
    );
  },
  async createItem(input) {
    return itemView(await safeCall(() => nativeAPI().CreateItem(input)));
  },
  async updateItem(id, input) {
    return itemView(await safeCall(() => nativeAPI().UpdateItem(id, input)));
  },
  async transferItem(input) {
    return itemTransferView(
      await safeCall(() => nativeAPI().TransferItem(input)),
    );
  },
  async archiveItem(id) {
    await safeCall(() => nativeAPI().ArchiveItem(id));
  },
  async createWorldRule(input) {
    return worldRuleView(
      await safeCall(() => nativeAPI().CreateWorldRule(input)),
    );
  },
  async updateWorldRule(id, input) {
    return worldRuleView(
      await safeCall(() => nativeAPI().UpdateWorldRule(id, input)),
    );
  },
  async archiveWorldRule(id) {
    await safeCall(() => nativeAPI().ArchiveWorldRule(id));
  },
  async createForeshadowingHook(input) {
    return foreshadowingHookView(
      await safeCall(() => nativeAPI().CreateForeshadowingHook(input)),
    );
  },
  async updateForeshadowingHook(id, input) {
    return foreshadowingHookView(
      await safeCall(() => nativeAPI().UpdateForeshadowingHook(id, input)),
    );
  },
  async archiveForeshadowingHook(id) {
    await safeCall(() => nativeAPI().ArchiveForeshadowingHook(id));
  },
  async continuity() {
    return continuityView(await safeCall(() => nativeAPI().Continuity()));
  },
  async createStoryEvent(input) {
    return storyEventView(
      await safeCall(() => nativeAPI().CreateStoryEvent(input)),
    );
  },
  async updateStoryEvent(id, input) {
    return storyEventView(
      await safeCall(() => nativeAPI().UpdateStoryEvent(id, input)),
    );
  },
  async archiveStoryEvent(id) {
    await safeCall(() => nativeAPI().ArchiveStoryEvent(id));
  },
  async linkStoryEvent(input) {
    return storyEventLinkView(
      await safeCall(() => nativeAPI().LinkStoryEvent(input)),
    );
  },
  async unlinkStoryEvent(eventId, targetType, targetId) {
    await safeCall(() =>
      nativeAPI().UnlinkStoryEvent(eventId, targetType, targetId),
    );
  },
  async createFact(input) {
    return factView(await safeCall(() => nativeAPI().CreateFact(input)));
  },
  async updateFact(id, input) {
    return factView(await safeCall(() => nativeAPI().UpdateFact(id, input)));
  },
  async confirmFact(id) {
    return factView(await safeCall(() => nativeAPI().ConfirmFact(id)));
  },
  async archiveFact(id) {
    await safeCall(() => nativeAPI().ArchiveFact(id));
  },
  async characterKnowledge() {
    return characterKnowledgeViews(
      await safeCall(() => nativeAPI().CharacterKnowledge()),
    );
  },
  async createCharacterKnowledge(input) {
    return characterKnowledgeView(
      await safeCall(() => nativeAPI().CreateCharacterKnowledge(input)),
    );
  },
  async archiveCharacterKnowledge(id) {
    await safeCall(() => nativeAPI().ArchiveCharacterKnowledge(id));
  },
  async addFactEvidence(input) {
    return factEvidenceView(
      await safeCall(() => nativeAPI().AddFactEvidence(input)),
    );
  },
  async removeFactEvidence(id) {
    await safeCall(() => nativeAPI().RemoveFactEvidence(id));
  },
  async addFactScope(input) {
    return factScopeView(await safeCall(() => nativeAPI().AddFactScope(input)));
  },
  async removeFactScope(id) {
    await safeCall(() => nativeAPI().RemoveFactScope(id));
  },
  async createFactConflict(input) {
    return factConflictView(
      await safeCall(() => nativeAPI().CreateFactConflict(input)),
    );
  },
  async resolveFactConflict(id) {
    return factConflictView(
      await safeCall(() => nativeAPI().ResolveFactConflict(id)),
    );
  },
  async deleteFactConflict(id) {
    await safeCall(() => nativeAPI().DeleteFactConflict(id));
  },
  onBeforeClose(handler) {
    if (!window.runtime) return () => undefined;
    return window.runtime.EventsOnMultiple(
      "app:before-close",
      () => void handler().catch(() => undefined),
      -1,
    );
  },
  async completeClose() {
    await safeCall(() => nativeAPI().CompleteClose());
  },
};
