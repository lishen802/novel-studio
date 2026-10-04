import type { RevisionCard } from '../components/creation/RevisionPanel';
export interface ConstraintChange {candidateId?:string;assetId:string;revision:number;chapterId:string;position:number;text:string;reason:string}
export interface CharacterSuggestion {branchId?:string;candidateId:string;entityId:string;text:string;actionId:string}
export interface PlanningField {path:string;label:string;value:string}
export interface ConstraintView {fields?:PlanningField[];candidateSuggestions?:CharacterSuggestion[];assetId:string;typedId?:string;kind:string;name:string;original:string;current:string;revision:number;locked:boolean;changes:ConstraintChange[]}
export interface ConstraintRequest {candidateId?:string;projectId?:string;branchId?:string;assetId:string;intent:string;chapterId:string;text:string;reason:string}
export interface PlanningRearchivePreview {request:{projectId:string;branchId:string;chapterId:string;reason:string};digest:string;position:number;protectedChapters:number;futureChapters:number;changes:Array<{assetId:string;kind:string;before:string;after:string}>}
export interface FormalOutlineAppendRequest {projectId?:string;branchId?:string;parentAssetId:string;chapterId:string;title:string;summary:string;goal:string;conflict:string;outcome:string;reason:string}
export interface FormalOutlineAppendPreview {request:FormalOutlineAppendRequest;digest:string;position:number;nodePosition:number;parentTitle:string;assetId?:string;nodeId?:string}
export interface FormalOutlineRetireRequest {projectId:string;branchId:string;assetId:string;chapterId:string;reason:string;unlinkStoryLineId?:string;unlinkStoryLineIds?:string[];rebindStoryLineId?:string;rebindStoryLineIds?:string[];rebindTargetAssetId?:string}
export interface FormalOutlineRetirePreview {request:FormalOutlineRetireRequest;digest:string;nodeId:string;title:string;introducedAt:number;position:number;unlinkedStoryLine?:{storyLineId:string;name:string;position:number;createdAt:string};unlinkedStoryLines?:Array<{storyLineId:string;name:string;position:number;createdAt:string}>;reboundStoryLine?:{storyLineId:string;name:string;position:number;createdAt:string;targetAssetId:string;targetNodeId:string;targetTitle:string;targetRevision:number;targetParentId:string;targetIntroducedAt:number};reboundStoryLines?:Array<{storyLineId:string;name:string;position:number;createdAt:string;targetAssetId:string;targetNodeId:string;targetTitle:string;targetRevision:number;targetParentId:string;targetIntroducedAt:number}>}
export interface FormalOutlineReferenceView {assetId:string;nodeId:string;title:string;references:Array<{kind:string;id:string;label:string}>}
export interface FormalOutlineReorderRequest {projectId:string;branchId:string;firstAssetId:string;secondAssetId:string;chapterId:string;reason:string}
export interface FormalOutlineReorderPreview {request:FormalOutlineReorderRequest;digest:string;firstNodeId:string;secondNodeId:string;firstTitle:string;secondTitle:string;parentNodeId:string;firstPosition:number;secondPosition:number;position:number;firstRevision:number;secondRevision:number}
export interface ConstraintPreview {request:ConstraintRequest;digest:string;before:string;revision:number;position:number;protectedChapters:number;futureChapters:number;canApply:boolean;blockedReason:string}
export interface ConstraintCommand {request:ConstraintRequest;digest:string;idempotencyKey:string}
export type ProjectAccessMode = 'editable' | 'read_only';

export interface WorkbenchProjection {
  stage: 'idea' | 'proposal' | 'roadmap' | 'batch_running' | 'review' | 'writing';
  canvas: string;
  aiPanel: string;
  pendingReviewCount: number;
  recoverableJobCount: number;
  archiveStatus: string;
}

export interface ProjectView {
  id: string;
  title: string;
  genre: string;
  summary: string;
  path: string;
  accessMode: ProjectAccessMode;
  profileSelection: { profileId?: string; version?: number; roleOverrides?: Record<string, string> };
  workbench: WorkbenchProjection;
}

export interface ChapterRevisionView {
 chapterId:string; revision:number; title:string; body:string; source:string; summary:string; createdAt:string; isCurrent:boolean;
}
export interface ChapterView {
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
}

export interface RecentProjectView {
  projectId: string;
  title: string;
  path: string;
  lastWorkspace: string;
  lastOpenedAt: string;
  missing: boolean;
}

export interface ProviderView {
  id: string;
  name: string;
  kind: string;
  baseUrl: string;
  hasCredential: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ProviderTestView {
  providerId: string;
  providerName: string;
  providerKind: string;
  model: string;
  status: string;
  latencyMs: number;
  providerRequestId: string;
  inputTokens: number;
  outputTokens: number;
  testedAt: string;
}

export interface RoleBindingView {
  role: string;
  connectionId: string;
  model: string;
  maxOutputTokens: number;
  reasoningEffort?: string;
  outputLimitMode?: string;
  temperature: number;
  enabled: boolean;
}

export interface CreativeProfileView {
  profileId: string;
  version: number;
  name: string;
  roles: RoleBindingView[];
  maxRunTokens: number;
  maxContextChars: number;
  runBudgetUnlimited?: boolean;
  contextBudgetUnlimited?: boolean;
  createdAt: string;
}

export interface ApplicationBootstrapView {
  preferences: { theme: string; defaultCreativeProfileId: string; updatedAt: string };
  recentProjects: RecentProjectView[];
  providers: ProviderView[];
  profiles: CreativeProfileView[];
}

export interface ProjectOpenResultView {
  outcome: 'opened' | 'lease_conflict';
  project?: ProjectView;
  conflict?: { path: string; retryable: boolean; holder?: Record<string, unknown> };
}

export interface SnapshotView {
  id: string;
  name: string;
  kind: string;
  createdAt: string;
}

export interface ChangeOperationView {
  id: string;
  kind: string;
  targetId: string;
  beforeValue: string;
  afterValue: string;
  reason: string;
  status: string;
}

export interface ChangeSetView {
  id: string;
  title: string;
  summary: string;
  createdBy: string;
  status: string;
  operations: ChangeOperationView[];
  createdAt: string;
  updatedAt: string;
}

export interface StoryStructureView {
  outlineNodes: Array<Record<string, unknown>>;
  storyLines: Array<Record<string, unknown>>;
  storyLineLinks: Array<Record<string, unknown>>;
  sceneCards: Array<Record<string, unknown>>;
}

export interface StoryWorldView {
  characters: Array<Record<string, unknown>>;
  factions: Array<Record<string, unknown>>;
  memberships: Array<Record<string, unknown>>;
  relationships: Array<Record<string, unknown>>;
  chapterAppearances: Array<Record<string, unknown>>;
  sceneAppearances: Array<Record<string, unknown>>;
  consistencyIssues: Array<Record<string, unknown>>;
}

export interface WorldAssetsView {
  regions: Array<Record<string, unknown>>;
  regionAdjacencies: Array<Record<string, unknown>>;
  locations: Array<Record<string, unknown>>;
  locationLinks: Array<Record<string, unknown>>;
  items: Array<Record<string, unknown>>;
  itemTransfers: Array<Record<string, unknown>>;
  rules: Array<Record<string, unknown>>;
  hooks: Array<Record<string, unknown>>;
  consistencyIssues: Array<Record<string, unknown>>;
}

export interface ContinuityView {
  events: Array<Record<string, unknown>>;
  eventLinks: Array<Record<string, unknown>>;
  facts: Array<Record<string, unknown>>;
  evidence: Array<Record<string, unknown>>;
  scopes: Array<Record<string, unknown>>;
  conflicts: Array<Record<string, unknown>>;
  consistencyIssues: Array<Record<string, unknown>>;
}

export interface AgentRunStepView {
  id: string;
  role: string;
  position: number;
  status: string;
  output: string;
  errorCode: string;
  errorMessage: string;
  providerRequestId: string;
  inputTokens: number;
  outputTokens: number;
  // 冻结快照字段：步骤实际使用的模型与 Provider。
  providerName: string;
  providerKind: string;
  model: string;
  maxOutputTokens: number;
  reasoningEffort?: string;
  outputLimitMode?: string;
  temperature: number;
}

export interface AgentRunView {
  id: string;
  chapterId: string;
  prompt: string;
  status: string;
  errorCode: string;
  errorMessage: string;
  changeSetId: string;
  retryOf: string;
  tokenBudget: number;
  contextCharBudget: number;
  tokenBudgetUnlimited?: boolean;
  contextBudgetUnlimited?: boolean;
  inputTokens: number;
  outputTokens: number;
  contextChars: number;
  contextTruncated: boolean;
  steps: AgentRunStepView[];
  createdAt: string;
  updatedAt: string;
}

export interface JobSnapshotView {
  jobId: string;
  jobKind: string;
  projectId: string;
  status: string;
  maxSequence: number;
  earliestReplayableSequence: number;
  inputTokens: number;
  outputTokens: number;
  pendingAction: string;
  terminalInfo: string;
  steps: Array<Record<string, unknown>>;
  publicStageId: string;
  completedSteps: number;
  totalSteps: number;
  qualityStatus: string;
  recoverable: boolean;
  candidateAvailable: boolean;
  updatedAt: string;
}

export interface JobEventsPageView {
  events: Array<Record<string, unknown>>;
  nextCursor: number;
  hasMore: boolean;
  snapshotRequired: boolean;
  earliestReplayableSequence: number;
}

export interface HarnessDatasetView {
  dataset: { dataset_id: string; version: string; sha256: string; originality_note: string; cases: Array<Record<string, unknown>> };
  variants: Array<{ id: string; workflow_id: string; workflow_version: number; default_enabled: boolean }>;
}

export interface HarnessPreviewView {
  datasetId: string; datasetVersion: string; datasetSha256: string; caseCount: number;
  estimatedCalls: number; maximumOutputTokens: number; currencyEstimate: string;
  runBudgetUnlimited?: boolean; contextBudgetUnlimited?: boolean;
  maxRunTokens?: number; maxContextChars?: number;
  provider: string; model: string; workflowId: string; workflowVersion: number;
  confirmationDigest: string; binding: Record<string, unknown>;
  roles?: Array<{ role: string; provider: string; model: string; reasoning_effort?: string }>;
}

export interface HarnessRunView {
  run: { id: string; status: string; case_ids: string[]; failure?: string; binding: Record<string, unknown>; started_at: string; retry_of?: string; current_case_id?: string; public_stage_id?: string; input_tokens?: number; output_tokens?: number; calls?: number };
  completedCases: number; completedCaseIds?: string[]; totalCases: number; report?: Record<string, unknown>;
  ratings?: { submissions: number; candidateRatings: number; acceptable: number; rewrite: number; dimensionAverage: Record<string, number> };
}

export interface HarnessCaseView {
  case: { case_id: string; genre: string; task_kind: string; rating_dimensions: string[]; chapter_goal: string };
  candidates: Array<{ slot: string; content: string; metrics: Record<string, unknown>; quality_report_json?: string }>;
  rating?: Record<string, unknown>; revealed?: Record<string, string>; binding?: Record<string, unknown>;
}

export interface CreationSampleInput {
 pointOfView: string; tone: string; constraints: string[]; targetRunes: number;
}

export interface RoadmapGoalItemView {
  id: string;
  statement: string;
  detail: string;
}

export interface RoadmapNodeView {
  parentId?: string;
  id: string;
  title: string;
  summary: string;
  position: number;
  readerExpectation?: string;
  stagePayoff?: string;
  irreversibleChange?: string;
  endingContribution?: string;
  commitments?: RoadmapGoalItemView[];
  retainedQuestions?: RoadmapGoalItemView[];
  // 逐项审阅状态：接受与排除都由后端投影给出，界面只做展示。
  accepted: boolean;
  rejected: boolean;
}

// 逐项审阅单位：方案分节用 section 作用域，人物世界实体与路线图节点用 entity 作用域。
export interface ReviewRefView {
  scope: string;
  id: string;
}

export interface CreationReviewView {
  candidateId: string;
  stage: string;
  reviewVersion: number;
  accepted: ReviewRefView[];
  rejected: ReviewRefView[];
  unreviewed: ReviewRefView[];
  complete: boolean;
  updatedAt: string;
}

export interface CreationReviewCursorView {
  stage: string;
  candidateId: string;
  entityId: string;
  sectionId: string;
  comparisonRef: string;
  viewVersion: number;
  updatedAt: string;
}

export interface CandidateVersionSummaryView {
  candidateId: string;
  version: number;
  kind: string;
  origin: string;
  status: string;
  quality: string;
  summary: string;
  createdAt: string;
  current: boolean;
}

export interface CandidateReviewDiff {
  path: string;
  label: string;
  group: string;
  before: string;
  after: string;
}

export interface CandidateVersionView extends CandidateVersionSummaryView {
  diff: CandidateReviewDiff[];
}

export interface CandidateCompareView {
  stage: string;
  baseCandidateId: string;
  versions: CandidateVersionView[];
}

// 结构增删入口的投影：集合用顶层字段名寻址，实体的 key 是稳定标识（id 或唯一名称）。
export interface StructureEntityView {
  key: string;
  title: string;
}

export interface StructureCollectionView {
  collection: string;
  title: string;
  identityField: string;
  entities: StructureEntityView[];
}

// 结构增删命令：entityKey 必须等于 value 内标识字段（identityField）的值，删除时 value 为空串。
export interface StructureEditCommand {
  candidateId: string;
  collection: string;
  operation: 'add' | 'remove';
  entityKey: string;
  value: string;
}

export interface CreationWorkbenchView {
 revisions?: RevisionCard[];
 directionActions?: DirectionActionView[];
 fieldEditor?: { candidateId: string; version: number; fields: Array<{path:string;ownerRef?:string;label:string;group:string;value:string;locked:boolean}> };
 canPrepareSample?: boolean;
 sampleHandoff?: (CreationSampleInput & { id: string; directionCandidateId: string; directionCardId: string; status: string }) | null;
  contentVersion?: number;
  runSlotVersion?: number;
  entry?: { id: string; version: number; kind: 'idea' | 'author_plan'; material: string };
  material?: { analysisId: string; status: string; availableStages: string[]; gaps: Array<{ id: string; stage: string; field_path: string; kind: string; reason: string; suggested_action: string; quote: string }> };

	readOnly?: boolean;
	applicability?: string;
  sessionId: string;
  projectId: string;
  branchId: string;
  stage: string;
  advanceMode: string;
  blockingReason: string;
  recoverable: boolean;
  archiveConfirmed: boolean;
  intentText: string;
  stageRail: Array<{ id: string; label: string; status: string }>;
  directionSet: {
	keptIds?: string[];
	combined?: Array<{ candidateId: string; version: number; card: CreationDirectionCardView }>;
    candidateId: string;
    selectedCandidateId?: string;
    version: number;
    selectedId: string;
    status: string;
    cards: CreationDirectionCardView[];
  };
  proposal: {
    candidateId: string;
    version: number;
    status: string;
    title: string;
    sections: Array<{ id: string; title: string; content: string; lockPath: string; locked: boolean; accepted: boolean; rejected: boolean }>;
  };
  peopleWorld: {
    candidateId: string;
    version: number;
    status: string;
    entities: Array<{ id: string; kind: string; title: string; summary: string; complete: boolean; accepted: boolean; rejected: boolean }>;
  };
  roadmap: {
    candidateId: string;
    version: number;
    status: string;
    book: RoadmapNodeView;
    volumes: RoadmapNodeView[];
    arcs: RoadmapNodeView[];
    chapters: RoadmapNodeView[];
  };
  currentRun: { runId: string; status: string; cancellable: boolean; publicStage: string; steps?: Array<{stage: string; status: string}>; createdAt?: string; updatedAt?: string };
  assistant: { pendingActionCount: number; messages?: AssistantMessageView[]; authorization?: DiscussionAuthorizationView; authorizations?: DiscussionAuthorizationView[]; pendingDiscussion?: DiscussionTurnView };
  impact: {
    items: Array<{ candidateId: string; stage: string; severity: string; reason: string; locked: boolean; confirmed: boolean; excepted: boolean }>;
    touchesLocked: boolean;
    touchesConfirmed: boolean;
    canAdvance: boolean;
    regenerateStages: string[];
    estimatedCalls: number;
    evidence: string;
  };
  capabilities: string[];
  // 可增删集合的投影；空集合只出现在该阶段支持的集合上，未支持的集合不出现。
  structure: StructureCollectionView[];
  // 逐项审阅投影、作者审阅位置与当前阶段候选版本列表。
  review: CreationReviewView;
  reviewCursor: CreationReviewCursorView;
  candidateVersions: CandidateVersionSummaryView[];
}

export interface CreationDirectionCardView {
  id: string; hook: string; genre: string; protagonist: string; coreConflict: string;
  growthCost: string; pacing: string; ending: string; selected: boolean;
}

export interface DirectionActionView {
  actionId: string; argumentsHash: string; idempotencyKey: string; mode: 'regenerate_one' | 'combine';
  candidateId: string; candidateVersion: number; targetCardId?: string; sourceCardIds?: string[];
  instruction: string; state: string; cost: CreationCostPreview; runId?: string;
  resultCandidateId?: string; resultCardId?: string; summary?: string; stale: boolean;
}

export interface AssistantMessageView {
  id: string; role: 'author' | 'assistant'; kind?: string; status?: string; text: string; stage: string;
  candidateId: string; entityRef: string; fieldPath: string; actionId: string;
  actionState: string; authorizationId?: string; runId?: string; replyToId?: string; errorCode?: string;
  suggestions?: DiscussionSuggestionView[]; createdAt: string;
}

export interface DiscussionSuggestionView {
  id: string; paths: string[]; instruction: string; preservePaths?: string[]; rationale: string;
}

export interface DiscussionAuthorizationView {
  id?: string; actionId: string; state: string; candidateId: string; candidateVersion: number;
  entityRef?: string; fieldPaths?: string[]; remainingCalls: number; remainingTokens: number;
  maxCalls: number; maxTotalTokens: number; maxTurnTokens: number; expiresAt: string;
  provider: string; model: string; cost?: CreationCostPreview; stale?: boolean;
}

export interface DiscussionTurnView {
  authorization: DiscussionAuthorizationView;
  actionId: string; argumentsHash: string; idempotencyKey: string; state: string; runId?: string;
	draftMessage?: string;
}

export interface CreationCostPreview {
	workflowVersion?: number;
	roleModels?: Array<{ role: string; provider: string; model: string }>;
  stage: string;
  workflowId: string;
  estimatedCalls: number;
  maxRunTokens: number;
  runBudgetUnlimited: boolean;
  remainingStages: string[];
  confirmationDigest: string;
}

type NativeMethod = (...args: unknown[]) => Promise<unknown>;
type NativeDesktopAPI = Record<string, NativeMethod>;

declare global {
  interface Window {
    go?: { main?: { DesktopAPI?: NativeDesktopAPI } };
  }
}

export class StudioClientError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
    this.name = 'StudioClientError';
  }
}

function nativeAPI(): NativeDesktopAPI {
  const api = window.go?.main?.DesktopAPI;
  if (!api) throw new StudioClientError('DESKTOP_UNAVAILABLE', '桌面服务尚未就绪');
  return api;
}

async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  const operation = nativeAPI()[method];
  if (typeof operation !== 'function') {
    throw new StudioClientError('FEATURE_NOT_AVAILABLE', `桌面功能 ${method} 尚未就绪`);
  }
  try {
    return (await operation(...args)) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    const coded = message.match(/([A-Z][A-Z_]+)\|(.+)$/s);
    if (coded) throw new StudioClientError(coded[1], coded[2]);
    throw error instanceof Error ? error : new Error('桌面操作失败');
  }
}

export const studio = {
  available: () => Boolean(window.go?.main?.DesktopAPI),
  completeClose: () => call<void>('CompleteClose'),
  applicationBootstrap: () => call<ApplicationBootstrapView>('ApplicationBootstrap'),
  saveApplicationPreferences: (input: Record<string, unknown>) => call<void>('SaveApplicationPreferences', input),
  saveApplicationProvider: (input: Record<string, unknown>) => call<ProviderView>('SaveApplicationProvider', input),
  deleteApplicationProvider: (id: string) => call<void>('DeleteApplicationProvider', id),
  testApplicationProvider: (input: { providerId: string; model: string }) =>
    call<ProviderTestView>('TestApplicationProvider', input),
  publishCreativeProfile: (input: Record<string, unknown>) => call<CreativeProfileView>('PublishCreativeProfile', input),
  getProfileVersion: (profileId: string, version: number) =>
    call<CreativeProfileView>('GetProfileVersion', profileId, version),

  chooseProjectParent: () => call<string>('ChooseProjectParent'),
  chooseProjectToOpen: () => call<string>('ChooseProjectToOpen'),
  chooseManuscriptToImport: () => call<string>('ChooseManuscriptToImport'),
  createProject: (input: Record<string, unknown>) => call<ProjectView>('CreateProject', input),
  openProject: (path: string, mode: ProjectAccessMode = 'editable') =>
    call<ProjectOpenResultView>('OpenProject', { path, mode }),
  openRecentProject: (projectId: string) => call<ProjectOpenResultView>('OpenRecentProject', projectId),
  closeProject: () => call<void>('CloseProject'),
  projectWorkbench: () => call<WorkbenchProjection>('ProjectWorkbench'),

  listChapters: () => call<ChapterView[]>('ListChapters'),
  createChapter: (title: string) => call<ChapterView>('CreateChapter', title),
  openChapter: (id: string) => call<ChapterView>('OpenChapter', id),
  saveDraft: (id: string, body: string, revision: number) => call<void>('SaveDraft', id, body, revision),
  discardDraft: (id: string, revision: number) => call<void>('DiscardDraft', id, revision),
  commitChapter: (id: string, body: string, revision: number) =>
    call<ChapterView>('CommitChapter', id, body, revision),
  chapterHistory: (id: string) => call<ChapterRevisionView[]>('ChapterHistory', id),
  restoreChapterRevision: (id:string,target:number,expected:number) => call<ChapterView>('RestoreChapterRevision',id,target,expected),

  changeSets: () => call<ChangeSetView[]>('ChangeSets'),
  applyChangeSet: (id: string) => call<Record<string, unknown>>('ApplyChangeSet', id),
  rejectChangeSet: (id: string) => call<void>('RejectChangeSet', id),
  applyChangeOperation: (changeSetId: string, operationId: string) =>
    call<Record<string, unknown>>('ApplyChangeOperation', changeSetId, operationId),
  rejectChangeOperation: (changeSetId: string, operationId: string) =>
    call<void>('RejectChangeOperation', changeSetId, operationId),

  exportManuscript: (format:string) => call<{path:string;chapterCount:number;draftCount:number}>('ExportManuscript',format),
  snapshots: () => call<SnapshotView[]>('Snapshots'),
  createSnapshot: (name: string) => call<SnapshotView>('CreateSnapshot', name),
  restoreSnapshot: (id: string) => call<{project:ProjectView;chapters:ChapterView[]}>('RestoreSnapshot', id),
  branches: () => call<Array<Record<string, unknown>>>('Branches'),
  createBranch: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateBranch', input),
  switchBranch: (id: string) => call<Record<string, unknown>>('SwitchBranch', id),

  storyStructure: () => call<StoryStructureView>('StoryStructure'),
  createOutlineNode: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateOutlineNode', input),
  previewFormalOutlineAppend: (input:FormalOutlineAppendRequest) => call<FormalOutlineAppendPreview>('PreviewFormalOutlineAppend',input),
  confirmFormalOutlineAppend: (command:{request:FormalOutlineAppendRequest;digest:string;idempotencyKey:string}) => call<FormalOutlineAppendPreview>('ConfirmFormalOutlineAppend',command),
  previewFormalOutlineRetire: (assetId:string,chapterId:string,reason:string,unlinkStoryLineId:string,unlinkStoryLineIds:string[],rebindStoryLineId:string,rebindTargetAssetId:string,rebindStoryLineIds:string[]) => call<FormalOutlineRetirePreview>('PreviewFormalOutlineRetire',assetId,chapterId,reason,unlinkStoryLineId,unlinkStoryLineIds,rebindStoryLineId,rebindTargetAssetId,rebindStoryLineIds),
  formalOutlineReferences: (assetId:string) => call<FormalOutlineReferenceView>('FormalOutlineReferences',assetId),
  confirmFormalOutlineRetire: (command:{request:FormalOutlineRetireRequest;digest:string;idempotencyKey:string}) => call<FormalOutlineRetirePreview>('ConfirmFormalOutlineRetire',command),
  previewFormalOutlineReorder: (firstAssetId:string,secondAssetId:string,chapterId:string,reason:string) => call<FormalOutlineReorderPreview>('PreviewFormalOutlineReorder',firstAssetId,secondAssetId,chapterId,reason),
  confirmFormalOutlineReorder: (command:{request:FormalOutlineReorderRequest;digest:string;idempotencyKey:string}) => call<FormalOutlineReorderPreview>('ConfirmFormalOutlineReorder',command),
  formalOutlinePositionsAtChapter: (chapterId:string) => call<Record<string,number>>('FormalOutlinePositionsAtChapter',chapterId),
  createStoryLine: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateStoryLine', input),
  createSceneCard: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateSceneCard', input),
  storyWorld: () => call<StoryWorldView>('StoryWorld'),
  createCharacter: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateCharacter', input),
  updateCharacter: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateCharacter', id, input),
  createFaction: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateFaction', input),
  updateFaction: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateFaction', id, input),
  planningRevisions: () => call<ConstraintView[]>('PlanningRevisions'),
  previewCharacterRevision: (request:{assetId:string;expectedRevision:number;field:string;instruction:string;idempotencyKey:string}) => call<RevisionCard>('PreviewCharacterRevision',request),
  characterRevisionActions: () => call<RevisionCard[]>('CharacterRevisionActions'),
  characterRevisions: () => call<ConstraintView[]>('CharacterRevisions'),
  constraints: () => call<ConstraintView[]>('Constraints'),
  previewConstraintChange: (request:ConstraintRequest) => call<ConstraintPreview>('PreviewConstraintChange',request),
  applyConstraintChange: (command:ConstraintCommand) => call<ConstraintPreview>('ApplyConstraintChange',command),
  worldAssets: () => call<WorldAssetsView>('WorldAssets'),
  createWorldRegion: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateWorldRegion', input),
  createLocation: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateLocation', input),
  createWorldRule: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateWorldRule', input),
  createForeshadowingHook: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateForeshadowingHook', input),
  updateForeshadowingHookStatus: (id: string, status: string, resolvedChapterId = '') => call<Record<string, unknown>>('UpdateForeshadowingHookStatus', id, { status, resolvedChapterId }),
  updateForeshadowingHook: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateForeshadowingHook', id, input),
  continuity: () => call<ContinuityView>('Continuity'),
  createStoryEvent: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateStoryEvent', input),
  createFact: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateFact', input),
  updateFact: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateFact', id, input),
  confirmFact: (id: string) => call<Record<string, unknown>>('ConfirmFact', id),

  prepareManuscriptImport: (path: string) => call<Record<string, unknown>>('PrepareManuscriptImport', path),
  applyImportJob: (id: string, chapterIds: string[]) => call<Record<string, unknown>>('ApplyImportJob', id, chapterIds),
  createImportedProject: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateImportedProject', input),
  startWholeBookArchive: () => call<void>('StartWholeBookArchive'),

  createChapterAgentRun: (chapterId: string, prompt: string) =>
    call<AgentRunView>('CreateChapterAgentRun', chapterId, prompt),
  executeAgentRun: (id: string) => call<AgentRunView>('ExecuteAgentRun', id),
  cancelAgentRun: (id: string) => call<void>('CancelAgentRun', id),
  retryAgentRun: (id: string) => call<AgentRunView>('RetryAgentRun', id),
  agentRun: (id: string) => call<AgentRunView>('AgentRun', id),
  agentRuns: () => call<AgentRunView[]>('AgentRuns'),
  getJobSnapshot: (id: string) => call<JobSnapshotView>('GetJobSnapshot', id),
  getJobEvents: (jobId: string, afterSequence: number, limit: number) =>
    call<JobEventsPageView>('GetJobEvents', jobId, afterSequence, limit),
  listRecoverableJobs: () => call<Array<Record<string, unknown>>>('ListRecoverableJobs'),
  listCreativeHarnessDatasets: () => call<HarnessDatasetView>('ListCreativeHarnessDatasets'),
  previewCreativeHarnessRun: (input: Record<string, unknown>) => call<HarnessPreviewView>('PreviewCreativeHarnessRun', input),
  createCreativeHarnessRun: (input: Record<string, unknown>) => call<HarnessRunView>('CreateCreativeHarnessRun', input),
  executeCreativeHarnessRun: (id: string) => call<HarnessRunView>('ExecuteCreativeHarnessRun', id),
  cancelCreativeHarnessRun: (id: string) => call<void>('CancelCreativeHarnessRun', id),
  listCreativeHarnessRuns: () => call<HarnessRunView[]>('ListCreativeHarnessRuns'),
  getCreativeHarnessRun: (id: string) => call<HarnessRunView>('GetCreativeHarnessRun', id),
  deleteCreativeHarnessRun: (id: string) => call<void>('DeleteCreativeHarnessRun', id),
  retryCreativeHarnessCase: (runId: string, caseId: string) => call<HarnessRunView>('RetryCreativeHarnessCase', runId, caseId),
  getCreativeHarnessCaseResult: (runId: string, caseId: string, raterId: string) => call<HarnessCaseView>('GetCreativeHarnessCaseResult', runId, caseId, raterId),
  saveCreativeHarnessRating: (input: Record<string, unknown>) => call<HarnessCaseView>('SaveCreativeHarnessRating', input),
  compareCreativeHarnessRuns: (baseId: string, candidateId: string) => call<Record<string, unknown>>('CompareCreativeHarnessRuns', baseId, candidateId),
  exportCreativeHarnessReport: (id: string) => call<string>('ExportCreativeHarnessReport', id),

  saveCreationSample: (request: CreationSampleInput & { projectId: string; sessionId: string; expectedVersion: number; expectedRunSlotVersion: number; directionCandidateId: string; directionCardId: string }) => call<CreationWorkbenchView>('SaveCreationSample', request),
  saveCreationEntry: (request: { projectId: string; sessionId: string; expectedVersion: number; expectedRunSlotVersion: number; kind: 'idea' | 'author_plan'; material: string }) => call<CreationWorkbenchView>('SaveCreationEntry', request),
  useCreationMaterial: (request: { projectId: string; sessionId: string; expectedVersion: number; analysisId: string; stage: string }) => call<CreationWorkbenchView>('UseCreationMaterial', request),
  creationWorkbench: () => call<CreationWorkbenchView>('CreationWorkbench'),
  saveCreationIntent: (text: string) => call<CreationWorkbenchView>('SaveCreationIntent', text),
  setCreationAdvanceMode: (mode: string) => call<CreationWorkbenchView>('SetCreationAdvanceMode', mode),
  selectCreationDirection: (candidateId: string, cardId: string) => call<CreationWorkbenchView>('SelectCreationDirection', candidateId, cardId),
  keepCreationDirection: (cardId: string) => call<CreationWorkbenchView>('KeepCreationDirection', cardId),
  unkeepCreationDirection: (cardId: string) => call<CreationWorkbenchView>('UnkeepCreationDirection', cardId),
  previewRegenerateDirection: (request: {candidateId:string;targetCardId:string;instruction:string;idempotencyKey:string}) => call<DirectionActionView>('PreviewRegenerateDirection', request),
  confirmRegenerateDirection: (command: {actionId:string;argumentsHash:string;idempotencyKey:string}) => call<DirectionActionView>('ConfirmRegenerateDirection', command),
  previewCombineDirections: (request: {candidateId:string;sourceCardIds:string[];instruction:string;idempotencyKey:string}) => call<DirectionActionView>('PreviewCombineDirections', request),
  confirmCombineDirections: (command: {actionId:string;argumentsHash:string;idempotencyKey:string}) => call<DirectionActionView>('ConfirmCombineDirections', command),
  rejectDirectionAction: (actionId:string) => call<CreationWorkbenchView>('RejectDirectionAction', actionId),
  acceptCreationCandidate: (candidateId: string) => call<CreationWorkbenchView>('AcceptCreationCandidate', candidateId),
  advanceCreationStage: () => call<CreationWorkbenchView>('AdvanceCreationStage'),
  lookBackCreationStage: (stage: string) => call<CreationWorkbenchView>('LookBackCreationStage', stage),
  // 结构授权：allowAdd/allowRemove 里的集合名显式允许 AI 在该次修订内增删实体，未列出的集合一律不允许结构变化。
  previewCreationRevision: (request:{candidateId:string;paths:string[];instruction:string;idempotencyKey:string;allowAdd?:string[];allowRemove?:string[]}) => call<RevisionCard>('PreviewCreationRevision',request),
  sendCreationMessage: (request:{candidateId:string;entityRef:string;fieldPath:string;text:string;idempotencyKey:string}) => call<RevisionCard>('SendCreationMessage',request),
  previewCreationDiscussionAuthorization: (request:{candidateId:string;entityRef?:string;fieldPaths:string[];message:string;idempotencyKey:string}) => call<DiscussionTurnView>('PreviewCreationDiscussionAuthorization',request),
  confirmCreationDiscussionAuthorization: (command:{actionId:string;argumentsHash:string;idempotencyKey:string;message:string;messageIdempotencyKey:string}) => call<DiscussionTurnView>('ConfirmCreationDiscussionAuthorization',command),
  sendCreationDiscussionMessage: (request:{authorizationId:string;candidateId:string;entityRef?:string;fieldPaths:string[];text:string;idempotencyKey:string}) => call<DiscussionTurnView>('SendCreationDiscussionMessage',request),
  revokeCreationDiscussionAuthorization: (id:string) => call<CreationWorkbenchView>('RevokeCreationDiscussionAuthorization',id),
  previewCreationDiscussionSuggestion: (request:{messageId:string;suggestionId:string;idempotencyKey?:string}) => call<RevisionCard>('PreviewCreationDiscussionSuggestion',request),
 confirmCreationRevision: (command:{actionId:string;argumentsHash:string;idempotencyKey:string}) => call<RevisionCard>('ConfirmCreationRevision',command),
 rejectCreationRevision: (id:string) => call<CreationWorkbenchView>('RejectCreationRevision',id),
 // 部分采用：paths 省略或为空表示采用整组差异；非空时只采用给定路径，其余保持基线原文。
 applyCreationRevision: (command:{actionId:string;argumentsHash:string;idempotencyKey:string;paths?:string[]}) => call<CreationWorkbenchView>('ApplyCreationRevision',command),
  saveCreationFields: (request: {candidateId:string;changes:Record<string,string>}) => call<CreationWorkbenchView>('SaveCreationFields', request),
  // 结构增删：value 是新增实体的 JSON 对象字符串，删除时传空串；成功后返回新的候选版本投影。
  saveCreationStructure: (request: StructureEditCommand) => call<CreationWorkbenchView>('SaveCreationStructure', request),
  saveCreationCandidateDraft: (candidateId: string, payloadJson: string) =>
    call<CreationWorkbenchView>('SaveCreationCandidateDraft', candidateId, payloadJson),
  lockCreationField: (candidateId: string, path: string) => call<CreationWorkbenchView>('LockCreationField', candidateId, path),
  unlockCreationField: (candidateId: string, path: string) => call<CreationWorkbenchView>('UnlockCreationField', candidateId, path),
  previewCreationGenerate: (stage: string) => call<CreationCostPreview>('PreviewCreationGenerate', stage),
  confirmCreationGenerate: (input: { stage: string; confirmationDigest: string; idempotencyKey: string }) =>
    call<{ runId: string }>('ConfirmCreationGenerate', input),
  cancelCreationRun: () => call<void>('CancelCreationRun'),
  recoverCreation: () => call<CreationWorkbenchView>('RecoverCreation'),
  previewCreationArchive: () => call<{ confirmationDigest: string; title: string; characterCount: number; outlineCount: number; anchorCount: number; operations: Array<{kind:string;action:string;summary:string}>; excluded: Array<{id:string;scope:string}> }>('PreviewCreationArchive'),
  previewPlanningRearchive: (chapterId:string,reason:string) => call<PlanningRearchivePreview>('PreviewPlanningRearchive',chapterId,reason),
  confirmPlanningRearchive: (command:{request:PlanningRearchivePreview['request'];digest:string;idempotencyKey:string}) => call<PlanningRearchivePreview>('ConfirmPlanningRearchive',command),
  confirmCreationArchive: (input: { confirmationDigest: string; idempotencyKey: string }) =>
    call<{ changeSetId: string }>('ConfirmCreationArchive', input),
  resolveCreationImpact: (choice: string) => call<{ projection: CreationWorkbenchView['impact']; regenerateStages: string[] }>('ResolveCreationImpact', choice),
  // 逐项审阅：命令只改审阅元数据，返回的工作台投影才是权威状态。
  setCreationReview: (request: { candidateId: string; scope: 'entity' | 'section'; refId: string; decision: 'accept' | 'reject' | 'unreviewed' }) =>
    call<CreationWorkbenchView>('SetCreationReview', request),
  acceptAllCreationReview: (candidateId: string) => call<CreationWorkbenchView>('AcceptAllCreationReview', candidateId),
  useCreationCandidateVersion: (candidateId: string) => call<CreationWorkbenchView>('UseCreationCandidateVersion', candidateId),
  compareCreationCandidateVersions: (stage: string, baseCandidateId: string) =>
    call<CandidateCompareView>('CompareCreationCandidateVersions', stage, baseCandidateId),
  saveCreationReviewCursor: (request: { stage: string; candidateId: string; entityId: string; sectionId: string; comparisonRef: string }) =>
    call<CreationWorkbenchView>('SaveCreationReviewCursor', request),
};
