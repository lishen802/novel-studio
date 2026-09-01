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
  chapterHistory: (id: string) => call<Array<Record<string, unknown>>>('ChapterHistory', id),

  changeSets: () => call<ChangeSetView[]>('ChangeSets'),
  applyChangeSet: (id: string) => call<Record<string, unknown>>('ApplyChangeSet', id),
  rejectChangeSet: (id: string) => call<void>('RejectChangeSet', id),
  applyChangeOperation: (changeSetId: string, operationId: string) =>
    call<Record<string, unknown>>('ApplyChangeOperation', changeSetId, operationId),
  rejectChangeOperation: (changeSetId: string, operationId: string) =>
    call<void>('RejectChangeOperation', changeSetId, operationId),

  snapshots: () => call<SnapshotView[]>('Snapshots'),
  createSnapshot: (name: string) => call<SnapshotView>('CreateSnapshot', name),
  restoreSnapshot: (id: string) => call<Record<string, unknown>>('RestoreSnapshot', id),
  branches: () => call<Array<Record<string, unknown>>>('Branches'),
  createBranch: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateBranch', input),
  switchBranch: (id: string) => call<Record<string, unknown>>('SwitchBranch', id),

  storyStructure: () => call<StoryStructureView>('StoryStructure'),
  createOutlineNode: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateOutlineNode', input),
  createStoryLine: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateStoryLine', input),
  createSceneCard: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateSceneCard', input),
  storyWorld: () => call<StoryWorldView>('StoryWorld'),
  createCharacter: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateCharacter', input),
  updateCharacter: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateCharacter', id, input),
  createFaction: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateFaction', input),
  updateFaction: (id: string, input: Record<string, unknown>) => call<Record<string, unknown>>('UpdateFaction', id, input),
  worldAssets: () => call<WorldAssetsView>('WorldAssets'),
  createWorldRegion: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateWorldRegion', input),
  createLocation: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateLocation', input),
  createWorldRule: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateWorldRule', input),
  createForeshadowingHook: (input: Record<string, unknown>) => call<Record<string, unknown>>('CreateForeshadowingHook', input),
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
};
