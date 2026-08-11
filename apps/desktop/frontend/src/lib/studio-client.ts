import type {
  JobEventView,
  JobEventsPage,
  JobSnapshotView,
  JobSummaryView,
} from "../features/agents/jobEventReducer";

export type { JobEventView, JobEventsPage, JobSnapshotView, JobSummaryView };

export type ProjectView = {
  id: string;
  title: string;
  genre: string;
  summary: string;
  path: string;
  accessMode: ProjectAccessMode;
  filesystemHealth: ProjectFilesystemHealth;
};

export type ProjectAccessMode = "editable" | "read_only";
export type ProjectFilesystemHealthLevel = "normal" | "warning" | "degraded";
export type ProjectFilesystemCategory =
  | ""
  | "sync_risk"
  | "path_too_long"
  | "sharing_violation"
  | "busy"
  | "read_only"
  | "disk_full"
  | "io"
  | "checkpoint"
  | "atomic_replace"
  | "preflight";
export type ProjectFilesystemAction =
  | "retry"
  | "copy_text"
  | "open_read_only"
  | "create_recovery_copy";
export type ProjectFilesystemHealth = {
  level: ProjectFilesystemHealthLevel;
  category: ProjectFilesystemCategory;
  actions: ProjectFilesystemAction[];
};
export type ProjectOpenInput = {
  path: string;
  mode: ProjectAccessMode;
};
export type ProjectLeaseHolderView = {
  pid: number;
  hostname: string;
  applicationVersion: string;
  startedAt: string;
  projectId?: string;
};
export type ProjectLeaseConflictView = {
  path: string;
  holder?: ProjectLeaseHolderView;
  retryable: boolean;
};
export type ProjectOpenResultView =
  | { outcome: "opened"; project: ProjectView }
  | { outcome: "lease_conflict"; conflict: ProjectLeaseConflictView };
export type RecoveryCopyInput = {
  sourcePath: string;
  parentDirectory: string;
};
export type RecoveryCopyResultView =
  | { outcome: "opened"; path: string; project: ProjectView }
  | { outcome: "published"; path: string };

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
export type ChangeSetStatus =
  | "pending"
  | "partially_applied"
  | "applied"
  | "rejected"
  | "conflicted";
export type ChangeOperationKind =
  | "replace_chapter_body"
  | "update_outline_node"
  | "update_character"
  | "update_fact"
  | "create_outline_node"
  | "create_character"
  | "create_relationship"
  | "create_world_rule"
  | "create_story_event"
  | "create_fact"
  | "create_foreshadowing_hook";
export type EvidenceView = {
  chapterId: string;
  chapterTitle: string;
  startLine: number;
  endLine: number;
  excerpt: string;
};
export type ChangeOperationView = {
  id: string;
  kind: ChangeOperationKind;
  targetId: string;
  baseRevision: number;
  baseFingerprint: string;
  beforeValue: string;
  afterValue: string;
  reason: string;
  status: ChangeSetStatus;
  evidence?: EvidenceView[];
};
export type ChangeSetView = {
  id: string;
  title: string;
  summary: string;
  createdBy: string;
  status: ChangeSetStatus;
  operations: ChangeOperationView[];
  createdAt: string;
  updatedAt: string;
};
export type RevisionSource = "initial" | "author" | "agent" | "restore";
export type ChapterRevisionView = {
  chapterId: string;
  revision: number;
  title: string;
  body: string;
  source: RevisionSource;
  summary: string;
  createdAt: string;
  isCurrent: boolean;
};
export type SnapshotKind = "named" | "recovery";
export type SnapshotView = {
  id: string;
  name: string;
  kind: SnapshotKind;
  createdAt: string;
};
export type SnapshotRestoreView = {
  snapshotId: string;
  project: ProjectView;
  chapters: ChapterView[];
};
export type BranchView = {
  id: string;
  name: string;
  description: string;
  parentId: string;
  active: boolean;
  main: boolean;
  generation: number;
  createdAt: string;
  updatedAt: string;
};
export type CreateBranchInput = Pick<BranchView, "name" | "description">;
export type BranchSwitchView = {
  activeBranchId: string;
  project: ProjectView;
  chapters: ChapterView[];
};
export type BranchChapterChangeKind = "added" | "modified" | "removed";
export type BranchChapterChangeView = {
  chapterId: string;
  title: string;
  kind: BranchChapterChangeKind;
};
export type BranchDomainChangeView = {
  key: string;
  label: string;
  sourceCount: number;
  targetCount: number;
  changed: boolean;
};
export type BranchComparisonView = {
  source: BranchView;
  target: BranchView;
  addedChapters: number;
  modifiedChapters: number;
  removedChapters: number;
  chapterChanges: BranchChapterChangeView[];
  domains: BranchDomainChangeView[];
};
export type BranchMergePreviewView = {
  token: string;
  sourceId: string;
  targetId: string;
  comparison: BranchComparisonView;
};
export type ChapterChangeInput = {
  chapterId: string;
  title: string;
  summary: string;
  proposedBody: string;
  reason: string;
  createdBy: string;
};
export type OutlineContentInput = Pick<
  OutlineNodeView,
  "title" | "summary" | "goal" | "conflict" | "outcome"
>;
export type StructuredChangeOperationInput = {
  kind: Exclude<ChangeOperationKind, "replace_chapter_body">;
  targetId: string;
  reason: string;
  outline?: OutlineContentInput;
  character?: CharacterInput;
  fact?: FactInput;
};
export type StructuredChangeInput = {
  title: string;
  summary: string;
  createdBy: string;
  operations: StructuredChangeOperationInput[];
};
export type ChangeOperationEditInput = {
  changeSetId: string;
  operationId: string;
  chapterBody: string;
  outline?: OutlineContentInput;
  character?: CharacterInput;
  fact?: FactInput;
};
export type ChangeSetApplyView = {
  changeSet: ChangeSetView;
  chapters: ChapterView[];
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
export type CharacterKnowledgeState = "known" | "believed" | "misinformed";
export type CharacterKnowledgeView = {
  id: string;
  characterId: string;
  factId: string;
  state: CharacterKnowledgeState;
  beliefText: string;
  availableFromChapterId: string;
  notes: string;
};
export type CharacterKnowledgeInput = Omit<CharacterKnowledgeView, "id">;
export type ContinuityView = {
  events: StoryEventView[];
  eventLinks: StoryEventLinkView[];
  facts: FactView[];
  evidence: FactEvidenceView[];
  scopes: FactScopeView[];
  conflicts: FactConflictView[];
  consistencyIssues: ConsistencyIssueView[];
};

export type ProviderKind = "openai" | "anthropic";
export type ProviderConnectionInput = {
  id?: string;
  name: string;
  kind: ProviderKind;
  baseUrl: string;
  apiKey: string;
};
export type ProviderConnectionView = {
  id: string;
  name: string;
  kind: ProviderKind;
  baseUrl: string;
  hasCredential: boolean;
  updatedAt: string;
};
export type AgentRole = "planner" | "writer" | "continuity_reviewer";
export type AgentProfileInput = {
  role: AgentRole;
  connectionId: string;
  model: string;
  maxOutputTokens: number;
  temperature: number;
  enabled: boolean;
};
export type AgentProfileView = AgentProfileInput & {
  id: string;
  updatedAt: string;
};
export type AgentRuntimeSettingsInput = {
  maxRunTokens: number;
  maxContextChars: number;
};
export type AgentRuntimeSettingsView = AgentRuntimeSettingsInput & {
  updatedAt: string;
};
export type AgentRunStatus =
  | "queued"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "recoverable";
export type AgentRunStepView = {
  id: string;
  role: AgentRole;
  position: number;
  status: "queued" | "running" | "completed" | "failed" | "cancelled";
  output: string;
  errorCode: string;
  errorMessage: string;
  providerRequestId: string;
  inputTokens: number;
  outputTokens: number;
};
export type AgentRunView = {
  id: string;
  chapterId: string;
  prompt: string;
  status: AgentRunStatus;
  errorCode: string;
  errorMessage: string;
  changeSetId: string;
  retryOf: string;
  tokenBudget: number;
  contextCharBudget: number;
  inputTokens: number;
  outputTokens: number;
  contextChars: number;
  contextTruncated: boolean;
  steps: AgentRunStepView[];
  createdAt: string;
  updatedAt: string;
};

export type ImportJobStatus = "prepared" | "applied" | "failed";
export type ImportSourceFormat = "txt" | "markdown";
export type ImportChapterView = {
  id: string;
  volumeTitle: string;
  title: string;
  wordCount: number;
  bodyPreview: string;
  position: number;
  sourceStartLine: number;
  sourceEndLine: number;
  contentHashSummary: string;
  selected: boolean;
  createdChapterId: string;
};
export type ImportJobView = {
  id: string;
  status: ImportJobStatus;
  sourceName: string;
  sourceFormat: ImportSourceFormat;
  sourceEncoding: string;
  sourceHashDigest: string;
  sourceBytes: number;
  chapterCount: number;
  selectedChapterCount: number;
  errorCode: string;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
  appliedAt: string;
  extractionStatus: string;
  extractionRunId: string;
  chapters: ImportChapterView[];
};
export type ImportPrepareView = {
  job: ImportJobView;
};
export type ImportApplyView = {
  job: ImportJobView;
  chapters: ChapterView[];
};

export type ExtractionStatus =
  | "none"
  | "running"
  | "completed"
  | "failed";
export type ExtractionRunInput = {
  importJobId: string;
  connectionId: string;
  model: string;
  instruction: string;
};
export type ExtractionStepView = {
  id: string;
  importChapterId: string;
  createdChapterId: string;
  chapterTitle: string;
  position: number;
  status: "queued" | "running" | "completed" | "failed" | "cancelled";
  output: string;
  errorCode: string;
  errorMessage: string;
  providerRequestId: string;
  inputTokens: number;
  outputTokens: number;
};
export type ExtractionTaskView = {
  id: string;
  importJobId: string;
  connectionId: string;
  model: string;
  prompt: string;
  status: "queued" | "running" | "completed" | "failed" | "cancelled" | "recoverable";
  errorCode: string;
  errorMessage: string;
  changeSetId: string;
  retryOf: string;
  tokenBudget: number;
  contextCharBudget: number;
  inputTokens: number;
  outputTokens: number;
  steps: ExtractionStepView[];
  createdAt: string;
  updatedAt: string;
};
export type ExtractionRunView = {
  task: ExtractionTaskView;
  changeSet: ChangeSetView | null;
};

export interface StudioClient {
  chooseProjectParent(): Promise<string | null>;
  chooseProjectToOpen(): Promise<string | null>;
  chooseManuscriptToImport(): Promise<string | null>;
  prepareManuscriptImport(sourcePath: string): Promise<ImportPrepareView>;
  importJobs(): Promise<ImportJobView[]>;
  importJob(id: string): Promise<ImportJobView>;
  deleteImportJob(id: string): Promise<void>;
  applyImportJob(
    id: string,
    selectedChapterIds: string[],
  ): Promise<ImportApplyView>;
  extractionTasks(): Promise<ExtractionTaskView[]>;
  extractionTask(id: string): Promise<ExtractionRunView>;
  startExtractionRun(input: ExtractionRunInput): Promise<ExtractionRunView>;
  executeExtractionRun(id: string): Promise<ExtractionRunView>;
  cancelExtractionRun(id: string): Promise<void>;
  retryExtractionRun(id: string): Promise<ExtractionRunView>;
  createProject(input: CreateProjectInput): Promise<ProjectView>;
  openProject(
    path: string,
    mode?: ProjectAccessMode,
  ): Promise<ProjectOpenResultView>;
  createProjectRecoveryCopy(
    input: RecoveryCopyInput,
  ): Promise<RecoveryCopyResultView>;
  filesystemHealth(): Promise<ProjectFilesystemHealth>;
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
  changeSets(): Promise<ChangeSetView[]>;
  proposeChapterChange(input: ChapterChangeInput): Promise<ChangeSetView>;
  proposeStructuredChange(input: StructuredChangeInput): Promise<ChangeSetView>;
  editChangeOperation(input: ChangeOperationEditInput): Promise<ChangeSetView>;
  applyChangeSet(id: string): Promise<ChangeSetApplyView>;
  applyChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<ChangeSetApplyView>;
  rejectChangeSet(id: string): Promise<void>;
  rejectChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<void>;
  chapterHistory(chapterId: string): Promise<ChapterRevisionView[]>;
  restoreChapterRevision(
    chapterId: string,
    targetRevision: number,
    expectedRevision: number,
  ): Promise<ChapterView>;
  snapshots(): Promise<SnapshotView[]>;
  createSnapshot(name: string): Promise<SnapshotView>;
  restoreSnapshot(id: string): Promise<SnapshotRestoreView>;
  branches(): Promise<BranchView[]>;
  createBranch(input: CreateBranchInput): Promise<BranchView>;
  switchBranch(id: string): Promise<BranchSwitchView>;
  compareBranches(
    sourceId: string,
    targetId: string,
  ): Promise<BranchComparisonView>;
  previewBranchMerge(sourceId: string): Promise<BranchMergePreviewView>;
  confirmBranchMerge(token: string): Promise<BranchSwitchView>;
  deleteBranch(id: string): Promise<void>;
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
  characterKnowledge(): Promise<CharacterKnowledgeView[]>;
  createCharacterKnowledge(
    input: CharacterKnowledgeInput,
  ): Promise<CharacterKnowledgeView>;
  archiveCharacterKnowledge(id: string): Promise<void>;
  addFactEvidence(input: FactEvidenceInput): Promise<FactEvidenceView>;
  removeFactEvidence(id: string): Promise<void>;
  addFactScope(input: FactScopeInput): Promise<FactScopeView>;
  removeFactScope(id: string): Promise<void>;
  createFactConflict(input: FactConflictInput): Promise<FactConflictView>;
  resolveFactConflict(id: string): Promise<FactConflictView>;
  deleteFactConflict(id: string): Promise<void>;
  providerConnections(): Promise<ProviderConnectionView[]>;
  saveProviderConnection(
    input: ProviderConnectionInput,
  ): Promise<ProviderConnectionView>;
  deleteProviderConnection(id: string): Promise<void>;
  testProviderConnection(id: string): Promise<void>;
  agentProfiles(): Promise<AgentProfileView[]>;
  saveAgentProfile(input: AgentProfileInput): Promise<AgentProfileView>;
  agentRuntimeSettings(): Promise<AgentRuntimeSettingsView>;
  saveAgentRuntimeSettings(
    input: AgentRuntimeSettingsInput,
  ): Promise<AgentRuntimeSettingsView>;
  createChapterAgentRun(
    chapterId: string,
    prompt: string,
  ): Promise<AgentRunView>;
  executeAgentRun(id: string): Promise<AgentRunView>;
  cancelAgentRun(id: string): Promise<void>;
  retryAgentRun(id: string): Promise<AgentRunView>;
  agentRuns(): Promise<AgentRunView[]>;
  agentRun(id: string): Promise<AgentRunView>;
  getJobSnapshot(jobId: string): Promise<JobSnapshotView>;
  getJobEvents(
    jobId: string,
    afterSequence: number,
    limit: number,
  ): Promise<JobEventsPage>;
  listRecoverableJobs(): Promise<JobSummaryView[]>;
  onJobUpdated(handler: (jobId: string, jobKind: string, latestSequence: number) => void): () => void;
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
